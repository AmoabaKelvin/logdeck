package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type createdTokenResponse struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"createdAt"`
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
