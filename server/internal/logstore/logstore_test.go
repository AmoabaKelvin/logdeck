package logstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

var baseTime = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

// testLimits are permissive enough that retention never interferes with a test
// that is not about retention.
func testLimits() config.ResolvedLogStoreConfig {
	return config.ResolvedLogStoreConfig{Enabled: true, PerContainerMB: 50, TotalMB: 1024}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "logs.db"), testLimits)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// rawLine renders a log line the way the engine hands it over with
// Timestamps: an RFC3339Nano prefix followed by the application's own line.
func rawLine(ts time.Time, message string) string {
	return ts.UTC().Format(time.RFC3339Nano) + " " + message
}

// entryAt builds the parsed entry the live path produces for one engine line.
func entryAt(ts time.Time, stream, message string) models.LogEntry {
	return models.ParseLogLine(rawLine(ts, message), stream)
}

// writeEntries pushes entries straight through the writer's batch commit, the
// same code path the ingestion pipeline uses.
func writeEntries(t *testing.T, s *Store, key genKey, name string, entries ...models.LogEntry) {
	t.Helper()
	batch := make([]ingestMsg, 0, len(entries))
	for _, entry := range entries {
		batch = append(batch, ingestMsg{
			kind: msgLine,
			key:  key,
			name: name,
			line: lineFromEntry(entry),
		})
	}
	if err := s.commit(batch, map[genKey]int64{}); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func markRemoved(t *testing.T, s *Store, key genKey) {
	t.Helper()
	if _, err := s.db.Exec(
		"UPDATE containers SET removed_ms = ? WHERE host = ? AND container_id = ?",
		time.Now().UnixMilli(), key.host, key.id); err != nil {
		t.Fatalf("mark removed: %v", err)
	}
}

func messages(entries []models.LogEntry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Message
	}
	return out
}

func TestSchemaInitAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")

	store, err := Open(path, testLimits)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	writeEntries(t, store, genKey{"local", "aaa"}, "web", entryAt(baseTime, "stdout", "hello"))
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path, testLimits)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	var version int
	if err := reopened.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}

	page, err := reopened.Query(context.Background(), LogQuery{Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Entries) != 1 || page.Entries[0].Message != "hello" {
		t.Fatalf("after reopen got %v, want one entry \"hello\"", messages(page.Entries))
	}
}

func TestIngestQueryRoundTrip(t *testing.T) {
	store := newTestStore(t)
	key := genKey{"local", "aaa"}

	writeEntries(t, store, key, "web",
		entryAt(baseTime, "stdout", "level=info starting up"),
		entryAt(baseTime.Add(time.Second), "stderr", "level=error boom"),
	)

	page, err := store.Query(context.Background(), LogQuery{Host: "local", Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(page.Entries))
	}

	first, second := page.Entries[0], page.Entries[1]
	// A stored entry must render exactly like the live one: same message, same
	// level, same stream — and the timestamp comes from the engine timestamp.
	want := entryAt(baseTime, "stdout", "level=info starting up")
	if first.Message != want.Message || first.Level != want.Level || first.Raw != want.Raw {
		t.Fatalf("stored entry %+v does not match the live entry %+v", first, want)
	}
	if !first.Timestamp.Equal(baseTime) {
		t.Fatalf("timestamp = %s, want %s", first.Timestamp, baseTime)
	}
	if first.ContainerID != "aaa" || first.ContainerName != "web" {
		t.Fatalf("entry not tagged with its container: %+v", first)
	}
	if second.Level != models.LogLevelError || second.Stream != "stderr" {
		t.Fatalf("second entry = %+v, want an ERROR on stderr", second)
	}
	if page.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty at the end of history", page.NextCursor)
	}
}

// TestRebuildSurvival is the acceptance test for the whole feature: after a
// `docker compose up` replaces a container (new engine ID, same name), the
// user must still see the logs from before, as one continuous timeline.
func TestRebuildSurvival(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	oldGen := genKey{"local", "container-a"}
	writeEntries(t, store, oldGen, "web",
		entryAt(baseTime, "stdout", "old generation line 1"),
		entryAt(baseTime.Add(time.Second), "stdout", "old generation line 2"),
	)

	// docker compose up: the old container is destroyed and a new one, with a
	// new engine ID but the same name, takes its place.
	markRemoved(t, store, oldGen)

	newGen := genKey{"local", "container-b"}
	writeEntries(t, store, newGen, "web",
		entryAt(baseTime.Add(2*time.Second), "stdout", "new generation line 1"),
		entryAt(baseTime.Add(3*time.Second), "stdout", "new generation line 2"),
	)

	containers, err := store.ListContainers(ctx)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("got %d logical containers, want 1 (both generations are \"web\"): %+v", len(containers), containers)
	}
	if containers[0].Name != "web" || containers[0].Host != "local" {
		t.Fatalf("logical container = %+v, want local/web", containers[0])
	}
	if containers[0].Removed {
		t.Fatal("web is Removed, but its current generation is alive")
	}
	if !containers[0].OldestTs.Equal(baseTime) || !containers[0].NewestTs.Equal(baseTime.Add(3*time.Second)) {
		t.Fatalf("span = %s..%s, want the full range across both generations",
			containers[0].OldestTs, containers[0].NewestTs)
	}

	page, err := store.Query(ctx, LogQuery{Host: "local", Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{
		"old generation line 1",
		"old generation line 2",
		"new generation line 1",
		"new generation line 2",
	}
	got := messages(page.Entries)
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("timeline = %v, want %v", got, want)
		}
	}
	// One timeline, ascending, spanning both engine containers.
	for i := 1; i < len(page.Entries); i++ {
		if page.Entries[i].Timestamp.Before(page.Entries[i-1].Timestamp) {
			t.Fatalf("entries are not ascending: %v", page.Entries)
		}
	}
	if page.Entries[0].ContainerID != "container-a" || page.Entries[3].ContainerID != "container-b" {
		t.Fatalf("entries lost their generation identity: %s .. %s",
			page.Entries[0].ContainerID, page.Entries[3].ContainerID)
	}
}

