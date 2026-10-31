package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
)

func imageUpdateRequest(t *testing.T, readOnly bool, labels map[string]string) (*httptest.ResponseRecorder, int) {
	t.Helper()
	mutations := 0
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_ping":
			w.Header().Set("API-Version", "1.47")
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "web", "Name": "/web", "Image": "sha256:old", "State": map[string]any{"Running": true}, "HostConfig": map[string]any{}, "Config": map[string]any{"Image": "nginx", "Labels": labels}})
		default:
			mutations++
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(engine.Close)
	t.Setenv("DOCKER_API_VERSION", "1.47")
	dc, err := docker.NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dc.Close)
	ar := &APIRouter{registry: services.NewRegistry(dc, nil, nil, &config.Config{ReadOnly: readOnly})}
	router := chi.NewRouter()
	router.Route("/api/v1", ar.registerContainerRoutes)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/containers/web/image/update?host=local", nil))
	return w, mutations
}

func TestUpdateContainerImageBlockedInReadOnlyMode(t *testing.T) {
	w, mutations := imageUpdateRequest(t, true, nil)
	if w.Code != http.StatusForbidden || mutations != 0 {
		t.Fatalf("status = %d with %d engine calls, want 403 and none", w.Code, mutations)
	}
}

func TestUpdateContainerImageExplainsCompose(t *testing.T) {
	w, mutations := imageUpdateRequest(t, false, map[string]string{"com.docker.compose.project": "shop"})
	if w.Code != http.StatusConflict || mutations != 0 {
		t.Fatalf("status = %d with %d engine calls, want 409 and none", w.Code, mutations)
	}
	if body := w.Body.String(); !strings.Contains(body, "docker compose pull") {
		t.Errorf("body = %q, want the Compose commands", body)
	}
}

func TestGetContainersReportsReadOnlyToReadUsers(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.47")
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(engine.Close)
	t.Setenv("DOCKER_API_VERSION", "1.47")
	dc, err := docker.NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dc.Close)
	ar := &APIRouter{registry: services.NewRegistry(dc, nil, nil, &config.Config{})}

	for role, want := range map[string]bool{auth.RoleAdmin: false, auth.RoleRead: true} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
		r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, models.User{Username: "someone", Role: role}))
		w := httptest.NewRecorder()
		ar.GetContainers(w, r)
		var body struct {
			ReadOnly bool `json:"readOnly"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %d %s", role, w.Code, w.Body.String())
		}
		if body.ReadOnly != want {
			t.Errorf("%s: readOnly = %v, want %v", role, body.ReadOnly, want)
		}
	}
}
