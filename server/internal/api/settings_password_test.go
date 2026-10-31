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
	// Older clients renamed the admin by sending a new name alone.
	if code := save(`{"enabled":true,"adminUsername":"renamed"}`); code != http.StatusBadRequest {
		t.Errorf("a new adminUsername without a password: got %d, want 400", code)
	}
}

func readFileAuth(t *testing.T, path string) config.FileAuthConfig {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Auth config.FileAuthConfig `json:"auth"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.Auth
}

// The saved config keeps a legacy admin in step with the users, so an older
// version rolled back to still requires login.
func TestLegacyAdminMirrorsFirstAdmin(t *testing.T) {
	router := newUsersTestRouter(t)
	cfgPath := os.Getenv("CONFIG_PATH")
	admin := login(t, router, "admin", "admin-password")

	do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"viewer","password":"viewer-password","role":"read"}`)
	do(t, router, "POST", "/api/v1/settings/users", admin, `{"username":"second","password":"second-password","role":"admin"}`)
	if saved := readFileAuth(t, cfgPath); saved.AdminUsername != "admin" || saved.AdminPasswordHash != saved.Users[0].PasswordHash {
		t.Fatalf("legacy admin should mirror the first admin, got %+v", saved)
	}

	if w := do(t, router, "PUT", "/api/v1/settings/users/admin", admin, `{"password":"changed-password"}`); w.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	if saved := readFileAuth(t, cfgPath); saved.AdminPasswordHash != saved.Users[0].PasswordHash {
		t.Error("a password reset should reach the legacy admin")
	}

	second := login(t, router, "second", "second-password")
	if w := do(t, router, "DELETE", "/api/v1/settings/users/admin", second, ""); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	saved := readFileAuth(t, cfgPath)
	var secondHash string
	for _, u := range saved.Users {
		if u.Username == "second" {
			secondHash = u.PasswordHash
		}
	}
	if saved.AdminUsername != "second" || saved.AdminPasswordHash != secondHash || secondHash == "" {
		t.Errorf("after deleting the first admin the legacy admin should be the next admin, got %+v", saved)
	}
}

// Env auth ignored the file's old admin before users existed, and still
// does: an operator who moved to env vars to replace that login must not see
// it come back.
func TestLegacyAdminIgnoredUnderEnvAuth(t *testing.T) {
	envHash, _ := auth.HashPassword("env-password")
	clearEnvForAuthTest(t)
	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("ADMIN_USERNAME", "ops")
	t.Setenv("ADMIN_PASSWORD", envHash)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", cfgPath)
	oldHash, _ := auth.HashPassword("old-password")
	legacy := `{"auth":{"enabled":true,"jwtSecret":"s","adminUsername":"admin","adminPasswordHash":"` + oldHash + `"}}`
	if err := os.WriteFile(cfgPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := config.NewManager()
	svc, err := auth.NewService(manager.FileConfigSnapshot().Auth, nil)
	if err != nil || svc == nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.ValidateCredentials("admin", "old-password"); err == nil {
		t.Error("the file's old admin must not sign in under env auth")
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
	if len(fa.Users) != 1 || fa.Users[0].Username != "admin" || fa.Users[0].Role != auth.RoleAdmin {
		t.Fatalf("expected the admin copied into users, got %+v", fa)
	}
	saved := readFileAuth(t, cfgPath)
	if len(saved.Users) != 1 || saved.AdminUsername != "admin" || saved.AdminPasswordSalt != "salt" {
		t.Errorf("the saved config should hold the user and still the legacy admin older versions read, got %+v", saved)
	}
	// The kept legacy fields are not migrated a second time.
	if again := config.NewManager().FileConfigSnapshot().Auth; len(again.Users) != 1 {
		t.Errorf("restart duplicated the admin: %+v", again.Users)
	}

	svc, err := auth.NewService(fa, nil)
	if err != nil || svc == nil {
		t.Fatalf("NewService: %v", err)
	}
	if user, err := svc.ValidateCredentials("admin", "password"); err != nil || user.Role != auth.RoleAdmin {
		t.Errorf("legacy admin sign-in = %+v, %v", user, err)
	}
}
