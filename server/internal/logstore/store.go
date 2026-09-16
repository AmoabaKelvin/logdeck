// Package logstore persists container logs to a local SQLite database so
// history survives container restarts, rebuilds, and logdeck restarts.
//
// The store subscribes to the shared logstream hub for live lines and reads
// the engine directly to backfill whatever it missed while it was not
// listening. Each engine container ID is stored as its own generation row;
// the logical container is (host, name), so a `docker compose up` that
// replaces a container keeps one continuous timeline under the same name.
package logstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"

	_ "modernc.org/sqlite" // pure-Go driver: every release build is CGO_ENABLED=0
)

const (
	// ingestBuffer bounds the queue between the hub sink and the writer.
	ingestBuffer = 8192
	// batchLines and batchInterval bound one write transaction.
	batchLines    = 500
	batchInterval = 250 * time.Millisecond
	// Commit failures back the writer off between these bounds. The cap is
	// also the most a failing store can delay shutdown.
	minCommitBackoff = time.Second
	maxCommitBackoff = 5 * time.Second
	// syncInterval is the container lifecycle poll cadence.
	syncInterval = 15 * time.Second
	// listTimeout bounds one container listing; an unreachable host must not
	// stall the lifecycle loop.
	listTimeout = 15 * time.Second
	// janitorInterval is the retention sweep cadence: the scheduler signals the
	// writer this often so retention still runs when ingestion is idle.
	janitorInterval = time.Minute
	// retainEveryBatches bounds how many committed batches accumulate between
	// writer-driven retention sweeps. Under a firehose the janitorInterval tick
	// is far too coarse to keep the file near its cap, so the writer also sweeps
	// every this many batches; this is what caps the file's high-water mark.
	retainEveryBatches = 5
	// maxDBConns bounds the connection pool. WAL mode serves concurrent readers,
	// so a small warm pool lets queries run in parallel with the writer without
	// churning connections; the exact ceiling is not load-bearing.
	maxDBConns = 8
	// dropLogEvery throttles the "sink fell behind" warning.
	dropLogEvery = 1000
)

// Hub is the slice of *logstream.Hub the store consumes. Tests inject fakes.
type Hub interface {
	Subscribe(spec logstream.ContainerSpec, opts models.LogOptions, sink func(logstream.Record)) func()
}

// RecoverableHub is implemented by the production logstream hub. Keeping it
// separate from Hub preserves simple test and alert subscribers while letting
// the store request engine rereads for upstream buffer loss and lifecycle
// boundaries.
type RecoverableHub interface {
	SubscribeRecoverable(
		spec logstream.ContainerSpec,
		opts models.LogOptions,
		sink func(logstream.Record),
		recover func(logstream.RecoveryHint),
	) func()
}

// Engine is the slice of *docker.MultiHostClient the store needs: container
// listings for lifecycle tracking and a callback tail for backfill.
type Engine interface {
	ListContainersAllHosts(ctx context.Context) (map[string][]models.ContainerInfo, []docker.HostError, error)
	TailContainerLogs(ctx context.Context, host, containerID string, opts models.LogOptions, emit func(models.LogEntry)) error
}

// DockerProvider yields the current Docker client set; reading through it on
// every use keeps the store correct across hot-swapped config.
type DockerProvider interface {
	Docker() *docker.MultiHostClient
}

// Limits supplies the retention caps, re-read on every janitor pass so config
// changes take effect without a restart.
type Limits func() config.ResolvedLogStoreConfig

