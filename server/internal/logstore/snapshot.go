package logstore

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// Snapshot writes a consistent copy of the database to path, which must not
// exist yet. VACUUM INTO reads inside a single read transaction, which WAL
// mode runs beside the writer, so ingestion carries on while it copies. The
// writer skips its WAL truncation in the meantime: a TRUNCATE checkpoint waits
// for every reader to finish, and would stall ingestion behind the copy. The
// next retention pass after the snapshot truncates the WAL it let grow.
func (s *Store) Snapshot(ctx context.Context, path string) error {
	s.snapshots.Add(1)
	defer func() {
		s.walGrew.Store(true)
		s.snapshots.Add(-1)
	}()
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("snapshot log store: %w", err)
	}
	return nil
}

// CheckSnapshot reports whether the file at path is a log database this
// version can open: a SQLite file whose schema is not newer than ours. It
// reads only the header, so checking a large snapshot is instant.
func CheckSnapshot(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 100)
	if _, err := io.ReadFull(f, header); err != nil || string(header[:16]) != "SQLite format 3\x00" {
		return errors.New("stored logs are not a SQLite database")
	}
	// PRAGMA user_version lives at offset 60 of the header.
	version := int(binary.BigEndian.Uint32(header[60:64]))
	if version > schemaVersion {
		return fmt.Errorf("stored logs use schema version %d, newer than this LogDeck supports (%d)", version, schemaVersion)
	}
	return nil
}
