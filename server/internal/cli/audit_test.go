package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditListsFilteredEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v1/settings/audit" || q.Get("actor") != "token:ci" || q.Get("result") != "failure" || q.Get("since") == "" || q.Has("target") || q.Has("offset") {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, `{"entries":[{"time":"2026-01-01T00:00:00Z","actor":"token:ci","action":"container.stop","host":"prod","target":"web","status":403}],"total":1}`)
	}))
	defer server.Close()

	var code int
	out := captureStdout(t, func() {
		code = execute(context.Background(), "test", []string{"audit", "--actor", "token:ci", "--failed", "--since", "1d", "--url", server.URL})
	})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	for _, want := range []string{"token:ci", "container.stop", "web@prod", "403"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestAuditPagesWithOffset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "1" || q.Get("offset") != "500" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, `{"entries":[{"time":"2026-01-01T00:00:00Z","actor":"admin","action":"container.stop","target":"web","status":200}],"total":502}`)
	}))
	defer server.Close()

	var code int
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code = execute(context.Background(), "test", []string{"audit", "--limit", "1", "--offset", "500", "--url", server.URL})
		})
	})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "container.stop") {
		t.Errorf("output lacks the entry:\n%s", stdout)
	}
	if !strings.Contains(stderr, "--offset 501") {
		t.Errorf("stderr does not point at the next page:\n%s", stderr)
	}
}
