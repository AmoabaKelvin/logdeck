package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// FieldFilter is a set of conditions on the fields of structured log lines,
// written as space-separated `key=value` or `key!=value` terms that must all
// hold. A value with spaces is double-quoted. A JSON object line is looked up
// by key or by dot path (`http.status`); any other line is read as logfmt. The
// frontend mirrors this in lib/field-filter.ts.
type FieldFilter []FieldTerm

// FieldTerm is one condition of a FieldFilter.
type FieldTerm struct {
	Key    string
	Value  string // lowercased: values compare case-insensitively
	Negate bool   // key!=value: the line lacks the field or holds another value
	// A numeric value equals a JSON number numerically, so 42 matches 42.0.
	number   float64
	isNumber bool
}

var fieldTermRegex = regexp.MustCompile(`^([A-Za-z_@][A-Za-z0-9_.@-]*)(!?=)(.*)$`)

// jsonNumberRegex is the JSON number grammar, which parses the same way here
// and in JavaScript.
var jsonNumberRegex = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

// ParseFieldFilter parses space-separated field terms. Every token must be a
// term; an empty string is an empty filter.
func ParseFieldFilter(s string) (FieldFilter, error) {
	var filter FieldFilter
	for _, token := range splitFieldTokens(s) {
		term, ok := parseFieldTerm(token)
		if !ok {
			return nil, fmt.Errorf("invalid field filter %q: expected key=value or key!=value", token)
		}
		filter = append(filter, term)
	}
	return filter, nil
}

// splitFieldTokens splits s on whitespace outside double quotes.
func splitFieldTokens(s string) []string {
	var (
		tokens  []string
		start   = -1
		quoted  bool
		escaped bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case quoted && c == '\\':
			escaped = true
		case c == '"':
			quoted = !quoted
		case !quoted && isFieldSpace(c):
			if start >= 0 {
				tokens = append(tokens, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		tokens = append(tokens, s[start:])
	}
	return tokens
}

func parseFieldTerm(token string) (FieldTerm, bool) {
	matches := fieldTermRegex.FindStringSubmatch(token)
	if matches == nil {
		return FieldTerm{}, false
	}
	value := matches[3]
	if strings.HasPrefix(value, `"`) {
		// An unclosed quote runs to the end of the term.
		value, _ = readQuoted(value, 1)
	}
	term := FieldTerm{Key: matches[1], Value: strings.ToLower(value), Negate: matches[2] == "!="}
	if jsonNumberRegex.MatchString(value) {
		term.number, _ = strconv.ParseFloat(value, 64)
		term.isNumber = true
	}
	return term, true
}

// readQuoted reads a double-quoted value whose opening quote is just before
// s[start]. A backslash takes the next byte literally. It returns the value
// and the index past the closing quote.
func readQuoted(s string, start int) (string, int) {
	var b strings.Builder
	i := start
	for ; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == '"':
			return b.String(), i + 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), i
}

func isFieldSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// Match reports whether message, a parsed log message, satisfies every term.
func (f FieldFilter) Match(message string) bool {
	if len(f) == 0 {
		return true
	}
	if !f.mayMatch(message) {
		return false
	}
	fields := parseLineFields(message)
	for _, term := range f {
		value, ok := fields.lookup(term.Key)
		if (ok && term.equals(value)) == term.Negate {
			return false
		}
	}
	return true
}

// mayMatch rules a line out before it is parsed. Without backslash escapes, a
// line holds its keys and values verbatim, so one missing the last segment of
// a key, or a non-numeric value, cannot match that term. Numbers are skipped:
// 42 can be written 42.0.
func (f FieldFilter) mayMatch(message string) bool {
	if strings.IndexByte(message, '\\') >= 0 {
		return true
	}
	for _, term := range f {
		key := term.Key[strings.LastIndexByte(term.Key, '.')+1:]
		if !term.Negate && !strings.Contains(message, key) {
			return false
		}
	}
	lower := ""
	for _, term := range f {
		if term.Negate || term.isNumber {
			continue
		}
		if lower == "" {
			lower = strings.ToLower(message)
		}
		if !strings.Contains(lower, term.Value) {
			return false
		}
	}
	return true
}

func (t FieldTerm) equals(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.ToLower(v) == t.Value
	case float64:
		return t.isNumber && v == t.number
	case bool:
		return strconv.FormatBool(v) == t.Value
	case nil:
		return t.Value == "null"
	}
	return false // an object or array
}

// lineFields holds the fields of one line: a JSON object, or logfmt pairs.
type lineFields struct {
	object map[string]any
	pairs  map[string]string
}

func parseLineFields(message string) lineFields {
	message = strings.TrimSpace(message)
	if strings.HasPrefix(message, "{") {
		var object map[string]any
		if json.Unmarshal([]byte(message), &object) == nil {
			return lineFields{object: object}
		}
	}
	return lineFields{pairs: parseLogfmt(message)}
}

// lookup finds key as a top-level JSON key first, so "log.level" names a
// dotted key when the line has one, then as a dot path.
func (l lineFields) lookup(key string) (any, bool) {
	if l.object == nil {
		value, ok := l.pairs[key]
		return value, ok
	}
	if value, ok := l.object[key]; ok {
		return value, true
	}
	var current any = l.object
	for segment := range strings.SplitSeq(key, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = object[segment]; !ok {
			return nil, false
		}
	}
	return current, true
}

// parseLogfmt reads the key=value and key="quoted value" pairs of a line and
// skips everything else, so the pairs inside a plain sentence count too. A
// repeated key keeps its last value.
func parseLogfmt(s string) map[string]string {
	var pairs map[string]string
	for i := 0; i < len(s); {
		if isFieldSpace(s[i]) {
			i++
			continue
		}
		start := i
		for i < len(s) && !isFieldSpace(s[i]) && s[i] != '=' && s[i] != '"' {
			i++
		}
		if i == start || i == len(s) || s[i] != '=' {
			for i < len(s) && !isFieldSpace(s[i]) {
				i++
			}
			continue
		}
		key := s[start:i]
		i++
		var value string
		if i < len(s) && s[i] == '"' {
			value, i = readQuoted(s, i+1)
		} else {
			valueStart := i
			for i < len(s) && !isFieldSpace(s[i]) {
				i++
			}
			value = s[valueStart:i]
		}
		if pairs == nil {
			pairs = make(map[string]string)
		}
		pairs[key] = value
	}
	return pairs
}
