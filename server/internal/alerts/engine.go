// Package alerts contains the alerting engine: it watches container events
// and log streams, matches them against the configured rules, and delivers
// webhook notifications.
//
// Concurrency model: a single run goroutine owns all mutable state (compiled
// rules, subscription handles, rate/cooldown windows, the event-stream child
// context). Log-rule sinks run on the hub's delivery goroutines and only push
// non-blocking match messages onto firedCh. A single dispatcher goroutine
// consumes fired alerts, delivers webhooks, and appends to history. Helper
// goroutines exist only for single container-inspect exit-code lookups.
package alerts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	firedBuffer    = 1024
	inspectBuffer  = 64
	dispatchBuffer = 128

	defaultResyncInterval = 60 * time.Second
	inspectTimeout        = 5 * time.Second
	// deliverBudget bounds one delivery cycle (attempt + retry wait +
	// attempt) so shutdown cannot hang on an unresponsive webhook.
	deliverBudget = 30 * time.Second
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

// matchMsg is one log-rule match pushed from a sink to the run loop.
type matchMsg struct {
	gen           uint64
	rule          *compiledRule
	host          string
	containerID   string
	containerName string
	sample        string
}

// inspectResult is the outcome of a die-event exit-code lookup posted back to
// the run loop.
type inspectResult struct {
	ev       docker.EngineEvent
	exitCode string
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
	dispatchCh  chan models.Alert
	flushStop   chan struct{}

	// deliverCtx survives Start-context cancellation so in-flight deliveries
	// can finish during shutdown; skipRetry aborts retry waits once shutdown
	// begins. Both are set in Start before any goroutine reads them.
	deliverCtx context.Context
	skipRetry  <-chan struct{}

	resyncInterval time.Duration
	now            func() time.Time
	wg             sync.WaitGroup

	matchDrops  atomic.Uint64
	loggedDrops uint64 // owned by the run loop
}

// NewEngine creates an alerting engine that reads rules through manager,
// watches events through the provider's Docker clients, and subscribes to
// log records through hub.
func NewEngine(provider DockerProvider, manager *config.Manager, hub *logstream.Hub) *Engine {
	alertsFn := func() config.AlertsConfig {
		fc := manager.FileConfigSnapshot()
		if fc.Alerts == nil {
			return config.AlertsConfig{}
		}
		return *fc.Alerts
	}
	historyPath := filepath.Join(filepath.Dir(manager.ConfigFilePath()), "alerts-history.json")
	return newEngine(hub, func() eventClient { return dockerEventAdapter{c: provider.Docker()} }, alertsFn, historyPath)
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
	e.deliverCtx = context.WithoutCancel(ctx)
	e.skipRetry = ctx.Done()
	e.wg.Add(3)
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

// TestWebhook sends a test notification to the configured webhook URL and
// reports the delivery outcome. The test alert is not recorded in history.
func (e *Engine) TestWebhook(ctx context.Context) models.DeliveryResult {
	url := e.alertsFn().WebhookURL
	if url == "" {
		return models.DeliveryResult{Status: "failed", Error: "no webhook configured"}
	}
	alert := models.Alert{
		ID:       newAlertID(),
		RuleName: "Test webhook",
		Type:     "test",
		Reason:   "test notification from LogDeck",
		Count:    1,
		FiredAt:  e.now().UTC().Format(time.RFC3339),
	}
	return e.notif.deliver(ctx, url, alert, ctx.Done())
}

// History returns the most recent fired alerts, newest first, capped at
// limit (limit <= 0 falls back to a default). Always returns a non-nil slice.
func (e *Engine) History(limit int) []models.Alert {
	return e.hist.list(limit)
}

// ClearHistory removes all recorded alert history entries.
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

	subs       map[string]*activeSub // live log-rule subscriptions by rule ID
	eventRules []*compiledRule
	windows    map[string]*ruleWindow // by ruleID|host|containerName

	events       <-chan docker.EngineEvent
	eventsCancel context.CancelFunc
	eventsClient eventClient
}

// run is the engine's run loop: the single owner of all rule and window
// state.
func (e *Engine) run(ctx context.Context) {
	st := &runState{
		ctx:     ctx,
		subs:    make(map[string]*activeSub),
		windows: make(map[string]*ruleWindow),
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
	}
}

// subscribeRule registers a hub subscription for one log rule. The sink runs
// on the hub's delivery goroutine: it filters against the captured immutable
// rule snapshot and pushes matches non-blockingly; it never touches engine
// state and never blocks.
func (e *Engine) subscribeRule(st *runState, rule *compiledRule) {
	st.gen++
	gen := st.gen
	spec := logstream.ContainerSpec{Hosts: rule.hosts, Containers: rule.containers, Projects: rule.projects}
	opts := models.LogOptions{Timestamps: true, Tail: "0", ShowStdout: true, ShowStderr: true}
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
			sample:        entrySample(rec.Entry),
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
	e.emit(m.rule, m.host, m.containerID, m.containerName, logReason(m.rule), m.sample, res.suppressed)
}

