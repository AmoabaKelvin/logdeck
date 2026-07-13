package logstore

import (
	"context"
	"database/sql"
	"fmt"
)

// schemaVersion is the current schema generation, tracked in PRAGMA
// user_version. Bump it and add a migration step when the schema changes.
const schemaVersion = 1

// schemaV1 is the initial schema.
//
// Identity model: one containers row per *generation* (one engine container
// ID). The logical container a user sees is (host, name), so a rebuilt
// container — new engine ID, same name — adds a new generation row and its
// lines join the same logical timeline. Queries UNION every generation of a
// name ordered by ts_ns, which is what makes history survive a rebuild.
const schemaV1 = `
CREATE TABLE containers (
  id              INTEGER PRIMARY KEY,
  host            TEXT NOT NULL,
  container_id    TEXT NOT NULL,
  name            TEXT NOT NULL,
  compose_project TEXT NOT NULL DEFAULT '',
  image           TEXT NOT NULL DEFAULT '',
  first_seen_ms   INTEGER NOT NULL,
  last_seen_ms    INTEGER NOT NULL,
  removed_ms      INTEGER,
  stored_bytes    INTEGER NOT NULL DEFAULT 0,
  -- Per-stream backfill high-water marks. stdout and stderr are demuxed
  -- independently, so merged emission is not globally timestamp-ordered; the
  -- backfill watermark is the minimum of the marks that have advanced.
  stdout_wm_ns    INTEGER NOT NULL DEFAULT 0,
  stderr_wm_ns    INTEGER NOT NULL DEFAULT 0,
  -- Non-empty when the engine cannot read this container's logs (e.g. a
  -- logging driver that does not support reading). Backfill is not retried.
  excluded_reason TEXT NOT NULL DEFAULT '',
  UNIQUE(host, container_id)
);
CREATE INDEX containers_host_name ON containers(host, name);

CREATE TABLE log_lines (
  container_ref INTEGER NOT NULL,
  ts_ns         INTEGER NOT NULL,
  stream        INTEGER NOT NULL, -- 0 stdout, 1 stderr
  level         INTEGER NOT NULL, -- models.LevelSeverity value
  raw           TEXT NOT NULL
);
CREATE INDEX log_lines_container_ts ON log_lines(container_ref, ts_ns);
`

// initSchema creates the schema on a fresh database and is a no-op on an
// already-current one. Unknown (newer) versions are rejected rather than
// silently downgraded.
func initSchema(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	switch {
	case version == schemaVersion:
		return nil
	case version > schemaVersion:
		return fmt.Errorf("log store schema version %d is newer than supported version %d", version, schemaVersion)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if version == 0 {
		if _, err := tx.ExecContext(ctx, schemaV1); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}

	// PRAGMA does not accept bound parameters.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return tx.Commit()
}
