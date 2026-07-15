package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// fakeHub records subscriptions and lets tests feed records to live sinks.
type fakeHub struct {
	mu   sync.Mutex
	subs []*fakeHubSub
}

type fakeHubSub struct {
	spec    logstream.ContainerSpec
	opts    models.LogOptions
	sink    func(logstream.Record)
	removed bool
}

func (f *fakeHub) Subscribe(spec logstream.ContainerSpec, opts models.LogOptions, sink func(logstream.Record)) func() {
	s := &fakeHubSub{spec: spec, opts: opts, sink: sink}
	f.mu.Lock()
	f.subs = append(f.subs, s)
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		s.removed = true
		f.mu.Unlock()
	}
}

// emit delivers a record to every live sink, in subscription order, like the
// hub's per-subscription delivery goroutines would.
func (f *fakeHub) emit(rec logstream.Record) {
	f.mu.Lock()
	var sinks []func(logstream.Record)
	for _, s := range f.subs {
		if !s.removed {
			sinks = append(sinks, s.sink)
		}
	}
	f.mu.Unlock()
	for _, sink := range sinks {
		sink(rec)
	}
}

func (f *fakeHub) liveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.subs {
		if !s.removed {
			n++
		}
	}
	return n
}

func (f *fakeHub) firstLive() *fakeHubSub {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.subs {
		if !s.removed {
			return s
		}
	}
	return nil
}

// fakeEvents is a comparable eventClient backed by a test-fed channel. The
// inspect fields are set before events are fed, never mutated concurrently.
type fakeEvents struct {
	ch                  chan docker.EngineEvent
	inspectExitCode     string
	inspectOOM          bool
	inspectHealthStatus string
	inspectErr          error
}

func newFakeEvents() *fakeEvents {
	return &fakeEvents{ch: make(chan docker.EngineEvent, 8)}
}

func (f *fakeEvents) streamEvents(ctx context.Context) <-chan docker.EngineEvent {
	return f.ch
}

func (f *fakeEvents) inspectExit(ctx context.Context, host, containerID string) (string, bool, error) {
	return f.inspectExitCode, f.inspectOOM, f.inspectErr
}

func (f *fakeEvents) inspectHealth(ctx context.Context, host, containerID string) (string, error) {
	return f.inspectHealthStatus, f.inspectErr
}

// fakeConf is a mutable alerts config source.
type fakeConf struct {
	mu  sync.Mutex
	cfg config.AlertsConfig
}

func (f *fakeConf) get() config.AlertsConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg
}

func (f *fakeConf) set(cfg config.AlertsConfig) {
	f.mu.Lock()
	f.cfg = cfg
	f.mu.Unlock()
}

// fakeClock is a manually advanced clock safe for cross-goroutine reads.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type testEngine struct {
	e      *Engine
	hub    *fakeHub
	events *fakeEvents
	conf   *fakeConf
	clock  *fakeClock
	cancel context.CancelFunc
	path   string
}

func startTestEngine(t *testing.T, rules ...config.AlertRule) *testEngine {
	t.Helper()
	te := &testEngine{
		hub:    &fakeHub{},
		events: newFakeEvents(),
		conf:   &fakeConf{cfg: config.AlertsConfig{Rules: rules}},
		clock:  &fakeClock{t: t0},
		path:   filepath.Join(t.TempDir(), "alerts-history.json"),
	}
	te.e = newEngine(te.hub, func() eventClient { return te.events }, te.conf.get, te.path)
	te.e.now = te.clock.now

	ctx, cancel := context.WithCancel(context.Background())
	te.cancel = cancel
	te.e.Start(ctx)
	t.Cleanup(func() {
		cancel()
		te.e.Wait()
	})
	return te
}