// Store owns the SQLite database, the ingestion pipeline, and retention.
type Store struct {
	db *sql.DB
	// writerDB has one connection shared by ingestion, lifecycle, retention,
	// and purge. They wait in Go's pool instead of competing for SQLite's
	// write lock. db keeps history queries independent of that queue.
	writerDB *sql.DB
	path     string
	limits   Limits
	// codec packs and unpacks sealed blocks. Its encoder and decoder are safe
	// for concurrent use, so the writer and every reader share one.
	codec *blockCodec

	ingestCh chan ingestMsg
	// retainCh signals the writer to run a retention sweep between batches. It is
	// coalescing (buffered one, non-blocking send): retention runs between ingest
	// batches, and writerDB serializes both with lifecycle and purge writes.
	retainCh chan struct{}
	drops    atomic.Uint64
	// evictions counts retention sweeps that actually freed bytes. The writer
	// checkpoints the WAL only when this advances, so a store that is under its
	// cap (nothing to evict) never pays for a checkpoint on its hot path.
	evictions atomic.Uint64
	// committed counts log lines successfully inserted, cumulatively. Unlike the
	// live row count it never decreases when retention evicts, so measurement can
	// report true ingestion throughput. Maintained only for the stress harness.
	committed atomic.Int64

	// mu guards resume, gaps, and invalidated, all read by goroutines other
	// than the one that writes them.
	mu sync.Mutex
	// resume holds every generation's persisted backfill resume point as it
	// stood before live ingestion started; see snapshotResume.
	resume map[genKey]resumePoint
	// gaps holds, per generation, the earliest timestamp a bounded buffer had
	// to drop. Its version changes on every notice, even when the timestamp is
	// identical, so a loss during an in-flight reread cannot be cleared by that
	// older read completing.
	gaps map[genKey]gapMark
	// refresh is the versioned lifecycle-boundary signal. A zero-timestamp
	// recovery hint rereads from the durable watermark.
	refresh map[genKey]uint64
	// invalidated holds the generations DeleteContainer removed, until the
	// writer has dropped them from its ref cache. See DeleteContainer.
	invalidated map[genKey]struct{}

	// producers counts the goroutines that may still send on ingestCh (the
	// hub sink and in-flight backfills); ingestCh is closed once they stop.
	producers sync.WaitGroup
	// workers counts every goroutine the store started, so Wait can block
	// until the pipeline has fully drained.
	workers sync.WaitGroup

	// backfillState is owned by the sync loop.
	backfillSem chan struct{}
	// reconcileCh coalesces loss and lifecycle notices into an immediate
	// lifecycle pass. The periodic sync remains the safety net, but recoverable
	// gaps should not sit pending until its next tick.
	reconcileCh chan struct{}
}

// DBPath returns the log database path that sits next to the config file
// (/data/logs.db in the shipped compose file, which already mounts a volume).
func DBPath(configFilePath string) string {
	return filepath.Join(filepath.Dir(configFilePath), "logs.db")
}

// OpenFromConfig prepares log persistence from the manager's settings. It
// never aborts startup and never returns an error: a disabled store creates no
// database file at all, and an unusable path (read-only volume, missing mount)
// logs a warning and leaves the caller running without stored logs. A nil
// return means "no stored logs".
func OpenFromConfig(manager *config.Manager) *Store {
	limits := manager.LogStore()
	if !limits.Enabled {
		log.Println("Log persistence is DISABLED")
		return nil
	}

	path := DBPath(manager.ConfigFilePath())
	store, err := Open(path, manager.LogStore)
	if err != nil {
		log.Printf("Warning: log persistence is DISABLED: %v", err)
		return nil
	}

	log.Printf("Log persistence is ENABLED (%s, %d MB per container, %d MB total)",
		path, limits.PerContainerMB, limits.TotalMB)
	return store
}

// Open prepares the SQLite database at path and initializes the schema.
func Open(path string, limits Limits) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create log store directory: %w", err)
	}

	// foreign_keys: log_lines.container_ref must resolve to a real generation.
	// _txlock=immediate: every transaction the store opens is a writer, and a
	// deferred read-then-write transaction (the janitor's eviction) takes the
	// write lock too late and dies with SQLITE_BUSY_SNAPSHOT under concurrent
	// ingestion — exactly when retention is needed most.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"+
			"&_pragma=foreign_keys(1)&_txlock=immediate",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open log store: %w", err)
	}
	// The default pool keeps only two idle connections, so a burst of concurrent
	// queries alongside the writer forces connections to be closed and reopened
	// constantly, and every reopen re-runs the DSN pragmas — pure overhead that
	// shows up as query-latency tails. WAL mode allows many concurrent readers,
	// so hold a small warm pool open instead of churning it.
	db.SetMaxOpenConns(maxDBConns)
	db.SetMaxIdleConns(maxDBConns)
	db.SetConnMaxIdleTime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open log store: %w", err)
	}
	if err := initSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	writerDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open log store writer: %w", err)
	}
	writerDB.SetMaxOpenConns(1)
	writerDB.SetMaxIdleConns(1)
	if err := writerDB.PingContext(ctx); err != nil {
		writerDB.Close()
		db.Close()
		return nil, fmt.Errorf("open log store writer: %w", err)
	}

	codec, err := newBlockCodec()
	if err != nil {
		writerDB.Close()
		db.Close()
		return nil, err
	}

	return &Store{
		db:          db,
		writerDB:    writerDB,
		path:        path,
		limits:      limits,
		codec:       codec,
		ingestCh:    make(chan ingestMsg, ingestBuffer),
		retainCh:    make(chan struct{}, 1),
		resume:      make(map[genKey]resumePoint),
		gaps:        make(map[genKey]gapMark),
		refresh:     make(map[genKey]uint64),
		invalidated: make(map[genKey]struct{}),
		backfillSem: make(chan struct{}, maxConcurrentBackfills),
		reconcileCh: make(chan struct{}, 1),
	}, nil
}

