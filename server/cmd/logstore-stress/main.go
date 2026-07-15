// Command logstore-stress exercises the real internal/logstore.Store through
// its actual live ingestion path at high, sustained write rates and reports
// where it slows or drops. It drives the store's unexported sink directly (via
// a capturing fake hub) while the real writeLoop, batching, dedup, watermarks,
// and janitor run, and while representative queries run concurrently.
//
// It changes no store behavior. It measures. Output is machine-readable JSON on
// stdout plus a human summary on stderr.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const stressHost = "stress"

// runConfig is the fully-resolved harness configuration (scenario preset with
// any explicit flag overrides applied).
type runConfig struct {
	Scenario       string        `json:"scenario"`
	Containers     int           `json:"containers"`
	RateLinesPerS  int           `json:"rateLinesPerSec"` // 0 == flood
	Flood          bool          `json:"flood"`
	Duration       time.Duration `json:"-"`
	DurationSec    float64       `json:"durationSec"`
	PerContainerMB int           `json:"perContainerMB"`
	TotalMB        int           `json:"totalMB"`
	LineBytes      int           `json:"lineBytes"`
	QueryWorkers   int           `json:"queryWorkers"`
}

// scenarioPreset is the default parameter set for a named scenario.
type scenarioPreset struct {
	containers   int
	rate         int
	flood        bool
	duration     time.Duration
	perMB        int
	totalMB      int
	lineBytes    int
	queryWorkers int
}

var presets = map[string]scenarioPreset{
	// The "chatty stack" baseline: expect zero drops, db plateaus at cap, flat memory.
	"steady": {containers: 12, rate: 1000, duration: 3 * time.Minute, perMB: 50, totalMB: 1024, lineBytes: 150},
	// One container, unbounded: find the max sustained commit rate and drop behavior.
	"flood": {containers: 1, rate: 0, flood: true, duration: 30 * time.Second, perMB: 50, totalMB: 1024, lineBytes: 150},
	// Tiny caps + high rate: the janitor evicts continuously; watch for throughput dips / contention.
	"retention-churn": {containers: 8, rate: 20000, duration: 2 * time.Minute, perMB: 1, totalMB: 5, lineBytes: 150},
	// Steady ingest with a concurrent query workload: report query p50/p95/p99.
	"query-under-load": {containers: 12, rate: 1000, duration: 2 * time.Minute, perMB: 50, totalMB: 1024, lineBytes: 150, queryWorkers: 4},
	// Breadth: many containers at a moderate aggregate rate.
	"many-containers": {containers: 100, rate: 5000, duration: 2 * time.Minute, perMB: 20, totalMB: 1024, lineBytes: 150},
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, "logstore-stress:", err)
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "logstore-stress:", err)
		os.Exit(1)
	}
}

func parseFlags() (runConfig, error) {
	var (
		scenario     = flag.String("scenario", "steady", "preset: steady|flood|retention-churn|query-under-load|many-containers")
		containers   = flag.Int("containers", -1, "number of synthetic containers (overrides scenario)")
		rate         = flag.Int("rate", -1, "aggregate lines/sec; 0 = flood (overrides scenario)")
		duration     = flag.Duration("duration", 0, "run duration (overrides scenario)")
		perMB        = flag.Int("per-container-mb", -1, "per-container retention cap in MB (overrides scenario)")
		totalMB      = flag.Int("total-mb", -1, "total retention cap in MB (overrides scenario)")
		lineBytes    = flag.Int("line-bytes", -1, "approximate stored bytes per line (overrides scenario)")
		queryWorkers = flag.Int("query-workers", -1, "concurrent query goroutines (overrides scenario)")
	)
	flag.Parse()

	preset, ok := presets[*scenario]
	if !ok {
		return runConfig{}, fmt.Errorf("unknown scenario %q", *scenario)
	}

	cfg := runConfig{
		Scenario:       *scenario,
		Containers:     preset.containers,
		RateLinesPerS:  preset.rate,
		Duration:       preset.duration,
		PerContainerMB: preset.perMB,
		TotalMB:        preset.totalMB,
		LineBytes:      preset.lineBytes,
		QueryWorkers:   preset.queryWorkers,
	}
	if *containers >= 0 {
		cfg.Containers = *containers
	}
	if *rate >= 0 {
		cfg.RateLinesPerS = *rate
	}
	if *duration > 0 {
		cfg.Duration = *duration
	}
	if *perMB >= 0 {
		cfg.PerContainerMB = *perMB
	}
	if *totalMB >= 0 {
		cfg.TotalMB = *totalMB
	}
	if *lineBytes >= 0 {
		cfg.LineBytes = *lineBytes
	}
	if *queryWorkers >= 0 {
		cfg.QueryWorkers = *queryWorkers
	}

	cfg.Flood = cfg.RateLinesPerS == 0
	cfg.DurationSec = cfg.Duration.Seconds()

	if cfg.Containers < 1 {
		return runConfig{}, fmt.Errorf("containers must be >= 1, got %d", cfg.Containers)
	}
	if cfg.LineBytes < 40 {
		return runConfig{}, fmt.Errorf("line-bytes must be >= 40, got %d", cfg.LineBytes)
	}
	return cfg, nil
}