func TestWatermarkMath(t *testing.T) {
	cases := []struct {
		name           string
		stdout, stderr int64
		want           int64
	}{
		{"both advanced: the lower one is the safe resume point", 200, 100, 100},
		{"stderr never wrote: it must not drag the mark to zero", 200, 0, 200},
		{"stdout never wrote", 0, 150, 150},
		{"nothing stored yet", 0, 0, 0},
	}
	for _, tc := range cases {
		if got := watermark(tc.stdout, tc.stderr); got != tc.want {
			t.Fatalf("%s: watermark(%d, %d) = %d, want %d", tc.name, tc.stdout, tc.stderr, got, tc.want)
		}
	}

	// The per-stream marks are persisted independently by the writer.
	store := newTestStore(t)
	key := genKey{"local", "aaa"}
	writeEntries(t, store, key, "web",
		entryAt(baseTime, "stdout", "out 1"),
		entryAt(baseTime.Add(5*time.Second), "stdout", "out 2"),
		entryAt(baseTime.Add(time.Second), "stderr", "err 1"),
	)

	var stdoutWM, stderrWM int64
	if err := store.db.QueryRow(
		"SELECT stdout_wm_ns, stderr_wm_ns FROM containers WHERE host = ? AND container_id = ?",
		key.host, key.id).Scan(&stdoutWM, &stderrWM); err != nil {
		t.Fatalf("read watermarks: %v", err)
	}
	if stdoutWM != baseTime.Add(5*time.Second).UnixNano() {
		t.Fatalf("stdout watermark = %d, want the newest stdout line", stdoutWM)
	}
	if stderrWM != baseTime.Add(time.Second).UnixNano() {
		t.Fatalf("stderr watermark = %d, want the newest stderr line", stderrWM)
	}

	// The backfill resume point is the lower of the two, minus the overlap:
	// stderr is behind, so re-reading from stdout's mark would lose stderr
	// lines emitted in between.
	since, err := store.backfillSince(context.Background(), key, 0)
	if err != nil {
		t.Fatalf("backfillSince: %v", err)
	}
	want := time.Unix(0, stderrWM).Add(-backfillOverlap).UTC().Format(time.RFC3339Nano)
	if since != want {
		t.Fatalf("backfill since = %s, want %s", since, want)
	}
}

