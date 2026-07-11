package logstream

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// fakeClient is a scripted engineClient: canned container snapshots (with
// optional per-host errors), a pushable event channel, and tails that either
// run a custom tailFn or block until cancelled / the client is "closed".
type fakeClient struct {
	events chan docker.EngineEvent
	closed chan struct{}
	once   sync.Once

	// tailFn, when set (before the hub runs), replaces the default blocking
	// tail body. Start/stop tracking applies either way.
	tailFn func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error

	mu       sync.Mutex
	snapshot map[string][]models.ContainerInfo
	hostErrs []docker.HostError
	active   map[containerKey]int
	starts   map[containerKey]int
	streams  int
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		events:   make(chan docker.EngineEvent, 16),
		closed:   make(chan struct{}),
		snapshot: map[string][]models.ContainerInfo{},
		active:   map[containerKey]int{},
		starts:   map[containerKey]int{},
	}
}

func (f *fakeClient) set(snapshot map[string][]models.ContainerInfo, hostErrs []docker.HostError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshot = snapshot
	f.hostErrs = hostErrs
}

func (f *fakeClient) close() { f.once.Do(func() { close(f.closed) }) }

func (f *fakeClient) activeTails(key containerKey) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active[key]
}

func (f *fakeClient) totalStarts(key containerKey) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts[key]
}

func (f *fakeClient) streamCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.streams
}

func (f *fakeClient) listContainers(context.Context) (map[string][]models.ContainerInfo, []docker.HostError, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot, f.hostErrs, nil
}

func (f *fakeClient) streamEvents(ctx context.Context) <-chan docker.EngineEvent {
	f.mu.Lock()
	f.streams++
	f.mu.Unlock()
	out := make(chan docker.EngineEvent)
	go func() {
		defer close(out)
		for {
			select {
			case ev := <-f.events:
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			case <-f.closed:
				return
			}
		}
	}()
	return out
}

func (f *fakeClient) openTail(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
	key := containerKey{host: host, id: id}
	f.mu.Lock()
	f.active[key]++
	f.starts[key]++
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active[key]--
		f.mu.Unlock()
	}()

	if f.tailFn != nil {
		return f.tailFn(ctx, host, id, opts, emit)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.closed:
		return errors.New("client closed")
	}
}

// recorder is a thread-safe sink.
type recorder struct {
	mu   sync.Mutex
	recs []Record
}

func (r *recorder) sink(rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, rec)
}

func (r *recorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.recs)
}

func (r *recorder) all() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Record(nil), r.recs...)
}

func ctr(id, name, state string, labels map[string]string) models.ContainerInfo {
	return models.ContainerInfo{ID: id, Names: []string{"/" + name}, State: state, Labels: labels}
}

// startHub runs a hub over source with fast retry timings and registers
// cleanup that cancels it and requires a prompt, leak-free shutdown.
func startHub(t *testing.T, source func() engineClient) *Hub {
	t.Helper()
	h := newHub(nil, source)
	h.retryBaseDelay = time.Millisecond
	h.retryMaxDelay = 4 * time.Millisecond
	h.listTimeout = time.Second
	h.resyncInterval = time.Hour // tests drive resync via Reconcile pokes

	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	t.Cleanup(func() {
		cancel()
		waitShutdown(t, h)
	})
	return h
}

func waitShutdown(t *testing.T, h *Hub) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		h.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hub did not shut down within 5s")
	}
}

func waitFor(t *testing.T, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", msg)
}

