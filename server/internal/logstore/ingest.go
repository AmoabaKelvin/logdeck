package logstore

import (
	"context"
	"database/sql"
	"log"
	"strings"
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

	// msgDone fields. complete is true only when the engine read reached EOF
	// successfully. A transiently failed read must leave the generation's
	// initial backfill pending so a retry still starts from creation.
	complete bool
	reason   string // non-empty when the generation is excluded from persistence
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

// writerState is everything the single writer goroutine remembers between
// batches. None of it is shared, so none of it is locked.
type writerState struct {
	// refs maps a generation to its containers.id.
	refs map[genKey]int64
	// hot counts each generation's unsealed lines, so the writer knows when a
	// full block's worth has accumulated without counting rows every batch.
	hot map[int64]int
	// sealedMaxTS is the newest timestamp each generation has already sealed.
	// A line newer than that cannot collide with a sealed block, which is what
	// keeps live ingestion from querying log_blocks at all.
	sealedMaxTS map[int64]int64
	// nextSeq hands out the store-wide monotonic line number.
	nextSeq int64
	// unpacked caches the most recently decompressed block, held only for the
	// span of one commit. A backfill re-read walks forward through time, so
	// consecutive lines usually land in the same block.
	unpacked      []sealedLine
	unpackedRowid int64
	scratch       []byte
}

func newWriterState() *writerState {
	return &writerState{
		refs:          make(map[genKey]int64),
		hot:           make(map[int64]int),
		sealedMaxTS:   make(map[int64]int64),
		unpackedRowid: -1,
		nextSeq:       1,
	}
}

// loadWriterState reads the writer's starting point from the database.
func (s *Store) loadWriterState() (*writerState, error) {
	state := newWriterState()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx,
		"SELECT container_ref, COUNT(*) FROM log_lines GROUP BY container_ref")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ref int64
		var count int
		if err := rows.Scan(&ref, &count); err != nil {
			rows.Close()
			return nil, err
		}
		state.hot[ref] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	blocks, err := s.db.QueryContext(ctx,
		"SELECT container_ref, MAX(ts_max_ns) FROM log_blocks GROUP BY container_ref")
	if err != nil {
		return nil, err
	}
	for blocks.Next() {
		var ref, tsMax int64
		if err := blocks.Scan(&ref, &tsMax); err != nil {
			blocks.Close()
			return nil, err
		}
		state.sealedMaxTS[ref] = tsMax
	}
	blocks.Close()
	if err := blocks.Err(); err != nil {
		return nil, err
	}

	// A reused sequence number would put two different lines at the same keyset
	// position, so the counter must clear everything either table has ever held.
	var hotMax, blockMax sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(seq) FROM log_lines").Scan(&hotMax); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(seq_max) FROM log_blocks").Scan(&blockMax); err != nil {
		return nil, err
	}
	state.nextSeq = max(hotMax.Int64, blockMax.Int64) + 1
	return state, nil
}

