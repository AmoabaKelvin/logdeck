package cli

import (
	"testing"
	"time"
)

func TestLogFlagsQuery(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("defaults and normalization", func(t *testing.T) {
		f := logFlags{tail: 100, level: "error", search: "boom", since: "15m"}
		q, err := f.query(now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.Get("tail") != "100" {
			t.Errorf("tail = %q, want 100", q.Get("tail"))
		}
		if q.Get("level") != "ERROR" {
			t.Errorf("level = %q, want ERROR (upper-cased)", q.Get("level"))
		}
		if q.Get("search") != "boom" {
			t.Errorf("search = %q, want boom", q.Get("search"))
		}
		if q.Get("since") != "2026-01-02T11:45:00Z" {
			t.Errorf("since = %q, want the resolved relative time", q.Get("since"))
		}
	})

	t.Run("empty optional flags are omitted", func(t *testing.T) {
		f := logFlags{tail: 100}
		q, err := f.query(now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, key := range []string{"level", "search", "since", "until"} {
			if _, ok := q[key]; ok {
				t.Errorf("expected %q to be omitted, got %q", key, q.Get(key))
			}
		}
	})

	t.Run("bad since is an error", func(t *testing.T) {
		f := logFlags{tail: 100, since: "yesterday"}
		if _, err := f.query(now); err == nil {
			t.Fatal("expected an error for an unparseable --since")
		}
	})

	t.Run("bad until is an error", func(t *testing.T) {
		f := logFlags{tail: 100, until: "nope"}
		if _, err := f.query(now); err == nil {
			t.Fatal("expected an error for an unparseable --until")
		}
	})
}
