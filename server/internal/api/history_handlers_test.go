package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/services"

	_ "modernc.org/sqlite"
)

var historyBase = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

// newHistoryTestRouter builds a router around the given store (nil means log
// persistence is disabled) with auth off.
func newHistoryTestRouter(t *testing.T, store *logstore.Store) http.Handler {
	t.Helper()
	return newHistoryTestRouterWithAuth(t, store, nil)
}

func newHistoryTestRouterWithAuth(t *testing.T, store *logstore.Store, authSvc *auth.Service) http.Handler {
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
	engine := alerts.NewEngine(registry, manager, nil)
	return NewRouter(registry, manager, engine, store, "test")
}

// newHistoryStore opens a real store over a temp database and returns it with
// a seeding function. The store's writer is unexported, so tests seed rows
// through a second connection to the same database — the schema is the store's
// own, created by Open.
func newHistoryStore(t *testing.T) (*logstore.Store, func(host, containerID, name string, ts time.Time, message string)) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs.db")
	store, err := logstore.Open(path, func() config.ResolvedLogStoreConfig {
		return config.ResolvedLogStoreConfig{Enabled: true, PerContainerMB: 50, TotalMB: 1024}
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	seed := func(host, containerID, name string, ts time.Time, message string) {
		t.Helper()
		var ref int64
		err := db.QueryRow(
			"SELECT id FROM containers WHERE host = ? AND container_id = ?", host, containerID,
		).Scan(&ref)
		if err == sql.ErrNoRows {
			res, insertErr := db.Exec(`
				INSERT INTO containers (host, container_id, name, image, first_seen_ms, last_seen_ms)
				VALUES (?, ?, ?, ?, ?, ?)`,
				host, containerID, name, "nginx:latest",
				ts.UnixMilli(), ts.UnixMilli())
			if insertErr != nil {
				t.Fatalf("insert container: %v", insertErr)
			}
			ref, _ = res.LastInsertId()
		} else if err != nil {
			t.Fatalf("lookup container: %v", err)
		}

		raw := ts.UTC().Format(time.RFC3339Nano) + " " + message
		entry := models.ParseLogLine(raw, "stdout")
		if _, err := db.Exec(
			"INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw) VALUES (?, ?, 0, ?, ?)",
			ref, ts.UnixNano(), models.LevelSeverity(entry.Level), raw,
		); err != nil {
			t.Fatalf("insert line: %v", err)
		}
	}

	return store, seed
}

func doHistoryRequest(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

type historyLogsResponse struct {
	Logs       []models.LogEntry `json:"logs"`
	NextCursor string            `json:"nextCursor"`
	Count      int               `json:"count"`
}

func TestHistoryStatus(t *testing.T) {
	store, _ := newHistoryStore(t)

	for _, tt := range []struct {
		name  string
		store *logstore.Store
		want  bool
	}{
		{"enabled", store, true},
		{"disabled", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := doHistoryRequest(t, newHistoryTestRouter(t, tt.store), "/api/v1/history/status")
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var body struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("parse response: %v", err)
			}
			if body.Enabled != tt.want {
				t.Fatalf("expected enabled=%v, got %v", tt.want, body.Enabled)
			}
		})
	}
}

