// Package health records whether LogDeck can still observe its hosts and
// containers. It is fed by the code that already talks to them: the log hub's
// container listings and tails, and the log store's backfills, writes and
// janitor. Nothing here calls the engine, and updates happen per state change
// or per batch, never per log line.
//
// Every method is safe on a nil *Tracker, which records nothing, so tests and
// callers without health reporting can pass nil.
package health

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// Observation states. Unsupported and excluded are intentional gaps, not
// failures: the engine cannot read the container's logs, or persistence is
// off for it.
const (
	StateOK          = "ok"
	StatePending     = "pending"
	StateFailing     = "failing"
	StateUnsupported = "unsupported"
	StateExcluded    = "excluded"
)

// maxReasonNames caps how many container names a host failure lists.
const maxReasonNames = 5

// Observation is the state of one collection path. Since is when State began;
// LastOK is the last time the path worked.
type Observation struct {
	State  string    `json:"state"`
	Since  time.Time `json:"since"`
	LastOK time.Time `json:"lastOk,omitzero"`
	Error  string    `json:"error,omitempty"`
}

// ContainerHealth is one container's collection state. Live is the
// background tail that feeds alerts and the log store, nil when nothing tails
// the container. Stored is the log store's engine read, nil when persistence
// is off or has not read the container yet.
type ContainerHealth struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Live   *Observation `json:"live,omitempty"`
	Stored *Observation `json:"stored,omitempty"`
}

// HostHealth is one host's listing state and its tracked containers.
type HostHealth struct {
	Host string `json:"host"`
	Observation
	Containers []ContainerHealth `json:"containers"`
}

// StorageHealth is the log store's write and disk state. The counters are
// filled in by the store at read time.
type StorageHealth struct {
	Observation
	LowSpace  bool  `json:"lowSpace"`
	FreeBytes int64 `json:"freeBytes"`
	// DroppedLines counts live lines the writer fell behind on since startup.
	// Each one marks a gap that is re-read from the engine.
	DroppedLines uint64 `json:"droppedLines"`
	Backlog      int    `json:"backlog"`
	BacklogCap   int    `json:"backlogCap"`
	// Recovering counts containers with a gap still waiting to be re-read.
	Recovering int `json:"recovering"`
}

// Report is the full collection health snapshot.
type Report struct {
	Status string       `json:"status"` // "ok" | "degraded"
	Hosts  []HostHealth `json:"hosts"`
	// Storage is nil when log persistence is disabled.
	Storage *StorageHealth `json:"storage,omitempty"`
}

// Failure is one alertable collection failure: a host that cannot be listed
// or has containers that cannot be collected, or the log store (Host "").
type Failure struct {
	Host   string
	Since  time.Time
	Reason string
}

type containerKey struct {
	host string
	id   string
}

type containerState struct {
	name   string
	live   *Observation
	stored *Observation
}

type storageState struct {
	write     Observation
	lowSpace  bool
	lowSince  time.Time
	freeBytes int64
}

// Tracker holds the latest collection state. One mutex guards it all: every
// update is a few map operations, and none is on a per-line path.
type Tracker struct {
	mu         sync.Mutex
	now        func() time.Time
	hosts      map[string]*Observation
	containers map[containerKey]*containerState
	storage    storageState
}

func NewTracker() *Tracker {
	return &Tracker{
		now:        time.Now,
		hosts:      make(map[string]*Observation),
		containers: make(map[containerKey]*containerState),
	}
}

// set moves o to state. A pending update never hides a failure: a retry stays
// failing until it works. err is the failure or exclusion reason.
func set(o *Observation, now time.Time, state, err string) {
	if state == StatePending && o.State == StateFailing {
		return
	}
	if o.State != state {
		o.State = state
		o.Since = now
	}
	o.Error = err
	if state == StateOK {
		o.LastOK = now
	}
}

// container returns the entry for a container, creating it. Callers hold mu.
func (t *Tracker) container(host, id, name string) *containerState {
	key := containerKey{host: host, id: id}
	c := t.containers[key]
	if c == nil {
		c = &containerState{}
		t.containers[key] = c
	}
	if name != "" {
		c.name = name
	}
	return c
}