func TestSpecMatches(t *testing.T) {
	cases := []struct {
		name   string
		spec   ContainerSpec
		host   string
		cname  string
		labels map[string]string
		want   bool
	}{
		{"empty spec matches all", ContainerSpec{}, "h1", "web", nil, true},
		{"host filter hit", ContainerSpec{Hosts: []string{"h1"}}, "h1", "web", nil, true},
		{"host filter miss", ContainerSpec{Hosts: []string{"h2"}}, "h1", "web", nil, false},
		{"name match", ContainerSpec{Containers: []string{"web"}}, "h1", "web", nil, true},
		{"name miss", ContainerSpec{Containers: []string{"web"}}, "h1", "db", nil, false},
		{"docker project label", ContainerSpec{Projects: []string{"demo"}}, "h1", "db", map[string]string{"com.docker.compose.project": "demo"}, true},
		{"podman project label", ContainerSpec{Projects: []string{"demo"}}, "h1", "db", map[string]string{"io.podman.compose.project": "demo"}, true},
		{"project miss", ContainerSpec{Projects: []string{"demo"}}, "h1", "db", map[string]string{"com.docker.compose.project": "other"}, false},
		{"name or project", ContainerSpec{Containers: []string{"web"}, Projects: []string{"demo"}}, "h1", "db", map[string]string{"com.docker.compose.project": "demo"}, true},
		{"host and name both required", ContainerSpec{Hosts: []string{"h2"}, Containers: []string{"web"}}, "h1", "web", nil, false},
	}
	for _, tc := range cases {
		if got := specMatches(tc.spec, tc.host, tc.cname, tc.labels); got != tc.want {
			t.Errorf("%s: specMatches = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSubscribeTailsOnlyMatchedRunningContainers(t *testing.T) {
	f := newFakeClient()
	f.tailFn = func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
		emit(models.LogEntry{Message: "hello-" + id})
		<-ctx.Done()
		return ctx.Err()
	}
	f.set(map[string][]models.ContainerInfo{
		"h1": {
			ctr("c1", "web", "running", map[string]string{"com.docker.compose.project": "demo"}),
			ctr("c2", "db", "running", nil),
			ctr("c3", "web", "exited", nil),
		},
	}, nil)
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	h.Subscribe(ContainerSpec{Containers: []string{"web"}}, models.LogOptions{Tail: "0", ShowStdout: true, ShowStderr: true}, rec.sink)

	waitFor(t, "matched tail to start", func() bool { return f.activeTails(containerKey{"h1", "c1"}) == 1 })
	waitFor(t, "record delivery", func() bool { return rec.len() == 1 })

	if n := f.totalStarts(containerKey{"h1", "c2"}); n != 0 {
		t.Errorf("unmatched container c2 was tailed %d times", n)
	}
	if n := f.totalStarts(containerKey{"h1", "c3"}); n != 0 {
		t.Errorf("exited container c3 was tailed %d times", n)
	}
	got := rec.all()[0]
	if got.Host != "h1" || got.ContainerID != "c1" || got.ContainerName != "web" || got.Entry.Message != "hello-c1" {
		t.Errorf("unexpected record: %+v", got)
	}
	if got.Labels["com.docker.compose.project"] != "demo" {
		t.Errorf("record labels not propagated: %+v", got.Labels)
	}
}

func TestStartEventSpawnsTail(t *testing.T) {
	f := newFakeClient()
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	h.Subscribe(ContainerSpec{Containers: []string{"api"}}, models.LogOptions{}, rec.sink)

	// No running containers yet; then the container starts.
	f.events <- docker.EngineEvent{Host: "h1", ContainerID: "c9", ContainerName: "api", Action: "start",
		Labels: map[string]string{"name": "api"}}

	waitFor(t, "start event to spawn tail", func() bool { return f.activeTails(containerKey{"h1", "c9"}) == 1 })
}

func TestDieEventCancelsTail(t *testing.T) {
	f := newFakeClient()
	f.set(map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}, nil)
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	h.Subscribe(ContainerSpec{}, models.LogOptions{}, rec.sink)
	key := containerKey{"h1", "c1"}
	waitFor(t, "tail to start", func() bool { return f.activeTails(key) == 1 })

	f.set(map[string][]models.ContainerInfo{"h1": {}}, nil)
	f.events <- docker.EngineEvent{Host: "h1", ContainerID: "c1", ContainerName: "web", Action: "die"}

	waitFor(t, "die event to cancel tail", func() bool { return f.activeTails(key) == 0 })
	if n := f.totalStarts(key); n != 1 {
		t.Errorf("tail restarted after die: %d starts", n)
	}
}

func TestUnsubscribeCancelsAndIsIdempotent(t *testing.T) {
	f := newFakeClient()
	f.set(map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}, nil)
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	unsubscribe := h.Subscribe(ContainerSpec{}, models.LogOptions{}, rec.sink)
	key := containerKey{"h1", "c1"}
	waitFor(t, "tail to start", func() bool { return f.activeTails(key) == 1 })

	unsubscribe()
	waitFor(t, "unsubscribe to cancel tail", func() bool { return f.activeTails(key) == 0 })
	unsubscribe() // second call must return without blocking or panicking

	before := rec.len()
	time.Sleep(20 * time.Millisecond)
	if after := rec.len(); after != before {
		t.Errorf("sink called after unsubscribe: %d -> %d records", before, after)
	}
}

