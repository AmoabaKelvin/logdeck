package docker

import (
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func TestMergeLogEntriesByTimestamp(t *testing.T) {
	at := func(sec int) time.Time {
		return time.Date(2026, 7, 9, 12, 0, sec, 0, time.UTC)
	}
	entry := func(name string, ts time.Time, msg string) models.LogEntry {
		return models.LogEntry{Timestamp: ts, Message: msg, ContainerName: name}
	}

	batches := [][]models.LogEntry{
		{
			entry("api", at(1), "api-1"),
			entry("api", at(3), "api-3"),
			entry("api", at(3), "api-3b"),
		},
		{
			entry("db", at(0), "db-0"),
			entry("db", at(2), "db-2"),
			entry("db", at(3), "db-3"),
		},
		nil, // a failed target contributes nothing
	}

	merged := mergeLogEntriesByTimestamp(batches)

	wantMessages := []string{"db-0", "api-1", "db-2", "api-3", "api-3b", "db-3"}
	if len(merged) != len(wantMessages) {
		t.Fatalf("expected %d merged entries, got %d", len(wantMessages), len(merged))
	}
	for i, want := range wantMessages {
		if merged[i].Message != want {
			t.Fatalf("entry %d: expected %q, got %q", i, want, merged[i].Message)
		}
	}

	for i := 1; i < len(merged); i++ {
		if merged[i].Timestamp.Before(merged[i-1].Timestamp) {
			t.Fatalf("entries out of timestamp order at index %d", i)
		}
	}
}

func TestMergeLogEntriesByTimestampEmpty(t *testing.T) {
	if got := mergeLogEntriesByTimestamp(nil); len(got) != 0 {
		t.Fatalf("expected empty result for nil batches, got %d entries", len(got))
	}
	if got := mergeLogEntriesByTimestamp([][]models.LogEntry{nil, {}}); len(got) != 0 {
		t.Fatalf("expected empty result for empty batches, got %d entries", len(got))
	}
}

func TestTagLogEntries(t *testing.T) {
	entries := []models.LogEntry{{Message: "a"}, {Message: "b"}}
	tagLogEntries(entries, LogTarget{Host: "local", ID: "abc123", Name: "web"})

	for i, entry := range entries {
		if entry.ContainerID != "abc123" || entry.ContainerName != "web" {
			t.Fatalf("entry %d not tagged: %+v", i, entry)
		}
	}
}
