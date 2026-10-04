package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const cliSavedEnv = `{"source":"coolify","env":{"KEY":"{{project.SECRET}}"},"variables":[
 {"uuid":"prod","key":"KEY","value":"{{project.SECRET}}","is_preview":false,"is_buildtime":true,"is_runtime":false,"is_shared":true},
 {"uuid":"preview","key":"KEY","value":"preview-value","is_preview":true},
 {"uuid":"hidden","key":"TOKEN","value":null,"is_shown_once":true}
]}`

// Exercise resolution and the HTTP client instead of supplying resolved IDs.
func envCLIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/containers" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"containers":[{"id":"container","names":["/web"],"host":"remote"}]}`))
			return
		}
		if r.URL.Path != "/api/v1/containers/container/env" || r.URL.Query().Get("host") != "remote" {
			t.Errorf("wrong target: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestEnvCommandShowsSavedScopesAndUnknownValues(t *testing.T) {
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			server := envCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("read used %s", r.Method)
				}
				_, _ = w.Write([]byte(cliSavedEnv))
			})
			output := captureStdout(t, func() {
				if code := execute(context.Background(), "test", []string{"env", "web", "--host", "remote", "--url", server.URL, "--token", "", "-o", format}); code != 0 {
					t.Fatalf("exit %d", code)
				}
			})
			if format == "table" {
				want := "KEY [production]={{project.SECRET}}\nKEY [preview]=preview-value\nTOKEN [production]=<unknown>\n"
				if output != want {
					t.Fatalf("scopes or unknown value lost: %q", output)
				}
				return
			}
			assertSavedEnvDocument(t, []byte(output))
		})
	}
}

func assertSavedEnvDocument(t *testing.T, data []byte) {
	t.Helper()
	var doc struct {
		Source    string
		Variables []struct {
			UUID    string
			Value   *string
			Preview bool `json:"is_preview"`
			Build   bool `json:"is_buildtime"`
			Runtime bool `json:"is_runtime"`
			Shared  bool `json:"is_shared"`
		}
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Source != "coolify" || len(doc.Variables) != 3 {
		t.Fatalf("records lost: %s", data)
	}
	prod, preview, hidden := doc.Variables[0], doc.Variables[1], doc.Variables[2]
	if prod.UUID != "prod" || prod.Value == nil || *prod.Value != "{{project.SECRET}}" || !prod.Build || prod.Runtime || !prod.Shared || preview.UUID != "preview" || !preview.Preview || preview.Value == nil || *preview.Value != "preview-value" || hidden.UUID != "hidden" || hidden.Value != nil {
		t.Fatalf("identities, settings or unknown values lost: %s", data)
	}
}

func envMCPSession(t *testing.T, serverURL string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "logdeck", Version: "test"}, nil)
	registerMCPTools(server, &app{client: newClient(serverURL, "")})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func TestMCPGetEnvReturnsSavedRecords(t *testing.T) {
	server := envCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("read used %s", r.Method)
		}
		_, _ = w.Write([]byte(cliSavedEnv))
	})
	ctx, session := envMCPSession(t, server.URL)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_env", Arguments: map[string]any{"container": "web", "host": "remote"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("get_env failed: %+v", result)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type %T", result.Content[0])
	}
	assertSavedEnvDocument(t, []byte(content.Text))
}

func TestMCPSetEnvForwardsExplicitChangesAndReportsFailure(t *testing.T) {
	changes := []any{
		map[string]any{"uuid": "preview", "key": "KEY", "expected_value": "preview-value", "value": "edited", "is_preview": true},
		map[string]any{"uuid": "hidden", "key": "TOKEN", "expected_value": nil, "is_preview": false, "remove": true},
	}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "partial failure"}[fail], func(t *testing.T) {
			var payload map[string]any
			writes := 0
			server := envCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("save used %s", r.Method)
				}
				writes++
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				if fail {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"saved":false,"completed":1,"message":"deletion failed; some changes may have been saved"}`))
					return
				}
				_, _ = w.Write([]byte(`{"saved":true,"applied":false,"completed":2}`))
			})
			ctx, session := envMCPSession(t, server.URL)
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "set_env", Arguments: map[string]any{"container": "web", "host": "remote", "changes": changes}})
			if err != nil {
				t.Fatal(err)
			}
			if writes != 1 || !reflect.DeepEqual(payload, map[string]any{"changes": changes}) {
				t.Fatalf("changes became a replacement map or were altered: %v", payload)
			}
			if result.IsError != fail || len(result.Content) != 1 {
				t.Fatalf("outcome lost: %+v", result)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("content type %T", result.Content[0])
			}
			if fail {
				if !strings.Contains(content.Text, "deletion failed") || !strings.Contains(content.Text, `"completed":1`) {
					t.Fatalf("partial failure hidden: %s", content.Text)
				}
			} else {
				var saved struct {
					Saved     bool
					Applied   bool
					Completed int
				}
				if err := json.Unmarshal([]byte(content.Text), &saved); err != nil {
					t.Fatal(err)
				}
				if !saved.Saved || saved.Applied || saved.Completed != 2 {
					t.Fatalf("save reported as applied: %s", content.Text)
				}
			}
		})
	}
}

func TestMCPSetEnvRejectsAmbiguousInputBeforeHTTP(t *testing.T) {
	for _, both := range []bool{false, true} {
		t.Run(map[bool]string{false: "neither mode", true: "both modes"}[both], func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(server.Close)
			args := map[string]any{"container": "web"}
			if both {
				args["env"] = map[string]string{"KEY": "replacement"}
				args["changes"] = []any{map[string]any{"key": "KEY", "value": "edit", "expected_value": nil, "is_preview": false}}
			}
			ctx, session := envMCPSession(t, server.URL)
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "set_env", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || requests != 0 || len(result.Content) != 1 {
				t.Fatalf("invalid modes reached HTTP: result=%+v requests=%d", result, requests)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok || !strings.Contains(content.Text, "provide env for Docker or changes for Coolify") {
				t.Fatalf("wrong rejection: %+v", result.Content)
			}
		})
	}
}

func TestMCPSetEnvPreservesDockerReplacementMap(t *testing.T) {
	for _, env := range []map[string]string{{"A": "one=two\nthree"}, {}} {
		t.Run(map[bool]string{true: "clear all", false: "replace values"}[len(env) == 0], func(t *testing.T) {
			var payload map[string]any
			writes := 0
			server := envCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("replacement used %s", r.Method)
				}
				writes++
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(`{"new_container_id":"replacement"}`))
			})
			ctx, session := envMCPSession(t, server.URL)
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "set_env", Arguments: map[string]any{"container": "web", "host": "remote", "env": env}})
			if err != nil {
				t.Fatal(err)
			}
			wantEnv := map[string]any{}
			for key, value := range env {
				wantEnv[key] = value
			}
			if result.IsError || writes != 1 || !reflect.DeepEqual(payload, map[string]any{"env": wantEnv}) {
				t.Fatalf("Docker replacement map rejected or altered: result=%+v payload=%v", result, payload)
			}
		})
	}
}
