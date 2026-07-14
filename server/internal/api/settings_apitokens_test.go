package api

import (
	"encoding/json"
	"fmt"
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

type createdTokenResponse struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"createdAt"`
	Scope     string `json:"scope"`
}

func createToken(t *testing.T, router http.Handler, jwt, name string) createdTokenResponse {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(fmt.Sprintf(`{"name":%q}`, name)))
	if jwt != "" {
		r.Header.Set("Authorization", "Bearer "+jwt)
	}
	router.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating token, got %d: %s", w.Code, w.Body.String())
	}
	var resp createdTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	return resp
}

func TestCreateAndListAPITokens(t *testing.T) {
	router := newTestRouter(t, nil)

	created := createToken(t, router, "", "my-cli")
	if !strings.HasPrefix(created.Token, "ldk_") {
		t.Errorf("token %q does not start with ldk_", created.Token)
	}
	if created.Prefix != created.Token[:12] {
		t.Errorf("prefix %q does not match token start", created.Prefix)
	}
	if created.Name != "my-cli" || created.CreatedAt == "" {
		t.Errorf("unexpected create response: %+v", created)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/settings/api-tokens", nil)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing tokens, got %d: %s", w.Code, w.Body.String())
	}

	var list struct {
		Tokens []map[string]any `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(list.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(list.Tokens))
	}
	entry := list.Tokens[0]
	if entry["name"] != "my-cli" || entry["prefix"] != created.Prefix {
		t.Errorf("unexpected list entry: %v", entry)
	}
	if entry["scope"] != "admin" {
		t.Errorf("expected default scope %q, got %v", "admin", entry["scope"])
	}
	if _, hasHash := entry["hash"]; hasHash {
		t.Error("list response must not include the token hash")
	}
	if _, hasToken := entry["token"]; hasToken {
		t.Error("list response must not include the full token")
	}
}

func TestCreateAPITokenValidation(t *testing.T) {
	router := newTestRouter(t, nil)
	createToken(t, router, "", "dup")

	cases := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":""}`},
		{"whitespace name", `{"name":"   "}`},
		{"duplicate name", `{"name":"dup"}`},
		{"name too long", fmt.Sprintf(`{"name":%q}`, strings.Repeat("a", 65))},
		{"invalid json", `{`},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens", strings.NewReader(tc.body))
		router.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}
}

func TestCreateAPITokenCap(t *testing.T) {
	router := newTestRouter(t, nil)
	for i := 0; i < 20; i++ {
		createToken(t, router, "", fmt.Sprintf("token-%d", i))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens", strings.NewReader(`{"name":"one-too-many"}`))
	router.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when exceeding token cap, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteAPIToken(t *testing.T) {
	router := newTestRouter(t, nil)
	created := createToken(t, router, "", "to-delete")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/settings/api-tokens/"+created.Prefix, nil)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting token, got %d: %s", w.Code, w.Body.String())
	}

	// Deleting again should 404.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("DELETE", "/api/v1/settings/api-tokens/"+created.Prefix, nil)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 deleting unknown token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPITokenAuthenticatesRequestsWhenAuthEnabled(t *testing.T) {
	svc := newTestAuthService(t)
	router := newTestRouter(t, svc)

	jwt, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	created := createToken(t, router, jwt, "cli")

	// The API token must authenticate protected routes.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with API token, got %d: %s", w.Code, w.Body.String())
	}
	var me struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("failed to parse /auth/me response: %v", err)
	}
	if me.User.Username != "token:cli" {
		t.Errorf("expected user %q, got %q", "token:cli", me.User.Username)
	}

	// Settings routes accept it too.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/settings", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on settings with API token, got %d: %s", w.Code, w.Body.String())
	}

	// A revoked token must stop working immediately.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("DELETE", "/api/v1/settings/api-tokens/"+created.Prefix, nil)
	r.Header.Set("Authorization", "Bearer "+jwt)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 revoking token, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with revoked API token, got %d", w.Code)
	}

	// A random ldk_ token must not authenticate.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer ldk_not-a-real-token")
	router.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with bogus API token, got %d", w.Code)
	}
}

