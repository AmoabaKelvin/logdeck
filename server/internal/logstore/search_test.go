package logstore

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func writeProjectEntries(t *testing.T, s *Store, key genKey, name, project string, entries ...models.LogEntry) {
	t.Helper()
	batch := make([]ingestMsg, 0, len(entries))
	for _, entry := range entries {
		batch = append(batch, ingestMsg{kind: msgLine, key: key, name: name, project: project, line: lineFromEntry(entry)})
	}
	if err := s.commit(batch, mustWriterState(t, s)); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// searchPages follows cursors to the end and returns every page's entries in
// ascending order, as a client stitching the pages together would.
func searchPages(t *testing.T, s *Store, q LogQuery, budget int) []models.LogEntry {
	t.Helper()
	var pages [][]models.LogEntry
	for range 10_000 {
		page, err := s.readPage(context.Background(), q, budget)
		if err != nil {
			t.Fatalf("readPage: %v", err)
		}
		pages = append(pages, page.Entries)
		if page.NextCursor == "" {
			var all []models.LogEntry
			for i := len(pages) - 1; i >= 0; i-- {
				all = append(all, pages[i]...)
			}
			return all
		}
		q.Cursor = page.NextCursor
	}
	t.Fatal("paging never reached the end of history")
	return nil
}

func TestSearchMergesContainersKeepingEntriesWhole(t *testing.T) {
	s := newTestStore(t)
	at := func(i int) time.Time { return baseTime.Add(time.Duration(i) * time.Second) }

	writeProjectEntries(t, s, genKey{"local", "w1"}, "web", "shop",
		entryAt(at(0), "stdout", "level=info starting"),
		entryAt(at(2), "stderr", "level=error unhandled exception"),
		entryAt(at(4), "stderr", "at com.example.Service.handle(Service.java:42)"),
		entryAt(at(6), "stderr", "at com.example.Server.dispatch(Server.java:17)"),
		entryAt(at(8), "stdout", "level=info recovered"),
	)
	writeProjectEntries(t, s, genKey{"local", "d1"}, "db", "shop",
		entryAt(at(1), "stdout", "level=info ready"),
		entryAt(at(3), "stdout", "level=error connection refused"),
		entryAt(at(5), "stdout", "level=info checkpoint"),
	)
	writeProjectEntries(t, s, genKey{"remote", "c1"}, "cache", "other",
		entryAt(at(7), "stdout", "level=info warm"),
	)

	all, err := s.Query(context.Background(), LogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"level=info starting",
		"level=info ready",
		"level=error unhandled exception\nat com.example.Service.handle(Service.java:42)\nat com.example.Server.dispatch(Server.java:17)",
		"level=error connection refused",
		"level=info checkpoint",
		"level=info warm",
		"level=info recovered",
	}
	if got := messages(all.Entries); !slices.Equal(got, want) {
		t.Fatalf("merged timeline:\n got %q\nwant %q", got, want)
	}
	if e := all.Entries[5]; e.ContainerName != "cache" || e.Host != "remote" {
		t.Fatalf("entry is not attributed to its container: %+v", e)
	}

	for _, tt := range []struct {
		name string
		q    LogQuery
		want []string
	}{
		{"levels", LogQuery{Levels: []string{"ERROR"}}, []string{want[2], want[3]}},
		{"search", LogQuery{Search: "REFUSED"}, []string{want[3]}},
		{"project", LogQuery{Project: "other"}, []string{want[5]}},
		{"host", LogQuery{Host: "local", Search: "info"}, []string{want[0], want[1], want[4], want[6]}},
		{"window", LogQuery{Since: at(7), Until: at(8)}, []string{want[5], want[6]}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page, err := s.Query(context.Background(), tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := messages(page.Entries); !slices.Equal(got, tt.want) {
				t.Fatalf("got %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Multi-line entries from different containers interleave, so page boundaries
// keep landing inside one container's entry while another's lines sit between
// its rows. Every page size and scan budget must stitch back to the same
// timeline, with each entry whole and none repeated or lost.
func TestSearchPagingIsExactAcrossInterleavedEntries(t *testing.T) {
	s := newTestStore(t)
	for c := range 3 {
		var entries []models.LogEntry
		for i := range 90 {
			ts := baseTime.Add(time.Duration(i*3+c) * time.Millisecond)
			message := fmt.Sprintf("level=error failure %d in svc%d", i, c)
			if i%3 != 0 {
				message = fmt.Sprintf("    at frame %d", i)
			}
			entries = append(entries, entryAt(ts, "stderr", message))
		}
		writeProjectEntries(t, s, genKey{"local", fmt.Sprint("id", c)}, fmt.Sprint("svc", c), "", entries...)
	}
	full, err := s.Query(context.Background(), LogQuery{Limit: MaxQueryLimit})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Entries) != 90 || full.Entries[0].ContinuationCount != 2 {
		t.Fatalf("expected 90 three-line entries, got %d", len(full.Entries))
	}
	want := messages(full.Entries)

	for _, limit := range []int{1, 2, 5, 7} {
		for _, budget := range []int{scanBudget, 3, 10} {
			for _, search := range []string{"", "svc1"} {
				got := searchPages(t, s, LogQuery{Limit: limit, Search: search}, budget)
				expected := want
				if search != "" {
					expected = slices.DeleteFunc(slices.Clone(want), func(m string) bool {
						return !containsFold(m, search)
					})
				}
				if !slices.Equal(messages(got), expected) {
					t.Fatalf("limit %d budget %d search %q: stitched pages differ from one full page:\n got %q\nwant %q",
						limit, budget, search, messages(got), expected)
				}
			}
		}
	}
}

func containsFold(s, substr string) bool {
	m, _ := newMatcher(LogQuery{Search: substr})
	return m.matches(models.LogEntry{Message: s})
}

func TestSearchStopsAtItsScanBudget(t *testing.T) {
	s := newTestStore(t)
	var entries []models.LogEntry
	for i := range 2500 {
		entries = append(entries, entryAt(baseTime.Add(time.Duration(i)*time.Second), "stdout", fmt.Sprint("line ", i)))
	}
	writeEntries(t, s, genKey{"local", "a"}, "a", entries...)
	q := LogQuery{Search: "line 0", Limit: 5}

	page, err := s.readPage(context.Background(), q, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 || page.NextCursor == "" || page.ScannedTo.IsZero() {
		t.Fatalf("expected an empty page that reports how far it searched, got %d entries, cursor %q, scannedTo %v",
			len(page.Entries), page.NextCursor, page.ScannedTo)
	}
	if want := baseTime.Add(1500 * time.Second); !page.ScannedTo.Equal(want) {
		t.Fatalf("scannedTo %v, want %v: one chunk back from the newest line", page.ScannedTo, want)
	}
	if got := messages(searchPages(t, s, q, 10)); !slices.Equal(got, []string{"line 0"}) {
		t.Fatalf("following the cursor found %q", got)
	}
}

// The same property over sealed blocks, entries of random length, and
// timestamps shared across containers, checked against a reference built by
// grouping each container's lines on their own and merging by position.
func TestSearchPagingMatchesReferenceOverSealedBlocks(t *testing.T) {
	s := newTestStore(t)
	state := mustWriterState(t, s)
	rng := rand.New(rand.NewPCG(1, 2))

	type ref struct {
		entry     models.LogEntry
		ts        time.Time
		container int
		line      int
	}
	var want []ref
	for c := range 4 {
		var entries []models.LogEntry
		ts := baseTime
		for i := range 2600 {
			ts = ts.Add(time.Duration(rng.IntN(3)) * time.Millisecond)
			message := fmt.Sprintf("level=error failure %d in svc%d", i, c)
			if i > 0 && rng.IntN(10) < 4 {
				message = fmt.Sprintf("    at frame %d", i)
			}
			entries = append(entries, entryAt(ts, "stderr", message))
		}
		batch := make([]ingestMsg, 0, len(entries))
		for _, entry := range entries {
			batch = append(batch, ingestMsg{kind: msgLine, key: genKey{"local", fmt.Sprint("id", c)},
				name: fmt.Sprint("svc", c), line: lineFromEntry(entry)})
		}
		if err := s.commit(batch, state); err != nil {
			t.Fatal(err)
		}
		if err := s.sealFullBlocks(state); err != nil {
			t.Fatal(err)
		}

		line := 0
		for _, entry := range models.GroupRelatedLogEntries(entries) {
			want = append(want, ref{entry: entry, ts: entry.Timestamp, container: c, line: line})
			line += entry.ContinuationCount + 1
		}
	}
	// Containers were committed in order, so equal timestamps are ordered by
	// container, then by line.
	slices.SortFunc(want, func(a, b ref) int {
		if c := a.ts.Compare(b.ts); c != 0 {
			return c
		}
		if a.container != b.container {
			return a.container - b.container
		}
		return a.line - b.line
	})
	var wantMessages []string
	for _, r := range want {
		wantMessages = append(wantMessages, r.entry.Message)
	}

	for _, tt := range []struct{ limit, budget int }{{500, scanBudget}, {7, scanBudget}, {13, 5}, {1000, 1}} {
		got := messages(searchPages(t, s, LogQuery{Limit: tt.limit}, tt.budget))
		if !slices.Equal(got, wantMessages) {
			i := 0
			for i < min(len(got), len(wantMessages)) && got[i] == wantMessages[i] {
				i++
			}
			t.Fatalf("limit %d budget %d: %d entries, want %d; first difference at %d", tt.limit, tt.budget, len(got), len(wantMessages), i)
		}
	}
}
