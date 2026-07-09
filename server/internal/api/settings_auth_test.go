package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

// newTestRouter builds a router with a clean env-derived config and the given
// auth service (nil means auth disabled).
func newTestRouter(t *testing.T, authSvc *auth.Service) http.Handler {
	t.Helper()
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "READONLY_MODE", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, authSvc, manager.Config())
	return NewRouter(registry, manager, "test")
}

func newTestAuthService(t *testing.T) *auth.Service {
	t.Helper()
	svc := auth.NewServiceFromFileConfig(&config.FileAuthConfig{
		Enabled:           true,
		JWTSecret:         "test-secret",
		AdminUsername:     "admin",
		AdminPasswordHash: auth.HashPasswordSHA256("password", "salt"),
		AdminPasswordSalt: "salt",
	})
	if svc == nil {
		t.Fatal("failed to create auth service from file config")
	}
	return svc
}

func TestSettingsRejectUnauthenticatedWhenAuthEnabled(t *testing.T) {
	router := newTestRouter(t, newTestAuthService(t))

	endpoints := []struct{ method, path string }{
		{"GET", "/api/v1/settings"},
		{"PUT", "/api/v1/settings/docker-hosts"},
		{"PUT", "/api/v1/settings/coolify-hosts"},
		{"PUT", "/api/v1/settings/read-only"},
		{"PUT", "/api/v1/settings/auth"},
		{"POST", "/api/v1/settings/test/docker-host"},
		{"POST", "/api/v1/settings/test/coolify-host"},
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

func TestSettingsAllowAuthenticatedRequests(t *testing.T) {
	svc := newTestAuthService(t)
	router := newTestRouter(t, svc)

	token, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/settings", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettingsAccessibleWhenAuthDisabled(t *testing.T) {
	router := newTestRouter(t, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/settings", nil)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with auth disabled, got %d: %s", w.Code, w.Body.String())
	}
}
