package models

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LogEntry represents a parsed log line with metadata
type LogEntry struct {
	Timestamp         time.Time         `json:"timestamp"`
	Level             LogLevel          `json:"level"`
	Message           string            `json:"message"`
	Stream            string            `json:"stream"` // "stdout" or "stderr"
	Raw               string            `json:"raw"`    // Original log line
	Fields            map[string]string `json:"fields,omitempty"`
	ContinuationCount int               `json:"continuationCount,omitempty"`
	// Set on live streams, which are emitted line by line: the line folds
	// into the entry emitted just before it. Grouped responses never set it.
	Continuation bool `json:"continuation,omitempty"`
	// Set only on aggregated multi-container streams; omitempty keeps the
	// single-container payload unchanged.
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	// Set on stored entries, where the same name can exist on several hosts.
	Host string `json:"host,omitempty"`
}

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LogLevelTrace   LogLevel = "TRACE"
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarn    LogLevel = "WARN"
	LogLevelError   LogLevel = "ERROR"
	LogLevelFatal   LogLevel = "FATAL"
	LogLevelPanic   LogLevel = "PANIC"
	LogLevelUnknown LogLevel = "UNKNOWN"
)

// exceptionName matches a class name such as "TypeError" or "NullPointerException".
const exceptionName = `[A-Z]\w*(?:Error|Exception)\b`

// levelWords is the keyword fallback, most severe first, so a message
// mentioning several levels is classified by the worst one. Phrases are failure
// wording with no level keyword; they are plain substrings because as regex
// alternations they slowed every match. Words are substrings every match of
// the regexes contains, so a level whose words are absent skips its regexes.
var levelWords = []struct {
	level   LogLevel
	words   []string
	regexes []*regexp.Regexp
	phrases []string
}{
	{level: LogLevelPanic, words: []string{"panic", "emergency"}, regexes: mustCompileAll(`(?i)\b(panic|panicked|emergency)\b`)},
	{level: LogLevelFatal, words: []string{"fatal", "crit"}, regexes: mustCompileAll(`(?i)\b(fatal|critical|crit)\b`)},
	{
		level:   LogLevelError,
		words:   []string{"err", "fail", "exception", "traceback"},
		regexes: mustCompileAll(`(?i)\b(error|err|fail|failed|exception|traceback)\b`, `\b`+exceptionName),
		phrases: []string{"connection refused", "permission denied", "access denied", "out of memory", "segmentation fault"},
	},
	{
		level:   LogLevelWarn,
		words:   []string{"warn", "wrn"},
		regexes: mustCompileAll(`(?i)\b(warn|warning|wrn)\b`),
		phrases: []string{"deprecated", "timed out", "unable to", "could not"},
	},
	{level: LogLevelInfo, words: []string{"inf", "notice", "log"}, regexes: mustCompileAll(`(?i)\b(info|inf|notice|log)\b`)},
	{level: LogLevelDebug, words: []string{"debug", "dbg"}, regexes: mustCompileAll(`(?i)\b(debug|dbg)\b`)},
	{level: LogLevelTrace, words: []string{"trace", "trc"}, regexes: mustCompileAll(`(?i)\b(trace|trc)\b`)},
}

func mustCompileAll(patterns ...string) []*regexp.Regexp {
	regexes := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regexes[i] = regexp.MustCompile(pattern)
	}
	return regexes
}

// notALevelRegex matches level keywords used in a way that says nothing about
// severity ("0 failed", "err=nil", "error handler"). They are dropped before
// the keyword fallback runs.
var notALevelRegex = regexp.MustCompile(`(?i)\b(?:no|0|zero|without)\s+(?:errors?|failures?|fail|failed|exceptions?|warnings?)\b` +
	`|\b(?:err|error)"?\s*[=:]\s*(?:nil|null|none|false|<nil>|""|'')` +
	`|\berror[ _-](?:handl\w+|pages?|report\w*|track\w*|rates?|counts?|boundar\w+)` +
	`|\bstack (?:back)?trace\b`)

// notALevelHints are the substrings notALevelRegex needs, as levelKeywords is
// for the level regexes. Its "error <noun>" branch is checked by noun instead,
// since nearly every ERROR line contains "error ".
var notALevelHints = []string{
	"no ", "0 ", "zero ", "without ", "nil", "null", "none", "false", `""`, "''", "stack ",
}

