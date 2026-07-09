package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
				wroteEntry: new(atomic.Bool),
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

// sendTick delivers a monitor tick, failing fast instead of deadlocking if the
// monitor stopped consuming.
func sendTick(t *testing.T, tick chan<- time.Time) {
	t.Helper()
	select {
	case tick <- time.Time{}:
	case <-time.After(5 * time.Second):
		t.Fatal("follow monitor did not consume tick")
	}
}

func TestFollowMonitorTearsDownStreamWhenDaemonDies(t *testing.T) {
	// A raw stream that never delivers data and never closes, like the socket
	// to a daemon that died mid-restart.
	rawReader, _ := io.Pipe()
	tick := make(chan time.Time)

	stream := newParsedLogStream(context.Background(), rawReader, models.LogOptions{Follow: true},
		func(context.Context) error { return errors.New("daemon down") }, tick)

	type result struct {
		out string
		err error
	}
	results := make(chan result, 1)
	go func() {
		out, err := io.ReadAll(stream)
		results <- result{string(out), err}
	}()

	sendTick(t, tick) // heartbeat + first failed ping
	sendTick(t, tick) // heartbeat + second failed ping: daemon declared dead

	select {
	case res := <-results:
		if res.err == nil || !strings.Contains(res.err.Error(), "docker daemon unreachable") {
			t.Fatalf("expected daemon-unreachable error, got %v", res.err)
		}
		if got := strings.Count(res.out, `"type":"heartbeat"`); got != 2 {
			t.Fatalf("expected 2 heartbeats before teardown, got %d in %q", got, res.out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not terminate after the daemon died")
	}
}

func TestFollowMonitorHeartbeatsOnlyWhenIdleAndStopsAfterClose(t *testing.T) {
	rawReader, rawWriter := io.Pipe()
	tick := make(chan time.Time)

	stream := newParsedLogStream(context.Background(), rawReader, models.LogOptions{Follow: true},
		func(context.Context) error { return nil }, tick)
	reader := bufio.NewReader(stream)

	go func() {
		stdw := stdcopy.NewStdWriter(rawWriter, stdcopy.Stdout)
		stdw.Write([]byte("hello world\n"))
	}()
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, "hello world") {
		t.Fatalf("expected log entry, got %q (err %v)", line, err)
	}

	sendTick(t, tick) // an entry was written this interval: no heartbeat
	sendTick(t, tick) // idle interval: heartbeat
	line, err = reader.ReadString('\n')
	if err != nil || !strings.Contains(line, `"type":"heartbeat"`) {
		t.Fatalf("expected heartbeat after idle tick, got %q (err %v)", line, err)
	}

	// The stream ends normally. If the first tick had wrongly produced a
	// heartbeat, an extra heartbeat line would be read here instead of EOF.
	rawWriter.Close()
	if line, err = reader.ReadString('\n'); err != io.EOF {
		t.Fatalf("expected EOF after the log stream ended, got %q (err %v)", line, err)
	}

	// The monitor exits once the stream ends: it either sees the done channel
	// or consumes one final tick whose heartbeat write fails on the closed
	// pipe. Two more ticks can therefore never both be consumed.
	consumed := 0
	for i := 0; i < 2; i++ {
		select {
		case tick <- time.Time{}:
			consumed++
		case <-time.After(100 * time.Millisecond):
		}
	}
	if consumed == 2 {
		t.Fatal("follow monitor kept running after the stream ended")
	}
}