func waitFor(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

func record(level models.LogLevel, msg string) logstream.Record {
	return logstream.Record{
		Host:          "local",
		ContainerID:   "abc123",
		ContainerName: "web",
		Entry:         models.LogEntry{Level: level, Message: msg, Raw: msg},
	}
}

func TestLogRuleFiresOnceWithCountAndFields(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "r1", Name: "High error rate", Enabled: true, Type: "log",
		MinLevel: "ERROR", Threshold: 3, WindowSeconds: 60,
	})

	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })
	sub := te.hub.firstLive()
	if sub.opts.Tail != "0" || !sub.opts.ShowStdout || !sub.opts.ShowStderr || !sub.opts.Timestamps {
		t.Fatalf("subscription opts = %+v, want Tail 0 with stdout+stderr+timestamps", sub.opts)
	}

	te.hub.emit(record(models.LogLevelInfo, "just info")) // filtered by MinLevel
	for range 3 {
		te.hub.emit(record(models.LogLevelError, "boom"))
	}

	waitFor(t, "alert in history", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.RuleID != "r1" || a.Type != "log" || a.Host != "local" || a.ContainerName != "web" || a.ContainerID != "abc123" {
		t.Fatalf("alert identity wrong: %+v", a)
	}
	if a.Count != 3 || a.Suppressed != 0 || a.Sample != "boom" {
		t.Fatalf("alert count/sample wrong: %+v", a)
	}
	if !strings.Contains(a.Reason, "3 matches (level >= ERROR) within 60s") {
		t.Fatalf("reason = %q", a.Reason)
	}
	if len(a.ID) != 8 {
		t.Fatalf("alert ID = %q, want 8 hex chars", a.ID)
	}
	if a.Delivery != nil {
		t.Fatalf("no webhook configured but Delivery = %+v, want nil (history-only)", a.Delivery)
	}
}

func TestCooldownSuppressedAccumulatesAndSurfaces(t *testing.T) {
	boom := config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1}
	flush := config.AlertRule{ID: "r2", Name: "flush", Enabled: true, Type: "log", Pattern: "flushmarker", Threshold: 1}
	te := startTestEngine(t, boom, flush)
	waitFor(t, "subscriptions", func() bool { return te.hub.liveCount() == 2 })

	te.hub.emit(record(models.LogLevelError, "boom"))
	waitFor(t, "first alert", func() bool { return len(te.e.History(0)) == 1 })

	// A burst inside the (default 300s) cooldown is suppressed.
	for range 3 {
		te.hub.emit(record(models.LogLevelError, "boom"))
	}
	// The flush rule proves the run loop has consumed the suppressed matches
	// (firedCh is FIFO) before the clock advances.
	te.hub.emit(record(models.LogLevelError, "flushmarker"))
	waitFor(t, "flush alert", func() bool { return len(te.e.History(0)) == 2 })

	te.clock.advance(301 * time.Second)
	te.hub.emit(record(models.LogLevelError, "boom"))
	waitFor(t, "post-cooldown alert", func() bool { return len(te.e.History(0)) == 3 })

	newest := te.e.History(0)[0]
	if newest.RuleID != "r1" || newest.Suppressed != 3 {
		t.Fatalf("post-cooldown alert = %+v, want rule r1 with Suppressed 3", newest)
	}
}

func TestDieEventFires(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "Container died", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1,
	})

	te.events.ch <- docker.EngineEvent{
		Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "137",
	}

	waitFor(t, "die alert", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.Type != "event" || a.RuleID != "e1" || a.Reason != "container died (exit 137)" {
		t.Fatalf("alert = %+v", a)
	}
	if a.Sample != "die (exit 137)" {
		t.Fatalf("sample = %q", a.Sample)
	}
}

func TestOOMEventFires(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "OOM", Enabled: true, Type: "event", Events: []string{"oom"}, Threshold: 1,
	})

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "oom"}

	waitFor(t, "oom alert", func() bool { return len(te.e.History(0)) == 1 })
	if got := te.e.History(0)[0].Reason; got != "container OOM-killed" {
		t.Fatalf("reason = %q", got)
	}
}

// Docker carries the health state in the action suffix, so no inspect is
// needed and the engine fires directly off the event.
func TestUnhealthyEventFiresFromDockerSuffix(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "Health check failing", Enabled: true, Type: "event", Events: []string{"unhealthy"}, Threshold: 1,
	})

	te.events.ch <- docker.EngineEvent{
		Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status: unhealthy", HealthStatus: "unhealthy",
	}

	waitFor(t, "unhealthy alert", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.Type != "event" || a.RuleID != "e1" || a.Reason != "container became unhealthy" {
		t.Fatalf("alert = %+v", a)
	}
	if a.Sample != "unhealthy" {
		t.Fatalf("sample = %q", a.Sample)
	}
}

