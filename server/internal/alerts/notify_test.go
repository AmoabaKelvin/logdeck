package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
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

// capturingServer records the last request it received.
type capturedRequest struct {
	method      string
	path        string
	rawQuery    string
	contentType string
	title       string
	priority    string
	tags        string
	gotifyKey   string
	body        []byte
}

func newCapturingServer(t *testing.T, captured *capturedRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.rawQuery = r.URL.RawQuery
		captured.contentType = r.Header.Get("Content-Type")
		captured.title = r.Header.Get("Title")
		captured.priority = r.Header.Get("Priority")
		captured.tags = r.Header.Get("Tags")
		captured.gotifyKey = r.Header.Get("X-Gotify-Key")
		captured.body, _ = io.ReadAll(r.Body)
	}))
}

func webhookChannel(url string) config.AlertChannel {
	return config.AlertChannel{ID: "c1", Type: "webhook", Enabled: true, URL: url}
}

func TestDeliverWebhookPayloadShape(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	n := newNotifier()
	res := n.deliver(context.Background(), webhookChannel(srv.URL), testAlert(), nil)

	if res.Status != "ok" || res.HTTPStatus != http.StatusOK {
		t.Fatalf("result = %+v, want ok/200", res)
	}
	if cap.contentType != "application/json" {
		t.Fatalf("Content-Type = %q", cap.contentType)
	}
	var payload webhookPayload
	if err := json.Unmarshal(cap.body, &payload); err != nil {
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

func TestDeliverNtfyPlainBodyWithTitle(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "ntfy", Enabled: true, URL: srv.URL + "/mytopic"}
	n := newNotifier()
	res := n.deliver(context.Background(), ch, testAlert(), nil)

	if res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if cap.path != "/mytopic" {
		t.Fatalf("path = %q, want /mytopic", cap.path)
	}
	if cap.title != alertTitle {
		t.Fatalf("Title header = %q, want %q", cap.title, alertTitle)
	}
	// Body is the plain-text alert summary, not JSON.
	if got := string(cap.body); got != alertText(testAlert()) {
		t.Fatalf("body = %q, want plain alert text %q", got, alertText(testAlert()))
	}
	if json.Valid(cap.body) {
		t.Fatalf("ntfy body should be plain text, got JSON-parseable %q", cap.body)
	}
}

func TestDeliverGotifyJSONWithToken(t *testing.T) {
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "gotify", Enabled: true, URL: srv.URL, Token: "app-token-123"}
	n := newNotifier()
	res := n.deliver(context.Background(), ch, testAlert(), nil)

	if res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if cap.path != "/message" {
		t.Fatalf("path = %q, want /message", cap.path)
	}
	// The token travels in the X-Gotify-Key header, never the URL.
	if cap.rawQuery != "" {
		t.Fatalf("query = %q, want no token in the URL", cap.rawQuery)
	}
	if cap.gotifyKey != "app-token-123" {
		t.Fatalf("X-Gotify-Key = %q, want app-token-123", cap.gotifyKey)
	}
	if cap.contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", cap.contentType)
	}
	var payload gotifyPayload
	if err := json.Unmarshal(cap.body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.Title != alertTitle || payload.Message != alertText(testAlert()) || payload.Priority != gotifyPriority {
		t.Fatalf("gotify payload = %+v", payload)
	}
}

func TestDeliverTelegramSendMessage(t *testing.T) {
	// Telegram targets a fixed public host, so intercept via a custom transport
	// pointed at a local server capturing the request.
	var cap capturedRequest
	srv := newCapturingServer(t, &cap)
	defer srv.Close()

	ch := config.AlertChannel{ID: "c1", Type: "telegram", Enabled: true, Token: "bot42:secret", Target: "-1001234"}
	spec, err := buildRequestSpec(ch, testAlert())
	if err != nil {
		t.Fatalf("buildRequestSpec: %v", err)
	}
	// Assert the resolved URL matches the Bot API sendMessage endpoint.
	wantURL := "https://api.telegram.org/botbot42:secret/sendMessage"
	if spec.url != wantURL {
		t.Fatalf("url = %q, want %q", spec.url, wantURL)
	}
	if spec.headers["Content-Type"] != "application/json" {
		t.Fatalf("Content-Type = %q", spec.headers["Content-Type"])
	}
	var payload telegramPayload
	if err := json.Unmarshal(spec.body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.ChatID != "-1001234" || payload.Text != alertText(testAlert()) {
		t.Fatalf("telegram payload = %+v", payload)
	}
}

// failingRoundTripper fails every request, letting the http.Client wrap the
// request URL (which for Telegram carries the bot token) in the *url.Error the
// caller sees.
type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp: connection refused")
}

// A network failure must not persist a channel secret into the delivery error,
// which is stored in alert history and shown in the UI.
func TestDeliveryErrorRedactsTelegramToken(t *testing.T) {
	n := &notifier{client: &http.Client{Transport: failingRoundTripper{}}, retryDelay: 0}
	ch := config.AlertChannel{ID: "c1", Type: "telegram", Enabled: true, Token: "bot42:secret", Target: "-1001234"}

	res := n.deliver(context.Background(), ch, testAlert(), nil)
	if res.Status != "failed" {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if strings.Contains(res.Error, "bot42:secret") {
		t.Fatalf("delivery error leaked the bot token: %q", res.Error)
	}
	if !strings.Contains(res.Error, "***") {
		t.Fatalf("expected the token to be redacted, got %q", res.Error)
	}
}

func TestDeliverAllSummarizesFailures(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer okSrv.Close()
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer badSrv.Close()

	channels := []config.AlertChannel{
		{ID: "c1", Type: "webhook", Name: "good", Enabled: true, URL: okSrv.URL},
		{ID: "c2", Type: "webhook", Name: "bad", Enabled: true, URL: badSrv.URL},
	}
	n := newNotifier()
	res := n.deliverAll(context.Background(), channels, testAlert(), nil)

	if res.Status != "failed" {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if !strings.Contains(res.Error, "bad") {
		t.Fatalf("error %q should name the failed channel", res.Error)
	}
	if strings.Contains(res.Error, "good:") {
		t.Fatalf("error %q should not list the succeeding channel", res.Error)
	}
}

func TestDeliverAllAllOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	channels := []config.AlertChannel{
		{ID: "c1", Type: "webhook", Enabled: true, URL: srv.URL},
		{ID: "c2", Type: "ntfy", Enabled: true, URL: srv.URL + "/t"},
	}
	n := newNotifier()
	res := n.deliverAll(context.Background(), channels, testAlert(), nil)
	if res.Status != "ok" {
		t.Fatalf("result = %+v, want ok", res)
	}
	// HTTP status is only carried through for a single channel.
	if res.HTTPStatus != 0 {
		t.Fatalf("httpStatus = %d, want 0 for a multi-channel summary", res.HTTPStatus)
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
	res := n.deliver(context.Background(), webhookChannel(srv.URL), testAlert(), nil)

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
	res := n.deliver(context.Background(), webhookChannel(srv.URL), testAlert(), nil)

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
	res := n.deliver(context.Background(), webhookChannel(srv.URL), testAlert(), skip)

	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1 (shutdown skips the retry)", got)
	}
	if res.Status != "failed" || res.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("result = %+v, want failed/500 first-attempt result", res)
	}
}
