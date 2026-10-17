package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
)

func TestDokployMappingSaveAndDeploy(t *testing.T) {
	dockerWrites := 0
	platformWrites := 0
	deploys := 0
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/container/json") {
			_, _ = w.Write([]byte(`{"Id":"container","Name":"/web","Config":{"Labels":{"com.docker.compose.project":"web"},"Env":["KEY=runtime"]},"HostConfig":{},"State":{"Running":true}}`))
			return
		}
		dockerWrites++
		http.Error(w, "unexpected Docker write", 500)
	}))
	defer engine.Close()
	platformDown := false
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if platformDown {
			http.Error(w, "down", 500)
			return
		}
		switch r.URL.Path {
		case "/api/project.all":
			_, _ = w.Write([]byte(`[{"environments":[{"applications":[{"applicationId":"app"}]}]}]`))
		case "/api/application.one":
			_, _ = w.Write([]byte(`{"applicationId":"app","appName":"web","serverId":null,"env":"KEY=saved","buildArgs":null,"buildSecrets":"SECRET=preserve","createEnvFile":true}`))
		case "/api/application.saveEnvironment":
			platformWrites++
			_, _ = w.Write([]byte(`true`))
		case "/api/application.deploy":
			deploys++
			_, _ = w.Write([]byte(`{"success":true}`))
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
	cfg := &config.Config{DokployHosts: []config.DokployHostConfig{{HostName: "local", APIURL: platform.URL + "/api", APIToken: "token"}}}
	ar := &APIRouter{registry: services.NewRegistry(dc, nil, nil, cfg)}
	router := chi.NewRouter()
	router.Route("/api/v1", ar.registerContainerRoutes)
	request := func(method, query, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/containers/container/env"+query, strings.NewReader(body)))
		return w
	}
	plain := request("GET", "?host=local&platform=docker&confirmed=true", "")
	if plain.Code != 200 || !strings.Contains(plain.Body.String(), `"source":"docker"`) || !strings.Contains(plain.Body.String(), `"plain_compose":true`) {
		t.Fatalf("plain Compose runtime editor unavailable: %d %s", plain.Code, plain.Body.String())
	}
	// Dokploy being unreachable must not hide the runtime editor from a
	// workload nothing marks as Dokploy's.
	platformDown = true
	unreachable := request("GET", "?host=local", "")
	if unreachable.Code != 200 || !strings.Contains(unreachable.Body.String(), `"plain_compose_allowed":true`) || !strings.Contains(unreachable.Body.String(), `"inventory_error"`) {
		t.Fatalf("unreachable Dokploy: %d %s", unreachable.Code, unreachable.Body.String())
	}
	platformDown = false
	discovered := request("GET", "?host=local", "")
	if discovered.Code != 200 || !strings.Contains(discovered.Body.String(), `"mapping_required":true`) || !strings.Contains(discovered.Body.String(), `"instance_url":"`+platform.URL+`"`) {
		t.Fatalf("discovery: %d %s", discovered.Code, discovered.Body.String())
	}
	for _, q := range []string{"?host=local", "?host=local&resource=app&resource_type=application", "?host=local&resource=wrong&resource_type=application&confirmed=true"} {
		w := request("PUT", q, `{"text":"KEY=new"}`)
		if w.Code != 409 {
			t.Fatalf("unconfirmed or invalid mapping: %d %s", w.Code, w.Body.String())
		}
	}
	var inventory struct {
		Resources []struct {
			MappingRevision string `json:"mapping_revision"`
		}
	}
	if err := json.Unmarshal(discovered.Body.Bytes(), &inventory); err != nil {
		t.Fatal(err)
	}
	query := "?host=local&resource=app&resource_type=application&confirmed=true&mapping_revision=" + inventory.Resources[0].MappingRevision
	read := request("GET", query, "")
	var loaded struct {
		Configuration struct {
			Revision string `json:"revision"`
		}
	}
	if err := json.Unmarshal(read.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"text": "KEY=new", "revision": loaded.Configuration.Revision})
	saved := request("PUT", query, string(body))
	if saved.Code != 200 || !strings.Contains(saved.Body.String(), `"applied":false`) || platformWrites != 1 || deploys != 0 {
		t.Fatalf("save=%d %s writes=%d deploys=%d", saved.Code, saved.Body.String(), platformWrites, deploys)
	}
	deployed := request("POST", "/deploy"+query, "")
	if deployed.Code != 202 || deploys != 1 || dockerWrites != 0 {
		t.Fatalf("deploy=%d deploys=%d Docker writes=%d", deployed.Code, deploys, dockerWrites)
	}
}

func TestUnconnectedDokployAndSwarmNeverRecreate(t *testing.T) {
	for _, labels := range []map[string]string{{"com.docker.compose.project.working_dir": "/etc/dokploy/compose/web/code"}, {"com.docker.swarm.service.name": "web"}} {
		router, writes := envTestRouter(t, labels, nil, false)
		for _, method := range []string{"GET", "PUT", "POST"} {
			suffix := ""
			if method == "POST" {
				suffix = "/deploy"
			}
			w := envRequest(router, method, suffix, `{"env":{"KEY":"runtime"}}`)
			if w.Code != 409 || *writes != 0 {
				t.Fatalf("%s: code=%d Docker writes=%d", method, w.Code, *writes)
			}
		}
	}
}

