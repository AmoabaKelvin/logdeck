package logstore

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// openStreams resolves the generations a query reads and positions each one
// below the cursor. Generations with nothing stored in range are left out.
func (s *Store) openStreams(ctx context.Context, r *pageRead, start *pageCursor) ([]*stream, error) {
	q := r.q
	generations, err := generations(ctx, r.conn, q.Host, q.Container, q.Project)
	if err != nil || len(generations) == 0 {
		return nil, err
	}
	newest, err := newestLines(ctx, r.conn, generations)
	if err != nil {
		return nil, err
	}

	streams := make([]*stream, 0, len(generations))
	for _, gen := range generations {
		newestTS, ok := newest[gen.ref]
		if !ok {
			continue
		}
		// Starting at the newest stored line rather than at the top of time
		// means a generation the page never reaches is never read.
		from := cursorPos{tsNS: newestTS + 1}
		if !q.Until.IsZero() {
			from = older(from, cursorPos{tsNS: q.Until.UnixNano() + 1})
		}
		if start != nil {
			resume, ok := start.resume[gen.ref]
			if !ok {
				resume = start.pos
			}
			from = older(from, resume)
		}
		if !q.Since.IsZero() && from.tsNS < q.Since.UnixNano() {
			continue
		}
		streams = append(streams, &stream{gen: gen, from: from, chunk: r.chunk})
	}
	if len(streams) > 1 {
		for _, st := range streams {
			st.chunk = min(st.chunk, firstChunk)
		}
	}
	return streams, nil
}

// newestStream returns the stream holding the newest entry not yet handed out,
// or nil once every stream is drained.
func newestStream(streams []*stream) *stream {
	var best *stream
	for _, st := range streams {
		if st.drained() {
			continue
		}
		if best == nil || st.ceiling().newer(best.ceiling()) {
			best = st
		}
	}
	return best
}

// pageRead is what every stream of one readPage call shares.
type pageRead struct {
	conn  *sql.Conn
	q     LogQuery
	match matcher
	chunk int // the most rows one advance reads

	// Buffers reused by every advance. Nothing a stream keeps points into
	// them: pending entries and carried rows are copied out.
	block   decodedBlock
	lines   []sealedLine
	rows    []storedRow
	grouped []groupedEntry
}

// stream reads one generation newest-first, a chunk of rows at a time, and
// holds its matching entries until the merge hands them out.
type stream struct {
	gen     generation
	from    cursorPos      // the next chunk reads rows strictly older than this
	chunk   int            // rows the next advance reads
	carry   []storedRow    // the oldest entry read so far, which may still grow
	pending []groupedEntry // matched and complete, newest-first
	done    bool           // every row has been read
}

// ceiling bounds the entries the stream has not handed out: each starts
// strictly older than it.
func (st *stream) ceiling() cursorPos {
	switch {
	case len(st.pending) > 0:
		return st.pending[0].anchor.next()
	case len(st.carry) > 0:
		// The carried entry starts at the oldest row read, or further back.
		return st.from.next()
	}
	return st.from
}

func (st *stream) drained() bool {
	return st.done && len(st.pending) == 0
}

// advance reads the stream's next chunk, groups it, and keeps the matching
// entries. It returns how many rows it read.
func (s *Store) advance(ctx context.Context, r *pageRead, st *stream) (int, error) {
	combined, n, err := s.scanRows(ctx, r, st)
	if err != nil {
		return 0, err
	}
	st.done = n < st.chunk
	st.chunk = min(st.chunk*2, r.chunk)
	if !st.done {
		st.from = combined[len(combined)-1].pos
	}

	// The oldest entry's parent line may be one row past the chunk, so it is
	// regrouped with the next chunk instead of being filtered while incomplete.
	grouped, open := groupRows(combined, r.grouped[:0])
	r.grouped = grouped
	st.carry = nil
	if !st.done && len(grouped) > 0 {
		st.carry = slices.Clone(combined[open:])
		grouped = grouped[1:]
	}

	kept := grouped[:0]
	for _, entry := range grouped {
		if r.match.matches(entry.entry) {
			kept = append(kept, entry)
		}
	}
	slices.Reverse(kept)
	st.pending = slices.Clone(kept)
	return n, nil
}

// scanRows reads the stream's next chunk — its newest rows strictly older than
// st.from — and returns it newest-first behind the carried rows it continues,
// along with how many rows it read.
//
// Lines live in two places — the hot table holds what has not been sealed yet,
// sealed blocks hold everything older — and a hot line is not always newer
// than a sealed one, so both are read and merged. Taking the newest chunk of
// the union is exact: each source contributed its own newest chunk. Lines are
// parsed only after the cut, so rows read past it cost no parse.
func (s *Store) scanRows(ctx context.Context, r *pageRead, st *stream) ([]storedRow, int, error) {
	lines, err := s.scanHotLines(ctx, r, st, r.lines[:0])
	if err != nil {
		return nil, 0, err
	}
	lines, err = s.scanSealedLines(ctx, r, st, lines)
	if err != nil {
		return nil, 0, err
	}
	r.lines = lines
	sortNewestFirst(lines)
	lines = lines[:min(len(lines), st.chunk)]

	rows := append(r.rows[:0], st.carry...)
	for _, l := range lines {
		rows = append(rows, storedRow{
			entry: entryFromRow(l.tsNS, l.stream, l.raw, st.gen),
			pos:   l.pos(),
		})
	}
	r.rows = rows
	return rows, len(lines), nil
}

