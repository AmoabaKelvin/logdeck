package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := range 3 {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for range 3 {
		rl.allow("1.2.3.4")
	}
	if rl.allow("1.2.3.4") {
		t.Error("request over the limit should be blocked")
	}
	// A different key is unaffected.
	if !rl.allow("5.6.7.8") {
		t.Error("a different key should not be blocked")
	}
}

func TestRateLimiterWindowResets(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	current := time.Unix(1000, 0)
	rl.now = func() time.Time { return current }

	for range 3 {
		rl.allow("1.2.3.4")
	}
	if rl.allow("1.2.3.4") {
		t.Fatal("request over the limit should be blocked")
	}

	current = current.Add(time.Minute)
	if !rl.allow("1.2.3.4") {
		t.Error("request should be allowed after the window resets")
	}
}

func TestRateLimiterMiddleware(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)
	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 2 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		r.RemoteAddr = "1.2.3.4:5555"
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 over the limit, got %d", w.Code)
	}
}
