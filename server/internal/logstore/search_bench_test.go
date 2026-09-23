package logstore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// benchStore opens (building once) a store of benchContainers containers
// with uneven traffic, about three million lines in all. Each container ends
// with a partly filled hot table above its sealed blocks. LOGSTORE_BENCH_DB
// reuses a built file.
const (
	benchContainers = 30
	benchSteps      = 220_000
)

func benchStore(b *testing.B) *Store {
	path := os.Getenv("LOGSTORE_BENCH_DB")
	if path == "" {
		path = b.TempDir() + "/logs.db"
	}
	_, statErr := os.Stat(path)
	s, err := Open(path, testLimits)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	if statErr == nil {
		return s
	}

	state := newWriterState()
	batch := make([]ingestMsg, 0, batchLines)
	messages := []string{
		"INFO request handled method=GET path=/api/v1/items status=200 dur=%dms",
		"DEBUG cache lookup key=user:%d hit=true",
		`{"level":"warn","msg":"slow query","table":"orders","ms":%d}`,
		"WARN slow query took %dms table=orders",
		"ERROR failed to process job %d: timeout",
		"    at com.example.Worker.run(Worker.java:%d)",
	}
	lines := make([]int, benchContainers)  // lines written per container
	traces := make([]int, benchContainers) // stack frames still to write
	for n := range benchSteps {
		for c := range benchContainers {
			// Container c logs on every (1 + c%5)th step.
			if n%(1+c%5) != 0 {
				continue
			}
			k := lines[c]
			lines[c]++
			var msg string
			switch {
			case c == 7 && k%15_000 == 0:
				msg = "ERROR dial tcp 10.0.0.5:5432: connect: connection refused"
			case traces[c] > 0:
				traces[c]--
				msg = fmt.Sprintf("    at com.example.Handler.step%d(Handler.java:%d)", traces[c], k)
			case k%500 == 499:
				traces[c] = 20
				msg = fmt.Sprintf("ERROR unhandled exception in request %d", k)
			default:
				msg = fmt.Sprintf(messages[k%len(messages)], k)
			}
			ts := baseTime.Add(time.Duration(n)*time.Millisecond + time.Duration(c%3)*time.Microsecond)
			raw := ts.Format(fixedNanoLayout) + " " + msg
			batch = append(batch, ingestMsg{
				key: genKey{"local", fmt.Sprint("id", c)}, name: fmt.Sprint("svc-", c), project: fmt.Sprint("proj", c%3),
				line: lineFromEntry(models.ParseLogLine(raw, "stdout")),
			})
			if len(batch) == batchLines {
				if err := s.commit(batch, state); err != nil {
					b.Fatal(err)
				}
				if err := s.sealFullBlocks(state); err != nil {
					b.Fatal(err)
				}
				batch = batch[:0]
			}
		}
	}
	if err := s.commit(batch, state); err != nil {
		b.Fatal(err)
	}
	return s
}

func BenchmarkQueryRareNeedleOneContainer(b *testing.B) {
	s := benchStore(b)
	b.ResetTimer()
	for range b.N {
		page, err := s.Query(context.Background(), LogQuery{Container: "svc-7", Search: "connection refused"})
		if err != nil {
			b.Fatal(err)
		}
		if len(page.Entries) != 5 {
			b.Fatalf("got %d entries", len(page.Entries))
		}
	}
}

func BenchmarkQueryFirstPage(b *testing.B) {
	s := benchStore(b)
	for _, q := range []LogQuery{
		{Container: "svc-7"},
		{Container: "svc-7", Levels: []string{"ERROR"}},
	} {
		b.Run(fmt.Sprint(len(q.Levels)), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				page, err := s.Query(context.Background(), q)
				if err != nil || len(page.Entries) != DefaultQueryLimit {
					b.Fatal(err, len(page.Entries))
				}
			}
		})
	}
}

// Follows cursors to the end of history, as a client paging through would.
func BenchmarkSearchRareNeedleAllContainers(b *testing.B) {
	s := benchStore(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		q := LogQuery{Search: "connection refused"}
		found, pages := 0, 0
		for {
			page, err := s.Query(context.Background(), q)
			if err != nil {
				b.Fatal(err)
			}
			found += len(page.Entries)
			pages++
			if page.NextCursor == "" {
				break
			}
			q.Cursor = page.NextCursor
		}
		if found != 5 {
			b.Fatalf("found %d", found)
		}
		b.ReportMetric(float64(pages), "requests")
	}
}

func BenchmarkSearchFirstPage(b *testing.B) {
	s := benchStore(b)
	b.ResetTimer()
	since := baseTime.Add(time.Minute)
	for name, q := range map[string]LogQuery{
		"all":           {},
		"errors":        {Levels: []string{"ERROR"}},
		"window-needle": {Search: "connection refused", Since: since, Until: since.Add(100 * time.Second)},
		"project":       {Project: "proj1", Levels: []string{"WARN"}},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := s.Query(context.Background(), q); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Reads another container while the writer commits and seals a busy one, on
// a copy of the fixture so the shared file is left as built. Compare with
// BenchmarkQueryFirstPage/1.
func BenchmarkSearchUnderIngest(b *testing.B) {
	path := b.TempDir() + "/logs.db"
	if _, err := benchStore(b).db.Exec("VACUUM INTO ?", path); err != nil {
		b.Fatal(err)
	}
	s, err := Open(path, testLimits)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	state, err := s.loadWriterState()
	if err != nil {
		b.Fatal(err)
	}

	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ts := baseTime.Add(benchSteps * time.Millisecond)
		batch := make([]ingestMsg, batchLines)
		for {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			for i := range batch {
				ts = ts.Add(time.Microsecond)
				raw := ts.Format(fixedNanoLayout) + " INFO live request handled"
				batch[i] = ingestMsg{key: genKey{"local", "live"}, name: "live",
					line: lineFromEntry(models.ParseLogLine(raw, "stdout"))}
			}
			if err := s.commit(batch, state); err != nil {
				done <- err
				return
			}
			if err := s.sealFullBlocks(state); err != nil {
				done <- err
				return
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.Query(context.Background(), LogQuery{Container: "svc-7", Levels: []string{"ERROR"}}); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(stop)
	if err := <-done; err != nil {
		b.Fatal(err)
	}
}
