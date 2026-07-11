// Package logstream provides a shared hub that supervises container log
// tailing for background subscribers (alerting, and later persistence). Each
// subscriber registers a ContainerSpec plus its own LogOptions; the hub keeps
// one tail per (subscription, host, container) so every subscriber controls
// its own backfill window, and fans parsed entries into per-subscription
// bounded buffers so a slow sink can never stall a tail read.
package logstream

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	defaultListTimeout    = 15 * time.Second
	defaultResyncInterval = 60 * time.Second
	defaultRetryBaseDelay = 2 * time.Second
	defaultRetryMaxDelay  = 30 * time.Second
)

// DockerProvider yields the current Docker client set; reading through it on
// every use keeps the hub correct across hot-swapped config updates.
type DockerProvider interface {
	Docker() *docker.MultiHostClient
}

// Record is one parsed log entry tagged with its origin container.
type Record struct {
	Host          string
	ContainerID   string
	ContainerName string
	Labels        map[string]string
	Entry         models.LogEntry
}

// ContainerSpec selects containers to tail. All dimensions are optional and
// ANDed together; an empty slice matches everything in that dimension.
type ContainerSpec struct {
	Hosts      []string
	Containers []string // exact container names
	Projects   []string // compose projects
}

// Docker Compose and recent podman-compose both set the com.docker label;
// older podman-compose releases only set the io.podman one. Mirrors the
// unexported helper in internal/docker/compose.go.
var composeProjectLabels = []string{
	"com.docker.compose.project",
	"io.podman.compose.project",
}

// specMatches reports whether a container (identified by host, name without
// the leading "/", and labels) is selected by spec.
func specMatches(spec ContainerSpec, host, name string, labels map[string]string) bool {
	if len(spec.Hosts) > 0 && !slices.Contains(spec.Hosts, host) {
		return false
	}
	if len(spec.Containers) == 0 && len(spec.Projects) == 0 {
		return true
	}
	if slices.Contains(spec.Containers, name) {
		return true
	}
	for _, project := range spec.Projects {
		for _, label := range composeProjectLabels {
			if labels[label] == project {
				return true
			}
		}
	}
	return false
}

// containerKey identifies one container on one host.
type containerKey struct {
	host string
	id   string
}

// listResult is a container snapshot posted back to the run loop by the
// single-flight listing goroutine.
type listResult struct {
	snapshot map[string][]models.ContainerInfo
	hostErrs []docker.HostError
	err      error
}

// tailExit notifies the run loop that a tail goroutine has stopped.
type tailExit struct {
	sub *subscription
	key containerKey
	t   *tail
}

// Hub owns the container log tails shared by all subscribers. Each live
// subscription gets its own tails (with its own LogOptions); the hub's run
// loop is the single owner of all subscription and tail state.
type Hub struct {
	provider DockerProvider
	source   func() engineClient

	reqCh      chan func()
	pokeCh     chan struct{}
	listCh     chan listResult
	tailExitCh chan tailExit
	stopped    chan struct{} // closed when the hub stops accepting requests
	finished   chan struct{} // closed when Run has fully drained

	listTimeout    time.Duration
	resyncInterval time.Duration
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration

	// State below is owned exclusively by the run loop goroutine.
	runCtx       context.Context
	subs         map[*subscription]struct{}
	events       <-chan docker.EngineEvent
	eventsCancel context.CancelFunc
	eventsClient engineClient
	listInFlight bool
	listQueued   bool
	tailWg       sync.WaitGroup
}

// New creates a hub that tails containers through the provider's current
// Docker client set.
func New(provider DockerProvider) *Hub {
	return newHub(provider, func() engineClient { return dockerAdapter{c: provider.Docker()} })
}

// newHub builds a hub over an arbitrary client source; tests inject fakes
// here. source must return comparable values so client swaps are detectable.
func newHub(provider DockerProvider, source func() engineClient) *Hub {
	return &Hub{
		provider:       provider,
		source:         source,
		reqCh:          make(chan func()),
		pokeCh:         make(chan struct{}, 1),
		listCh:         make(chan listResult, 1),
		tailExitCh:     make(chan tailExit),
		stopped:        make(chan struct{}),
		finished:       make(chan struct{}),
		listTimeout:    defaultListTimeout,
		resyncInterval: defaultResyncInterval,
		retryBaseDelay: defaultRetryBaseDelay,
		retryMaxDelay:  defaultRetryMaxDelay,
		subs:           make(map[*subscription]struct{}),
	}
}

