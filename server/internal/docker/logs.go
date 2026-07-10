package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// Follow-mode liveness monitor knobs. A daemon that dies without closing the
// socket leaves StdCopy blocked forever (issue #53); each monitor tick emits a
// heartbeat if nothing was written since the previous tick (so the client can
// tell a quiet stream from a dead one) and pings the daemon so a dead socket
// tears the stream down instead of hanging.
const (
	monitorInterval = 15 * time.Second
	pingTimeout     = 3 * time.Second
	maxPingFailures = 2
)

// parseDockerLogs parses the Docker log stream into structured entries,
// optionally filtering by level and/or search regex.
func parseDockerLogs(reader io.Reader, levelFilter string, searchRegex *regexp.Regexp) ([]models.LogEntry, error) {
	var entries []models.LogEntry

	stdout := &logWriter{stream: "stdout", entries: &entries}
	stderr := &logWriter{stream: "stderr", entries: &entries}

	_, err := stdcopy.StdCopy(stdout, stderr, reader)
	if err != nil && err != io.EOF {
		return nil, err
	}

	stdout.Flush()
	stderr.Flush()
	entries = models.GroupRelatedLogEntries(entries)

	if levelFilter == "" && searchRegex == nil {
		return entries, nil
	}

	filtered := make([]models.LogEntry, 0, len(entries))
	for _, e := range entries {
		if levelFilter != "" && !strings.EqualFold(string(e.Level), levelFilter) {
			continue
		}
		if searchRegex != nil && !searchRegex.MatchString(e.Message) && !searchRegex.MatchString(e.Raw) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

// maxLineBufferSize caps the pending (newline-less) line buffer in the log
// writers. An oversized chunk is flushed as its own log entry so a single
// line without a newline cannot grow without bound.
const maxLineBufferSize = 1 << 20 // 1 MiB

// logWriter implements io.Writer and parses log lines
type logWriter struct {
	stream  string
	entries *[]models.LogEntry
	buffer  []byte
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	for {
		idx := bytes.IndexByte(w.buffer, '\n')
		if idx == -1 {
			break
		}

		line := string(w.buffer[:idx])
		w.buffer = w.buffer[idx+1:]

		if line != "" {
			line = strings.TrimSuffix(line, "\r")
			entry := models.ParseLogLine(line, w.stream)
			*w.entries = append(*w.entries, entry)
		}
	}

	if len(w.buffer) > maxLineBufferSize {
		w.Flush()
	}

	return len(p), nil
}

func (w *logWriter) Flush() {
	if len(w.buffer) == 0 {
		return
	}
	line := strings.TrimSuffix(string(w.buffer), "\r")
	if line != "" {
		entry := models.ParseLogLine(line, w.stream)
		*w.entries = append(*w.entries, entry)
	}
	w.buffer = nil
}

type streamingLogWriter struct {
	stream      string
	buffer      []byte
	encoder     *json.Encoder
	encoderMu   *sync.Mutex
	pipeWriter  *io.PipeWriter
	levelFilter string
	searchRegex *regexp.Regexp
	wroteEntry  *atomic.Bool // set on each encoded entry; monitor clears it per tick
}

func (w *streamingLogWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	for {
		idx := bytes.IndexByte(w.buffer, '\n')
		if idx == -1 {
			break
		}

		line := string(w.buffer[:idx])
		w.buffer = w.buffer[idx+1:]

		if line != "" {
			line = strings.TrimSuffix(line, "\r")
			if encodeErr := w.emit(line); encodeErr != nil {
				return 0, encodeErr
			}
		}
	}

	if len(w.buffer) > maxLineBufferSize {
		line := strings.TrimSuffix(string(w.buffer), "\r")
		w.buffer = nil
		if encodeErr := w.emit(line); encodeErr != nil {
			return 0, encodeErr
		}
	}

	return len(p), nil
}

func (w *streamingLogWriter) Flush() {
	if len(w.buffer) == 0 {
		return
	}
	line := strings.TrimSuffix(string(w.buffer), "\r")
	if line != "" {
		_ = w.emit(line)
	}
	w.buffer = nil
}

func (w *streamingLogWriter) emit(line string) error {
	entry := models.ParseLogLine(line, w.stream)

	if w.levelFilter != "" && !strings.EqualFold(string(entry.Level), w.levelFilter) {
		return nil
	}

	if w.searchRegex != nil && !w.searchRegex.MatchString(entry.Message) && !w.searchRegex.MatchString(entry.Raw) {
		return nil
	}

	w.encoderMu.Lock()
	// Set before Encode (which blocks on the pipe) so a monitor tick during
	// the write already counts it as activity.
	w.wroteEntry.Store(true)
	err := w.encoder.Encode(entry)
	w.encoderMu.Unlock()

	if err != nil {
		w.pipeWriter.CloseWithError(err)
	}

	return err
}

func buildLogsOptions(options models.LogOptions, follow, timestamps bool) container.LogsOptions {
	return container.LogsOptions{
		Follow:     follow,
		Timestamps: timestamps,
		Details:    options.Details,
		Since:      options.Since,
		Until:      options.Until,
		Tail:       options.Tail,
		ShowStdout: options.ShowStdout,
		ShowStderr: options.ShowStderr,
	}
}

func (c *MultiHostClient) GetContainerLogsParsed(ctx context.Context, hostName, id string, options models.LogOptions) ([]models.LogEntry, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	logs, err := apiClient.ContainerLogs(ctx, id, buildLogsOptions(options, false, true))
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	var searchRegex *regexp.Regexp
	if options.Search != "" {
		searchRegex, _ = regexp.Compile(options.Search) // already validated by handler
	}

	return parseDockerLogs(logs, options.Level, searchRegex)
}

// StreamContainerLogsParsed streams parsed logs. The Docker log stream is tied
// to ctx, so a disconnected client cancels the follow-mode stream.
func (c *MultiHostClient) StreamContainerLogsParsed(ctx context.Context, hostName, id string, options models.LogOptions) (io.ReadCloser, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	logs, err := apiClient.ContainerLogs(ctx, id, buildLogsOptions(options, options.Follow, true))
	if err != nil {
		return nil, err
	}

	ping := func(ctx context.Context) error {
		_, err := apiClient.Ping(ctx)
		return err
	}
	return newParsedLogStream(ctx, logs, options, ping, nil), nil
}

// newParsedLogStream parses the raw Docker log stream into NDJSON on a pipe.
// When following, a monitor goroutine keeps the stream honest: heartbeats on
// quiet intervals and teardown when the daemon stops answering pings. tick
// overrides the monitor cadence in tests; nil means a real time.Ticker at
// monitorInterval.
func newParsedLogStream(ctx context.Context, logs io.ReadCloser, options models.LogOptions, ping func(context.Context) error, tick <-chan time.Time) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()

	var searchRegex *regexp.Regexp
	if options.Search != "" {
		searchRegex, _ = regexp.Compile(options.Search) // already validated by handler
	}

	encoder := json.NewEncoder(pipeWriter)
	var mu sync.Mutex
	var wroteEntry atomic.Bool

	stdout := &streamingLogWriter{
		stream:      "stdout",
		encoder:     encoder,
		encoderMu:   &mu,
		pipeWriter:  pipeWriter,
		levelFilter: options.Level,
		searchRegex: searchRegex,
		wroteEntry:  &wroteEntry,
	}
	stderr := &streamingLogWriter{
		stream:      "stderr",
		encoder:     encoder,
		encoderMu:   &mu,
		pipeWriter:  pipeWriter,
		levelFilter: options.Level,
		searchRegex: searchRegex,
		wroteEntry:  &wroteEntry,
	}

	done := make(chan struct{})

	if options.Follow {
		go func() {
			if tick == nil {
				ticker := time.NewTicker(monitorInterval)
				defer ticker.Stop()
				tick = ticker.C
			}

			pingFailures := 0
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-tick:
				}

				if !wroteEntry.Swap(false) {
					mu.Lock()
					err := encoder.Encode(map[string]string{"type": "heartbeat"})
					mu.Unlock()
					if err != nil {
						return
					}
				}

				pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
				err := ping(pingCtx)
				cancel()
				if err == nil {
					pingFailures = 0
					continue
				}
				pingFailures++
				if pingFailures >= maxPingFailures {
					pipeWriter.CloseWithError(fmt.Errorf("docker daemon unreachable while streaming logs: %w", err))
					logs.Close() // unblock StdCopy, which is stuck reading the dead socket
					return
				}
			}
		}()
	}

	go func() {
		defer close(done)
		defer logs.Close()
		defer pipeWriter.Close()

		_, err := stdcopy.StdCopy(stdout, stderr, logs)
		stdout.Flush()
		stderr.Flush()

		if err != nil && err != io.EOF {
			pipeWriter.CloseWithError(err)
		}
	}()

	return pipeReader
}
