package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientSendsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	c := newClient(server.URL, "secret123")
	var out struct {
		Status string `json:"status"`
	}
	if err := c.get(context.Background(), "/healthz", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("Authorization = %q, want Bearer secret123", gotAuth)
	}
	if out.Status != "ok" {
		t.Errorf("status = %q, want ok", out.Status)
	}
}

func TestClientUnauthorizedHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
	}))
	defer server.Close()

	err := newClient(server.URL, "").get(context.Background(), "/containers", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LOGDECK_TOKEN") || !strings.Contains(err.Error(), "LogDeck Settings") {
		t.Errorf("401 error should hint at token auth, got: %v", err)
	}
}

func TestClientSurfacesServerMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server is in read-only mode", http.StatusForbidden)
	}))
	defer server.Close()

	err := newClient(server.URL, "").post(context.Background(), "/containers/x/stop", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "read-only mode") || !strings.Contains(err.Error(), "403") {
		t.Errorf("expected server message and status, got: %v", err)
	}
}

func TestClientUnreachable(t *testing.T) {
	err := newClient("http://127.0.0.1:1", "").get(context.Background(), "/healthz", nil, nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "cannot reach LogDeck server") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestAggregatedLogsBatching verifies >20 targets are split into multiple
// requests and the results are merged chronologically.
func TestAggregatedLogsBatching(t *testing.T) {
	var requests int
	var perRequestTargets []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/logs/aggregate" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		requests++
		targets := strings.Split(r.URL.Query().Get("targets"), ",")
		perRequestTargets = append(perRequestTargets, len(targets))

		// Later batches return earlier timestamps to prove merging happens.
		ts := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC).Add(-time.Duration(requests) * time.Minute)
		fmt.Fprintf(w, `{"logs":[{"timestamp":%q,"level":"INFO","message":"batch %d"}],"count":1}`,
			ts.Format(time.RFC3339), requests)
	}))
	defer server.Close()

	a := &app{client: newClient(server.URL, ""), output: "table"}

	targets := make([]string, 25)
	for i := range targets {
		targets[i] = fmt.Sprintf("prod~id%d~name%d", i, i)
	}

	logs, err := a.aggregatedLogs(context.Background(), targets, url.Values{"tail": {"100"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 2 {
		t.Errorf("expected 2 requests, got %d", requests)
	}
	if perRequestTargets[0] != 20 || perRequestTargets[1] != 5 {
		t.Errorf("unexpected batch sizes: %v", perRequestTargets)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 merged entries, got %d", len(logs))
	}
	if logs[0].Message != "batch 2" || logs[1].Message != "batch 1" {
		t.Errorf("entries not merged by timestamp: %q then %q", logs[0].Message, logs[1].Message)
	}
}

func TestIsHeartbeat(t *testing.T) {
	if !isHeartbeat([]byte(`{"type":"heartbeat"}`)) {
		t.Error("heartbeat line not detected")
	}
	if isHeartbeat([]byte(`{"timestamp":"2026-01-02T12:00:00Z","message":"type: heartbeat"}`)) {
		t.Error("regular entry misdetected as heartbeat")
	}
	if isHeartbeat([]byte(`not json`)) {
		t.Error("non-JSON misdetected as heartbeat")
	}
}
