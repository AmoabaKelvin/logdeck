package models

import (
	"strings"
	"testing"
)

func TestDetectLogLevelUsesExplicitStructuredLevels(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    LogLevel
	}{
		{
			name:    "json string level",
			message: `{"level":"error","time":1716872438,"msg":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "pino numeric level",
			message: `{"level":30,"time":1716872438,"msg":"request completed"}`,
			want:    LogLevelInfo,
		},
		{
			name:    "otel severity text",
			message: `{"severity_text":"WARN","body":"retrying request"}`,
			want:    LogLevelWarn,
		},
		{
			name:    "otel camel case severity text",
			message: `{"severityText":"ERROR","body":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "otel numeric severity",
			message: `{"severityNumber":17,"body":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "logfmt level",
			message: `time=2026-05-28T05:00:38Z level=debug msg="cache hit"`,
			want:    LogLevelDebug,
		},
		{
			name:    "python level name",
			message: `{"levelname":"WARNING","message":"queue latency high"}`,
			want:    LogLevelWarn,
		},
		{
			name:    "logfmt otel numeric severity",
			message: `time=2026-05-28T05:00:38Z severity_number=21 msg="service unavailable"`,
			want:    LogLevelFatal,
		},
		{
			name:    "slog uppercase level",
			message: `time=2026-05-28T05:00:38Z level=ERROR msg="request failed"`,
			want:    LogLevelError,
		},
		{
			name:    "bracketed prefix",
			message: `[FATAL] database migration failed`,
			want:    LogLevelFatal,
		},
		{
			name:    "plain prefix",
			message: `WARNING -- queue latency high`,
			want:    LogLevelWarn,
		},
		{
			name:    "glog prefix",
			message: `E0528 05:00:38.367891 request failed`,
			want:    LogLevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectLogLevel(tt.message); got != tt.want {
				t.Fatalf("DetectLogLevel(%q) = %s, want %s", tt.message, got, tt.want)
			}
		})
	}
}

func TestDetectLogLevelKeepsMessageFallbacks(t *testing.T) {
	tests := []struct {
		message string
		want    LogLevel
	}{
		{message: "request failed after retry", want: LogLevelError},
		{message: "worker emitted notice event", want: LogLevelInfo},
		{message: "request completed successfully", want: LogLevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if got := DetectLogLevel(tt.message); got != tt.want {
				t.Fatalf("DetectLogLevel(%q) = %s, want %s", tt.message, got, tt.want)
			}
		})
	}
}

func TestParseLogLineDetectsExplicitLevelAfterTimestamp(t *testing.T) {
	entry := ParseLogLine(`2026-05-28T05:00:38.367Z {"level":50,"msg":"request failed"}`, "stdout")

	if entry.Level != LogLevelError {
		t.Fatalf("expected ERROR from JSON level after Docker timestamp, got %s", entry.Level)
	}
}

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
