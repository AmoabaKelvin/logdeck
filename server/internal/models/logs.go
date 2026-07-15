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
	// Set only on aggregated multi-container streams; omitempty keeps the
	// single-container payload unchanged.
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
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

// LogLevelRegexes are patterns to detect log levels in messages
var LogLevelRegexes = map[LogLevel]*regexp.Regexp{
	LogLevelTrace: regexp.MustCompile(`(?i)\b(trace|trc)\b`),
	LogLevelDebug: regexp.MustCompile(`(?i)\b(debug|dbg)\b`),
	LogLevelInfo:  regexp.MustCompile(`(?i)\b(info|inf|notice|log)\b`),
	LogLevelWarn:  regexp.MustCompile(`(?i)\b(warn|warning|wrn)\b`),
	LogLevelError: regexp.MustCompile(`(?i)\b(error|err|fail|failed|exception)\b`),
	LogLevelFatal: regexp.MustCompile(`(?i)\b(fatal|critical|crit)\b`),
	LogLevelPanic: regexp.MustCompile(`(?i)\b(panic|emergency)\b`),
}

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
var otelSeverityNumberRegex = regexp.MustCompile(`(?i)(?:^|[\s,{([])severity_?number\s*[:=]\s*"?([0-9]{1,3})"?`)
var keyedLevelRegex = regexp.MustCompile(`(?i)(?:^|[\s,{([])(?:level|lvl|level_?name|severity|severity_?text|log[._-]?level)\s*[:=]\s*"?([A-Za-z]+|[0-9]{1,3})"?`)
var prefixedLevelRegex = regexp.MustCompile(`(?i)^(?:\[|\(|<)?(trace|trc|debug|dbg|dbug|verbose|info|inf|information|notice|warn|warning|wrn|error|err|fatal|critical|crit|panic|emergency|emerg)(?:\]|\)|>|:|\s+-|\s+--|\s+)`)
var glogPrefixRegex = regexp.MustCompile(`^([IWEF])\d{4}\s`)

// levelCheckOrder ranks levels most-severe first, so a message mentioning
// several level keywords is classified by the worst one.
var levelCheckOrder = []LogLevel{
	LogLevelPanic,
	LogLevelFatal,
	LogLevelError,
	LogLevelWarn,
	LogLevelInfo,
	LogLevelDebug,
	LogLevelTrace,
}

// DetectLogLevel analyzes a log message to determine its severity level
func DetectLogLevel(message string) LogLevel {
	if level, ok := ExtractExplicitLogLevel(message); ok {
		return level
	}

	for _, level := range levelCheckOrder {
		if LogLevelRegexes[level].MatchString(message) {
			return level
		}
	}

	return LogLevelUnknown
}

func ExtractExplicitLogLevel(message string) (LogLevel, bool) {
	message = strings.TrimSpace(message)
	if message == "" {
		return LogLevelUnknown, false
	}

	if level, ok := extractJSONLogLevel(message); ok {
		return level, true
	}

	if matches := otelSeverityNumberRegex.FindStringSubmatch(message); len(matches) == 2 {
		if level, ok := normalizeOtelSeverityNumber(matches[1]); ok {
			return level, true
		}
	}

	if matches := keyedLevelRegex.FindStringSubmatch(message); len(matches) == 2 {
		if level, ok := normalizeLogLevel(matches[1]); ok {
			return level, true
		}
	}

	if matches := prefixedLevelRegex.FindStringSubmatch(message); len(matches) == 2 {
		if level, ok := normalizeLogLevel(matches[1]); ok {
			return level, true
		}
	}

	if matches := glogPrefixRegex.FindStringSubmatch(message); len(matches) == 2 {
		switch matches[1] {
		case "I":
			return LogLevelInfo, true
		case "W":
			return LogLevelWarn, true
		case "E":
			return LogLevelError, true
		case "F":
			return LogLevelFatal, true
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
	return strings.TrimSpace(ansiRegex.ReplaceAllString(message, ""))
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
			appendContinuationLine(&grouped[len(grouped)-1], entry)
			continue
		}

		grouped = append(grouped, entry)
	}

	return grouped
}

func IsContinuationLogEntry(entry LogEntry, previous LogEntry) bool {
	if entry.Level != LogLevelUnknown {
		return false
	}

	message := strings.TrimSpace(entry.Message)
	if message == "" || strings.TrimSpace(previous.Message) == "" {
		return false
	}

	if structuredFieldRegex.MatchString(message) {
		return true
	}

	return isStackTraceContinuation(message) && isProblemLevel(previous.Level)
}

func appendContinuationLine(entry *LogEntry, continuation LogEntry) {
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

func isStackTraceContinuation(message string) bool {
	stackPrefixes := []string{
		"at ",
		"File ",
		"Traceback ",
		"Caused by:",
		"... ",
		"goroutine ",
	}

	for _, prefix := range stackPrefixes {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}

	return strings.HasPrefix(message, "/") && strings.Contains(message, ":")
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