// dokployRouter serves one container with labels on a host connected to a
// Dokploy instance that deployed app (an application named "web") and db (a
// Postgres database).
func dokployRouter(t *testing.T, labels map[string]string) http.Handler {
	t.Helper()
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": "container", "Names": []string{"/web.1.task"}, "State": "running", "Labels": labels}})
		case strings.HasSuffix(r.URL.Path, "/containers/container/json"):
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "container", "Name": "/web.1.task", "State": map[string]any{"Running": true}, "HostConfig": map[string]any{}, "Config": map[string]any{"Labels": labels, "Env": []string{"KEY=runtime"}}})
		default:
			http.Error(w, "unexpected Docker request", 500)
		}
	}))
	t.Cleanup(engine.Close)
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/project.all":
			_, _ = w.Write([]byte(`[{"environments":[{"name":"production","isDefault":true,"applications":[{"applicationId":"app"}],"postgres":[{"postgresId":"db","name":"db","applicationStatus":"done"}]}]}]`))
		case "/api/application.one":
			_, _ = w.Write([]byte(`{"applicationId":"app","appName":"web","serverId":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(platform.Close)
	t.Setenv("DOCKER_API_VERSION", "1.47")
	dc, err := docker.NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client, _ := dc.GetClient("local"); _ = client.Close() })
	cfg := &config.Config{DokployHosts: []config.DokployHostConfig{{HostName: "local", APIURL: platform.URL, APIToken: "token"}}}
	ar := &APIRouter{registry: services.NewRegistry(dc, nil, nil, cfg)}
	router := chi.NewRouter()
	router.Route("/api/v1", ar.registerContainerRoutes)
	return router
}

func TestDokploySwarmServicesAreMarked(t *testing.T) {
	for _, tc := range []struct {
		labels map[string]string
		marked bool
	}{
		{map[string]string{"com.docker.swarm.service.name": "web"}, true},
		{map[string]string{"com.docker.swarm.service.name": "other"}, false},
		{map[string]string{"dokploy.application.id": "db", "com.docker.swarm.service.name": "postgres-db"}, true},
		{map[string]string{"com.docker.compose.project.working_dir": "/etc/dokploy/compose/web/code"}, true},
		{map[string]string{"com.docker.compose.project.working_dir": "/home/me/dokploy/web"}, false},
	} {
		w := httptest.NewRecorder()
		dokployRouter(t, tc.labels).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers", nil))
		if marked := strings.Contains(w.Body.String(), `"platform":"dokploy"`); w.Code != 200 || marked != tc.marked {
			t.Fatalf("labels=%v: code=%d marked=%v, want %v: %s", tc.labels, w.Code, marked, tc.marked, w.Body.String())
		}
	}
}

func TestDokployResourceLabelPicksTheDeployment(t *testing.T) {
	get := func(labels map[string]string, query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		dokployRouter(t, labels).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/container/env?host=local"+query, nil))
		return w
	}
	// The label wins over a deployment name that happens to match.
	labelled := get(map[string]string{"dokploy.application.id": "app", "com.docker.swarm.service.name": "renamed"}, "")
	if !strings.Contains(labelled.Body.String(), `"suggested_resources":["application/app"]`) || !strings.Contains(labelled.Body.String(), `"mapping_key":"com.docker.swarm.service.name=renamed"`) {
		t.Fatalf("labelled: %d %s", labelled.Code, labelled.Body.String())
	}
	other := get(map[string]string{"dokploy.application.id": "elsewhere", "com.docker.swarm.service.name": "web"}, "")
	if !strings.Contains(other.Body.String(), `"suggested_resources":[]`) {
		t.Fatalf("another resource's label: %d %s", other.Code, other.Body.String())
	}
	var inventory struct {
		Resources []struct {
			MappingRevision string `json:"mapping_revision"`
		}
	}
	if err := json.Unmarshal(other.Body.Bytes(), &inventory); err != nil || len(inventory.Resources) != 1 {
		t.Fatalf("inventory: %v %s", err, other.Body.String())
	}
	mismatch := get(map[string]string{"dokploy.application.id": "elsewhere"}, "&resource=app&resource_type=application&confirmed=true&mapping_revision="+inventory.Resources[0].MappingRevision)
	if mismatch.Code != 409 {
		t.Fatalf("mapping to a resource the label rules out: %d %s", mismatch.Code, mismatch.Body.String())
	}
	database := get(map[string]string{"dokploy.application.id": "db", "com.docker.swarm.service.name": "postgres-db"}, "")
	if database.Code != 409 || !strings.Contains(database.Body.String(), "databases") {
		t.Fatalf("database: %d %s", database.Code, database.Body.String())
	}
}
