package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
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
	return NewRouter(registry, manager, engine, "test"), configPath
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
		`{"name":"  noisy errors  ","type":"log","minLevel":" error ","pattern":" boom "}`)

	if created.Name != "noisy errors" {
		t.Errorf("expected trimmed name, got %q", created.Name)
	}
	if created.MinLevel != "ERROR" {
		t.Errorf("expected minLevel normalized to ERROR, got %q", created.MinLevel)
	}
	if created.Pattern != "boom" {
		t.Errorf("expected trimmed pattern, got %q", created.Pattern)
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

func TestAlertsWebhookSetAndClear(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	getURL := func() string {
		t.Helper()
		w := doAlertsRequest(t, router, "GET", "/api/v1/alerts/webhook", "")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 getting webhook, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse webhook response: %v", err)
		}
		return resp.URL
	}

	if got := getURL(); got != "" {
		t.Errorf("expected empty webhook url initially, got %q", got)
	}

	w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/webhook", `{"url":"https://example.com/hook"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 setting webhook, got %d: %s", w.Code, w.Body.String())
	}
	if got := getURL(); got != "https://example.com/hook" {
		t.Errorf("expected webhook url to persist, got %q", got)
	}

	for _, body := range []string{
		`{"url":"not a url"}`,
		`{"url":"ftp://example.com/hook"}`,
		`{"url":"http://"}`,
		`{`,
	} {
		w = doAlertsRequest(t, router, "PUT", "/api/v1/alerts/webhook", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d: %s", body, w.Code, w.Body.String())
		}
	}
	if got := getURL(); got != "https://example.com/hook" {
		t.Errorf("invalid updates must not change the webhook, got %q", got)
	}

	w = doAlertsRequest(t, router, "PUT", "/api/v1/alerts/webhook", `{"url":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 clearing webhook, got %d: %s", w.Code, w.Body.String())
	}
	if got := getURL(); got != "" {
		t.Errorf("expected webhook cleared, got %q", got)
	}
}

func TestAlertsTestWebhookEndpoint(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)

	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/test", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from test webhook, got %d: %s", w.Code, w.Body.String())
	}
	var result models.DeliveryResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse delivery result: %v", err)
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
	w := doAlertsRequest(t, router, "PUT", "/api/v1/alerts/webhook", `{"url":"https://example.com/hook"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 setting webhook, got %d: %s", w.Code, w.Body.String())
	}

	// Re-open the config file with a fresh manager (CONFIG_PATH is still set).
	reloaded := config.NewManager()
	fc := reloaded.FileConfigSnapshot()
	if fc.Alerts == nil {
		t.Fatal("expected alerts config to persist across reload")
	}
	if fc.Alerts.WebhookURL != "https://example.com/hook" {
		t.Errorf("expected webhook url to persist, got %q", fc.Alerts.WebhookURL)
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
		{"GET", "/api/v1/alerts/webhook"},
		{"PUT", "/api/v1/alerts/webhook"},
		{"POST", "/api/v1/alerts/test"},
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
