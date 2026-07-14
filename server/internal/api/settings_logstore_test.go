package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

// newLogStoreTestRouter builds a router over a clean config file with the log
// store env vars unset, so each test decides which ones are pinned.
func newLogStoreTestRouter(t *testing.T, authSvc *auth.Service) (http.Handler, *config.Manager) {
	t.Helper()
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "READONLY_MODE", "CORS_ALLOWED_ORIGINS",
		"LOG_STORE_ENABLED", "LOG_STORE_PER_CONTAINER_MB", "LOG_STORE_TOTAL_MB",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, authSvc, manager.Config())
	return NewRouter(registry, manager, nil, nil, "test"), manager
}

func putLogStorage(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/v1/settings/log-storage", strings.NewReader(body))
	router.ServeHTTP(w, r)
	return w
}

// getLogStoreBlock reads the logStore section out of GET /settings.
func getLogStoreBlock(t *testing.T, router http.Handler) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/settings", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /settings = %d, want 200: %s", w.Code, w.Body.String())
	}

	var body struct {
		LogStore map[string]any `json:"logStore"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if body.LogStore == nil {
		t.Fatal("GET /settings returned no logStore block")
	}
	return body.LogStore
}

func TestGetSettingsReportsLogStoreDefaultsAsFileSourced(t *testing.T) {
	router, _ := newLogStoreTestRouter(t, nil)

	block := getLogStoreBlock(t, router)
	want := map[string]any{
		"enabled":              true,
		"enabledSource":        "file",
		"perContainerMB":       float64(config.DefaultLogStorePerContainerMB),
		"perContainerMBSource": "file",
		"totalMB":              float64(config.DefaultLogStoreTotalMB),
		"totalMBSource":        "file",
	}
	for key, expected := range want {
		if block[key] != expected {
			t.Errorf("logStore[%q] = %v, want %v", key, block[key], expected)
		}
	}
}

func TestGetSettingsMarksEnvPinnedCapsAsEnvSourced(t *testing.T) {
	router, _ := newLogStoreTestRouter(t, nil)
	t.Setenv("LOG_STORE_TOTAL_MB", "256")

	block := getLogStoreBlock(t, router)
	if block["totalMB"] != float64(256) {
		t.Errorf("totalMB = %v, want the env value 256", block["totalMB"])
	}
	if block["totalMBSource"] != "env" {
		t.Errorf("totalMBSource = %v, want env", block["totalMBSource"])
	}
	if block["perContainerMBSource"] != "file" {
		t.Errorf("perContainerMBSource = %v, want file (no env var set)", block["perContainerMBSource"])
	}
}

func TestUpdateLogStoragePersistsAndIsVisible(t *testing.T) {
	router, manager := newLogStoreTestRouter(t, nil)

	w := putLogStorage(t, router, `{"perContainerMB":10,"totalMB":200}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want 200: %s", w.Code, w.Body.String())
	}

	resolved := manager.LogStore()
	if resolved.PerContainerMB != 10 || resolved.TotalMB != 200 {
		t.Fatalf("resolved config = %+v, want the new caps", resolved)
	}

	block := getLogStoreBlock(t, router)
	if block["perContainerMB"] != float64(10) || block["totalMB"] != float64(200) {
		t.Fatalf("GET /settings did not reflect the update: %v", block)
	}
}

// Only the provided fields change; the others keep their current values.
func TestUpdateLogStorageAppliesOnlyProvidedFields(t *testing.T) {
	router, manager := newLogStoreTestRouter(t, nil)

	if w := putLogStorage(t, router, `{"totalMB":200}`); w.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want 200: %s", w.Code, w.Body.String())
	}

	resolved := manager.LogStore()
	if resolved.TotalMB != 200 {
		t.Fatalf("TotalMB = %d, want 200", resolved.TotalMB)
	}
	if resolved.PerContainerMB != config.DefaultLogStorePerContainerMB {
		t.Fatalf("PerContainerMB = %d, want the untouched default", resolved.PerContainerMB)
	}
	if !resolved.Enabled {
		t.Fatal("Enabled = false, want the untouched default")
	}
}

