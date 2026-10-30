package alerts

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/health"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// collectionEngine builds an engine whose collection check is driven by hand.
// The tracker stamps real time, so the engine clock is real time plus an
// offset the test moves forward.
func collectionEngine(t *testing.T, rules ...config.AlertRule) (*Engine, *runState, *health.Tracker, *time.Duration) {
	t.Helper()
	cfg := config.AlertsConfig{Rules: rules}
	e := newEngine(&fakeHub{}, func() eventClient { return newFakeEvents() },
		func() config.AlertsConfig { return cfg }, filepath.Join(t.TempDir(), "alerts-history.json"))
	e.health = health.NewTracker()
	offset := new(time.Duration)
	e.now = func() time.Time { return time.Now().Add(*offset) }
	st := &runState{eventRules: compileRules(cfg), incidents: make(map[string]*incident)}
	return e, st, e.health, offset
}

func drainAlerts(e *Engine) []models.Alert {
	var out []models.Alert
	for {
		select {
		case a := <-e.dispatchCh:
			out = append(out, a)
		default:
			return out
		}
	}
}

func TestCollectionRuleFiresOncePerHostAndRecovers(t *testing.T) {
	e, st, tracker, offset := collectionEngine(t, config.AlertRule{
		ID: "c1", Name: "Collection", Enabled: true, Type: "collection",
		WindowSeconds: 120, CooldownSeconds: 600,
	})
	down := []docker.HostError{{HostName: "remote", Err: errors.New("dial tcp: connection refused")}}

	tracker.ObserveListing(map[string][]models.ContainerInfo{"remote": {
		{ID: "a", State: "running"}, {ID: "b", State: "running"},
	}}, nil)
	tracker.TailFailed("remote", "a", errors.New("EOF"))
	tracker.TailFailed("remote", "b", errors.New("EOF"))
	tracker.ObserveListing(nil, down)

	e.checkCollection(st)
	if alerts := drainAlerts(e); len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none before the delay", alerts)
	}

	*offset = 3 * time.Minute
	e.checkCollection(st)
	e.checkCollection(st)
	alerts := drainAlerts(e)
	if len(alerts) != 1 || alerts[0].Host != "remote" || alerts[0].Type != "collection" ||
		!strings.Contains(alerts[0].Reason, "cannot list containers") {
		t.Fatalf("alerts = %+v, want one host alert", alerts)
	}

	tracker.ObserveListing(map[string][]models.ContainerInfo{"remote": {}}, nil)
	e.checkCollection(st)
	alerts = drainAlerts(e)
	if len(alerts) != 1 || alerts[0].Reason != "collection recovered" ||
		!strings.Contains(alerts[0].Sample, "cannot list containers") {
		t.Fatalf("alerts = %+v, want one recovery", alerts)
	}

	// Down again inside the cooldown: held back until the cooldown has passed.
	tracker.ObserveListing(nil, down)
	*offset = 6 * time.Minute
	e.checkCollection(st)
	if alerts := drainAlerts(e); len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none inside the cooldown", alerts)
	}
	*offset = 14 * time.Minute
	e.checkCollection(st)
	if alerts := drainAlerts(e); len(alerts) != 1 {
		t.Fatalf("alerts = %+v, want the failure once the cooldown passed", alerts)
	}
}

func TestCollectionRuleTargetsHostsButAlwaysWatchesStorage(t *testing.T) {
	e, st, tracker, offset := collectionEngine(t, config.AlertRule{
		ID: "c1", Name: "Collection", Enabled: true, Type: "collection",
		Hosts: []string{"local"}, WindowSeconds: 60,
	})
	tracker.ObserveListing(nil, []docker.HostError{{HostName: "remote", Err: errors.New("refused")}})
	tracker.StorageWrite(errors.New("disk I/O error"))

	*offset = 2 * time.Minute
	e.checkCollection(st)
	alerts := drainAlerts(e)
	if len(alerts) != 1 || alerts[0].Host != "" || !strings.Contains(alerts[0].Reason, "log storage") {
		t.Fatalf("alerts = %+v, want only the storage failure", alerts)
	}
}
