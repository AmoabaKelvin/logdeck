package cli

import (
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0KiB"},
		{1536, "1.5KiB"},
		{1 << 20, "1.0MiB"},
		{1 << 30, "1.0GiB"},
		{1 << 40, "1.0TiB"},
		{1 << 50, "1.0PiB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatLogLine(t *testing.T) {
	ts := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	entry := logEntry{Timestamp: ts, Level: "ERROR", Message: "boom", ContainerName: "web"}

	// Without the name, the container column is omitted.
	if got := formatLogLine(entry, false); got != "2026-01-02T12:00:00Z ERROR   boom" {
		t.Errorf("formatLogLine(withName=false) = %q", got)
	}

	// With the name and a set ContainerName, the [name] column appears.
	if got := formatLogLine(entry, true); got != "2026-01-02T12:00:00Z ERROR   [web] boom" {
		t.Errorf("formatLogLine(withName=true) = %q", got)
	}

	// A zero timestamp renders as "-" instead of the zero time.
	noTS := logEntry{Level: "INFO", Message: "hi"}
	if got := formatLogLine(noTS, false); got != "- INFO    hi" {
		t.Errorf("formatLogLine(zero ts) = %q", got)
	}

	// withName but no ContainerName falls back to the nameless form.
	noName := logEntry{Timestamp: ts, Level: "INFO", Message: "hi"}
	if got := formatLogLine(noName, true); got != "2026-01-02T12:00:00Z INFO    hi" {
		t.Errorf("formatLogLine(withName, no name) = %q", got)
	}
}
