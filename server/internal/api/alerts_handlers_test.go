package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

// newAlertsTestRouter builds a router with a clean env-derived config, the
// given auth service (nil means auth disabled), and a real alerting engine.
// It returns the router and the config file path so tests can re-open the
// persisted config.
func newAlertsTestRouter(t *testing.T, authSvc *auth.Service) (http.Handler, string) {
	t.Helper()
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "READONLY_MODE", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", configPath)

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, authSvc, manager.Config())
	engine := alerts.NewEngine(registry, manager, nil)
	return NewRouter(registry, manager, engine, nil, "test"), configPath
}

func doAlertsRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, reader)
	router.ServeHTTP(w, r)
	return w
}

func createAlertRule(t *testing.T, router http.Handler, body string) config.AlertRule {
	t.Helper()
	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/rules", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating rule, got %d: %s", w.Code, w.Body.String())
	}
	var rule config.AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &rule); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	return rule
}

func listAlertRulesRaw(t *testing.T, router http.Handler) (json.RawMessage, []config.AlertRule) {
	t.Helper()
	w := doAlertsRequest(t, router, "GET", "/api/v1/alerts/rules", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing rules, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Rules json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	var rules []config.AlertRule
	if err := json.Unmarshal(resp.Rules, &rules); err != nil {
		t.Fatalf("failed to parse rules array: %v", err)
	}
	return resp.Rules, rules
}

func TestListAlertRulesEmptyIsNotNull(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	raw, _ := listAlertRulesRaw(t, router)
	if string(raw) != "[]" {
		t.Errorf("expected rules to be [], got %s", raw)
	}
}

func TestAlertRulesCRUDRoundTrip(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	created := createAlertRule(t, router, `{"name":"container crashes","type":"event","events":["die","oom"]}`)
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(created.ID) {
		t.Errorf("expected 8-hex rule id, got %q", created.ID)
	}
	if _, err := time.Parse(time.RFC3339, created.CreatedAt); err != nil {
		t.Errorf("createdAt %q is not RFC3339: %v", created.CreatedAt, err)
	}
	if !created.Enabled {
		t.Error("expected enabled to default to true")
	}
	if created.Threshold != 1 {
		t.Errorf("expected threshold default 1, got %d", created.Threshold)
	}
	if created.WindowSeconds != 60 {
		t.Errorf("expected windowSeconds default 60, got %d", created.WindowSeconds)
	}
	if created.CooldownSeconds != 0 {
		t.Errorf("expected cooldownSeconds default 0, got %d", created.CooldownSeconds)
	}

	_, rules := listAlertRulesRaw(t, router)
	if len(rules) != 1 || rules[0].ID != created.ID || rules[0].Name != "container crashes" {
		t.Fatalf("unexpected rules after create: %+v", rules)
	}

	// Full-rule update: ID and CreatedAt must be preserved.
	w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/rules/"+created.ID,
		`{"name":"crashes only","type":"event","events":["die"],"enabled":false,"threshold":3,"windowSeconds":120}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating rule, got %d: %s", w.Code, w.Body.String())
	}
	var updated config.AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse update response: %v", err)
	}
	if updated.ID != created.ID || updated.CreatedAt != created.CreatedAt {
		t.Errorf("update must preserve id and createdAt: %+v vs %+v", updated, created)
	}
	if updated.Name != "crashes only" || updated.Enabled || updated.Threshold != 3 || updated.WindowSeconds != 120 {
		t.Errorf("unexpected updated rule: %+v", updated)
	}
	if len(updated.Events) != 1 || updated.Events[0] != "die" {
		t.Errorf("unexpected updated events: %v", updated.Events)
	}

	_, rules = listAlertRulesRaw(t, router)
	if len(rules) != 1 || rules[0].Name != "crashes only" {
		t.Fatalf("unexpected rules after update: %+v", rules)
	}

	w = doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/rules/"+created.ID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting rule, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "rule deleted") {
		t.Errorf("unexpected delete response: %s", w.Body.String())
	}

	raw, rules := listAlertRulesRaw(t, router)
	if len(rules) != 0 || string(raw) != "[]" {
		t.Errorf("expected empty [] rules after delete, got %s", raw)
	}
}

func TestCreateAlertRuleAcceptsUnhealthyEvent(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	created := createAlertRule(t, router, `{"name":"health check failing","type":"event","events":["unhealthy"]}`)
	if len(created.Events) != 1 || created.Events[0] != "unhealthy" {
		t.Fatalf("unexpected events: %v", created.Events)
	}
}

var alertRuleValidationCases = []struct{ name, body string }{
	{"invalid json", `{`},
	{"missing name", `{"type":"log","minLevel":"ERROR"}`},
	{"whitespace name", `{"name":"   ","type":"log","minLevel":"ERROR"}`},
	{"name too long", fmt.Sprintf(`{"name":%q,"type":"log","minLevel":"ERROR"}`, strings.Repeat("a", 65))},
	{"missing type", `{"name":"r"}`},
	{"invalid type", `{"name":"r","type":"metric"}`},
	{"event missing events", `{"name":"r","type":"event"}`},
	{"event empty events", `{"name":"r","type":"event","events":[]}`},
	{"event invalid event", `{"name":"r","type":"event","events":["start"]}`},
	{"event with minLevel", `{"name":"r","type":"event","events":["die"],"minLevel":"ERROR"}`},
	{"event with pattern", `{"name":"r","type":"event","events":["die"],"pattern":"x"}`},
	{"log without minLevel or pattern", `{"name":"r","type":"log"}`},
	{"log invalid minLevel", `{"name":"r","type":"log","minLevel":"VERBOSE"}`},
	{"log minLevel UNKNOWN", `{"name":"r","type":"log","minLevel":"UNKNOWN"}`},
	{"log invalid pattern", `{"name":"r","type":"log","pattern":"("}`},
	{"log with events", `{"name":"r","type":"log","minLevel":"ERROR","events":["die"]}`},
	{"negative threshold", `{"name":"r","type":"log","minLevel":"ERROR","threshold":-1}`},
	{"threshold too high", `{"name":"r","type":"log","minLevel":"ERROR","threshold":1001}`},
	{"negative windowSeconds", `{"name":"r","type":"log","minLevel":"ERROR","windowSeconds":-1}`},
	{"windowSeconds too small", `{"name":"r","type":"log","minLevel":"ERROR","windowSeconds":4}`},
	{"windowSeconds too large", `{"name":"r","type":"log","minLevel":"ERROR","windowSeconds":3601}`},
	{"negative cooldownSeconds", `{"name":"r","type":"log","minLevel":"ERROR","cooldownSeconds":-1}`},
	{"cooldownSeconds too large", `{"name":"r","type":"log","minLevel":"ERROR","cooldownSeconds":86401}`},
	{"empty hosts entry", `{"name":"r","type":"log","minLevel":"ERROR","hosts":[""]}`},
	{"blank containers entry", `{"name":"r","type":"log","minLevel":"ERROR","containers":[" "]}`},
	{"slash-only containers entry", `{"name":"r","type":"log","minLevel":"ERROR","containers":["/"]}`},
	{"empty projects entry", `{"name":"r","type":"log","minLevel":"ERROR","projects":[""]}`},
}

func TestCreateAlertRuleValidation(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	for _, tc := range alertRuleValidationCases {
		w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/rules", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}

	// No invalid rule may have been persisted.
	raw, _ := listAlertRulesRaw(t, router)
	if string(raw) != "[]" {
		t.Errorf("expected no rules persisted after invalid creates, got %s", raw)
	}
}

func TestUpdateAlertRuleValidation(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	created := createAlertRule(t, router, `{"name":"errors","type":"log","minLevel":"ERROR"}`)

	for _, tc := range alertRuleValidationCases {
		w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/rules/"+created.ID, tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}

	// The stored rule must be untouched.
	_, rules := listAlertRulesRaw(t, router)
	if len(rules) != 1 || rules[0].Name != "errors" {
		t.Errorf("expected original rule intact after invalid updates, got %+v", rules)
	}
}

func TestCreateAlertRuleNormalization(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	created := createAlertRule(t, router,
		`{"name":"  noisy errors  ","type":"log","minLevel":" error ","pattern":" boom ","containers":["/web"," api "]}`)

	if created.Name != "noisy errors" {
		t.Errorf("expected trimmed name, got %q", created.Name)
	}
	if created.MinLevel != "ERROR" {
		t.Errorf("expected minLevel normalized to ERROR, got %q", created.MinLevel)
	}
	if created.Pattern != "boom" {
		t.Errorf("expected trimmed pattern, got %q", created.Pattern)
	}
	// Container names are matched without the Docker API's leading "/": a
	// rule created with "/web" must be stored slash-stripped and trimmed.
	if len(created.Containers) != 2 || created.Containers[0] != "web" || created.Containers[1] != "api" {
		t.Errorf("expected containers normalized to [web api], got %v", created.Containers)
	}
}

func TestCreateAlertRuleExplicitEnabledFalse(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	created := createAlertRule(t, router, `{"name":"paused","type":"log","minLevel":"WARN","enabled":false}`)
	if created.Enabled {
		t.Error("expected enabled false to be respected")
	}
}

func TestCreateAlertRuleLimit(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	for i := 0; i < 50; i++ {
		createAlertRule(t, router, fmt.Sprintf(`{"name":"rule-%d","type":"log","minLevel":"ERROR"}`, i))
	}

	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/rules", `{"name":"one-too-many","type":"log","minLevel":"ERROR"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when exceeding rule cap, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAndDeleteUnknownAlertRule(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/rules/deadbeef", `{"name":"r","type":"log","minLevel":"ERROR"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 updating unknown rule, got %d: %s", w.Code, w.Body.String())
	}

	w = doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/rules/deadbeef", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 deleting unknown rule, got %d: %s", w.Code, w.Body.String())
	}
}

