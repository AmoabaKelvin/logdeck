package logstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	// DefaultQueryLimit and MaxQueryLimit clamp LogQuery.Limit.
	DefaultQueryLimit = 500
	MaxQueryLimit     = 1000
	// scanChunk is how many rows a filtered query reads per round. Level and
	// search filters run on *grouped* entries, so a page is filled by scanning
	// backwards until it holds a full page or history runs out; a page is never
	// both empty and continuable.
	scanChunk = 1000
	// scanBudget bounds how many rows one Query reads, so a rare term cannot
	// hold a request for seconds; the page resumes where it stopped.
	scanBudget = 200_000
	// firstChunk is what each stream of a multi-container page reads first;
	// a stream the merge keeps returning to doubles its chunk.
	firstChunk = 64
)

// ErrInvalidCursor is returned when LogQuery.Cursor is not a cursor this store
// produced.
var ErrInvalidCursor = errors.New("invalid cursor")

// StoredContainer is one logical container (host, name) in the store —
// every generation of that name collapsed into a single entry, including
// containers the engine no longer knows about.
type StoredContainer struct {
	Host           string    `json:"host"`
	Name           string    `json:"name"`
	ComposeProject string    `json:"composeProject,omitempty"`
	Image          string    `json:"image,omitempty"`
	StoredBytes    int64     `json:"storedBytes"`
	OldestTs       time.Time `json:"oldestTs"`
	NewestTs       time.Time `json:"newestTs"`
	Removed        bool      `json:"removed"`
	Excluded       bool      `json:"excluded"`
	ExcludedReason string    `json:"excludedReason,omitempty"`
}

// LogQuery selects stored lines. Every field is optional: Container, Host,
// and Project narrow which containers are read (none reads them all), and
// Since/Until, Levels, and Search filter their lines.
type LogQuery struct {
	Host      string
	Container string // logical container name
	Project   string // Compose project
	Since     time.Time
	Until     time.Time
	Levels    []string // level names, e.g. "ERROR"; empty = all levels
	Search    string
	Regex     bool // Search is an RE2 pattern rather than a substring
	Limit     int  // clamped to [1, MaxQueryLimit]; 0 means DefaultQueryLimit
	Cursor    string
}

// LogPage is one page of stored lines. Entries are ascending by timestamp;
// pages walk backwards through history, so following NextCursor yields
// successively older pages. NextCursor is empty at the end of the history.
type LogPage struct {
	Entries    []models.LogEntry `json:"entries"`
	NextCursor string            `json:"nextCursor,omitempty"`
	// ScannedTo is set when Query hit its scan budget before filling the page:
	// everything newer has been searched, and NextCursor carries on below it.
	ScannedTo time.Time `json:"scannedTo,omitzero"`
}

