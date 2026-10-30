// Package alerts contains the alerting engine: it watches container events,
// log streams, collection health and container resource use, matches them
// against the configured rules, and delivers notifications to the configured
// channels.
//
// Concurrency model: a single run goroutine owns all mutable state (compiled
// rules, subscription handles, rate/cooldown windows, the event-stream child
// context). Log-rule sinks run on the hub's delivery goroutines and only push
// non-blocking match messages onto firedCh. A single dispatcher goroutine
// consumes fired alerts, appends them to history, delivers them to the
// configured channels, and records each delivery outcome on the history entry.
// Helper goroutines exist only for single container-inspect lookups and the
// resource-rule stats sample taken on each resync tick.
package alerts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/health"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/redact"
)

const (
	firedBuffer    = 1024
	inspectBuffer  = 64
	dispatchBuffer = 128

	defaultResyncInterval = 60 * time.Second
	inspectTimeout        = 5 * time.Second
	// sampleTimeout bounds one resource-rule stats sample across every host.
	sampleTimeout = 30 * time.Second
	// sampleJitter absorbs the scheduling delay between resync ticks, so a
	// condition held for exactly a rule's window fires on that tick rather
	// than the next one.
	sampleJitter = 5 * time.Second
	// deliverBudget bounds one delivery cycle (attempt + retry wait +
	// attempt).
	deliverBudget = 30 * time.Second
	// shutdownDrainBudget bounds the entire dispatch-queue drain after the
	// Start context is cancelled: once it expires, remaining alerts are
	// recorded as failed ("shutdown") instead of waiting out their delivery
	// timeouts one by one.
	shutdownDrainBudget = 30 * time.Second
	// oomDieWindow is how long after an "oom" event a "die" for the same
	// container is treated as part of the same incident by rules watching
	// both actions.
	oomDieWindow = 10 * time.Second
	// stopWindow is how long after a kill or restart event a die counts as
	// a stop rather than a crash.
	stopWindow = 60 * time.Second
	// restartSettle separates a restart event that announces a stop from one
	// that trails a die or start already seen.
	restartSettle = 2 * time.Second
	// windowIdleTTL is how long a (rule, host, container) key may go without
	// a match before its window state is pruned.
	windowIdleTTL = 24 * time.Hour
)

// DockerProvider yields the current Docker client set; reading through it on
// every use keeps the engine correct across hot-swapped config updates.
type DockerProvider interface {
	Docker() *docker.MultiHostClient
}

// engineHub is the slice of logstream.Hub the engine uses; tests inject
// fakes. *logstream.Hub satisfies it directly.
type engineHub interface {
	Subscribe(spec logstream.ContainerSpec, opts models.LogOptions, sink func(logstream.Record)) func()
}

// eventClient is the engine's view of the Docker client set. Implementations
// must be comparable so client hot-swaps are detectable by value comparison.
type eventClient interface {
	streamEvents(ctx context.Context) <-chan docker.EngineEvent
	inspectExit(ctx context.Context, host, containerID string) (exitCode string, oomKilled bool, err error)
	inspectHealth(ctx context.Context, host, containerID string) (status string, err error)
	stoppedByUser(ctx context.Context, host, containerID string) bool
	// sampleStats reads stats for the running containers any of rules
	// targets. listed holds the hosts whose containers could be listed.
	sampleStats(ctx context.Context, rules []*compiledRule) (containers []sampledContainer, listed map[string]bool)
}

// dockerEventAdapter adapts *docker.MultiHostClient to eventClient. It is a
// comparable value type: two adapters are equal iff they wrap the same client
// pointer.
type dockerEventAdapter struct {
	c *docker.MultiHostClient
}

func (a dockerEventAdapter) streamEvents(ctx context.Context) <-chan docker.EngineEvent {
	return a.c.StreamEngineEvents(ctx)
}

func (a dockerEventAdapter) inspectExit(ctx context.Context, host, containerID string) (string, bool, error) {
	resp, err := a.c.GetContainer(ctx, host, containerID)
	if err != nil {
		return "", false, err
	}
	if resp.State == nil {
		return "", false, fmt.Errorf("inspect response has no state")
	}
	return strconv.Itoa(resp.State.ExitCode), resp.State.OOMKilled, nil
}

func (a dockerEventAdapter) stoppedByUser(ctx context.Context, host, containerID string) bool {
	return a.c.StoppedByUser(ctx, host, containerID)
}

// inspectHealth resolves a container's current healthcheck state. Returns ""
// (not an error) for a container with no healthcheck configured. Works on both
// Docker and Podman, which is why it can recover Podman's health state after
// the SDK drops it from the bare-action event.
func (a dockerEventAdapter) inspectHealth(ctx context.Context, host, containerID string) (string, error) {
	resp, err := a.c.GetContainer(ctx, host, containerID)
	if err != nil {
		return "", err
	}
	if resp.State == nil || resp.State.Health == nil {
		return "", nil
	}
	return resp.State.Health.Status, nil
}