func listAlertChannelsRaw(t *testing.T, router http.Handler) (json.RawMessage, []config.AlertChannel) {
	t.Helper()
	w := doAlertsRequest(t, router, "GET", "/api/v1/alerts/channels", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing channels, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Channels json.RawMessage `json:"channels"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	var channels []config.AlertChannel
	if err := json.Unmarshal(resp.Channels, &channels); err != nil {
		t.Fatalf("failed to parse channels array: %v", err)
	}
	return resp.Channels, channels
}

func TestListAlertChannelsEmptyIsNotNull(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	raw, _ := listAlertChannelsRaw(t, router)
	if string(raw) != "[]" {
		t.Errorf("expected channels to be [], got %s", raw)
	}
}

func TestAlertChannelsCRUD(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	// Create one of each type.
	create := func(body string) config.AlertChannel {
		t.Helper()
		w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating channel, got %d: %s", w.Code, w.Body.String())
		}
		var ch config.AlertChannel
		if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
			t.Fatalf("failed to parse create response: %v", err)
		}
		if ch.ID == "" {
			t.Fatalf("server must assign an id, got %+v", ch)
		}
		return ch
	}

	hook := create(`{"type":"webhook","name":"Slack","url":"https://example.com/hook"}`)
	create(`{"type":"ntfy","url":"https://ntfy.sh/mytopic"}`)
	create(`{"type":"gotify","url":"https://gotify.example.com","token":"apptoken"}`)
	tg := create(`{"type":"telegram","token":"bot42:secret","target":"-1001234"}`)

	_, channels := listAlertChannelsRaw(t, router)
	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}

	// Irrelevant fields are cleared for the chosen type.
	if tg.URL != "" {
		t.Errorf("telegram channel should not store a url, got %q", tg.URL)
	}

	// Update: disable the webhook channel.
	w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/channels/"+hook.ID,
		`{"type":"webhook","name":"Slack","url":"https://example.com/hook","enabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating channel, got %d: %s", w.Code, w.Body.String())
	}
	var updated config.AlertChannel
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse update response: %v", err)
	}
	if updated.ID != hook.ID || updated.Enabled {
		t.Errorf("expected update to keep id and disable, got %+v", updated)
	}

	// Delete.
	w = doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/channels/"+hook.ID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting channel, got %d: %s", w.Code, w.Body.String())
	}
	_, channels = listAlertChannelsRaw(t, router)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels after delete, got %d", len(channels))
	}

	// Update / delete of an unknown id are 404s.
	w = doAlertsRequest(t, router, "PUT", "/api/v1/alerts/channels/nope",
		`{"type":"webhook","url":"https://example.com/hook"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 updating unknown channel, got %d", w.Code)
	}
	w = doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/channels/nope", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 deleting unknown channel, got %d", w.Code)
	}
}

func TestAlertChannelValidation(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"unknown type", `{"type":"pushover","url":"https://x.com"}`},
		{"webhook empty url", `{"type":"webhook"}`},
		{"webhook bad url", `{"type":"webhook","url":"not a url"}`},
		{"webhook non-http", `{"type":"webhook","url":"ftp://x.com"}`},
		{"ntfy empty url", `{"type":"ntfy"}`},
		{"gotify no token", `{"type":"gotify","url":"https://gotify.example.com"}`},
		{"gotify no url", `{"type":"gotify","token":"t"}`},
		{"telegram no token", `{"type":"telegram","target":"123"}`},
		{"telegram no target", `{"type":"telegram","token":"bot:secret"}`},
		{"malformed json", `{`},
	} {
		w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}

	// No invalid channel was stored.
	_, channels := listAlertChannelsRaw(t, router)
	if len(channels) != 0 {
		t.Errorf("expected no channels stored after invalid requests, got %d", len(channels))
	}
}

func TestAlertChannelTestEndpoint(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	// A local server stands in for the webhook so the test never reaches the
	// public network (CI must not depend on DNS or egress).
	var delivered int32
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&delivered, 1)
	}))
	defer hook.Close()

	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels",
		fmt.Sprintf(`{"type":"webhook","url":%q}`, hook.URL))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating channel, got %d: %s", w.Code, w.Body.String())
	}
	var ch config.AlertChannel
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}

	w = doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels/"+ch.ID+"/test", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from channel test, got %d: %s", w.Code, w.Body.String())
	}
	var result models.DeliveryResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse delivery result: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("delivery = %+v, want ok", result)
	}
	if atomic.LoadInt32(&delivered) != 1 {
		t.Fatalf("webhook received %d deliveries, want 1", delivered)
	}

	// Testing an unknown channel is a 404.
	w = doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels/nope/test", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 testing unknown channel, got %d", w.Code)
	}
}

func TestAlertHistory(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	w := doAlertsRequest(t, router, "GET", "/api/v1/alerts/history", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 getting history, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Alerts json.RawMessage `json:"alerts"`
		Count  int             `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse history response: %v", err)
	}
	if string(resp.Alerts) != "[]" {
		t.Errorf("expected alerts to be [], got %s", resp.Alerts)
	}
	if resp.Count != 0 {
		t.Errorf("expected count 0, got %d", resp.Count)
	}

	// Non-integer limit is rejected; out-of-range limits are clamped, not errors.
	w = doAlertsRequest(t, router, "GET", "/api/v1/alerts/history?limit=abc", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-integer limit, got %d", w.Code)
	}
	for _, limit := range []string{"0", "-5", "1", "500", "9999"} {
		w = doAlertsRequest(t, router, "GET", "/api/v1/alerts/history?limit="+limit, "")
		if w.Code != http.StatusOK {
			t.Errorf("limit=%s: expected 200, got %d: %s", limit, w.Code, w.Body.String())
		}
	}

	w = doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/history", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 clearing history, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "history cleared") {
		t.Errorf("unexpected clear response: %s", w.Body.String())
	}
}