// ObserveListing records a container listing: listed hosts are reachable,
// failed ones are not. Hosts in neither were removed from the config and are
// forgotten, as are containers a listed host no longer has. A container that
// stopped keeps its stored state but loses its live one, since nothing tails
// it.
func (t *Tracker) ObserveListing(snapshot map[string][]models.ContainerInfo, hostErrs []docker.HostError) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()

	failed := make(map[string]bool, len(hostErrs))
	for _, hostErr := range hostErrs {
		failed[hostErr.HostName] = true
		o := t.hosts[hostErr.HostName]
		if o == nil {
			o = &Observation{}
			t.hosts[hostErr.HostName] = o
		}
		set(o, now, StateFailing, hostErr.Err.Error())
	}

	running := make(map[containerKey]bool)
	listed := make(map[containerKey]bool)
	for host, containers := range snapshot {
		o := t.hosts[host]
		if o == nil {
			o = &Observation{}
			t.hosts[host] = o
		}
		set(o, now, StateOK, "")
		for _, ctr := range containers {
			key := containerKey{host: host, id: ctr.ID}
			listed[key] = true
			if ctr.State == "running" {
				running[key] = true
			}
		}
	}

	for host := range t.hosts {
		if _, ok := snapshot[host]; !ok && !failed[host] {
			delete(t.hosts, host)
		}
	}
	for key, c := range t.containers {
		if failed[key.host] {
			continue
		}
		if _, ok := snapshot[key.host]; !ok || !listed[key] {
			delete(t.containers, key)
			continue
		}
		if !running[key] {
			c.live = nil
			if c.stored == nil {
				delete(t.containers, key)
			}
		}
	}
}

// ContainerRemoved forgets a destroyed container at once, so a tail that
// failed because the container went away is not reported until the next
// listing.
func (t *Tracker) ContainerRemoved(host, id string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.containers, containerKey{host: host, id: id})
}

// TailStarted records that the hub began tailing a container.
func (t *Tracker) TailStarted(host, id, name string) {
	t.setLive(host, id, name, StatePending, "")
}

// TailAttached records that the engine accepted a tail's log request.
func (t *Tracker) TailAttached(host, id string) {
	t.setLive(host, id, "", StateOK, "")
}

// TailFailed records a tail attempt that ended with an error.
func (t *Tracker) TailFailed(host, id string, err error) {
	state := StateFailing
	if docker.UnreadableLogs(err) {
		state = StateUnsupported
	}
	t.setLive(host, id, "", state, err.Error())
}

func (t *Tracker) setLive(host, id, name, state, err string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.container(host, id, name)
	if c.live == nil {
		c.live = &Observation{}
	}
	set(c.live, t.now(), state, err)
}

// BackfillScheduled records that the log store is about to read a container
// from the engine, for its first read or to heal a gap.
func (t *Tracker) BackfillScheduled(host, id, name string) {
	t.setStored(host, id, name, StatePending, "")
}

// BackfillDone records a finished engine read. pending reports that a newer
// gap is already waiting for another read.
func (t *Tracker) BackfillDone(host, id string, err error, pending bool) {
	switch {
	case err != nil:
		t.setStored(host, id, "", StateFailing, err.Error())
	case pending:
		t.setStored(host, id, "", StatePending, "")
	default:
		t.setStored(host, id, "", StateOK, "")
	}
}

// BackfillExcluded records a container the log store does not persist.
func (t *Tracker) BackfillExcluded(host, id, name, reason string) {
	t.setStored(host, id, name, StateExcluded, reason)
}

func (t *Tracker) setStored(host, id, name, state, err string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.container(host, id, name)
	if c.stored == nil {
		c.stored = &Observation{}
	}
	set(c.stored, t.now(), state, err)
}

// StorageWrite records the outcome of a log store write. A nil err after a
// failure is the recovery.
func (t *Tracker) StorageWrite(err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err != nil {
		set(&t.storage.write, t.now(), StateFailing, err.Error())
		return
	}
	set(&t.storage.write, t.now(), StateOK, "")
}

// StorageSpace records the free disk space under the log store and whether it
// is too little for the store to reach its cap.
func (t *Tracker) StorageSpace(freeBytes int64, low bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if low && !t.storage.lowSpace {
		t.storage.lowSince = t.now()
	}
	t.storage.freeBytes = freeBytes
	t.storage.lowSpace = low
}

