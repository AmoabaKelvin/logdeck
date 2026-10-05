package dokploy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

// Exercise the documented HTTP contract: applications require all build
// settings while Compose must never receive application-only settings.
func TestSaveEnvironmentPreservesDokploySettings(t *testing.T) {
	for _, kind := range []ResourceType{Application, Compose} {
		t.Run(string(kind), func(t *testing.T) {
			rec := map[string]any{string(kind) + "Id": "resource", "appName": "web", "serverId": "remote", "env": "# keep\nTOKEN=${{vault.prod.secret}}\nURL=postgres://db?sslmode=require&pool=5\nMULTI=\"one\ntwo\"\n", "buildArgs": "MODE=build", "buildSecrets": nil, "createEnvFile": false}
			var payload map[string]any
			deploys := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") != "key" {
					t.Error("missing API authentication")
				}
				switch r.URL.Path {
				case "/api/" + string(kind) + ".one":
					if r.URL.Query().Get(string(kind)+"Id") != "resource" {
						t.Error("wrong resource query")
					}
					// Dokploy's encoder leaves & < > unescaped, unlike Go's default.
					encoder := json.NewEncoder(w)
					encoder.SetEscapeHTML(false)
					_ = encoder.Encode(rec)
				case "/api/" + string(kind) + ".saveEnvironment":
					if r.Method != "POST" {
						t.Error("save must POST")
					}
					_ = json.NewDecoder(r.Body).Decode(&payload)
					rec["env"] = payload["env"]
					_, _ = w.Write([]byte("true"))
				case "/api/" + string(kind) + ".deploy":
					deploys++
					_, _ = w.Write([]byte(`{"success":true}`))
				default:
					t.Errorf("unexpected route %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			client := NewClient(config.DokployHostConfig{APIURL: upstream.URL + "/api/", APIToken: "key", ServerID: "remote"})
			resource := Resource{Type: kind, ID: "resource"}
			initial, err := client.ReadEnvironment(context.Background(), resource)
			if err != nil {
				t.Fatal(err)
			}
			edited := *initial.Text + "NEW=value\n"
			result, err := client.SaveEnvironment(context.Background(), resource, edited, initial.Revision)
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string]any{string(kind) + "Id": "resource", "env": edited, "createEnvFile": false}
			if kind == Application {
				expected["buildArgs"] = "MODE=build"
				expected["buildSecrets"] = nil
			}
			if !reflect.DeepEqual(payload, expected) {
				t.Fatalf("save payload %#v, want %#v", payload, expected)
			}
			if deploys != 0 || result.Text == nil || *result.Text != edited {
				t.Fatalf("save deployed or lost raw text: %#v", result)
			}
			// The revision a save returns must match what Dokploy serves next,
			// or a second save from the same panel is rejected as stale.
			if _, err := client.SaveEnvironment(context.Background(), resource, edited+"MORE=value\n", result.Revision); err != nil {
				t.Fatalf("second save: %v", err)
			}
			if err := client.Deploy(context.Background(), resource); err != nil {
				t.Fatal(err)
			}
			if deploys != 1 {
				t.Fatalf("deploy requests=%d", deploys)
			}
		})
	}
}

func TestStaleOrIncompleteConfigurationNeverWrites(t *testing.T) {
	for _, scenario := range []string{"stale environment", "stale build settings", "moved server", "missing env", "missing build secrets", "null env"} {
		t.Run(scenario, func(t *testing.T) {
			rec := map[string]any{"applicationId": "resource", "appName": "web", "serverId": nil, "env": "KEY=old", "buildArgs": nil, "buildSecrets": nil, "createEnvFile": true}
			writes := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
					_, _ = w.Write([]byte("true"))
					return
				}
				_ = json.NewEncoder(w).Encode(rec)
			}))
			defer upstream.Close()
			client := NewClient(config.DokployHostConfig{APIURL: upstream.URL})
			resource := Resource{Type: Application, ID: "resource"}
			initial, err := client.ReadEnvironment(context.Background(), resource)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "stale environment":
				rec["env"] = "KEY=someone-else"
			case "stale build settings":
				rec["buildSecrets"] = "SECRET=new"
			case "moved server":
				rec["serverId"] = "other"
			case "missing env":
				delete(rec, "env")
			case "missing build secrets":
				delete(rec, "buildSecrets")
			case "null env":
				rec["env"] = nil
			}
			_, err = client.SaveEnvironment(context.Background(), resource, "KEY=mine", initial.Revision)
			if err == nil || writes != 0 {
				t.Fatalf("err=%v writes=%d", err, writes)
			}
			if strings.HasPrefix(scenario, "stale") && !errors.Is(err, ErrStaleEnvironment) {
				t.Fatalf("want stale error, got %v", err)
			}
		})
	}
}

func TestDiscoveryReadsServerScopedRecords(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/project.all":
			_, _ = w.Write([]byte(`[{"name":"project","environments":[{"name":"production","applications":[{"applicationId":"remote"},{"applicationId":"local"}],"compose":[{"composeId":"compose"}]}]}]`))
		case "/api/application.one":
			if r.URL.Query().Get("applicationId") == "local" {
				_, _ = w.Write([]byte(`{"applicationId":"local","appName":"local-app","serverId":null}`))
			} else {
				_, _ = w.Write([]byte(`{"applicationId":"remote","appName":"remote-app","serverId":"server","previewDeployments":[{"appName":"preview-app"}]}`))
			}
		case "/api/compose.one":
			_, _ = w.Write([]byte(`{"composeId":"compose","appName":"stack","serverId":"server"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	client := NewClient(config.DokployHostConfig{APIURL: upstream.URL, ServerID: "server"})
	resources, err := client.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 || resources[0].AppName != "remote-app" || resources[1].Type != Compose || resources[0].Project != "project" || !reflect.DeepEqual(resources[0].PreviewNames, []string{"preview-app"}) {
		t.Fatalf("resources=%#v", resources)
	}
}

func TestDokployDoesNotForwardAPIKeyThroughRedirect(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true; _, _ = w.Write([]byte(`[]`)) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	client := NewClient(config.DokployHostConfig{APIURL: source.URL, APIToken: "secret"})
	if err := client.TestConnection(context.Background()); err == nil {
		t.Fatal("redirect should be rejected")
	}
	if forwarded {
		t.Fatal("API key was forwarded through redirect")
	}
}