var notALevelErrorNouns = []string{"handl", "page", "report", "track", "rate", "count", "boundar"}

// Common timestamp formats found in Docker logs
var timestampFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05.999",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"02/Jan/2006:15:04:05 -0700",
	time.ANSIC,
	time.UnixDate,
	time.RubyDate,
}

// minTimestampLen and maxTimestampLen bound the rendered length of every layout
// in timestampFormats (the shortest is "2006/01/02 15:04:05" at 19, the longest
// is RFC3339Nano with nine fractional digits and a numeric offset at 35). The
// bounds carry a small margin and are used only to skip time.Parse for
// candidates that provably cannot match any layout; they never widen what parses.
const (
	minTimestampLen = 17
	maxTimestampLen = 40
)

var tzOffsetNoColon = regexp.MustCompile(`([+-]\d{2})(\d{2})$`)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var structuredFieldRegex = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.-]*)\s*[:=]\s*(.+)$`)

// Continuation heuristics: a raw line indented after the engine timestamp
// (Raw is "<RFC3339Nano> <line>" because logs are always requested with
// timestamps; without one, the line itself starts the raw), a message the app
// stamped itself ("2026-09-14 12:53:29.888 | WARN", "[2026-09-14 12:53:30]
// INFO").
// isIndentedRaw matches `^(?:\d{4}-\d{2}-\d{2}T\S+ )?[ \t]` by hand, since it
// runs on every grouped line.
func isIndentedRaw(raw string) bool {
	if len(raw) > 11 && isDigits(raw[0:4]) && raw[4] == '-' && isDigits(raw[5:7]) &&
		raw[7] == '-' && isDigits(raw[8:10]) && raw[10] == 'T' {
		if i := strings.IndexAny(raw[11:], " \t\n\f\r"); i > 0 && raw[11+i] == ' ' && len(raw) > 12+i {
			return raw[12+i] == ' ' || raw[12+i] == '\t'
		}
	}
	return raw != "" && (raw[0] == ' ' || raw[0] == '\t')
}

func isDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

var appStampedRegex = regexp.MustCompile(`^\W{0,2}\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}`)

// stackLineRegex matches an unindented line that can only belong to a trace.
var stackLineRegex = regexp.MustCompile(`^(?:` +
	`at |Caused by:|\.\.\. ` + // Java, JS
	`|File |Traceback |During handling of the above exception|The above exception was the direct cause` + // Python
	`|goroutine |created by |\[signal |\S+\(.*\)$` + // Go
	`|Stack trace:|#\d+ ` + // PHP
	`|stack backtrace:` + // Rust
	`|/.*:)`) // file:line

// "java.lang.IllegalStateException: boom", "ValueError: bad value".
var exceptionHeaderRegex = regexp.MustCompile(`^(?:[\w$]+\.)*` + exceptionName)

// Postgres follows an ERROR/LOG line with secondary lines for the same event.
// It puts two spaces after the colon; a multi-line STATEMENT ends at the colon.
// The group is the backend PID of the default "%m [%p] " line prefix.
var postgresDetailRegex = regexp.MustCompile(`(?:^|\[(\d+)\] |[\]\s])(?:DETAIL|HINT|QUERY|CONTEXT|LOCATION|STATEMENT|BACKTRACE):(?:  |$)`)

// The regex is unanchored and runs on every grouped line, so lines without a
// keyword are ruled out with substring checks first.
var postgresDetailKeywords = []string{"DETAIL:", "HINT:", "QUERY:", "CONTEXT:", "LOCATION:", "STATEMENT:", "BACKTRACE:"}

func mayBePostgresDetail(message string) bool {
	for _, keyword := range postgresDetailKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

// How far an unstamped spill-over line may trail a stamped entry and still fold into it.
const continuationWindow = 2 * time.Second

var otelSeverityNumberRegex = regexp.MustCompile(`(?i)(?:^|[\s,{([])severity_?number\s*[:=]\s*"?([0-9]{1,3})"?`)
var keyedLevelRegex = regexp.MustCompile(`(?i)(?:^|[\s,{([])(?:level|lvl|level_?name|severity|severity_?text|log[._-]?level)\s*[:=]\s*"?([A-Za-z]+|[0-9]{1,3})"?`)
var prefixedLevelRegex = regexp.MustCompile(`(?i)^(?:\[|\(|<)?(trace|trc|debug|dbg|dbug|verbose|info|inf|information|notice|warn|warning|wrn|error|err|fatal|critical|crit|panic|emergency|emerg)(?:\]|\)|>|:|\s+-|\s+--|\s+)`)

// markerFormats are line formats that spell the level as a single character:
// glog ("E0914 12:00:00 ...") and Redis, after its own timestamp
// ("1:M 04 Sep 2026 00:14:53.950 * Background saving terminated with success").
var markerFormats = []struct {
	regex  *regexp.Regexp
	levels map[string]LogLevel
}{
	{regexp.MustCompile(`^([IWEF])\d{4}\s`), map[string]LogLevel{"I": LogLevelInfo, "W": LogLevelWarn, "E": LogLevelError, "F": LogLevelFatal}},
	{regexp.MustCompile(`^\d+:[XCSM] \d{2} \w{3} \d{4} \d{2}:\d{2}:\d{2}\.\d{3} ([.\-*#]) `), map[string]LogLevel{".": LogLevelDebug, "-": LogLevelDebug, "*": LogLevelInfo, "#": LogLevelWarn}},
}

// levelKeywords and levelKeyNames are the substrings levelWords and the keyed
// level regexes need. A message without them cannot match, so they are skipped.
var levelKeywords = []string{
	"trace", "trc", "debug", "dbg", "dbug", "verbose", "inf", "notice", "log",
	"warn", "wrn", "err", "fail", "exception", "fatal", "crit", "panic", "emerg",
	"refused", "denied", "out of memory", "segmentation", "deprecated", "timed out", "unable to", "could not",
}

var levelKeyNames = []string{"level", "lvl", "severity"}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DetectLogLevel analyzes a log message to determine its severity level
func DetectLogLevel(message string) LogLevel {
	message = strings.TrimSpace(message)
	lower := strings.ToLower(message)
	if level, ok := extractExplicitLogLevel(message, lower); ok {
		return level
	}

	if level, ok := accessLogLevel(message); ok {
		return level
	}

	if !containsAny(lower, levelKeywords) {
		return LogLevelUnknown
	}
	if containsAny(lower, notALevelHints) ||
		(strings.Contains(lower, "error") && containsAny(lower, notALevelErrorNouns)) {
		message = notALevelRegex.ReplaceAllString(message, "")
		lower = strings.ToLower(message)
	}
	for _, words := range levelWords {
		for _, regex := range words.regexes {
			if containsAny(lower, words.words) && regex.MatchString(message) {
				return words.level
			}
		}
		if containsAny(lower, words.phrases) {
			return words.level
		}
	}

	return LogLevelUnknown
}

// accessLogLevel reads the severity of a common-log (`"GET / HTTP/1.1" 500`)
// or Gin (`[GIN] ... | 500 |`) line from its status code, so a URL such as
// /api/errors cannot decide it. A 2xx/3xx line is deliberately UNKNOWN.
func accessLogLevel(message string) (LogLevel, bool) {
	var status string
	if i := strings.Index(message, `HTTP/`); i >= 0 {
		_, status, _ = strings.Cut(message[i:], `" `)
	} else if strings.HasPrefix(message, "[GIN]") {
		_, status, _ = strings.Cut(message, "| ")
	} else {
		return LogLevelUnknown, false
	}
	status, _, _ = strings.Cut(status, " ")
	code, err := strconv.Atoi(status)
	switch {
	case err != nil || code < 100 || code > 599:
		return LogLevelUnknown, false
	case code >= 500:
		return LogLevelError, true
	case code >= 400:
		return LogLevelWarn, true
	}
	return LogLevelUnknown, true
}

func extractExplicitLogLevel(message, lower string) (LogLevel, bool) {
	if message == "" {
		return LogLevelUnknown, false
	}

	if level, ok := extractJSONLogLevel(message); ok {
		return level, true
	}

	if containsAny(lower, levelKeyNames) {
		// The OTel regex needs severity_number or severitynumber.
		if strings.Contains(lower, "severity") {
			if matches := otelSeverityNumberRegex.FindStringSubmatch(message); len(matches) == 2 {
				if level, ok := normalizeOtelSeverityNumber(matches[1]); ok {
					return level, true
				}
			}
		}

		if matches := keyedLevelRegex.FindStringSubmatch(message); len(matches) == 2 {
			if level, ok := normalizeLogLevel(matches[1]); ok {
				return level, true
			}
		}
	}

	// Every prefix the regex accepts starts with a bracket or a level word.
	if strings.IndexByte("[(<tdviwnefcp", lower[0]) >= 0 && containsAny(lower, levelKeywords) {
		if matches := prefixedLevelRegex.FindStringSubmatch(message); len(matches) == 2 {
			if level, ok := normalizeLogLevel(matches[1]); ok {
				return level, true
			}
		}
	}

	for _, format := range markerFormats {
		if matches := format.regex.FindStringSubmatch(message); len(matches) == 2 {
			return format.levels[matches[1]], true
		}
	}

	return LogLevelUnknown, false
}

func extractJSONLogLevel(message string) (LogLevel, bool) {
	if !strings.HasPrefix(message, "{") {
		return LogLevelUnknown, false
	}

	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(message))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return LogLevelUnknown, false
	}

	for _, key := range []string{"level", "lvl", "levelname", "level_name", "severity", "severity_text", "severityText", "log.level"} {
		value, exists := payload[key]
		if !exists {
			continue
		}
		if level, ok := normalizeLogLevelValue(value, false); ok {
			return level, true
		}
	}

	for _, key := range []string{"severity_number", "severityNumber"} {
		value, exists := payload[key]
		if !exists {
			continue
		}
		if level, ok := normalizeLogLevelValue(value, true); ok {
			return level, true
		}
	}

	return LogLevelUnknown, false
}