// Report snapshots the state of every configured host, in the order given.
// A host the hub has not listed yet is pending. Storage is left for the caller
// to fill in, since only it knows whether persistence is enabled.
func (t *Tracker) Report(hosts []string) Report {
	report := Report{Status: "ok", Hosts: make([]HostHealth, 0, len(hosts))}
	if t == nil {
		return report
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	byHost := make(map[string][]ContainerHealth)
	for key, c := range t.containers {
		byHost[key.host] = append(byHost[key.host], ContainerHealth{
			ID:     key.id,
			Name:   c.name,
			Live:   copyObservation(c.live),
			Stored: copyObservation(c.stored),
		})
	}
	for _, host := range hosts {
		h := HostHealth{Host: host, Observation: Observation{State: StatePending}}
		if o := t.hosts[host]; o != nil {
			h.Observation = *o
		}
		h.Containers = byHost[host]
		if h.Containers == nil {
			h.Containers = []ContainerHealth{}
		}
		slices.SortFunc(h.Containers, func(a, b ContainerHealth) int {
			return strings.Compare(a.Name, b.Name)
		})
		report.Hosts = append(report.Hosts, h)
	}
	if len(t.failures()) > 0 {
		report.Status = "degraded"
	}
	return report
}

// Storage snapshots the log store's write and disk state.
func (t *Tracker) Storage() StorageHealth {
	if t == nil {
		return StorageHealth{Observation: Observation{State: StateOK}}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := StorageHealth{
		Observation: t.storage.write,
		LowSpace:    t.storage.lowSpace,
		FreeBytes:   t.storage.freeBytes,
	}
	if s.State == "" {
		s.State = StateOK
	}
	return s
}

// Degraded reports whether any collection path is failing right now.
func (t *Tracker) Degraded() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.failures()) > 0
}

// Failures lists what is failing right now, one entry per host so an
// unreachable host is one alert rather than one per container. A host that
// cannot be listed hides its containers' failures, which follow from it.
func (t *Tracker) Failures() []Failure {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failures()
}

func (t *Tracker) failures() []Failure {
	var out []Failure
	for host, o := range t.hosts {
		if o.State == StateFailing {
			out = append(out, Failure{Host: host, Since: o.Since, Reason: "cannot list containers: " + o.Error})
		}
	}

	type hostContainers struct {
		names []string
		since time.Time
	}
	byHost := make(map[string]*hostContainers)
	for key, c := range t.containers {
		if o := t.hosts[key.host]; o != nil && o.State == StateFailing {
			continue
		}
		for _, o := range []*Observation{c.live, c.stored} {
			if o == nil || o.State != StateFailing {
				continue
			}
			h := byHost[key.host]
			if h == nil {
				h = &hostContainers{since: o.Since}
				byHost[key.host] = h
			}
			if o.Since.Before(h.since) {
				h.since = o.Since
			}
			name := c.name
			if name == "" {
				name = key.id
			}
			if !slices.Contains(h.names, name) {
				h.names = append(h.names, name)
			}
		}
	}
	for host, h := range byHost {
		out = append(out, Failure{Host: host, Since: h.since, Reason: containersReason(h.names)})
	}

	var storage []string
	var since time.Time
	if t.storage.write.State == StateFailing {
		storage = append(storage, "writes are failing: "+t.storage.write.Error)
		since = t.storage.write.Since
	}
	if t.storage.lowSpace {
		storage = append(storage, fmt.Sprintf("low disk space (%d MB free)", t.storage.freeBytes/(1<<20)))
		if since.IsZero() || t.storage.lowSince.Before(since) {
			since = t.storage.lowSince
		}
	}
	if len(storage) > 0 {
		out = append(out, Failure{Since: since, Reason: "log storage " + strings.Join(storage, "; ")})
	}

	slices.SortFunc(out, func(a, b Failure) int { return strings.Compare(a.Host, b.Host) })
	return out
}

func containersReason(names []string) string {
	slices.Sort(names)
	unit := "containers"
	if len(names) == 1 {
		unit = "container"
	}
	shown := names
	more := ""
	if len(names) > maxReasonNames {
		shown = names[:maxReasonNames]
		more = fmt.Sprintf(" and %d more", len(names)-maxReasonNames)
	}
	return fmt.Sprintf("cannot collect logs from %d %s: %s%s", len(names), unit, strings.Join(shown, ", "), more)
}

func copyObservation(o *Observation) *Observation {
	if o == nil {
		return nil
	}
	c := *o
	return &c
}