// --- pipeline fakes ---------------------------------------------------------

// captureHub satisfies logstore.Hub: it records the sink the store hands it so
// the harness can drive live ingestion directly.
type captureHub struct {
	mu   sync.Mutex
	sink func(logstream.Record)
}

func (h *captureHub) Subscribe(_ logstream.ContainerSpec, _ models.LogOptions, sink func(logstream.Record)) func() {
	h.mu.Lock()
	h.sink = sink
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.sink = nil
		h.mu.Unlock()
	}
}

func (h *captureHub) get() func(logstream.Record) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sink
}

// emptyProvider satisfies logstore.DockerProvider with a real MultiHostClient
// that has no hosts, so the store's lifecycle poll lists nothing and never
// tails — the harness feeds the sink directly.
type emptyProvider struct{ client *docker.MultiHostClient }

func (p emptyProvider) Docker() *docker.MultiHostClient { return p.client }

// --- metrics ----------------------------------------------------------------

// sample is one point-in-time reading of the store under load. CommittedLines
// is cumulative (monotonic) so the derived commit rate is true ingestion, never
// negative when retention evicts; RetainedLines is the live row count, which
// falls as the janitor sweeps. RetainedLines is -1 when its query failed.
type sample struct {
	TSec              float64 `json:"tSec"`
	OfferedLines      uint64  `json:"offeredLines"`
	CommittedLines    int64   `json:"committedLines"`
	RetainedLines     int64   `json:"retainedLines"`
	DroppedLines      uint64  `json:"droppedLines"`
	OfferedRatePerSec float64 `json:"offeredRatePerSec"`
	CommitRatePerSec  float64 `json:"commitRatePerSec"`
	DBBytes           int64   `json:"dbBytes"`
	HeapAllocBytes    uint64  `json:"heapAllocBytes"`
	HeapInuseBytes    uint64  `json:"heapInuseBytes"`
	Goroutines        int     `json:"goroutines"`
}

// queryStats summarizes the concurrent query workload.
type queryStats struct {
	Count  int     `json:"count"`
	Errors int     `json:"errors"`
	P50Ms  float64 `json:"p50Ms"`
	P95Ms  float64 `json:"p95Ms"`
	P99Ms  float64 `json:"p99Ms"`
	MaxMs  float64 `json:"maxMs"`
}