// sampleStats lists the running containers once per host the rules can
// target, then reads stats only for the containers they target. The stats
// reads go through the same short-lived cache the dashboard uses.
func (a dockerEventAdapter) sampleStats(ctx context.Context, rules []*compiledRule) ([]sampledContainer, map[string]bool) {
	byHost := a.c.RunningContainers(ctx, func(host string) bool {
		return slices.ContainsFunc(rules, func(r *compiledRule) bool {
			return len(r.spec.Hosts) == 0 || slices.Contains(r.spec.Hosts, host)
		})
	})
	listed := make(map[string]bool, len(byHost))
	var containers []sampledContainer
	var ids []docker.ContainerIdentifier
	for host, infos := range byHost {
		listed[host] = true
		for _, info := range infos {
			name := ""
			if len(info.Names) > 0 {
				name = strings.TrimPrefix(info.Names[0], "/")
			}
			if !slices.ContainsFunc(rules, func(r *compiledRule) bool { return r.spec.Matches(host, name, info.Labels) }) {
				continue
			}
			containers = append(containers, sampledContainer{host: host, id: info.ID, name: name, labels: info.Labels})
			ids = append(ids, docker.ContainerIdentifier{ID: info.ID, Host: host})
		}
	}
	if len(ids) == 0 {
		return nil, listed
	}
	stats := a.c.GetBulkContainerStats(ctx, ids)
	byID := make(map[string]*models.ContainerStats, len(stats))
	for i := range stats {
		byID[stats[i].Host+"|"+stats[i].ID] = &stats[i]
	}
	for i := range containers {
		containers[i].stats = byID[containers[i].host+"|"+containers[i].id]
	}
	return containers, listed
}

// sampledContainer is one running container a resource rule targets, with
// its stats; stats is nil when the read failed.
type sampledContainer struct {
	host, id, name string
	labels         map[string]string
	stats          *models.ContainerStats
}

// statsSample is one resource-rule sample posted back to the run loop.
type statsSample struct {
	at         time.Time
	containers []sampledContainer
	listed     map[string]bool
}

// matchMsg is one log-rule match pushed from a sink to the run loop.
type matchMsg struct {
	gen           uint64
	rule          *compiledRule
	host          string
	containerID   string
	containerName string
	project       string
	sample        string
}

// inspectResult is the outcome of an event lookup that needed a container
// inspect (a die-event exit code, or a Podman bare-action health state) posted
// back to the run loop.
type inspectResult struct {
	ev       docker.EngineEvent
	action   string // "die" | "unhealthy"
	exitCode string // die only
	fire     bool
}

// Engine evaluates alert rules against engine events and log records and
// records fired alerts in an in-memory history.
type Engine struct {
	hub      engineHub
	source   func() eventClient
	alertsFn func() config.AlertsConfig

	hist  *history
	notif *notifier

	reconcileCh chan struct{}
	firedCh     chan matchMsg
	inspectCh   chan inspectResult
	statsCh     chan statsSample
	dispatchCh  chan models.Alert
	flushStop   chan struct{}

	// deliverCtx survives Start-context cancellation so in-flight deliveries
	// can finish during shutdown, but is cancelled once shutdownDrainBudget
	// elapses after shutdown begins; skipRetry aborts retry waits once
	// shutdown begins. Both are set in Start before any goroutine reads them.
	deliverCtx context.Context
	skipRetry  <-chan struct{}

	// health is read by collection rules on every resync tick; nil disables
	// them.
	health *health.Tracker

	resyncInterval time.Duration
	now            func() time.Time
	wg             sync.WaitGroup

	matchDrops  atomic.Uint64
	loggedDrops uint64 // owned by the run loop
}

// NewEngine creates an alerting engine that reads rules through manager,
// watches events through the provider's Docker clients, subscribes to log
// records through hub, and evaluates collection rules against tracker.
func NewEngine(provider DockerProvider, manager *config.Manager, hub *logstream.Hub, tracker *health.Tracker) *Engine {
	alertsFn := func() config.AlertsConfig {
		fc := manager.FileConfigSnapshot()
		if fc.Alerts == nil {
			return config.AlertsConfig{}
		}
		return *fc.Alerts
	}
	e := newEngine(hub, func() eventClient { return dockerEventAdapter{c: provider.Docker()} }, alertsFn, HistoryPath(manager.ConfigFilePath()))
	e.health = tracker
	return e
}

// HistoryPath returns the alert history file that sits next to the config file.
func HistoryPath(configFilePath string) string {
	return filepath.Join(filepath.Dir(configFilePath), "alerts-history.json")
}

// newEngine is the internal constructor; tests inject fakes here.
func newEngine(hub engineHub, source func() eventClient, alertsFn func() config.AlertsConfig, historyPath string) *Engine {
	return &Engine{
		hub:            hub,
		source:         source,
		alertsFn:       alertsFn,
		hist:           newHistory(historyPath, historyCap),
		notif:          newNotifier(),
		reconcileCh:    make(chan struct{}, 1),
		firedCh:        make(chan matchMsg, firedBuffer),
		inspectCh:      make(chan inspectResult, inspectBuffer),
		statsCh:        make(chan statsSample, 1),
		dispatchCh:     make(chan models.Alert, dispatchBuffer),
		flushStop:      make(chan struct{}),
		resyncInterval: defaultResyncInterval,
		now:            time.Now,
	}
}

