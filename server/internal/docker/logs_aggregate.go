package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/client"
)

// LogTarget identifies one container in an aggregated log request.
type LogTarget struct {
	Host string
	ID   string
	Name string
}

// SourceStatus says how one target fared in an aggregated log read.
type SourceStatus string

const (
	SourceOK        SourceStatus = "ok"         // read, with lines to show
	SourceNoMatches SourceStatus = "no_matches" // read, nothing matched
	SourceStopped   SourceStatus = "stopped"    // its live stream ended with the container
	SourceRemoved   SourceStatus = "removed"    // the container no longer exists
	SourceFailed    SourceStatus = "failed"     // its logs could not be read
)

// SourceCoverage reports one target of an aggregated read, so a result
// missing a source can never pass for a complete one.
type SourceCoverage struct {
	Host          string       `json:"host"`
	ContainerID   string       `json:"containerId"`
	ContainerName string       `json:"containerName,omitempty"`
	Status        SourceStatus `json:"status"`
	Error         string       `json:"error,omitempty"`
}

// sourceEvent is the NDJSON line an aggregated stream writes when a source
// opens, fails or ends, framed like the heartbeat line.
type sourceEvent struct {
	Type string `json:"type"`
	SourceCoverage
}

func newSourceCoverage(target LogTarget, status SourceStatus, err error) SourceCoverage {
	source := SourceCoverage{Host: target.Host, ContainerID: target.ID, ContainerName: target.Name, Status: status}
	if err != nil {
		source.Error = err.Error()
	}
	return source
}

// readFailure classifies a target whose logs could not be read. A container
// that no longer exists is expected, not a failure.
func readFailure(target LogTarget, err error) SourceCoverage {
	if client.IsErrNotFound(err) {
		return newSourceCoverage(target, SourceRemoved, nil)
	}
	return newSourceCoverage(target, SourceFailed, err)
}

// Incomplete reports whether any source's logs could not be read.
func Incomplete(sources []SourceCoverage) bool {
	for _, source := range sources {
		if source.Status == SourceFailed {
			return true
		}
	}
	return false
}

// NoReadableSourceError is returned when targets failed and none could be
// read, so that case is never answered with an empty result.
type NoReadableSourceError struct {
	Sources []SourceCoverage
}

func (e *NoReadableSourceError) Error() string {
	var failures []string
	for _, source := range e.Sources {
		if source.Status != SourceFailed {
			continue
		}
		name := source.ContainerName
		if name == "" {
			name = source.ContainerID
		}
		failures = append(failures, fmt.Sprintf("%s on %s: %s", name, source.Host, source.Error))
	}
	return "no source could be read: " + strings.Join(failures, "; ")
}

func noReadableSource(sources []SourceCoverage) error {
	for _, source := range sources {
		if source.Status == SourceOK || source.Status == SourceNoMatches {
			return nil
		}
	}
	if !Incomplete(sources) {
		return nil // every target is gone, which is expected
	}
	return &NoReadableSourceError{Sources: sources}
}

// GetAggregatedLogsParsed fetches parsed logs for every target concurrently,
// tags each entry with its container, and merges them by timestamp. The
// returned sources report each target's outcome, in target order. A failing
// target is skipped; an error is returned only when targets failed and none
// could be read.
func (c *MultiHostClient) GetAggregatedLogsParsed(ctx context.Context, targets []LogTarget, options models.LogOptions) ([]models.LogEntry, []SourceCoverage, error) {
	histOptions := options
	histOptions.Follow = false

	batches := make([][]models.LogEntry, len(targets))
	sources := make([]SourceCoverage, len(targets))

	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target LogTarget) {
			defer wg.Done()
			entries, err := c.GetContainerLogsParsed(ctx, target.Host, target.ID, histOptions)
			if err != nil {
				sources[i] = readFailure(target, err)
				return
			}
			tagLogEntries(entries, target)
			batches[i] = entries
			status := SourceOK
			if len(entries) == 0 {
				status = SourceNoMatches
			}
			sources[i] = newSourceCoverage(target, status, nil)
		}(i, target)
	}
	wg.Wait()

	if err := noReadableSource(sources); err != nil {
		return nil, sources, err
	}
	return mergeLogEntriesByTimestamp(batches), sources, nil
}

