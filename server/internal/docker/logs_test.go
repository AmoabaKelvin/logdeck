package docker

import (
	"bytes"
	"strings"
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
