package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func entry(i int, actor string, status int) Entry {
	return Entry{Time: t0.Add(time.Duration(i) * time.Minute), Actor: actor, Action: "container.stop", Target: "c" + string(rune('a'+i)), Status: status}
}

func TestStoreSurvivesRestartAndStaysBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	s := open(path, 4)
	for i := range 10 {
		s.Record(entry(i, "admin", 200))
	}
	s.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := bytes.Count(data, []byte("\n")); lines >= 6 {
		t.Fatalf("file has %d lines, want it compacted below 6", lines)
	}
	// A line cut short by a crash is skipped on load.
	if err := os.WriteFile(path, append(data, `{"time":"2026-`...), 0o600); err != nil {
		t.Fatal(err)
	}

	s = open(path, 4)
	defer s.Close()
	got := s.Entries()
	if len(got) != 4 || got[0].Target != entry(6, "", 0).Target || got[3].Target != entry(9, "", 0).Target {
		t.Fatalf("reloaded entries = %+v", got)
	}
}

func TestListFiltersAndPages(t *testing.T) {
	s := open(filepath.Join(t.TempDir(), "audit.jsonl"), 100)
	for i := range 6 {
		actor, status := "admin", 200
		if i%2 == 1 {
			actor, status = "token:ci", 500
		}
		s.Record(entry(i, actor, status))
	}
	s.Close()

	page, total := s.List(Filter{Actor: "TOKEN", Result: "failure", Limit: 2})
	if total != 3 || len(page) != 2 || page[0].Time != entry(5, "", 0).Time || page[1].Time != entry(3, "", 0).Time {
		t.Fatalf("page 1 = %+v, total %d", page, total)
	}
	page, _ = s.List(Filter{Actor: "token", Result: "failure", Offset: 2, Limit: 2})
	if len(page) != 1 || page[0].Time != entry(1, "", 0).Time {
		t.Fatalf("page 2 = %+v", page)
	}
	page, total = s.List(Filter{Result: "success", Since: entry(2, "", 0).Time, Until: entry(4, "", 0).Time, Limit: 10})
	if total != 2 || page[0].Time != entry(4, "", 0).Time {
		t.Fatalf("time range = %+v, total %d", page, total)
	}
}

func TestMergeIntoKeepsLiveEntries(t *testing.T) {
	dir := t.TempDir()
	restored, live := filepath.Join(dir, "restored.jsonl"), filepath.Join(dir, "live.jsonl")
	// The backup was taken after entry 2; the live log has since been
	// compacted down to entries 2 to 4.
	if err := os.WriteFile(restored, Encode([]Entry{entry(0, "a", 200), entry(1, "a", 200), entry(2, "a", 200)}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, Encode([]Entry{entry(2, "a", 200), entry(3, "a", 200), entry(4, "a", 200)}), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := MergeInto(restored, live); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(restored)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Decode(data)
	if len(got) != 5 {
		t.Fatalf("merged = %+v", got)
	}
	for i, e := range got {
		if e.Time != entry(i, "", 0).Time {
			t.Fatalf("merged = %+v", got)
		}
	}
	if err := Check(restored); err != nil {
		t.Fatal(err)
	}
}

// A line too long for any record must not cost the entries after it.
func TestDecodeKeepsEntriesAfterOversizedLine(t *testing.T) {
	huge := entry(1, "admin", 200)
	huge.Target = strings.Repeat("x", 2<<20)
	before, after := Encode([]Entry{entry(0, "admin", 200)}), Encode([]Entry{entry(2, "admin", 200)})

	for _, tc := range []struct {
		name        string
		line        []byte
		wantEntries int
	}{
		{"valid entry", Encode([]Entry{huge}), 3},
		{"not an entry", append(bytes.Repeat([]byte("x"), 2<<20), '\n'), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, lines := Decode(bytes.Join([][]byte{before, tc.line, after}, nil))
			if lines != 3 || len(got) != tc.wantEntries {
				t.Fatalf("got %d entries of %d lines, want %d of 3", len(got), lines, tc.wantEntries)
			}
			if got[len(got)-1].Target != entry(2, "", 0).Target {
				t.Fatalf("last entry = %+v", got[len(got)-1])
			}
		})
	}
}

// A file that does not end in a newline must not swallow the next record.
func TestRecordAfterUnterminatedLastLine(t *testing.T) {
	complete := bytes.TrimSuffix(Encode([]Entry{entry(0, "admin", 200)}), []byte("\n"))
	for _, tc := range []struct {
		name string
		tail []byte
		want []string
	}{
		{"line cut short by a crash", []byte(`{"time":"2026-`), []string{entry(1, "", 0).Target}},
		{"complete record", complete, []string{entry(0, "", 0).Target, entry(1, "", 0).Target}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "audit.jsonl")
			if err := os.WriteFile(path, tc.tail, 0o600); err != nil {
				t.Fatal(err)
			}
			s := open(path, 10)
			s.Record(entry(1, "admin", 200))
			s.Close()

			s = open(path, 10)
			defer s.Close()
			got := s.Entries()
			if len(got) != len(tc.want) {
				t.Fatalf("reloaded entries = %+v, want targets %v", got, tc.want)
			}
			for i, target := range tc.want {
				if got[i].Target != target {
					t.Errorf("entry %d target = %q, want %q", i, got[i].Target, target)
				}
			}
		})
	}
}

func TestRecordCapsTextFields(t *testing.T) {
	s := open(filepath.Join(t.TempDir(), "audit.jsonl"), 10)
	s.Record(Entry{
		Time:   t0,
		Actor:  "admin",
		Action: "compose." + strings.Repeat("a", 1000),
		Host:   strings.Repeat("h", 1000),
		// The cap falls inside a two-byte character, which must not be split.
		Target: "x" + strings.Repeat("é", 1000),
		Status: 400,
	})
	s.Close()

	got := s.Entries()[0]
	if got.Actor != "admin" {
		t.Errorf("actor = %q", got.Actor)
	}
	if want := "compose." + strings.Repeat("a", 248); got.Action != want {
		t.Errorf("action = %q, want %q", got.Action, want)
	}
	if want := strings.Repeat("h", 256); got.Host != want {
		t.Errorf("host = %q, want %q", got.Host, want)
	}
	if want := "x" + strings.Repeat("é", 127); got.Target != want {
		t.Errorf("target = %q, want %q", got.Target, want)
	}
}
