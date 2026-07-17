package api

import (
	"fmt"
	"net/http/httptest"
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
