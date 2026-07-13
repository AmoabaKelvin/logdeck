package alerts

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

func at(seconds int) time.Time {
	return t0.Add(time.Duration(seconds) * time.Second)
}

func TestWindowTripsAtExactlyThreshold(t *testing.T) {
	w := newRuleWindow(3, 60*time.Second, 300*time.Second)

	if res := w.observe(at(0)); res.fire {
		t.Fatal("fired after 1 of 3 matches")
	}
	if res := w.observe(at(1)); res.fire {
		t.Fatal("fired after 2 of 3 matches")
	}
	res := w.observe(at(2))
	if !res.fire {
		t.Fatal("did not fire at exactly 3 matches inside the window")
	}
	if res.suppressed != 0 {
		t.Fatalf("suppressed = %d, want 0", res.suppressed)
	}
}

func TestWindowNoTripBelowThreshold(t *testing.T) {
	w := newRuleWindow(3, 60*time.Second, 300*time.Second)

	if w.observe(at(0)).fire || w.observe(at(1)).fire {
		t.Fatal("fired below threshold")
	}
}

func TestWindowStraddling(t *testing.T) {
	w := newRuleWindow(3, 60*time.Second, 300*time.Second)

	w.observe(at(0))
	w.observe(at(30))
	if w.observe(at(90)).fire {
		t.Fatal("fired although the 3 matches span more than the window")
	}
	if w.observe(at(100)).fire {
		t.Fatal("fired although the last 3 matches span 70s > 60s window")
	}
	if !w.observe(at(110)).fire {
		t.Fatal("did not fire once the last 3 matches fit inside the window")
	}
}

func TestCooldownSuppressAggregateAndSurface(t *testing.T) {
	w := newRuleWindow(1, 60*time.Second, 300*time.Second)

	if !w.observe(at(0)).fire {
		t.Fatal("first match did not fire")
	}
	if w.observe(at(10)).fire || w.observe(at(20)).fire {
		t.Fatal("fired inside the cooldown")
	}

	res := w.observe(at(301))
	if !res.fire {
		t.Fatal("did not fire after the cooldown elapsed")
	}
	if res.suppressed != 2 {
		t.Fatalf("suppressed = %d, want 2", res.suppressed)
	}

	// Suppression counter resets after surfacing.
	if w.observe(at(310)).fire {
		t.Fatal("fired inside the second cooldown")
	}
	res = w.observe(at(602))
	if !res.fire {
		t.Fatal("did not fire after the second cooldown elapsed")
	}
	if res.suppressed != 1 {
		t.Fatalf("suppressed = %d, want 1", res.suppressed)
	}
}

func TestWindowResetsAfterFire(t *testing.T) {
	w := newRuleWindow(2, 60*time.Second, time.Second)

	w.observe(at(0))
	if !w.observe(at(1)).fire {
		t.Fatal("did not fire at threshold")
	}
	// The ring was reset: one fresh match must not fire even though the
	// pre-fire matches were recent and the cooldown has elapsed.
	if w.observe(at(3)).fire {
		t.Fatal("fired with only 1 fresh match after reset")
	}
	if !w.observe(at(4)).fire {
		t.Fatal("did not fire once threshold fresh matches accumulated again")
	}
}

func TestWindowsAreIndependent(t *testing.T) {
	a := newRuleWindow(2, 60*time.Second, 300*time.Second)
	b := newRuleWindow(2, 60*time.Second, 300*time.Second)

	a.observe(at(0))
	if b.observe(at(1)).fire {
		t.Fatal("window b fired from window a's match")
	}
	if !a.observe(at(2)).fire {
		t.Fatal("window a did not fire at its own threshold")
	}
}

func TestWindowIdleSince(t *testing.T) {
	w := newRuleWindow(1, 60*time.Second, 300*time.Second)
	w.observe(at(0))
	if got := w.idleSince(at(90)); got != 90*time.Second {
		t.Fatalf("idleSince = %v, want 90s", got)
	}
}
