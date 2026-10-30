package docker

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/redact"
)

// callbackLogWriter parses raw log bytes into lines and hands each parsed
// entry to a callback. Same line handling as logWriter (newline splitting,
// CR trimming, oversized-line flush), but emits instead of accumulating.
type callbackLogWriter struct {
	stream   string
	buffer   []byte
	emit     func(models.LogEntry)
	redactor *redact.Stream
}

func (w *callbackLogWriter) Write(p []byte) (n int, err error) {
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
			w.emit(models.ParseLogLine(w.redactor.Line(line), w.stream))
		}
	}

	if len(w.buffer) > maxLineBufferSize {
		w.Flush()
	}

	return len(p), nil
}

func (w *callbackLogWriter) Flush() {
	if len(w.buffer) == 0 {
		return
	}
	line := strings.TrimSuffix(string(w.buffer), "\r")
	if line != "" {
		w.emit(models.ParseLogLine(w.redactor.Line(line), w.stream))
	}
	w.buffer = nil
}

// TailContainerLogs streams one container's logs and hands each parsed entry
// to emit. It honors opts (Since, Tail, Follow, ShowStdout/ShowStderr,
// Timestamps) and returns once ctx is cancelled or the log stream ends. emit
// is called sequentially from a single goroutine, and no calls happen after
// TailContainerLogs returns.
func (c *MultiHostClient) TailContainerLogs(ctx context.Context, host, containerID string, opts models.LogOptions, emit func(models.LogEntry)) error {
	return c.FollowContainerLogs(ctx, host, containerID, opts, nil, emit)
}

// FollowContainerLogs is TailContainerLogs with an attached hook, called once
// the engine has accepted the log request and before any entry is emitted. It
// is what tells a quiet container apart from one whose logs cannot be read.
func (c *MultiHostClient) FollowContainerLogs(ctx context.Context, host, containerID string, opts models.LogOptions, attached func(), emit func(models.LogEntry)) error {
	apiClient, err := c.GetClient(host)
	if err != nil {
		return err
	}

	tty, redactor, err := inspectLogSource(ctx, apiClient, host, containerID, opts)
	if err != nil {
		return err
	}

	logs, err := apiClient.ContainerLogs(ctx, containerID, buildLogsOptions(opts, opts.Follow, opts.Timestamps))
	if err != nil {
		return err
	}
	if attached != nil {
		attached()
	}

	return tailLogStream(ctx, logs, tty, redactor, emit)
}

// UnreadableLogs reports whether the engine cannot read a container's logs at
// all: a logging driver without a read API (awslogs, syslog, none, ...).
// Retrying such a container never helps.
func UnreadableLogs(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not support reading") || // Docker
		strings.Contains(msg, "cannot read logs") // Podman
}

// tailLogStream parses the raw Docker log stream and emits entries until the
// stream ends or ctx is cancelled. On cancellation it closes logs to unblock
// the reader and waits for it to exit, so no goroutine outlives the call.
func tailLogStream(ctx context.Context, logs io.ReadCloser, tty bool, redactor *redact.Stream, emit func(models.LogEntry)) error {
	done := make(chan error, 1)
	go func() {
		stdout := &callbackLogWriter{stream: "stdout", emit: emit, redactor: redactor}
		stderr := &callbackLogWriter{stream: "stderr", emit: emit, redactor: redactor}
		err := copyLogStream(stdout, stderr, logs, tty)
		stdout.Flush()
		stderr.Flush()
		done <- err
	}()

	select {
	case <-ctx.Done():
		logs.Close() // unblock the reader goroutine
		// The reader's error only reports that close.
		<-done //nolint:errcheck
		return ctx.Err()
	case err := <-done:
		logs.Close()
		if err != nil && err != io.EOF {
			return err
		}
		return nil
	}
}