// report is the full machine-readable result.
type report struct {
	Config          runConfig   `json:"config"`
	OfferedLines    uint64      `json:"offeredLines"`
	CommittedLines  int64       `json:"committedLines"`
	RetainedLines   int64       `json:"retainedLines"`
	SampleErrors    int         `json:"sampleErrors"`
	DroppedLines    uint64      `json:"droppedLines"`
	OfferedRate     float64     `json:"offeredRateLinesPerSec"`
	CommitRate      float64     `json:"committedRateLinesPerSec"`
	DropFraction    float64     `json:"dropFraction"`
	DBBytesFinal    int64       `json:"dbBytesFinal"`
	DBBytesPeak     int64       `json:"dbBytesPeak"`
	TotalCapBytes   int64       `json:"totalCapBytes"`
	HeapAllocPeak   uint64      `json:"heapAllocPeakBytes"`
	HeapInusePeak   uint64      `json:"heapInusePeakBytes"`
	HeapAllocFirst  uint64      `json:"heapAllocFirstBytes"`
	HeapAllocLast   uint64      `json:"heapAllocLastBytes"`
	GoroutinesPeak  int         `json:"goroutinesPeak"`
	GoroutinesFirst int         `json:"goroutinesFirst"`
	GoroutinesLast  int         `json:"goroutinesLast"`
	Query           *queryStats `json:"query"`
	Samples         []sample    `json:"samples"`
}

// --- run --------------------------------------------------------------------

func run(cfg runConfig) error {
	tmpDir, err := os.MkdirTemp("", "logstore-stress")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	dbPath := filepath.Join(tmpDir, "logs.db")

	limits := func() config.ResolvedLogStoreConfig {
		return config.ResolvedLogStoreConfig{
			Enabled:        true,
			PerContainerMB: cfg.PerContainerMB,
			TotalMB:        cfg.TotalMB,
		}
	}
	store, err := logstore.Open(dbPath, limits)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	client, err := docker.NewMultiHostClient(nil)
	if err != nil {
		return fmt.Errorf("empty docker client: %w", err)
	}
	hub := &captureHub{}

	// storeCtx keeps the store's loops (writeLoop, syncLoop, janitorLoop) alive
	// for the whole run; cancelling it triggers the final flush.
	storeCtx, storeCancel := context.WithCancel(context.Background())
	store.Start(storeCtx, hub, emptyProvider{client: client})

	sink := hub.get()
	if sink == nil {
		storeCancel()
		return fmt.Errorf("store did not subscribe a sink")
	}

	var offered atomic.Uint64
	emit := func(rec logstream.Record) {
		sink(rec)
		offered.Add(1)
	}

	// genCtx bounds the ingest + query window.
	genCtx, genCancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer genCancel()

	start := time.Now()
	var (
		samples      []sample
		sampleErrors int
		qLatMu       sync.Mutex
		qLatency     []int64
		qErrors      atomic.Int64
		workersWG    sync.WaitGroup
	)

	// Metrics sampler: one reading per second plus a final reading.
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var last sample
		haveLast := false
		record := func() {
			s, err := readSample(store, dbPath, start, &offered)
			if err != nil {
				sampleErrors++
			}
			if haveLast {
				dt := s.TSec - last.TSec
				if dt > 0 {
					s.OfferedRatePerSec = float64(s.OfferedLines-last.OfferedLines) / dt
					s.CommitRatePerSec = float64(s.CommittedLines-last.CommittedLines) / dt
				}
			}
			samples = append(samples, s)
			last, haveLast = s, true
		}
		for {
			select {
			case <-genCtx.Done():
				record() // final in-window reading
				return
			case <-ticker.C:
				record()
			}
		}
	}()

	// Query workers: representative reads concurrent with heavy ingestion.
	for i := 0; i < cfg.QueryWorkers; i++ {
		workersWG.Add(1)
		go func(seed int64) {
			defer workersWG.Done()
			rng := rand.New(rand.NewSource(seed))
			for genCtx.Err() == nil {
				q := pickQuery(rng, cfg.Containers)
				t0 := time.Now()
				_, err := store.Query(genCtx, q)
				d := time.Since(t0)
				if err != nil {
					if genCtx.Err() == nil {
						qErrors.Add(1)
					}
					continue
				}
				qLatMu.Lock()
				qLatency = append(qLatency, int64(d))
				qLatMu.Unlock()
				// A real history viewer is not a tight loop; this also bounds the
				// sample count on long runs.
				time.Sleep(5 * time.Millisecond)
			}
		}(int64(i) + 1)
	}

	// Generators drive the sink until genCtx expires.
	runGenerators(genCtx, cfg, emit)

	// Ingest window closed: stop query workers and the sampler.
	genCancel()
	workersWG.Wait()
	<-samplerDone
	ingestElapsed := time.Since(start).Seconds()

	// Drain: cancel the store so writeLoop flushes the last batch, then wait.
	storeCancel()
	store.Wait()

	committed := store.Committed()
	retained, err := store.CountLines(context.Background())
	if err != nil {
		return fmt.Errorf("final retained count: %w", err)
	}

	rep := buildReport(cfg, samples, offered.Load(), committed, retained, sampleErrors,
		store.Drops(), dbSize(dbPath), ingestElapsed, qLatency, int(qErrors.Load()))

	if err := emitJSON(rep); err != nil {
		return err
	}
	writeHumanSummary(rep)
	return nil
}