// Start runs the ingestion, lifecycle, and retention loops until ctx is
// cancelled. Use Wait to block until everything has drained.
func (s *Store) Start(ctx context.Context, hub Hub, provider DockerProvider) {
	s.start(ctx, hub, func() Engine { return provider.Docker() })
}

// start is the injectable form of Start; tests supply fakes.
func (s *Store) start(ctx context.Context, hub Hub, source func() Engine) {
	// Before a single live line can advance a watermark: the first backfill of
	// each generation must resume from where the *previous* process stopped, not
	// from wherever live ingestion has already reached.
	if err := s.snapshotResume(ctx); err != nil {
		log.Printf("logstore: reading stored resume points failed, gap healing may be incomplete: %v", err)
	}

	// The hub sink is a producer: it must stop before ingestCh is closed.
	s.producers.Add(1)
	spec := logstream.ContainerSpec{} // all containers on all hosts
	opts := models.LogOptions{
		// Tail one retained record when the followed response attaches. The hub
		// converts that first record into a recovery hint, which closes the
		// finite container-start-to-stream-attachment window. The store's exact
		// insertion key removes the intentional overlap.
		Follow: true, Timestamps: true, Tail: "1",
		ShowStdout: true, ShowStderr: true,
	}
	var unsubscribe func()
	if recoverable, ok := hub.(RecoverableHub); ok {
		unsubscribe = recoverable.SubscribeRecoverable(spec, opts, s.sink, s.recover)
	} else {
		unsubscribe = hub.Subscribe(spec, opts, s.sink)
	}

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.writeLoop()
	}()

	// syncStopped closes once the sync loop has returned, and with it the only
	// code that registers new backfill producers.
	syncStopped := make(chan struct{})
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer close(syncStopped)
		s.syncLoop(ctx, source)
	}()

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.janitorLoop(ctx)
	}()

	// Shutdown: drop the subscription (after which the sink is never called
	// again), wait for the sync loop to stop scheduling backfills, let the
	// in-flight ones finish, then close the queue so the writer flushes what is
	// left and exits.
	//
	// Waiting for the sync loop is what makes the producer count final: a
	// backfill registered after the count reached zero would send on a closed
	// queue.
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		<-ctx.Done()
		unsubscribe()
		<-syncStopped
		s.producers.Done()
		s.producers.Wait()
		close(s.ingestCh)
	}()
}

// sink receives live records on the hub's delivery goroutine. It must never
// block for long, so a full queue drops the record and bumps a counter.
func (s *Store) sink(rec logstream.Record) {
	msg := ingestMsg{
		kind:    msgLine,
		key:     genKey{host: rec.Host, id: rec.ContainerID},
		name:    rec.ContainerName,
		project: composeProject(rec.Labels),
	}
	msg.line = lineFromEntry(rec.Entry)

	select {
	case s.ingestCh <- msg:
	default:
		// A dropped line is not lost: the mark makes the next sync re-read this
		// generation from the dropped line's timestamp, and the insert dedup
		// makes that re-read safe.
		s.markGap(msg.key, msg.line.tsNS)
		if n := s.drops.Add(1); n%dropLogEvery == 0 {
			log.Printf("logstore: dropped %d log lines (writer fell behind); they will be re-read from the engine", n)
		}
	}
}