// Start launches the engine's background loops (event watching and rule
// evaluation) tied to ctx. It returns immediately; use Wait to block until
// shutdown completes.
func (e *Engine) Start(ctx context.Context) {
	e.hist.load()
	drainCtx, drainCancel := context.WithCancel(context.WithoutCancel(ctx))
	e.deliverCtx = drainCtx
	e.skipRetry = ctx.Done()
	e.wg.Add(4)
	go func() {
		defer e.wg.Done()
		e.run(ctx)
	}()
	go func() {
		defer e.wg.Done()
		e.dispatchLoop()
	}()
	go func() {
		defer e.wg.Done()
		e.hist.flushLoop(e.flushStop)
	}()
	// The whole post-shutdown dispatch drain shares one budget: deliverCtx is
	// cancelled shutdownDrainBudget after the Start context, so a queue of
	// alerts against an unresponsive webhook cannot stall shutdown.
	go func() {
		defer e.wg.Done()
		defer drainCancel()
		<-ctx.Done()
		timer := time.NewTimer(shutdownDrainBudget)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-e.flushStop: // drain finished within the budget
		}
	}()
}

// Wait blocks until the engine's background loops have stopped after the
// Start context was cancelled.
func (e *Engine) Wait() {
	e.wg.Wait()
}

// Reconcile re-reads the rule set and the current Docker client set and
// adjusts event watches and log subscriptions to match. Called on rule
// changes and after the Docker clients are hot-swapped. Non-blocking; pokes
// coalesce.
func (e *Engine) Reconcile() {
	select {
	case e.reconcileCh <- struct{}{}:
	default:
	}
}

// TestChannel delivers a synthetic alert to a single channel and reports the
// delivery outcome. The test alert is not recorded in history.
func (e *Engine) TestChannel(ctx context.Context, channel config.AlertChannel) models.DeliveryResult {
	alert := models.Alert{
		ID:       newAlertID(),
		RuleName: "Test channel",
		Type:     "test",
		Reason:   "test notification from LogDeck",
		Count:    1,
		FiredAt:  e.now().UTC().Format(time.RFC3339),
	}
	return e.notif.deliver(ctx, channel, alert, ctx.Done())
}

// History returns the most recent fired alerts, newest first, capped at
// limit (limit <= 0 falls back to a default). Always returns a non-nil slice.
func (e *Engine) History(limit int) []models.Alert {
	return e.hist.list(limit)
}

// AllHistory returns every retained alert, newest first. Backups use it.
func (e *Engine) AllHistory() []models.Alert {
	return e.hist.list(e.hist.max)
}

func (e *Engine) ClearHistory() {
	e.hist.clear()
}

// activeSub pairs a live hub subscription with the compiled rule and the
// generation stamped into its sink; matches carrying a different generation
// for the same rule ID are stale and discarded.
type activeSub struct {
	rule  *compiledRule
	gen   uint64
	unsub func()
}

// runState is the mutable state owned exclusively by the run goroutine.
type runState struct {
	ctx context.Context
	gen uint64

	subs map[string]*activeSub // live log-rule subscriptions by rule ID
	// eventRules holds every rule that is not a log rule: event, collection
	// and resource rules.
	eventRules []*compiledRule
	windows    map[string]*ruleWindow // by ruleID|host|containerName
	incidents  map[string]*incident   // collection failures by ruleID|host
	resources  map[string]*incident   // resource rule state by ruleID|host|containerName
	loops      map[string]*incident   // fired restart loops by ruleID|host|containerName
	lastOOM    map[string]time.Time   // last oom event by host|containerID
	stopping   map[string]time.Time   // last kill or restart event by host|containerID
	lifecycle  map[string]time.Time   // last die or start event by host|containerID
	// crashed holds containers whose last die was not a stop, by
	// host|containerID; their next start is a restart.
	crashed map[string]bool
	// sampling is set while a resource-rule stats sample is in flight.
	sampling bool

	events       <-chan docker.EngineEvent
	eventsCancel context.CancelFunc
	eventsClient eventClient
}

// run is the engine's run loop: the single owner of all rule and window
// state.
func (e *Engine) run(ctx context.Context) {
	st := &runState{
		ctx:       ctx,
		subs:      make(map[string]*activeSub),
		windows:   make(map[string]*ruleWindow),
		incidents: make(map[string]*incident),
		resources: make(map[string]*incident),
		loops:     make(map[string]*incident),
		lastOOM:   make(map[string]time.Time),
		stopping:  make(map[string]time.Time),
		lifecycle: make(map[string]time.Time),
		crashed:   make(map[string]bool),
	}
	e.openEvents(st, e.source())
	e.reconcile(st)

	ticker := time.NewTicker(e.resyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.shutdownRun(st)
			return
		case <-e.reconcileCh:
			e.checkEventClient(st)
			e.reconcile(st)
		case <-ticker.C:
			e.checkEventClient(st)
			e.pruneWindows(st)
			e.logDrops()
			e.checkCollection(st)
			e.settleRestarts(st)
			e.sampleResources(st)
		case s := <-e.statsCh:
			st.sampling = false
			e.checkResources(st, s)
		case m := <-e.firedCh:
			e.handleMatch(st, m)
		case ev, ok := <-st.events:
			if !ok {
				// All host watchers stopped (e.g. the client was closed after
				// a hot swap). Reopen on the current client.
				st.events = nil
				e.checkEventClient(st)
				continue
			}
			e.handleEvent(st, ev)
		case r := <-e.inspectCh:
			e.handleInspect(st, r)
		}
	}
}