func normalizeLogLevelValue(value any, otelSeverityNumber bool) (LogLevel, bool) {
	switch typed := value.(type) {
	case string:
		if otelSeverityNumber {
			return normalizeOtelSeverityNumber(typed)
		}
		return normalizeLogLevel(typed)
	case json.Number:
		if otelSeverityNumber {
			return normalizeOtelSeverityNumber(typed.String())
		}
		return normalizeNumericLogLevel(typed.String())
	default:
		return LogLevelUnknown, false
	}
}

func normalizeOtelSeverityNumber(value string) (LogLevel, bool) {
	levelNumber, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return LogLevelUnknown, false
	}

	switch {
	case levelNumber >= 1 && levelNumber <= 4:
		return LogLevelTrace, true
	case levelNumber >= 5 && levelNumber <= 8:
		return LogLevelDebug, true
	case levelNumber >= 9 && levelNumber <= 12:
		return LogLevelInfo, true
	case levelNumber >= 13 && levelNumber <= 16:
		return LogLevelWarn, true
	case levelNumber >= 17 && levelNumber <= 20:
		return LogLevelError, true
	case levelNumber >= 21 && levelNumber <= 24:
		return LogLevelFatal, true
	default:
		return LogLevelUnknown, false
	}
}

func normalizeLogLevel(value string) (LogLevel, bool) {
	normalized := strings.ToLower(strings.Trim(value, `"'[](){}<>: ,`))

	switch normalized {
	case "trace", "trc":
		return LogLevelTrace, true
	case "debug", "dbg", "dbug", "verbose":
		return LogLevelDebug, true
	case "info", "inf", "information", "notice", "log":
		return LogLevelInfo, true
	case "warn", "warning", "wrn":
		return LogLevelWarn, true
	case "error", "err", "fail", "failed", "exception":
		return LogLevelError, true
	case "fatal", "critical", "crit", "alert":
		return LogLevelFatal, true
	case "panic", "emergency", "emerg":
		return LogLevelPanic, true
	}

	return normalizeNumericLogLevel(normalized)
}

