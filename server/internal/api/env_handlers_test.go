package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/go-chi/chi/v5"
)

func envTestRouter(t *testing.T, labels map[string]string, upstream http.HandlerFunc, connected bool) (http.Handler, *int) {
	t.Helper()
	mutations := new(int)
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.47")
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/container/json") {
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "container", "Name": "/web", "State": map[string]any{"Running": true}, "HostConfig": map[string]any{}, "Config": map[string]any{"Labels": labels, "Env": []string{"KEY=runtime", "GENERATED=runtime-only"}}})
			return
		}
		*mutations++
		http.Error(w, "unexpected Docker mutation", http.StatusInternalServerError)
	}))
	return envRouterWithEngine(t, engine, upstream, connected), mutations
}

func envRouterWithEngine(t *testing.T, engine *httptest.Server, upstream http.HandlerFunc, connected bool) http.Handler {
	t.Helper()
	t.Cleanup(engine.Close)
	t.Setenv("DOCKER_API_VERSION", "1.47")
	dc, err := docker.NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(engine.URL, "http://")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client, _ := dc.GetClient("local"); _ = client.Close() })
	var cc *coolify.MultiClient
	if connected {
		platform := httptest.NewServer(upstream)
		t.Cleanup(platform.Close)
		cc = coolify.NewMultiClient([]config.CoolifyHostConfig{{HostName: "local", APIURL: platform.URL, APIToken: "token"}})
	}
	ar := &APIRouter{registry: services.NewRegistry(dc, cc, nil, &config.Config{})}
	router := chi.NewRouter()
	router.Route("/api/v1", ar.registerContainerRoutes)
	return router
}

func coolifyLabels(resourceType string) map[string]string {
	return map[string]string{"coolify.managed": "true", "coolify.type": resourceType, "com.docker.compose.project": "resource"}
}

func envRequest(router http.Handler, method, suffix, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/containers/container/env"+suffix+"?host=local", strings.NewReader(body)))
	return w
}

// These records differ intentionally from Docker's runtime snapshot. The raw
// shared reference, preview duplicate and hidden value must survive the API.
const savedEnvFixture = `[
 {"uuid":"prod","key":"KEY","value":"{{project.SECRET}}","real_value":"resolved-secret","is_preview":false,"is_buildtime":true,"is_runtime":false,"is_literal":true,"is_multiline":true,"is_shared":true},
 {"uuid":"preview","key":"KEY","value":"preview-only","is_preview":true,"is_buildtime":false,"is_runtime":true,"is_literal":false,"is_multiline":false},
 {"uuid":"hidden","key":"TOKEN","is_shown_once":true},
 {"uuid":"build","key":"BUILD_ONLY","value":"build-only","is_buildtime":true,"is_runtime":false},
 {"uuid":"empty","key":"EMPTY","value":""}
]`

// Services have no preview, build-time or runtime selection in their API.
const savedServiceEnvFixture = `[
 {"uuid":"prod","key":"KEY","value":"{{project.SECRET}}","is_literal":true,"is_multiline":true,"is_shown_once":false,"is_shared":true},
 {"uuid":"service-only","key":"SERVICE_SETTING","value":"service-value","is_literal":false,"is_multiline":false,"is_shown_once":false}
]`

func TestCoolifyEnvironmentReadsSavedRecords(t *testing.T) {
	router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/applications/resource/envs" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(savedEnvFixture))
	}, true)
	w := envRequest(router, http.MethodGet, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Source    string
		Variables []struct {
			UUID  string
			Key   string
			Value *string
		}
		Env map[string]string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Source != "coolify" || len(response.Variables) != 5 {
		t.Fatalf("saved records missing: %s", w.Body.String())
	}
	if response.Env["KEY"] != "{{project.SECRET}}" || response.Variables[1].Value == nil || *response.Variables[1].Value != "preview-only" || response.Variables[2].Value != nil || response.Variables[4].Value == nil || *response.Variables[4].Value != "" {
		t.Fatalf("raw values, scopes or unknown state lost: %s", w.Body.String())
	}
	if _, ok := response.Env["GENERATED"]; ok {
		t.Fatal("runtime-only variable included in saved configuration")
	}
	if *mutations != 0 {
		t.Fatal("reading environment mutated Docker")
	}
}

