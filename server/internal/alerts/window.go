package alerts

import "time"

// ruleWindow tracks the sliding match window and cooldown state for one
// (rule, host, container) key. All transitions take explicit now values, so
// the type is pure and directly unit-testable. It is owned exclusively by the
// engine's run loop; no locking.
type ruleWindow struct {
	threshold int
	window    time.Duration
	cooldown  time.Duration

	// times is a ring of the last threshold match times.
	times []time.Time
	head  int
	count int

	fired      bool
	lastFired  time.Time
	suppressed int
	lastSeen   time.Time
}

// observeResult reports the outcome of one recorded match.
type observeResult struct {
	fire bool
	// suppressed is the number of window trips swallowed by the cooldown
	// since the previous fire; only meaningful when fire is true.
	suppressed int
}

func newRuleWindow(threshold int, window, cooldown time.Duration) *ruleWindow {
	return &ruleWindow{
		threshold: threshold,
		window:    window,
		cooldown:  cooldown,
		times:     make([]time.Time, threshold),
	}
}

// observe records one rule match at now. The window trips when the ring holds
// threshold matches no further apart than window. A trip inside the cooldown
// is counted as suppressed; a trip outside it fires, resets the ring, and
// surfaces the accumulated suppressed count.
func (w *ruleWindow) observe(now time.Time) observeResult {
	w.lastSeen = now

	if w.count == w.threshold {
		w.times[w.head] = time.Time{}
		w.head = (w.head + 1) % w.threshold
		w.count--
	}
	w.times[(w.head+w.count)%w.threshold] = now
	w.count++

	if w.count < w.threshold || now.Sub(w.times[w.head]) > w.window {
		return observeResult{}
	}
	if w.fired && now.Sub(w.lastFired) < w.cooldown {
		w.suppressed++
		return observeResult{}
	}

	res := observeResult{fire: true, suppressed: w.suppressed}
	w.head = 0
	w.count = 0
	w.fired = true
	w.lastFired = now
	w.suppressed = 0
	return res
}

// idleSince reports how long ago the key last saw a match, used by the run
// loop to prune long-idle state.
func (w *ruleWindow) idleSince(now time.Time) time.Duration {
	return now.Sub(w.lastSeen)
}
