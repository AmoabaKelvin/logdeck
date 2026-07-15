package logstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	bytesPerMB = 1024 * 1024
	// deleteChunk bounds one eviction transaction so a sweep never holds the
	// write lock long enough to stall ingestion.
	deleteChunk = 5000
	// maxTrimRounds bounds one global sweep; the next pass continues if the
	// store is still over cap.
	maxTrimRounds = 1000
)

// logicalGroup is one logical container — every generation of a (host, name)
// pair — which is the unit retention operates on. Trimming per generation
// would let a rebuilt container hold N times the cap.
type logicalGroup struct {
	host  string
	name  string
	refs  []int64
	bytes int64
}

// janitorLoop schedules retention periodically. It performs no database writes
// itself: SQLite is single-writer, and a separate eviction transaction competes
// with the writeLoop for the one write lock and dies with SQLITE_BUSY under a
// firehose. So the janitor only signals the writer, which runs retention inline
// between its batches (see writeLoop). The signal is coalescing: a sweep already
// pending is enough. The database file is never VACUUMed: SQLite reuses freed
// pages, so the file plateaus at its high-water mark instead of shrinking, which
// avoids a long exclusive lock.
func (s *Store) janitorLoop(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case s.retainCh <- struct{}{}:
			default: // a sweep is already pending
			}
		}
	}
}

// retain evicts oldest-first until every logical container is under the
// per-container cap and the store as a whole is under the total cap.
func (s *Store) retain(ctx context.Context) error {
	limits := s.limits()
	perCap := int64(limits.PerContainerMB) * bytesPerMB
	totalCap := int64(limits.TotalMB) * bytesPerMB

	groups, err := s.loadGroups(ctx)
	if err != nil {
		return err
	}

	total := int64(0)
	for i := range groups {
		for groups[i].bytes > perCap {
			evicted, err := s.evictOldest(ctx, groups[i].refs, groups[i].bytes-perCap)
			if err != nil {
				return err
			}
			if evicted == 0 {
				break // nothing left to evict (stored_bytes drifted)
			}
			groups[i].bytes -= evicted
		}
		total += groups[i].bytes
	}

	for round := 0; total > totalCap && round < maxTrimRounds; round++ {
		oldest, err := s.oldestGroup(ctx, groups)
		if err != nil {
			return err
		}
		if oldest < 0 {
			break
		}
		evicted, err := s.evictOldest(ctx, groups[oldest].refs, total-totalCap)
		if err != nil {
			return err
		}
		if evicted == 0 {
			groups[oldest].bytes = 0 // exhausted; stop reconsidering it
			continue
		}
		groups[oldest].bytes -= evicted
		total -= evicted
	}

	return s.pruneEmptyGenerations(ctx)
}

// loadGroups collapses the generation rows into logical containers.
func (s *Store) loadGroups(ctx context.Context) ([]logicalGroup, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, host, name, stored_bytes FROM containers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	index := make(map[genKey]int) // reuse the (host, id-as-name) pair shape
	var groups []logicalGroup
	for rows.Next() {
		var (
			ref   int64
			host  string
			name  string
			bytes int64
		)
		if err := rows.Scan(&ref, &host, &name, &bytes); err != nil {
			return nil, err
		}
		key := genKey{host: host, id: name}
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, logicalGroup{host: host, name: name})
		}
		groups[i].refs = append(groups[i].refs, ref)
		groups[i].bytes += bytes
	}
	return groups, rows.Err()
}

// oldestGroup returns the index of the non-empty group holding the oldest
// stored line, or -1 when nothing is left to evict.
func (s *Store) oldestGroup(ctx context.Context, groups []logicalGroup) (int, error) {
	best := -1
	var bestTS int64
	for i := range groups {
		if groups[i].bytes <= 0 {
			continue
		}
		ts, ok, err := s.oldestLine(ctx, groups[i].refs)
		if err != nil {
			return -1, err
		}
		if !ok {
			continue
		}
		if best < 0 || ts < bestTS {
			best, bestTS = i, ts
		}
	}
	return best, nil
}

func (s *Store) oldestLine(ctx context.Context, refs []int64) (int64, bool, error) {
	placeholders, args := refArgs(refs)
	var ts sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		"SELECT MIN(ts_ns) FROM log_lines WHERE container_ref IN ("+placeholders+")", args...).Scan(&ts)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	return ts.Int64, ts.Valid, nil
}

// evictOldest deletes the oldest lines of one logical container until it has
// freed excess bytes, reading at most deleteChunk rows so one transaction can
// never hold the write lock long enough to stall ingestion (the caller loops
// if more is needed). It returns how many bytes it freed. The row deletion and
// the stored_bytes adjustment share a transaction, and stored_bytes is updated
// relative to its current value so it composes with concurrent ingestion.
//
// The transaction takes the write lock up front (_txlock=immediate on the DSN):
// as a deferred read-then-write transaction it would fail with
// SQLITE_BUSY_SNAPSHOT whenever a line was ingested between its SELECT and its
// DELETE, so retention would stop working exactly when the store is busiest.
func (s *Store) evictOldest(ctx context.Context, refs []int64, excess int64) (int64, error) {
	placeholders, args := refArgs(refs)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		"SELECT rowid, container_ref, length(CAST(raw AS BLOB)) FROM log_lines"+
			" WHERE container_ref IN ("+placeholders+") ORDER BY ts_ns, rowid LIMIT ?",
		append(args, deleteChunk)...)
	if err != nil {
		return 0, err
	}

	var (
		rowids []any
		freed  int64
		perRef = make(map[int64]int64)
	)
	for rows.Next() {
		var rowid, ref, size int64
		if err := rows.Scan(&rowid, &ref, &size); err != nil {
			rows.Close()
			return 0, err
		}
		rowids = append(rowids, rowid)
		perRef[ref] += size
		freed += size
		if freed >= excess {
			break // under the cap: evicting further would throw away history
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(rowids) == 0 {
		return 0, nil
	}

	rowPlaceholders := strings.TrimSuffix(strings.Repeat("?,", len(rowids)), ",")
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM log_lines WHERE rowid IN ("+rowPlaceholders+")", rowids...); err != nil {
		return 0, err
	}
	for ref, size := range perRef {
		if _, err := tx.ExecContext(ctx,
			"UPDATE containers SET stored_bytes = max(0, stored_bytes - ?) WHERE id = ?", size, ref); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if freed > 0 {
		// Signals the writer to fold the WAL: these deletes are the store's
		// heaviest WAL churn, so the file only stays near its cap if it is
		// truncated after them.
		s.evictions.Add(1)
	}
	return freed, nil
}

// pruneEmptyGenerations drops generation rows whose container is gone from the
// engine and whose lines have all been evicted. A removed generation that
// still has lines is kept — its history is part of the logical container's
// timeline.
func (s *Store) pruneEmptyGenerations(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM containers
		WHERE removed_ms IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM log_lines WHERE container_ref = containers.id)`)
	return err
}

// refArgs renders a container_ref IN (...) clause.
func refArgs(refs []int64) (string, []any) {
	args := make([]any, 0, len(refs)+1)
	for _, ref := range refs {
		args = append(args, ref)
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(refs)), ","), args
}
