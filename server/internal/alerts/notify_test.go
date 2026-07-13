package alerts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func testAlert() models.Alert {
	return models.Alert{
		ID:            "abcd1234",
		RuleID:        "r1",
		RuleName:      "High error rate",
		Type:          "log",
		Host:          "local",
		ContainerName: "web",
		Reason:        "5 matches (level >= ERROR) within 60s",
		Count:         5,
		FiredAt:       "2026-07-10T12:00:00Z",
	}
}

func TestDeliverPayloadShape(t *testing.T) {
	var body []byte
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		body, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	n := newNotifier()
	res := n.deliver(context.Background(), srv.URL, testAlert(), nil)

	if res.Status != "ok" || res.HTTPStatus != http.StatusOK {
		t.Fatalf("result = %+v, want ok/200", res)
	}
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q", contentType)
	}
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.Source != "logdeck" || payload.Version != 1 {
		t.Fatalf("source/version = %q/%d, want logdeck/1", payload.Source, payload.Version)
	}
	if payload.Text == "" || payload.Text != payload.Content {
		t.Fatalf("text %q and content %q must be the same non-empty summary", payload.Text, payload.Content)
	}
	if payload.Alert.ID != "abcd1234" || payload.Alert.RuleName != "High error rate" {
		t.Fatalf("embedded alert wrong: %+v", payload.Alert)
	}
}

func TestDeliverRetriesOnceOn5xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}))
	defer srv.Close()

	n := newNotifier()
	n.retryDelay = 10 * time.Millisecond
	res := n.deliver(context.Background(), srv.URL, testAlert(), nil)

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if res.Status != "ok" {
		t.Fatalf("result = %+v, want ok after retry", res)
	}
}

func TestDeliverNoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	n := newNotifier()
	n.retryDelay = 10 * time.Millisecond
	res := n.deliver(context.Background(), srv.URL, testAlert(), nil)

	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (4xx is permanent)", got)
	}
	if res.Status != "failed" || res.HTTPStatus != http.StatusBadRequest || res.Error == "" {
		t.Fatalf("result = %+v, want failed/400 with error", res)
	}
}

func TestDeliverSkipsRetryOnShutdown(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := newNotifier()
	n.retryDelay = time.Minute // would hang the test if the skip is ignored
	skip := make(chan struct{})
	close(skip)
	res := n.deliver(context.Background(), srv.URL, testAlert(), skip)

	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (shutdown skips the retry)", got)
	}
	if res.Status != "failed" || res.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("result = %+v, want failed/500 first-attempt result", res)
	}
}