// openEvents (re)opens the engine event stream on client under a child
// context of the run context.
func (e *Engine) openEvents(st *runState, client eventClient) {
	ctx, cancel := context.WithCancel(st.ctx)
	st.eventsCancel = cancel
	st.events = client.streamEvents(ctx)
	st.eventsClient = client
}

// checkEventClient reopens the event stream when the provider's client set
// was hot-swapped (or the stream closed), so watches never stay bound to a
// closed client.
func (e *Engine) checkEventClient(st *runState) {
	if current := e.source(); st.events == nil || current != st.eventsClient {
		st.eventsCancel()
		e.openEvents(st, current)
	}
}

// reconcile recompiles the rule set and diffs log-rule subscriptions:
// unchanged rules keep their subscription (and generation); removed or
// changed rules are unsubscribed and, if still present, resubscribed with a
// fresh compiled snapshot. Window state for removed or changed rules is
// dropped.
func (e *Engine) reconcile(st *runState) {
	compiled := compileRules(e.alertsFn())

	newLog := make(map[string]*compiledRule)
	var newEvent []*compiledRule
	for _, r := range compiled {
		if r.typ == "log" {
			newLog[r.id] = r
		} else {
			newEvent = append(newEvent, r)
		}
	}

	stale := make(map[string]bool)

	for id, sub := range st.subs {
		if nr, ok := newLog[id]; ok && reflect.DeepEqual(nr.src, sub.rule.src) {
			delete(newLog, id) // unchanged: keep the existing subscription
			continue
		}
		sub.unsub()
		delete(st.subs, id)
		stale[id] = true
	}
	for _, r := range newLog {
		e.subscribeRule(st, r)
	}

	oldEvent := make(map[string]config.AlertRule, len(st.eventRules))
	for _, r := range st.eventRules {
		oldEvent[r.id] = r.src
	}
	for _, r := range newEvent {
		if old, ok := oldEvent[r.id]; !ok || !reflect.DeepEqual(old, r.src) {
			stale[r.id] = true
		}
		delete(oldEvent, r.id)
	}
	for id := range oldEvent {
		stale[id] = true
	}
	st.eventRules = newEvent

	if len(stale) > 0 {
		for key := range st.windows {
			if id, _, _ := strings.Cut(key, "|"); stale[id] {
				delete(st.windows, key)
			}
		}
		for _, m := range []map[string]*incident{st.incidents, st.resources, st.loops} {
			for key := range m {
				if id, _, _ := strings.Cut(key, "|"); stale[id] {
					delete(m, key)
				}
			}
		}
	}
}

// incident is the state of a condition that sends a recovery when it clears:
// a collection rule's failure on one host ("" for the log store), a resource
// rule's container, or a fired restart loop. It outlives the condition until
// the cooldown has passed, so a flapping host or container does not notify on
// every flap.
type incident struct {
	rule          *compiledRule
	host          string
	containerID   string
	containerName string
	project       string
	reason        string    // what the last fire reported
	since         time.Time // resource rules: first sample over the limit, zero when under
	fired         bool      // a failure was sent and its recovery has not been
	lastFired     time.Time
}

// checkCollection fires a collection rule for every host (and the log store)
// that has been failing for at least the rule's window, and sends a recovery
// once a fired failure clears. Failures come one per host, so a host outage is
// one alert however many containers it takes down. It runs on the resync
// tick, which bounds how late a failure or recovery is noticed.
func (e *Engine) checkCollection(st *runState) {
	if e.health == nil {
		return
	}
	failures := e.health.Failures()
	now := e.now()

	failing := make(map[string]bool)
	for _, rule := range st.eventRules {
		if rule.typ != "collection" {
			continue
		}
		for _, f := range failures {
			// The log store (Host "") serves every host, so every collection
			// rule watches it.
			if f.Host != "" && !rule.spec.Matches(f.Host, "", nil) {
				continue
			}
			key := rule.id + "|" + f.Host
			failing[key] = true
			inc := st.incidents[key]
			if inc == nil {
				inc = &incident{rule: rule, host: f.Host}
				st.incidents[key] = inc
			}
			if inc.fired || now.Sub(f.Since) < rule.window {
				continue
			}
			if !inc.lastFired.IsZero() && now.Sub(inc.lastFired) < rule.cooldown {
				continue
			}
			inc.fired, inc.lastFired, inc.reason = true, now, f.Reason
			e.emit(rule, f.Host, "", "", "", f.Reason, "", 0)
		}
	}

	for key, inc := range st.incidents {
		if failing[key] {
			continue
		}
		if inc.fired {
			inc.fired = false
			e.emitRecovery(inc.rule, inc.host, "", "", "", "collection recovered", inc.reason)
		}
		if now.Sub(inc.lastFired) >= inc.rule.cooldown {
			delete(st.incidents, key)
		}
	}
}

