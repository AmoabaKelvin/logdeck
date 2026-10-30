package alerts

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func TestSilenceActiveOneOff(t *testing.T) {
	at := func(d time.Duration) string { return t0.Add(d).Format(time.RFC3339) }
	tests := []struct {
		name      string
		silence   config.AlertSilence
		active    bool
		wantUntil time.Time
	}{
		{"until deleted", config.AlertSilence{}, true, time.Time{}},
		{"for an hour", config.AlertSilence{EndsAt: at(time.Hour)}, true, t0.Add(time.Hour)},
		{"ended", config.AlertSilence{EndsAt: at(-time.Minute)}, false, time.Time{}},
		{"not started", config.AlertSilence{StartsAt: at(time.Minute), EndsAt: at(time.Hour)}, false, time.Time{}},
		{"started", config.AlertSilence{StartsAt: at(-time.Minute), EndsAt: at(time.Hour)}, true, t0.Add(time.Hour)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, until := SilenceActive(tt.silence, t0)
			if active != tt.active || !until.Equal(tt.wantUntil) {
				t.Fatalf("SilenceActive = %v, %v; want %v, %v", active, until, tt.active, tt.wantUntil)
			}
		})
	}
}

func TestSilenceActiveSchedule(t *testing.T) {
	// Sundays 23:00 to 01:00 in Berlin (UTC+2 in July).
	s := config.AlertSilence{Schedule: &config.SilenceSchedule{
		Days: []string{"sun"}, Start: "23:00", DurationMinutes: 120, Timezone: "Europe/Berlin",
	}}
	sunday := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) // t0 is a Friday
	tests := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{"before", sunday.Add(20*time.Hour + 59*time.Minute), false},
		{"start", sunday.Add(21 * time.Hour), true},
		{"after midnight", sunday.Add(22*time.Hour + 30*time.Minute), true},
		{"end", sunday.Add(23 * time.Hour), false},
		{"other day", sunday.Add(-24*time.Hour + 21*time.Hour + 30*time.Minute), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, until := SilenceActive(s, tt.now)
			if active != tt.active {
				t.Fatalf("SilenceActive at %v = %v, want %v", tt.now, active, tt.active)
			}
			if active && !until.Equal(sunday.Add(23*time.Hour)) {
				t.Fatalf("until = %v, want the window's end", until)
			}
		})
	}
}

func TestSilencedMatchesEveryDimension(t *testing.T) {
	alert := models.Alert{RuleID: "r1", Host: "prod", ContainerName: "api", Project: "shop"}
	tests := []struct {
		name    string
		silence config.AlertSilence
		want    bool
	}{
		{"everything", config.AlertSilence{}, true},
		{"rule", config.AlertSilence{RuleID: "r1"}, true},
		{"other rule", config.AlertSilence{RuleID: "r2"}, false},
		{"host and project", config.AlertSilence{Host: "prod", Project: "shop"}, true},
		{"host but other project", config.AlertSilence{Host: "prod", Project: "blog"}, false},
		{"container", config.AlertSilence{Container: "api"}, true},
		{"ended", config.AlertSilence{Container: "api", EndsAt: t0.Add(-time.Second).Format(time.RFC3339)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := silenced([]config.AlertSilence{tt.silence}, alert, t0); got != tt.want {
				t.Fatalf("silenced = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSilencedAlertIsRecordedButNotDelivered(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts.Add(1) }))
	defer srv.Close()

	rule := config.AlertRule{ID: "e1", Name: "Container died", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1, CooldownSeconds: 1}
	te := startTestEngine(t, rule)
	te.conf.set(config.AlertsConfig{
		Channels: webhookChannels(srv.URL),
		Rules:    []config.AlertRule{rule},
		Silences: []config.AlertSilence{{ID: "s1", Project: "shop", EndsAt: t0.Add(time.Hour).Format(time.RFC3339)}},
	})
	die := func(id string, labels map[string]string) {
		te.events.ch <- docker.EngineEvent{Host: "local", ContainerID: id, ContainerName: id, Action: "die", ExitCode: "1", Labels: labels}
	}

	die("cart", map[string]string{"com.docker.compose.project": "shop"})
	waitFor(t, "muted alert", func() bool { return len(te.e.History(0)) == 1 })
	a := te.e.History(0)[0]
	if a.Project != "shop" || a.Delivery == nil || a.Delivery.Status != "muted" {
		t.Fatalf("alert = %+v, want muted with its project", a)
	}

	die("blog", nil)
	waitFor(t, "delivered alert", func() bool {
		h := te.e.History(0)
		return len(h) == 2 && h[0].Delivery != nil
	})
	if d := te.e.History(0)[0].Delivery; d.Status != "ok" || posts.Load() != 1 {
		t.Fatalf("delivery = %+v with %d posts, want one ok delivery", d, posts.Load())
	}
}
