package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
)

func resourcesRequest(router http.Handler, method, suffix, query, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/containers/container/resources"+suffix+"?host=local"+query, strings.NewReader(body)))
	return w
}

const limitsBody = `{"memoryBytes":536870912,"nanoCPUs":1500000000}`

func TestCoolifyApplicationLimitsSaveBeforeDeploy(t *testing.T) {
	var patch map[string]any
	deployed := false
	app := map[string]any{"build_pack": "nixpacks", "limits_memory": "1g", "limits_memory_swap": "0", "limits_cpus": "2"}
	router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/applications/resource":
			_ = json.NewEncoder(w).Encode(app)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/applications/resource":
			_ = json.NewDecoder(r.Body).Decode(&patch)
			_, _ = w.Write([]byte(`{"uuid":"resource"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/deploy":
			deployed = true
			_, _ = w.Write([]byte(`{"deployments":[{"resource_uuid":"resource","deployment_uuid":"deployment"}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}, true)

	read := resourcesRequest(router, http.MethodGet, "", "", "")
	if read.Code != 200 || !strings.Contains(read.Body.String(), `"configured":{"memoryBytes":1073741824,"nanoCPUs":2000000000}`) || !strings.Contains(read.Body.String(), `"persistent":true`) {
		t.Fatalf("read: %d %s", read.Code, read.Body.String())
	}
	// Coolify fixes the restart policy itself, so it is not sent along.
	if w := resourcesRequest(router, http.MethodPut, "", "", `{"memoryBytes":0,"nanoCPUs":0,"restartPolicy":{"name":"always"}}`); w.Code != 400 || patch != nil {
		t.Fatalf("restart policy: %d %s", w.Code, w.Body.String())
	}
	saved := resourcesRequest(router, http.MethodPut, "", "", limitsBody)
	if saved.Code != 200 || !strings.Contains(saved.Body.String(), `"saved":true`) || !strings.Contains(saved.Body.String(), `"applied":false`) {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	if want := map[string]any{"limits_memory": "512m", "limits_cpus": "1.5"}; !reflect.DeepEqual(patch, want) || deployed || *mutations != 0 {
		t.Fatalf("patch=%v deployed=%t Docker writes=%d", patch, deployed, *mutations)
	}
	if w := resourcesRequest(router, http.MethodPost, "/deploy", "", ""); w.Code != 202 || !deployed || *mutations != 0 {
		t.Fatalf("deploy: %d %s", w.Code, w.Body.String())
	}

	// Docker would refuse a swap limit below the new memory limit on deploy.
	patch = nil
	app["limits_memory_swap"] = "256m"
	if w := resourcesRequest(router, http.MethodPut, "", "", limitsBody); w.Code != 502 || patch != nil {
		t.Fatalf("swap below memory: %d %s", w.Code, w.Body.String())
	}
	// A Compose application's limits come from its Compose file.
	app["build_pack"] = "dockercompose"
	if w := resourcesRequest(router, http.MethodGet, "", "", ""); !strings.Contains(w.Body.String(), `"persistent":false`) || !strings.Contains(w.Body.String(), "Compose file") {
		t.Fatalf("compose application: %d %s", w.Code, w.Body.String())
	}
	if w := resourcesRequest(router, http.MethodPut, "", "", limitsBody); w.Code != 502 || patch != nil {
		t.Fatalf("compose application save: %d %s", w.Code, w.Body.String())
	}
}

// Workloads LogDeck cannot save limits for are refused rather than updated
// live, which their platform would undo on the next deployment.
func TestManagedLimitsNeverUpdateLive(t *testing.T) {
	for _, tc := range []struct {
		name      string
		labels    map[string]string
		connected bool
	}{
		{"coolify service", coolifyLabels("service"), true},
		{"coolify database", coolifyLabels("database"), true},
		{"unconnected coolify", coolifyLabels("application"), false},
		{"unconnected dokploy", map[string]string{"com.docker.compose.project.working_dir": "/etc/dokploy/compose/web/code"}, false},
		{"swarm task", map[string]string{"com.docker.swarm.service.name": "web"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, mutations := envTestRouter(t, tc.labels, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected platform request %s %s", r.Method, r.URL.Path)
			}, tc.connected)
			read := resourcesRequest(router, http.MethodGet, "", "", "")
			if read.Code != 200 || !strings.Contains(read.Body.String(), `"persistent":false`) || !strings.Contains(read.Body.String(), `"reason":`) {
				t.Fatalf("read: %d %s", read.Code, read.Body.String())
			}
			for _, suffix := range []string{"", "/deploy"} {
				method := http.MethodPut
				if suffix != "" {
					method = http.MethodPost
				}
				if w := resourcesRequest(router, method, suffix, "", limitsBody); w.Code != 409 || *mutations != 0 {
					t.Fatalf("%s: %d %s, Docker writes=%d", method, w.Code, w.Body.String(), *mutations)
				}
			}
		})
	}
}

func TestPlainComposeLimitsUpdateLive(t *testing.T) {
	router, mutations := envTestRouter(t, map[string]string{"com.docker.compose.project": "web"}, nil, false)
	read := resourcesRequest(router, http.MethodGet, "", "", "")
	if read.Code != 200 || !strings.Contains(read.Body.String(), `"platform":"compose"`) {
		t.Fatalf("read: %d %s", read.Code, read.Body.String())
	}
	// The test engine answers any write with an error, so one counted write
	// shows the request reached Docker's live update.
	resourcesRequest(router, http.MethodPut, "", "", limitsBody)
	if *mutations != 1 {
		t.Fatalf("Docker writes=%d, want the live update", *mutations)
	}
}

func TestDokployApplicationLimitsFollowTheMappedDeployment(t *testing.T) {
	labels := map[string]string{"com.docker.swarm.service.name": "web"}
	var update map[string]any
	deploys := 0
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/container/json") {
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "container", "Name": "/web.1.task", "State": map[string]any{"Running": true}, "HostConfig": map[string]any{"Memory": 1073741824}, "Config": map[string]any{"Labels": labels}})
			return
		}
		http.Error(w, "unexpected Docker request", 500)
	}))
	t.Cleanup(engine.Close)
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/project.all":
			_, _ = w.Write([]byte(`[{"environments":[{"applications":[{"applicationId":"app"}]}]}]`))
		case "/api/application.one":
			_, _ = w.Write([]byte(`{"applicationId":"app","appName":"web","serverId":null,"memoryLimit":"1073741824","cpuLimit":null}`))
		case "/api/application.update":
			_ = json.NewDecoder(r.Body).Decode(&update)
			_, _ = w.Write([]byte(`true`))
		case "/api/application.deploy":
			deploys++
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

	// Until the deployment is chosen, nothing is saved anywhere.
	unmapped := resourcesRequest(router, http.MethodGet, "", "", "")
	if !strings.Contains(unmapped.Body.String(), `"platform":"dokploy"`) || !strings.Contains(unmapped.Body.String(), "Environment panel") {
		t.Fatalf("unmapped: %d %s", unmapped.Code, unmapped.Body.String())
	}
	if w := resourcesRequest(router, http.MethodPut, "", "", limitsBody); w.Code != 409 || update != nil {
		t.Fatalf("unmapped save: %d %s", w.Code, w.Body.String())
	}

	mapped := "&resource=app&resource_type=application"
	read := resourcesRequest(router, http.MethodGet, "", mapped, "")
	if !strings.Contains(read.Body.String(), `"configured":{"memoryBytes":1073741824,"nanoCPUs":0}`) {
		t.Fatalf("mapped read: %d %s", read.Code, read.Body.String())
	}
	saved := resourcesRequest(router, http.MethodPut, "", mapped, `{"memoryBytes":0,"nanoCPUs":1500000000}`)
	if want := map[string]any{"applicationId": "app", "memoryLimit": "", "cpuLimit": "1500000000"}; saved.Code != 200 || !reflect.DeepEqual(update, want) || deploys != 0 {
		t.Fatalf("save: %d %s update=%v deploys=%d", saved.Code, saved.Body.String(), update, deploys)
	}
	if w := resourcesRequest(router, http.MethodPost, "/deploy", mapped, ""); w.Code != 202 || deploys != 1 {
		t.Fatalf("deploy: %d %s", w.Code, w.Body.String())
	}

	// A mapping to an application this task was not deployed from is refused
	// by the record read before anything is written.
	update = nil
	labels["com.docker.swarm.service.name"] = "other"
	labels["dokploy.application.id"] = "app"
	if w := resourcesRequest(router, http.MethodPut, "", mapped, limitsBody); w.Code != 502 || update != nil {
		t.Fatalf("mismatched service: %d %s", w.Code, w.Body.String())
	}
}