func TestFailedHostListingPreservesTails(t *testing.T) {
	f := newFakeClient()
	f.set(map[string][]models.ContainerInfo{
		"h1": {ctr("c1", "web", "running", nil)},
		"h2": {ctr("c2", "db", "running", nil)},
	}, nil)
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	h.Subscribe(ContainerSpec{}, models.LogOptions{}, rec.sink)
	k1, k2 := containerKey{"h1", "c1"}, containerKey{"h2", "c2"}
	waitFor(t, "both tails to start", func() bool { return f.activeTails(k1) == 1 && f.activeTails(k2) == 1 })

	// h1 becomes unreachable, h2 lists fine but its container is gone.
	f.set(map[string][]models.ContainerInfo{"h2": {}},
		[]docker.HostError{{HostName: "h1", Err: errors.New("connection refused")}})
	h.Reconcile()

	waitFor(t, "healthy-host disappearance to cancel tail", func() bool { return f.activeTails(k2) == 0 })
	if f.activeTails(k1) != 1 {
		t.Error("tail on failed host was cancelled; it must be preserved")
	}
}

func TestSlowSinkDropsOldestAndTailKeepsReading(t *testing.T) {
	total := ringSize + 100
	emitted := make(chan struct{})
	f := newFakeClient()
	f.tailFn = func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
		for i := 0; i < total; i++ {
			emit(models.LogEntry{Message: strconv.Itoa(i)})
		}
		close(emitted)
		<-ctx.Done()
		return ctx.Err()
	}
	f.set(map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}, nil)
	h := startHub(t, func() engineClient { return f })

	gate := make(chan struct{})
	rec := &recorder{}
	sub, unsubscribe := h.subscribe(ContainerSpec{}, models.LogOptions{}, func(r Record) {
		<-gate
		rec.sink(r)
	})
	defer unsubscribe()

	// The tail must finish all pushes while the sink is stuck: it never blocks.
	select {
	case <-emitted:
	case <-time.After(3 * time.Second):
		t.Fatal("tail blocked on slow sink")
	}

	drops := sub.drops.Load()
	if drops < uint64(total-ringSize-1) {
		t.Fatalf("drop counter = %d, want >= %d", drops, total-ringSize-1)
	}

	close(gate)
	want := total - int(drops)
	waitFor(t, "buffered records to drain", func() bool { return rec.len() == want })
	recs := rec.all()
	if last := recs[len(recs)-1].Entry.Message; last != strconv.Itoa(total-1) {
		t.Errorf("newest record lost: last delivered = %q, want %q (oldest must be dropped)", last, strconv.Itoa(total-1))
	}
}

func TestTailRetriesThenGivesUpUntilResync(t *testing.T) {
	f := newFakeClient()
	f.tailFn = func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
		return errors.New("stream broke")
	}
	f.set(map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}, nil)
	h := startHub(t, func() engineClient { return f })

	rec := &recorder{}
	h.Subscribe(ContainerSpec{}, models.LogOptions{}, rec.sink)
	key := containerKey{"h1", "c1"}

	waitFor(t, "retries to reach the attempt cap", func() bool { return f.totalStarts(key) == maxTailAttempts })
	time.Sleep(30 * time.Millisecond)
	if n := f.totalStarts(key); n != maxTailAttempts {
		t.Fatalf("tail kept retrying after giving up: %d starts", n)
	}

	// The next resync (driven here by a poke) respawns the tail.
	waitFor(t, "resync to respawn the tail", func() bool {
		h.Reconcile()
		return f.totalStarts(key) >= 2*maxTailAttempts
	})
}