// sampleResources reads stats for the containers that enabled resource rules
// target, off the run loop, and posts the sample back on statsCh. With no
// enabled resource rule nothing is read; a sample still in flight skips the
// tick.
func (e *Engine) sampleResources(st *runState) {
	var rules []*compiledRule
	for _, rule := range st.eventRules {
		if rule.typ == "resource" {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 || st.sampling {
		return
	}
	st.sampling = true
	client := st.eventsClient
	runCtx := st.ctx
	at := e.now()
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ctx, cancel := context.WithTimeout(runCtx, sampleTimeout)
		defer cancel()
		containers, listed := client.sampleStats(ctx, rules)
		select {
		case e.statsCh <- statsSample{at: at, containers: containers, listed: listed}:
		case <-runCtx.Done():
		}
	}()
}

// checkResources fires a resource rule for every container whose usage has
// stayed over the limit for the rule's window, and sends a recovery once a
// fired container is back under it or has stopped. A container whose stats
// could not be read, or whose host could not be listed, keeps its state.
func (e *Engine) checkResources(st *runState, s statsSample) {
	seen := make(map[string]bool)
	for _, rule := range st.eventRules {
		if rule.typ != "resource" {
			continue
		}
		for _, c := range s.containers {
			if !rule.spec.Matches(c.host, c.name, c.labels) {
				continue
			}
			key := rule.id + "|" + c.host + "|" + c.name
			seen[key] = true
			if c.stats == nil {
				continue
			}
			value, over := rule.usage(c.stats)
			metric, limit := rule.limit()
			inc := st.resources[key]
			if !over {
				if inc != nil {
					e.settleResource(st, key, inc, s.at, fmt.Sprintf("%s back under %s (now %s)", metric, limit, value))
				}
				continue
			}
			if inc == nil {
				inc = &incident{rule: rule, host: c.host, containerName: c.name, project: docker.ComposeProject(c.labels)}
				st.resources[key] = inc
			}
			inc.containerID = c.id
			if inc.since.IsZero() {
				inc.since = s.at
			}
			if inc.fired || s.at.Sub(inc.since) < rule.window-sampleJitter {
				continue
			}
			if !inc.lastFired.IsZero() && s.at.Sub(inc.lastFired) < rule.cooldown {
				continue
			}
			inc.fired, inc.lastFired = true, s.at
			inc.reason = fmt.Sprintf("%s above %s for %ds (now %s)", metric, limit, int(rule.window.Seconds()), value)
			e.emit(rule, c.host, c.id, c.name, inc.project, inc.reason, "", 0)
		}
	}

	for key, inc := range st.resources {
		if !seen[key] && s.listed[inc.host] {
			e.settleResource(st, key, inc, s.at, "container stopped")
		}
	}
}

// settleResource records that a resource rule's container is no longer over
// the limit: a fired incident sends its recovery, and the state is dropped
// once the cooldown since the last fire has passed.
func (e *Engine) settleResource(st *runState, key string, inc *incident, now time.Time, reason string) {
	inc.since = time.Time{}
	if inc.fired {
		inc.fired = false
		e.emitRecovery(inc.rule, inc.host, inc.containerID, inc.containerName, inc.project, reason, inc.reason)
	}
	if now.Sub(inc.lastFired) >= inc.rule.cooldown {
		delete(st.resources, key)
	}
}

// settleRestarts sends a recovery for every fired restart loop whose
// container has since gone a full window without restarting.
func (e *Engine) settleRestarts(st *runState) {
	now := e.now()
	for key, inc := range st.loops {
		if w := st.windows[key]; w != nil && w.idleSince(now) < inc.rule.window {
			continue
		}
		delete(st.loops, key)
		e.emitRecovery(inc.rule, inc.host, inc.containerID, inc.containerName, inc.project,
			fmt.Sprintf("no restart for %ds", int(inc.rule.window.Seconds())), inc.reason)
	}
}

// subscribeRule registers a hub subscription for one log rule. The sink runs
// on the hub's delivery goroutine: it filters against the captured immutable
// rule snapshot and pushes matches non-blockingly; it never touches engine
// state and never blocks.
func (e *Engine) subscribeRule(st *runState, rule *compiledRule) {
	st.gen++
	gen := st.gen
	spec := rule.spec
	// Rules match the original line, so turning redaction on never changes
	// which alerts fire. The sample is stored and sent, so it is redacted.
	opts := models.LogOptions{Timestamps: true, Tail: "0", ShowStdout: true, ShowStderr: true, Unredacted: true}
	sink := func(rec logstream.Record) {
		if !rule.matchesEntry(rec.Entry) {
			return
		}
		m := matchMsg{
			gen:           gen,
			rule:          rule,
			host:          rec.Host,
			containerID:   rec.ContainerID,
			containerName: rec.ContainerName,
			project:       docker.ComposeProject(rec.Labels),
			sample:        recordSample(rec),
		}
		select {
		case e.firedCh <- m:
		default:
			e.matchDrops.Add(1)
		}
	}
	unsub := e.hub.Subscribe(spec, opts, sink)
	st.subs[rule.id] = &activeSub{rule: rule, gen: gen, unsub: unsub}
}

// handleMatch applies rate limiting and cooldown to one log-rule match.
func (e *Engine) handleMatch(st *runState, m matchMsg) {
	sub, ok := st.subs[m.rule.id]
	if !ok || sub.gen != m.gen {
		return // stale: the rule was removed or resubscribed since this match
	}
	key := m.rule.id + "|" + m.host + "|" + m.containerName
	res := e.window(st, key, m.rule).observe(e.now())
	if !res.fire {
		return
	}
	e.emit(m.rule, m.host, m.containerID, m.containerName, m.project, logReason(m.rule), m.sample, res.suppressed)
}

// handleEvent routes one engine event. Only die, oom, and unhealthy health
// transitions can fire rules; the other watched actions exist for the log hub
// and are ignored here.
func (e *Engine) handleEvent(st *runState, ev docker.EngineEvent) {
	base, _, _ := strings.Cut(ev.Action, ": ")
	key := ev.Host + "|" + ev.ContainerID
	switch base {
	case "kill":
		// Docker sends kill before the die of every stop and restart. HUP,
		// USR1 and USR2 are reloads, not stops.
		switch ev.Labels["signal"] {
		case "1", "10", "12":
		default:
			st.stopping[key] = e.now()
		}
	case "restart":
		// Podman sends restart before a user restart's die, but after the die
		// when a restart policy brings a crashed container back. Docker sends
		// it after the start.
		if e.now().Sub(st.lifecycle[key]) > restartSettle {
			st.stopping[key] = e.now()
		}
	case "start":
		st.lifecycle[key] = e.now()
		// A start after a die that was not a stop is a restart: a restart
		// policy, or someone, bringing a crashed container back.
		if st.crashed[key] {
			delete(st.crashed, key)
			e.recordEventMatch(st, ev, "restart", "")
		}
	case "destroy":
		delete(st.stopping, key)
		delete(st.lifecycle, key)
		delete(st.crashed, key)
	case "die":
		st.lifecycle[key] = e.now()
		if at, ok := st.stopping[key]; ok {
			delete(st.stopping, key)
			if e.now().Sub(at) < stopWindow {
				return
			}
		}
		st.crashed[key] = true
		switch ev.ExitCode {
		case "":
			e.spawnInspect(st, ev)
		case "0":
			// Clean exit: not an alert condition.
		default:
			e.spawnStopCheck(st, ev)
		}
	case "oom":
		e.recordEventMatch(st, ev, "oom", "")
	case "health_status":
		// Only the unhealthy transition is an alert; healthy/starting are
		// recoveries and never fire.
		switch ev.HealthStatus {
		case "unhealthy":
			// Docker put the state in the action suffix; use it directly.
			e.recordEventMatch(st, ev, "unhealthy", "")
		case "":
			// Podman emits a bare "health_status" action and carries the state
			// only in a top-level field the SDK drops on decode. Resolve it
			// with a bounded inspect, mirroring the die exit-code fallback.
			e.spawnHealthInspect(st, ev)
		}
	}
}

// spawnInspect looks up the exit code for a die event that arrived without
// one (single inspect, bounded timeout) and posts the result back to the run
// loop. A container that is already gone still fires with exit "unknown";
// die events are never silently skipped on lookup failure.
func (e *Engine) spawnInspect(st *runState, ev docker.EngineEvent) {
	client := st.eventsClient
	runCtx := st.ctx
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ictx, cancel := context.WithTimeout(runCtx, inspectTimeout)
		defer cancel()
		res := inspectResult{ev: ev, action: "die"}
		exitCode, oomKilled, err := client.inspectExit(ictx, ev.Host, ev.ContainerID)
		if err != nil {
			res.exitCode = "unknown"
			res.fire = true
		} else {
			res.exitCode = exitCode
			res.fire = (exitCode != "0" || oomKilled) && !client.stoppedByUser(ictx, ev.Host, ev.ContainerID)
		}
		select {
		case e.inspectCh <- res:
		case <-runCtx.Done():
		}
	}()
}