func normalizeNumericLogLevel(value string) (LogLevel, bool) {
	switch strings.TrimSpace(value) {
	case "10":
		return LogLevelTrace, true
	case "20":
		return LogLevelDebug, true
	case "30":
		return LogLevelInfo, true
	case "40":
		return LogLevelWarn, true
	case "50":
		return LogLevelError, true
	case "60":
		return LogLevelFatal, true
	default:
		return LogLevelUnknown, false
	}
}

// ParseTimestamp attempts to extract a timestamp from the beginning of a log line
func ParseTimestamp(logLine string) (time.Time, string) {
	line := strings.TrimSpace(logLine)
	if line == "" {
		return time.Time{}, ""
	}

	const maxPrefix = 96
	searchLimit := min(len(line), maxPrefix)

	// Fast path: the engine (logs are always requested with timestamps) and most
	// loggers put the stamp first, so the leading token nearly always is it. A
	// token that parses on its own is a complete space-free layout, and no
	// layout begins with one of those, so it is also the longest parseable
	// prefix: the scan below would return the same thing after failing
	// time.Parse a hundred times, and each failure allocates a ParseError, which
	// made this the process's dominant allocation.
	if space := strings.IndexByte(line, ' '); space > 0 && space <= searchLimit {
		if ts, ok := tryParseTimestampCandidate(line[:space]); ok {
			return ts, strings.TrimLeft(strings.TrimSpace(line[space:]), ")]}> \t")
		}
	}

	// The timestamp is the longest leading prefix that parses, so scan from the
	// longest candidate down and stop at the first hit: the result is identical
	// to keeping the last hit of an ascending scan, but a line whose prefix is a
	// timestamp (the common case) settles in a handful of iterations instead of
	// paying a time.Parse attempt at every offset up to the limit.
	for i := searchLimit; i >= 1; i-- {
		if ts, ok := tryParseTimestampCandidate(line[:i]); ok {
			remaining := strings.TrimSpace(line[i:])
			remaining = strings.TrimLeft(remaining, ")]}> \t")
			return ts, remaining
		}
	}

	return time.Time{}, line
}

