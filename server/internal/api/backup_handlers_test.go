package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/backup"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
)

func newBackupTestRouter(t *testing.T) http.Handler {
	t.Helper()
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "DOCKER_HOSTS", "COOLIFY_CONFIGS",
		"READONLY_MODE", "BACKUP_DIR", "BACKUP_PASSPHRASE",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, nil, manager.Config())
	backups := backup.NewService(manager, nil, nil, "test")
	return NewRouter(registry, manager, nil, nil, nil, backups, "test")
}

func serve(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestBackupCreateListDownloadAndRestore(t *testing.T) {
	router := newBackupTestRouter(t)

	if w := serve(router, "POST", "/api/v1/settings/backups", `{"includeSecrets":true}`); w.Code != http.StatusAccepted {
		t.Fatalf("create = %d: %s", w.Code, w.Body.String())
	}
	var status backup.Status
	deadline := time.Now().Add(5 * time.Second)
	for {
		w := serve(router, "GET", "/api/v1/settings/backups", "")
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if !status.Running && status.Last != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("backup did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status.Last.Error != "" || len(status.Backups) != 1 || status.Backups[0].Manifest == nil {
		t.Fatalf("status = %+v", status)
	}

	name := status.Backups[0].Name
	download := serve(router, "GET", "/api/v1/settings/backups/"+name, "")
	if download.Code != http.StatusOK || !strings.Contains(download.Header().Get("Content-Disposition"), name) {
		t.Fatalf("download = %d %v", download.Code, download.Header())
	}

	w := serve(router, "POST", "/api/v1/settings/backups/restore", download.Body.String())
	if w.Code != http.StatusOK {
		t.Fatalf("restore upload = %d: %s", w.Code, w.Body.String())
	}
	if w := serve(router, "POST", "/api/v1/settings/backups/restore/confirm", ""); w.Code != http.StatusOK {
		t.Fatalf("confirm = %d: %s", w.Code, w.Body.String())
	}
}

func TestBackupRoutesRejectBadInput(t *testing.T) {
	router := newBackupTestRouter(t)
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/v1/settings/backups/restore", "not a backup", http.StatusBadRequest},
		{"POST", "/api/v1/settings/backups/restore/confirm", "", http.StatusConflict},
		{"GET", "/api/v1/settings/backups/..%2Fconfig.json", "", http.StatusNotFound},
		{"DELETE", "/api/v1/settings/backups/config.json", "", http.StatusNotFound},
		{"POST", "/api/v1/settings/backups", `{"passphrase":"short"}`, http.StatusBadRequest},
		{"PUT", "/api/v1/settings/backups/schedule", `{"intervalHours":24,"keep":0}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if w := serve(router, tc.method, tc.path, tc.body); w.Code != tc.want {
			t.Errorf("%s %s = %d, want %d: %s", tc.method, tc.path, w.Code, tc.want, w.Body.String())
		}
	}
}
