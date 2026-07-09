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
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
	} {
		t.Setenv(key, "")
	}
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
	if file.Auth == nil {
		t.Fatal("expected auth section in config file")
	}
	if !strings.HasPrefix(file.Auth.AdminPasswordHash, "$2") {
		t.Errorf("expected bcrypt hash, got %q", file.Auth.AdminPasswordHash)
	}
	if file.Auth.AdminPasswordSalt != "" {
		t.Errorf("expected salt to be cleared, got %q", file.Auth.AdminPasswordSalt)
	}

	svc := auth.NewServiceFromFileConfig(file.Auth)
	if svc == nil {
		t.Fatal("failed to create auth service from written config")
	}
	if err := svc.ValidateCredentials("admin", "newpass123"); err != nil {
		t.Errorf("new password should validate against the bcrypt hash: %v", err)
	}
}

func TestExistingSHA256ConfigStillValidates(t *testing.T) {
	svc := newTestAuthService(t) // configured with a SHA256 hash + salt
	if err := svc.ValidateCredentials("admin", "password"); err != nil {
		t.Errorf("existing SHA256 config should keep validating: %v", err)
	}
}
