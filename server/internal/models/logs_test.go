package models

import (
	"strings"
	"testing"
)

func TestGroupRelatedLogEntriesFoldsStructuredFieldsIntoPreviousEntry(t *testing.T) {
	entries := []LogEntry{
		ParseLogLine("2026-05-28T05:00:38.367Z [2026-05-28 05:00:38.367 +0000] INFO: Request received", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z service: \"api\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z requestId: \"e675e98a-8c63-42a2-b23b-925a4846b0c4\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z method: \"GET\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z path: \"/favicon.ico\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.368Z [2026-05-28 05:00:38.368 +0000] INFO: Request completed", "stdout"),
	}

	grouped := GroupRelatedLogEntries(entries)

	if len(grouped) != 2 {
		t.Fatalf("expected 2 grouped entries, got %d", len(grouped))
	}

	first := grouped[0]
	if first.Level != LogLevelInfo {
		t.Fatalf("expected first grouped entry to remain INFO, got %s", first.Level)
	}
	if first.ContinuationCount != 4 {
		t.Fatalf("expected 4 continuation lines, got %d", first.ContinuationCount)
	}
	if first.Fields["requestId"] != "\"e675e98a-8c63-42a2-b23b-925a4846b0c4\"" {
		t.Fatalf("expected requestId field to be captured, got %q", first.Fields["requestId"])
	}
	if !strings.Contains(first.Message, "Request received\nservice: \"api\"\nrequestId:") {
		t.Fatalf("expected continuation fields in message, got %q", first.Message)
	}

	if grouped[1].Level != LogLevelInfo || !strings.Contains(grouped[1].Message, "Request completed") {
		t.Fatalf("expected second grouped entry to be request completed INFO, got %#v", grouped[1])
	}
}

func TestGroupRelatedLogEntriesDoesNotFoldStandaloneUnknownMessages(t *testing.T) {
	entries := []LogEntry{
		ParseLogLine("2026-05-28T05:00:38.367Z INFO starting worker", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.368Z worker ready on port 3000", "stdout"),
	}

	grouped := GroupRelatedLogEntries(entries)

	if len(grouped) != 2 {
		t.Fatalf("expected standalone unknown message to remain separate, got %d entries", len(grouped))
	}
	if grouped[1].Level != LogLevelUnknown {
		t.Fatalf("expected second entry to remain UNKNOWN, got %s", grouped[1].Level)
	}
}
