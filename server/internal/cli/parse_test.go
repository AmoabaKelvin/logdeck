package cli

import (
	"testing"
	"time"
)

func TestParseTimeArg(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}, // RFC3339 passes through
		{"30s", "2026-01-02T11:59:30Z"},
		{"15m", "2026-01-02T11:45:00Z"},
		{"2h", "2026-01-02T10:00:00Z"},
		{"1d", "2026-01-01T12:00:00Z"},
		{"1h30m", "2026-01-02T10:30:00Z"},
	}
	for _, tt := range tests {
		got, err := parseTimeArg(tt.in, now)
		if err != nil {
			t.Errorf("parseTimeArg(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTimeArg(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	for _, invalid := range []string{"tomorrow", "5x", "-1h", "h"} {
		if _, err := parseTimeArg(invalid, now); err == nil {
			t.Errorf("parseTimeArg(%q) expected error", invalid)
		}
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"512m", 512 << 20},
		{"512M", 512 << 20},
		{"512mb", 512 << 20},
		{"1g", 1 << 30},
		{"1.5g", 3 << 29},
		{"64k", 64 << 10},
		{"100", 100},
		{"100b", 100},
		{"0", 0},
	}
	for _, tt := range tests {
		got, err := parseMemory(tt.in)
		if err != nil {
			t.Errorf("parseMemory(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseMemory(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}

	for _, invalid := range []string{"", "abc", "-1g", "1t5"} {
		if _, err := parseMemory(invalid); err == nil {
			t.Errorf("parseMemory(%q) expected error", invalid)
		}
	}
}

func TestCpusToNano(t *testing.T) {
	if got := cpusToNano(1.5); got != 1_500_000_000 {
		t.Errorf("cpusToNano(1.5) = %d, want 1500000000", got)
	}
	if got := cpusToNano(0); got != 0 {
		t.Errorf("cpusToNano(0) = %d, want 0", got)
	}
}

func TestBatchStrings(t *testing.T) {
	items := make([]string, 45)
	for i := range items {
		items[i] = "x"
	}
	batches := batchStrings(items, 20)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	if len(batches[0]) != 20 || len(batches[1]) != 20 || len(batches[2]) != 5 {
		t.Errorf("unexpected batch sizes: %d, %d, %d", len(batches[0]), len(batches[1]), len(batches[2]))
	}
	if batchStrings(nil, 20) != nil {
		t.Error("expected nil for empty input")
	}
	if got := batchStrings([]string{"a"}, 20); len(got) != 1 || len(got[0]) != 1 {
		t.Errorf("single item should give one batch of one, got %v", got)
	}
}

func TestMergeByTimestamp(t *testing.T) {
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	entries := []logEntry{
		{Timestamp: base.Add(2 * time.Second), Message: "c"},
		{Timestamp: base, Message: "a"},
		{Timestamp: base.Add(time.Second), Message: "b"},
		{Timestamp: base, Message: "a2"}, // same timestamp: stable order preserved
	}
	mergeByTimestamp(entries)
	got := entries[0].Message + entries[1].Message + entries[2].Message + entries[3].Message
	if got != "aa2bc" {
		t.Errorf("unexpected merge order: %q", got)
	}
}
