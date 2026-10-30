package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

// newUsersTestRouter builds a router with settings-configured auth and one
// admin, rebuilding the auth service on every config change as main does.
func newUsersTestRouter(t *testing.T) http.Handler {
	t.Helper()
	clearEnvForAuthTest(t)
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	hash, _ := auth.HashPassword("admin-password")
	err := manager.UpdateAuth(func(*config.FileAuthConfig) (*config.FileAuthConfig, error) {
		return &config.FileAuthConfig{Enabled: true, JWTSecret: "test-secret", Users: []config.User{
			{Username: "admin", PasswordHash: hash, Role: auth.RoleAdmin},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := auth.NewService(manager.FileConfigSnapshot().Auth, nil)
	registry := services.NewRegistry(nil, nil, svc, manager.Config())
	manager.OnChange(func(*config.Config) {
		svc, _ := auth.NewService(manager.FileConfigSnapshot().Auth, manager.OIDC())
		registry.SwapAuth(svc)
	})
	return NewRouter(registry, manager, nil, nil, nil, nil, "test")
}

func do(t *testing.T, router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	n := oidcTestIP.Add(1)
	r.RemoteAddr = fmt.Sprintf("10.189.%d.%d:1000", n/250, n%250)
	router.ServeHTTP(w, r)
	return w
}

func login(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	w := do(t, router, "POST", "/api/v1/auth/login", "", `{"username":"`+username+`","password":"`+password+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, w.Code, w.Body.String())
	}
	var resp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token
}

func TestUserManagement(t *testing.T) {
	router := newUsersTestRouter(t)
	admin := login(t, router, "admin", "admin-password")

	if w := do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"viewer","password":"viewer-password","role":"read"}`); w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"viewer","password":"viewer-password","role":"read"}`); w.Code != http.StatusConflict {
		t.Errorf("duplicate create: got %d, want 409", w.Code)
	}
	if w := do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"x","password":"short","role":"read"}`); w.Code != http.StatusBadRequest {
		t.Errorf("short password: got %d, want 400", w.Code)
	}

	w := do(t, router, "GET", "/api/v1/settings/users", admin, "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "passwordHash") || !strings.Contains(w.Body.String(), `"username":"viewer"`) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}

	// The read-only user can look but not change, and settings are off-limits.
	viewer := login(t, router, "viewer", "viewer-password")
	if w := do(t, router, "GET", "/api/v1/auth/me", viewer, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"role":"read"`) {
		t.Errorf("viewer /auth/me: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "GET", "/api/v1/settings", viewer, ""); w.Code != http.StatusForbidden {
		t.Errorf("viewer GET /settings: got %d, want 403", w.Code)
	}
	if w := do(t, router, "POST", "/api/v1/containers/abc/restart", viewer, ""); w.Code != http.StatusForbidden {
		t.Errorf("viewer restart: got %d, want 403", w.Code)
	}

	// Promotion applies to the existing session on its next request.
	if w := do(t, router, "PUT", "/api/v1/settings/users/viewer", admin, `{"role":"admin"}`); w.Code != http.StatusOK {
		t.Fatalf("promote: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "GET", "/api/v1/settings", viewer, ""); w.Code != http.StatusOK {
		t.Errorf("promoted viewer GET /settings: got %d, want 200", w.Code)
	}

	// A password reset ends the sessions signed in with the old password.
	if w := do(t, router, "PUT", "/api/v1/settings/users/viewer", admin, `{"password":"another-password"}`); w.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "GET", "/api/v1/auth/me", viewer, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("session after reset: got %d, want 401", w.Code)
	}
	viewer = login(t, router, "viewer", "another-password")

	// Deleting a user ends their sessions.
	if w := do(t, router, "DELETE", "/api/v1/settings/users/viewer", admin, ""); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "GET", "/api/v1/auth/me", viewer, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("session after delete: got %d, want 401", w.Code)
	}
	if w := do(t, router, "DELETE", "/api/v1/settings/users/viewer", admin, ""); w.Code != http.StatusNotFound {
		t.Errorf("delete missing: got %d, want 404", w.Code)
	}

	// An email as username, escaped in the path the way the UI sends it.
	do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"ops@example.com","password":"ops-password","role":"read"}`)
	if w := do(t, router, "DELETE", "/api/v1/settings/users/ops%40example.com", admin, ""); w.Code != http.StatusOK {
		t.Errorf("delete escaped username: %d %s", w.Code, w.Body.String())
	}
}

func TestLastAdminCannotBeRemoved(t *testing.T) {
	router := newUsersTestRouter(t)
	admin := login(t, router, "admin", "admin-password")

	if w := do(t, router, "DELETE", "/api/v1/settings/users/admin", admin, ""); w.Code != http.StatusConflict {
		t.Errorf("delete last admin: got %d, want 409", w.Code)
	}
	if w := do(t, router, "PUT", "/api/v1/settings/users/admin", admin, `{"role":"read"}`); w.Code != http.StatusConflict {
		t.Errorf("demote last admin: got %d, want 409", w.Code)
	}

	// With a second admin, the first can go.
	do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"second","password":"second-password","role":"admin"}`)
	if w := do(t, router, "PUT", "/api/v1/settings/users/admin", admin, `{"role":"read"}`); w.Code != http.StatusOK {
		t.Errorf("demote with another admin: got %d %s", w.Code, w.Body.String())
	}
}

