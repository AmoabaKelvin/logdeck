package logstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	// maxConcurrentBackfills bounds how many generations are re-read at once.
	maxConcurrentBackfills = 4
	// backfillOverlap re-reads a little before the watermark so a line that
	// landed in the same instant as the last stored one cannot slip through
	// the gap. The overlap is deduplicated on insert.
	backfillOverlap = 2 * time.Second
	// maxBackfillAttempts bounds retries of a transiently failing backfill
	// within one process lifetime.
	maxBackfillAttempts = 3
)

// backfillResult reports a finished backfill back to the sync loop.
type backfillResult struct {
	key      genKey
	err      error
	excluded bool
}

// unreadableDriverErr reports whether the engine cannot read this container's
// logs at all — a logging driver without a read API (awslogs, syslog, none,
// ...). Such a generation is excluded permanently instead of being retried.
func unreadableDriverErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not support reading") || // Docker
		strings.Contains(msg, "cannot read logs") // Podman
}

// backfill re-reads one generation's engine logs from its watermark (or from
// container creation for a generation we have never stored) and pushes the
// lines onto the writer queue. It runs in its own goroutine, bounded by
// backfillSem, and never blocks live ingestion: both feed the same queue and
// the writer deduplicates the overlap.
//
// Every request carries Tail: "" (unlimited) — the engine's default of 100
// lines would silently cap gap healing.
func (s *Store) backfill(ctx context.Context, engine Engine, info models.ContainerInfo, results chan<- backfillResult) {
	defer s.producers.Done()

	key := genKey{host: info.Host, id: info.ID}

	select {
	case s.backfillSem <- struct{}{}:
		defer func() { <-s.backfillSem }()
	case <-ctx.Done():
		s.report(ctx, results, backfillResult{key: key, err: ctx.Err()})
		return
	}

	since, err := s.backfillSince(ctx, key, info.Created)
	if err != nil {
		s.report(ctx, results, backfillResult{key: key, err: err})
		return
	}

	name := containerName(info)
	project := composeProject(info.Labels)
	opts := models.LogOptions{
		Follow:     false,
		Timestamps: true,
		Since:      since,
		Tail:       "", // unlimited: a capped tail would truncate the gap
		ShowStdout: true,
		ShowStderr: true,
	}

	tailErr := engine.TailContainerLogs(ctx, key.host, key.id, opts, func(entry models.LogEntry) {
		s.send(ctx, ingestMsg{
			kind:    msgLine,
			key:     key,
			name:    name,
			project: project,
			line:    lineFromEntry(entry),
		})
	})

	excluded := unreadableDriverErr(tailErr)
	done := ingestMsg{kind: msgDone, key: key, name: name, project: project}
	if excluded {
		done.reason = tailErr.Error()
	}
	s.send(ctx, done)

	s.report(ctx, results, backfillResult{key: key, err: tailErr, excluded: excluded})
}

// backfillSince computes the engine "since" for one generation. stdout and
// stderr are demuxed independently, so their high-water marks advance
// independently; the safe resume point is the lowest mark that has actually
// advanced, minus the overlap. A generation with no stored lines is read from
// container creation.
func (s *Store) backfillSince(ctx context.Context, key genKey, createdUnix int64) (string, error) {
	var stdoutWM, stderrWM int64
	err := s.db.QueryRowContext(ctx,
		"SELECT stdout_wm_ns, stderr_wm_ns FROM containers WHERE host = ? AND container_id = ?",
		key.host, key.id,
	).Scan(&stdoutWM, &stderrWM)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	wm := watermark(stdoutWM, stderrWM)
	if wm == 0 {
		return time.Unix(createdUnix, 0).UTC().Format(time.RFC3339Nano), nil
	}
	return time.Unix(0, wm).Add(-backfillOverlap).UTC().Format(time.RFC3339Nano), nil
}

// watermark is the backfill resume point for a generation: the minimum of the
// per-stream marks that have advanced. A stream that has never produced a line
// carries no information and must not drag the mark back to zero.
func watermark(stdoutWM, stderrWM int64) int64 {
	switch {
	case stdoutWM == 0:
		return stderrWM
	case stderrWM == 0:
		return stdoutWM
	default:
		return min(stdoutWM, stderrWM)
	}
}

// send queues a message for the writer. Backfill may block here (unlike the
// live sink, which drops): the writer keeps draining, and losing backfilled
// history to a full buffer would defeat the point.
func (s *Store) send(ctx context.Context, msg ingestMsg) {
	select {
	case s.ingestCh <- msg:
	case <-ctx.Done():
	}
}

func (s *Store) report(ctx context.Context, results chan<- backfillResult, res backfillResult) {
	select {
	case results <- res:
	case <-ctx.Done():
	}
}

// containerName is the slash-stripped primary name of a listed container.
func containerName(info models.ContainerInfo) string {
	if len(info.Names) == 0 {
		return ""
	}
	return strings.TrimPrefix(info.Names[0], "/")
}