// readSample takes one instantaneous reading of the store and process. The
// error is non-nil when the retained-row query failed, in which case
// RetainedLines is -1 so a transient failure reads as invalid, not as zero.
func readSample(store *logstore.Store, dbPath string, start time.Time, offered *atomic.Uint64) (sample, error) {
	retained, err := store.CountLines(context.Background())
	if err != nil {
		retained = -1
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return sample{
		TSec:           time.Since(start).Seconds(),
		OfferedLines:   offered.Load(),
		CommittedLines: store.Committed(),
		RetainedLines:  retained,
		DroppedLines:   store.Drops(),
		DBBytes:        dbSize(dbPath),
		HeapAllocBytes: mem.HeapAlloc,
		HeapInuseBytes: mem.HeapInuse,
		Goroutines:     runtime.NumGoroutine(),
	}, err
}

// runGenerators drives synthetic records into the sink at the configured rate,
// or as fast as possible in flood mode, until ctx expires.
func runGenerators(ctx context.Context, cfg runConfig, emit func(logstream.Record)) {
	var seq atomic.Uint64
	next := func() logstream.Record {
		n := seq.Add(1) - 1
		return makeRecord(n%uint64(cfg.Containers), n, cfg.LineBytes)
	}

	if cfg.Flood {
		// One busy producer per container; the sink drops when the writer falls
		// behind, so producers never block and we find the commit ceiling.
		var wg sync.WaitGroup
		for c := 0; c < cfg.Containers; c++ {
			wg.Add(1)
			go func(container int) {
				defer wg.Done()
				for ctx.Err() == nil {
					// seq.Add is globally unique, keeping (ts, stream, raw) unique
					// across all producers so the store's dedup never fires.
					emit(makeRecord(uint64(container), seq.Add(1), cfg.LineBytes))
				}
			}(c)
		}
		wg.Wait()
		return
	}

	// Paced mode: hold the configured aggregate rate on a 200Hz tick. The count
	// per tick comes from a cumulative target (rate*tick/tickHz), not rate/tickHz,
	// so the integer remainder carries across ticks instead of being truncated —
	// 1001/s stays 1001, and rates below tickHz stay exact rather than snapping up.
	const tickHz = 200
	ticker := time.NewTicker(time.Second / tickHz)
	defer ticker.Stop()
	var tick, emitted int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick++
			target := int64(cfg.RateLinesPerS) * tick / tickHz
			for ; emitted < target; emitted++ {
				emit(next())
			}
		}
	}
}

// levelBodies are pre-rendered log bodies carrying a level keyword the store's
// classifier and the query filters both recognize.
type levelBody struct {
	level  models.LogLevel
	stream string
	body   string
}

