package docker

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestParseDockerLogsGroupsStructuredContinuationLines(t *testing.T) {
	var stream bytes.Buffer
	stdout := stdcopy.NewStdWriter(&stream, stdcopy.Stdout)
	_, err := stdout.Write([]byte(strings.Join([]string{
		"2026-05-28T05:00:38.367Z [2026-05-28 05:00:38.367 +0000] INFO: Request received",
		"2026-05-28T05:00:38.367Z service: \"api\"",
		"2026-05-28T05:00:38.367Z requestId: \"e675e98a-8c63-42a2-b23b-925a4846b0c4\"",
		"2026-05-28T05:00:38.368Z [2026-05-28 05:00:38.368 +0000] INFO: Request completed",
	}, "\n") + "\n"))
	if err != nil {
		t.Fatalf("failed to write docker log stream: %v", err)
	}

	entries, err := parseDockerLogs(&stream, "", nil)
	if err != nil {
		t.Fatalf("failed to parse docker logs: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 grouped entries, got %d", len(entries))
	}
	if entries[0].Level != models.LogLevelInfo {
		t.Fatalf("expected first entry to remain INFO, got %s", entries[0].Level)
	}
	if entries[0].ContinuationCount != 2 {
		t.Fatalf("expected first entry to include 2 continuation lines, got %d", entries[0].ContinuationCount)
	}
	if !strings.Contains(entries[0].Message, "Request received\nservice: \"api\"\nrequestId:") {
		t.Fatalf("expected grouped message to include continuation fields, got %q", entries[0].Message)
	}
}

func TestLogWriterLineBufferCap(t *testing.T) {
	tests := []struct {
		name        string
		writes      []string
		wantEntries int
	}{
		{
			name:        "line under cap stays buffered until flush",
			writes:      []string{strings.Repeat("a", 1024)},
			wantEntries: 0,
		},
		{
			name:        "oversized line flushed as its own entry",
			writes:      []string{strings.Repeat("a", maxLineBufferSize+1)},
			wantEntries: 1,
		},
		{
			name:        "oversized line accumulated across writes",
			writes:      []string{strings.Repeat("a", maxLineBufferSize), strings.Repeat("b", 10)},
			wantEntries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []models.LogEntry
			w := &logWriter{stream: "stdout", entries: &entries}
			for _, chunk := range tt.writes {
				if _, err := w.Write([]byte(chunk)); err != nil {
					t.Fatalf("write failed: %v", err)
				}
			}
			if len(entries) != tt.wantEntries {
				t.Fatalf("expected %d entries, got %d", tt.wantEntries, len(entries))
			}
			if len(w.buffer) > maxLineBufferSize {
				t.Fatalf("buffer exceeds cap: %d bytes", len(w.buffer))
			}
		})
	}
}

func TestStreamingLogWriterLineBufferCap(t *testing.T) {
	tests := []struct {
		name        string
		writes      []string
		wantEntries int
	}{
		{
			name:        "line under cap stays buffered until flush",
			writes:      []string{strings.Repeat("a", 1024)},
			wantEntries: 0,
		},
		{
			name:        "oversized line emitted as its own entry",
			writes:      []string{strings.Repeat("a", maxLineBufferSize+1)},
			wantEntries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var mu sync.Mutex
			_, pipeWriter := io.Pipe()
			w := &streamingLogWriter{
				stream:     "stdout",
				encoder:    json.NewEncoder(&out),
				encoderMu:  &mu,
				pipeWriter: pipeWriter,
			}
			for _, chunk := range tt.writes {
				if _, err := w.Write([]byte(chunk)); err != nil {
					t.Fatalf("write failed: %v", err)
				}
			}
			got := 0
			if out.Len() > 0 {
				got = strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1
			}
			if got != tt.wantEntries {
				t.Fatalf("expected %d emitted entries, got %d", tt.wantEntries, got)
			}
			if len(w.buffer) > maxLineBufferSize {
				t.Fatalf("buffer exceeds cap: %d bytes", len(w.buffer))
			}
		})
	}
}