// do runs fn on the run loop goroutine. It returns false (without running fn)
// once the hub has begun shutting down.
func (h *Hub) do(fn func()) bool {
	select {
	case h.reqCh <- fn:
		return true
	case <-h.stopped:
		return false
	}
}

// Subscribe registers a sink for records from containers matching spec,
// tailed with the subscriber's opts. The returned function removes the
// subscription; after it returns, sink is never called again (do not call
// unsubscribe from inside the sink). Subscribe blocks until the hub's run
// loop is started; after shutdown it registers nothing and returns a no-op.
// Follow is forced on: hub tails are continuous by nature, and a non-follow
// tail would end immediately and be retried as a failure.
func (h *Hub) Subscribe(spec ContainerSpec, opts models.LogOptions, sink func(Record)) (unsubscribe func()) {
	opts.Follow = true
	_, unsubscribe = h.subscribe(spec, opts, sink)
	return unsubscribe
}

// subscribe is the internal form of Subscribe that also exposes the
// subscription handle (used by tests to observe drop counters).
func (h *Hub) subscribe(spec ContainerSpec, opts models.LogOptions, sink func(Record)) (*subscription, func()) {
	sub := newSubscription(spec, opts, sink)
	registered := h.do(func() {
		h.subs[sub] = struct{}{}
		go sub.deliverLoop()
		h.requestList()
	})
	if !registered {
		return sub, func() {}
	}

	var once sync.Once
	return sub, func() {
		once.Do(func() {
			if !h.do(func() { h.removeSub(sub) }) {
				// Hub already shut down; its shutdown path closes the
				// subscription (close is idempotent either way).
				sub.close(true)
			}
			<-sub.delivered
		})
	}
}

// removeSub runs on the loop: it drops the subscription, cancels its tails,
// and stops its delivery goroutine, discarding buffered records.
func (h *Hub) removeSub(sub *subscription) {
	if _, ok := h.subs[sub]; !ok {
		return
	}
	delete(h.subs, sub)
	for key, t := range sub.tails {
		t.cancel()
		delete(sub.tails, key)
	}
	sub.close(true)
}

// Reconcile pokes the run loop to re-list containers and converge tails.
// Non-blocking; pokes coalesce. Called on container lifecycle events, config
// reloads, and subscription changes.
func (h *Hub) Reconcile() {
	select {
	case h.pokeCh <- struct{}{}:
	default:
	}
}

// Run drives the hub's supervision until ctx is cancelled. It is typically
// invoked in its own goroutine; use Wait to block until every tail and
// delivery goroutine has fully stopped.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.finished)
	h.runCtx = ctx

	h.openEventStream(h.source())

	ticker := time.NewTicker(h.resyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case fn := <-h.reqCh:
			fn()
		case <-h.pokeCh:
			h.requestList()
		case <-ticker.C:
			h.requestList()
		case ev, ok := <-h.events:
			if !ok {
				// All host watchers stopped (e.g. the client was closed
				// after a hot swap). Re-list; handleList reopens the stream.
				h.events = nil
				h.requestList()
				continue
			}
			h.handleEvent(ev)
		case res := <-h.listCh:
			h.handleList(res)
		case ex := <-h.tailExitCh:
			h.handleTailExit(ex)
		}
	}
}

// Wait blocks until the hub has shut down: Run returned and all container
// tails and delivery goroutines have stopped.
func (h *Hub) Wait() {
	<-h.finished
}

// openEventStream (re)opens the engine event stream on client under a child
// context of the run context.
func (h *Hub) openEventStream(client engineClient) {
	ctx, cancel := context.WithCancel(h.runCtx)
	h.eventsCancel = cancel
	h.events = client.streamEvents(ctx)
	h.eventsClient = client
}

// requestList starts a single-flight container listing; pokes arriving while
// one is in flight coalesce into at most one follow-up listing.
func (h *Hub) requestList() {
	if h.listInFlight {
		h.listQueued = true
		return
	}
	h.listInFlight = true

	client := h.source()
	runCtx := h.runCtx
	timeout := h.listTimeout
	go func() {
		ctx, cancel := context.WithTimeout(runCtx, timeout)
		defer cancel()
		snapshot, hostErrs, err := client.listContainers(ctx)
		select {
		case h.listCh <- listResult{snapshot: snapshot, hostErrs: hostErrs, err: err}:
		case <-runCtx.Done():
		}
	}()
}