// makeRecord builds one synthetic live record for a container. The timestamp is
// wall-clock (so bounded-window queries are meaningful) and the sequence number
// keeps (ts, stream, raw) unique, so the store's insert dedup never
// under-counts a genuinely new line.
func makeRecord(container, seq uint64, lineBytes int) logstream.Record {
	lb := pickLevelBody(seq)
	ts := time.Now()
	prefix := ts.UTC().Format(time.RFC3339Nano)
	// prefix + space + "level=xxxx seq=<n> " + body, padded to lineBytes.
	head := prefix + " " + string(lb.level) + " seq=" + itoa(seq) + " " + lb.body
	raw := padTo(head, lineBytes)

	return logstream.Record{
		Host:          stressHost,
		ContainerID:   containerID(container),
		ContainerName: containerName(container),
		Labels:        map[string]string{"com.docker.compose.project": "stressstack"},
		Entry: models.LogEntry{
			Timestamp: ts,
			Level:     lb.level,
			Stream:    lb.stream,
			Raw:       raw,
		},
	}
}

// pickLevelBody returns a level/stream/body for a sequence number with a
// realistic mix: ~70% info, ~15% warn, ~10% error, ~5% debug. Errors and
// warnings go to stderr.
func pickLevelBody(seq uint64) levelBody {
	switch seq % 20 {
	case 0, 1:
		return levelBody{models.LogLevelError, "stderr", "error request failed handler=api status=500"}
	case 2:
		return levelBody{models.LogLevelDebug, "stdout", "debug cache lookup key=session hit=false"}
	case 3, 4, 5:
		return levelBody{models.LogLevelWarn, "stderr", "warn slow request handled latency=812ms"}
	default:
		return levelBody{models.LogLevelInfo, "stdout", "info request handled method=GET path=/api/v1/things"}
	}
}

// padTo pads s with a filler suffix so its byte length is at least n.
func padTo(s string, n int) string {
	if len(s) >= n {
		return s
	}
	pad := make([]byte, n-len(s))
	for i := range pad {
		pad[i] = 'x'
	}
	return s + " " + string(pad[1:])
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func containerID(c uint64) string   { return "gen-" + itoa(c) }
func containerName(c uint64) string { return "svc-" + itoa(c) }

// pickQuery rotates through representative reads: a recent page, a level
// filter, a bounded-window substring search, and a bounded-window regex search.
func pickQuery(rng *rand.Rand, containers int) logstore.LogQuery {
	name := containerName(uint64(rng.Intn(containers)))
	base := logstore.LogQuery{Host: stressHost, Container: name}
	switch rng.Intn(4) {
	case 0:
		base.Limit = 500 // recent window
	case 1:
		base.Levels = []string{"ERROR"}
		base.Limit = 200
	case 2:
		base.Since = time.Now().Add(-30 * time.Second)
		base.Search = "request"
		base.Limit = 200
	default:
		base.Since = time.Now().Add(-30 * time.Second)
		base.Search = "req.*handled"
		base.Regex = true
		base.Limit = 200
	}
	return base
}

// --- reporting --------------------------------------------------------------

func buildReport(cfg runConfig, samples []sample, offered uint64, committed, retained int64,
	sampleErrors int, drops uint64, dbFinal int64, elapsed float64, qLatency []int64, qErrors int) report {

	rep := report{
		Config:         cfg,
		OfferedLines:   offered,
		CommittedLines: committed,
		RetainedLines:  retained,
		SampleErrors:   sampleErrors,
		DroppedLines:   drops,
		DBBytesFinal:   dbFinal,
		TotalCapBytes:  int64(cfg.TotalMB) * 1024 * 1024,
		Samples:        samples,
	}
	if elapsed > 0 {
		rep.OfferedRate = float64(offered) / elapsed
		rep.CommitRate = float64(committed) / elapsed
	}
	if offered > 0 {
		rep.DropFraction = float64(drops) / float64(offered)
	}

	for i, s := range samples {
		if s.DBBytes > rep.DBBytesPeak {
			rep.DBBytesPeak = s.DBBytes
		}
		if s.HeapAllocBytes > rep.HeapAllocPeak {
			rep.HeapAllocPeak = s.HeapAllocBytes
		}
		if s.HeapInuseBytes > rep.HeapInusePeak {
			rep.HeapInusePeak = s.HeapInuseBytes
		}
		if s.Goroutines > rep.GoroutinesPeak {
			rep.GoroutinesPeak = s.Goroutines
		}
		if i == 0 {
			rep.HeapAllocFirst = s.HeapAllocBytes
			rep.GoroutinesFirst = s.Goroutines
		}
		rep.HeapAllocLast = s.HeapAllocBytes
		rep.GoroutinesLast = s.Goroutines
	}
	if dbFinal > rep.DBBytesPeak {
		rep.DBBytesPeak = dbFinal
	}
	if cfg.QueryWorkers > 0 {
		rep.Query = summarizeQueries(qLatency, qErrors)
	}
	return rep
}

func summarizeQueries(latency []int64, errors int) *queryStats {
	qs := &queryStats{Count: len(latency), Errors: errors}
	if len(latency) == 0 {
		return qs
	}
	sort.Slice(latency, func(i, j int) bool { return latency[i] < latency[j] })
	qs.P50Ms = ms(percentile(latency, 0.50))
	qs.P95Ms = ms(percentile(latency, 0.95))
	qs.P99Ms = ms(percentile(latency, 0.99))
	qs.MaxMs = ms(latency[len(latency)-1])
	return qs
}

// percentile returns the p-quantile of a sorted slice by the nearest-rank
// method: rank ceil(p*N) taken 1-based, so the slice index is ceil(p*N)-1.
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ms(ns int64) float64 { return float64(ns) / 1e6 }

// dbSize sums the SQLite main file and its WAL/SHM sidecars.
func dbSize(dbPath string) int64 {
	var total int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if info, err := os.Stat(dbPath + suffix); err == nil {
			total += info.Size()
		}
	}
	return total
}