func TestQueryLevelAndSearchFilters(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	key := genKey{"local", "aaa"}

	writeEntries(t, store, key, "web",
		entryAt(baseTime, "stdout", "level=info request served"),
		entryAt(baseTime.Add(time.Second), "stderr", "level=error database timeout"),
		entryAt(baseTime.Add(2*time.Second), "stdout", "level=warn retrying"),
		entryAt(baseTime.Add(3*time.Second), "stderr", "level=error cache timeout"),
	)

	page, err := store.Query(ctx, LogQuery{Container: "web", Levels: []string{"ERROR"}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 2 ||
		got[0] != "level=error database timeout" || got[1] != "level=error cache timeout" {
		t.Fatalf("level filter returned %v", got)
	}

	page, err = store.Query(ctx, LogQuery{Container: "web", Search: "timeout"})
	if err != nil {
		t.Fatalf("substring Query: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("substring search returned %v", messages(page.Entries))
	}

	page, err = store.Query(ctx, LogQuery{Container: "web", Search: `(database|cache) timeout`, Regex: true})
	if err != nil {
		t.Fatalf("regex Query: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("regex search returned %v", messages(page.Entries))
	}

	page, err = store.Query(ctx, LogQuery{Container: "web", Search: "cache", Regex: true, Levels: []string{"ERROR"}})
	if err != nil {
		t.Fatalf("combined Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "level=error cache timeout" {
		t.Fatalf("combined filter returned %v", got)
	}

	if _, err := store.Query(ctx, LogQuery{Container: "web", Search: "([a-z", Regex: true}); err == nil {
		t.Fatal("an invalid regex must be rejected")
	}

	// Time bounds.
	page, err = store.Query(ctx, LogQuery{
		Container: "web",
		Since:     baseTime.Add(time.Second),
		Until:     baseTime.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("bounded Query: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("time-bounded query returned %v", messages(page.Entries))
	}
}

func TestKeysetPaginationIsStable(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	key := genKey{"local", "aaa"}

	const total = 25
	entries := make([]models.LogEntry, 0, total)
	for i := range total {
		entries = append(entries, entryAt(baseTime.Add(time.Duration(i)*time.Second), "stdout", fmt.Sprintf("line %02d", i)))
	}
	writeEntries(t, store, key, "web", entries...)

	seen := make([]string, 0, total)
	cursor := ""
	for page := 0; page < 10; page++ {
		got, err := store.Query(ctx, LogQuery{Container: "web", Limit: 10, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		// Pages walk backwards through history, so prepend each page.
		seen = append(messages(got.Entries), seen...)
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor

		// A line ingested mid-pagination must not shift the pages already read.
		writeEntries(t, store, key, "web",
			entryAt(baseTime.Add(time.Duration(100+page)*time.Second), "stdout", "late arrival"))
	}

	if len(seen) != total {
		t.Fatalf("paginated over %d entries, want %d: %v", len(seen), total, seen)
	}
	for i := range total {
		if want := fmt.Sprintf("line %02d", i); seen[i] != want {
			t.Fatalf("entry %d = %q, want %q (pagination lost or duplicated a line)", i, seen[i], want)
		}
	}

	if _, err := store.Query(ctx, LogQuery{Container: "web", Cursor: "not-a-cursor!"}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("bad cursor error = %v, want ErrInvalidCursor", err)
	}
}

func TestRetentionPerContainerCap(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "logs.db"), func() config.ResolvedLogStoreConfig {
		return config.ResolvedLogStoreConfig{Enabled: true, PerContainerMB: 1, TotalMB: 1024}
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	key := genKey{"local", "aaa"}
	// Few, large lines: enough bytes to blow the 1 MB cap without paying for
	// tens of thousands of inserts under -race.
	filler := strings.Repeat("x", 1000)
	const lines = 2500 // ~2.6 MB of raw lines, well over the 1 MB cap

	entries := make([]models.LogEntry, 0, lines)
	for i := range lines {
		entries = append(entries, entryAt(baseTime.Add(time.Duration(i)*time.Millisecond), "stdout",
			fmt.Sprintf("line %05d %s", i, filler)))
	}
	writeEntries(t, store, key, "web", entries...)

	if err := store.retain(context.Background()); err != nil {
		t.Fatalf("retain: %v", err)
	}

	var storedBytes int64
	if err := store.db.QueryRow("SELECT stored_bytes FROM containers WHERE container_id = ?", key.id).
		Scan(&storedBytes); err != nil {
		t.Fatalf("read stored_bytes: %v", err)
	}
	if storedBytes > bytesPerMB {
		t.Fatalf("stored_bytes = %d, want at most the 1 MB cap", storedBytes)
	}
	if storedBytes == 0 {
		t.Fatal("retention evicted everything; it must only trim down to the cap")
	}

	// Eviction is oldest-first: the newest line survives, the oldest is gone.
	page, err := store.Query(context.Background(), LogQuery{Container: "web", Limit: 1})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Entries) != 1 || !strings.HasPrefix(page.Entries[0].Message, "line 02499") {
		t.Fatalf("newest line missing after retention: %v", messages(page.Entries))
	}
	var oldest int64
	if err := store.db.QueryRow("SELECT MIN(ts_ns) FROM log_lines").Scan(&oldest); err != nil {
		t.Fatalf("read oldest: %v", err)
	}
	if oldest == baseTime.UnixNano() {
		t.Fatal("the oldest line survived; eviction must be oldest-first")
	}
}

func TestRetentionGlobalCapAndEmptyGenerationPruning(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "logs.db"), func() config.ResolvedLogStoreConfig {
		return config.ResolvedLogStoreConfig{Enabled: true, PerContainerMB: 1024, TotalMB: 1}
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	filler := strings.Repeat("x", 1000)
	writeBulk := func(key genKey, name string, start time.Time, count int) {
		entries := make([]models.LogEntry, 0, count)
		for i := range count {
			entries = append(entries, entryAt(start.Add(time.Duration(i)*time.Millisecond), "stdout",
				fmt.Sprintf("%s %05d %s", name, i, filler)))
		}
		writeEntries(t, store, key, name, entries...)
	}

	// ~0.8 MB each: under the per-container cap, together over the 1 MB total.
	writeBulk(genKey{"local", "old"}, "old-app", baseTime, 800)
	writeBulk(genKey{"local", "new"}, "new-app", baseTime.Add(time.Hour), 800)

	// An already-drained generation of a container the engine no longer has.
	if _, err := store.db.Exec(`
		INSERT INTO containers (host, container_id, name, first_seen_ms, last_seen_ms, removed_ms)
		VALUES ('local', 'ghost', 'ghost-app', 1, 1, 1)`); err != nil {
		t.Fatalf("insert ghost generation: %v", err)
	}

	if err := store.retain(context.Background()); err != nil {
		t.Fatalf("retain: %v", err)
	}

	var total int64
	if err := store.db.QueryRow("SELECT COALESCE(SUM(stored_bytes), 0) FROM containers").Scan(&total); err != nil {
		t.Fatalf("read total: %v", err)
	}
	if total > bytesPerMB {
		t.Fatalf("total stored bytes = %d, want at most the 1 MB cap", total)
	}

	// The oldest container's data is trimmed first, so the newer one keeps all
	// of its lines.
	var oldBytes, newBytes int64
	if err := store.db.QueryRow("SELECT stored_bytes FROM containers WHERE container_id = 'old'").Scan(&oldBytes); err != nil {
		t.Fatalf("read old bytes: %v", err)
	}
	if err := store.db.QueryRow("SELECT stored_bytes FROM containers WHERE container_id = 'new'").Scan(&newBytes); err != nil {
		t.Fatalf("read new bytes: %v", err)
	}
	if oldBytes >= newBytes {
		t.Fatalf("old=%d new=%d: the global sweep must trim the oldest data first", oldBytes, newBytes)
	}

	var ghosts int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM containers WHERE container_id = 'ghost'").Scan(&ghosts); err != nil {
		t.Fatalf("count ghost: %v", err)
	}
	if ghosts != 0 {
		t.Fatal("a removed generation with no lines left must be deleted")
	}
}

func TestOpenFromConfigDisabledCreatesNoDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONFIG_PATH", filepath.Join(dir, "config.json"))
	t.Setenv("LOG_STORE_ENABLED", "false")

	if store := OpenFromConfig(config.NewManager()); store != nil {
		store.Close()
		t.Fatal("OpenFromConfig returned a store while persistence is disabled")
	}
	if _, err := os.Stat(filepath.Join(dir, "logs.db")); !os.IsNotExist(err) {
		t.Fatalf("a disabled store must not create a database file (stat err = %v)", err)
	}
}

func TestOpenFromConfigUnwritablePathDisablesStore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	readonly := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(readonly, 0o500); err != nil {
		t.Fatalf("create read-only dir: %v", err)
	}
	t.Setenv("CONFIG_PATH", filepath.Join(readonly, "data", "config.json"))

	// The server must still boot: an unusable path disables persistence with a
	// warning instead of failing.
	if store := OpenFromConfig(config.NewManager()); store != nil {
		store.Close()
		t.Fatal("OpenFromConfig returned a store for an unwritable path")
	}
}

// --- pipeline fakes ---------------------------------------------------------

type fakeHub struct {
	mu    sync.Mutex
	sink  func(logstream.Record)
	unsub bool
}

func (h *fakeHub) Subscribe(_ logstream.ContainerSpec, opts models.LogOptions, sink func(logstream.Record)) func() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !opts.ShowStdout || !opts.ShowStderr || !opts.Timestamps || opts.Tail != "0" {
		panic(fmt.Sprintf("logstore subscribed with unusable options: %+v", opts))
	}
	h.sink = sink
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.unsub = true
		h.sink = nil
	}
}

func (h *fakeHub) emit(rec logstream.Record) {
	h.mu.Lock()
	sink := h.sink
	h.mu.Unlock()
	if sink != nil {
		sink(rec)
	}
}

type fakeEngine struct {
	mu         sync.Mutex
	hosts      []string
	containers []models.ContainerInfo
	tailCalls  map[string]int
	tailOpts   map[string]models.LogOptions
	tail       func(host, id string, opts models.LogOptions, emit func(models.LogEntry)) error
}

func newFakeEngine(containers ...models.ContainerInfo) *fakeEngine {
	return &fakeEngine{
		hosts:      []string{"local"},
		containers: containers,
		tailCalls:  make(map[string]int),
		tailOpts:   make(map[string]models.LogOptions),
	}
}

func (e *fakeEngine) ListContainersAllHosts(context.Context) (map[string][]models.ContainerInfo, []docker.HostError, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Like the real client: every reachable host is a key, even with no
	// containers on it. A host missing from the map is a host that failed.
	snapshot := make(map[string][]models.ContainerInfo)
	for _, host := range e.hosts {
		snapshot[host] = nil
	}
	for _, info := range e.containers {
		snapshot[info.Host] = append(snapshot[info.Host], info)
	}
	return snapshot, nil, nil
}

func (e *fakeEngine) TailContainerLogs(_ context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
	e.mu.Lock()
	e.tailCalls[id]++
	e.tailOpts[id] = opts
	tail := e.tail
	e.mu.Unlock()

	if tail == nil {
		return nil
	}
	return tail(host, id, opts, emit)
}

func (e *fakeEngine) calls(id string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tailCalls[id]
}

func (e *fakeEngine) opts(id string) models.LogOptions {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tailOpts[id]
}

func containerInfo(host, id, name string, created time.Time) models.ContainerInfo {
	return models.ContainerInfo{
		ID:      id,
		Names:   []string{"/" + name},
		Image:   "app:latest",
		Created: created.Unix(),
		State:   "running",
		Labels:  map[string]string{"com.docker.compose.project": "demostack"},
		Host:    host,
	}
}

// waitFor polls until cond holds, so tests never depend on writer timing.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func countLines(t *testing.T, s *Store) int {
	t.Helper()
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM log_lines").Scan(&count); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	return count
}

// TestBackfillDedupAtBoundary re-reads a generation whose watermark already
// covers part of the engine's output: the overlapping lines must not be stored
// twice, and the lines that are genuinely new must land.
func TestBackfillDedupAtBoundary(t *testing.T) {
	store := newTestStore(t)
	key := genKey{"local", "aaa"}

	// Already stored from a previous run of the process.
	stored := []models.LogEntry{
		entryAt(baseTime, "stdout", "line 1"),
		entryAt(baseTime.Add(time.Second), "stdout", "line 2"),
		entryAt(baseTime.Add(time.Second), "stderr", "line 2 on stderr"),
	}
	writeEntries(t, store, key, "web", stored...)

	engine := newFakeEngine(containerInfo("local", "aaa", "web", baseTime.Add(-time.Minute)))
	// The engine re-emits everything from the requested "since", which overlaps
	// the stored lines, plus the two lines written while nobody was listening.
	engine.tail = func(_, _ string, _ models.LogOptions, emit func(models.LogEntry)) error {
		for _, entry := range stored {
			emit(entry)
		}
		emit(entryAt(baseTime.Add(2*time.Second), "stdout", "line 3"))
		emit(entryAt(baseTime.Add(3*time.Second), "stderr", "line 4"))
		return nil
	}

	hub := &fakeHub{}
	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, hub, func() Engine { return engine })

	waitFor(t, "backfill to land", func() bool { return countLines(t, store) == 5 })

	// A live line that duplicates a backfilled one (the tail and the hub can
	// both deliver the boundary line) must also be deduplicated.
	hub.emit(logstream.Record{
		Host: "local", ContainerID: "aaa", ContainerName: "web",
		Entry: entryAt(baseTime.Add(3*time.Second), "stderr", "line 4"),
	})
	time.Sleep(2 * batchInterval)

	cancel()
	store.Wait()

	if got := countLines(t, store); got != 5 {
		t.Fatalf("stored %d lines, want 5 (the overlap must be deduplicated)", got)
	}

	page, err := store.Query(context.Background(), LogQuery{Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"line 1", "line 2", "line 2 on stderr", "line 3", "line 4"}
	got := messages(page.Entries)
	if len(got) != len(want) {
		t.Fatalf("timeline = %v, want %v", got, want)
	}

	// Gap healing must not be capped by the engine's default tail of 100 lines,
	// and it must resume from the watermark rather than from creation.
	opts := engine.opts("aaa")
	if opts.Tail != "" {
		t.Fatalf("backfill Tail = %q, want \"\" (unlimited)", opts.Tail)
	}
	if opts.Follow || !opts.Timestamps || !opts.ShowStdout || !opts.ShowStderr {
		t.Fatalf("backfill opts = %+v", opts)
	}
	wantSince := time.Unix(0, baseTime.Add(time.Second).UnixNano()).Add(-backfillOverlap).UTC().Format(time.RFC3339Nano)
	if opts.Since != wantSince {
		t.Fatalf("backfill Since = %s, want the watermark minus the overlap (%s)", opts.Since, wantSince)
	}
}

func TestNewGenerationBackfillsFromCreation(t *testing.T) {
	store := newTestStore(t)
	created := baseTime.Add(-time.Hour)
	engine := newFakeEngine(containerInfo("local", "aaa", "web", created))
	engine.tail = func(_, _ string, _ models.LogOptions, emit func(models.LogEntry)) error {
		emit(entryAt(baseTime, "stdout", "from the engine"))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, &fakeHub{}, func() Engine { return engine })
	waitFor(t, "backfill to land", func() bool { return countLines(t, store) == 1 })
	cancel()
	store.Wait()

	if since := engine.opts("aaa").Since; since != created.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("Since = %s, want container creation time %s", since, created.UTC().Format(time.RFC3339Nano))
	}

	containers, err := store.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 1 || containers[0].Image != "app:latest" || containers[0].ComposeProject != "demostack" {
		t.Fatalf("engine metadata was not recorded: %+v", containers)
	}
}

// TestUnreadableDriverIsExcludedAndNotRetried covers containers whose logging
// driver has no read API: the generation is marked excluded, surfaced in the
// listing, and never re-read.
func TestUnreadableDriverIsExcludedAndNotRetried(t *testing.T) {
	store := newTestStore(t)
	engine := newFakeEngine(containerInfo("local", "aaa", "web", baseTime))
	engine.tail = func(_, _ string, _ models.LogOptions, _ func(models.LogEntry)) error {
		return errors.New("Error response from daemon: configured logging driver does not support reading")
	}

	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, &fakeHub{}, func() Engine { return engine })

	waitFor(t, "the generation to be excluded", func() bool {
		containers, err := store.ListContainers(context.Background())
		return err == nil && len(containers) == 1 && containers[0].Excluded
	})

	// Re-sync: an excluded generation must never be re-read.
	store.sync(ctx, engine, newBackfillTracker(), make(chan backfillResult, 1))
	time.Sleep(50 * time.Millisecond)

	cancel()
	store.Wait()

	if calls := engine.calls("aaa"); calls != 1 {
		t.Fatalf("the engine was tailed %d times, want exactly 1 (an unreadable driver must not be retried)", calls)
	}

	containers, err := store.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if !containers[0].Excluded || !strings.Contains(containers[0].ExcludedReason, "does not support reading") {
		t.Fatalf("exclusion is not surfaced in the listing: %+v", containers[0])
	}
}