// scanHotLines appends unsealed lines, read straight out of log_lines, to lines.
func (s *Store) scanHotLines(ctx context.Context, r *pageRead, st *stream, lines []sealedLine) ([]sealedLine, error) {
	q, chunk := r.q, st.chunk
	statement := "SELECT seq, ts_ns, stream, raw FROM log_lines" +
		" WHERE container_ref = ? AND ts_ns <= ? AND (ts_ns < ? OR seq < ?)"
	args := []any{st.gen.ref, st.from.tsNS, st.from.tsNS, st.from.seq}
	if !q.Since.IsZero() {
		statement += " AND ts_ns >= ?"
		args = append(args, q.Since.UnixNano())
	}
	statement += " ORDER BY ts_ns DESC, seq DESC LIMIT ?"
	args = append(args, chunk)

	rows, err := r.conn.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var l sealedLine
		if err := rows.Scan(&l.seq, &l.tsNS, &l.stream, &l.raw); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}

// scanSealedLines appends lines from compressed blocks to lines, reading blocks
// newest-first and stopping as soon as a full chunk is held. Blocks outside the
// query's time window, or entirely newer than the stream's position, are ruled
// out by their stored bounds and are never decompressed.
func (s *Store) scanSealedLines(ctx context.Context, r *pageRead, st *stream, lines []sealedLine) ([]sealedLine, error) {
	q, chunk := r.q, st.chunk
	statement := "SELECT ts_max_ns, seq_max, payload FROM log_blocks" +
		" WHERE container_ref = ? AND (ts_min_ns < ? OR (ts_min_ns = ? AND seq_min < ?))"
	args := []any{st.gen.ref, st.from.tsNS, st.from.tsNS, st.from.seq}
	if !q.Since.IsZero() {
		statement += " AND ts_max_ns >= ?"
		args = append(args, q.Since.UnixNano())
	}
	statement += " ORDER BY ts_max_ns DESC, seq_max DESC"

	rows, err := r.conn.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tsMax   int64
			seqMax  int64
			payload []byte
		)
		if err := rows.Scan(&tsMax, &seqMax, &payload); err != nil {
			return nil, err
		}

		// Blocks arrive ordered by their newest line, but a backfill re-read
		// makes their time ranges overlap, so a later block can still hold lines
		// newer than ones already collected. Stopping on count alone would drop
		// those, and the cursor would then page straight past them. Stop only
		// once this block's newest possible position is older than the chunk
		// boundary, which is the point nothing further can qualify.
		if len(lines) >= chunk {
			sortNewestFirst(lines)
			lines = lines[:chunk]
			if !(cursorPos{tsNS: tsMax, seq: seqMax}).newer(lines[chunk-1].pos()) {
				break
			}
		}

		if err := s.codec.decode(payload, &r.block); err != nil {
			return nil, err
		}

		// Only this block's newest chunk of qualifying lines can make the cut,
		// so only those have their raw line rebuilt.
		kept := 0
		for i := len(r.block.lines) - 1; i >= 0 && kept < chunk; i-- { // newest-first
			l := r.block.lines[i]
			if !st.from.newer(l.pos()) || (!q.Since.IsZero() && l.tsNS < q.Since.UnixNano()) {
				continue
			}
			l.raw = r.block.raw(i)
			lines = append(lines, l)
			kept++
		}
	}
	return lines, rows.Err()
}

func sortNewestFirst(lines []sealedLine) {
	slices.SortFunc(lines, func(a, b sealedLine) int {
		if a.pos().newer(b.pos()) {
			return -1
		}
		return 1
	})
}

