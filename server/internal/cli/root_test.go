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
)

// captureStderr runs fn and returns everything it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(data)
}

func TestUsageErrorJSONShape(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var code int
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{"-o", "json", "--no-such-flag"})
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload); err != nil {
		t.Fatalf("stderr is not a JSON error document: %v\nstderr: %q", err, stderr)
	}
	if !strings.Contains(payload.Error, "no-such-flag") {
		t.Errorf("error message %q should mention the bad flag", payload.Error)
	}
}

func TestUsageErrorPlainWhenOutputFlagUnparsed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var code int
	// The bad flag comes first, so parsing stops before -o json is seen;
	// the error must fall back to plain text.
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{"--no-such-flag", "-o", "json"})
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "Error: ") || !strings.Contains(stderr, "logdeck --help") {
		t.Errorf("expected plain-text usage error, got: %q", stderr)
	}
}

func TestGrepNoRunningContainersHint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"containers":[],"hosts":[],"hostErrors":[]}`)
	}))
	defer server.Close()

	var code int
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{"grep", "x", "--url", server.URL})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %q", code, stderr)
	}
	if !strings.Contains(stderr, "no running containers to search") {
		t.Errorf("expected empty-hint on stderr, got: %q", stderr)
	}
}

func TestGrepNoMatchesHint(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/containers", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"containers":[{"id":"abc123","names":["/web"],"state":"running","host":"prod"}],"hosts":[],"hostErrors":[]}`)
	})
	mux.HandleFunc("/api/v1/logs/aggregate", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"logs":[],"count":0}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var code int
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{"grep", "x", "--url", server.URL, "--since", "30m"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %q", code, stderr)
	}
	if !strings.Contains(stderr, "no matches in 1 containers since 30m") {
		t.Errorf("expected no-matches hint on stderr, got: %q", stderr)
	}
}

func TestGrepReportsMissingSources(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/containers", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"containers":[{"id":"web1","names":["/web"],"state":"running","host":"prod"},{"id":"api1","names":["/api"],"state":"running","host":"prod"}],"hosts":[],"hostErrors":[]}`)
	})
	mux.HandleFunc("/api/v1/logs/aggregate", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"logs":[],"count":0,"incomplete":true,"sources":[`+
			`{"host":"prod","containerId":"web1","containerName":"web","status":"no_matches"},`+
			`{"host":"prod","containerId":"api1","containerName":"api","status":"failed","error":"daemon exploded"}]}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var code int
	stderr := captureStderr(t, func() {
		code = execute(context.Background(), "test", []string{"grep", "x", "--url", server.URL, "--since", "30m"})
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %q", code, stderr)
	}
	if !strings.Contains(stderr, "logs of api on prod could not be read: daemon exploded") || !strings.Contains(stderr, "no matches in 1 containers") {
		t.Errorf("expected a missing-source warning and a searched count of 1, got: %q", stderr)
	}

	stdout := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{"grep", "x", "--url", server.URL, "-o", "json"})
	})
	var out struct {
		Incomplete bool        `json:"incomplete"`
		Sources    []logSource `json:"sources"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("bad json %q: %v", stdout, err)
	}
	if code != 0 || !out.Incomplete || len(out.Sources) != 2 {
		t.Errorf("json output must carry coverage, got code %d: %s", code, stdout)
	}
}
