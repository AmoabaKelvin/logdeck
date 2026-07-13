package docker

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/pkg/stdcopy"
)

// callbackLogWriter parses raw log bytes into lines and hands each parsed
// entry to a callback. Same line handling as logWriter (newline splitting,
// CR trimming, oversized-line flush), but emits instead of accumulating.
type callbackLogWriter struct {
	stream string
	buffer []byte
	emit   func(models.LogEntry)
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
			w.emit(models.ParseLogLine(line, w.stream))
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
		w.emit(models.ParseLogLine(line, w.stream))
	}
	w.buffer = nil
}

// TailContainerLogs streams one container's logs and hands each parsed entry
// to emit. It honors opts (Since, Tail, Follow, ShowStdout/ShowStderr,
// Timestamps) and returns once ctx is cancelled or the log stream ends. emit
// is called sequentially from a single goroutine, and no calls happen after
// TailContainerLogs returns.
func (c *MultiHostClient) TailContainerLogs(ctx context.Context, host, containerID string, opts models.LogOptions, emit func(models.LogEntry)) error {
	apiClient, err := c.GetClient(host)
	if err != nil {
		return err
	}

	// stdcopy demux garbles raw TTY streams: a TTY container exposes a single
	// unframed stream, so detect TTY up front and read it directly.
	inspect, err := apiClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}
	tty := inspect.Config != nil && inspect.Config.Tty

	logs, err := apiClient.ContainerLogs(ctx, containerID, buildLogsOptions(opts, opts.Follow, opts.Timestamps))
	if err != nil {
		return err
	}

	return tailLogStream(ctx, logs, tty, emit)
}

// tailLogStream parses the raw Docker log stream and emits entries until the
// stream ends or ctx is cancelled. On cancellation it closes logs to unblock
// the reader and waits for it to exit, so no goroutine outlives the call.
func tailLogStream(ctx context.Context, logs io.ReadCloser, tty bool, emit func(models.LogEntry)) error {
	done := make(chan error, 1)
	go func() {
		var err error
		if tty {
			// TTY streams carry no stdcopy framing; everything is stdout.
			w := &callbackLogWriter{stream: "stdout", emit: emit}
			_, err = io.Copy(w, logs)
			w.Flush()
		} else {
			stdout := &callbackLogWriter{stream: "stdout", emit: emit}
			stderr := &callbackLogWriter{stream: "stderr", emit: emit}
			_, err = stdcopy.StdCopy(stdout, stderr, logs)
			stdout.Flush()
			stderr.Flush()
		}
		done <- err
	}()

	select {
	case <-ctx.Done():
		logs.Close() // unblock the reader goroutine
		<-done
		return ctx.Err()
	case err := <-done:
		logs.Close()
		if err != nil && err != io.EOF {
			return err
		}
		return nil
	}
}