func (s *Store) recover(hint logstream.RecoveryHint) {
	key := genKey{host: hint.Host, id: hint.ContainerID}
	if hint.Earliest.IsZero() {
		s.markRefresh(key)
		return
	}
	s.markGap(key, hint.Earliest.UnixNano())
}

// requestReconcile wakes the lifecycle loop without ever blocking a live-log
// delivery goroutine. One pending wake is sufficient: a reconciliation reads
// every outstanding generation marker.
func (s *Store) requestReconcile() {
	select {
	case s.reconcileCh <- struct{}{}:
	default:
	}
}

// resumePoint is one generation's persisted per-stream backfill watermarks.
type resumePoint struct {
	stdoutWM    int64
	stderrWM    int64
	initialDone bool
}

// snapshotResume records every generation's persisted resume point. It runs
// once, before live ingestion can advance the watermark columns, so the first
// backfill after a restart re-reads the whole downtime window instead of the
// last few seconds.
func (s *Store) snapshotResume(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT host, container_id, stdout_wm_ns, stderr_wm_ns,
		        initial_backfill_done
		 FROM containers`)
	if err != nil {
		return err
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var (
			key         genKey
			point       resumePoint
			initialDone int
		)
		if err := rows.Scan(
			&key.host,
			&key.id,
			&point.stdoutWM,
			&point.stderrWM,
			&initialDone,
		); err != nil {
			return err
		}
		point.initialDone = initialDone != 0
		s.resume[key] = point
	}
	return rows.Err()
}

// takeResume reports the snapshotted resume point of a generation. The entry
// survives a failed backfill (which must retry from the same point) and is
// dropped by clearResume once the generation has been read.
func (s *Store) takeResume(key genKey) (resumePoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	point, ok := s.resume[key]
	return point, ok
}

// clearResume retires a generation's snapshot after a successful backfill: from
// then on the live watermark in the database is the truth.
func (s *Store) clearResume(key genKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.resume, key)
}

type gapMark struct {
	from    int64
	version uint64
}

// markGap records the earliest dropped timestamp for a generation and advances
// the notice version even if a repeated line has the same timestamp.
func (s *Store) markGap(key genKey, tsNS int64) {
	s.mu.Lock()
	mark := s.gaps[key]
	wasPending := mark.version != 0
	mark.version++
	if mark.from == 0 || tsNS < mark.from {
		mark.from = tsNS
	}
	s.gaps[key] = mark
	s.mu.Unlock()

	if !wasPending {
		s.requestReconcile()
	}
}

// gapAt reports the generation's outstanding gap, or 0 when it has none.
func (s *Store) gapAt(key genKey) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gaps[key].from
}

func (s *Store) gapSnapshot(key genKey) gapMark {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gaps[key]
}

// clearGap retires only the exact notice version a backfill captured. A new
// drop during that read remains pending even when it has the same timestamp.
func (s *Store) clearGap(key genKey, healed gapMark) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gaps[key].version == healed.version {
		delete(s.gaps, key)
	}
}

func (s *Store) markRefresh(key genKey) {
	s.mu.Lock()
	wasPending := s.refresh[key] != 0
	s.refresh[key]++
	s.mu.Unlock()

	if !wasPending {
		s.requestReconcile()
	}
}

func (s *Store) refreshAt(key genKey) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refresh[key]
}

func (s *Store) clearRefresh(key genKey, healed uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refresh[key] == healed {
		delete(s.refresh, key)
	}
}

// Wait blocks until every store goroutine has stopped and the final batch has
// been committed.
func (s *Store) Wait() {
	s.workers.Wait()
}

// Close releases the database. Call it after Wait.
func (s *Store) Close() error {
	s.codec.close()
	return errors.Join(s.writerDB.Close(), s.db.Close())
}

// composeProject reads the compose project from container labels. Docker
// Compose and recent podman-compose set the com.docker label; older
// podman-compose releases only set the io.podman one.
func composeProject(labels map[string]string) string {
	for _, label := range []string{"com.docker.compose.project", "io.podman.compose.project"} {
		if value := labels[label]; value != "" {
			return value
		}
	}
	return ""
}