func TestHealthyAndStartingDockerSuffixDoNotFire(t *testing.T) {
	unhealthy := config.AlertRule{ID: "e1", Name: "unhealthy", Enabled: true, Type: "event", Events: []string{"unhealthy"}, Threshold: 1}
	flush := config.AlertRule{ID: "e2", Name: "flush", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1}
	te := startTestEngine(t, unhealthy, flush)

	// Recovery transitions must not fire the unhealthy rule.
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status: healthy", HealthStatus: "healthy"}
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status: starting", HealthStatus: "starting"}
	// The die event proves the run loop has consumed the health events before
	// we assert; only it may appear in history.
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "1"}

	waitFor(t, "flush die alert", func() bool { return len(te.e.History(0)) == 1 })
	if got := te.e.History(0)[0].RuleID; got != "e2" {
		t.Fatalf("history = %+v, want only the die alert (healthy/starting must not fire)", te.e.History(0))
	}
}

// Podman emits a bare "health_status" action with no state in the message, so
// the engine inspects the container and reads State.Health.Status.
func TestUnhealthyEventFiresViaInspectFallback(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "Health check failing", Enabled: true, Type: "event", Events: []string{"unhealthy"}, Threshold: 1,
	})
	te.events.inspectHealthStatus = "unhealthy"

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status"}

	waitFor(t, "unhealthy alert via inspect", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.RuleID != "e1" || a.Reason != "container became unhealthy" || a.Sample != "unhealthy" {
		t.Fatalf("alert = %+v", a)
	}
}

func TestBareHealthInspectHealthyDoesNotFire(t *testing.T) {
	unhealthy := config.AlertRule{ID: "e1", Name: "unhealthy", Enabled: true, Type: "event", Events: []string{"unhealthy"}, Threshold: 1}
	flush := config.AlertRule{ID: "e2", Name: "flush", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1}
	te := startTestEngine(t, unhealthy, flush)
	te.events.inspectHealthStatus = "healthy" // the bare-action inspect resolves to a recovery

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status"}
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "1"}

	waitFor(t, "flush die alert", func() bool { return len(te.e.History(0)) == 1 })
	time.Sleep(50 * time.Millisecond) // give a wrong unhealthy alert time to appear
	if h := te.e.History(0); len(h) != 1 || h[0].RuleID != "e2" {
		t.Fatalf("history = %+v, want only the die alert (healthy inspect must not fire)", h)
	}
}

func TestBareHealthInspectErrorDoesNotFire(t *testing.T) {
	unhealthy := config.AlertRule{ID: "e1", Name: "unhealthy", Enabled: true, Type: "event", Events: []string{"unhealthy"}, Threshold: 1}
	flush := config.AlertRule{ID: "e2", Name: "flush", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1}
	te := startTestEngine(t, unhealthy, flush)
	te.events.inspectErr = errors.New("no such container") // state cannot be confirmed

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "health_status"}
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "1"}

	waitFor(t, "flush die alert", func() bool { return len(te.e.History(0)) == 1 })
	time.Sleep(50 * time.Millisecond) // give a wrong unhealthy alert time to appear
	if h := te.e.History(0); len(h) != 1 || h[0].RuleID != "e2" {
		t.Fatalf("history = %+v, want only the die alert (unresolvable state must not fire)", h)
	}
}

func TestOOMThenDieCountsOnceForRuleWatchingBoth(t *testing.T) {
	both := config.AlertRule{ID: "both", Name: "both", Enabled: true, Type: "event", Events: []string{"die", "oom"}, Threshold: 2, WindowSeconds: 60}
	dieOnly := config.AlertRule{ID: "die-only", Name: "die only", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1}
	te := startTestEngine(t, both, dieOnly)

	// One OOM kill: the daemon emits "oom" followed by "die" (exit 137).
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "oom"}
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "137"}

	// The die-only rule fires on the die. The both rule must have observed
	// the incident once (the oom), so at threshold 2 it must not fire yet.
	waitFor(t, "die-only alert", func() bool { return len(te.e.History(0)) == 1 })
	time.Sleep(50 * time.Millisecond) // give a wrong double-counted alert time to appear
	h := te.e.History(0)
	if len(h) != 1 || h[0].RuleID != "die-only" {
		t.Fatalf("history = %+v, want only the die-only alert (oom+die pair must count once)", h)
	}

	// A die outside the oom correlation window is a separate incident: the
	// both rule's second observation, so it fires.
	te.clock.advance(11 * time.Second)
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die", ExitCode: "1"}
	waitFor(t, "both-rule alert", func() bool { return len(te.e.History(0)) == 2 })
	if got := te.e.History(0)[0].RuleID; got != "both" {
		t.Fatalf("newest alert rule = %q, want both", got)
	}
}

