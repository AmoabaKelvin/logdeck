package alerts

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	historyCap          = 500
	historyFlushEvery   = 2 * time.Second
	defaultHistoryLimit = 100
)

// history is a bounded in-memory store of fired alerts, newest first,
// mirrored to a JSON file next to the config file. Appends mark the store
// dirty; flushLoop persists at most every historyFlushEvery via an atomic
// tmp+rename write and flushes synchronously once more on shutdown.
type history struct {
	path string
	max  int

	mu      sync.Mutex
	entries []models.Alert // newest first
	dirty   bool
}

func newHistory(path string, max int) *history {
	return &history{path: path, max: max}
}

// load reads the persisted history from disk. A missing file yields an empty
// history; a corrupt file logs a warning and starts empty. Never fatal.
func (h *history) load() {
	data, err := os.ReadFile(h.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("alerts: failed to read history file %s: %v", h.path, err)
		}
		return
	}
	var entries []models.Alert
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("alerts: history file %s is corrupt, starting empty: %v", h.path, err)
		return
	}
	if len(entries) > h.max {
		entries = entries[:h.max]
	}
	h.mu.Lock()
	h.entries = entries
	h.mu.Unlock()
}

// append inserts an alert at the front (newest first), evicting the oldest
// entry beyond the cap, and marks the store dirty.
func (h *history) append(a models.Alert) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, models.Alert{})
	copy(h.entries[1:], h.entries)
	h.entries[0] = a
	if len(h.entries) > h.max {
		h.entries = h.entries[:h.max]
	}
	h.dirty = true
}

// list returns up to limit entries, newest first. limit <= 0 falls back to
// defaultHistoryLimit. The result is always a non-nil copy.
func (h *history) list(limit int) []models.Alert {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if limit > len(h.entries) {
		limit = len(h.entries)
	}
	out := make([]models.Alert, limit)
	copy(out, h.entries[:limit])
	return out
}

// clear empties the history and persists the empty state immediately.
func (h *history) clear() {
	h.mu.Lock()
	h.entries = nil
	h.dirty = true
	h.mu.Unlock()
	h.flushIfDirty()
}

// flushIfDirty writes the current entries to disk when there are unpersisted
// changes. On write failure the store stays dirty so the next flush retries.
func (h *history) flushIfDirty() {
	h.mu.Lock()
	if !h.dirty {
		h.mu.Unlock()
		return
	}
	snapshot := make([]models.Alert, len(h.entries))
	copy(snapshot, h.entries)
	h.dirty = false
	h.mu.Unlock()

	if err := writeHistoryFile(h.path, snapshot); err != nil {
		log.Printf("alerts: failed to persist alert history: %v", err)
		h.mu.Lock()
		h.dirty = true
		h.mu.Unlock()
	}
}

// flushLoop persists dirty state at most every historyFlushEvery until stop
// closes, then performs a final synchronous flush.
func (h *history) flushLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(historyFlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.flushIfDirty()
		case <-stop:
			h.flushIfDirty()
			return
		}
	}
}

// writeHistoryFile writes entries as a JSON array (never null) atomically via
// a temp file and rename.
func writeHistoryFile(path string, entries []models.Alert) error {
	if entries == nil {
		entries = []models.Alert{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal alert history: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to write temp history file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to rename history file: %w", err)
	}
	return nil
}