// CleanMessage removes common log formatting artifacts
func CleanMessage(message string) string {
	// A terminal shows only what follows the last carriage return; progress
	// bars (tqdm, pip, npm) rewrite their line that way.
	message = strings.TrimRight(message, "\r")
	if i := strings.LastIndexByte(message, '\r'); i >= 0 {
		message = message[i+1:]
	}
	// ReplaceAllString copies the message even when nothing matches.
	if strings.Contains(message, "\x1b[") {
		message = ansiRegex.ReplaceAllString(message, "")
	}
	return strings.TrimSpace(message)
}

// ParseLogLine parses a Docker log line into a structured LogEntry
func ParseLogLine(logLine string, stream string) LogEntry {
	timestamp, messageWithoutTimestamp := ParseTimestamp(logLine)
	cleanedMessage := CleanMessage(messageWithoutTimestamp)
	level := DetectLogLevel(cleanedMessage)

	return LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   cleanedMessage,
		Stream:    stream,
		Raw:       logLine,
	}
}

// GroupRelatedLogEntries folds structured continuation lines into the previous
// logical log event. Docker adds its own timestamp to every physical line, so
// multi-line app logs can otherwise appear as separate UNKNOWN rows.
func GroupRelatedLogEntries(entries []LogEntry) []LogEntry {
	grouped := make([]LogEntry, 0, len(entries))

	for _, entry := range entries {
		if len(grouped) > 0 && IsContinuationLogEntry(entry, grouped[len(grouped)-1]) {
			AppendContinuationLine(&grouped[len(grouped)-1], entry)
			continue
		}

		grouped = append(grouped, entry)
	}

	return grouped
}

