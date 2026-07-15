package logstore

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// TestRetentionHoldsCapUnderConcurrentIngestion is the BUG 1 guard: with
// retention on and a firehose of live lines, the database file must stay bounded
// near its cap.
//
// It reproduces the original defect, in which the janitor ran retention in its
// own write transaction that competed with the writeLoop for SQLite's single
// write lock. Under sustained ingestion the writer held the lock nearly
// continuously, the eviction transaction lost it (SQLITE_BUSY), no lines were
// ever evicted, and the file grew without bound. The fix routes every write —
// ingestion and eviction — through the one writer goroutine, so they can never
// contend. Against the pre-fix code this test fails: nothing is evicted and the
// file blows far past the assertion.
func TestRetentionHoldsCapUnderConcurrentIngestion(t *testing.T) {
	limits := func() config.ResolvedLogStoreConfig {
		return config.ResolvedLogStoreConfig{Enabled: true, PerContainerMB: 1, TotalMB: 1}
	}
	store, err := Open(filepath.Join(t.TempDir(), "logs.db"), limits)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Capture the store's own log so the test can assert retention never failed.
	var logBuf bytes.Buffer
	prevOut := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(prevOut) })

	ctx, cancel := context.WithCancel(context.Background())
	hub := &fakeHub{}
	store.start(ctx, hub, func() Engine { return newFakeEngine() })

	// Flood eight containers with unique lines until the writer has committed far
	// more than the ~1 MB cap can hold (~4.5k lines at this size), so retention is
	// forced to evict no matter how fast this worker is. Driving to a commit
	// target rather than a fixed wall-clock duration is what keeps the eviction
	// assertion below meaningful on a slow CI machine.
	const (
		containers      = 8
		lineBytes       = 200
		targetCommitted = 12_000
	)
	var (
		seq  atomic.Uint64
		wg   sync.WaitGroup
		stop = make(chan struct{})
	)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					n := seq.Add(1)
					hub.emit(floodRecord(n%containers, n, lineBytes))
				}
			}
		}()
	}
	deadline := time.Now().Add(90 * time.Second)
	for store.Committed() < targetCommitted && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	close(stop)
	wg.Wait()

	// Let the writer drain the queue and run a final sweep.
	cancel()
	store.Wait()

	size, err := store.DBSize()
	if err != nil {
		t.Fatalf("DBSize: %v", err)
	}
	// The store held ~1 MB of live data; the file (plus a small ramp overshoot
	// and its WAL) must stay well under this. Without the fix it reaches tens of
	// megabytes.
	const maxSize = 8 * bytesPerMB
	if size > maxSize {
		t.Fatalf("database grew to %d bytes under sustained ingestion; retention did not hold the cap (want <= %d)",
			size, maxSize)
	}

	// Retention must actually have evicted: committing well past the cap is only
	// possible if the janitor swept. This is the real regression guard — on the
	// pre-fix code the sweep loses the write lock (SQLITE_BUSY) and evicts
	// nothing, rather than the test passing because a slow worker never filled up.
	if store.evictions.Load() == 0 {
		t.Fatal("retention never evicted; the cap-holding path was not exercised")
	}
	retained, err := store.CountLines(context.Background())
	if err != nil {
		t.Fatalf("CountLines: %v", err)
	}
	if committed := store.Committed(); committed <= retained {
		t.Fatalf("committed=%d retained=%d: retention did not evict any lines", committed, retained)
	}

	if out := logBuf.String(); strings.Contains(out, "retention sweep failed") ||
		strings.Contains(out, "database is locked") {
		t.Fatalf("retention logged a failure under load:\n%s", out)
	}
}

// floodRecord builds one synthetic live record with a unique (ts, stream, raw)
// so the writer's dedup never collapses distinct lines.
func floodRecord(container, seq uint64, lineBytes int) logstream.Record {
	name := "svc-" + itoa(container)
	ts := time.Now().Add(time.Duration(seq)) // strictly increasing, unique
	raw := ts.UTC().Format(time.RFC3339Nano) + " INFO seq=" + itoa(seq) + " request handled " +
		strings.Repeat("x", lineBytes)
	return logstream.Record{
		Host:          "stress",
		ContainerID:   "gen-" + itoa(container),
		ContainerName: name,
		Labels:        map[string]string{"com.docker.compose.project": "stressstack"},
		Entry: models.LogEntry{
			Timestamp: ts,
			Stream:    "stdout",
			Raw:       raw,
		},
	}
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
