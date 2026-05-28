package docker

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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

// logWriter implements io.Writer and parses log lines
type logWriter struct {
	stream  string
	entries *[]models.LogEntry
	buffer  []byte
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	for {
		idx := -1
		for i, b := range w.buffer {
			if b == '\n' {
				idx = i
				break
			}
		}

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
}

func (w *streamingLogWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	for {
		idx := -1
		for i, b := range w.buffer {
			if b == '\n' {
				idx = i
				break
			}
		}

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

// multi host client methods
func (c *MultiHostClient) GetContainerLogsParsed(hostName, id string, options models.LogOptions) ([]models.LogEntry, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	logs, err := apiClient.ContainerLogs(context.Background(), id, buildLogsOptions(options, false, true))
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

func (c *MultiHostClient) StreamContainerLogsParsed(hostName, id string, options models.LogOptions) (io.ReadCloser, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	logs, err := apiClient.ContainerLogs(context.Background(), id, buildLogsOptions(options, options.Follow, true))
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()

	var searchRegex *regexp.Regexp
	if options.Search != "" {
		searchRegex, _ = regexp.Compile(options.Search) // already validated by handler
	}

	go func() {
		defer logs.Close()
		defer pipeWriter.Close()

		encoder := json.NewEncoder(pipeWriter)
		var mu sync.Mutex

		stdout := &streamingLogWriter{
			stream:      "stdout",
			encoder:     encoder,
			encoderMu:   &mu,
			pipeWriter:  pipeWriter,
			levelFilter: options.Level,
			searchRegex: searchRegex,
		}
		stderr := &streamingLogWriter{
			stream:      "stderr",
			encoder:     encoder,
			encoderMu:   &mu,
			pipeWriter:  pipeWriter,
			levelFilter: options.Level,
			searchRegex: searchRegex,
		}

		_, err = stdcopy.StdCopy(stdout, stderr, logs)
		stdout.Flush()
		stderr.Flush()

		if err != nil && err != io.EOF {
			pipeWriter.CloseWithError(err)
		}
	}()

	return pipeReader, nil
}