func TestCoolifyEnvironmentSavesExplicitChangesBeforeDeploy(t *testing.T) {
	for _, tc := range []struct {
		resourceType, fixture, changes, deployBody string
		wantWrites                                 []map[string]any
	}{
		{"application", savedEnvFixture,
			`[{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"line1\nline2"},{"uuid":"preview","key":"KEY","expected_value":"preview-only","is_preview":true,"value":"new-preview"}]`,
			`{"deployments":[{"resource_uuid":"resource","deployment_uuid":"deployment"}]}`,
			[]map[string]any{
				{"key": "KEY", "value": "line1\nline2", "is_preview": false, "is_buildtime": true, "is_runtime": false, "is_literal": true, "is_multiline": true, "is_shown_once": false},
				{"key": "KEY", "value": "new-preview", "is_preview": true, "is_buildtime": false, "is_runtime": true, "is_literal": false, "is_multiline": false, "is_shown_once": false},
			}},
		{"service", savedServiceEnvFixture,
			`[{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"line1\nline2"}]`,
			`{"message":"Service restarting request queued."}`,
			[]map[string]any{{"key": "KEY", "value": "line1\nline2", "is_literal": true, "is_multiline": true, "is_shown_once": false}}},
	} {
		t.Run(tc.resourceType, func(t *testing.T) {
			var writes []map[string]any
			deployed := false
			router, mutations := envTestRouter(t, coolifyLabels(tc.resourceType), func(w http.ResponseWriter, r *http.Request) {
				envPath := "/api/v1/" + tc.resourceType + "s/resource/envs"
				if r.Method == http.MethodGet && r.URL.Path == envPath {
					_, _ = w.Write([]byte(tc.fixture))
					return
				}
				if (tc.resourceType == "application" && r.Method == http.MethodGet && r.URL.Path == "/api/v1/deploy" && r.URL.Query().Get("uuid") == "resource") || (tc.resourceType == "service" && r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/resource/restart") {
					deployed = true
					_, _ = w.Write([]byte(tc.deployBody))
					return
				}
				if r.Method != http.MethodPatch || r.URL.Path != envPath {
					t.Errorf("unexpected write %s %s", r.Method, r.URL.Path)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				writes = append(writes, payload)
				w.WriteHeader(http.StatusCreated)
			}, true)
			w := envRequest(router, http.MethodPut, "", `{"changes":`+tc.changes+`}`)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"saved":true`) || !strings.Contains(w.Body.String(), `"applied":false`) {
				t.Fatalf("save failed: %d %s", w.Code, w.Body.String())
			}
			if !reflect.DeepEqual(writes, tc.wantWrites) || deployed || *mutations != 0 {
				t.Fatalf("saved payload/settings or lifecycle incorrect: writes=%v want=%v deployed=%t docker=%d", writes, tc.wantWrites, deployed, *mutations)
			}
			w = envRequest(router, http.MethodPost, "/deploy", "")
			if w.Code != http.StatusAccepted || !deployed || *mutations != 0 {
				t.Fatalf("platform deployment failed: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCoolifySaveReportsWriteFailures(t *testing.T) {
	const productionEdit = `{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"new-production"}`
	for _, tc := range []struct {
		name, changes, failMethod, failPath string
		completed                           int
	}{
		{"first update fails", productionEdit, http.MethodPatch, "/api/v1/applications/resource/envs", 0},
		{"first creation fails", `{"key":"NEW","value":"new"}`, http.MethodPost, "/api/v1/applications/resource/envs", 0},
		{"later update fails", productionEdit + `,{"uuid":"preview","key":"KEY","expected_value":"preview-only","is_preview":true,"value":"new-preview"}`, http.MethodPatch, "/api/v1/applications/resource/envs", 1},
		{"later creation fails", productionEdit + `,{"key":"NEW","value":"new"}`, http.MethodPost, "/api/v1/applications/resource/envs", 1},
		{"later deletion fails", productionEdit + `,{"uuid":"build","key":"BUILD_ONLY","expected_value":"build-only","remove":true}`, http.MethodDelete, "/api/v1/applications/resource/envs/build", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var operations []string
			router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v1/applications/resource/envs" {
					_, _ = w.Write([]byte(savedEnvFixture))
					return
				}
				operations = append(operations, r.Method+" "+r.URL.Path)
				if len(operations) == tc.completed+1 {
					if r.Method != tc.failMethod || r.URL.Path != tc.failPath {
						t.Errorf("failure reached wrong operation: %s %s", r.Method, r.URL.Path)
					}
					http.Error(w, "denied", http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusCreated)
			}, true)
			w := envRequest(router, http.MethodPut, "", `{"changes":[`+tc.changes+`,{"uuid":"hidden","key":"TOKEN","value":"must-not-write"}]}`)
			var result struct {
				Saved          bool
				Completed      int
				Message        string
				ReloadRequired bool `json:"reload_required"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusBadGateway || result.Saved || result.Completed != tc.completed || !result.ReloadRequired || !strings.Contains(result.Message, "some changes may have been saved") {
				t.Fatalf("failure presented incorrectly: %d %s", w.Code, w.Body.String())
			}
			if len(operations) != tc.completed+1 || *mutations != 0 {
				t.Fatalf("save continued after failure or touched Docker: %v docker=%d", operations, *mutations)
			}
		})
	}
}

func TestCoolifySaveDeletesOnlyExplicitRecord(t *testing.T) {
	for _, tc := range []struct{ resourceType, fixture, change, uuid string }{
		{"application", savedEnvFixture, `{"uuid":"preview","key":"KEY","expected_value":"preview-only","is_preview":true,"remove":true}`, "preview"},
		{"service", savedServiceEnvFixture, `{"uuid":"service-only","key":"SERVICE_SETTING","expected_value":"service-value","remove":true}`, "service-only"},
	} {
		t.Run(tc.resourceType, func(t *testing.T) {
			var operations []string
			router, mutations := envTestRouter(t, coolifyLabels(tc.resourceType), func(w http.ResponseWriter, r *http.Request) {
				envPath := "/api/v1/" + tc.resourceType + "s/resource/envs"
				if r.Method == http.MethodGet && r.URL.Path == envPath {
					_, _ = w.Write([]byte(tc.fixture))
					return
				}
				operations = append(operations, r.Method+" "+r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			}, true)
			w := envRequest(router, http.MethodPut, "", `{"changes":[`+tc.change+`]}`)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"saved":true`) || !strings.Contains(w.Body.String(), `"completed":1`) || !strings.Contains(w.Body.String(), `"applied":false`) {
				t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
			}
			want := []string{"DELETE /api/v1/" + tc.resourceType + "s/resource/envs/" + tc.uuid}
			if !reflect.DeepEqual(operations, want) || *mutations != 0 {
				t.Fatalf("deleted unrelated records or deployed: %v", operations)
			}
		})
	}
}

func TestCoolifySaveHiddenReplacementAndNewVariable(t *testing.T) {
	var writes []map[string]any
	router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(savedEnvFixture))
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if len(writes) == 0 && r.Method != http.MethodPatch {
			t.Errorf("replacement used %s", r.Method)
		}
		if len(writes) == 1 && r.Method != http.MethodPost {
			t.Errorf("creation used %s", r.Method)
		}
		writes = append(writes, payload)
		w.WriteHeader(http.StatusCreated)
	}, true)
	w := envRequest(router, http.MethodPut, "", `{"changes":[{"uuid":"hidden","key":"TOKEN","value":"replacement"},{"key":"NEW","value":"one\ntwo"}]}`)
	if w.Code != http.StatusOK || len(writes) != 2 || *mutations != 0 {
		t.Fatalf("save failed: %d %s writes=%v", w.Code, w.Body.String(), writes)
	}
	if writes[0]["value"] != "replacement" || writes[0]["is_shown_once"] != true || writes[1]["is_multiline"] != true {
		t.Fatalf("replacement lost hidden status or creation lost multiline value: %v", writes)
	}
}

func TestCoolifySaveValidatesAllChangesBeforeWriting(t *testing.T) {
	for _, tc := range []struct{ name, change, wantError string }{
		{"wrong preview scope", `{"uuid":"preview","key":"KEY","expected_value":"preview-only","value":"wrong-scope"}`, "changed or is ambiguous"},
		{"missing UUID", `{"uuid":"missing","key":"KEY","value":"stale-row"}`, "changed or is ambiguous"},
		{"existing key creation", `{"key":"KEY","value":"already-exists"}`, "already exists"},
		{"missing replacement", `{"uuid":"hidden","key":"TOKEN"}`, "replacement value is required"},
		{"stale value", `{"uuid":"prod","key":"KEY","expected_value":"stale-value","value":"overwrite"}`, "changed since it was loaded"},
		{"missing readable baseline", `{"uuid":"prod","key":"KEY","value":"overwrite"}`, "changed since it was loaded"},
		{"readable value now unknown", `{"uuid":"hidden","key":"TOKEN","expected_value":"previously-readable","value":"overwrite"}`, "changed since it was loaded"},
		{"UUID key mismatch", `{"uuid":"prod","key":"TOKEN","expected_value":"{{project.SECRET}}","value":"wrong-record"}`, "changed or is ambiguous"},
		{"reserved key", `{"key":"COOLIFY_URL","value":"bad"}`, "invalid or reserved"},
		{"invalid key", `{"key":"INVALID KEY","value":"bad"}`, "invalid or reserved"},
		{"duplicate changes", `{"uuid":"build","key":"BUILD_ONLY","expected_value":"build-only","value":"again"}`, "duplicate change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(savedEnvFixture))
					return
				}
				writes++
			}, true)
			// The first valid edit uses a different key so the intended second-row
			// failure cannot be masked by duplicate-change validation.
			w := envRequest(router, http.MethodPut, "", `{"changes":[{"uuid":"build","key":"BUILD_ONLY","expected_value":"build-only","value":"valid"},`+tc.change+`]}`)
			if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), tc.wantError) || !strings.Contains(w.Body.String(), `"completed":0`) || writes != 0 || *mutations != 0 {
				t.Fatalf("wrong rejection or invalid batch partly written: %d %s writes=%d", w.Code, w.Body.String(), writes)
			}
		})
	}
}

func TestCoolifyDeploymentRequiresAcknowledgement(t *testing.T) {
	for _, response := range []string{
		`{"deployments":[{"resource_uuid":"resource","message":"Unauthorized to deploy this application."}]}`,
		`{"deployments":[{"resource_uuid":"another-resource","deployment_uuid":"deployment"}]}`,
		`not-json`,
	} {
		t.Run(response, func(t *testing.T) {
			router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(response)) }, true)
			w := envRequest(router, http.MethodPost, "/deploy", "")
			if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "saved configuration is unchanged") || *mutations != 0 {
				t.Fatalf("false deployment success: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCoolifyManagedEnvironmentNeverFallsBackToDocker(t *testing.T) {
	for _, tc := range []struct {
		name           string
		labels         map[string]string
		connected      bool
		upstreamStatus int
	}{
		{"missing integration", coolifyLabels("application"), false, 200},
		{"missing resource", map[string]string{"coolify.managed": "true", "coolify.type": "application"}, true, 200},
		{"database", coolifyLabels("database"), true, 200},
		{"unknown resource type", coolifyLabels("mystery"), true, 200},
		{"Coolify cannot read", coolifyLabels("application"), true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, mutations := envTestRouter(t, tc.labels, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.upstreamStatus) }, tc.connected)
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				w := envRequest(router, method, "", `{"changes":[{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"new"}]}`)
				if w.Code < 400 {
					t.Errorf("%s unexpectedly succeeded: %s", method, w.Body.String())
				}
			}
			if *mutations != 0 {
				t.Fatalf("managed editing reached Docker mutations: %d", *mutations)
			}
		})
	}
}

func TestCoolifyMalformedConfigurationNeverWrites(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			writes := 0
			router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					writes++
				}
				_, _ = w.Write([]byte(`{"unexpected":"not an env array"}`))
			}, true)
			w := envRequest(router, method, "", `{"changes":[{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"new"}]}`)
			if w.Code != http.StatusBadGateway || writes != 0 || *mutations != 0 {
				t.Fatalf("malformed config caused mutation/success: %d %s writes=%d", w.Code, w.Body.String(), writes)
			}
		})
	}
}

func TestCoolifyRejectsReplacementRuntimeMap(t *testing.T) {
	for _, body := range []string{
		`{"env":{"KEY":"runtime","GENERATED":"runtime-only"}}`,
		`{"env":{},"changes":[{"uuid":"prod","key":"KEY","expected_value":"{{project.SECRET}}","value":"new"}]}`,
		`{}`,
	} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			router, mutations := envTestRouter(t, coolifyLabels("application"), func(w http.ResponseWriter, r *http.Request) { calls++ }, true)
			w := envRequest(router, http.MethodPut, "", body)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "explicit variable changes") || calls != 0 || *mutations != 0 {
				t.Fatalf("runtime-map replacement was accepted: %d %s calls=%d", w.Code, w.Body.String(), calls)
			}
		})
	}
}

func TestStandaloneEnvironmentReadsAndReplacesThroughDocker(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantEnv    map[string]string
	}{
		{"replace values", `{"env":{"KEEP":"updated","NEW":"a=b\nc"}}`, map[string]string{"KEEP": "updated", "NEW": "a=b\nc"}},
		{"clear all", `{"env":{}}`, map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var createdEnv []string
			var lifecycle []string
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/v1.47")
				if r.Method == http.MethodGet && path == "/containers/container/json" {
					_, _ = w.Write([]byte(`{"Id":"container","Name":"/web","State":{"Running":true},"HostConfig":{},"Config":{"Labels":{},"Env":["KEEP=old","REMOVE=old"]}}`))
					return
				}
				lifecycle = append(lifecycle, r.Method+" "+path)
				if r.Method == http.MethodPost && path == "/containers/create" {
					var payload struct{ Env []string }
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					createdEnv = payload.Env
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"Id":"replacement","Warnings":[]}`))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			router := envRouterWithEngine(t, engine, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("standalone editing reached Coolify: %s %s", r.Method, r.URL.Path)
			}, true)
			w := envRequest(router, http.MethodGet, "", "")
			var response struct {
				Source string
				Env    map[string]string
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusOK || response.Source != "docker" || !reflect.DeepEqual(response.Env, map[string]string{"KEEP": "old", "REMOVE": "old"}) {
				t.Fatalf("runtime read failed: %d %s", w.Code, w.Body.String())
			}
			w = envRequest(router, http.MethodPut, "", tc.body)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"new_container_id":"replacement"`) {
				t.Fatalf("standalone save failed: %d %s", w.Code, w.Body.String())
			}
			if len(createdEnv) != len(tc.wantEnv) {
				t.Fatalf("replacement environment not exact: %v", createdEnv)
			}
			got := map[string]string{}
			for _, entry := range createdEnv {
				parts := strings.SplitN(entry, "=", 2)
				if len(parts) != 2 {
					t.Fatalf("invalid env entry %q", entry)
				}
				got[parts[0]] = parts[1]
			}
			if !reflect.DeepEqual(got, tc.wantEnv) {
				t.Fatalf("replacement values lost or removed variable retained: %v", createdEnv)
			}
			want := []string{"POST /containers/container/stop", "POST /containers/container/rename", "POST /containers/create", "POST /containers/replacement/start", "DELETE /containers/container"}
			if !reflect.DeepEqual(lifecycle, want) {
				t.Fatalf("unexpected Docker lifecycle: %v", lifecycle)
			}
		})
	}
}