// StreamAggregatedLogsParsed multiplexes the targets' parsed logs onto one
// NDJSON pipe. The historical phase is fetched before the pipe is returned,
// so a request where no source can be read fails instead of streaming
// nothing, and is emitted in timestamp order; when following, each target's
// live stream (tail=0, since history was already emitted) is then fanned
// into the pipe as entries arrive. One target failing does not kill the
// aggregate: a source line reports each target as ok, removed or failed when
// its live stream opens, and as stopped or failed when it ends. The pipe
// closes once every live stream has ended or ctx is done. Per-stream
// heartbeats are passed through, so the client's liveness watchdog keeps
// working.
func (c *MultiHostClient) StreamAggregatedLogsParsed(ctx context.Context, targets []LogTarget, options models.LogOptions) (io.ReadCloser, error) {
	historical, sources, err := c.GetAggregatedLogsParsed(ctx, targets, options)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	encoder := json.NewEncoder(pipeWriter)
	var mu sync.Mutex
	report := func(source SourceCoverage) {
		mu.Lock()
		_ = encoder.Encode(sourceEvent{Type: "source", SourceCoverage: source})
		mu.Unlock()
	}

	go func() {
		defer pipeWriter.Close()

		for _, entry := range historical {
			if err := encoder.Encode(entry); err != nil {
				return
			}
		}

		if !options.Follow {
			return
		}

		liveOptions := options
		liveOptions.Tail = "0" // historical lines were already emitted

		var wg sync.WaitGroup
		for i, target := range targets {
			if status := sources[i].Status; status == SourceFailed || status == SourceRemoved {
				// Its history could not be read; the live stream would fail the
				// same way.
				report(sources[i])
				continue
			}
			stream, err := c.StreamContainerLogsParsed(ctx, target.Host, target.ID, liveOptions)
			if err != nil {
				report(readFailure(target, err))
				continue
			}
			report(newSourceCoverage(target, SourceOK, nil))
			wg.Add(1)
			go func(target LogTarget, stream io.ReadCloser) {
				defer wg.Done()
				defer stream.Close()
				err := copyTaggedEntries(encoder, &mu, stream, target)
				switch {
				case ctx.Err() != nil:
					// The client is gone; there is nobody left to tell.
				case errors.Is(err, io.EOF):
					report(newSourceCoverage(target, SourceStopped, nil))
				default:
					report(newSourceCoverage(target, SourceFailed, err))
				}
			}(target, stream)
		}
		wg.Wait()
	}()

	return pipeReader, nil
}

// copyTaggedEntries re-encodes one container's NDJSON log stream onto the
// shared aggregate encoder, tagging each entry with its container. Heartbeats
// pass through untouched. It returns the error that ended this target's
// stream: io.EOF when the container's stream ended cleanly.
func copyTaggedEntries(encoder *json.Encoder, mu *sync.Mutex, stream io.Reader, target LogTarget) error {
	decoder := json.NewDecoder(stream)
	for {
		var line struct {
			models.LogEntry
			Type string `json:"type"`
		}
		if err := decoder.Decode(&line); err != nil {
			return err
		}

		var payload any
		if line.Type == "heartbeat" {
			payload = map[string]string{"type": "heartbeat"}
		} else {
			entry := line.LogEntry
			entry.ContainerID = target.ID
			entry.ContainerName = target.Name
			payload = entry
		}

		mu.Lock()
		err := encoder.Encode(payload)
		mu.Unlock()
		if err != nil {
			return err
		}
	}
}

func tagLogEntries(entries []models.LogEntry, target LogTarget) {
	for i := range entries {
		entries[i].ContainerID = target.ID
		entries[i].ContainerName = target.Name
	}
}

// mergeLogEntriesByTimestamp flattens per-container batches into one slice
// ordered by timestamp. Each batch is already in log order, so a stable sort
// keeps same-timestamp lines in their original relative order.
func mergeLogEntriesByTimestamp(batches [][]models.LogEntry) []models.LogEntry {
	total := 0
	for _, batch := range batches {
		total += len(batch)
	}

	merged := make([]models.LogEntry, 0, total)
	for _, batch := range batches {
		merged = append(merged, batch...)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})
	return merged
}