func emitJSON(rep report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if _, err := os.Stdout.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

func writeHumanSummary(rep report) {
	w := os.Stderr
	p := func(format string, args ...any) { fmt.Fprintf(w, format, args...) }

	p("\n=== logstore-stress: %s ===\n", rep.Config.Scenario)
	p("containers=%d rate=%s duration=%.0fs line-bytes=%d caps=%d/%dMB queryWorkers=%d\n",
		rep.Config.Containers, rateLabel(rep.Config), rep.Config.DurationSec,
		rep.Config.LineBytes, rep.Config.PerContainerMB, rep.Config.TotalMB, rep.Config.QueryWorkers)
	p("offered=%d committed=%d retained=%d dropped=%d (%.2f%% dropped)\n",
		rep.OfferedLines, rep.CommittedLines, rep.RetainedLines, rep.DroppedLines, rep.DropFraction*100)
	if rep.SampleErrors > 0 {
		p("WARNING: %d retained-count sample(s) failed\n", rep.SampleErrors)
	}
	p("offered rate=%.0f lines/s   committed rate=%.0f lines/s\n", rep.OfferedRate, rep.CommitRate)
	p("db size: final=%.1fMB peak=%.1fMB   total cap=%.1fMB\n",
		mb(rep.DBBytesFinal), mb(rep.DBBytesPeak), mb(rep.TotalCapBytes))
	p("heap alloc: first=%.1fMB last=%.1fMB peak=%.1fMB\n",
		mb(int64(rep.HeapAllocFirst)), mb(int64(rep.HeapAllocLast)), mb(int64(rep.HeapAllocPeak)))
	p("goroutines: first=%d last=%d peak=%d\n", rep.GoroutinesFirst, rep.GoroutinesLast, rep.GoroutinesPeak)
	if rep.Query != nil {
		p("query latency: n=%d errors=%d  p50=%.2fms p95=%.2fms p99=%.2fms max=%.2fms\n",
			rep.Query.Count, rep.Query.Errors, rep.Query.P50Ms, rep.Query.P95Ms, rep.Query.P99Ms, rep.Query.MaxMs)
	}
	p("samples=%d (1/s)\n", len(rep.Samples))
}

func rateLabel(cfg runConfig) string {
	if cfg.Flood {
		return "flood"
	}
	return itoa(uint64(cfg.RateLinesPerS)) + "/s"
}

func mb(bytes int64) float64 { return float64(bytes) / (1024 * 1024) }
