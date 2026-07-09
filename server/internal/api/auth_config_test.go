package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getAuthConfig(t *testing.T, router http.Handler) map[string]bool {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return body
}

func TestAuthConfigEnabled(t *testing.T) {
	router := newTestRouter(t, newTestAuthService(t))

	body := getAuthConfig(t, router)
	if !body["authEnabled"] {
		t.Error("expected authEnabled to be true when auth service is configured")
	}
}

func TestAuthConfigDisabled(t *testing.T) {
	router := newTestRouter(t, nil)

	body := getAuthConfig(t, router)
	if body["authEnabled"] {
		t.Error("expected authEnabled to be false when auth service is nil")
	}
}
