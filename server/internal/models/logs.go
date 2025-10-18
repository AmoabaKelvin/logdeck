package models

import (
	"regexp"
	"strings"
	"time"
)

// LogEntry represents a parsed log line with metadata
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
	Raw       string    `json:"raw"`    // Original log line
}

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LogLevelTrace   LogLevel = "TRACE"
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarn    LogLevel = "WARN"
	LogLevelWarning LogLevel = "WARNING"
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

var tzOffsetNoColon = regexp.MustCompile(`([+-]\d{2})(\d{2})$`)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// DetectLogLevel analyzes a log message to determine its severity level
func DetectLogLevel(message string) LogLevel {
	checkOrder := []LogLevel{
		LogLevelPanic,
		LogLevelFatal,
		LogLevelError,
		LogLevelWarn,
		LogLevelInfo,
		LogLevelDebug,
		LogLevelTrace,
	}

	for _, level := range checkOrder {
		if regex, exists := LogLevelRegexes[level]; exists {
			if regex.MatchString(message) {
				return level
			}
		}
	}

	return LogLevelUnknown
}

// ParseTimestamp attempts to extract a timestamp from the beginning of a log line
func ParseTimestamp(logLine string) (time.Time, string) {
	line := strings.TrimSpace(logLine)
	if line == "" {
		return time.Time{}, ""
	}

	const maxPrefix = 96
	searchLimit := min(len(line), maxPrefix)

	var (
		foundTimestamp time.Time
		foundMessage   string
		found          bool
	)

	for i := 1; i <= searchLimit; i++ {
		prefix := line[:i]
		if ts, ok := tryParseTimestampCandidate(prefix); ok {
			remaining := strings.TrimSpace(line[i:])
			remaining = strings.TrimLeft(remaining, ")]}> \t")
			foundTimestamp = ts
			foundMessage = remaining
			found = true
		}
	}

	if found {
		return foundTimestamp, foundMessage
	}

	return time.Time{}, line
}

// CleanMessage removes common log formatting artifacts
func CleanMessage(message string) string {
	message = ansiRegex.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)

	return message
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

func tryParseTimestampCandidate(candidate string) (time.Time, bool) {
	sanitized := strings.TrimSpace(candidate)
	if sanitized == "" {
		return time.Time{}, false
	}

	sanitized = strings.Trim(sanitized, "[](){}<>")
	if sanitized == "" {
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
