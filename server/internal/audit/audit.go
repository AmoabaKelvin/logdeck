// Package audit keeps a record of the changes made through LogDeck: who made
// each one, what, on which host and target, when, and whether it worked.
//
// Records are kept in memory for reads and appended to a JSON-lines file next
// to the config file by a single writer goroutine, so a request never waits on
// the disk. The file is rewritten only when it has grown well past the cap,
// keeping the newest entries.
package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// MaxEntries is how many records are kept; older ones are dropped.
	MaxEntries = 10000
	// queueSize bounds the records waiting for the writer. Changes are rare
	// next to it, so a full queue means the disk has stalled.
	queueSize = 256
)

// Entry is one recorded change. It holds no request bodies, so env values,
// passwords and tokens never land in it.
type Entry struct {
	Time time.Time `json:"time"`
	// Actor is the username of a session, "token:<name>" for an API token,
	// or empty when authentication is disabled.
	Actor string `json:"actor"`
	// Role is the session's role or the API token's scope.
	Role   string `json:"role,omitempty"`
	Action string `json:"action"`
	Method string `json:"method"`
	// Route is the matched route pattern, e.g. /api/v1/containers/{id}/stop.
	Route string `json:"route"`
	Host  string `json:"host,omitempty"`
	// Target is what the change applied to: a container, compose project,
	// alert rule, token prefix or backup name.
	Target     string `json:"target,omitempty"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"durationMs"`
}

// OK reports whether the change succeeded.
func (e Entry) OK() bool { return e.Status > 0 && e.Status < 400 }

// Path returns the audit log file that sits next to the config file.
func Path(configFilePath string) string {
	return filepath.Join(filepath.Dir(configFilePath), "audit.jsonl")
}

// Store is the audit log. A nil *Store records nothing.
type Store struct {
	path  string
	max   int
	queue chan Entry
	done  chan struct{}

	// sendMu orders Record against Close, so nothing is sent on the closed
	// queue.
	sendMu sync.RWMutex
	closed bool

	mu      sync.RWMutex
	entries []Entry // oldest first, at most max

	// lines counts the records in the file. Only the writer touches it.
	lines int
}

// Open loads the audit log at path and starts its writer. It never fails: an
// unreadable file is logged and the log starts empty, and a file that cannot
// be opened for writing leaves the log in memory only.
func Open(path string) *Store {
	return open(path, MaxEntries)
}

func open(path string, max int) *Store {
	s := &Store{path: path, max: max, queue: make(chan Entry, queueSize), done: make(chan struct{})}
	if data, err := os.ReadFile(path); err == nil {
		s.entries, s.lines = Decode(data)
		if len(s.entries) > max {
			s.entries = s.entries[len(s.entries)-max:]
		}
	} else if !os.IsNotExist(err) {
		log.Printf("audit: failed to read %s: %v", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("audit: failed to open %s, changes are recorded in memory only: %v", path, err)
	}
	go s.writeLoop(f)
	return s
}

// Record queues e for the writer and returns at once. If the queue is full
// the record is dropped and logged rather than holding up the request.
func (s *Store) Record(e Entry) {
	if s == nil {
		return
	}
	s.sendMu.RLock()
	defer s.sendMu.RUnlock()
	if s.closed {
		return
	}
	select {
	case s.queue <- e:
	default:
		log.Printf("audit: queue full, dropped %s by %q on %s", e.Action, e.Actor, e.Target)
	}
}

// Close writes out the queued records and stops the writer.
func (s *Store) Close() {
	if s == nil {
		return
	}
	s.sendMu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	s.sendMu.Unlock()
	<-s.done
}

// writeLoop appends each batch of queued records to the file in one write,
// then publishes them to readers. Entries reach memory only through here, so
// memory always matches what was given to the file and can be used to compact
// it.
func (s *Store) writeLoop(f *os.File) {
	defer close(s.done)
	var buf []byte
	for e := range s.queue {
		batch := []Entry{e}
	drain:
		for {
			select {
			case e, ok := <-s.queue:
				if !ok {
					break drain
				}
				batch = append(batch, e)
			default:
				break drain
			}
		}

		buf = buf[:0]
		for _, e := range batch {
			buf = appendLine(buf, e)
		}
		if f != nil {
			if _, err := f.Write(buf); err != nil {
				log.Printf("audit: failed to append to %s: %v", s.path, err)
			} else if err := f.Sync(); err != nil {
				log.Printf("audit: failed to sync %s: %v", s.path, err)
			}
		}
		s.lines += len(batch)

		s.mu.Lock()
		s.entries = append(s.entries, batch...)
		if over := len(s.entries) - s.max; over > 0 {
			s.entries = s.entries[over:]
		}
		s.mu.Unlock()

		// Rewriting at one and a half times the cap keeps the cost of a
		// rewrite spread over max/2 appends.
		if f != nil && s.lines >= s.max+s.max/2 {
			f = s.compact(f)
		}
	}
	if f != nil {
		f.Close()
	}
}

// compact rewrites the file with only the entries in memory, via a temp file
// and rename, and returns the file to append to next.
func (s *Store) compact(f *os.File) *os.File {
	s.mu.RLock()
	data := Encode(s.entries)
	kept := len(s.entries)
	s.mu.RUnlock()

	if err := writeFileAtomic(s.path, data); err != nil {
		log.Printf("audit: failed to compact %s: %v", s.path, err)
		return f
	}
	f.Close()
	s.lines = kept
	nf, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("audit: failed to reopen %s, changes are recorded in memory only: %v", s.path, err)
		return nil
	}
	return nf
}