func TestUpdateLogStorageRejectsInvalidCaps(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // substring the message must name
	}{
		{"per-container above total", `{"perContainerMB":500,"totalMB":100}`, "perContainerMB"},
		{"per-container above the current total", `{"perContainerMB":2000}`, "perContainerMB"},
		{"zero per-container", `{"perContainerMB":0}`, "perContainerMB"},
		{"negative per-container", `{"perContainerMB":-1}`, "perContainerMB"},
		{"zero total", `{"totalMB":0}`, "totalMB"},
		{"negative total", `{"totalMB":-5}`, "totalMB"},
		{"total above the upper bound", `{"totalMB":1048577}`, "totalMB"},
		{"non-integer", `{"perContainerMB":1.5}`, "invalid request body"},
		{"non-numeric", `{"totalMB":"lots"}`, "invalid request body"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, manager := newLogStoreTestRouter(t, nil)

			w := putLogStorage(t, router, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("PUT %s = %d, want 400: %s", tc.body, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Errorf("message %q does not name %q", strings.TrimSpace(w.Body.String()), tc.want)
			}
			if resolved := manager.LogStore(); resolved.PerContainerMB != config.DefaultLogStorePerContainerMB ||
				resolved.TotalMB != config.DefaultLogStoreTotalMB {
				t.Errorf("a rejected request changed the config: %+v", resolved)
			}
		})
	}
}

// A cap pinned by an environment variable is read-only, the same rule the other
// env-sourced settings follow (409 Conflict, as settingsErrorStatus maps them).
func TestUpdateLogStorageRejectsEnvPinnedFields(t *testing.T) {
	router, manager := newLogStoreTestRouter(t, nil)
	t.Setenv("LOG_STORE_PER_CONTAINER_MB", "5")

	w := putLogStorage(t, router, `{"perContainerMB":10}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("PUT = %d, want 409 for an env-pinned cap: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "LOG_STORE_PER_CONTAINER_MB") {
		t.Errorf("message %q does not name the environment variable", strings.TrimSpace(w.Body.String()))
	}
	if got := manager.LogStore().PerContainerMB; got != 5 {
		t.Fatalf("PerContainerMB = %d, want the env value 5 untouched", got)
	}

	// The caps that are not pinned stay editable.
	if w := putLogStorage(t, router, `{"totalMB":200}`); w.Code != http.StatusOK {
		t.Fatalf("PUT totalMB = %d, want 200 (only perContainerMB is pinned): %s", w.Code, w.Body.String())
	}
}

func TestUpdateLogStorageRejectedInReadOnlyMode(t *testing.T) {
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "CORS_ALLOWED_ORIGINS",
		"LOG_STORE_ENABLED", "LOG_STORE_PER_CONTAINER_MB", "LOG_STORE_TOTAL_MB",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("READONLY_MODE", "true")
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, nil, manager.Config())
	router := NewRouter(registry, manager, nil, nil, "test")

	// Lowering a cap evicts stored logs, so it is blocked like any other
	// destructive route.
	if w := putLogStorage(t, router, `{"totalMB":10}`); w.Code != http.StatusForbidden {
		t.Fatalf("PUT = %d, want 403 in read-only mode: %s", w.Code, w.Body.String())
	}
	if got := manager.LogStore().TotalMB; got != config.DefaultLogStoreTotalMB {
		t.Fatalf("TotalMB = %d, want the config untouched in read-only mode", got)
	}
}

func TestUpdateLogStorageDeniesReadScopedToken(t *testing.T) {
	svc := newTestAuthService(t)
	router, _ := newLogStoreTestRouter(t, svc)

	jwt, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"agent","scope":"read"}`))
	r.Header.Set("Authorization", "Bearer "+jwt)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create read token = %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode token: %v", err)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/api/v1/settings/log-storage", strings.NewReader(`{"totalMB":200}`))
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("PUT with a read-scoped token = %d, want 403: %s", w.Code, w.Body.String())
	}
}

func TestUpdateLogStorageRejectsUnauthenticated(t *testing.T) {
	router, _ := newLogStoreTestRouter(t, newTestAuthService(t))

	if w := putLogStorage(t, router, `{"totalMB":200}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("PUT without a token = %d, want 401: %s", w.Code, w.Body.String())
	}
}
