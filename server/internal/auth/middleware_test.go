package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func testService(t *testing.T) *Service {
	t.Helper()
	svc := NewServiceFromFileConfig(&config.FileAuthConfig{
		Enabled:           true,
		JWTSecret:         "test-secret",
		AdminUsername:     "admin",
		AdminPasswordHash: HashPasswordSHA256("password", "salt"),
		AdminPasswordSalt: "salt",
	})
	if svc == nil {
		t.Fatal("failed to create auth service")
	}
	return svc
}

// echoUserHandler records the user set on the request context.
func echoUserHandler(gotUser *models.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := r.Context().Value(UserContextKey).(models.User); ok {
			*gotUser = u
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestDynamicMiddlewareAcceptsAPIToken(t *testing.T) {
	svc := testService(t)
	token, hash, _, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken failed: %v", err)
	}

	lookup := func(presented string) (string, bool) {
		if HashAPIToken(presented) == hash {
			return "ci", true
		}
		return "", false
	}

	var gotUser models.User
	handler := DynamicMiddleware(func() *Service { return svc }, lookup)(echoUserHandler(&gotUser))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid API token, got %d: %s", w.Code, w.Body.String())
	}
	if gotUser.Username != "token:ci" {
		t.Errorf("expected context user %q, got %q", "token:ci", gotUser.Username)
	}
}

func TestDynamicMiddlewareRejectsUnknownAPIToken(t *testing.T) {
	svc := testService(t)
	lookup := func(string) (string, bool) { return "", false }

	var gotUser models.User
	handler := DynamicMiddleware(func() *Service { return svc }, lookup)(echoUserHandler(&gotUser))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer ldk_unknown-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown API token, got %d", w.Code)
	}
}

func TestDynamicMiddlewareStillAcceptsJWT(t *testing.T) {
	svc := testService(t)
	lookup := func(string) (string, bool) { return "", false }

	jwtToken, err := svc.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	var gotUser models.User
	handler := DynamicMiddleware(func() *Service { return svc }, lookup)(echoUserHandler(&gotUser))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+jwtToken)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid JWT, got %d: %s", w.Code, w.Body.String())
	}
	if gotUser.Username != "admin" {
		t.Errorf("expected context user %q, got %q", "admin", gotUser.Username)
	}
}

func TestDynamicMiddlewarePassesThroughWhenAuthDisabled(t *testing.T) {
	var gotUser models.User
	handler := DynamicMiddleware(func() *Service { return nil }, nil)(echoUserHandler(&gotUser))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with auth disabled, got %d", w.Code)
	}
}
