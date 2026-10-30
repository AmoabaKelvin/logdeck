package health

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func newTestTracker() (*Tracker, *time.Time) {
	now := t0
	tr := NewTracker()
	tr.now = func() time.Time { return now }
	return tr, &now
}

func running(ids ...string) []models.ContainerInfo {
	out := make([]models.ContainerInfo, 0, len(ids))
	for _, id := range ids {
		out = append(out, models.ContainerInfo{ID: id, State: "running"})
	}
	return out
}

func container(r Report, host, id string) *ContainerHealth {
	for _, h := range r.Hosts {
		if h.Host != host {
			continue
		}
		for i := range h.Containers {
			if h.Containers[i].ID == id {
				return &h.Containers[i]
			}
		}
	}
	return nil
}

func TestRetryDoesNotHideFailure(t *testing.T) {
	tr, now := newTestTracker()
	tr.TailStarted("local", "a", "web")
	tr.TailFailed("local", "a", errors.New("connection reset"))
	failedAt := *now

	*now = now.Add(time.Minute)
	tr.TailStarted("local", "a", "web") // the hub respawns the tail

	live := container(tr.Report([]string{"local"}), "local", "a").Live
	if live.State != StateFailing || !live.Since.Equal(failedAt) {
		t.Fatalf("live = %+v, want failing since the first failure", live)
	}

	*now = now.Add(time.Minute)
	tr.TailAttached("local", "a")
	live = container(tr.Report([]string{"local"}), "local", "a").Live
	if live.State != StateOK || !live.LastOK.Equal(*now) || live.Error != "" {
		t.Fatalf("live = %+v, want ok now", live)
	}
}

func TestUnreadableDriverIsNotAFailure(t *testing.T) {
	tr, _ := newTestTracker()
	tr.TailFailed("local", "a", errors.New(`configured logging driver does not support reading`))
	tr.BackfillExcluded("local", "b", "db", "configured logging driver does not support reading")

	report := tr.Report([]string{"local"})
	if got := container(report, "local", "a").Live.State; got != StateUnsupported {
		t.Fatalf("live state = %q, want unsupported", got)
	}
	if got := container(report, "local", "b").Stored.State; got != StateExcluded {
		t.Fatalf("stored state = %q, want excluded", got)
	}
	if report.Status != "ok" || tr.Degraded() || len(tr.Failures()) != 0 {
		t.Fatalf("intentional gaps reported as failures: %+v", tr.Failures())
	}
}

func TestObserveListingForgetsWhatIsGone(t *testing.T) {
	tr, _ := newTestTracker()
	for _, id := range []string{"a", "b", "c"} {
		tr.TailStarted("local", id, id)
	}
	tr.BackfillScheduled("local", "b", "b")
	tr.TailStarted("old", "x", "x")

	// a keeps running, b stopped, c was removed, host "old" left the config.
	tr.ObserveListing(map[string][]models.ContainerInfo{
		"local": {{ID: "a", State: "running"}, {ID: "b", State: "exited"}},
	}, nil)

	report := tr.Report([]string{"local", "old"})
	if c := container(report, "local", "a"); c == nil || c.Live == nil {
		t.Fatalf("running container lost its live state: %+v", c)
	}
	if c := container(report, "local", "b"); c == nil || c.Live != nil || c.Stored == nil {
		t.Fatalf("stopped container = %+v, want stored state only", c)
	}
	if c := container(report, "local", "c"); c != nil {
		t.Fatalf("removed container still tracked: %+v", c)
	}
	if report.Hosts[1].State != StatePending || len(report.Hosts[1].Containers) != 0 {
		t.Fatalf("removed host = %+v, want forgotten", report.Hosts[1])
	}
}

func TestHostFailureIsOneFailure(t *testing.T) {
	tr, now := newTestTracker()
	tr.ObserveListing(map[string][]models.ContainerInfo{"remote": running("a", "b")}, nil)
	tr.TailFailed("remote", "a", errors.New("EOF"))
	tr.TailFailed("remote", "b", errors.New("EOF"))

	failures := tr.Failures()
	if len(failures) != 1 || failures[0].Host != "remote" ||
		!strings.Contains(failures[0].Reason, "2 containers") {
		t.Fatalf("failures = %+v, want one entry for both containers", failures)
	}

	*now = now.Add(time.Minute)
	tr.ObserveListing(nil, []docker.HostError{{HostName: "remote", Err: errors.New("dial tcp: connection refused")}})
	failures = tr.Failures()
	if len(failures) != 1 || !strings.HasPrefix(failures[0].Reason, "cannot list containers") ||
		!failures[0].Since.Equal(*now) {
		t.Fatalf("failures = %+v, want only the host failure", failures)
	}
	// An unreachable host keeps its containers: they are not known to be gone.
	if container(tr.Report([]string{"remote"}), "remote", "a") == nil {
		t.Fatal("containers of an unreachable host were forgotten")
	}
}

func TestStorageFailures(t *testing.T) {
	tr, now := newTestTracker()
	tr.StorageSpace(10<<20, true)
	*now = now.Add(time.Minute)
	tr.StorageWrite(errors.New("disk I/O error"))

	failures := tr.Failures()
	if len(failures) != 1 || failures[0].Host != "" || !failures[0].Since.Equal(t0) ||
		!strings.Contains(failures[0].Reason, "writes are failing") ||
		!strings.Contains(failures[0].Reason, "low disk space") {
		t.Fatalf("failures = %+v, want one storage failure since the low-space sample", failures)
	}

	tr.StorageSpace(10<<30, false)
	tr.StorageWrite(nil)
	if tr.Degraded() {
		t.Fatalf("still degraded after recovery: %+v", tr.Failures())
	}
	if s := tr.Storage(); s.State != StateOK || !s.LastOK.Equal(*now) {
		t.Fatalf("storage = %+v, want ok", s)
	}
}

func TestNilTrackerRecordsNothing(t *testing.T) {
	var tr *Tracker
	tr.TailFailed("local", "a", errors.New("x"))
	tr.StorageWrite(errors.New("x"))
	if tr.Degraded() || len(tr.Failures()) != 0 {
		t.Fatal("nil tracker reported state")
	}
}