// ListContainers returns every logical container in the store, newest data
// first is not assumed — entries are sorted by host then name. The slice is
// never nil.
func (s *Store) ListContainers(ctx context.Context) ([]StoredContainer, error) {
	spans, err := s.lineSpans(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, host, container_id, name, compose_project, image,
		       first_seen_ms, removed_ms, stored_bytes, excluded_reason
		FROM containers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type accum struct {
		container   StoredContainer
		currentSeen int64 // first_seen_ms of the generation whose metadata is shown
		currentRef  int64 // tie-break: the later row is the later generation
	}
	byName := make(map[genKey]*accum)

	for rows.Next() {
		var (
			ref         int64
			host        string
			containerID string
			name        string
			project     string
			image       string
			firstSeenMS int64
			removedMS   *int64
			storedBytes int64
			reason      string
		)
		if err := rows.Scan(&ref, &host, &containerID, &name, &project, &image,
			&firstSeenMS, &removedMS, &storedBytes, &reason); err != nil {
			return nil, err
		}

		key := genKey{host: host, id: name}
		entry, ok := byName[key]
		if !ok {
			entry = &accum{
				container:   StoredContainer{Host: host, Name: name},
				currentSeen: -1,
			}
			byName[key] = entry
		}

		entry.container.StoredBytes += storedBytes
		if span, ok := spans[ref]; ok {
			if entry.container.OldestTs.IsZero() || span.oldest.Before(entry.container.OldestTs) {
				entry.container.OldestTs = span.oldest
			}
			if span.newest.After(entry.container.NewestTs) {
				entry.container.NewestTs = span.newest
			}
		}

		// The newest generation carries the metadata a user expects to see.
		if firstSeenMS > entry.currentSeen || (firstSeenMS == entry.currentSeen && ref > entry.currentRef) {
			entry.currentSeen, entry.currentRef = firstSeenMS, ref
			entry.container.ComposeProject = project
			entry.container.Image = image
			entry.container.Removed = removedMS != nil
			entry.container.Excluded = reason != ""
			entry.container.ExcludedReason = reason
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	containers := make([]StoredContainer, 0, len(byName))
	for _, entry := range byName {
		containers = append(containers, entry.container)
	}
	sort.Slice(containers, func(i, j int) bool {
		if containers[i].Host != containers[j].Host {
			return containers[i].Host < containers[j].Host
		}
		return containers[i].Name < containers[j].Name
	})
	return containers, nil
}

type lineSpan struct {
	oldest time.Time
	newest time.Time
}

// lineSpans reports the stored time range of every generation that has lines,
// covering both the hot table and the sealed blocks. A block's bounds are stored
// alongside it, so the full range costs no decompression.
func (s *Store) lineSpans(ctx context.Context) (map[int64]lineSpan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT container_ref, MIN(ts_ns), MAX(ts_ns) FROM log_lines GROUP BY container_ref
		UNION ALL
		SELECT container_ref, MIN(ts_min_ns), MAX(ts_max_ns) FROM log_blocks GROUP BY container_ref`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spans := make(map[int64]lineSpan)
	for rows.Next() {
		var ref, oldest, newest int64
		if err := rows.Scan(&ref, &oldest, &newest); err != nil {
			return nil, err
		}
		span, seen := spans[ref]
		if !seen || time.Unix(0, oldest).UTC().Before(span.oldest) {
			span.oldest = time.Unix(0, oldest).UTC()
		}
		if !seen || time.Unix(0, newest).UTC().After(span.newest) {
			span.newest = time.Unix(0, newest).UTC()
		}
		spans[ref] = span
	}
	return spans, rows.Err()
}

// generation is one stored container generation resolved for a query.
type generation struct {
	ref  int64
	host string
	id   string
	name string
}

// Query returns one page of stored lines, merged by timestamp across the
// containers it reads. Every generation of a name is part of its timeline,
// which is what makes history survive a rebuild: the caller asks for "web" and
// gets the lines of every engine container that has ever been called "web".
// It reads at most scanBudget rows; a page cut short by that carries ScannedTo.
func (s *Store) Query(ctx context.Context, q LogQuery) (LogPage, error) {
	return s.readPage(ctx, q, scanBudget)
}

// readPage merges one stream per generation, newest entry first. Each
// generation is grouped on its own, so however two containers' lines
// interleave, one's continuation lines never fold into the other's entry.
//
// Rows are read unfiltered and only then grouped and filtered, which is the
// order the live path uses. Filtering rows in SQL first would delete the
// continuation lines of every multi-line entry — they classify as UNKNOWN — and
// a level-filtered stack trace would come back as its first line with no body.
func (s *Store) readPage(ctx context.Context, q LogQuery, budget int) (LogPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	limit = min(limit, MaxQueryLimit)

	match, err := newMatcher(q)
	if err != nil {
		return LogPage{}, err
	}

	var cursor *pageCursor
	if q.Cursor != "" {
		if cursor, err = decodeCursor(q.Cursor); err != nil {
			return LogPage{}, err
		}
	}

	// The whole page reads one snapshot, so a line sealed mid-page is never
	// read twice or missed, and the read lock is taken once rather than per
	// statement.
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return LogPage{}, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		return LogPage{}, err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()

	// One extra entry beyond the page is what proves an older page exists, and
	// one more row covers the entry held back at the chunk boundary, so an
	// unfiltered page of one generation still settles in a single round.
	r := &pageRead{conn: conn, q: q, match: match, chunk: limit + 2}
	if match.active() {
		r.chunk = max(r.chunk, scanChunk)
	}

	streams, err := s.openStreams(ctx, r, cursor)
	if err != nil {
		return LogPage{}, err
	}

	var entries []models.LogEntry // newest-first
	start := cursorPos{tsNS: math.MaxInt64}
	if cursor != nil {
		start = cursor.pos
	}
	scanned := 0
	for {
		st := newestStream(streams)
		if st == nil {
			return newPage(entries, ""), nil
		}
		if len(st.pending) > 0 {
			if len(entries) == limit {
				return newPage(entries, encodeCursor(st.ceiling(), streams)), nil
			}
			entries = append(entries, st.pending[0].entry)
			st.pending = st.pending[1:]
			continue
		}
		// The budget only stops a page that has moved past its own cursor. A
		// resumed page starts every stream at the cursor, so stopping before
		// each has read once would hand the same cursor back forever.
		if scanned >= budget && start.newer(st.ceiling()) {
			page := newPage(entries, encodeCursor(st.ceiling(), streams))
			page.ScannedTo = time.Unix(0, st.ceiling().tsNS).UTC()
			return page, nil
		}
		// Each chunk's statements run uncancellable: the driver starts a watcher
		// goroutine per cancellable statement, so the page checks between chunks.
		if err := ctx.Err(); err != nil {
			return LogPage{}, err
		}
		n, err := s.advance(context.WithoutCancel(ctx), r, st)
		if err != nil {
			return LogPage{}, err
		}
		scanned += n
	}
}

func newPage(entries []models.LogEntry, cursor string) LogPage {
	if entries == nil {
		entries = []models.LogEntry{}
	}
	slices.Reverse(entries)
	return LogPage{Entries: entries, NextCursor: cursor}
}

// generations resolves the generation rows a query reads: every generation of
// the logical container name, or of every container when name is empty. host
// and project narrow the set when they are set.
func generations(ctx context.Context, conn *sql.Conn, host, name, project string) ([]generation, error) {
	statement := "SELECT id, host, container_id, name FROM containers"
	var (
		where []string
		args  []any
	)
	for _, filter := range []struct{ column, value string }{
		{"host", host}, {"name", name}, {"compose_project", project},
	} {
		if filter.value != "" {
			where = append(where, filter.column+" = ?")
			args = append(args, filter.value)
		}
	}
	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}

	rows, err := conn.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var generations []generation
	for rows.Next() {
		var gen generation
		if err := rows.Scan(&gen.ref, &gen.host, &gen.id, &gen.name); err != nil {
			return nil, err
		}
		generations = append(generations, gen)
	}
	return generations, rows.Err()
}

// matcher applies the level and search filters to grouped entries, with the
// same semantics as the live view: case-insensitive, over the parsed message
// rather than the raw line, so History finds what Live finds and a search for a
// timestamp-like string cannot match the engine's timestamp prefix.
type matcher struct {
	levels []int
	needle string         // lowercased substring search
	regex  *regexp.Regexp // case-insensitive pattern search
}

func newMatcher(q LogQuery) (matcher, error) {
	m := matcher{levels: levelSeverities(q.Levels)}
	switch {
	case q.Search == "":
	case q.Regex:
		compiled, err := regexp.Compile("(?i)" + q.Search)
		if err != nil {
			return matcher{}, fmt.Errorf("invalid search pattern: %w", err)
		}
		m.regex = compiled
	default:
		m.needle = strings.ToLower(q.Search)
	}
	return m, nil
}

func (m matcher) active() bool {
	return len(m.levels) > 0 || m.needle != "" || m.regex != nil
}

func (m matcher) matches(entry models.LogEntry) bool {
	if len(m.levels) > 0 && !slices.Contains(m.levels, models.LevelSeverity(entry.Level)) {
		return false
	}
	switch {
	case m.regex != nil:
		return m.regex.MatchString(entry.Message)
	case m.needle != "":
		return strings.Contains(strings.ToLower(entry.Message), m.needle)
	}
	return true
}

// levelSeverities maps level names to the severities stored on each row,
// dropping duplicates. An unrecognized name maps to UNKNOWN, matching the
// live path's classification.
func levelSeverities(levels []string) []int {
	severities := make([]int, 0, len(levels))
	for _, level := range levels {
		if level == "" {
			continue
		}
		severity := models.LevelSeverity(models.LogLevel(strings.ToUpper(strings.TrimSpace(level))))
		if !slices.Contains(severities, severity) {
			severities = append(severities, severity)
		}
	}
	return severities
}
