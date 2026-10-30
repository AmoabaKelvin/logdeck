package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/redact"
	"github.com/docker/docker/pkg/stdcopy"
)

func activateRedaction(tb testing.TB, cfg config.RedactionConfig) {
	tb.Helper()
	set, err := redact.Compile(cfg)
	if err != nil {
		tb.Fatal(err)
	}
	redact.Activate(set)
	tb.Cleanup(func() { redact.Activate(nil) })
}

func framed(tb testing.TB, lines ...string) []byte {
	tb.Helper()
	var stream bytes.Buffer
	stdout := stdcopy.NewStdWriter(&stream, stdcopy.Stdout)
	for _, line := range lines {
		if _, err := stdout.Write([]byte(line + "\n")); err != nil {
			tb.Fatal(err)
		}
	}
	return stream.Bytes()
}

func TestTailLogStreamRedactsBeforeParsing(t *testing.T) {
	activateRedaction(t, config.RedactionConfig{Enabled: true, Builtins: redact.DefaultBuiltins()})
	raw := framed(t, "2026-10-30T12:00:00.000000001Z ERROR login failed password=hunter2")

	var entries []models.LogEntry
	redactor := redact.NewStream(redact.Target{Host: "local", Name: "web"})
	if err := tailLogStream(context.Background(), io.NopCloser(bytes.NewReader(raw)), false, redactor, func(e models.LogEntry) {
		entries = append(entries, e)
	}); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Raw != "2026-10-30T12:00:00.000000001Z ERROR login failed password=[REDACTED]" {
		t.Fatalf("raw = %q", e.Raw)
	}
	if e.Message != "ERROR login failed password=[REDACTED]" || e.Level != models.LogLevelError || e.Timestamp.IsZero() {
		t.Fatalf("entry = %+v", e)
	}
}

func TestGroupedEntryIsRedactedInEveryRepresentation(t *testing.T) {
	activateRedaction(t, config.RedactionConfig{Enabled: true, Builtins: redact.DefaultBuiltins()})
	raw := framed(t,
		"2026-10-30T12:00:00.000000001Z ERROR request failed",
		"2026-10-30T12:00:00.000000002Z   token: abc123",
	)

	redactor := redact.NewStream(redact.Target{Host: "local", Name: "web"})
	entries, err := parseDockerLogs(bytes.NewReader(raw), false, "", nil, redactor)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 grouped entry", len(entries))
	}
	e := entries[0]
	for name, value := range map[string]string{"raw": e.Raw, "message": e.Message, "field": e.Fields["token"]} {
		if strings.Contains(value, "abc123") {
			t.Fatalf("%s leaks the secret: %q", name, value)
		}
	}
	if e.Fields["token"] != "[REDACTED]" {
		t.Fatalf("fields = %v", e.Fields)
	}
}

// BenchmarkTailLogStream measures the path every stored and alerted line
// takes: demultiplexing, redaction and parsing. "clean" lines hold no
// secrets; in "mixed" one line in five does.
//
//   - baseline: no redactor at all, as before redaction existed.
//   - off: the production state with redaction disabled.
//   - other-container: enabled, but the only rule targets another container.
//   - on: the default built-ins.
func BenchmarkTailLogStream(b *testing.B) {
	clean := []string{
		`2026-10-30T12:00:00.%09dZ 172.18.0.1 - - [30/Oct/2026:12:00:00 +0000] "GET /api/v1/containers HTTP/1.1" 200 5123 "-" "Mozilla/5.0"`,
		`2026-10-30T12:00:00.%09dZ {"level":"info","ts":"2026-10-30T12:00:00Z","msg":"request handled","method":"GET","path":"/healthz","status":200,"duration_ms":3}`,
		`2026-10-30T12:00:00.%09dZ time="2026-10-30T12:00:00Z" level=warning msg="slow query" duration=1.2s table=orders`,
		`2026-10-30T12:00:00.%09dZ [INFO] worker 7 picked up job 81231 from queue default`,
		`2026-10-30T12:00:00.%09dZ ERROR failed to connect to upstream: connection refused`,
		`2026-10-30T12:00:00.%09dZ DEBUG cache hit key=user:4211 ttl=300`,
		`2026-10-30T12:00:00.%09dZ INFO 2026/10/30 12:00:00 server listening on :8080`,
		`2026-10-30T12:00:00.%09dZ    at com.example.Service.handle(Service.java:42)`,
	}
	secrets := []string{
		`2026-10-30T12:00:00.%09dZ INFO login ok user=jane password=hunter2 session=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2lnbmF0dXJl`,
		`2026-10-30T12:00:00.%09dZ GET /oauth/callback Authorization: Bearer abcdef0123456789`,
	}
	const lines = 1000
	stream := func(templates []string) []byte {
		all := make([]string, lines)
		for i := range all {
			all[i] = fmt.Sprintf(templates[i%len(templates)], i)
		}
		return framed(b, all...)
	}
	cleanRaw := stream(clean)
	mixedRaw := stream(append(clean, secrets...))

	run := func(b *testing.B, raw []byte, redactor func() *redact.Stream) {
		b.ReportAllocs()
		b.SetBytes(int64(len(raw)))
		for b.Loop() {
			err := tailLogStream(context.Background(), io.NopCloser(bytes.NewReader(raw)), false, redactor(), func(models.LogEntry) {})
			if err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*lines), "ns/line")
	}
	target := redact.Target{Host: "local", Name: "web"}
	newStream := func() *redact.Stream { return redact.NewStream(target) }

	for _, corpus := range []struct {
		name string
		raw  []byte
	}{{"clean", cleanRaw}, {"mixed", mixedRaw}} {
		b.Run(corpus.name+"/baseline", func(b *testing.B) {
			run(b, corpus.raw, func() *redact.Stream { return nil })
		})
		b.Run(corpus.name+"/off", func(b *testing.B) {
			redact.Activate(nil)
			run(b, corpus.raw, newStream)
		})
		b.Run(corpus.name+"/other-container", func(b *testing.B) {
			activateRedaction(b, config.RedactionConfig{Enabled: true, Rules: []config.RedactionRule{
				{Name: "db", Enabled: true, Pattern: `secret`, Containers: []string{"db"}},
			}})
			run(b, corpus.raw, newStream)
		})
		b.Run(corpus.name+"/on", func(b *testing.B) {
			activateRedaction(b, config.RedactionConfig{Enabled: true, Builtins: redact.DefaultBuiltins()})
			run(b, corpus.raw, newStream)
		})
	}
}
