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
	// syncInterval is the container lifecycle poll cadence.
	syncInterval = 15 * time.Second
	// janitorInterval is the retention sweep cadence.
	janitorInterval = time.Minute
	// dropLogEvery throttles the "sink fell behind" warning.
	dropLogEvery = 1000
)

// Hub is the slice of *logstream.Hub the store consumes. Tests inject fakes.
type Hub interface {
	Subscribe(spec logstream.ContainerSpec, opts models.LogOptions, sink func(logstream.Record)) func()
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
	db     *sql.DB
	path   string
	limits Limits

	ingestCh chan ingestMsg
	drops    atomic.Uint64

	// producers counts the goroutines that may still send on ingestCh (the
	// hub sink and in-flight backfills); ingestCh is closed once they stop.
	producers sync.WaitGroup
	// workers counts every goroutine the store started, so Wait can block
	// until the pipeline has fully drained.
	workers sync.WaitGroup

	// backfillState is owned by the sync loop.
	backfillSem chan struct{}
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

	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open log store: %w", err)
	}

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

	return &Store{
		db:          db,
		path:        path,
		limits:      limits,
		ingestCh:    make(chan ingestMsg, ingestBuffer),
		backfillSem: make(chan struct{}, maxConcurrentBackfills),
	}, nil
}

// Start runs the ingestion, lifecycle, and retention loops until ctx is
// cancelled. Use Wait to block until everything has drained.
func (s *Store) Start(ctx context.Context, hub Hub, provider DockerProvider) {
	s.start(ctx, hub, func() Engine { return provider.Docker() })
}

// start is the injectable form of Start; tests supply fakes.
func (s *Store) start(ctx context.Context, hub Hub, source func() Engine) {
	// The hub sink is a producer: it must stop before ingestCh is closed.
	s.producers.Add(1)
	unsubscribe := hub.Subscribe(
		logstream.ContainerSpec{}, // all containers on all hosts
		models.LogOptions{Follow: true, Timestamps: true, Tail: "0", ShowStdout: true, ShowStderr: true},
		s.sink,
	)

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.writeLoop()
	}()

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.syncLoop(ctx, source)
	}()

	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.janitorLoop(ctx)
	}()

	// Shutdown: drop the subscription (after which the sink is never called
	// again), let in-flight backfills finish, then close the queue so the
	// writer flushes what is left and exits.
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		<-ctx.Done()
		unsubscribe()
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
		if n := s.drops.Add(1); n%dropLogEvery == 0 {
			log.Printf("logstore: dropped %d log lines (writer fell behind)", n)
		}
	}
}

// Wait blocks until every store goroutine has stopped and the final batch has
// been committed.
func (s *Store) Wait() {
	s.workers.Wait()
}

// Close releases the database. Call it after Wait.
func (s *Store) Close() error {
	return s.db.Close()
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
