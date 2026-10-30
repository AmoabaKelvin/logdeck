package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

func TestUpdateAuthWritesBcryptHash(t *testing.T) {
	clearEnvForAuthTest(t)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", cfgPath)

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, nil, manager.Config())
	ar := &APIRouter{registry: registry, manager: manager}

	body := `{"enabled":true,"adminUsername":"admin","newPassword":"newpass123"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auth", strings.NewReader(body))
	ar.UpdateAuth(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	var file struct {
		Auth *config.FileAuthConfig `json:"auth"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("failed to decode config file: %v", err)
	}
	if file.Auth == nil || len(file.Auth.Users) != 1 {
		t.Fatalf("expected one user in the auth section, got %+v", file.Auth)
	}
	if u := file.Auth.Users[0]; !strings.HasPrefix(u.PasswordHash, "$2") || u.PasswordSalt != "" || u.Role != auth.RoleAdmin {
		t.Errorf("expected a bcrypt-hashed admin, got %+v", u)
	}

	svc, err := auth.NewService(file.Auth, nil)
	if err != nil || svc == nil {
		t.Fatalf("failed to create auth service from written config: %v", err)
	}
	if _, err := svc.ValidateCredentials("admin", "newpass123"); err != nil {
		t.Errorf("new password should validate against the bcrypt hash: %v", err)
	}
}

func TestExistingSHA256ConfigStillValidates(t *testing.T) {
	svc := newTestAuthService(t) // configured with a SHA256 hash + salt
	if _, err := svc.ValidateCredentials("admin", "password"); err != nil {
		t.Errorf("existing SHA256 config should keep validating: %v", err)
	}
}

func TestUpdateAuthFirstAdmin(t *testing.T) {
	clearEnvForAuthTest(t)
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, nil, manager.Config())
	ar := &APIRouter{registry: registry, manager: manager}

	save := func(body string) int {
		w := httptest.NewRecorder()
		ar.UpdateAuth(w, httptest.NewRequest(http.MethodPut, "/api/v1/settings/auth", strings.NewReader(body)))
		return w.Code
	}

	if code := save(`{"enabled":true}`); code != http.StatusBadRequest {
		t.Errorf("enabling with no admin: got %d, want 400", code)
	}
	if code := save(`{"enabled":true,"adminUsername":"admin","newPassword":"password-one"}`); code != http.StatusOK {
		t.Fatalf("enabling with an admin: got %d", code)
	}
	first := manager.FileConfigSnapshot().Auth.JWTSecret
	if code := save(`{"enabled":false}`); code != http.StatusOK {
		t.Fatalf("disabling: got %d", code)
	}
	if code := save(`{"enabled":true}`); code != http.StatusOK {
		t.Errorf("re-enabling with the existing admin: got %d", code)
	}
	if kept := manager.FileConfigSnapshot().Auth.JWTSecret; kept != first {
		t.Error("toggling auth should keep the JWT secret")
	}
}

// A config written by a version with a single admin keeps its login: the
// admin becomes the first user, and the file is rewritten that way.
func TestLegacyAdminMigratesToFirstUser(t *testing.T) {
	clearEnvForAuthTest(t)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", cfgPath)
	legacy := `{"auth":{"enabled":true,"jwtSecret":"s","adminUsername":"admin","adminPasswordHash":"` +
		auth.HashPasswordSHA256("password", "salt") + `","adminPasswordSalt":"salt"}}`
	if err := os.WriteFile(cfgPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := config.NewManager()
	fa := manager.FileConfigSnapshot().Auth
	if fa.AdminUsername != "" || len(fa.Users) != 1 || fa.Users[0].Username != "admin" || fa.Users[0].Role != auth.RoleAdmin {
		t.Fatalf("expected the admin moved into users, got %+v", fa)
	}
	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(raw), "adminUsername") {
		t.Errorf("the migration should be persisted, file is %s", raw)
	}

	svc, err := auth.NewService(fa, nil)
	if err != nil || svc == nil {
		t.Fatalf("NewService: %v", err)
	}
	if user, err := svc.ValidateCredentials("admin", "password"); err != nil || user.Role != auth.RoleAdmin {
		t.Errorf("legacy admin sign-in = %+v, %v", user, err)
	}
}
