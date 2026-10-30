package logstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotCopiesAnOpenStore(t *testing.T) {
	s := newTestStore(t)
	key := genKey{host: "local", id: "c1"}
	base := time.Now().Add(-time.Hour)
	writeEntries(t, s, key, "web", entryAt(base, "stdout", "one"), entryAt(base.Add(time.Second), "stderr", "two"))

	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := s.Snapshot(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if err := CheckSnapshot(path); err != nil {
		t.Fatal(err)
	}
	copied, err := Open(path, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	if got, want := countLines(t, copied), countLines(t, s); got != want || got == 0 {
		t.Fatalf("snapshot has %d lines, store has %d", got, want)
	}
}

func TestCheckSnapshotRejectsNewerSchemas(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := s.Snapshot(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := CheckSnapshot(path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("CheckSnapshot = %v, want a newer-schema error", err)
	}
}
