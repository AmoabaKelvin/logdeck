package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}

func TestAlertRuleCreateLogPayload(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/alerts/rules" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"r1","name":"errors","type":"log"}`)
	}))
	defer server.Close()

	code := execute(context.Background(), "test", []string{
		"alerts", "rules", "create", "--type", "log", "--name", "errors",
		"--host", "prod", "--container", "web", "--min-level", "ERROR",
		"--pattern", "timeout", "--threshold", "5", "--window", "60s",
		"--cooldown", "5m", "--url", server.URL,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	want := map[string]any{
		"name":            "errors",
		"enabled":         true,
		"type":            "log",
		"minLevel":        "ERROR",
		"pattern":         "timeout",
		"threshold":       float64(5),
		"windowSeconds":   float64(60),
		"cooldownSeconds": float64(300),
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("body[%q] = %v, want %v", key, body[key], value)
		}
	}
	if hosts := fmt.Sprintf("%v", body["hosts"]); hosts != "[prod]" {
		t.Errorf("body[hosts] = %v, want [prod]", body["hosts"])
	}
	if containers := fmt.Sprintf("%v", body["containers"]); containers != "[web]" {
		t.Errorf("body[containers] = %v, want [web]", body["containers"])
	}
	for _, absent := range []string{"events", "projects", "id"} {
		if _, ok := body[absent]; ok {
			t.Errorf("body should not contain %q, got %v", absent, body[absent])
		}
	}
}

func TestAlertRuleCreateEventPayload(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"r2"}`)
	}))
	defer server.Close()

	code := execute(context.Background(), "test", []string{
		"alerts", "rules", "create", "--type", "event", "--name", "crashes",
		"--events", "die,oom", "--window", "120", "--disabled",
		"--url", server.URL,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if events := fmt.Sprintf("%v", body["events"]); events != "[die oom]" {
		t.Errorf("body[events] = %v, want [die oom]", body["events"])
	}
	if body["enabled"] != false {
		t.Errorf("body[enabled] = %v, want false", body["enabled"])
	}
	if body["windowSeconds"] != float64(120) {
		t.Errorf("body[windowSeconds] = %v, want 120", body["windowSeconds"])
	}
	for _, absent := range []string{"minLevel", "pattern", "threshold", "cooldownSeconds"} {
		if _, ok := body[absent]; ok {
			t.Errorf("body should not contain %q, got %v", absent, body[absent])
		}
	}
}

func TestParseSecondsFlag(t *testing.T) {
	tests := []struct {
		value   string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"60s", 60, false},
		{"5m", 300, false},
		{"90", 90, false},
		{"1h", 3600, false},
		{"-5", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseSecondsFlag("window", tt.value)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSecondsFlag(%q) expected error, got %d", tt.value, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSecondsFlag(%q) unexpected error: %v", tt.value, err)
		} else if got != tt.want {
			t.Errorf("parseSecondsFlag(%q) = %d, want %d", tt.value, got, tt.want)
		}
	}
}

func TestAlertRuleCreateUsageErrorsBeforeHTTP(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"min-level on event", []string{"--type", "event", "--name", "x", "--events", "die", "--min-level", "ERROR"}},
		{"pattern on event", []string{"--type", "event", "--name", "x", "--events", "die", "--pattern", "re"}},
		{"events on log", []string{"--type", "log", "--name", "x", "--events", "die"}},
		{"missing events on event", []string{"--type", "event", "--name", "x"}},
		{"unknown event", []string{"--type", "event", "--name", "x", "--events", "start"}},
		{"missing name", []string{"--type", "log"}},
		{"missing type", []string{"--name", "x"}},
		{"invalid type", []string{"--type", "bogus", "--name", "x"}},
		{"invalid window", []string{"--type", "log", "--name", "x", "--window", "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			requested := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested = true
			}))
			defer server.Close()

			args := append([]string{"alerts", "rules", "create"}, tt.args...)
			args = append(args, "--url", server.URL)
			var code int
			captureStderr(t, func() {
				code = execute(context.Background(), "test", args)
			})

			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if requested {
				t.Error("usage error should not reach the server")
			}
		})
	}
}

func TestAlertRuleEnableRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var putBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts/rules", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rules":[{"id":"r1","name":"errors","enabled":false,"type":"log","createdAt":"2026-07-01T00:00:00Z"}]}`)
	})
	mux.HandleFunc("/api/v1/alerts/rules/r1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	code := execute(context.Background(), "test", []string{
		"alerts", "rules", "enable", "r1", "--url", server.URL,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if putBody["enabled"] != true {
		t.Errorf("putBody[enabled] = %v, want true", putBody["enabled"])
	}
	// The full rule round-trips: fields the CLI doesn't model must survive.
	if putBody["createdAt"] != "2026-07-01T00:00:00Z" {
		t.Errorf("putBody[createdAt] = %v, want original value preserved", putBody["createdAt"])
	}
	if putBody["name"] != "errors" {
		t.Errorf("putBody[name] = %v, want errors", putBody["name"])
	}
}

