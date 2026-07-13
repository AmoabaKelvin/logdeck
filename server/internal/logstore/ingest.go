package logstore

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// genKey identifies one container generation: a single engine container ID on
// one host.
type genKey struct {
	host string
	id   string
}

type msgKind int

const (
	msgLine msgKind = iota // a log line to store
	msgDone                // a generation's backfill finished (or was excluded)
)

const (
	streamStdout = 0
	streamStderr = 1
)

// line is one stored log line.
type line struct {
	tsNS   int64
	stream int
	level  int
	raw    string
}

// ingestMsg is the single message type on the writer's queue. Keeping lines
// and backfill completions on one ordered channel is what lets the writer know
// exactly when a generation's backfill is done and its dedup check can stop.
type ingestMsg struct {
	kind    msgKind
	key     genKey
	name    string
	project string

	line line // msgLine

	// msgDone: non-empty when the generation is excluded from persistence.
	reason string
}

// lineFromEntry converts a parsed hub/tail entry into a storable line. The
// engine timestamp is authoritative; entries without one (a log driver that
// dropped it) fall back to arrival time so they still order sensibly.
//
// Raw is stored verbatim — including the engine's timestamp prefix — so a
// stored entry can be reconstructed by the exact same parse the live path
// runs, and cannot drift from it.
func lineFromEntry(entry models.LogEntry) line {
	tsNS := time.Now().UnixNano()
	if !entry.Timestamp.IsZero() {
		tsNS = entry.Timestamp.UnixNano()
	}
	stream := streamStdout
	if entry.Stream == "stderr" {
		stream = streamStderr
	}
	return line{
		tsNS:   tsNS,
		stream: stream,
		level:  models.LevelSeverity(entry.Level),
		raw:    entry.Raw,
	}
}

// writeLoop is the store's single writer. It batches queued messages into one
// transaction per batchLines rows or batchInterval, whichever comes first, and
// exits once the queue is closed and drained.
func (s *Store) writeLoop() {
	refs := make(map[genKey]int64) // generation -> containers.id
	batch := make([]ingestMsg, 0, batchLines)

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.commit(batch, refs); err != nil {
			log.Printf("logstore: write batch failed (%d messages dropped): %v", len(batch), err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-s.ingestCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, msg)
			if len(batch) >= batchLines {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// commit writes one batch in a single transaction: it upserts every generation
// row it touches, inserts the lines, and advances stored_bytes and the
// per-stream watermarks together, so a crash can never leave a watermark ahead
// of the rows it claims.
//
// Generation ids discovered inside the transaction are held in a local map and
// published to the caller's cache only after the commit succeeds. A rolled-back
// INSERT ... RETURNING id hands back a rowid SQLite will hand out again, so
// caching it eagerly would file the *next* generation's lines against this key
// — one container's lines landing in another container's timeline.
func (s *Store) commit(batch []ingestMsg, refs map[genKey]int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Per-generation aggregates applied once at the end of the transaction.
	type agg struct {
		bytes    int64
		stdoutWM int64
		stderrWM int64
	}
	aggs := make(map[int64]*agg)
	fresh := make(map[genKey]int64) // ids this transaction discovered
	nowMS := time.Now().UnixMilli()

	for _, msg := range batch {
		ref, ok := refs[msg.key]
		if !ok {
			ref, ok = fresh[msg.key]
		}
		if !ok {
			ref, err = upsertGeneration(ctx, tx, msg.key, msg.name, msg.project, nowMS)
			if err != nil {
				return err
			}
			fresh[msg.key] = ref
		}

		if msg.kind == msgDone {
			if msg.reason != "" {
				if _, err := tx.ExecContext(ctx,
					"UPDATE containers SET excluded_reason = ? WHERE id = ?", msg.reason, ref); err != nil {
					return err
				}
			}
			continue
		}

		inserted, err := insertLine(ctx, tx, ref, msg.line)
		if err != nil {
			return err
		}
		if !inserted {
			continue
		}

		a := aggs[ref]
		if a == nil {
			a = &agg{}
			aggs[ref] = a
		}
		a.bytes += int64(len(msg.line.raw))
		if msg.line.stream == streamStderr {
			a.stderrWM = max(a.stderrWM, msg.line.tsNS)
		} else {
			a.stdoutWM = max(a.stdoutWM, msg.line.tsNS)
		}
	}

	for ref, a := range aggs {
		// max(x, 0) leaves an untouched stream's watermark alone.
		if _, err := tx.ExecContext(ctx, `
			UPDATE containers
			SET stored_bytes = stored_bytes + ?,
			    stdout_wm_ns = max(stdout_wm_ns, ?),
			    stderr_wm_ns = max(stderr_wm_ns, ?),
			    last_seen_ms = ?
			WHERE id = ?`,
			a.bytes, a.stdoutWM, a.stderrWM, nowMS, ref); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// The ids are real only now.
	for key, ref := range fresh {
		refs[key] = ref
	}
	return nil
}

// upsertGeneration inserts or refreshes the generation row for one engine
// container ID and returns its primary key.
func upsertGeneration(ctx context.Context, tx *sql.Tx, key genKey, name, project string, nowMS int64) (int64, error) {
	var ref int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO containers (host, container_id, name, compose_project, first_seen_ms, last_seen_ms)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(host, container_id) DO UPDATE SET
			name = excluded.name,
			compose_project = CASE WHEN excluded.compose_project != '' THEN excluded.compose_project ELSE containers.compose_project END,
			last_seen_ms = excluded.last_seen_ms
		RETURNING id`,
		key.host, key.id, name, project, nowMS, nowMS,
	).Scan(&ref)
	return ref, err
}

// insertLine stores one line unless an identical (ts_ns, stream, raw) row is
// already stored for the generation, and reports whether a row was written.
//
// Every insert is deduplicated, not just backfilled ones: live delivery and a
// backfill re-read of the same window can reach the writer in either order
// (the hub buffers records, so a live line can arrive after the backfill that
// also read it). The check is an index seek on (container_ref, ts_ns), the
// same B-tree the insert already touches. Note that this never drops a line by
// timestamp alone — only a byte-identical line on the same stream in the same
// nanosecond, which is the duplicate we are trying to avoid.
func insertLine(ctx context.Context, tx *sql.Tx, ref int64, l line) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw)
		SELECT ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM log_lines
			WHERE container_ref = ? AND ts_ns = ? AND stream = ? AND raw = ?
		)`,
		ref, l.tsNS, l.stream, l.level, l.raw,
		ref, l.tsNS, l.stream, l.raw)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
