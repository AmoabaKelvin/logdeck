package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestTailLogStreamParsesMultiplexedFrames(t *testing.T) {
	var stream bytes.Buffer
	stdout := stdcopy.NewStdWriter(&stream, stdcopy.Stdout)
	stderr := stdcopy.NewStdWriter(&stream, stdcopy.Stderr)
	if _, err := stdout.Write([]byte("INFO: request received\n")); err != nil {
		t.Fatalf("failed to write stdout frame: %v", err)
	}
	if _, err := stderr.Write([]byte("ERROR: boom\n")); err != nil {
		t.Fatalf("failed to write stderr frame: %v", err)
	}

	var entries []models.LogEntry
	err := tailLogStream(context.Background(), io.NopCloser(&stream), false, func(e models.LogEntry) {
		entries = append(entries, e)
	})
	if err != nil {
		t.Fatalf("tailLogStream failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Stream != "stdout" || entries[0].Level != models.LogLevelInfo {
		t.Fatalf("expected stdout INFO entry, got stream=%s level=%s", entries[0].Stream, entries[0].Level)
	}
	if entries[1].Stream != "stderr" || entries[1].Level != models.LogLevelError {
		t.Fatalf("expected stderr ERROR entry, got stream=%s level=%s", entries[1].Stream, entries[1].Level)
	}
}

func TestTailLogStreamTTYReadsRawStream(t *testing.T) {
	// Raw unframed bytes, as a TTY container produces them. Feeding these to
	// stdcopy would fail on the bogus header; the TTY path must read directly.
	raw := "INFO: request received\nERROR: boom\n"

	var entries []models.LogEntry
	err := tailLogStream(context.Background(), io.NopCloser(strings.NewReader(raw)), true, func(e models.LogEntry) {
		entries = append(entries, e)
	})
	if err != nil {
		t.Fatalf("tailLogStream failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for i, entry := range entries {
		if entry.Stream != "stdout" {
			t.Fatalf("expected TTY entry %d on stdout, got %s", i, entry.Stream)
		}
	}
	if entries[0].Level != models.LogLevelInfo || entries[1].Level != models.LogLevelError {
		t.Fatalf("expected INFO then ERROR, got %s then %s", entries[0].Level, entries[1].Level)
	}
}

func TestTailLogStreamStopsOnContextCancel(t *testing.T) {
	// A stream that never delivers data and never closes on its own.
	rawReader, _ := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	results := make(chan error, 1)
	go func() {
		results <- tailLogStream(ctx, rawReader, false, func(models.LogEntry) {})
	}()

	cancel()

	select {
	case err := <-results:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tailLogStream did not terminate after context cancellation")
	}
}

// roundTripFunc fakes the Docker daemon behind the SDK's HTTP client.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestTailContainerLogsPassesOptionsToAPI(t *testing.T) {
	var stream bytes.Buffer
	stdout := stdcopy.NewStdWriter(&stream, stdcopy.Stdout)
	if _, err := stdout.Write([]byte("hello world\n")); err != nil {
		t.Fatalf("failed to write stdout frame: %v", err)
	}

	var logsQuery url.Values
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/containers/abc/json"):
			return jsonResponse(req, `{"Id":"abc","Config":{"Tty":false}}`), nil
		case strings.HasSuffix(req.URL.Path, "/containers/abc/logs"):
			logsQuery = req.URL.Query()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:       io.NopCloser(bytes.NewReader(stream.Bytes())),
				Request:    req,
			}, nil
		default:
			t.Errorf("unexpected request path %s", req.URL.Path)
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
	})

	apiClient, err := client.NewClientWithOpts(
		client.WithHost("tcp://127.0.0.1:2375"),
		client.WithVersion("1.44"),
		client.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("failed to create fake client: %v", err)
	}
	c := &MultiHostClient{clients: map[string]*client.Client{"local": apiClient}}

	opts := models.LogOptions{
		Since:      "1700000000",
		Tail:       "50",
		Timestamps: true,
		ShowStdout: true,
		ShowStderr: true,
	}

	var entries []models.LogEntry
	if err := c.TailContainerLogs(context.Background(), "local", "abc", opts, func(e models.LogEntry) {
		entries = append(entries, e)
	}); err != nil {
		t.Fatalf("TailContainerLogs failed: %v", err)
	}

	if logsQuery == nil {
		t.Fatal("expected a logs API call")
	}
	if got := logsQuery.Get("since"); !strings.HasPrefix(got, "1700000000") {
		t.Fatalf("expected since to be passed through, got %q", got)
	}
	if got := logsQuery.Get("tail"); got != "50" {
		t.Fatalf("expected tail=50, got %q", got)
	}
	if got := logsQuery.Get("timestamps"); got != "1" {
		t.Fatalf("expected timestamps=1, got %q", got)
	}
	if got := logsQuery.Get("stdout"); got != "1" {
		t.Fatalf("expected stdout=1, got %q", got)
	}
	if got := logsQuery.Get("stderr"); got != "1" {
		t.Fatalf("expected stderr=1, got %q", got)
	}
	if got := logsQuery.Get("follow"); got != "" && got != "0" {
		t.Fatalf("expected follow to be unset, got %q", got)
	}

	if len(entries) != 1 || entries[0].Message != "hello world" {
		t.Fatalf("expected the parsed entry to be emitted, got %+v", entries)
	}
}

func TestTailContainerLogsUsesTTYPathFromInspect(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/containers/abc/json"):
			return jsonResponse(req, `{"Id":"abc","Config":{"Tty":true}}`), nil
		case strings.HasSuffix(req.URL.Path, "/containers/abc/logs"):
			// Raw TTY bytes; stdcopy would choke on this.
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:       io.NopCloser(strings.NewReader("WARN: tty line\n")),
				Request:    req,
			}, nil
		default:
			t.Errorf("unexpected request path %s", req.URL.Path)
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
	})

	apiClient, err := client.NewClientWithOpts(
		client.WithHost("tcp://127.0.0.1:2375"),
		client.WithVersion("1.44"),
		client.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("failed to create fake client: %v", err)
	}
	c := &MultiHostClient{clients: map[string]*client.Client{"local": apiClient}}

	var entries []models.LogEntry
	opts := models.LogOptions{ShowStdout: true, ShowStderr: true}
	if err := c.TailContainerLogs(context.Background(), "local", "abc", opts, func(e models.LogEntry) {
		entries = append(entries, e)
	}); err != nil {
		t.Fatalf("TailContainerLogs failed on TTY container: %v", err)
	}

	if len(entries) != 1 || entries[0].Stream != "stdout" || entries[0].Level != models.LogLevelWarn {
		t.Fatalf("expected one stdout WARN entry from the TTY stream, got %+v", entries)
	}
}