func TestAlertRuleDisableUnknownID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rules":[]}`)
	}))
	defer server.Close()

	var code int
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{
			"alerts", "rules", "disable", "nope", "--url", server.URL,
		})
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `no rule with id "nope"`) {
		t.Errorf("expected unknown-id error, got: %q", stderr)
	}
}

func TestAlertHistoryTable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	now := time.Now().UTC()
	older := now.Add(-time.Hour).Format(time.RFC3339)
	newer := now.Add(-3 * time.Minute).Format(time.RFC3339)

	var gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		fmt.Fprintf(w, `{"alerts":[
			{"id":"a1","ruleName":"older-rule","host":"prod","containerName":"web","reason":"exited","suppressed":0,"firedAt":%q,"delivery":{"status":"ok","httpStatus":200,"error":""}},
			{"id":"a2","ruleName":"newer-rule","host":"prod","containerName":"api","reason":"oom","suppressed":3,"firedAt":%q,"delivery":{"status":"failed","httpStatus":500,"error":"boom"}}
		],"count":2}`, older, newer)
	}))
	defer server.Close()

	var code int
	stdout := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{
			"alerts", "history", "--limit", "5", "--url", server.URL,
		})
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotLimit != "5" {
		t.Errorf("limit query = %q, want 5", gotLimit)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d lines:\n%s", len(lines), stdout)
	}
	for _, column := range []string{"TIME", "RULE", "CONTAINER@HOST", "REASON", "SUPPRESSED", "DELIVERY"} {
		if !strings.Contains(lines[0], column) {
			t.Errorf("header missing %q: %q", column, lines[0])
		}
	}
	if !strings.Contains(lines[1], "newer-rule") {
		t.Errorf("first row should be the newest alert, got: %q", lines[1])
	}
	if !strings.Contains(lines[1], "api@prod") || !strings.Contains(lines[1], "failed (HTTP 500): boom") {
		t.Errorf("newest row missing container@host or delivery summary: %q", lines[1])
	}
	if !strings.Contains(lines[2], "older-rule") || !strings.Contains(lines[2], "ok (HTTP 200)") {
		t.Errorf("second row should be the older alert: %q", lines[2])
	}
}

func TestAlertTestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantCode int
	}{
		{"ok", `{"status":"ok","httpStatus":200,"error":""}`, 0},
		{"failed", `{"status":"failed","httpStatus":500,"error":"boom"}`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/alerts/test" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				fmt.Fprint(w, tt.response)
			}))
			defer server.Close()

			var code int
			captureStdout(t, func() {
				captureStderr(t, func() {
					code = execute(context.Background(), "test", []string{
						"alerts", "test", "--url", server.URL,
					})
				})
			})
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestAlertRulesTableCooldown(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rules":[
			{"id":"r1","name":"default-cd","enabled":true,"type":"event","events":["die"]},
			{"id":"r2","name":"explicit-cd","enabled":true,"type":"log","minLevel":"ERROR","threshold":3,"windowSeconds":60,"cooldownSeconds":120}
		]}`)
	}))
	defer server.Close()

	var code int
	stdout := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{
			"alerts", "rules", "--url", server.URL,
		})
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d lines:\n%s", len(lines), stdout)
	}
	// cooldownSeconds absent/0 means the server default of 300s; the trigger
	// column must say so instead of hiding the cooldown.
	if !strings.Contains(lines[1], "cooldown 300s (default)") {
		t.Errorf("default-cooldown row missing default marker: %q", lines[1])
	}
	if !strings.Contains(lines[2], "cooldown 120s") || strings.Contains(lines[2], "default") {
		t.Errorf("explicit-cooldown row wrong: %q", lines[2])
	}
}

func TestAlertRulesJSONPassthrough(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rules":[{"id":"r1","name":"errors","enabled":true,"type":"log","futureField":42}]}`)
	}))
	defer server.Close()

	var code int
	stdout := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{
			"alerts", "rules", "-o", "json", "--url", server.URL,
		})
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var doc struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout: %q", err, stdout)
	}
	if len(doc.Rules) != 1 || doc.Rules[0]["id"] != "r1" {
		t.Fatalf("unexpected rules document: %q", stdout)
	}
	// -o json passes the server's list through, keeping unmodeled fields.
	if doc.Rules[0]["futureField"] != float64(42) {
		t.Errorf("futureField = %v, want 42 (raw passthrough)", doc.Rules[0]["futureField"])
	}
}

func TestAlertWebhookNotSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url":""}`)
	}))
	defer server.Close()

	var code int
	stdout := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{
			"alerts", "webhook", "--url", server.URL,
		})
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "(not set)" {
		t.Errorf("stdout = %q, want (not set)", stdout)
	}
}