// TestHistoryStatusStorage covers the storage fields the frontend renders next
// to the retention caps.
func TestHistoryStatusStorage(t *testing.T) {
	t.Run("enabled reports size and caps", func(t *testing.T) {
		store, seed := newHistoryStore(t)
		seed("local", "abc123", "web", historyBase, "hello")

		w := doHistoryRequest(t, newHistoryTestRouter(t, store), "/api/v1/history/status")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Enabled        bool  `json:"enabled"`
			DBSizeBytes    int64 `json:"dbSizeBytes"`
			PerContainerMB int   `json:"perContainerMB"`
			TotalMB        int   `json:"totalMB"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if !body.Enabled {
			t.Fatal("expected enabled=true")
		}
		if body.DBSizeBytes <= 0 {
			t.Fatalf("expected a positive dbSizeBytes, got %d", body.DBSizeBytes)
		}
		// The manager's defaults, since the test config file sets no caps.
		if body.PerContainerMB != config.DefaultLogStorePerContainerMB || body.TotalMB != config.DefaultLogStoreTotalMB {
			t.Fatalf("expected the configured caps, got perContainerMB=%d totalMB=%d",
				body.PerContainerMB, body.TotalMB)
		}
	})

	t.Run("disabled reports enabled only", func(t *testing.T) {
		w := doHistoryRequest(t, newHistoryTestRouter(t, nil), "/api/v1/history/status")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if len(body) != 1 || body["enabled"] != false {
			t.Fatalf("expected {\"enabled\": false} and nothing else, got %v", body)
		}
	})
}

func TestHistoryContainers(t *testing.T) {
	t.Run("empty store returns [] not null", func(t *testing.T) {
		store, _ := newHistoryStore(t)
		w := doHistoryRequest(t, newHistoryTestRouter(t, store), "/api/v1/history/containers")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var raw struct {
			Containers json.RawMessage `json:"containers"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if string(raw.Containers) != "[]" {
			t.Fatalf("expected [], got %s", raw.Containers)
		}
	})

	t.Run("nil store returns empty list", func(t *testing.T) {
		w := doHistoryRequest(t, newHistoryTestRouter(t, nil), "/api/v1/history/containers")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var raw struct {
			Containers json.RawMessage `json:"containers"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if string(raw.Containers) != "[]" {
			t.Fatalf("expected [], got %s", raw.Containers)
		}
	})

	t.Run("lists stored containers", func(t *testing.T) {
		store, seed := newHistoryStore(t)
		seed("local", "abc123", "web", historyBase, "hello")
		seed("local", "def456", "db", historyBase, "ready")

		w := doHistoryRequest(t, newHistoryTestRouter(t, store), "/api/v1/history/containers")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Containers []logstore.StoredContainer `json:"containers"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if len(body.Containers) != 2 {
			t.Fatalf("expected 2 containers, got %d", len(body.Containers))
		}
		// Sorted by host then name.
		if body.Containers[0].Name != "db" || body.Containers[1].Name != "web" {
			t.Fatalf("unexpected order: %+v", body.Containers)
		}
		if body.Containers[0].Host != "local" {
			t.Fatalf("expected host local, got %q", body.Containers[0].Host)
		}
	})
}

func TestHistoryLogsValidation(t *testing.T) {
	store, seed := newHistoryStore(t)
	seed("local", "abc123", "web", historyBase, "hello")
	router := newHistoryTestRouter(t, store)

	for _, tt := range []struct {
		name string
		path string
	}{
		{"missing container", "/api/v1/history/logs"},
		{"blank container", "/api/v1/history/logs?container=%20"},
		{"unknown level", "/api/v1/history/logs?container=web&levels=ERROR,BOGUS"},
		{"invalid regex", "/api/v1/history/logs?container=web&search=%5B&regex=true"},
		{"non-integer limit", "/api/v1/history/logs?container=web&limit=lots"},
		{"invalid cursor", "/api/v1/history/logs?container=web&cursor=not-a-cursor"},
		{"invalid since", "/api/v1/history/logs?container=web&since=yesterday"},
		{"invalid until", "/api/v1/history/logs?container=web&until=yesterday"},
		{"invalid regex flag", "/api/v1/history/logs?container=web&regex=maybe"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := doHistoryRequest(t, router, tt.path)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHistoryLogsDisabled(t *testing.T) {
	w := doHistoryRequest(t, newHistoryTestRouter(t, nil), "/api/v1/history/logs?container=web")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if body.Error != "log persistence is disabled" {
		t.Fatalf("unexpected error message: %q", body.Error)
	}
}

func TestHistoryLogsQuery(t *testing.T) {
	store, seed := newHistoryStore(t)
	seed("local", "abc123", "web", historyBase, "starting up")
	seed("local", "abc123", "web", historyBase.Add(time.Second), "ERROR database is down")
	seed("local", "abc123", "web", historyBase.Add(2*time.Second), "recovered")
	seed("local", "xyz789", "db", historyBase, "db line")
	router := newHistoryTestRouter(t, store)

	t.Run("returns entries for the container only", func(t *testing.T) {
		body := historyLogs(t, router, "/api/v1/history/logs?container=web")
		if body.Count != 3 || len(body.Logs) != 3 {
			t.Fatalf("expected 3 logs, got %d: %+v", body.Count, body.Logs)
		}
		if body.Logs[0].Message != "starting up" {
			t.Fatalf("expected ascending order, got %q first", body.Logs[0].Message)
		}
		if body.NextCursor != "" {
			t.Fatalf("expected no cursor at end of history, got %q", body.NextCursor)
		}
	})

	t.Run("empty result is [] not null", func(t *testing.T) {
		w := doHistoryRequest(t, router, "/api/v1/history/logs?container=nosuch")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var raw struct {
			Logs json.RawMessage `json:"logs"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if string(raw.Logs) != "[]" {
			t.Fatalf("expected [], got %s", raw.Logs)
		}
	})

	t.Run("level filter", func(t *testing.T) {
		body := historyLogs(t, router, "/api/v1/history/logs?container=web&levels=ERROR")
		if len(body.Logs) != 1 || body.Logs[0].Level != models.LogLevelError {
			t.Fatalf("expected one ERROR line, got %+v", body.Logs)
		}
	})

	t.Run("regex search", func(t *testing.T) {
		body := historyLogs(t, router, "/api/v1/history/logs?container=web&search=data(base)?&regex=true")
		if len(body.Logs) != 1 {
			t.Fatalf("expected one match, got %d", len(body.Logs))
		}
	})

	t.Run("since and until", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/history/logs?container=web&since=%s&until=%s",
			historyBase.Add(time.Second).Format(time.RFC3339),
			historyBase.Add(time.Second).Format(time.RFC3339))
		body := historyLogs(t, router, path)
		if len(body.Logs) != 1 || body.Logs[0].Message != "ERROR database is down" {
			t.Fatalf("expected the single in-range line, got %+v", body.Logs)
		}
	})

	t.Run("cursor pages backwards through history", func(t *testing.T) {
		first := historyLogs(t, router, "/api/v1/history/logs?container=web&limit=2")
		if len(first.Logs) != 2 || first.NextCursor == "" {
			t.Fatalf("expected a full page and a cursor, got %d logs, cursor %q", len(first.Logs), first.NextCursor)
		}
		// Page one is the newest slice of history.
		if first.Logs[1].Message != "recovered" {
			t.Fatalf("expected newest line last on page one, got %q", first.Logs[1].Message)
		}

		second := historyLogs(t, router, "/api/v1/history/logs?container=web&limit=2&cursor="+first.NextCursor)
		if len(second.Logs) != 1 || second.Logs[0].Message != "starting up" {
			t.Fatalf("expected the oldest line on page two, got %+v", second.Logs)
		}
		if second.NextCursor != "" {
			t.Fatalf("expected no cursor at end of history, got %q", second.NextCursor)
		}
	})
}

func TestHistoryRequiresAuth(t *testing.T) {
	store, _ := newHistoryStore(t)
	router := newHistoryTestRouterWithAuth(t, store, newTestAuthService(t))

	for _, path := range []string{
		"/api/v1/history/status",
		"/api/v1/history/containers",
		"/api/v1/history/logs?container=web",
	} {
		w := doHistoryRequest(t, router, path)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s, got %d: %s", path, w.Code, w.Body.String())
		}
	}
}

// doHistoryDelete issues a purge request, optionally authenticated.
func doHistoryDelete(t *testing.T, router http.Handler, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", path, nil)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	router.ServeHTTP(w, r)
	return w
}

func TestDeleteHistoryContainer(t *testing.T) {
	t.Run("purges the container and reports the line count", func(t *testing.T) {
		store, seed := newHistoryStore(t)
		seed("local", "abc123", "web", historyBase, "first")
		seed("local", "def456", "web", historyBase.Add(time.Second), "second generation")
		seed("local", "xyz789", "db", historyBase, "db line")
		router := newHistoryTestRouter(t, store)

		w := doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", "")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Message      string `json:"message"`
			LinesDeleted int64  `json:"linesDeleted"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if body.Message != "stored logs deleted" {
			t.Fatalf("unexpected message: %q", body.Message)
		}
		if body.LinesDeleted != 2 {
			t.Fatalf("expected 2 lines deleted (both generations), got %d", body.LinesDeleted)
		}

		// The purged container is gone; the other one is untouched.
		if logs := historyLogs(t, router, "/api/v1/history/logs?container=web"); logs.Count != 0 {
			t.Fatalf("expected no stored logs for web, got %d", logs.Count)
		}
		if logs := historyLogs(t, router, "/api/v1/history/logs?container=db"); logs.Count != 1 {
			t.Fatalf("expected db to keep its line, got %d", logs.Count)
		}

		// Purging again finds nothing left.
		if w := doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", ""); w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on the second purge, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("host is required", func(t *testing.T) {
		store, seed := newHistoryStore(t)
		seed("local", "abc123", "web", historyBase, "hello")
		router := newHistoryTestRouter(t, store)

		for _, path := range []string{
			"/api/v1/history/containers/web",
			"/api/v1/history/containers/web?host=",
			"/api/v1/history/containers/web?host=%20",
		} {
			if w := doHistoryDelete(t, router, path, ""); w.Code != http.StatusBadRequest {
				t.Fatalf("DELETE %s: expected 400, got %d: %s", path, w.Code, w.Body.String())
			}
		}

		// The container survived every rejected request.
		if logs := historyLogs(t, router, "/api/v1/history/logs?container=web"); logs.Count != 1 {
			t.Fatalf("expected the container to be untouched, got %d logs", logs.Count)
		}
	})

	t.Run("unknown container", func(t *testing.T) {
		store, seed := newHistoryStore(t)
		seed("local", "abc123", "web", historyBase, "hello")
		router := newHistoryTestRouter(t, store)

		for _, path := range []string{
			"/api/v1/history/containers/nosuch?host=local",
			"/api/v1/history/containers/web?host=nosuch", // right name, wrong host
		} {
			if w := doHistoryDelete(t, router, path, ""); w.Code != http.StatusNotFound {
				t.Fatalf("DELETE %s: expected 404, got %d: %s", path, w.Code, w.Body.String())
			}
		}
	})

	t.Run("persistence disabled", func(t *testing.T) {
		w := doHistoryDelete(t, newHistoryTestRouter(t, nil), "/api/v1/history/containers/web?host=local", "")
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if body.Error != "log persistence is disabled" {
			t.Fatalf("unexpected error message: %q", body.Error)
		}
	})
}

func TestDeleteHistoryContainerRequiresAuth(t *testing.T) {
	store, seed := newHistoryStore(t)
	seed("local", "abc123", "web", historyBase, "hello")
	router := newHistoryTestRouterWithAuth(t, store, newTestAuthService(t))

	w := doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteHistoryContainerDeniesReadScope proves the purge route is closed to
// read-scoped API tokens, which the read-only history queries are open to.
func TestDeleteHistoryContainerDeniesReadScope(t *testing.T) {
	store, seed := newHistoryStore(t)
	seed("local", "abc123", "web", historyBase, "hello")

	svc := newTestAuthService(t)
	router := newHistoryTestRouterWithAuth(t, store, svc)

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
		t.Fatalf("expected 201 creating read token, got %d: %s", w.Code, w.Body.String())
	}
	var readToken createdTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &readToken); err != nil {
		t.Fatalf("parse create response: %v", err)
	}

	// The read token can query stored logs but not purge them.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/history/logs?container=web", nil)
	r.Header.Set("Authorization", "Bearer "+readToken.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 querying history with a read token, got %d: %s", w.Code, w.Body.String())
	}

	w = doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", readToken.Token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 purging with a read token, got %d: %s", w.Code, w.Body.String())
	}

	// A JWT session and an admin token are unaffected.
	if w := doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", jwt); w.Code != http.StatusOK {
		t.Fatalf("expected 200 purging with an admin session, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteHistoryContainerReadOnlyMode covers the global read-only switch:
// purging stored logs is destructive, so it is blocked exactly like removing a
// container.
func TestDeleteHistoryContainerReadOnlyMode(t *testing.T) {
	store, seed := newHistoryStore(t)
	seed("local", "abc123", "web", historyBase, "hello")

	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("READONLY_MODE", "true")
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, nil, manager.Config())
	router := NewRouter(registry, manager, alerts.NewEngine(registry, manager, nil), store, "test")

	w := doHistoryDelete(t, router, "/api/v1/history/containers/web?host=local", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 in read-only mode, got %d: %s", w.Code, w.Body.String())
	}

	// Reads still work, and the container is still there.
	if logs := historyLogs(t, router, "/api/v1/history/logs?container=web"); logs.Count != 1 {
		t.Fatalf("expected the container to survive read-only mode, got %d logs", logs.Count)
	}
}

func historyLogs(t *testing.T, router http.Handler, path string) historyLogsResponse {
	t.Helper()
	w := doHistoryRequest(t, router, path)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body historyLogsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return body
}

// TestAlertRoutesDenyReadScope proves the alert routes are closed to read-scoped
// API tokens. The webhook URL is a secret — a Slack or Discord webhook lets its
// holder post to the channel — so a token handed to a CI job or an AI agent must
// not be able to read it back.
func TestAlertRoutesDenyReadScope(t *testing.T) {
	store, _ := newHistoryStore(t)
	svc := newTestAuthService(t)
	router := newHistoryTestRouterWithAuth(t, store, svc)

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
		t.Fatalf("expected 201 creating read token, got %d: %s", w.Code, w.Body.String())
	}
	var readToken createdTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &readToken); err != nil {
		t.Fatalf("parse create response: %v", err)
	}

	for _, path := range []string{
		"/api/v1/alerts/webhook",
		"/api/v1/alerts/rules",
		"/api/v1/alerts/history",
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+readToken.Token)
		router.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 on %s with a read token, got %d: %s", path, w.Code, w.Body.String())
		}
	}

	// An admin session still reaches them.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/alerts/webhook", nil)
	r.Header.Set("Authorization", "Bearer "+jwt)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on the webhook with an admin session, got %d: %s", w.Code, w.Body.String())
	}
}