func TestRemovedContainerKeepsItsHistory(t *testing.T) {
	store := newTestStore(t)
	engine := newFakeEngine(containerInfo("local", "aaa", "web", baseTime))

	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, &fakeHub{}, func() Engine { return engine })
	waitFor(t, "the container to be recorded", func() bool {
		containers, err := store.ListContainers(context.Background())
		return err == nil && len(containers) == 1
	})
	cancel()
	store.Wait()

	writeEntries(t, store, genKey{"local", "aaa"}, "web", entryAt(baseTime, "stdout", "before removal"))

	// The container disappears from the engine.
	engine.mu.Lock()
	engine.containers = nil
	engine.mu.Unlock()
	store.sync(context.Background(), engine, newBackfillTracker(), make(chan backfillResult, 1))

	containers, err := store.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 1 || !containers[0].Removed {
		t.Fatalf("a removed container must still be listed, marked Removed: %+v", containers)
	}

	page, err := store.Query(context.Background(), LogQuery{Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "before removal" {
		t.Fatalf("a removed container lost its history: %v", got)
	}
}

func TestLiveIngestionThroughHub(t *testing.T) {
	store := newTestStore(t)
	hub := &fakeHub{}
	engine := newFakeEngine()

	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, hub, func() Engine { return engine })

	for i := range 3 {
		hub.emit(logstream.Record{
			Host:          "local",
			ContainerID:   "aaa",
			ContainerName: "web",
			Labels:        map[string]string{"io.podman.compose.project": "demostack"},
			Entry:         entryAt(baseTime.Add(time.Duration(i)*time.Second), "stdout", fmt.Sprintf("live %d", i)),
		})
	}

	waitFor(t, "live lines to be committed", func() bool { return countLines(t, store) == 3 })
	cancel()
	store.Wait()

	if !hub.unsub {
		t.Fatal("the hub subscription must be dropped on shutdown")
	}

	containers, err := store.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(containers) != 1 || containers[0].ComposeProject != "demostack" {
		t.Fatalf("compose project not taken from the podman label: %+v", containers)
	}
}