// spawnStopCheck fires a die unless Podman recorded it as a user stop, which
// it announces with no event beforehand. The event's exit code is kept:
// inspect resets it once a restart policy brings the container back.
func (e *Engine) spawnStopCheck(st *runState, ev docker.EngineEvent) {
	client := st.eventsClient
	runCtx := st.ctx
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ictx, cancel := context.WithTimeout(runCtx, inspectTimeout)
		defer cancel()
		res := inspectResult{ev: ev, action: "die", exitCode: ev.ExitCode}
		res.fire = !client.stoppedByUser(ictx, ev.Host, ev.ContainerID)
		select {
		case e.inspectCh <- res:
		case <-runCtx.Done():
		}
	}()
}

// spawnHealthInspect resolves the health state of a Podman bare-action
// "health_status" event (single inspect, bounded timeout) and posts the result
// back to the run loop. Only a confirmed "unhealthy" fires: unlike a die, an
// unresolvable state is not fired, since firing on an unknown state would be a
// false positive on a container that may well be healthy.
func (e *Engine) spawnHealthInspect(st *runState, ev docker.EngineEvent) {
	client := st.eventsClient
	runCtx := st.ctx
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ictx, cancel := context.WithTimeout(runCtx, inspectTimeout)
		defer cancel()
		status, err := client.inspectHealth(ictx, ev.Host, ev.ContainerID)
		res := inspectResult{ev: ev, action: "unhealthy", fire: err == nil && status == "unhealthy"}
		select {
		case e.inspectCh <- res:
		case <-runCtx.Done():
		}
	}()
}