func TestShutdownDrainsBufferedRecordsAndWaitReturns(t *testing.T) {
	const total = 50
	pushed := make(chan struct{})
	f := newFakeClient()
	f.tailFn = func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
		for i := 0; i < total; i++ {
			emit(models.LogEntry{Message: strconv.Itoa(i)})
		}
		close(pushed)
		<-ctx.Done()
		return ctx.Err()
	}
	f.set(map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}, nil)

	h := newHub(nil, func() engineClient { return f })
	h.resyncInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	gate := make(chan struct{})
	rec := &recorder{}
	h.Subscribe(ContainerSpec{}, models.LogOptions{}, func(r Record) {
		<-gate
		rec.sink(r)
	})

	<-pushed // all records pushed; sink still blocked, so most sit in the buffer
	cancel()
	close(gate)
	waitShutdown(t, h)

	if got := rec.len(); got != total {
		t.Errorf("shutdown delivered %d buffered records, want %d", got, total)
	}
}

func TestTwoSubscribersGetIndependentTailsAndRecords(t *testing.T) {
	f := newFakeClient()
	f.tailFn = func(ctx context.Context, host, id string, opts models.LogOptions, emit func(models.LogEntry)) error {
		emit(models.LogEntry{Message: "from-" + id})
		<-ctx.Done()
		return ctx.Err()
	}
	f.set(map[string][]models.ContainerInfo{
		"h1": {ctr("c1", "web", "running", nil), ctr("c2", "db", "running", nil)},
	}, nil)
	h := startHub(t, func() engineClient { return f })

	recA, recB := &recorder{}, &recorder{}
	h.Subscribe(ContainerSpec{Containers: []string{"web"}}, models.LogOptions{Tail: "0"}, recA.sink)
	h.Subscribe(ContainerSpec{}, models.LogOptions{Tail: "100"}, recB.sink)

	k1, k2 := containerKey{"h1", "c1"}, containerKey{"h1", "c2"}
	// Overlapping specs still get one tail each: c1 is tailed twice.
	waitFor(t, "per-subscription tails", func() bool {
		return f.activeTails(k1) == 2 && f.activeTails(k2) == 1
	})
	waitFor(t, "records to fan out", func() bool { return recA.len() == 1 && recB.len() == 2 })

	if r := recA.all()[0]; r.ContainerID != "c1" {
		t.Errorf("subscriber A got record for %s, want c1", r.ContainerID)
	}
	seen := map[string]bool{}
	for _, r := range recB.all() {
		seen[r.ContainerID] = true
	}
	if !seen["c1"] || !seen["c2"] {
		t.Errorf("subscriber B records incomplete: %v", seen)
	}
}

func TestHotSwapReopensStreamAndRespawnsTails(t *testing.T) {
	snapshot := map[string][]models.ContainerInfo{"h1": {ctr("c1", "web", "running", nil)}}
	fakeA, fakeB := newFakeClient(), newFakeClient()
	fakeA.set(snapshot, nil)
	fakeB.set(snapshot, nil)

	var mu sync.Mutex
	current := fakeA
	h := startHub(t, func() engineClient {
		mu.Lock()
		defer mu.Unlock()
		return current
	})

	rec := &recorder{}
	h.Subscribe(ContainerSpec{}, models.LogOptions{}, rec.sink)
	key := containerKey{"h1", "c1"}
	waitFor(t, "tail on old client", func() bool { return fakeA.activeTails(key) == 1 })
	if n := fakeA.streamCount(); n != 1 {
		t.Fatalf("expected 1 event stream on old client, got %d", n)
	}

	// Swap the client (config reload) and poke, as main.go does.
	mu.Lock()
	current = fakeB
	mu.Unlock()
	h.Reconcile()
	waitFor(t, "event stream reopened on new client", func() bool { return fakeB.streamCount() == 1 })

	// Old client closes; its tail dies, retries fail, and resync respawns on
	// the new client.
	fakeA.close()
	waitFor(t, "tail respawned on new client", func() bool {
		h.Reconcile()
		return fakeB.activeTails(key) == 1
	})
	if fakeA.activeTails(key) != 0 {
		t.Error("old client's tail still running after close")
	}
}

func TestReconcileBeforeRunDoesNotBlock(t *testing.T) {
	h := newHub(nil, func() engineClient { return newFakeClient() })
	done := make(chan struct{})
	go func() {
		h.Reconcile()
		h.Reconcile() // coalesces; must never block
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Reconcile blocked before Run")
	}
}
