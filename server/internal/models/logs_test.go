package models

import (
	"strings"
	"testing"
	"time"
)

func TestDetectLogLevelUsesExplicitStructuredLevels(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    LogLevel
	}{
		{
			name:    "json string level",
			message: `{"level":"error","time":1716872438,"msg":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "pino numeric level",
			message: `{"level":30,"time":1716872438,"msg":"request completed"}`,
			want:    LogLevelInfo,
		},
		{
			name:    "otel severity text",
			message: `{"severity_text":"WARN","body":"retrying request"}`,
			want:    LogLevelWarn,
		},
		{
			name:    "otel camel case severity text",
			message: `{"severityText":"ERROR","body":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "otel numeric severity",
			message: `{"severityNumber":17,"body":"request failed"}`,
			want:    LogLevelError,
		},
		{
			name:    "logfmt level",
			message: `time=2026-05-28T05:00:38Z level=debug msg="cache hit"`,
			want:    LogLevelDebug,
		},
		{
			name:    "python level name",
			message: `{"levelname":"WARNING","message":"queue latency high"}`,
			want:    LogLevelWarn,
		},
		{
			name:    "logfmt otel numeric severity",
			message: `time=2026-05-28T05:00:38Z severity_number=21 msg="service unavailable"`,
			want:    LogLevelFatal,
		},
		{
			name:    "slog uppercase level",
			message: `time=2026-05-28T05:00:38Z level=ERROR msg="request failed"`,
			want:    LogLevelError,
		},
		{
			name:    "bracketed prefix",
			message: `[FATAL] database migration failed`,
			want:    LogLevelFatal,
		},
		{
			name:    "plain prefix",
			message: `WARNING -- queue latency high`,
			want:    LogLevelWarn,
		},
		{
			name:    "glog prefix",
			message: `E0528 05:00:38.367891 request failed`,
			want:    LogLevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectLogLevel(tt.message); got != tt.want {
				t.Fatalf("DetectLogLevel(%q) = %s, want %s", tt.message, got, tt.want)
			}
		})
	}
}

func TestDetectLogLevelKeepsMessageFallbacks(t *testing.T) {
	tests := []struct {
		message string
		want    LogLevel
	}{
		{message: "request failed after retry", want: LogLevelError},
		{message: "worker emitted notice event", want: LogLevelInfo},
		{message: "request completed successfully", want: LogLevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if got := DetectLogLevel(tt.message); got != tt.want {
				t.Fatalf("DetectLogLevel(%q) = %s, want %s", tt.message, got, tt.want)
			}
		})
	}
}

func TestParseLogLineDetectsExplicitLevelAfterTimestamp(t *testing.T) {
	entry := ParseLogLine(`2026-05-28T05:00:38.367Z {"level":50,"msg":"request failed"}`, "stdout")

	if entry.Level != LogLevelError {
		t.Fatalf("expected ERROR from JSON level after Docker timestamp, got %s", entry.Level)
	}
}

