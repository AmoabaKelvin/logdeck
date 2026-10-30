package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/health"
)

func TestHandleHealthz(t *testing.T) {
	ar := &APIRouter{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	rec := httptest.NewRecorder()
	ar.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status \"ok\", got %q", body["status"])
	}
}

func TestHandleHealthzDegradedCollection(t *testing.T) {
	tracker := health.NewTracker()
	tracker.StorageWrite(errors.New("disk I/O error"))
	ar := &APIRouter{health: tracker}

	for _, tc := range []struct {
		url  string
		code int
	}{
		// Liveness stays 200 so an orchestrator never restarts LogDeck over a
		// collection problem; strict mode is for external monitors.
		{"/api/v1/healthz", http.StatusOK},
		{"/api/v1/healthz?strict=1", http.StatusServiceUnavailable},
	} {
		rec := httptest.NewRecorder()
		ar.handleHealthz(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))
		if rec.Code != tc.code {
			t.Fatalf("%s: expected %d, got %d", tc.url, tc.code, rec.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["status"] != "ok" || body["collection"] != "degraded" {
			t.Fatalf("%s: unexpected body %v", tc.url, body)
		}
	}
}

func TestHandleVersion(t *testing.T) {
	ar := &APIRouter{version: "1.2.3"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	ar.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["version"] != "1.2.3" {
		t.Fatalf("expected version \"1.2.3\", got %q", body["version"])
	}
}
