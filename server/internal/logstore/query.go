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
	// maxRegexScan bounds how many rows one regex query reads before it
	// returns a cursor and lets the caller continue.
	maxRegexScan = 50000
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
func (s *Store) Query(ctx context.Context, q LogQuery) (LogPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	limit = min(limit, MaxQueryLimit)

	var search *regexp.Regexp
	if q.Regex && q.Search != "" {
		compiled, err := regexp.Compile(q.Search)
		if err != nil {
			return LogPage{}, fmt.Errorf("invalid search pattern: %w", err)
		}
		search = compiled
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

	statement, args, err := buildSelect(refs, q, search != nil, limit)
	if err != nil {
		return LogPage{}, err
	}

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return LogPage{}, err
	}
	defer rows.Close()

	var (
		entries    []models.LogEntry
		scanned    int
		lastTS     int64
		lastRowID  int64
		matchedTS  int64
		matchedRow int64
		hitLimit   bool
	)
	for rows.Next() {
		var (
			rowid  int64
			ref    int64
			tsNS   int64
			stream int
			level  int
			raw    string
		)
		if err := rows.Scan(&rowid, &ref, &tsNS, &stream, &level, &raw); err != nil {
			return LogPage{}, err
		}
		scanned++
		lastTS, lastRowID = tsNS, rowid

		if search != nil && !search.MatchString(raw) {
			continue
		}

		gen := byRef[ref]
		entries = append(entries, entryFromRow(tsNS, stream, raw, gen))
		matchedTS, matchedRow = tsNS, rowid
		if len(entries) >= limit {
			hitLimit = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return LogPage{}, err
	}

	// Rows arrive newest-first (backward page traversal); a page reads
	// oldest-first, like the live view.
	slices.Reverse(entries)

	page := LogPage{Entries: groupByContainer(entries)}
	switch {
	case hitLimit:
		page.NextCursor = encodeCursor(matchedTS, matchedRow)
	case search != nil && scanned >= maxRegexScan:
		// The scan budget ran out before the page filled; hand back a cursor
		// so the caller can keep walking instead of silently truncating.
		page.NextCursor = encodeCursor(lastTS, lastRowID)
	}
	return page, nil
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

// buildSelect renders the keyset page query: newest-first over every
// generation of the container, with the time, level, and substring filters
// pushed into SQL. A regex search is applied in Go over a bounded scan.
func buildSelect(refs []int64, q LogQuery, regexSearch bool, limit int) (string, []any, error) {
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
	if severities := levelSeverities(q.Levels); len(severities) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?,", len(severities)), ",")
		where = append(where, "level IN ("+marks+")")
		for _, severity := range severities {
			args = append(args, severity)
		}
	}
	if q.Search != "" && !regexSearch {
		where = append(where, "instr(raw, ?) > 0")
		args = append(args, q.Search)
	}
	if q.Cursor != "" {
		tsNS, rowid, err := decodeCursor(q.Cursor)
		if err != nil {
			return "", nil, err
		}
		where = append(where, "(ts_ns < ? OR (ts_ns = ? AND rowid < ?))")
		args = append(args, tsNS, tsNS, rowid)
	}

	scanLimit := limit
	if regexSearch {
		scanLimit = maxRegexScan
	}
	args = append(args, scanLimit)

	statement := "SELECT rowid, container_ref, ts_ns, stream, level, raw FROM log_lines WHERE " +
		strings.Join(where, " AND ") +
		" ORDER BY ts_ns DESC, rowid DESC LIMIT ?"
	return statement, args, nil
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

// groupByContainer folds continuation lines into their parent entry exactly
// like the live historical path, but only within a run of lines from the same
// generation, so a rebuild boundary can never merge two containers' lines.
func groupByContainer(entries []models.LogEntry) []models.LogEntry {
	grouped := make([]models.LogEntry, 0, len(entries))
	for start := 0; start < len(entries); {
		end := start + 1
		for end < len(entries) && entries[end].ContainerID == entries[start].ContainerID {
			end++
		}
		grouped = append(grouped, models.GroupRelatedLogEntries(entries[start:end])...)
		start = end
	}
	return grouped
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