// IsContinuationLogEntry reports whether entry is a physical line of the same
// logical event as previous. The rules run in this order, and the order is
// load-bearing:
//
//  1. Blank or indented lines (stack frames, YAML/JSON dumps) always fold,
//     whatever level keywords they contain.
//  2. Postgres DETAIL/HINT/STATEMENT lines fold regardless of their own
//     level: "DETAIL:  Failed process was running" classifies as ERROR. One
//     from another backend PID never folds.
//  3. After a WARN+ entry, stack-shaped lines fold regardless of their own
//     level: "Caused by: ... failed" classifies as ERROR and must still fold.
//  4. Everything below applies to UNKNOWN lines only, so "ERROR: x" never
//     folds into the entry before it.
//  5. A "key: value" line folds (structured fields printed one per line).
//  6. An unstamped line right behind an app-stamped one is spill-over from
//     that event (progress bar, access log, bare print).
func IsContinuationLogEntry(entry LogEntry, previous LogEntry) bool {
	message := strings.TrimSpace(entry.Message)
	if message == "" || isIndentedRaw(entry.Raw) {
		return true
	}
	if strings.TrimSpace(previous.Message) == "" {
		return false
	}

	if mayBePostgresDetail(message) {
		if matches := postgresDetailRegex.FindStringSubmatch(message); matches != nil {
			// Backends interleave: only fold into a line from the same PID.
			return matches[1] == "" || strings.Contains(previous.Message, "["+matches[1]+"] ")
		}
	}

	if isProblemLevel(previous.Level) {
		if stackLineRegex.MatchString(message) {
			return true
		}
		// An exception line closes a Python traceback and opens a Java one, but
		// behind another exception line it is a new event.
		if exceptionHeaderRegex.MatchString(message) && !exceptionHeaderRegex.MatchString(previous.Message) {
			return true
		}
	}

	if entry.Level != LogLevelUnknown {
		return false
	}

	if structuredFieldRegex.MatchString(message) {
		return true
	}

	return appStampedRegex.MatchString(previous.Message) &&
		!appStampedRegex.MatchString(message) &&
		!entry.Timestamp.IsZero() && !previous.Timestamp.IsZero() &&
		entry.Timestamp.Sub(previous.Timestamp) <= continuationWindow
}

// AppendContinuationLine folds continuation into entry.
func AppendContinuationLine(entry *LogEntry, continuation LogEntry) {
	message := strings.TrimSpace(continuation.Message)
	if message != "" {
		if entry.Message == "" {
			entry.Message = message
		} else {
			entry.Message += "\n" + message
		}
	}

	if continuation.Raw != "" {
		if entry.Raw == "" {
			entry.Raw = continuation.Raw
		} else {
			entry.Raw += "\n" + continuation.Raw
		}
	}

	if key, value, ok := parseStructuredField(message); ok {
		if entry.Fields == nil {
			entry.Fields = make(map[string]string)
		}
		entry.Fields[key] = value
	}

	entry.ContinuationCount++
}

func parseStructuredField(message string) (string, string, bool) {
	matches := structuredFieldRegex.FindStringSubmatch(strings.TrimSpace(message))
	if len(matches) != 3 {
		return "", "", false
	}

	return matches[1], strings.TrimSpace(matches[2]), true
}

func isProblemLevel(level LogLevel) bool {
	switch level {
	case LogLevelWarn, LogLevelError, LogLevelFatal, LogLevelPanic:
		return true
	default:
		return false
	}
}

func tryParseTimestampCandidate(candidate string) (time.Time, bool) {
	sanitized := strings.TrimSpace(candidate)
	if sanitized == "" {
		return time.Time{}, false
	}

	sanitized = strings.Trim(sanitized, "[](){}<>")
	if sanitized == "" {
		return time.Time{}, false
	}

	// A candidate outside the layout length bounds cannot match any format, so
	// skip the time.Parse attempts entirely and keep the prefix scan cheap.
	if n := len(sanitized); n < minTimestampLen || n > maxTimestampLen {
		return time.Time{}, false
	}
	// Every layout starts with a digit or a weekday name; anything else cannot
	// parse, and a failed time.Parse is not free.
	if c := sanitized[0]; (c < '0' || c > '9') && !startsWithWeekday(sanitized) {
		return time.Time{}, false
	}

	sanitized = normalizeFractionSeparator(sanitized)

	for _, format := range timestampFormats {
		if ts, err := time.Parse(format, sanitized); err == nil {
			return ts.UTC(), true
		}
	}

	if matches := tzOffsetNoColon.FindStringSubmatch(sanitized); len(matches) == 3 {
		withColon := sanitized[:len(sanitized)-len(matches[0])] + matches[1] + ":" + matches[2]
		for _, format := range []string{time.RFC3339Nano, time.RFC3339} {
			if ts, err := time.Parse(format, withColon); err == nil {
				return ts.UTC(), true
			}
		}
	}

	return time.Time{}, false
}

func startsWithWeekday(value string) bool {
	for _, day := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		if strings.HasPrefix(value, day) {
			return true
		}
	}
	return false
}

func normalizeFractionSeparator(value string) string {
	if strings.Contains(value, ",") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) == 2 && isNumericSuffix(parts[1]) {
			return parts[0] + "." + parts[1]
		}
	}
	return value
}

func isNumericSuffix(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
