package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/redact"
)

func redactionRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestGetRedactionOffersDefaultsUntilSaved(t *testing.T) {
	router, _ := newLogStoreTestRouter(t, nil)

	w := redactionRequest(t, router, "GET", "/api/v1/settings/redaction", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Enabled  bool              `json:"enabled"`
		Builtins []string          `json:"builtins"`
		Catalog  []redact.Builtin  `json:"catalog"`
		Rules    []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Enabled || !slices.Equal(body.Builtins, redact.DefaultBuiltins()) || len(body.Catalog) == 0 || body.Rules == nil {
		t.Fatalf("unexpected defaults: %s", w.Body.String())
	}
}

func TestUpdateRedactionSavesAndActivates(t *testing.T) {
	t.Cleanup(func() { redact.Activate(nil) })
	router, manager := newLogStoreTestRouter(t, nil)

	w := redactionRequest(t, router, "PUT", "/api/v1/settings/redaction",
		`{"enabled":true,"builtins":[],"rules":[{"name":"order ids","enabled":true,"pattern":"order-\\d+"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT = %d: %s", w.Code, w.Body.String())
	}
	if got := manager.Redaction(); !got.Enabled || len(got.Rules) != 1 {
		t.Fatalf("stored config = %+v", got)
	}
	if got := redact.Active().For(redact.Target{}).Line("shipped order-42"); got != "shipped [REDACTED]" {
		t.Fatalf("active rules redact to %q", got)
	}

	// Saving with redaction off turns it off at once.
	w = redactionRequest(t, router, "PUT", "/api/v1/settings/redaction", `{"enabled":false,"builtins":[]}`)
	if w.Code != http.StatusOK || redact.Active() != nil {
		t.Fatalf("disable: %d, active = %v", w.Code, redact.Active())
	}
}

func TestUpdateRedactionRejectsInvalidRule(t *testing.T) {
	t.Cleanup(func() { redact.Activate(nil) })
	router, manager := newLogStoreTestRouter(t, nil)

	w := redactionRequest(t, router, "PUT", "/api/v1/settings/redaction",
		`{"enabled":true,"builtins":["credentials"],"rules":[{"name":"broken","enabled":false,"pattern":"(unclosed"}]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `rule "broken": invalid pattern`) {
		t.Fatalf("PUT = %d %q, want 400 naming the rule", w.Code, w.Body.String())
	}
	if manager.Redaction().Builtins != nil || redact.Active() != nil {
		t.Fatal("a rejected config must be neither saved nor activated")
	}
}

func TestPreviewRedactionDoesNotSaveOrActivate(t *testing.T) {
	router, manager := newLogStoreTestRouter(t, nil)

	w := redactionRequest(t, router, "POST", "/api/v1/settings/redaction/preview",
		`{"config":{"enabled":false,"builtins":["credentials"]},"sample":"a password=x\nb token: y"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("preview = %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Output != "a password=[REDACTED]\nb token: [REDACTED]" {
		t.Fatalf("output = %q", body.Output)
	}
	if manager.Redaction().Builtins != nil || redact.Active() != nil {
		t.Fatal("preview must not save or activate anything")
	}

	w = redactionRequest(t, router, "POST", "/api/v1/settings/redaction/preview",
		`{"config":{"rules":[{"name":"e","pattern":"x*"}]},"sample":"abc"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid preview = %d, want 400", w.Code)
	}
}