func TestGroupRelatedLogEntriesFoldsStructuredFieldsIntoPreviousEntry(t *testing.T) {
	entries := []LogEntry{
		ParseLogLine("2026-05-28T05:00:38.367Z [2026-05-28 05:00:38.367 +0000] INFO: Request received", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z service: \"api\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z requestId: \"e675e98a-8c63-42a2-b23b-925a4846b0c4\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z method: \"GET\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.367Z path: \"/favicon.ico\"", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.368Z [2026-05-28 05:00:38.368 +0000] INFO: Request completed", "stdout"),
	}

	grouped := GroupRelatedLogEntries(entries)

	if len(grouped) != 2 {
		t.Fatalf("expected 2 grouped entries, got %d", len(grouped))
	}

	first := grouped[0]
	if first.Level != LogLevelInfo {
		t.Fatalf("expected first grouped entry to remain INFO, got %s", first.Level)
	}
	if first.ContinuationCount != 4 {
		t.Fatalf("expected 4 continuation lines, got %d", first.ContinuationCount)
	}
	if first.Fields["requestId"] != "\"e675e98a-8c63-42a2-b23b-925a4846b0c4\"" {
		t.Fatalf("expected requestId field to be captured, got %q", first.Fields["requestId"])
	}
	if !strings.Contains(first.Message, "Request received\nservice: \"api\"\nrequestId:") {
		t.Fatalf("expected continuation fields in message, got %q", first.Message)
	}

	if grouped[1].Level != LogLevelInfo || !strings.Contains(grouped[1].Message, "Request completed") {
		t.Fatalf("expected second grouped entry to be request completed INFO, got %#v", grouped[1])
	}
}

func TestGroupRelatedLogEntriesDoesNotFoldStandaloneUnknownMessages(t *testing.T) {
	entries := []LogEntry{
		ParseLogLine("2026-05-28T05:00:38.367Z INFO starting worker", "stdout"),
		ParseLogLine("2026-05-28T05:00:38.368Z worker ready on port 3000", "stdout"),
	}

	grouped := GroupRelatedLogEntries(entries)

	if len(grouped) != 2 {
		t.Fatalf("expected standalone unknown message to remain separate, got %d entries", len(grouped))
	}
	if grouped[1].Level != LogLevelUnknown {
		t.Fatalf("expected second entry to remain UNKNOWN, got %s", grouped[1].Level)
	}
}

func TestCleanMessageKeepsTextAfterLastCarriageReturn(t *testing.T) {
	got := CleanMessage("Batches:   0%|          | 0/1 [00:00<?, ?it/s]\rBatches: 100%|██████████| 1/1 [00:00<00:00, 44.89it/s]\r")
	if got != "Batches: 100%|██████████| 1/1 [00:00<00:00, 44.89it/s]" {
		t.Fatalf("expected final progress state, got %q", got)
	}
}

func TestGroupRelatedLogEntriesFoldsMultiLineTraces(t *testing.T) {
	lines := []string{
		"2026-09-14T13:51:07.813Z INFO request handled id=7",
		"2026-09-14T13:51:07.814Z Traceback (most recent call last):",
		"2026-09-14T13:51:07.814Z   File \"/app/main.py\", line 12, in <module>",
		"2026-09-14T13:51:07.814Z     main()",
		"2026-09-14T13:51:07.814Z ValueError: bad value 7",
		"2026-09-14T13:51:09.817Z ERROR Request failed",
		"2026-09-14T13:51:09.817Z java.lang.RuntimeException: boom",
		"2026-09-14T13:51:09.817Z \tat com.example.Main.run(Main.java:42)",
		"2026-09-14T13:51:09.817Z Caused by: java.lang.IllegalStateException: connection failed",
		"2026-09-14T13:51:09.817Z \t... 2 more",
		"2026-09-14T13:51:11.820Z panic: runtime error: index out of range [5] with length 3",
		"2026-09-14T13:51:11.820Z ",
		"2026-09-14T13:51:11.820Z goroutine 1 [running]:",
		"2026-09-14T13:51:11.820Z main.main()",
		"2026-09-14T13:51:11.820Z \t/app/main.go:12 +0x1d",
		"2026-09-14T13:51:11.821Z exit status 2",
	}
	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		entries = append(entries, ParseLogLine(line, "stderr"))
	}

	grouped := GroupRelatedLogEntries(entries)

	want := []struct {
		level LogLevel
		count int
	}{
		{LogLevelInfo, 0},
		{LogLevelError, 3}, // Traceback + File + main() + ValueError
		{LogLevelError, 4}, // ERROR + exception + at + Caused by + ... more
		{LogLevelPanic, 4}, // panic + blank + goroutine + frame + file
		{LogLevelUnknown, 0},
	}
	if len(grouped) != len(want) {
		for _, g := range grouped {
			t.Logf("%s cont=%d %q", g.Level, g.ContinuationCount, g.Message)
		}
		t.Fatalf("expected %d entries, got %d", len(want), len(grouped))
	}
	for i, w := range want {
		if grouped[i].Level != w.level || grouped[i].ContinuationCount != w.count {
			t.Fatalf("entry %d: expected %s cont=%d, got %s cont=%d (%q)", i, w.level, w.count, grouped[i].Level, grouped[i].ContinuationCount, grouped[i].Message)
		}
	}
}

