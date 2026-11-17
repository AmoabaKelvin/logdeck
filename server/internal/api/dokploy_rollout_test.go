package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
)

// A deployment is followed through Dokploy's run and the Swarm update it
// causes. Dokploy says "done" before Swarm has replaced the task, and Swarm may
// roll back, so neither alone means the change is live. Runs are told apart by
// Dokploy's own clock: only those after "after" count.
func TestDokployRolloutFollowsDokployThenSwarm(t *testing.T) {
	after := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return after.Add(d).Format(time.RFC3339Nano) }
	// The run "after" names, finished, with a later completed update: never ours.
	earlier := map[string]any{"status": "done", "createdAt": at(0), "finishedAt": at(time.Minute)}
	done := func(finished string) map[string]any {
		return map[string]any{"status": "done", "createdAt": at(time.Minute), "finishedAt": finished}
	}
	justNow := time.Now().UTC().Format(time.RFC3339Nano)
	running := `[{"Id":"old","Created":100},{"Id":"new","Created":200}]`
	cases := []struct {
		name         string
		resourceType string
		service      string
		deployments  []map[string]any
		// update is the service's UpdateStatus JSON; empty fails the inspect.
		update     string
		containers string
		filter     string
		phase      string
		message    string
		container  string
	}{
		{name: "waiting in Dokploy's queue", update: `{"State":"completed"}`, phase: "queued"},
		{name: "building", deployments: []map[string]any{{"status": "running", "createdAt": at(time.Minute)}}, phase: "building"},
		{name: "a status Dokploy adds later keeps it open", deployments: []map[string]any{{"status": "pending", "createdAt": at(time.Minute)}}, phase: "building"},
		{name: "build failed", deployments: []map[string]any{{"status": "error", "createdAt": at(time.Minute), "errorMessage": "docker build exited 1"}}, phase: "failed", message: "docker build exited 1"},
		{name: "done, Swarm not started", deployments: []map[string]any{done(justNow)}, update: `null`, phase: "updating"},
		{name: "done, Swarm updating", deployments: []map[string]any{done(justNow)}, update: `{"State":"updating"}`, phase: "updating"},
		{name: "rolled back", deployments: []map[string]any{done(justNow)}, update: `{"State":"rollback_completed","Message":"rollback completed"}`, phase: "rolled_back", message: "rollback completed"},
		{
			name: "completed, newest of unsorted runs",
			// An older failed run listed after the newer one must not win.
			deployments: []map[string]any{done(justNow), {"status": "error", "createdAt": at(30 * time.Second)}},
			update:      `{"State":"completed"}`, containers: running, filter: "com.docker.swarm.service.name=web", phase: "completed", container: "new",
		},
		{name: "completed, new task not running yet", deployments: []map[string]any{done(justNow)}, update: `{"State":"completed"}`, containers: `[]`, filter: "com.docker.swarm.service.name=web", phase: "updating"},
		{
			name: "unchanged stack service", resourceType: "compose", service: "web_api",
			deployments: []map[string]any{done(at(2 * time.Minute))}, update: `null`, containers: running, filter: "com.docker.stack.namespace=web",
			phase: "completed", message: "Swarm had nothing to change", container: "new",
		},
		{
			name: "plain Compose ends with Dokploy", resourceType: "compose", service: "api",
			deployments: []map[string]any{done(justNow)}, containers: running, filter: "com.docker.compose.service=api",
			phase: "completed", container: "new",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resourceType := tc.resourceType
			if resourceType == "" {
				resourceType = "application"
			}
			swarmName := "web"
			if strings.HasPrefix(tc.service, "web_") {
				swarmName = tc.service
			}
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/services/"+swarmName) && tc.update != "":
					_, _ = w.Write([]byte(`{"ID":"svc","Spec":{"Name":"` + swarmName + `"},"UpdateStatus":` + tc.update + `}`))
				case strings.HasSuffix(r.URL.Path, "/containers/json") && tc.containers != "":
					if filters := r.URL.Query().Get("filters"); !strings.Contains(filters, tc.filter) || !strings.Contains(filters, "running") {
						t.Errorf("containers listed without the workload's running filter %q: %s", tc.filter, filters)
					}
					_, _ = w.Write([]byte(tc.containers))
				default:
					http.Error(w, "unexpected Docker call "+r.URL.Path, 500)
				}
			}))
			defer engine.Close()
			deployments := append([]map[string]any{earlier}, tc.deployments...)
			platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/project.all":
					_, _ = w.Write([]byte(`[{"environments":[{"applications":[{"applicationId":"app"}],"compose":[{"composeId":"stack"}]}]}]`))
				case "/api/application.one":
					_ = json.NewEncoder(w).Encode(map[string]any{"applicationId": "app", "appName": "web", "serverId": nil, "deployments": deployments})
				case "/api/compose.one":
					_ = json.NewEncoder(w).Encode(map[string]any{"composeId": "stack", "appName": "web", "serverId": nil, "deployments": deployments})
				default:
					http.NotFound(w, r)
				}
			}))
			defer platform.Close()
			t.Setenv("DOCKER_API_VERSION", "1.47")
			dc, err := docker.NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { client, _ := dc.GetClient("local"); _ = client.Close() }()
			cfg := &config.Config{DokployHosts: []config.DokployHostConfig{{HostName: "local", APIURL: platform.URL, APIToken: "token"}}}
			ar := &APIRouter{registry: services.NewRegistry(dc, nil, nil, cfg)}
			inventory, err := ar.registry.Dokploy().GetClient("local").Inventory(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			revision := ""
			for _, res := range inventory.Resources {
				if string(res.Type) == resourceType {
					revision = res.MappingRevision
				}
			}
			router := chi.NewRouter()
			router.Get("/api/v1/dokploy/rollout", ar.GetDokployRollout)
			id := map[string]string{"application": "app", "compose": "stack"}[resourceType]
			query := url.Values{"host": {"local"}, "resource": {id}, "resource_type": {resourceType}, "mapping_revision": {revision}, "after": {at(0)}, "service": {tc.service}}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/dokploy/rollout?"+query.Encode(), nil))
			var got rolloutStatus
			if err := json.Unmarshal(w.Body.Bytes(), &got); w.Code != 200 || err != nil {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			if got.Phase != tc.phase || got.Message != tc.message || got.ContainerID != tc.container {
				t.Fatalf("got %+v, want phase=%s message=%q container=%q", got, tc.phase, tc.message, tc.container)
			}
		})
	}
}

// A deployment renamed or moved since it was confirmed is not followed, and
// the conflict tells the panel to stop polling.
func TestDokployRolloutRejectsAChangedMapping(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"applicationId":"app","appName":"web","serverId":null,"deployments":[]}`))
	}))
	defer platform.Close()
	cfg := &config.Config{DokployHosts: []config.DokployHostConfig{{HostName: "local", APIURL: platform.URL, APIToken: "token"}}}
	ar := &APIRouter{registry: services.NewRegistry(nil, nil, nil, cfg)}
	w := httptest.NewRecorder()
	ar.GetDokployRollout(w, httptest.NewRequest("GET", "/api/v1/dokploy/rollout?host=local&resource=app&resource_type=application&mapping_revision=stale", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