func TestAlertInHistoryWhileDeliveryHangs(t *testing.T) {
	unblock := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(unblock) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblock // hang until the test releases the webhook
	}))
	defer srv.Close()
	defer release()

	te := startTestEngine(t, config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1})
	te.conf.set(config.AlertsConfig{Channels: webhookChannels(srv.URL), Rules: te.conf.get().Rules})
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })

	te.hub.emit(record(models.LogLevelError, "boom"))

	// The alert must reach history while the webhook is still hanging.
	waitFor(t, "alert in history during delivery", func() bool { return len(te.e.History(0)) == 1 })
	if d := te.e.History(0)[0].Delivery; d != nil {
		t.Fatalf("delivery = %+v, want nil while the webhook hangs", d)
	}

	// Once the webhook responds, the stored entry gains the delivery result.
	release()
	waitFor(t, "delivery recorded", func() bool {
		h := te.e.History(0)
		return len(h) == 1 && h[0].Delivery != nil
	})
	if d := te.e.History(0)[0].Delivery; d.Status != "ok" {
		t.Fatalf("delivery = %+v, want ok", d)
	}
}

func TestDieEventEmptyExitCodeFailingInspectFiresUnknown(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "Container died", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1,
	})
	te.events.inspectErr = errors.New("no such container")

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "web", Action: "die"}

	waitFor(t, "unknown-exit alert", func() bool { return len(te.e.History(0)) == 1 })
	if got := te.e.History(0)[0].Reason; got != "container died (exit unknown)" {
		t.Fatalf("reason = %q", got)
	}
}

func TestDieEventCleanExitViaInspectDoesNotFire(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{
		ID: "e1", Name: "Container died", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1,
	})
	te.events.inspectExitCode = "0"

	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c1", ContainerName: "clean", Action: "die"}
	te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: "c2", ContainerName: "web", Action: "die", ExitCode: "1"}

	waitFor(t, "non-clean alert", func() bool { return len(te.e.History(0)) >= 1 })
	time.Sleep(50 * time.Millisecond) // give a wrong clean-exit alert time to appear
	alerts := te.e.History(0)
	if len(alerts) != 1 || alerts[0].ContainerName != "web" {
		t.Fatalf("alerts = %+v, want only the exit-1 container", alerts)
	}
}

func TestReconcileUnsubscribesDisabledRule(t *testing.T) {
	rule := config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1}
	te := startTestEngine(t, rule)
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })

	rule.Enabled = false
	te.conf.set(config.AlertsConfig{Rules: []config.AlertRule{rule}})
	te.e.Reconcile()

	waitFor(t, "unsubscribe", func() bool { return te.hub.liveCount() == 0 })
}

func TestStaleGenerationMatchDiscarded(t *testing.T) {
	r1 := config.AlertRule{ID: "r1", Name: "old", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1}
	te := startTestEngine(t, r1)
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })
	oldSink := te.hub.firstLive().sink

	r2 := config.AlertRule{ID: "r2", Name: "new", Enabled: true, Type: "log", Pattern: "second", Threshold: 1}
	te.conf.set(config.AlertsConfig{Rules: []config.AlertRule{r2}})
	te.e.Reconcile()
	waitFor(t, "resubscribe", func() bool {
		te.hub.mu.Lock()
		defer te.hub.mu.Unlock()
		return len(te.hub.subs) == 2 && te.hub.subs[0].removed && !te.hub.subs[1].removed
	})

	// Simulate a record that was already in flight on the old delivery
	// goroutine: its match carries the old generation and must be discarded.
	oldSink(record(models.LogLevelError, "boom"))
	// FIFO on firedCh: once the r2 alert lands, the stale match has been
	// processed (and discarded) too.
	te.hub.emit(record(models.LogLevelError, "second"))

	waitFor(t, "new-rule alert", func() bool { return len(te.e.History(0)) == 1 })
	if got := te.e.History(0)[0].RuleID; got != "r2" {
		t.Fatalf("alert rule = %q, want r2 (stale r1 match must be discarded)", got)
	}
}