func TestGroupRelatedLogEntriesFoldsUnstampedSpillOverBehindStampedLine(t *testing.T) {
	entries := []LogEntry{
		ParseLogLine("2026-09-14T12:53:29.888Z 2026-09-14 12:53:29.888 | WARNING  | feature_engineer:718 - Geocoding rows - this may take some time...", "stderr"),
		ParseLogLine("2026-09-14T12:53:29.933Z Geocoding batches:   0%|          | 0/1 [00:00<?, ?it/s]\rGeocoding batches: 100%|██████████| 1/1 [00:00<00:00, 22.58it/s]", "stderr"),
		ParseLogLine("2026-09-14T12:53:30.100Z [2026-09-14 12:53:30] INFO [main.predict] Prediction for None", "stdout"),
		ParseLogLine("2026-09-14T12:53:30.167Z 172.20.1.7:45532 - \"POST /v1/predict HTTP/1.1\" 200", "stdout"),
		// Far behind the last stamped line: a separate event.
		ParseLogLine("2026-09-14T12:54:47.090Z 172.20.1.7:49598 - \"POST / HTTP/1.1\" 405", "stdout"),
		ParseLogLine("2026-09-14T12:54:47.120Z 172.20.1.7:49598 - \"POST / HTTP/1.1\" 405", "stdout"),
	}

	grouped := GroupRelatedLogEntries(entries)

	if len(grouped) != 4 {
		for _, g := range grouped {
			t.Logf("%s cont=%d %q", g.Level, g.ContinuationCount, g.Message)
		}
		t.Fatalf("expected 4 entries, got %d", len(grouped))
	}
	if grouped[0].ContinuationCount != 1 || !strings.HasSuffix(grouped[0].Message, "22.58it/s]") {
		t.Fatalf("expected progress bar folded into WARNING in its final state, got %q", grouped[0].Message)
	}
	if grouped[1].ContinuationCount != 1 || grouped[1].Level != LogLevelInfo {
		t.Fatalf("expected access line folded into INFO, got %#v", grouped[1])
	}
	if grouped[2].ContinuationCount != 0 || grouped[3].ContinuationCount != 0 {
		t.Fatalf("expected late unstamped lines to stay separate, got %#v", grouped[2:])
	}
}

// parseTimestampScan is the exhaustive prefix scan ParseTimestamp falls back
// to; the fast path must agree with it on every input.
func parseTimestampScan(logLine string) (time.Time, string) {
	line := strings.TrimSpace(logLine)
	for i := min(len(line), 96); i >= 1; i-- {
		if ts, ok := tryParseTimestampCandidate(line[:i]); ok {
			return ts, strings.TrimLeft(strings.TrimSpace(line[i:]), ")]}> \t")
		}
	}
	return time.Time{}, line
}

func TestParseTimestampFastPathMatchesScan(t *testing.T) {
	lines := []string{
		"2026-09-15T12:55:38.123456789Z GET /healthz 200",
		"2026-09-15T12:55:38.123456789Z [2026-09-15 12:55:38] INFO ready",
		"2026-09-15T12:55:38Z",
		"[2026-09-15T12:55:38Z] bracketed",
		"2026-09-15T12:55:38Z ] stray bracket",
		"2026-09-15 12:55:38,123 INFO comma fraction",
		"2026-09-15 12:55:38 space separated",
		"10/Oct/2000:13:55:36 -0700 access log",
		"Mon Jan  2 15:04:05 2006 ansic",
		"Mon Jan 02 15:04:05 MST 2006 unix date",
		"2026-09-15T12:55:38+0000 no colon offset",
		"no timestamp at all",
		"12:55:38 time only",
		"",
		"   ",
		"2026-09-15T12:55:38Zmsg glued",
		"    indented stack frame",
	}
	for _, line := range lines {
		wantTS, wantRest := parseTimestampScan(line)
		gotTS, gotRest := ParseTimestamp(line)
		if !gotTS.Equal(wantTS) || gotRest != wantRest {
			t.Errorf("ParseTimestamp(%q) = (%v, %q), scan = (%v, %q)", line, gotTS, gotRest, wantTS, wantRest)
		}
	}
}

func BenchmarkParseLogLine(b *testing.B) {
	lines := map[string]string{
		"access":  "2026-09-15T12:55:38.123456789Z 10.0.0.1 - - [15/Sep/2026:12:55:38 +0000] \"GET /api/v1/containers HTTP/1.1\" 200 512",
		"keyword": "2026-09-15T12:55:38.123456789Z Connection to upstream failed, retrying in 5s",
		"keyed":   "2026-09-15T12:55:38.123456789Z time=2026-09-15T12:55:38Z level=info msg=\"listening on :8080\"",
		"json":    "2026-09-15T12:55:38.123456789Z {\"level\":\"warn\",\"msg\":\"slow query\",\"ms\":812}",
	}
	for name, line := range lines {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				ParseLogLine(line, "stdout")
			}
		})
	}
}