// --- data integrity regressions --------------------------------------------

// TestMigrationFromV1AddsTheForeignKey upgrades a database written by the
// previous schema: the lines are kept, rows that were already misfiled against a
// generation that does not exist are dropped, and new dangling refs are rejected.
func TestMigrationFromV1AddsTheForeignKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := db.Exec(schemaV1 + `
		INSERT INTO containers (id, host, container_id, name, first_seen_ms, last_seen_ms)
		VALUES (1, 'local', 'aaa', 'web', 1, 1);
		INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw) VALUES (1, 10, 0, 0, 'kept');
		INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw) VALUES (77, 20, 0, 0, 'orphan');
		PRAGMA user_version = 1;`); err != nil {
		t.Fatalf("write a v1 database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	store, err := Open(path, testLimits)
	if err != nil {
		t.Fatalf("Open (migration): %v", err)
	}
	defer store.Close()

	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}

	page, err := store.Query(context.Background(), LogQuery{Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("the migration lost attributable history: %v", got)
	}
	if got := countLines(t, store); got != 1 {
		t.Fatalf("stored %d lines, want 1: the unattributable row must not survive", got)
	}
	if _, err := store.db.Exec(
		"INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw) VALUES (77, 30, 0, 0, 'orphan')",
	); err == nil {
		t.Fatal("after the migration a dangling container_ref must be rejected")
	}
}

// commitThrough writes a batch through the writer's own generation-id cache,
// the way writeLoop does across batches.
func commitThrough(t *testing.T, s *Store, refs map[genKey]int64, key genKey, name string, entries ...models.LogEntry) {
	t.Helper()
	batch := make([]ingestMsg, 0, len(entries))
	for _, entry := range entries {
		batch = append(batch, ingestMsg{kind: msgLine, key: key, name: name, line: lineFromEntry(entry)})
	}
	if err := s.commit(batch, refs); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// TestFailedCommitDoesNotPoisonTheRefCache covers the worst failure this store
// can have: one container's lines landing in another container's timeline.
//
// A transaction that dies after INSERT ... RETURNING id (disk full, a lock held
// past the busy timeout, an I/O error) is rolled back, and SQLite hands the very
// same rowid to the next generation that is created. Caching that id before the
// commit succeeds therefore files the lines of the generation that failed
// against whatever generation inherited the rowid.
func TestFailedCommitDoesNotPoisonTheRefCache(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	refs := map[genKey]int64{} // the writer's cache, alive across batches
	alpha := genKey{"local", "container-a"}
	beta := genKey{"local", "container-b"}

	// Make the next write transaction fail, after the generation row was
	// inserted and its id returned.
	if _, err := store.db.Exec(
		"CREATE TRIGGER fail_writes BEFORE INSERT ON log_lines BEGIN SELECT RAISE(ABORT, 'disk full'); END",
	); err != nil {
		t.Fatalf("install failing trigger: %v", err)
	}
	batch := []ingestMsg{{
		kind: msgLine, key: alpha, name: "alpha",
		line: lineFromEntry(entryAt(baseTime, "stdout", "alpha line 1")),
	}}
	if err := store.commit(batch, refs); err == nil {
		t.Fatal("the batch must fail while the trigger is installed")
	}
	if len(refs) != 0 {
		t.Fatalf("a rolled-back transaction published %v into the ref cache: SQLite reuses that rowid", refs)
	}
	if _, err := store.db.Exec("DROP TRIGGER fail_writes"); err != nil {
		t.Fatalf("drop trigger: %v", err)
	}

	// A different container is created next and takes the rowid the rolled-back
	// row had been handed.
	commitThrough(t, store, refs, beta, "beta", entryAt(baseTime.Add(time.Second), "stdout", "beta line 1"))
	// alpha writes again, through the same cache.
	commitThrough(t, store, refs, alpha, "alpha", entryAt(baseTime.Add(2*time.Second), "stdout", "alpha line 2"))

	page, err := store.Query(ctx, LogQuery{Container: "beta"})
	if err != nil {
		t.Fatalf("Query beta: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "beta line 1" {
		t.Fatalf("beta's timeline = %v, want only its own line (it was given alpha's)", got)
	}
	page, err = store.Query(ctx, LogQuery{Container: "alpha"})
	if err != nil {
		t.Fatalf("Query alpha: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "alpha line 2" {
		t.Fatalf("alpha's timeline = %v, want its own line", got)
	}

	// And a dangling ref can no longer be written at all: the foreign key turns
	// a misattribution into a loud failure.
	if _, err := store.db.Exec(
		"INSERT INTO log_lines (container_ref, ts_ns, stream, level, raw) VALUES (9999, 1, 0, 0, 'orphan')",
	); err == nil {
		t.Fatal("a line with a container_ref that does not exist must be rejected")
	}
}

// TestRestartDoesNotLoseTheDowntimeWindow is the regression for the gap this
// whole feature exists to heal: logdeck is down, the container keeps logging,
// and on restart live ingestion starts before the first backfill. If the
// backfill read its resume point from the live watermark columns, they would
// already have jumped to "now" and the entire downtime window would be lost.
func TestRestartDoesNotLoseTheDowntimeWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	key := genKey{"local", "aaa"}

	// A previous process stored history up to baseTime, then stopped.
	previous, err := Open(path, testLimits)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	writeEntries(t, previous, key, "web", entryAt(baseTime, "stdout", "before the restart"))
	if err := previous.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store, err := Open(path, testLimits)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()

	engine := newFakeEngine(containerInfo("local", "aaa", "web", baseTime.Add(-time.Hour)))
	engine.tail = func(_, _ string, _ models.LogOptions, emit func(models.LogEntry)) error {
		emit(entryAt(baseTime.Add(30*time.Minute), "stdout", "written while logdeck was down"))
		return nil
	}

	// The sync loop cannot reach the engine until the gate opens, so a live line
	// is guaranteed to be committed before the first backfill computes its
	// resume point — the exact ordering the bug needed.
	hub := &fakeHub{}
	gate := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx, hub, func() Engine {
		<-gate
		return engine
	})

	hub.emit(logstream.Record{
		Host: "local", ContainerID: "aaa", ContainerName: "web",
		Entry: entryAt(baseTime.Add(time.Hour), "stdout", "live line after the restart"),
	})
	waitFor(t, "the live line to be committed", func() bool { return countLines(t, store) == 2 })

	close(gate)
	waitFor(t, "the backfill to land", func() bool { return countLines(t, store) == 3 })
	cancel()
	store.Wait()

	wantSince := baseTime.Add(-backfillOverlap).UTC().Format(time.RFC3339Nano)
	if since := engine.opts("aaa").Since; since != wantSince {
		t.Fatalf("backfill Since = %s, want the watermark stored before the restart (%s): the downtime window is lost",
			since, wantSince)
	}

	page, err := store.Query(context.Background(), LogQuery{Container: "web"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"before the restart", "written while logdeck was down", "live line after the restart"}
	if got := messages(page.Entries); !slices.Equal(got, want) {
		t.Fatalf("timeline = %v, want %v", got, want)
	}
}

// TestLevelFilterKeepsMultiLineEntriesWhole covers the history/live divergence
// that a SQL-side level filter causes: the continuation lines of a stack trace
// classify as UNKNOWN, so filtering rows before grouping strips the body out of
// the very entry the user filtered for.
func TestLevelFilterKeepsMultiLineEntriesWhole(t *testing.T) {
	store := newTestStore(t)
	key := genKey{"local", "aaa"}

	writeEntries(t, store, key, "web",
		entryAt(baseTime, "stdout", "level=info request served"),
		entryAt(baseTime.Add(time.Second), "stderr", "level=error unhandled exception"),
		entryAt(baseTime.Add(2*time.Second), "stderr", "at com.example.Service.handle(Service.java:42)"),
		entryAt(baseTime.Add(3*time.Second), "stderr", "at com.example.Server.dispatch(Server.java:17)"),
		entryAt(baseTime.Add(4*time.Second), "stdout", "level=info recovered"),
	)

	page, err := store.Query(context.Background(), LogQuery{Container: "web", Levels: []string{"ERROR"}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("level filter returned %d entries, want the single grouped ERROR: %v",
			len(page.Entries), messages(page.Entries))
	}
	want := "level=error unhandled exception\n" +
		"at com.example.Service.handle(Service.java:42)\n" +
		"at com.example.Server.dispatch(Server.java:17)"
	if got := page.Entries[0].Message; got != want {
		t.Fatalf("the filtered entry lost its body:\n got %q\nwant %q", got, want)
	}
}

// TestSearchMatchesLikeTheLiveView pins the search semantics to the live path's:
// case-insensitive, over the parsed message rather than the raw line.
func TestSearchMatchesLikeTheLiveView(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	key := genKey{"local", "aaa"}

	writeEntries(t, store, key, "web",
		entryAt(baseTime, "stderr", "ERROR database is on fire"),
		entryAt(baseTime.Add(time.Second), "stdout", "all good"),
	)

	page, err := store.Query(ctx, LogQuery{Container: "web", Search: "error"})
	if err != nil {
		t.Fatalf("substring Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 || got[0] != "ERROR database is on fire" {
		t.Fatalf("case-insensitive substring search returned %v", got)
	}

	page, err = store.Query(ctx, LogQuery{Container: "web", Search: "^error db?atabase", Regex: true})
	if err != nil {
		t.Fatalf("regex Query: %v", err)
	}
	if got := messages(page.Entries); len(got) != 1 {
		t.Fatalf("case-insensitive regex search returned %v", got)
	}

	// The engine's timestamp prefix is not part of the message, so a search for a
	// timestamp-like string must not match every line.
	page, err = store.Query(ctx, LogQuery{Container: "web", Search: baseTime.UTC().Format("2006-01-02")})
	if err != nil {
		t.Fatalf("timestamp Query: %v", err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("search matched the engine timestamp prefix: %v", messages(page.Entries))
	}
}

// TestPageThatExhaustsHistoryHasNoCursor covers both cursor defects: a page that
// exactly empties history must not offer a phantom "load older", and a filtered
// query must keep scanning rather than return an empty page with a cursor.
func TestPageThatExhaustsHistoryHasNoCursor(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	key := genKey{"local", "aaa"}

	entries := make([]models.LogEntry, 0, 10)
	for i := range 10 {
		entries = append(entries, entryAt(baseTime.Add(time.Duration(i)*time.Second), "stdout",
			fmt.Sprintf("line %02d", i)))
	}
	writeEntries(t, store, key, "web", entries...)

	page, err := store.Query(ctx, LogQuery{Container: "web", Limit: 10})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Entries) != 10 {
		t.Fatalf("got %d entries, want the whole history", len(page.Entries))
	}
	if page.NextCursor != "" {
		t.Fatalf("a page that exhausts history must not offer another one (cursor %q)", page.NextCursor)
	}

	// A filter that matches nothing: an empty page must be a final page.
	page, err = store.Query(ctx, LogQuery{Container: "web", Limit: 10, Search: "nothing matches this"})
	if err != nil {
		t.Fatalf("filtered Query: %v", err)
	}
	if len(page.Entries) != 0 || page.NextCursor != "" {
		t.Fatalf("empty page carries cursor %q with %d entries", page.NextCursor, len(page.Entries))
	}

	// A filter that matches more than one page still pages cleanly.
	page, err = store.Query(ctx, LogQuery{Container: "web", Limit: 4})
	if err != nil {
		t.Fatalf("paged Query: %v", err)
	}
	if len(page.Entries) != 4 || page.NextCursor == "" {
		t.Fatalf("a page with older history behind it must carry a cursor: %v", messages(page.Entries))
	}
	if got := messages(page.Entries); got[0] != "line 06" || got[3] != "line 09" {
		t.Fatalf("first page = %v, want the newest four lines", got)
	}
}

// TestDroppedLinesAreReReadFromTheEngine covers the ingest buffer overflowing:
// the sink has to drop the record, but the loss must be recoverable — the next
// sync re-reads the generation from the dropped line's timestamp.
func TestDroppedLinesAreReReadFromTheEngine(t *testing.T) {
	store := newTestStore(t)
	key := genKey{"local", "aaa"}
	dropped := entryAt(baseTime.Add(time.Minute), "stdout", "dropped by a full buffer")

	// The sink drops when the queue is full; fill it without a writer running.
	for len(store.ingestCh) < cap(store.ingestCh) {
		store.ingestCh <- ingestMsg{kind: msgLine, key: key, name: "web",
			line: lineFromEntry(entryAt(baseTime, "stdout", "filler"))}
	}
	store.sink(logstream.Record{
		Host: "local", ContainerID: "aaa", ContainerName: "web", Entry: dropped,
	})
	if got := store.gapAt(key); got != dropped.Timestamp.UnixNano() {
		t.Fatalf("gap mark = %d, want the dropped line's timestamp %d", got, dropped.Timestamp.UnixNano())
	}

	// The next backfill resumes from the drop, even though the watermark is newer.
	writeEntries(t, store, key, "web", entryAt(baseTime.Add(2*time.Minute), "stdout", "later line"))
	since, err := store.backfillSince(context.Background(), key, 0)
	if err != nil {
		t.Fatalf("backfillSince: %v", err)
	}
	want := dropped.Timestamp.Add(-backfillOverlap).UTC().Format(time.RFC3339Nano)
	if since != want {
		t.Fatalf("backfill Since = %s, want the dropped line's timestamp (%s): the drop is unrecoverable", since, want)
	}

	// A fresh drop makes an already-read generation eligible for another read,
	// even after its retry budget was spent.
	track := newBackfillTracker()
	track.done[key] = true
	track.attempts[key] = maxBackfillAttempts
	if !track.schedule(store, key, false) {
		t.Fatal("a dropped line must schedule a re-read of the generation")
	}
	delete(track.inFlight, key)
	track.complete(store, key)
	if got := store.gapAt(key); got != 0 {
		t.Fatalf("the healed gap was not retired: %d", got)
	}
	if track.schedule(store, key, false) {
		t.Fatal("a healed generation must not be re-read forever")
	}
}

// TestSuccessfulBackfillIsNotRetried: the attempt budget counts failures, not
// successes. Re-reading a healthy generation every sync would re-scan its whole
// engine log, and would burn the budget that gap healing depends on.
func TestSuccessfulBackfillIsNotRetried(t *testing.T) {
	store := newTestStore(t)
	engine := newFakeEngine(containerInfo("local", "aaa", "web", baseTime))
	engine.tail = func(_, _ string, _ models.LogOptions, emit func(models.LogEntry)) error {
		emit(entryAt(baseTime, "stdout", "from the engine"))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	track := newBackfillTracker()
	results := make(chan backfillResult, 1)

	store.sync(ctx, engine, track, results)
	res := <-results
	if res.err != nil {
		t.Fatalf("backfill failed: %v", res.err)
	}
	delete(track.inFlight, res.key)
	track.complete(store, res.key)

	// Two more lifecycle passes: a generation that has already been read must not
	// be read again.
	store.sync(ctx, engine, track, results)
	store.sync(ctx, engine, track, results)
	time.Sleep(50 * time.Millisecond)

	if calls := engine.calls("aaa"); calls != 1 {
		t.Fatalf("the engine was tailed %d times, want exactly 1 (a success must not be retried)", calls)
	}
}

// TestGenerationsOfAnUnconfiguredHostAreMarkedRemoved: a host taken out of the
// config is gone, but a host that merely failed to list is not — that
// distinction is what keeps an unreachable host from looking like a mass
// container removal.
func TestGenerationsOfAnUnconfiguredHostAreMarkedRemoved(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	writeEntries(t, store, genKey{"local", "aaa"}, "web", entryAt(baseTime, "stdout", "local line"))
	writeEntries(t, store, genKey{"staging", "bbb"}, "api", entryAt(baseTime, "stdout", "staging line"))
	writeEntries(t, store, genKey{"prod", "ccc"}, "api", entryAt(baseTime, "stdout", "prod line"))

	// local lists and still has its container; staging is unreachable; prod is no
	// longer configured at all.
	live := map[genKey]bool{{host: "local", id: "aaa"}: true}
	failed := map[string]bool{"staging": true}
	if err := store.markRemoved(ctx, failed, live, time.Now().UnixMilli()); err != nil {
		t.Fatalf("markRemoved: %v", err)
	}

	removed := func(key genKey) bool {
		t.Helper()
		var stamp *int64
		if err := store.db.QueryRow(
			"SELECT removed_ms FROM containers WHERE host = ? AND container_id = ?", key.host, key.id,
		).Scan(&stamp); err != nil {
			t.Fatalf("read removed_ms: %v", err)
		}
		return stamp != nil
	}
	if removed(genKey{"local", "aaa"}) {
		t.Fatal("a live container was marked removed")
	}
	if removed(genKey{"staging", "bbb"}) {
		t.Fatal("a host that failed to list must keep its generations: an outage is not a removal")
	}
	if !removed(genKey{"prod", "ccc"}) {
		t.Fatal("a host that is no longer configured must have its generations marked removed")
	}
}
