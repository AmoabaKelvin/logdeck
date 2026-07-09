package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginRateLimiter throttles /auth/login attempts per client IP.
var loginRateLimiter = newRateLimiter(10, time.Minute)

// rateLimiter is a fixed-window per-key rate limiter.
type rateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	entries   map[string]*windowEntry
	lastSweep time.Time
	now       func() time.Time // swappable for tests
}

type windowEntry struct {
	count       int
	windowStart time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*windowEntry),
		now:     time.Now,
	}
}

// allow reports whether key may make another request in the current window.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	rl.sweep(now)

	e, ok := rl.entries[key]
	if !ok || now.Sub(e.windowStart) >= rl.window {
		rl.entries[key] = &windowEntry{count: 1, windowStart: now}
		return true
	}
	e.count++
	return e.count <= rl.limit
}

// sweep drops expired entries, at most once per window. Callers must hold rl.mu.
func (rl *rateLimiter) sweep(now time.Time) {
	if now.Sub(rl.lastSweep) < rl.window {
		return
	}
	rl.lastSweep = now
	for key, e := range rl.entries {
		if now.Sub(e.windowStart) >= rl.window {
			delete(rl.entries, key)
		}
	}
}

// middleware rejects requests over the per-IP limit with 429 Too Many Requests.
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.allow(ip) {
			http.Error(w, "Too many login attempts, please try again later", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