// Entries returns a copy of every entry, oldest first.
func (s *Store) Entries() []Entry {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Filter selects entries. Text fields match case-insensitively anywhere in
// the field; zero values match everything.
type Filter struct {
	Actor  string
	Action string
	Target string
	Host   string
	// Result is "success", "failure" or empty for both.
	Result string
	Since  time.Time
	Until  time.Time
	Offset int
	Limit  int
}

// List returns one page of the entries matching f, newest first, and how many
// match in all.
func (s *Store) List(f Filter) ([]Entry, int) {
	out := []Entry{}
	if s == nil {
		return out, 0
	}
	actor, action, target, host := strings.ToLower(f.Actor), strings.ToLower(f.Action), strings.ToLower(f.Target), strings.ToLower(f.Host)
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if !contains(e.Actor, actor) || !contains(e.Action, action) || !contains(e.Target, target) || !contains(e.Host, host) {
			continue
		}
		if (f.Result == "success" && !e.OK()) || (f.Result == "failure" && e.OK()) {
			continue
		}
		if (!f.Since.IsZero() && e.Time.Before(f.Since)) || (!f.Until.IsZero() && e.Time.After(f.Until)) {
			continue
		}
		if total >= f.Offset && len(out) < f.Limit {
			out = append(out, e)
		}
		total++
	}
	return out, total
}

func contains(field, lowerSub string) bool {
	return lowerSub == "" || strings.Contains(strings.ToLower(field), lowerSub)
}

// Encode renders entries as JSON lines, the file and backup format.
func Encode(entries []Entry) []byte {
	var buf []byte
	for _, e := range entries {
		buf = appendLine(buf, e)
	}
	return buf
}

// Decode parses JSON lines, skipping any line that does not parse, such as
// one cut short by a crash. It returns the entries and the number of lines.
func Decode(data []byte) ([]Entry, int) {
	var entries []Entry
	lines := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		lines++
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, lines
}

// Check reports whether the file at path is an audit log this version can
// load: every line must parse.
func Check(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	entries, lines := Decode(data)
	if len(entries) != lines {
		return fmt.Errorf("%d of %d lines are not audit entries", lines-len(entries), lines)
	}
	return nil
}

// MergeInto completes a restored audit log with the live one, so restoring a
// backup never hides changes made since it was taken, including the restore
// itself. It rewrites restoredPath to the restored entries older than the
// oldest live entry, followed by every live entry. Running it twice gives the
// same file.
func MergeInto(restoredPath, livePath string) error {
	live, err := os.ReadFile(livePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	liveEntries, _ := Decode(live)
	if len(liveEntries) == 0 {
		return nil
	}
	data, err := os.ReadFile(restoredPath)
	if err != nil {
		return err
	}
	restored, _ := Decode(data)
	oldest := liveEntries[0].Time
	for _, e := range liveEntries {
		if e.Time.Before(oldest) {
			oldest = e.Time
		}
	}
	var merged []Entry
	for _, e := range restored {
		if e.Time.Before(oldest) {
			merged = append(merged, e)
		}
	}
	merged = append(merged, liveEntries...)
	if len(merged) > MaxEntries {
		merged = merged[len(merged)-MaxEntries:]
	}
	return writeFileAtomic(restoredPath, Encode(merged))
}

func appendLine(buf []byte, e Entry) []byte {
	line, err := json.Marshal(e)
	if err != nil {
		// An Entry is strings, numbers and a time; it always marshals.
		return buf
	}
	return append(append(buf, line...), '\n')
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