func TestAlertRulesPersistAcrossManagerReload(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	created := createAlertRule(t, router, `{"name":"persisted","type":"log","minLevel":"ERROR"}`)
	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/channels", `{"type":"webhook","url":"https://example.com/hook"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating channel, got %d: %s", w.Code, w.Body.String())
	}
	var ch config.AlertChannel
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}

	// Re-open the config file with a fresh manager (CONFIG_PATH is still set).
	reloaded := config.NewManager()
	fc := reloaded.FileConfigSnapshot()
	if fc.Alerts == nil {
		t.Fatal("expected alerts config to persist across reload")
	}
	if len(fc.Alerts.Channels) != 1 || fc.Alerts.Channels[0].ID != ch.ID || fc.Alerts.Channels[0].URL != "https://example.com/hook" {
		t.Errorf("expected created channel to persist, got %+v", fc.Alerts.Channels)
	}
	if len(fc.Alerts.Rules) != 1 || fc.Alerts.Rules[0].ID != created.ID || fc.Alerts.Rules[0].Name != "persisted" {
		t.Errorf("expected created rule to persist, got %+v", fc.Alerts.Rules)
	}
}

func TestAlertRoutesRequireAuthWhenEnabled(t *testing.T) {
	router, _ := newAlertsTestRouter(t, newTestAuthService(t))

	endpoints := []struct{ method, path string }{
		{"GET", "/api/v1/alerts/rules"},
		{"POST", "/api/v1/alerts/rules"},
		{"PUT", "/api/v1/alerts/rules/deadbeef"},
		{"DELETE", "/api/v1/alerts/rules/deadbeef"},
		{"GET", "/api/v1/alerts/channels"},
		{"POST", "/api/v1/alerts/channels"},
		{"PUT", "/api/v1/alerts/channels/deadbeef"},
		{"DELETE", "/api/v1/alerts/channels/deadbeef"},
		{"POST", "/api/v1/alerts/channels/deadbeef/test"},
		{"GET", "/api/v1/alerts/history"},
		{"DELETE", "/api/v1/alerts/history"},
	}
	for _, e := range endpoints {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(e.method, e.path, nil)
		router.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401 without token, got %d", e.method, e.path, w.Code)
		}
	}
}