// newestLines maps each generation that has stored lines to the timestamp of
// its newest one. Both halves are answered from an index.
func newestLines(ctx context.Context, conn *sql.Conn, generations []generation) (map[int64]int64, error) {
	refs := make([]int64, len(generations))
	for i, gen := range generations {
		refs[i] = gen.ref
	}
	placeholders, args := refArgs(refs)
	rows, err := conn.QueryContext(ctx,
		"SELECT container_ref, MAX(ts_ns) FROM log_lines WHERE container_ref IN ("+placeholders+") GROUP BY container_ref"+
			" UNION ALL "+
			"SELECT container_ref, MAX(ts_max_ns) FROM log_blocks WHERE container_ref IN ("+placeholders+") GROUP BY container_ref",
		append(args, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	newest := make(map[int64]int64, len(generations))
	for rows.Next() {
		var ref, tsNS int64
		if err := rows.Scan(&ref, &tsNS); err != nil {
			return nil, err
		}
		if current, ok := newest[ref]; !ok || tsNS > current {
			newest[ref] = tsNS
		}
	}
	return newest, rows.Err()
}

// cursorPos is the keyset position of one stored line: its timestamp and its
// store-wide sequence number. seq keeps the position stable when the line moves
// from the hot table into a sealed block, where it no longer has a rowid.
type cursorPos struct {
	tsNS int64
	seq  int64
}

// newer reports whether p sorts before other in the newest-first order pages
// are read in.
func (p cursorPos) newer(other cursorPos) bool {
	if p.tsNS != other.tsNS {
		return p.tsNS > other.tsNS
	}
	return p.seq > other.seq
}

// next is the position just above p, for bounds that must include p itself.
func (p cursorPos) next() cursorPos {
	return cursorPos{tsNS: p.tsNS, seq: p.seq + 1}
}

func older(a, b cursorPos) cursorPos {
	if a.newer(b) {
		return b
	}
	return a
}

func (l sealedLine) pos() cursorPos {
	return cursorPos{tsNS: l.tsNS, seq: l.seq}
}

// storedRow is one scanned row: the entry it parses to and where it sits.
type storedRow struct {
	entry models.LogEntry
	pos   cursorPos
}

// groupedEntry is one grouped entry and the span of rows it covers.
type groupedEntry struct {
	entry  models.LogEntry
	anchor cursorPos // its first, oldest row
	last   cursorPos // its newest row
}

// entryFromRow rebuilds a log entry from a stored row. The raw line is parsed
// by the very same function the live path uses, so message cleaning and level
// classification cannot drift; only the timestamp is taken from the stored
// engine timestamp rather than re-derived, which keeps an app-embedded
// timestamp inside the line from overriding it.
func entryFromRow(tsNS int64, stream int, raw string, gen generation) models.LogEntry {
	name := "stdout"
	if stream == streamStderr {
		name = "stderr"
	}
	entry := models.ParseLogLine(raw, name)
	entry.Timestamp = time.Unix(0, tsNS).UTC()
	entry.ContainerID = gen.id
	entry.ContainerName = gen.name
	entry.Host = gen.host
	return entry
}

// groupRows folds continuation lines into their parent entry exactly like the
// live historical path. Rows are all one generation's, so a rebuild boundary
// can never merge two containers' lines.
//
// Rows arrive newest-first; entries come back oldest-first, each with the
// position of its *first* row. A continuation line is always newer than its
// parent, so resuming at the parent keeps the entry whole rather than
// splitting its body across two pages. Entries are appended to entries.
//
// The second return is the index in rows at which the oldest entry begins. It
// is the entry whose parent may lie beyond the scanned rows, so a caller that
// has not reached the end of history hands those rows to the next, older chunk.
func groupRows(rows []storedRow, entries []groupedEntry) ([]groupedEntry, int) {
	open := len(rows) - 1 // the oldest row starts the oldest entry

	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if last := len(entries) - 1; last >= 0 && models.IsContinuationLogEntry(row.entry, entries[last].entry) {
			models.AppendContinuationLine(&entries[last].entry, row.entry)
			entries[last].last = row.pos
			if last == 0 {
				open = i // a continuation line of the oldest entry
			}
			continue
		}
		entries = append(entries, groupedEntry{entry: row.entry, anchor: row.pos, last: row.pos})
	}
	return entries, open
}

// pageCursor is where the next page resumes: strictly below pos. A generation
// whose entry straddles pos — it starts below and continues above — resumes
// from its own position instead, so the entry comes back whole. (ts_ns, seq)
// is unique and immutable, since a line keeps its sequence number when it is
// sealed, so pages stay stable while lines are ingested and compressed.
type pageCursor struct {
	pos    cursorPos
	resume map[int64]cursorPos // generation ref -> its own position
}

func encodeCursor(pos cursorPos, streams []*stream) string {
	buf := fmt.Appendf(nil, "%d:%d", pos.tsNS, pos.seq)
	for _, st := range streams {
		var newest cursorPos
		switch {
		case len(st.pending) > 0:
			newest = st.pending[0].last
		case len(st.carry) > 0:
			newest = st.carry[0].pos
		default:
			continue
		}
		if !pos.newer(newest) {
			buf = fmt.Appendf(buf, ";%d:%d:%d", st.gen.ref, newest.tsNS, newest.seq+1)
		}
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func decodeCursor(cursor string) (*pageCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	parts := strings.Split(string(decoded), ";")
	pos, ok := parseCursorFields(parts[0], 2)
	if !ok {
		return nil, ErrInvalidCursor
	}
	start := &pageCursor{pos: cursorPos{tsNS: pos[0], seq: pos[1]}}
	for _, part := range parts[1:] {
		fields, ok := parseCursorFields(part, 3)
		if !ok {
			return nil, ErrInvalidCursor
		}
		if start.resume == nil {
			start.resume = make(map[int64]cursorPos)
		}
		start.resume[fields[0]] = cursorPos{tsNS: fields[1], seq: fields[2]}
	}
	return start, nil
}

func parseCursorFields(part string, n int) ([]int64, bool) {
	fields := strings.Split(part, ":")
	if len(fields) != n {
		return nil, false
	}
	values := make([]int64, n)
	for i, field := range fields {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, false
		}
		values[i] = value
	}
	return values, true
}