func TestEnvAdminIsListedAndFixed(t *testing.T) {
	hash, _ := auth.HashPassword("env-password")
	clearEnvForAuthTest(t)
	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", hash)
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	svc, err := auth.NewService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry := services.NewRegistry(nil, nil, svc, manager.Config())
	router := NewRouter(registry, manager, nil, nil, nil, nil, "test")
	root := login(t, router, "root", "env-password")

	w := do(t, router, "GET", "/api/v1/settings/users", root, "")
	if !strings.Contains(w.Body.String(), `"source":"env"`) {
		t.Errorf("env admin not listed: %s", w.Body.String())
	}
	if w := do(t, router, "DELETE", "/api/v1/settings/users/root", root, ""); w.Code != http.StatusConflict {
		t.Errorf("delete env admin: got %d, want 409", w.Code)
	}
	// Users can still be added next to an env-configured admin.
	if w := do(t, router, "POST", "/api/v1/settings/users", root, `{"username":"viewer","password":"viewer-password","role":"read"}`); w.Code != http.StatusCreated {
		t.Errorf("create next to env admin: %d %s", w.Code, w.Body.String())
	}
}

func TestUpdateOIDCKeepsMaskedSecret(t *testing.T) {
	router := newUsersTestRouter(t)
	admin := login(t, router, "admin", "admin-password")

	body := `{"enabled":true,"issuer":"https://id.example.com/","clientId":"logdeck","clientSecret":"s3cret","redirectUrl":"https://logdeck.example.com/api/v1/auth/oidc/callback","adminGroups":["ops"]}`
	if w := do(t, router, "PUT", "/api/v1/settings/oidc", admin, body); w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	w := do(t, router, "GET", "/api/v1/settings", admin, "")
	if strings.Contains(w.Body.String(), "s3cret") || !strings.Contains(w.Body.String(), `"issuer":"https://id.example.com"`) {
		t.Fatalf("settings should show OIDC with the secret masked: %s", w.Body.String())
	}

	// Saving the masked value back keeps the stored secret.
	masked := strings.Replace(body, "s3cret", secretMask, 1)
	if w := do(t, router, "PUT", "/api/v1/settings/oidc", admin, masked); w.Code != http.StatusOK {
		t.Fatalf("resave: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, router, "GET", "/api/v1/auth/config", "", ""); !strings.Contains(w.Body.String(), `"oidcEnabled":true`) {
		t.Errorf("auth config should report OIDC: %s", w.Body.String())
	}

	if w := do(t, router, "PUT", "/api/v1/settings/oidc", admin, `{"enabled":true,"issuer":"not a url","clientId":"x","redirectUrl":"https://a"}`); w.Code != http.StatusBadRequest {
		t.Errorf("invalid issuer: got %d, want 400", w.Code)
	}
}
