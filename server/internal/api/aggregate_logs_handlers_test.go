package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseAggregateTargets(t *testing.T) {
	t.Run("single target with name", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/logs/aggregate?targets=prod~abc123~web", nil)
		targets, err := parseAggregateTargets(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].Host != "prod" || targets[0].ID != "abc123" || targets[0].Name != "web" {
			t.Fatalf("unexpected target: %+v", targets[0])
		}
	})

	t.Run("name is optional", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/logs/aggregate?targets=prod~abc123", nil)
		targets, err := parseAggregateTargets(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(targets) != 1 || targets[0].Name != "" {
			t.Fatalf("expected one nameless target, got %+v", targets)
		}
	})

	t.Run("comma-separated in one value", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/logs/aggregate?targets=prod~a~web,staging~b~db", nil)
		targets, err := parseAggregateTargets(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(targets) != 2 || targets[0].Host != "prod" || targets[1].Host != "staging" {
			t.Fatalf("unexpected targets: %+v", targets)
		}
	})

	t.Run("repeated params merge", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/logs/aggregate?targets=prod~a~web&targets=staging~b~db", nil)
		targets, err := parseAggregateTargets(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(targets) != 2 {
			t.Fatalf("expected 2 targets, got %d", len(targets))
		}
	})

	t.Run("blank entries are skipped", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/logs/aggregate?targets=prod~a~web,%20,staging~b~db", nil)
		targets, err := parseAggregateTargets(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(targets) != 2 {
			t.Fatalf("expected 2 targets after skipping blanks, got %d", len(targets))
		}
	})

	for _, tt := range []struct {
		name string
		path string
	}{
		{"missing targets param", "/logs/aggregate"},
		{"empty targets value", "/logs/aggregate?targets="},
		{"only whitespace", "/logs/aggregate?targets=%20"},
		{"missing id", "/logs/aggregate?targets=prod"},
		{"blank host", "/logs/aggregate?targets=~id~name"},
		{"blank id", "/logs/aggregate?targets=host~~name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.path, nil)
			if _, err := parseAggregateTargets(r); err == nil {
				t.Fatalf("expected error for %q", tt.path)
			}
		})
	}
}

func TestParseAggregateTargetsLimit(t *testing.T) {
	build := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf("prod~id%d~name%d", i, i)
		}
		return strings.Join(parts, ",")
	}

	// The documented cap is 20: exactly 20 is accepted, 21 is rejected.
	r := httptest.NewRequest("GET", "/logs/aggregate?targets="+build(maxAggregateTargets), nil)
	if targets, err := parseAggregateTargets(r); err != nil {
		t.Fatalf("expected %d targets to be accepted, got error: %v", maxAggregateTargets, err)
	} else if len(targets) != maxAggregateTargets {
		t.Fatalf("expected %d targets, got %d", maxAggregateTargets, len(targets))
	}

	r = httptest.NewRequest("GET", "/logs/aggregate?targets="+build(maxAggregateTargets+1), nil)
	if _, err := parseAggregateTargets(r); err == nil {
		t.Fatalf("expected %d targets to be rejected", maxAggregateTargets+1)
	}
}

// aggregateTestRouter serves /logs/aggregate against a fake engine with one
// healthy container (web), one whose daemon errors (api), one that no longer
// exists (gone), and one whose live stream breaks after a line (flaky).
func aggregateTestRouter(t *testing.T) http.Handler {
	t.Helper()
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/v1.47") {
		case "/_ping":
			w.Header().Set("API-Version", "1.47")
		case "/containers/web/json", "/containers/flaky/json":
			// A TTY container's logs are one unframed stream.
			_, _ = w.Write([]byte(`{"Id":"x","Config":{"Tty":true}}`))
		case "/containers/web/logs":
			if r.URL.Query().Get("follow") == "" {
				_, _ = w.Write([]byte("2026-07-09T12:00:00Z hello from web\n"))
			}
		case "/containers/flaky/logs":
			_, _ = w.Write([]byte("2026-07-09T12:00:01Z hello from flaky\n"))
			if r.URL.Query().Get("follow") == "" {
				return
			}
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close() // the chunked body never terminates
		case "/containers/api/json":
			http.Error(w, `{"message":"daemon exploded"}`, http.StatusInternalServerError)
		default:
			http.Error(w, `{"message":"No such container"}`, http.StatusNotFound)
		}
	}))
	return envRouterWithEngine(t, engine, nil, false)
}

