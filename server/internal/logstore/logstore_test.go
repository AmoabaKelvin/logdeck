package logstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	store.sync(ctx, engine, map[genKey]int{}, map[genKey]bool{}, make(chan backfillResult, 1))
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
	store.sync(context.Background(), engine, map[genKey]int{}, map[genKey]bool{}, make(chan backfillResult, 1))

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
