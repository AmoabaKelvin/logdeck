package logstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// linesByName reports how many stored lines each logical container owns,
// resolved through the foreign key. It is the check that catches a line filed
// against the wrong generation: a misattributed line shows up under whichever
// container actually owns that id.
func linesByName(t *testing.T, s *Store) map[string]int {
	t.Helper()
	rows, err := s.db.Query(`
		SELECT c.host || '/' || c.name, COUNT(*)
		FROM log_lines l JOIN containers c ON c.id = l.container_ref
		GROUP BY 1`)
	if err != nil {
		t.Fatalf("count lines: %v", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			name  string
			count int
		)
		if err := rows.Scan(&name, &count); err != nil {
			t.Fatalf("scan count: %v", err)
		}
		counts[name] = count
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("count lines: %v", err)
	}

	// A line whose container_ref resolves to nothing would be invisible to the
	// join above, so it is counted separately rather than silently ignored.
	var orphans int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM log_lines
		WHERE container_ref NOT IN (SELECT id FROM containers)`).Scan(&orphans); err != nil {
		t.Fatalf("count orphans: %v", err)
	}
	if orphans > 0 {
		t.Fatalf("%d stored lines reference a generation that does not exist", orphans)
	}
	return counts
}

func storedMessages(t *testing.T, s *Store, host, name string) []string {
	t.Helper()
	page, err := s.Query(context.Background(), LogQuery{Host: host, Container: name})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return messages(page.Entries)
}

// TestDeleteContainerRemovesEveryGeneration purges a container that has been
// rebuilt (two generations) and leaves every other logical container — including
// the same name on another host — untouched.
func TestDeleteContainerRemovesEveryGeneration(t *testing.T) {
	store := newTestStore(t)

	// web on local, rebuilt: two generations, one timeline.
	writeEntries(t, store, genKey{"local", "aaa"}, "web",
		entryAt(baseTime, "stdout", "gen one"),
		entryAt(baseTime.Add(time.Second), "stdout", "gen one again"))
	writeEntries(t, store, genKey{"local", "bbb"}, "web",
		entryAt(baseTime.Add(2*time.Second), "stdout", "gen two"))
	// The same name on another host is a different logical container.
	writeEntries(t, store, genKey{"remote", "ccc"}, "web",
		entryAt(baseTime, "stdout", "other host"))
	// An unrelated container on the purged host.
	writeEntries(t, store, genKey{"local", "ddd"}, "db",
		entryAt(baseTime, "stdout", "db line"))

	deleted, err := store.DeleteContainer(context.Background(), "local", "web")
	if err != nil {
		t.Fatalf("DeleteContainer: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted %d lines, want 3 (both generations)", deleted)
	}

	if got := storedMessages(t, store, "local", "web"); len(got) != 0 {
		t.Fatalf("local/web still has %v, want no stored logs", got)
	}

	// Both generation rows are gone, not just their lines.
	var generations int
	if err := store.db.QueryRow(
		"SELECT COUNT(*) FROM containers WHERE host = 'local' AND name = 'web'").Scan(&generations); err != nil {
		t.Fatalf("count generations: %v", err)
	}
	if generations != 0 {
		t.Fatalf("%d generation rows survived the purge, want 0", generations)
	}

	// Everything else is intact.
	if got := storedMessages(t, store, "remote", "web"); len(got) != 1 || got[0] != "other host" {
		t.Fatalf("remote/web = %v, want [other host]", got)
	}
	if got := storedMessages(t, store, "local", "db"); len(got) != 1 || got[0] != "db line" {
		t.Fatalf("local/db = %v, want [db line]", got)
	}

	counts := linesByName(t, store)
	if counts["local/web"] != 0 || counts["remote/web"] != 1 || counts["local/db"] != 1 {
		t.Fatalf("stored line counts = %v, want remote/web and local/db to hold one line each", counts)
	}
}

func TestDeleteContainerUnknown(t *testing.T) {
	store := newTestStore(t)
	writeEntries(t, store, genKey{"local", "aaa"}, "web", entryAt(baseTime, "stdout", "hello"))

	for _, tt := range []struct{ host, name string }{
		{"local", "nosuch"}, // unknown name on a known host
		{"nosuch", "web"},   // known name on an unknown host
	} {
		deleted, err := store.DeleteContainer(context.Background(), tt.host, tt.name)
		if !errors.Is(err, ErrContainerNotFound) {
			t.Fatalf("DeleteContainer(%q, %q) error = %v, want ErrContainerNotFound", tt.host, tt.name, err)
		}
		if deleted != 0 {
			t.Fatalf("DeleteContainer(%q, %q) deleted %d lines, want 0", tt.host, tt.name, deleted)
		}
	}

	// The store still holds the container the failed calls did not name.
	if got := storedMessages(t, store, "local", "web"); len(got) != 1 {
		t.Fatalf("local/web = %v, want the untouched line", got)
	}
}

// TestDeleteContainerInvalidatesWriterCache purges a container that is live and
// being ingested. The writer caches genKey -> containers.id, and the purge
// removes exactly those ids — ids SQLite is then free to hand to a different
// container. A stale cached ref would either break the foreign key or file the
// live container's next lines under whichever container inherited the id.
//
// refs is shared across commits here because that is what writeLoop does: it
// carries one ref cache across every batch for the lifetime of the store.
func TestDeleteContainerInvalidatesWriterCache(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	refs := map[genKey]int64{}

	web := genKey{"local", "aaa"}
	commit := func(key genKey, name string, entries ...models.LogEntry) {
		t.Helper()
		batch := make([]ingestMsg, 0, len(entries))
		for _, entry := range entries {
			batch = append(batch, ingestMsg{kind: msgLine, key: key, name: name, line: lineFromEntry(entry)})
		}
		if err := store.commit(batch, refs); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	commit(web, "web", entryAt(baseTime, "stdout", "before purge"))
	before := refs[web]
	if before == 0 {
		t.Fatal("the writer did not cache the live generation, so this test proves nothing")
	}

	if _, err := store.DeleteContainer(ctx, "local", "web"); err != nil {
		t.Fatalf("DeleteContainer: %v", err)
	}

	// A different container starts up right after the purge and may be handed
	// the rowid the purge freed.
	commit(genKey{"local", "zzz"}, "db", entryAt(baseTime.Add(time.Second), "stdout", "db line"))

	// The live container keeps producing: its lines must land under a fresh
	// generation of *web*, never under db.
	commit(web, "web", entryAt(baseTime.Add(2*time.Second), "stdout", "after purge"))

	if got := refs[web]; got == before {
		t.Fatalf("the writer still caches the purged generation id %d", got)
	}

	if got := storedMessages(t, store, "local", "web"); len(got) != 1 || got[0] != "after purge" {
		t.Fatalf("local/web = %v, want only the line written after the purge", got)
	}
	if got := storedMessages(t, store, "local", "db"); len(got) != 1 || got[0] != "db line" {
		t.Fatalf("local/db = %v, want only its own line", got)
	}

	// No line was misattributed and none is dangling (linesByName fails on both).
	counts := linesByName(t, store)
	if counts["local/web"] != 1 || counts["local/db"] != 1 {
		t.Fatalf("stored line counts = %v, want one line under each container", counts)
	}
}

func TestDBSize(t *testing.T) {
	store := newTestStore(t)
	writeEntries(t, store, genKey{"local", "aaa"}, "web", entryAt(baseTime, "stdout", "hello"))

	size, err := store.DBSize()
	if err != nil {
		t.Fatalf("DBSize: %v", err)
	}
	if size <= 0 {
		t.Fatalf("DBSize = %d, want the size of the database on disk", size)
	}
}