func TestEngineDeliversToWebhook(t *testing.T) {
	var mu sync.Mutex
	var payloads []webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p webhookPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		payloads = append(payloads, p)
		mu.Unlock()
	}))
	defer srv.Close()

	te := startTestEngine(t, config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1})
	te.conf.set(config.AlertsConfig{Channels: webhookChannels(srv.URL), Rules: te.conf.get().Rules})
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })

	te.hub.emit(record(models.LogLevelError, "boom"))

	waitFor(t, "delivered alert", func() bool {
		h := te.e.History(0)
		return len(h) == 1 && h[0].Delivery != nil
	})
	if d := te.e.History(0)[0].Delivery; d.Status != "ok" || d.HTTPStatus != http.StatusOK {
		t.Fatalf("delivery = %+v, want ok/200", d)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 1 || payloads[0].Alert.RuleID != "r1" {
		t.Fatalf("webhook payloads = %+v", payloads)
	}
}

// webhookChannels returns a one-element enabled webhook channel list for url.
func webhookChannels(url string) []config.AlertChannel {
	return []config.AlertChannel{{ID: "c1", Type: "webhook", Enabled: true, URL: url}}
}

func TestTestChannelDelivery(t *testing.T) {
	te := startTestEngine(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	res := te.e.TestChannel(context.Background(), webhookChannels(srv.URL)[0])
	if res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if h := te.e.History(0); len(h) != 0 {
		t.Fatalf("test channel must not be recorded in history, got %+v", h)
	}
}

func TestDispatchDisabledChannelSkipped(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	te := startTestEngine(t, config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1})
	te.conf.set(config.AlertsConfig{
		Channels: []config.AlertChannel{{ID: "c1", Type: "webhook", Enabled: false, URL: srv.URL}},
		Rules:    te.conf.get().Rules,
	})
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })

	te.hub.emit(record(models.LogLevelError, "boom"))

	// The alert is recorded (history-only) but never delivered to the disabled
	// channel, so its delivery stays nil.
	waitFor(t, "alert in history", func() bool { return len(te.e.History(0)) == 1 })
	time.Sleep(50 * time.Millisecond)
	if d := te.e.History(0)[0].Delivery; d != nil {
		t.Fatalf("delivery = %+v, want nil (channel disabled)", d)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("disabled channel received %d requests, want 0", got)
	}
}

func TestShutdownIsPromptAndFlushesHistory(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1})
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })

	te.hub.emit(record(models.LogLevelError, "boom"))
	waitFor(t, "alert", func() bool { return len(te.e.History(0)) == 1 })

	te.cancel()
	done := make(chan struct{})
	go func() {
		te.e.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return promptly after cancel")
	}

	if te.hub.liveCount() != 0 {
		t.Fatal("hub subscriptions not removed on shutdown")
	}
	data, err := os.ReadFile(te.path)
	if err != nil {
		t.Fatalf("history not flushed on shutdown: %v", err)
	}
	var onDisk []models.Alert
	if err := json.Unmarshal(data, &onDisk); err != nil || len(onDisk) != 1 {
		t.Fatalf("flushed history = %q (err %v), want 1 entry", data, err)
	}
}

func TestClearHistoryThroughEngine(t *testing.T) {
	te := startTestEngine(t, config.AlertRule{ID: "r1", Name: "boom", Enabled: true, Type: "log", Pattern: "boom", Threshold: 1})
	waitFor(t, "subscription", func() bool { return te.hub.liveCount() == 1 })
	te.hub.emit(record(models.LogLevelError, "boom"))
	waitFor(t, "alert", func() bool { return len(te.e.History(0)) == 1 })

	te.e.ClearHistory()
	if h := te.e.History(0); h == nil || len(h) != 0 {
		t.Fatalf("after ClearHistory History = %#v, want non-nil empty", h)
	}
}
