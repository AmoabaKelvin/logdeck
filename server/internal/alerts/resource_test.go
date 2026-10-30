package alerts

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// resourceEngine builds an engine whose resource check is driven by hand
// with samples the test builds.
func resourceEngine(t *testing.T, rules ...config.AlertRule) (*Engine, *runState) {
	t.Helper()
	cfg := config.AlertsConfig{Rules: rules}
	e := newEngine(&fakeHub{}, func() eventClient { return newFakeEvents() },
		func() config.AlertsConfig { return cfg }, filepath.Join(t.TempDir(), "alerts-history.json"))
	st := &runState{eventRules: compileRules(cfg), resources: make(map[string]*incident)}
	return e, st
}

func memSample(at time.Time, percent float64) statsSample {
	return statsSample{
		at:         at,
		containers: []sampledContainer{{host: "local", id: "c1", name: "web", stats: &models.ContainerStats{MemoryPercent: percent}}},
		listed:     map[string]bool{"local": true},
	}
}

func TestResourceRuleFiresWhenSustainedAndRecovers(t *testing.T) {
	e, st := resourceEngine(t, config.AlertRule{
		ID: "m1", Name: "Memory", Enabled: true, Type: "resource",
		Metric: "memory", AbovePercent: 90, WindowSeconds: 300, CooldownSeconds: 600,
	})

	for i := range 5 {
		e.checkResources(st, memSample(t0.Add(time.Duration(i)*time.Minute), 95))
	}
	if alerts := drainAlerts(e); len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none before the window", alerts)
	}

	e.checkResources(st, memSample(t0.Add(5*time.Minute), 95))
	e.checkResources(st, memSample(t0.Add(6*time.Minute), 96))
	alerts := drainAlerts(e)
	if len(alerts) != 1 || alerts[0].ContainerID != "c1" || alerts[0].Recovered ||
		alerts[0].Reason != "memory above 90% for 300s (now 95%)" {
		t.Fatalf("alerts = %+v, want one sustained alert", alerts)
	}

	e.checkResources(st, memSample(t0.Add(7*time.Minute), 50))
	alerts = drainAlerts(e)
	if len(alerts) != 1 || !alerts[0].Recovered || alerts[0].Reason != "memory back under 90% (now 50%)" ||
		alerts[0].Sample != "memory above 90% for 300s (now 95%)" {
		t.Fatalf("alerts = %+v, want one recovery", alerts)
	}

	// Over again for a full window, but inside the cooldown of the last fire:
	// held back until the cooldown has passed.
	for i := 8; i <= 14; i++ {
		e.checkResources(st, memSample(t0.Add(time.Duration(i)*time.Minute), 95))
	}
	if alerts := drainAlerts(e); len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none inside the cooldown", alerts)
	}
	e.checkResources(st, memSample(t0.Add(15*time.Minute), 95))
	if alerts := drainAlerts(e); len(alerts) != 1 || alerts[0].Recovered {
		t.Fatalf("alerts = %+v, want the alert once the cooldown passed", alerts)
	}
}

func TestResourceRuleStateAcrossMissedReadsAndStops(t *testing.T) {
	e, st := resourceEngine(t, config.AlertRule{
		ID: "c1", Name: "CPU", Enabled: true, Type: "resource",
		Containers: []string{"web"}, Metric: "cpu", AbovePercent: 150, WindowSeconds: 60,
	})
	sample := func(at time.Duration, stats *models.ContainerStats, listed bool, running ...string) statsSample {
		s := statsSample{at: t0.Add(at), listed: map[string]bool{"local": listed}}
		for _, name := range running {
			s.containers = append(s.containers, sampledContainer{host: "local", id: name, name: name, stats: stats})
		}
		return s
	}
	busy := &models.ContainerStats{CPUPercent: 200}

	e.checkResources(st, sample(0, busy, true, "web", "worker"))
	// Ticks drift: a sample a little short of the window still counts.
	e.checkResources(st, sample(59*time.Second, busy, true, "web", "worker"))
	alerts := drainAlerts(e)
	if len(alerts) != 1 || alerts[0].ContainerName != "web" || alerts[0].Reason != "CPU above 150% for 60s (now 200%)" {
		t.Fatalf("alerts = %+v, want one alert for the targeted container", alerts)
	}

	// A failed stats read and an unreachable host say nothing about the
	// container, so neither recovers it.
	e.checkResources(st, sample(2*time.Minute, nil, true, "web"))
	e.checkResources(st, sample(3*time.Minute, nil, false))
	if alerts := drainAlerts(e); len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none while the container is unread", alerts)
	}

	e.checkResources(st, sample(4*time.Minute, nil, true))
	alerts = drainAlerts(e)
	if len(alerts) != 1 || !alerts[0].Recovered || alerts[0].Reason != "container stopped" {
		t.Fatalf("alerts = %+v, want a recovery once the container stopped", alerts)
	}
}

func TestResourceUsageInBytes(t *testing.T) {
	rule := compileOne(t, config.AlertRule{ID: "m", Type: "resource", Metric: "memory", AboveBytes: 512 << 20})
	value, over := rule.usage(&models.ContainerStats{MemoryUsed: 768 << 20, MemoryPercent: 10})
	if !over || value != "768.0 MiB" {
		t.Fatalf("usage = %q, %v; want 768.0 MiB over", value, over)
	}
	if _, limit := rule.limit(); limit != "512.0 MiB" {
		t.Fatalf("limit = %q", limit)
	}
}

func TestRestartLoopFiresAndSettles(t *testing.T) {
	te := &testEngine{
		hub:    &fakeHub{},
		events: newFakeEvents(),
		conf: &fakeConf{cfg: config.AlertsConfig{Rules: []config.AlertRule{{
			ID: "r1", Name: "Restart loop", Enabled: true, Type: "event",
			Events: []string{"restart"}, Threshold: 3, WindowSeconds: 600,
		}}}},
		clock: &fakeClock{t: t0},
	}
	te.e = newEngine(te.hub, func() eventClient { return te.events }, te.conf.get, filepath.Join(t.TempDir(), "alerts-history.json"))
	te.e.now = te.clock.now
	te.e.resyncInterval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	te.e.Start(ctx)
	t.Cleanup(func() {
		cancel()
		te.e.Wait()
	})
	ev := func(id, action, exit string) {
		te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: id, ContainerName: id, Action: action, ExitCode: exit}
	}

	// docker restart and docker stop/start are not restarts of a crashed
	// container, and a fresh container's first start is not a restart.
	for range 3 {
		ev("user", "kill", "")
		ev("user", "die", "143")
		ev("user", "start", "")
	}
	ev("fresh", "start", "")
	for range 3 {
		ev("loop", "die", "1")
		ev("loop", "start", "")
	}

	waitFor(t, "restart loop alert", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.ContainerName != "loop" || a.Reason != "3 restarts within 600s" || a.Sample != "restart" || a.Recovered {
		t.Fatalf("alert = %+v, want the crash loop", a)
	}

	te.clock.advance(601 * time.Second)
	waitFor(t, "settled notice", func() bool { return len(te.e.History(0)) == 2 })
	a = te.e.History(0)[0]
	if !a.Recovered || a.ContainerName != "loop" || a.Reason != "no restart for 600s" || a.Sample != "3 restarts within 600s" {
		t.Fatalf("recovery = %+v", a)
	}
}
