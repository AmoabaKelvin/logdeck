package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/audit"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func TestAuditRecordsChangesWithIdentity(t *testing.T) {
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("READONLY_MODE", "true")
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", cfgPath)
	svc := newTestAuthService(t)
	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, svc, manager.Config())
	store := audit.Open(audit.Path(cfgPath))
	router := NewRouter(registry, manager, nil, nil, nil, nil, store, "test")

	session, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, r)
		return w
	}

	w := do("POST", "/api/v1/settings/api-tokens", session, `{"name":"ci"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", w.Code, w.Body.String())
	}
	var created struct{ Token, Prefix string }
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// Blocked by read-only mode, still recorded.
	if w := do("POST", "/api/v1/containers/abc123/stop?host=local", created.Token, ""); w.Code != http.StatusForbidden {
		t.Fatalf("stop: %d", w.Code)
	}
	// Reads and previews change nothing and are not recorded.
	do("GET", "/api/v1/settings/api-tokens", created.Token, "")
	do("POST", "/api/v1/settings/redaction/preview", session, `{"sample":"x"}`)
	store.Close()

	got := store.Entries()
	want := []audit.Entry{
		{Actor: "admin", Role: "admin", Action: "api_token.create", Method: "POST", Route: "/api/v1/settings/api-tokens", Target: "ci (" + created.Prefix + ")", Status: 201},
		{Actor: "token:ci", Role: "admin", Action: "container.stop", Method: "POST", Route: "/api/v1/containers/{id}/stop", Host: "local", Target: "abc123", Status: 403},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v", got)
	}
	for i := range want {
		g := got[i]
		if g.Time.IsZero() {
			t.Errorf("entry %d has no time", i)
		}
		g.Time, g.DurationMs = time.Time{}, 0
		if g != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, g, want[i])
		}
	}
	data, err := os.ReadFile(audit.Path(cfgPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), created.Token) {
		t.Fatal("the audit log holds the created token")
	}

	w = do("GET", "/api/v1/settings/audit?actor=TOKEN:&result=failure", session, "")
	var page struct {
		Entries []audit.Entry `json:"entries"`
		Total   int           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].Action != "container.stop" {
		t.Fatalf("filtered page = %+v", page)
	}
	if w := do("GET", "/api/v1/settings/audit?result=maybe", session, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad result filter: %d", w.Code)
	}
}

func TestAuditNamesUserChanges(t *testing.T) {
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", cfgPath)
	svc := newTestAuthService(t)
	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, svc, manager.Config())
	store := audit.Open(audit.Path(cfgPath))
	router := NewRouter(registry, manager, nil, nil, nil, nil, store, "test")
	session, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatal(err)
	}

	do(t, router, "POST", "/api/v1/settings/users", session, `{"username":"dev@example.com","password":"viewer-password","role":"read"}`)
	do(t, router, "PUT", "/api/v1/settings/users/dev%40example.com", session, `{"role":"admin"}`)
	do(t, router, "DELETE", "/api/v1/settings/users/dev%40example.com", session, "")
	store.Close()

	got := store.Entries()
	want := []struct{ action, target string }{
		{"user.create", "dev@example.com"},
		{"user.update", "dev@example.com"},
		{"user.delete", "dev@example.com"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v", got)
	}
	for i, w := range want {
		if got[i].Action != w.action || got[i].Target != w.target {
			t.Errorf("entry %d = %s %q, want %s %q", i, got[i].Action, got[i].Target, w.action, w.target)
		}
	}
}

// A terminal is recorded once its websocket opens, not when it closes.
func TestAuditRecordsWebsocketOnOpen(t *testing.T) {
	store := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	defer store.Close()
	ar := &APIRouter{audit: store}
	release := make(chan struct{})
	defer close(release)
	router := chi.NewRouter()
	router.With(ar.audited).Get("/api/v1/containers/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		<-release
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/containers/abc/exec?host=local", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	deadline := time.Now().Add(5 * time.Second)
	for len(store.Entries()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := store.Entries()
	if len(got) != 1 || got[0].Action != "container.terminal" || got[0].Status != http.StatusSwitchingProtocols || got[0].Target != "abc" {
		t.Fatalf("entries = %+v", got)
	}
}