// handleEvent routes one engine event. Only die and oom can fire rules; the
// other watched actions exist for the log hub and are ignored here.
func (e *Engine) handleEvent(st *runState, ev docker.EngineEvent) {
	base, _, _ := strings.Cut(ev.Action, ": ")
	switch base {
	case "die":
		switch ev.ExitCode {
		case "":
			e.spawnInspect(st, ev)
		case "0":
			// Clean exit: not an alert condition.
		default:
			e.recordEventMatch(st, ev, "die", ev.ExitCode)
		}
	case "oom":
		e.recordEventMatch(st, ev, "oom", "")
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
		res := inspectResult{ev: ev}
		exitCode, oomKilled, err := client.inspectExit(ictx, ev.Host, ev.ContainerID)
		if err != nil {
			res.exitCode = "unknown"
			res.fire = true
		} else {
			res.exitCode = exitCode
			res.fire = exitCode != "0" || oomKilled
		}
		select {
		case e.inspectCh <- res:
		case <-runCtx.Done():
		}
	}()
}

// handleInspect completes a die event whose exit code needed an inspect.
func (e *Engine) handleInspect(st *runState, r inspectResult) {
	if !r.fire {
		return
	}
	e.recordEventMatch(st, r.ev, "die", r.exitCode)
}

// recordEventMatch runs one die/oom occurrence through every matching event
// rule's window.
func (e *Engine) recordEventMatch(st *runState, ev docker.EngineEvent, action, exitCode string) {
	name := strings.TrimPrefix(ev.ContainerName, "/")
	for _, rule := range st.eventRules {
		if !rule.hasEvent(action) || !rule.matchesTarget(ev.Host, name, ev.Labels) {
			continue
		}
		key := rule.id + "|" + ev.Host + "|" + name
		res := e.window(st, key, rule).observe(e.now())
		if !res.fire {
			continue
		}
		e.emit(rule, ev.Host, ev.ContainerID, name, eventReason(rule, action, exitCode), eventSample(action, exitCode), res.suppressed)
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

// emit builds the alert and hands it to the dispatcher without blocking the
// run loop; a full dispatch queue drops the alert with a log line.
func (e *Engine) emit(rule *compiledRule, host, containerID, containerName, reason, sample string, suppressed int) {
	alert := models.Alert{
		ID:            newAlertID(),
		RuleID:        rule.id,
		RuleName:      rule.name,
		Type:          rule.typ,
		Host:          host,
		ContainerID:   containerID,
		ContainerName: containerName,
		Reason:        reason,
		Sample:        sample,
		Count:         rule.threshold,
		Suppressed:    suppressed,
		FiredAt:       e.now().UTC().Format(time.RFC3339),
	}
	select {
	case e.dispatchCh <- alert:
	default:
		log.Printf("alerts: dispatch queue full, dropping alert %s (rule %q)", alert.ID, rule.name)
	}
}

// pruneWindows drops rate/cooldown state for keys with no match in
// windowIdleTTL.
func (e *Engine) pruneWindows(st *runState) {
	now := e.now()
	for key, w := range st.windows {
		if w.idleSince(now) > windowIdleTTL {
			delete(st.windows, key)
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
		default:
			close(e.dispatchCh)
			return
		}
	}
}

// dispatchLoop consumes fired alerts: it delivers to the webhook (read live
// from config; empty means history-only) and appends the result to history.
// It exits when the run loop closes dispatchCh, then stops the history
// flusher, which performs the final synchronous flush.
func (e *Engine) dispatchLoop() {
	defer close(e.flushStop)
	for alert := range e.dispatchCh {
		if url := e.alertsFn().WebhookURL; url != "" {
			ctx, cancel := context.WithTimeout(e.deliverCtx, deliverBudget)
			result := e.notif.deliver(ctx, url, alert, e.skipRetry)
			cancel()
			alert.Delivery = &result
		}
		e.hist.append(alert)
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
		return fmt.Sprintf("container died (exit %s)", exitCode)
	}
	if action == "oom" {
		return fmt.Sprintf("%d OOM kills within %ds", rule.threshold, int(rule.window.Seconds()))
	}
	return fmt.Sprintf("%d container deaths within %ds (last exit %s)", rule.threshold, int(rule.window.Seconds()), exitCode)
}

// eventSample renders the sample detail for an event alert.
func eventSample(action, exitCode string) string {
	if action == "oom" {
		return "oom"
	}
	return "die (exit " + exitCode + ")"
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