type aggregateSource struct {
	Type        string `json:"type"`
	Host        string `json:"host"`
	ContainerID string `json:"containerId"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}

func getAggregate(router http.Handler, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/logs/aggregate?"+query, nil))
	return w
}

func TestAggregatedLogsReportSourceCoverage(t *testing.T) {
	w := getAggregate(aggregateTestRouter(t), "targets=local~web~web,local~api~api,local~gone~gone")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Logs       []map[string]any  `json:"logs"`
		Count      int               `json:"count"`
		Sources    []aggregateSource `json:"sources"`
		Incomplete bool              `json:"incomplete"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 || len(resp.Logs) != 1 {
		t.Fatalf("readable source's logs missing: %s", w.Body.String())
	}
	if !resp.Incomplete {
		t.Fatalf("a failed source must mark the result incomplete: %s", w.Body.String())
	}
	got := map[string]aggregateSource{}
	for _, source := range resp.Sources {
		got[source.ContainerID] = source
	}
	if got["web"].Status != "ok" || got["gone"].Status != "removed" {
		t.Fatalf("wrong statuses: %+v", resp.Sources)
	}
	if got["api"].Status != "failed" || !strings.Contains(got["api"].Error, "daemon exploded") || got["api"].Host != "local" {
		t.Fatalf("failed source not reported with its reason: %+v", got["api"])
	}
}

func TestAggregatedLogsTellNoMatchesFromFailure(t *testing.T) {
	w := getAggregate(aggregateTestRouter(t), "targets=local~web~web,local~gone~gone&search=nothing-matches")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Sources    []aggregateSource `json:"sources"`
		Incomplete bool              `json:"incomplete"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Incomplete || len(resp.Sources) != 2 || resp.Sources[0].Status != "no_matches" || resp.Sources[1].Status != "removed" {
		t.Fatalf("expected a complete result with no matches: %s", w.Body.String())
	}
}

func TestAggregatedLogsFailWhenNoSourceIsReadable(t *testing.T) {
	router := aggregateTestRouter(t)
	for _, query := range []string{"targets=local~api~api,local~gone~gone", "targets=local~api~api&follow=true"} {
		w := getAggregate(router, query)
		var resp struct {
			Error   string            `json:"error"`
			Sources []aggregateSource `json:"sources"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if w.Code != http.StatusBadGateway || !strings.Contains(resp.Error, "api on local: ") || len(resp.Sources) == 0 || resp.Sources[0].Status != "failed" {
			t.Fatalf("%s: expected 502 naming each failure, got %d: %s", query, w.Code, w.Body.String())
		}
	}
}

func TestAggregatedStreamReportsSourceStatus(t *testing.T) {
	w := getAggregate(aggregateTestRouter(t), "targets=local~web~web,local~api~api,local~flaky~flaky&follow=true")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	statuses := map[string][]string{}
	var lines int
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		var event aggregateSource
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
		if event.Type != "source" {
			lines++
			continue
		}
		statuses[event.ContainerID] = append(statuses[event.ContainerID], event.Status)
		if event.Status == "failed" && event.Error == "" {
			t.Fatalf("failure without a reason: %s", line)
		}
	}

	// History from web and flaky, then flaky's one live line.
	if lines != 3 {
		t.Fatalf("expected 3 log lines, got %d:\n%s", lines, w.Body.String())
	}
	want := map[string][]string{
		"web":   {"ok", "stopped"},
		"api":   {"failed"},
		"flaky": {"ok", "failed"},
	}
	if !reflect.DeepEqual(statuses, want) {
		t.Fatalf("source statuses = %v, want %v\n%s", statuses, want, w.Body.String())
	}
}
