package logstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

type synchronizedLog struct {
	sync.Mutex
	bytes.Buffer
}

func (l *synchronizedLog) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.Buffer.Write(p)
}

// Exercise lifecycle metadata and initial backfills together. The original
// ingestion stress test lists no containers, so it cannot catch these writers
// competing for SQLite's write lock.
func TestThirtyContainerStartup(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Amplify contention without depending on a slow machine: configure every
	// connection in the existing pool with a short SQLite busy timeout.
	var conns []*sql.Conn
	for range maxDBConns {
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, conn)
		if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout(1)"); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns {
		conn.Close()
	}
	if _, err := s.writerDB.ExecContext(ctx, "PRAGMA busy_timeout(1)"); err != nil {
		t.Fatal(err)
	}

	var output synchronizedLog
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	const containers, lines = 30, 2000
	var infos []models.ContainerInfo
	for i := range containers {
		infos = append(infos, containerInfo("local", fmt.Sprint(i), fmt.Sprint(i), baseTime))
	}
	engine := newFakeEngine(infos...)
	engine.tail = func(_, _ string, _ models.LogOptions, emit func(models.LogEntry)) error {
		for i := range lines {
			emit(entryAt(baseTime.Add(time.Duration(i)*time.Millisecond), "stdout", "INFO request handled"))
		}
		return nil
	}
	s.start(ctx, &fakeHub{}, func() Engine { return engine })
	for s.Committed() < containers*lines && ctx.Err() == nil {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	s.Wait()
	if out := output.String(); strings.Contains(out, "database is locked") {
		t.Fatalf("concurrent lifecycle and ingestion writes failed:\n%s", out)
	}
	if got := s.Committed(); got != containers*lines {
		t.Fatalf("committed %d lines, want %d", got, containers*lines)
	}
	var completed int
	if err := s.db.QueryRow("SELECT count(*) FROM containers WHERE initial_backfill_done = 1").Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != containers {
		t.Fatalf("completed %d backfills, want %d", completed, containers)
	}
}

func TestMetadataWaitsForWriterWithoutBlockingReaders(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := genKey{"local", "web"}
	info := containerInfo(key.host, key.id, "web", baseTime)
	writeEntries(t, s, key, "web", entryAt(baseTime, "stdout", "hello"))

	tx, err := s.writerDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	waiting := s.writerDB.Stats().WaitCount
	result := make(chan error, 1)
	go func() {
		_, err := s.upsertMeta(ctx, key, info, time.Now().UnixMilli())
		result <- err
	}()
	waitFor(t, "metadata queued behind writer", func() bool {
		return s.writerDB.Stats().WaitCount > waiting
	})
	page, err := s.Query(ctx, LogQuery{Host: key.host, Container: "web"})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("history query while writer is occupied: entries=%d, err=%v", len(page.Entries), err)
	}

	// A canceled metadata request must leave the queue without acquiring a
	// second SQLite write connection or delaying shutdown.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := s.upsertMeta(canceled, key, info, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled metadata update: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("queued metadata update: %v", err)
	}
}

func BenchmarkIngestThirtyContainers(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "logs.db"), testLimits)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	state := newWriterState()
	batch := make([]ingestMsg, batchLines)
	for i := range batch {
		id := fmt.Sprint(i % 30)
		batch[i] = ingestMsg{key: genKey{"local", id}, name: id,
			line: line{raw: "INFO request handled " + strings.Repeat("x", 130)}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := range b.N {
		for i := range batch {
			batch[i].line.tsNS = baseTime.UnixNano() + int64(n*batchLines+i)
		}
		if err := s.commit(batch, state); err != nil {
			b.Fatal(err)
		}
		if err := s.sealFullBlocks(state); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N*batchLines)/b.Elapsed().Seconds(), "lines/s")
}

// Concurrent tails and backfills interleave generations in the writer queue.
// Replaying sealed history must not decompress a block for every such switch.
func BenchmarkReplaySealedInterleaved(b *testing.B) {
	s, state, batch := sealedReplayFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := s.commit(batch, state); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if got := s.Committed(); got != 30*blockLines {
		b.Fatalf("replay inserted duplicates: committed %d lines", got)
	}
	b.ReportMetric(float64(b.N*batchLines)/b.Elapsed().Seconds(), "lines/s")
}