// handleInspect completes an event whose alert condition needed an inspect (a
// die exit code, or a Podman bare-action health state).
func (e *Engine) handleInspect(st *runState, r inspectResult) {
	if !r.fire {
		return
	}
	e.recordEventMatch(st, r.ev, r.action, r.exitCode)
}

// recordEventMatch runs one die/oom/unhealthy/restart occurrence through every
// matching event rule's window.
func (e *Engine) recordEventMatch(st *runState, ev docker.EngineEvent, action, exitCode string) {
	name := strings.TrimPrefix(ev.ContainerName, "/")
	oomKey := ev.Host + "|" + ev.ContainerID
	if action == "oom" {
		st.lastOOM[oomKey] = e.now()
	}
	for _, rule := range st.eventRules {
		if !rule.hasEvent(action) || !rule.spec.Matches(ev.Host, name, ev.Labels) {
			continue
		}
		// An OOM kill emits "oom" followed by "die" (137): for a rule that
		// watches both actions the pair is one incident, so the die is
		// skipped when this container OOMed just before. Rules watching only
		// die still observe it.
		if action == "die" && rule.hasEvent("oom") {
			if t, ok := st.lastOOM[oomKey]; ok && e.now().Sub(t) <= oomDieWindow {
				continue
			}
		}
		key := rule.id + "|" + ev.Host + "|" + name
		res := e.window(st, key, rule).observe(e.now())
		if !res.fire {
			continue
		}
		reason := eventReason(rule, action, exitCode)
		project := docker.ComposeProject(ev.Labels)
		if action == "restart" {
			st.loops[key] = &incident{rule: rule, host: ev.Host, containerID: ev.ContainerID, containerName: name, project: project, reason: reason}
		}
		e.emit(rule, ev.Host, ev.ContainerID, name, project, reason, eventSample(action, exitCode), res.suppressed)
	}
}

// window returns the rate/cooldown state for key, creating it from the
// rule's normalized parameters on first use.
func (e *Engine) window(st *runState, key string, rule *compiledRule) *ruleWindow {
	w, ok := st.windows[key]
	if !ok {
		w = newRuleWindow(rule.threshold, rule.window, rule.cooldown)
		st.windows[key] = w
	}
	return w
}

// emit builds the alert and hands it to the dispatcher.
func (e *Engine) emit(rule *compiledRule, host, containerID, containerName, project, reason, sample string, suppressed int) {
	e.send(e.newAlert(rule, host, containerID, containerName, project, reason, sample, suppressed))
}

// emitRecovery sends the notice that a fired condition has cleared.
func (e *Engine) emitRecovery(rule *compiledRule, host, containerID, containerName, project, reason, sample string) {
	alert := e.newAlert(rule, host, containerID, containerName, project, reason, sample, 0)
	alert.Recovered = true
	e.send(alert)
}

func (e *Engine) newAlert(rule *compiledRule, host, containerID, containerName, project, reason, sample string, suppressed int) models.Alert {
	return models.Alert{
		ID:            newAlertID(),
		RuleID:        rule.id,
		RuleName:      rule.name,
		Type:          rule.typ,
		Host:          host,
		ContainerID:   containerID,
		ContainerName: containerName,
		Project:       project,
		Reason:        reason,
		Sample:        sample,
		Count:         rule.threshold,
		Suppressed:    suppressed,
		FiredAt:       e.now().UTC().Format(time.RFC3339),
	}
}

// send hands an alert to the dispatcher without blocking the run loop; a
// full dispatch queue drops the alert with a log line.
func (e *Engine) send(alert models.Alert) {
	select {
	case e.dispatchCh <- alert:
	default:
		log.Printf("alerts: dispatch queue full, dropping alert %s (rule %q)", alert.ID, alert.RuleName)
	}
}

// pruneWindows drops rate/cooldown state for keys with no match in
// windowIdleTTL, and oom timestamps too old to suppress a paired die.
func (e *Engine) pruneWindows(st *runState) {
	now := e.now()
	for key, w := range st.windows {
		if w.idleSince(now) > windowIdleTTL {
			delete(st.windows, key)
		}
	}
	for key, t := range st.lastOOM {
		if now.Sub(t) > oomDieWindow {
			delete(st.lastOOM, key)
		}
	}
}

// logDrops reports sink-side match drops accumulated since the last resync.
func (e *Engine) logDrops() {
	if d := e.matchDrops.Load(); d > e.loggedDrops {
		log.Printf("alerts: dropped %d rule matches since last report (evaluation backlog)", d-e.loggedDrops)
		e.loggedDrops = d
	}
}

