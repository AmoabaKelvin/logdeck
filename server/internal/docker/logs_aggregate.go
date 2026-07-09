package docker

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// LogTarget identifies one container in an aggregated log request.
type LogTarget struct {
	Host string
	ID   string
	Name string
}

// GetAggregatedLogsParsed fetches parsed logs for every target concurrently,
// tags each entry with its container, and merges them by timestamp. A failing
// target is skipped; an error is returned only when every target failed.
func (c *MultiHostClient) GetAggregatedLogsParsed(ctx context.Context, targets []LogTarget, options models.LogOptions) ([]models.LogEntry, error) {
	histOptions := options
	histOptions.Follow = false

	batches := make([][]models.LogEntry, len(targets))
	errs := make([]error, len(targets))

	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target LogTarget) {
			defer wg.Done()
			entries, err := c.GetContainerLogsParsed(ctx, target.Host, target.ID, histOptions)
			if err != nil {
				errs[i] = err
				return
			}
			tagLogEntries(entries, target)
			batches[i] = entries
		}(i, target)
	}
	wg.Wait()

	failed := 0
	var firstErr error
	for _, err := range errs {
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if failed == len(targets) {
		return nil, firstErr
	}

	return mergeLogEntriesByTimestamp(batches), nil
}

// StreamAggregatedLogsParsed multiplexes the targets' parsed logs onto one
// NDJSON pipe. The historical phase is fetched up front and emitted in
// timestamp order; when following, each target's live stream (tail=0, since
// history was already emitted) is then fanned into the pipe as entries
// arrive. One target failing does not kill the aggregate; the pipe closes
// once every live stream has ended or ctx is done. Per-stream heartbeats are
// passed through, so the client's liveness watchdog keeps working.
func (c *MultiHostClient) StreamAggregatedLogsParsed(ctx context.Context, targets []LogTarget, options models.LogOptions) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()
	encoder := json.NewEncoder(pipeWriter)
	var mu sync.Mutex

	go func() {
		defer pipeWriter.Close()

		historical, err := c.GetAggregatedLogsParsed(ctx, targets, options)
		if err != nil {
			pipeWriter.CloseWithError(err)
			return
		}
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
		for _, target := range targets {
			stream, err := c.StreamContainerLogsParsed(ctx, target.Host, target.ID, liveOptions)
			if err != nil {
				continue // a missing or failed target must not kill the aggregate
			}
			wg.Add(1)
			go func(target LogTarget, stream io.ReadCloser) {
				defer wg.Done()
				defer stream.Close()
				copyTaggedEntries(encoder, &mu, stream, target)
			}(target, stream)
		}
		wg.Wait()
	}()

	return pipeReader, nil
}

// copyTaggedEntries re-encodes one container's NDJSON log stream onto the
// shared aggregate encoder, tagging each entry with its container. Heartbeats
// pass through untouched. Any error ends this target's stream only.
func copyTaggedEntries(encoder *json.Encoder, mu *sync.Mutex, stream io.Reader, target LogTarget) {
	decoder := json.NewDecoder(stream)
	for {
		var line struct {
			models.LogEntry
			Type string `json:"type"`
		}
		if err := decoder.Decode(&line); err != nil {
			return
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
			return
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