// writeLoop is the store's single writer. It batches queued messages into one
// transaction per batchLines rows or batchInterval, whichever comes first, and
// exits once the queue is closed and drained.
func (s *Store) writeLoop() {
	state, err := s.loadWriterState()
	if err != nil {
		// Starting from a blank slate is not safe: seq would restart at 1 and
		// collide with stored positions, so refuse to ingest rather than corrupt
		// the ordering. The store stays readable.
		log.Printf("logstore: reading writer state failed, ingestion is disabled: %v", err)
		for range s.ingestCh { //nolint:revive // drain so producers never block
		}
		return
	}
	batch := make([]ingestMsg, 0, batchLines)

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	// batchesSinceRetain counts committed batches since the last retention sweep.
	// Retention runs on this goroutine so it never competes with ingestion for
	// SQLite's single write lock; see janitorLoop.
	batchesSinceRetain := 0

	// failDelay throttles the writer after a failed commit. Every dropped batch
	// becomes a gap that backfill re-reads, so retrying at full speed turns a
	// persistent write error (locked or full database) into an endless
	// re-read-and-drop cycle that pegs the CPU. Sleeping here applies
	// backpressure to the backfill producers; the live sink keeps dropping.
	var failDelay time.Duration

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.commit(batch, state); err != nil {
			log.Printf("logstore: write batch failed (%d messages dropped): %v", len(batch), err)
			// A failed transaction drops the whole batch. Mark every generation it
			// carried, exactly like the sink's full-queue path: the next sync
			// re-reads each one from its earliest dropped line, and the insert
			// dedup makes that re-read safe.
			s.markBatchGaps(batch)
			failDelay = min(max(2*failDelay, minCommitBackoff), maxCommitBackoff)
			time.Sleep(failDelay)
		} else {
			failDelay = 0
		}
		batch = batch[:0]
		batchesSinceRetain++

		// Sealing is what makes stored history cheap, so it runs on the same
		// goroutine as ingestion rather than competing with it for the write
		// lock. A failed seal leaves the lines hot and is retried next batch.
		if err := s.sealFullBlocks(state); err != nil {
			log.Printf("logstore: sealing log blocks failed: %v", err)
		}
	}

	retain := func() {
		batchesSinceRetain = 0
		before := s.evictions.Load()
		if err := s.retain(context.Background()); err != nil {
			log.Printf("logstore: retention sweep failed: %v", err)
		}
		// Fold the WAL only when the sweep actually evicted: retention's deletes
		// are the bulk of the WAL churn, so truncating right after them keeps the
		// file near its cap. A store under its cap evicts nothing and must not pay
		// for a checkpoint here — that is what preserves the flood throughput
		// ceiling. SQLite's automatic checkpoint keeps the WAL bounded otherwise.
		if s.evictions.Load() != before {
			s.checkpoint()
		}
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
		case <-s.retainCh:
			flush()
			retain()
		}
		// Cap the file's high-water mark under sustained load: the janitorInterval
		// signal alone is far too coarse when batches flush many times a second.
		if batchesSinceRetain >= retainEveryBatches {
			retain()
		}
	}
}

// checkpoint folds the write-ahead log back into the main database and truncates
// the WAL file. It runs on the writer goroutine right after an eviction, so it
// never races the store's own writes. TRUNCATE (rather than PASSIVE) is what
// actually shrinks the file on disk; it briefly blocks new readers while it
// truncates, but queries are short enough that the pause is not observable, and
// bounding the file is worth it. A checkpoint that cannot complete because a
// reader is mid-query simply truncates less this pass and catches up on the
// next, so its error is not worth logging.
func (s *Store) checkpoint() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.writerDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
}