func TestCreateAPITokenScopes(t *testing.T) {
	router := newTestRouter(t, nil)

	// A read-scoped token is created and listed with its scope.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"agent","scope":"read"}`))
	router.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating read token, got %d: %s", w.Code, w.Body.String())
	}
	var created createdTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}
	if created.Scope != "read" {
		t.Errorf("expected scope %q in create response, got %q", "read", created.Scope)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/settings/api-tokens", nil)
	router.ServeHTTP(w, r)
	var list struct {
		Tokens []map[string]any `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(list.Tokens) != 1 || list.Tokens[0]["scope"] != "read" {
		t.Errorf("expected listed token with scope read, got %v", list.Tokens)
	}

	// An invalid scope is rejected.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"bad","scope":"superuser"}`))
	router.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid scope, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReadScopeAPITokenEnforcement(t *testing.T) {
	svc := newTestAuthService(t)
	router := newTestRouter(t, svc)

	jwt, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"agent","scope":"read"}`))
	r.Header.Set("Authorization", "Bearer "+jwt)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating read token, got %d: %s", w.Code, w.Body.String())
	}
	var created createdTokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}

	// Plain read routes are allowed.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on read route with read token, got %d: %s", w.Code, w.Body.String())
	}

	// A container that happens to be named "exec" is a plain read: the
	// request passes auth and fails on the missing host param (400), not 403.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/v1/containers/exec", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on container named exec with read token, got %d: %s", w.Code, w.Body.String())
	}

	// Mutating routes are denied.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"another"}`))
	r.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 on mutating route with read token, got %d: %s", w.Code, w.Body.String())
	}

	// GET routes that are sensitive or mutating are denied too.
	deniedReads := []string{
		"/api/v1/containers/abc123/exec",
		"/api/v1/containers/abc123/env",
		"/api/v1/settings",
		"/api/v1/settings/api-tokens",
	}
	for _, path := range deniedReads {
		w = httptest.NewRecorder()
		r = httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+created.Token)
		router.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("GET %s: expected 403 with read token, got %d: %s", path, w.Code, w.Body.String())
		}
	}

	// Admin credentials (JWT session and admin token) are unaffected on the
	// routes denied to read tokens.
	adminToken := createToken(t, router, jwt, "full-access")
	for _, bearer := range []string{jwt, adminToken.Token} {
		w = httptest.NewRecorder()
		r = httptest.NewRequest("GET", "/api/v1/settings", nil)
		r.Header.Set("Authorization", "Bearer "+bearer)
		router.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 on settings with admin credentials, got %d: %s", w.Code, w.Body.String())
		}

		// Passes auth; fails on the missing host param, not authorization.
		w = httptest.NewRecorder()
		r = httptest.NewRequest("GET", "/api/v1/containers/abc123/env", nil)
		r.Header.Set("Authorization", "Bearer "+bearer)
		router.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on env route with admin credentials, got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestLegacyAPITokenWithoutScopeIsAdmin(t *testing.T) {
	token, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken failed: %v", err)
	}

	// Simulate a config written before scopes existed: no scope field.
	for _, key := range []string{
		"JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "ADMIN_PASSWORD_SALT",
		"DOCKER_HOSTS", "COOLIFY_CONFIGS", "READONLY_MODE", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	cfg := fmt.Sprintf(
		`{"apiTokens":[{"name":"legacy","hash":%q,"prefix":%q,"createdAt":"2026-01-01T00:00:00Z"}]}`,
		hash, prefix)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Setenv("CONFIG_PATH", cfgPath)

	svc := newTestAuthService(t)
	manager := config.NewManager()
	registry := services.NewRegistry(nil, nil, svc, manager.Config())
	router := NewRouter(registry, manager, nil, nil, "test")

	// The legacy token is listed with an admin scope.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/settings/api-tokens", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing tokens with legacy token, got %d: %s", w.Code, w.Body.String())
	}
	var list struct {
		Tokens []map[string]any `json:"tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(list.Tokens) != 1 || list.Tokens[0]["scope"] != "admin" {
		t.Errorf("expected legacy token listed with scope admin, got %v", list.Tokens)
	}

	// The legacy token can still perform mutating operations.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/api/v1/settings/api-tokens",
		strings.NewReader(`{"name":"created-by-legacy"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 creating token with legacy token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPITokenEndpointsRequireAuthWhenEnabled(t *testing.T) {
	router := newTestRouter(t, newTestAuthService(t))

	endpoints := []struct{ method, path string }{
		{"GET", "/api/v1/settings/api-tokens"},
		{"POST", "/api/v1/settings/api-tokens"},
		{"DELETE", "/api/v1/settings/api-tokens/ldk_abcd1234"},
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
