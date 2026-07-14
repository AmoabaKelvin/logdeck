package logstore

import (
	"context"
	"errors"
	"os"
)

// ErrContainerNotFound is returned when the store holds no generation of the
// requested logical container, so the caller can answer 404.
var ErrContainerNotFound = errors.New("container not found")

// DeleteContainer removes every generation of the logical container (host, name)
// and all of its stored lines. Returns the number of lines deleted.
//
// host is required: the same name can exist on several hosts and each one owns
// its own history. The database file is not VACUUMed — like the janitor's
// eviction, it plateaus at its high-water mark and SQLite reuses the freed
// pages for subsequent writes.
//
// Purging a container that is currently being ingested is allowed: the writer
// simply files its next line under a fresh generation. See invalidate for how
// the writer's ref cache is kept from pointing at the ids this removes.
func (s *Store) DeleteContainer(ctx context.Context, host, name string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		"SELECT id, container_id FROM containers WHERE host = ? AND name = ?", host, name)
	if err != nil {
		return 0, err
	}
	var (
		refs []int64
		keys []genKey
	)
	for rows.Next() {
		var (
			ref int64
			id  string
		)
		if err := rows.Scan(&ref, &id); err != nil {
			rows.Close()
			return 0, err
		}
		refs = append(refs, ref)
		keys = append(keys, genKey{host: host, id: id})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(refs) == 0 {
		return 0, ErrContainerNotFound
	}

	// Lines first: log_lines.container_ref references containers(id).
	placeholders, args := refArgs(refs)
	result, err := tx.ExecContext(ctx,
		"DELETE FROM log_lines WHERE container_ref IN ("+placeholders+")", args...)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM containers WHERE id IN ("+placeholders+")", args...); err != nil {
		return 0, err
	}

	// Published before COMMIT, while this transaction still holds the write lock
	// (_txlock=immediate). The writer drains the set immediately after its own
	// BEGIN, so any deletion that committed before the writer's transaction
	// started is already visible to it: the writer can never insert against an
	// id this deletion removed — an id SQLite is free to hand to a *different*
	// container's next generation.
	s.invalidate(keys)

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}

// invalidate publishes purged generations to the writer and forgets their
// backfill state, so a gap or resume point recorded before the purge cannot
// re-read the deleted window back out of the engine.
//
// Over-invalidating is harmless — the writer just re-resolves the generation
// with an upsert — so a deletion that ends up rolled back needs no undo.
func (s *Store) invalidate(keys []genKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		s.invalidated[key] = struct{}{}
		delete(s.resume, key)
		delete(s.gaps, key)
	}
}

// dropInvalidated evicts purged generations from the writer's ref cache. The
// writer calls it inside its write transaction, so the keys it drops are
// exactly those of the deletions that already committed.
func (s *Store) dropInvalidated(refs map[genKey]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.invalidated {
		delete(refs, key)
		delete(s.invalidated, key)
	}
}

// DBSize reports the database's size on disk: the main file plus its write-ahead
// log. The -wal file holds committed pages that have not been checkpointed back
// into the main file yet, so ignoring it would under-report a busy store by the
// size of its recent writes. The -shm file is scratch coordination state rather
// than stored data, so it is excluded.
func (s *Store) DBSize() (int64, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return 0, err
	}
	total := info.Size()
	if wal, err := os.Stat(s.path + "-wal"); err == nil {
		total += wal.Size()
	}
	return total, nil
}
