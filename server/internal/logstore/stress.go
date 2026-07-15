package logstore

import "context"

// This file exposes a tiny, read-only measurement surface consumed ONLY by the
// out-of-tree stress harness (server/cmd/logstore-stress). It adds no behavior
// to the store and changes no write path: it just reads counters the pipeline
// already maintains. It exists because those counters (s.drops) and the row
// table (s.db) are unexported, and the harness must observe the REAL store, not
// a copy. Do not use these methods in production code.

// Drops reports how many live records the ingest buffer has discarded because
// the writer fell behind — the store's backpressure signal. Stress/measurement
// use only.
func (s *Store) Drops() uint64 {
	return s.drops.Load()
}

// CountLines reports how many log lines are currently committed to the
// database. Stress/measurement use only.
func (s *Store) CountLines(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM log_lines").Scan(&n)
	return n, err
}
