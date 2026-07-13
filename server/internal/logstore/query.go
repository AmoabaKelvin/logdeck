package logstore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
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

// LogQuery selects stored lines for one logical container. Host is optional
// (empty matches the name on any host); Since/Until, Levels, and Search are
// all optional filters.
type LogQuery struct {
	Host      string
	Container string // logical container name
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

// lineSpans reports the stored time range of every generation that has lines.
func (s *Store) lineSpans(ctx context.Context) (map[int64]lineSpan, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT container_ref, MIN(ts_ns), MAX(ts_ns) FROM log_lines GROUP BY container_ref")
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
		spans[ref] = lineSpan{
			oldest: time.Unix(0, oldest).UTC(),
			newest: time.Unix(0, newest).UTC(),
		}
	}
	return spans, rows.Err()
}

// generation is one stored container generation resolved for a query.
type generation struct {
	ref  int64
	id   string
	name string
}

// Query returns one page of stored lines for a logical container. Every
// generation of the name is read as one timeline, which is what makes history
// survive a container rebuild: the caller asks for "web" and gets the lines of
// every engine container that has ever been called "web" on that host.
//
// Rows are read unfiltered and only then grouped and filtered, which is the
// order the live path uses. Filtering rows in SQL first would delete the
// continuation lines of every multi-line entry — they classify as UNKNOWN — and
// a level-filtered stack trace would come back as its first line with no body.
func (s *Store) Query(ctx context.Context, q LogQuery) (LogPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	limit = min(limit, MaxQueryLimit)

	match, err := newMatcher(q)
	if err != nil {
		return LogPage{}, err
	}

	generations, err := s.generations(ctx, q.Host, q.Container)
	if err != nil {
		return LogPage{}, err
	}
	if len(generations) == 0 {
		return LogPage{Entries: []models.LogEntry{}}, nil
	}

	refs := make([]int64, len(generations))
	byRef := make(map[int64]generation, len(generations))
	for i, gen := range generations {
		refs[i] = gen.ref
		byRef[gen.ref] = gen
	}

	var (
		from   cursorPos
		hasPos bool
	)
	if q.Cursor != "" {
		tsNS, rowid, err := decodeCursor(q.Cursor)
		if err != nil {
			return LogPage{}, err
		}
		from, hasPos = cursorPos{tsNS: tsNS, rowid: rowid}, true
	}

	// One extra entry beyond the page is what proves an older page exists; an
	// unfiltered query needs exactly one extra row to find out.
	chunk := limit + 1
	if match.active() {
		chunk = max(chunk, scanChunk)
	}

	var window []storedRow // newest-first, accumulated across rounds
	for {
		rows, err := s.scanRows(ctx, refs, q, from, hasPos, chunk, byRef)
		if err != nil {
			return LogPage{}, err
		}
		window = append(window, rows...)

		entries, anchors := match.filter(groupRows(window))
		// Stop once the page is provably full, or once history runs out — never
		// hand back a page that is empty but still carries a cursor.
		if len(entries) > limit || len(rows) < chunk {
			return newPage(entries, anchors, limit), nil
		}
		from, hasPos = rows[len(rows)-1].pos, true
	}
}

// newPage keeps the newest limit entries of the scanned window. Pages walk
// backwards through history, so an entry older than the page is not truncated
// data: it is the proof that another page exists, and its position is the
// cursor the next page resumes from.
func newPage(entries []models.LogEntry, anchors []cursorPos, limit int) LogPage {
	if len(entries) <= limit {
		if entries == nil {
			entries = []models.LogEntry{}
		}
		return LogPage{Entries: entries}
	}
	cut := len(entries) - limit
	return LogPage{
		Entries:    entries[cut:],
		NextCursor: encodeCursor(anchors[cut].tsNS, anchors[cut].rowid),
	}
}

// cursorPos is the keyset position of one stored row.
type cursorPos struct {
	tsNS  int64
	rowid int64
}

// storedRow is one scanned row: the entry it parses to and where it sits.
type storedRow struct {
	entry models.LogEntry
	pos   cursorPos
}

// scanRows reads one chunk of rows, newest-first, strictly older than from.
func (s *Store) scanRows(ctx context.Context, refs []int64, q LogQuery, from cursorPos, hasPos bool, chunk int, byRef map[int64]generation) ([]storedRow, error) {
	statement, args := buildSelect(refs, q, from, hasPos, chunk)

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scanned := make([]storedRow, 0, chunk)
	for rows.Next() {
		var (
			rowid  int64
			ref    int64
			tsNS   int64
			stream int
			raw    string
		)
		if err := rows.Scan(&rowid, &ref, &tsNS, &stream, &raw); err != nil {
			return nil, err
		}
		scanned = append(scanned, storedRow{
			entry: entryFromRow(tsNS, stream, raw, byRef[ref]),
			pos:   cursorPos{tsNS: tsNS, rowid: rowid},
		})
	}
	return scanned, rows.Err()
}