func TestReplaySealedInterleaved(t *testing.T) {
	s, state, batch := sealedReplayFixture(t)
	allocs := testing.AllocsPerRun(1, func() {
		if err := s.commit(batch, state); err != nil {
			t.Fatal(err)
		}
	})
	// A cached replay allocates ~106k objects; the old one-block cache, 1.5M.
	if allocs > 500_000 {
		t.Fatalf("interleaved replay allocated %.0f objects; blocks are being repeatedly decoded", allocs)
	}
	if got := s.Committed(); got != 30*blockLines {
		t.Fatalf("replay inserted duplicates: committed %d lines", got)
	}
}

func TestDedupCacheResetsAfterPurge(t *testing.T) {
	s := newTestStore(t)
	entries := chatty(blockLines, baseTime, dockerEntry)
	key := genKey{"local", "api"}
	writeAndSeal(t, s, key, "api", entries...)
	state := mustWriterState(t, s)
	batch := []ingestMsg{{key: key, name: "api", line: lineFromEntry(entries[0])}}
	if err := s.commit(batch, state); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteContainer(context.Background(), "local", "api"); err != nil {
		t.Fatal(err)
	}
	// Reuse the deleted block's rowid, but with different content at the same
	// timestamps. The old cached block must not hide a newly arriving line.
	for i := range entries {
		entries[i] = dockerEntry(entries[i].Timestamp, "stdout", "replacement")
	}
	writeAndSeal(t, s, key, "api", entries...)
	before := s.Committed()
	if err := s.commit(batch, state); err != nil {
		t.Fatal(err)
	}
	if got := s.Committed() - before; got != 1 {
		t.Fatalf("inserted %d lines after purge, want 1", got)
	}
}

func TestSealedDedupWithFullCache(t *testing.T) {
	s := newTestStore(t)
	key := genKey{"local", "api"}
	entries := chatty(blockLines, baseTime, dockerEntry)
	writeAndSeal(t, s, key, "api", entries...)
	state := mustWriterState(t, s)
	state.unpackedBytes = maxDedupCacheBytes
	ctx := context.Background()
	tx, err := s.writerDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var ref int64
	if err := tx.QueryRow("SELECT id FROM containers WHERE container_id = 'api'").Scan(&ref); err != nil {
		t.Fatal(err)
	}
	l := lineFromEntry(entries[0])
	if found, err := s.sealedHasLine(ctx, tx, state, ref, l); err != nil || !found {
		t.Fatalf("duplicate with full cache: found=%v, err=%v", found, err)
	}
	l.raw += " different message"
	if found, err := s.sealedHasLine(ctx, tx, state, ref, l); err != nil || found {
		t.Fatalf("distinct line with full cache: found=%v, err=%v", found, err)
	}
	if len(state.unpacked) != 0 || state.unpackedBytes != maxDedupCacheBytes {
		t.Fatal("decoded block exceeded the cache budget")
	}
}

func sealedReplayFixture(t testing.TB) (*Store, *writerState, []ingestMsg) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "logs.db"), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	state := newWriterState()
	const containers = 30
	entries := chatty(blockLines, baseTime, dockerEntry)
	for c := range containers {
		id := fmt.Sprint(c)
		batch := make([]ingestMsg, len(entries))
		for i, entry := range entries {
			batch[i] = ingestMsg{key: genKey{"local", id}, name: id, line: lineFromEntry(entry)}
		}
		if err := s.commit(batch, state); err != nil {
			t.Fatal(err)
		}
		if err := s.sealFullBlocks(state); err != nil {
			t.Fatal(err)
		}
	}
	batch := make([]ingestMsg, batchLines)
	for i := range batch {
		id := fmt.Sprint(i % containers)
		batch[i] = ingestMsg{key: genKey{"local", id}, name: id, line: lineFromEntry(entries[i/containers])}
	}
	return s, state, batch
}