func TestDetectLogLevelFallbackHeuristics(t *testing.T) {
	tests := []struct {
		message string
		want    LogLevel
	}{
		{`TypeError: x is not a function`, LogLevelError},
		{`java.lang.NullPointerException: null`, LogLevelError},
		{`Unhandled rejection ReferenceError: foo is not defined`, LogLevelError},
		{`registered ExceptionHandler middleware`, LogLevelUnknown},
		{`tests: 12 passed, 0 failed`, LogLevelUnknown},
		{`tests: 2 passed, 10 failed`, LogLevelError},
		{`connected to db err=nil`, LogLevelUnknown},
		{`msg="sync done" error=""`, LogLevelUnknown},
		{`no error detected during healthcheck`, LogLevelUnknown},
		{`registered error handler for route /x`, LogLevelUnknown},
		{`Loaded error pages from /etc/nginx/pages`, LogLevelUnknown},
		{`Stack trace:`, LogLevelUnknown},
		{`connection refused: dial tcp 10.0.0.5:5432`, LogLevelError},
		{`Killed process 123 (node) out of memory`, LogLevelError},
		{`request timed out after 30s`, LogLevelWarn},
		{`Unable to connect to redis, retrying in 5s`, LogLevelWarn},
		{`thread 'main' panicked at src/main.rs:4:5:`, LogLevelPanic},
		{`172.17.0.1 - - [14/Sep/2026:12:00:00 +0000] "GET /checkout HTTP/1.1" 500 512`, LogLevelError},
		{`172.17.0.1 - - [14/Sep/2026:12:00:00 +0000] "GET /missing HTTP/1.1" 404 12`, LogLevelWarn},
		{`172.17.0.1 - - [14/Sep/2026:12:00:00 +0000] "GET /api/error HTTP/1.1" 200 512`, LogLevelUnknown},
		{`[GIN] 2026/09/14 - 12:00:00 | 500 |  1.2ms | 172.17.0.1 | GET "/x"`, LogLevelError},
		{`1:M 14 Sep 2026 12:00:00.000 * Ready to accept connections`, LogLevelInfo},
		{`1:M 14 Sep 2026 12:00:00.000 # WARNING overcommit_memory is set to 0`, LogLevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if got := DetectLogLevel(tt.message); got != tt.want {
				t.Fatalf("DetectLogLevel(%q) = %s, want %s", tt.message, got, tt.want)
			}
		})
	}
}

func TestGroupRelatedLogEntriesKeepsTracesWhole(t *testing.T) {
	type row struct {
		level LogLevel
		count int
	}
	tests := []struct {
		name string
		text string
		want []row
	}{
		{"python chained", `2026-09-14 12:00:00,123 INFO worker started
Traceback (most recent call last):
  File "/app/main.py", line 10, in <module>
ValueError: invalid literal for int()

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "/app/main.py", line 14, in <module>
requests.exceptions.ConnectionError: Max retries exceeded (Failed to establish a new connection)`,
			[]row{{LogLevelInfo, 0}, {LogLevelError, 8}}},
		{"java exception after info", `2026-09-14 12:00:00.123 INFO 1 --- [main] c.f.App : Started App
java.lang.NullPointerException: null
	at com.foo.App.main(App.java:5)`,
			[]row{{LogLevelInfo, 0}, {LogLevelError, 1}}},
		{"go nil deref", `panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x10a2b3c]

goroutine 1 [running]:
main.(*Server).handle(0xc000010000)
	/app/main.go:12 +0x1d`,
			[]row{{LogLevelPanic, 5}}},
		{"node errors back to back", `TypeError: a is not a function
    at /app/index.js:5:11
TypeError: b is not a function
    at /app/index.js:9:11`,
			[]row{{LogLevelError, 1}, {LogLevelError, 1}}},
		{"php", `[14-Sep-2026 12:00:00 UTC] PHP Fatal error:  Uncaught Exception: boom in /var/www/index.php:3
Stack trace:
#0 /var/www/index.php(7): foo()
#1 {main}
  thrown in /var/www/index.php on line 3`,
			[]row{{LogLevelFatal, 4}}},
		{"postgres", `2026-09-14 12:00:00.000 UTC [77] ERROR:  duplicate key value violates unique constraint "users_pkey"
2026-09-14 12:00:00.000 UTC [77] DETAIL:  Key (id)=(1) already exists.
2026-09-14 12:00:00.000 UTC [77] STATEMENT:  INSERT INTO users VALUES (1)
2026-09-14 12:00:01.000 UTC [77] LOG:  checkpoint starting: time`,
			[]row{{LogLevelError, 2}, {LogLevelInfo, 0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []LogEntry
			for _, line := range strings.Split(tt.text, "\n") {
				entries = append(entries, ParseLogLine("2026-09-14T12:00:00.000Z "+line, "stderr"))
			}
			grouped := GroupRelatedLogEntries(entries)
			if len(grouped) != len(tt.want) {
				for _, g := range grouped {
					t.Logf("%s cont=%d %q", g.Level, g.ContinuationCount, g.Message)
				}
				t.Fatalf("expected %d entries, got %d", len(tt.want), len(grouped))
			}
			for i, w := range tt.want {
				if grouped[i].Level != w.level || grouped[i].ContinuationCount != w.count {
					t.Fatalf("entry %d: expected %s cont=%d, got %s cont=%d (%q)", i, w.level, w.count, grouped[i].Level, grouped[i].ContinuationCount, grouped[i].Message)
				}
			}
		})
	}
}