// generations resolves a logical container name to its generation rows. An
// empty host matches the name on every host.
func (s *Store) generations(ctx context.Context, host, name string) ([]generation, error) {
	if name == "" {
		return nil, nil
	}

	statement := "SELECT id, container_id, name FROM containers WHERE name = ?"
	args := []any{name}
	if host != "" {
		statement += " AND host = ?"
		args = append(args, host)
	}

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var generations []generation
	for rows.Next() {
		var gen generation
		if err := rows.Scan(&gen.ref, &gen.id, &gen.name); err != nil {
			return nil, err
		}
		generations = append(generations, gen)
	}
	return generations, rows.Err()
}

// buildSelect renders the keyset page query: newest-first over every generation
// of the container, bounded by the time window and the cursor. Only filters that
// cannot delete part of a multi-line entry live in SQL — level and search are
// applied in Go, on grouped entries.
func buildSelect(refs []int64, q LogQuery, from cursorPos, hasPos bool, chunk int) (string, []any) {
	placeholders, args := refArgs(refs)
	where := []string{"container_ref IN (" + placeholders + ")"}

	if !q.Since.IsZero() {
		where = append(where, "ts_ns >= ?")
		args = append(args, q.Since.UnixNano())
	}
	if !q.Until.IsZero() {
		where = append(where, "ts_ns <= ?")
		args = append(args, q.Until.UnixNano())
	}
	if hasPos {
		where = append(where, "(ts_ns < ? OR (ts_ns = ? AND rowid < ?))")
		args = append(args, from.tsNS, from.tsNS, from.rowid)
	}
	args = append(args, chunk)

	statement := "SELECT rowid, container_ref, ts_ns, stream, raw FROM log_lines WHERE " +
		strings.Join(where, " AND ") +
		" ORDER BY ts_ns DESC, rowid DESC LIMIT ?"
	return statement, args
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

// filter keeps the matching entries and their anchors, in order.
func (m matcher) filter(entries []models.LogEntry, anchors []cursorPos) ([]models.LogEntry, []cursorPos) {
	if !m.active() {
		return entries, anchors
	}
	keptEntries := make([]models.LogEntry, 0, len(entries))
	keptAnchors := make([]cursorPos, 0, len(anchors))
	for i, entry := range entries {
		if m.matches(entry) {
			keptEntries = append(keptEntries, entry)
			keptAnchors = append(keptAnchors, anchors[i])
		}
	}
	return keptEntries, keptAnchors
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
	return entry
}

// groupRows folds continuation lines into their parent entry exactly like the
// live historical path, but only within a run of lines from the same
// generation, so a rebuild boundary can never merge two containers' lines.
//
// Rows arrive newest-first; entries come back oldest-first, each paired with the
// position of its *first* row. That is what the next page resumes from: a
// continuation line is always newer than its parent, so resuming at the parent
// keeps the entry whole rather than splitting its body across two pages.
func groupRows(rows []storedRow) ([]models.LogEntry, []cursorPos) {
	entries := make([]models.LogEntry, 0, len(rows))
	anchors := make([]cursorPos, 0, len(rows))

	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if last := len(entries) - 1; last >= 0 && entries[last].ContainerID == row.entry.ContainerID {
			// The live grouper decides; a folded pair comes back as one entry.
			if merged := models.GroupRelatedLogEntries([]models.LogEntry{entries[last], row.entry}); len(merged) == 1 {
				entries[last] = merged[0]
				continue
			}
		}
		entries = append(entries, row.entry)
		anchors = append(anchors, row.pos)
	}
	return entries, anchors
}

// encodeCursor renders the keyset position of the oldest entry on a page.
// (ts_ns, rowid) is unique and immutable, so pages stay stable while new lines
// are ingested.
func encodeCursor(tsNS, rowid int64) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d:%d", tsNS, rowid))
}

func decodeCursor(cursor string) (int64, int64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	tsPart, rowPart, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return 0, 0, ErrInvalidCursor
	}
	tsNS, err := strconv.ParseInt(tsPart, 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	rowid, err := strconv.ParseInt(rowPart, 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	return tsNS, rowid, nil
}