// markBatchGaps records a dropped-line gap for every generation in a batch the
// writer could not commit, so the lines it lost are re-read rather than gone.
// markGap keeps the earliest timestamp per generation.
func (s *Store) markBatchGaps(batch []ingestMsg) {
	for _, msg := range batch {
		if msg.kind == msgLine {
			s.markGap(msg.key, msg.line.tsNS)
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
func (s *Store) commit(batch []ingestMsg, state *writerState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	refs := state.refs

	tx, err := s.writerDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Inside the transaction, so the write lock is already held: every purge
	// that committed before this transaction began has published its
	// generations, and the ids it removed leave the cache before they can be
	// written against.
	s.dropInvalidated(state)

	// The cached block belongs to the previous transaction's snapshot; retention
	// may have deleted it since.
	state.unpacked, state.unpackedRowid = nil, -1

	// Per-generation aggregates applied once at the end of the transaction.
	type agg struct {
		bytes    int64
		stdoutWM int64
		stderrWM int64
	}
	aggs := make(map[int64]*agg)
	fresh := make(map[genKey]int64) // ids this transaction discovered
	// newHot counts lines this transaction added, published to the writer's hot
	// counts only after the commit succeeds.
	newHot := make(map[int64]int)
	nowMS := time.Now().UnixMilli()
	insertedCount := int64(0)
	// Every line uses the same dedup/insert statement. Prepare it once per
	// batch so the driver reuses SQLite's compiled statement for all 500 rows.
	insert, err := tx.PrepareContext(ctx, insertLineSQL)
	if err != nil {
		return err
	}
	defer insert.Close()

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
			if msg.complete {
				if _, err := tx.ExecContext(ctx,
					"UPDATE containers SET initial_backfill_done = 1 WHERE id = ?", ref); err != nil {
					return err
				}
			}
			if msg.reason != "" {
				if _, err := tx.ExecContext(ctx,
					"UPDATE containers SET excluded_reason = ? WHERE id = ?", msg.reason, ref); err != nil {
					return err
				}
			}
			continue
		}

		inserted, err := s.insertLine(ctx, tx, insert, state, ref, msg.line)
		if err != nil {
			return err
		}
		if !inserted {
			continue
		}
		insertedCount++
		newHot[ref]++

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
	s.committed.Add(insertedCount)

	// The ids and counts are real only now.
	for key, ref := range fresh {
		refs[key] = ref
	}
	for ref, count := range newHot {
		state.hot[ref] += count
	}
	return nil
}

// upsertGeneration inserts or refreshes the generation row for one engine
// container ID and returns its primary key.
//
// An empty incoming value never overwrites a stored one: a snapshot without
// names would otherwise blank the name, and the logical container — (host,
// name) — is what makes history survive a rebuild. An unnamed generation is
// unresolvable, so a blank name is a loss, never an update.
func upsertGeneration(ctx context.Context, tx *sql.Tx, key genKey, name, project string, nowMS int64) (int64, error) {
	var ref int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO containers (host, container_id, name, compose_project, first_seen_ms, last_seen_ms)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(host, container_id) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE containers.name END,
			compose_project = CASE WHEN excluded.compose_project != '' THEN excluded.compose_project ELSE containers.compose_project END,
			last_seen_ms = excluded.last_seen_ms
		RETURNING id`,
		key.host, key.id, name, project, nowMS, nowMS,
	).Scan(&ref)
	return ref, err
}

// insertLine stores one line unless an identical (ts_ns, stream, raw) line is
// already stored for the generation, hot or sealed, and reports whether a row
// was written.
//
// Every insert is deduplicated, not just backfilled ones: live delivery and a
// backfill re-read of the same window can reach the writer in either order
// (the hub buffers records, so a live line can arrive after the backfill that
// also read it). Note that this never drops a line by timestamp alone — only a
// byte-identical line on the same stream in the same nanosecond, which is the
// duplicate we are trying to avoid.
func (s *Store) insertLine(ctx context.Context, tx *sql.Tx, insert *sql.Stmt, state *writerState, ref int64, l line) (bool, error) {
	sealed, err := s.sealedHasLine(ctx, tx, state, ref, l)
	if err != nil {
		return false, err
	}
	if sealed {
		return false, nil
	}

	// The hot check is an index seek on (container_ref, ts_ns), the same B-tree
	// the insert already touches.
	result, err := insert.ExecContext(ctx,
		ref, l.tsNS, l.stream, l.level, l.raw, state.nextSeq,
		ref, l.tsNS, l.stream, l.raw)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows > 0 {
		state.nextSeq++
	}
	return rows > 0, nil
}

const insertLineSQL = `INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw, seq)
	SELECT ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM log_lines
		WHERE container_ref = ? AND ts_ns = ? AND stream = ? AND raw = ?
	)`

// sealedHasLine reports whether a line is already inside one of the
// generation's sealed blocks.
//
// Live ingestion never reaches the database here: its timestamps are newer than
// anything sealed, which the writer already knows. Only a backfill re-reading a
// window the store has kept gets this far, and then a block's filter usually
// rules it out without decompressing.
func (s *Store) sealedHasLine(ctx context.Context, tx *sql.Tx, state *writerState, ref int64, l line) (bool, error) {
	if l.tsNS > state.sealedMaxTS[ref] {
		return false, nil
	}

	// The payload is deliberately not selected here. This runs once per line of a
	// backfill re-read, and reading a block's compressed blob is far more
	// expensive than testing its filter; only the survivors are worth fetching.
	rows, err := tx.QueryContext(ctx, `
		SELECT rowid, dedup_filter FROM log_blocks
		WHERE container_ref = ? AND ts_min_ns <= ? AND ts_max_ns >= ?`,
		ref, l.tsNS, l.tsNS)
	if err != nil {
		return false, err
	}

	var candidates []int64
	key := lineKey(l.tsNS, l.stream, l.raw)
	for rows.Next() {
		var (
			rowid  int64
			filter []byte
		)
		if err := rows.Scan(&rowid, &filter); err != nil {
			rows.Close()
			return false, err
		}
		if !filterMayContain(filter, key) {
			continue
		}
		candidates = append(candidates, rowid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	for _, rowid := range candidates {
		lines := state.unpacked
		if state.unpackedRowid != rowid {
			var payload []byte
			if err := tx.QueryRowContext(ctx,
				"SELECT payload FROM log_blocks WHERE rowid = ?", rowid).Scan(&payload); err != nil {
				return false, err
			}
			var err error
			lines, state.scratch, err = s.codec.unpack(payload, state.scratch)
			if err != nil {
				return false, err
			}
			state.unpacked, state.unpackedRowid = lines, rowid
		}
		for _, sl := range lines {
			if sl.tsNS == l.tsNS && sl.stream == l.stream && sl.raw == l.raw {
				return true, nil
			}
		}
	}
	return false, nil
}

// sealFullBlocks compresses every complete run of hot lines into a block. It
// runs between batches on the writer goroutine, so it never competes with
// ingestion for SQLite's single write lock.
func (s *Store) sealFullBlocks(state *writerState) error {
	for ref, count := range state.hot {
		for count >= blockLines {
			sealed, err := s.sealOneBlock(ref, state)
			if err != nil {
				return err
			}
			if sealed == 0 {
				// The rows are gone (purged, or evicted by retention); trust the
				// database over the cached count.
				delete(state.hot, ref)
				break
			}
			count -= sealed
			state.hot[ref] = count
		}
	}
	return nil
}

// sealOneBlock moves a generation's oldest blockLines hot lines into a single
// compressed row, and reports how many lines it sealed.
//
// stored_bytes moves with them: the raw bytes leave and the payload arrives, so
// the retention caps keep measuring what the generation actually occupies.
func (s *Store) sealOneBlock(ref int64, state *writerState) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := s.writerDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT seq, ts_ns, stream, level, raw FROM log_lines
		WHERE container_ref = ?
		ORDER BY ts_ns, seq
		LIMIT ?`, ref, blockLines)
	if err != nil {
		return 0, err
	}
	lines := make([]sealedLine, 0, blockLines)
	rawBytes := int64(0)
	full := false
	for rows.Next() {
		var l sealedLine
		if err := rows.Scan(&l.seq, &l.tsNS, &l.stream, &l.level, &l.raw); err != nil {
			rows.Close()
			return 0, err
		}
		// A block is also full once it holds enough text, so a container that
		// logs very long lines cannot build one too large to read back. The
		// line that would cross the cap starts the next block instead; a single
		// line larger than the cap still gets a block of its own, and pack
		// refuses anything the decoder could not read back.
		if len(lines) > 0 && rawBytes+int64(len(l.raw)) > maxBlockRawBytes {
			full = true
			break
		}
		lines = append(lines, l)
		rawBytes += int64(len(l.raw))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	// A partial run stays hot: sealing it would give up most of the compression,
	// since a block earns its ratio from the lines it holds together.
	if !full && len(lines) < blockLines {
		return 0, nil
	}

	payload, filter, summary, err := s.codec.pack(lines)
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO log_blocks
			(container_ref, ts_min_ns, ts_max_ns, seq_min, seq_max,
			 lines, level_mask, raw_bytes, dedup_filter, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, summary.tsMinNS, summary.tsMaxNS, summary.seqMin, summary.seqMax,
		summary.lines, summary.levelMask, summary.rawBytes, filter, payload,
	); err != nil {
		return 0, err
	}

	seqs := make([]any, 0, len(lines)+1)
	for _, l := range lines {
		seqs = append(seqs, l.seq)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(seqs)), ",")
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM log_lines WHERE container_ref = ? AND seq IN ("+placeholders+")",
		append([]any{ref}, seqs...)...); err != nil {
		return 0, err
	}

	// The filter counts too: eviction frees payload plus filter per block, so
	// sealing must add exactly that or repeated seal/evict cycles drift
	// stored_bytes away from what the generation actually occupies.
	if _, err := tx.ExecContext(ctx, `
		UPDATE containers
		SET stored_bytes = max(0, stored_bytes - ? + ?)
		WHERE id = ?`, summary.rawBytes, int64(len(payload)+len(filter)), ref); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if summary.tsMaxNS > state.sealedMaxTS[ref] {
		state.sealedMaxTS[ref] = summary.tsMaxNS
	}
	return len(lines), nil
}