// handleEvent is the fast path: start spawns matching tails immediately,
// die/destroy cancel them, rename is treated as destroy plus a re-list.
func (h *Hub) handleEvent(ev docker.EngineEvent) {
	base, _, _ := strings.Cut(ev.Action, ": ")
	switch base {
	case "start":
		name := strings.TrimPrefix(ev.ContainerName, "/")
		key := containerKey{host: ev.Host, id: ev.ContainerID}
		for sub := range h.subs {
			if _, ok := sub.tails[key]; ok {
				continue
			}
			if !specMatches(sub.spec, ev.Host, name, ev.Labels) {
				continue
			}
			h.spawnTail(sub, key, name, ev.Labels)
		}
	case "die", "destroy":
		h.cancelTails(containerKey{host: ev.Host, id: ev.ContainerID})
	case "rename":
		h.cancelTails(containerKey{host: ev.Host, id: ev.ContainerID})
		h.requestList()
	}
}

// cancelTails cancels every subscription's tail for key.
func (h *Hub) cancelTails(key containerKey) {
	for sub := range h.subs {
		if t, ok := sub.tails[key]; ok {
			t.cancel()
			delete(sub.tails, key)
		}
	}
}

// handleList converges tails against a fresh container snapshot. Hosts that
// failed to list keep their existing tails untouched: a genuinely dead host's
// tails exit on their own and are retried by the next resync.
func (h *Hub) handleList(res listResult) {
	h.listInFlight = false
	defer func() {
		if h.listQueued {
			h.listQueued = false
			h.requestList()
		}
	}()

	// Hot-swap check: if the provider's client changed since the event stream
	// was opened (or the stream closed), reopen it on the current client.
	if current := h.source(); h.events == nil || current != h.eventsClient {
		h.eventsCancel()
		h.openEventStream(current)
	}

	if res.err != nil {
		log.Printf("logstream: container listing failed: %v", res.err)
		return
	}
	for _, hostErr := range res.hostErrs {
		log.Printf("logstream: listing containers on host %s failed: %v", hostErr.HostName, hostErr.Err)
	}

	type containerMeta struct {
		name   string
		labels map[string]string
	}
	running := make(map[containerKey]containerMeta)
	listedHosts := make(map[string]bool, len(res.snapshot))
	for host, containers := range res.snapshot {
		listedHosts[host] = true
		for _, ctr := range containers {
			if ctr.State != "running" {
				continue
			}
			name := ""
			if len(ctr.Names) > 0 {
				name = strings.TrimPrefix(ctr.Names[0], "/")
			}
			running[containerKey{host: host, id: ctr.ID}] = containerMeta{name: name, labels: ctr.Labels}
		}
	}

	for sub := range h.subs {
		// Cancel tails whose host listed successfully but whose container is
		// no longer running (or no longer matches, e.g. after a rename).
		for key, t := range sub.tails {
			if !listedHosts[key.host] {
				continue
			}
			if meta, ok := running[key]; ok && specMatches(sub.spec, key.host, meta.name, meta.labels) {
				continue
			}
			t.cancel()
			delete(sub.tails, key)
		}
		// Spawn tails for matching running containers we are not tailing yet.
		for key, meta := range running {
			if _, ok := sub.tails[key]; ok {
				continue
			}
			if !specMatches(sub.spec, key.host, meta.name, meta.labels) {
				continue
			}
			h.spawnTail(sub, key, meta.name, meta.labels)
		}
	}
}

// handleTailExit removes a self-terminated tail (stream ended and retries
// exhausted) so the next resync or start event can respawn it.
func (h *Hub) handleTailExit(ex tailExit) {
	if _, ok := h.subs[ex.sub]; !ok {
		return
	}
	if current, ok := ex.sub.tails[ex.key]; ok && current == ex.t {
		delete(ex.sub.tails, ex.key)
	}
}

// shutdown drains the hub: reject new requests, cancel the event stream and
// every tail, wait for tails to stop pushing, then let each delivery
// goroutine finish its buffered records.
func (h *Hub) shutdown() {
	close(h.stopped)
	h.eventsCancel()
	for sub := range h.subs {
		for key, t := range sub.tails {
			t.cancel()
			delete(sub.tails, key)
		}
	}
	h.tailWg.Wait()
	for sub := range h.subs {
		sub.close(false) // drain buffered records
	}
	for sub := range h.subs {
		<-sub.delivered
	}
}