// shutdownRun tears down run-loop state: unsubscribe all hub subscriptions,
// cancel the event stream (which also aborts pending inspects via the run
// context), discard buffered matches, and close the dispatch channel so the
// dispatcher can drain and finish.
func (e *Engine) shutdownRun(st *runState) {
	for id, sub := range st.subs {
		sub.unsub()
		delete(st.subs, id)
	}
	st.eventsCancel()
	for {
		select {
		case <-e.firedCh:
			// Discarded: window state is in-memory and dies with the process,
			// so partial matches at shutdown cannot fire later anyway.
		case <-e.inspectCh:
		case <-e.statsCh:
		default:
			close(e.dispatchCh)
			return
		}
	}
}

// dispatchLoop consumes fired alerts: each alert is appended to history first
// (Delivery nil) so a hung channel can never lose it, then delivered to every
// enabled channel (read live from config; no enabled channels means
// history-only) and its history entry updated with the summary result. An
// alert an active silence matches is recorded as muted and not delivered. It exits
// when the run loop closes dispatchCh, then stops the history flusher, which
// performs the final synchronous flush. Once the shutdown drain budget has
// expired, remaining alerts are recorded as failed ("shutdown") without a
// delivery attempt.
func (e *Engine) dispatchLoop() {
	defer close(e.flushStop)
	for alert := range e.dispatchCh {
		cfg := e.alertsFn()
		if firedAt, err := time.Parse(time.RFC3339, alert.FiredAt); err == nil && silenced(cfg.Silences, alert, firedAt) {
			alert.Delivery = &models.DeliveryResult{Status: "muted"}
			e.hist.append(alert)
			continue
		}
		e.hist.append(alert)
		channels := enabledChannels(cfg.Channels)
		if len(channels) == 0 {
			continue
		}
		var result models.DeliveryResult
		if e.deliverCtx.Err() != nil {
			result = models.DeliveryResult{Status: "failed", Error: "shutdown"}
		} else {
			ctx, cancel := context.WithTimeout(e.deliverCtx, deliverBudget)
			result = e.notif.deliverAll(ctx, channels, alert, e.skipRetry)
			cancel()
		}
		e.hist.setDelivery(alert.ID, result)
	}
}

// logReason renders the human-readable reason for a fired log rule, e.g.
// "5 matches (level >= ERROR) within 60s".
func logReason(rule *compiledRule) string {
	unit := "matches"
	if rule.threshold == 1 {
		unit = "match"
	}
	var conds []string
	if rule.minLevel != "" {
		conds = append(conds, "level >= "+rule.minLevel)
	}
	if rule.pattern != nil {
		conds = append(conds, fmt.Sprintf("pattern %q", rule.pattern.String()))
	}
	cond := ""
	if len(conds) > 0 {
		cond = " (" + strings.Join(conds, ", ") + ")"
	}
	return fmt.Sprintf("%d %s%s within %ds", rule.threshold, unit, cond, int(rule.window.Seconds()))
}

// eventReason renders the human-readable reason for a fired event rule, e.g.
// "container died (exit 137)".
func eventReason(rule *compiledRule, action, exitCode string) string {
	if rule.threshold <= 1 {
		if action == "oom" {
			return "container OOM-killed"
		}
		if action == "unhealthy" {
			return "container became unhealthy"
		}
		if action == "restart" {
			return "container restarted"
		}
		return fmt.Sprintf("container died (exit %s)", exitCode)
	}
	if action == "oom" {
		return fmt.Sprintf("%d OOM kills within %ds", rule.threshold, int(rule.window.Seconds()))
	}
	if action == "unhealthy" {
		return fmt.Sprintf("%d unhealthy transitions within %ds", rule.threshold, int(rule.window.Seconds()))
	}
	if action == "restart" {
		return fmt.Sprintf("%d restarts within %ds", rule.threshold, int(rule.window.Seconds()))
	}
	return fmt.Sprintf("%d container deaths within %ds (last exit %s)", rule.threshold, int(rule.window.Seconds()), exitCode)
}

func eventSample(action, exitCode string) string {
	if action == "oom" {
		return "oom"
	}
	if action == "unhealthy" || action == "restart" {
		return action
	}
	return "die (exit " + exitCode + ")"
}

// recordSample is the redacted sample of a record from an unredacted
// subscription. The line is parsed again after redaction, so the sample reads
// as every other reader sees the line.
func recordSample(rec logstream.Record) string {
	entry := rec.Entry
	target := redact.Target{Host: rec.Host, Name: rec.ContainerName, Project: docker.ComposeProject(rec.Labels)}
	if rules := redact.Active().For(target); rules != nil {
		entry = models.ParseLogLine(rules.Line(entry.Raw), entry.Stream)
	}
	return entrySample(entry)
}

// entrySample picks the line recorded as the alert's sample.
func entrySample(entry models.LogEntry) string {
	if entry.Message != "" {
		return entry.Message
	}
	return entry.Raw
}

// newAlertID returns an 8-hex-char random alert ID.
func newAlertID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%08x", uint32(time.Now().UnixNano()))
	}
	return hex.EncodeToString(b[:])
}
