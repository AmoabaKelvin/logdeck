// Package redact masks secrets in log lines before LogDeck stores or serves
// them. The rules are compiled once when the settings change and published
// atomically. Every engine log read redacts its lines through a Stream, so
// a line costs one atomic load while the rules are unchanged, and nothing
// more when no rule applies to the container.
package redact

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

// DefaultReplacement stands in for a redacted value when the settings name
// no replacement.
const DefaultReplacement = "[REDACTED]"

// Target identifies the container a line came from, for rule targeting.
type Target struct {
	Host    string
	Name    string // without the leading "/"
	Project string // compose project, "" when none
}

type rule struct {
	re          *regexp.Regexp
	replacement string
	// hints are substrings every match contains: a line holding none of them
	// skips the regex. fold marks lower-case hints matched ignoring ASCII case.
	// lead bounds how far before its hint a match can start, so the regex
	// only scans from the earliest hint on.
	hints []string
	fold  bool
	lead  int

	// Targeting, custom rules only. Same semantics as alert rules.
	hosts      []string
	containers []string
	projects   []string
}

func (r *rule) targeted() bool {
	return len(r.hosts) > 0 || len(r.containers) > 0 || len(r.projects) > 0
}

// appliesTo mirrors logstream.ContainerSpec.Matches: hosts are ANDed with the
// container/project dimension, which is an OR between names and projects.
func (r *rule) appliesTo(t Target) bool {
	if len(r.hosts) > 0 && !slices.Contains(r.hosts, t.Host) {
		return false
	}
	if len(r.containers) == 0 && len(r.projects) == 0 {
		return true
	}
	return slices.Contains(r.containers, t.Name) || (t.Project != "" && slices.Contains(r.projects, t.Project))
}

// start returns where a match can begin in line at the earliest, or -1 when
// the line holds no hint and so cannot match. Without hints it is 0. Fold
// hints are looked up in lower, the line in ASCII lower case.
func (r *rule) start(line, lower string) int {
	if len(r.hints) == 0 {
		return 0
	}
	if r.fold {
		line = lower
	}
	first := -1
	for _, hint := range r.hints {
		// Only a hint that starts before the earliest one found matters.
		s := line
		if first >= 0 {
			s = line[:min(len(line), first-1+len(hint))]
		}
		if i := strings.Index(s, hint); i >= 0 {
			first = i
		}
	}
	if first < 0 {
		return -1
	}
	return max(0, first-r.lead)
}

// Rules is the ordered list of rules that apply to one container.
type Rules struct {
	rules []*rule
}

// Line redacts one physical log line. The engine's timestamp prefix is left
// alone: a rule that rewrote it would cost the line its time and its place
// in the timeline.
func (r *Rules) Line(line string) string {
	var scratch []byte
	return r.line(line, &scratch)
}

// line is Line with a reusable buffer for the lower-cased copy of the line.
func (r *Rules) line(line string, scratch *[]byte) string {
	head := engineTimestampLen(line)
	body := line[head:]
	lower := ""
	lowered, changed := false, false
	for _, rule := range r.rules {
		if rule.fold && !lowered {
			// One lower-cased copy serves every case-insensitive hint, which
			// is several times cheaper than folding case per hint.
			*scratch = appendLower((*scratch)[:0], body)
			lower = unsafe.String(unsafe.SliceData(*scratch), len(*scratch))
			lowered = true
		}
		from := rule.start(body, lower)
		// MatchString first: ReplaceAllString copies the line even when the
		// hint was a false alarm.
		if from < 0 || !rule.re.MatchString(body[from:]) {
			continue
		}
		body = body[:from] + rule.re.ReplaceAllString(body[from:], rule.replacement)
		lowered, changed = false, true
	}
	if !changed {
		return line
	}
	return line[:head] + body
}

// engineTimestampLen returns the length of the "<RFC3339Nano> " prefix the
// engine puts on every line read with timestamps, or 0 when there is none.
func engineTimestampLen(line string) int {
	if len(line) < 21 || line[4] != '-' || line[7] != '-' || line[10] != 'T' || line[0] < '0' || line[0] > '9' {
		return 0
	}
	if i := strings.IndexByte(line[:min(len(line), 41)], ' '); i > 0 {
		return i + 1
	}
	return 0
}

// Set is one compiled redaction config. It is immutable once built.
type Set struct {
	rules []*rule
	// all serves every container when no rule is targeted, so For allocates
	// nothing in the common case.
	all *Rules
}

// For returns the rules that apply to t, or nil when none do. A nil Set has
// no rules.
func (s *Set) For(t Target) *Rules {
	if s == nil {
		return nil
	}
	if s.all != nil {
		return s.all
	}
	var rules []*rule
	for _, r := range s.rules {
		if r.appliesTo(t) {
			rules = append(rules, r)
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return &Rules{rules: rules}
}

// Preview redacts sample text line by line with every rule, ignoring
// targeting. A nil Set returns the sample unchanged.
func (s *Set) Preview(sample string) string {
	if s == nil {
		return sample
	}
	all := &Rules{rules: s.rules}
	lines := strings.Split(sample, "\n")
	for i, line := range lines {
		lines[i] = all.Line(line)
	}
	return strings.Join(lines, "\n")
}

// Compile validates cfg and builds its rules. Every rule is validated, even
// disabled ones, so a broken rule cannot be saved and enabled later. It
// returns a nil Set when redaction is off or no rule is enabled.
func Compile(cfg config.RedactionConfig) (*Set, error) {
	replacement := cfg.Replacement
	if replacement == "" {
		replacement = DefaultReplacement
	}
	// A literal replacement must not expand $ references.
	literal := strings.ReplaceAll(replacement, "$", "$$")

	for _, id := range cfg.Builtins {
		if !slices.ContainsFunc(builtins, func(b builtin) bool { return b.ID == id }) {
			return nil, fmt.Errorf("unknown built-in pattern %q", id)
		}
	}
	var rules []*rule
	// Built-ins run first, in catalog order: the specific ones (an
	// Authorization header) before the broad ones (any bearer token).
	for _, b := range builtins {
		if slices.Contains(cfg.Builtins, b.ID) {
			rules = append(rules, &rule{
				re:          b.re,
				replacement: strings.ReplaceAll(b.template, "{}", literal),
				hints:       b.hints,
				fold:        b.fold,
				lead:        b.lead,
			})
		}
	}

	for i, r := range cfg.Rules {
		label := fmt.Sprintf("rule %d", i+1)
		if name := strings.TrimSpace(r.Name); name != "" {
			label = fmt.Sprintf("rule %q", name)
		}
		if r.Pattern == "" {
			return nil, fmt.Errorf("%s: pattern is required", label)
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid pattern: %w", label, err)
		}
		if matchesEmpty(r.Pattern) {
			return nil, fmt.Errorf("%s: pattern can match empty text, which would insert the replacement between characters", label)
		}
		if !r.Enabled {
			continue
		}
		c := &rule{
			re:          re,
			replacement: r.Replacement,
			hosts:       r.Hosts,
			containers:  r.Containers,
			projects:    r.Projects,
		}
		if c.replacement == "" {
			c.replacement = literal
		}
		if prefix, _ := re.LiteralPrefix(); prefix != "" {
			c.hints = []string{prefix}
		}
		rules = append(rules, c)
	}

	if !cfg.Enabled || len(rules) == 0 {
		return nil, nil
	}
	s := &Set{rules: rules}
	if !slices.ContainsFunc(rules, (*rule).targeted) {
		s.all = &Rules{rules: rules}
	}
	return s, nil
}

// matchesEmpty reports whether pattern can match the empty string anywhere,
// counting every zero-width assertion as satisfiable. Such a pattern would
// make ReplaceAllString insert the replacement between characters.
func matchesEmpty(pattern string) bool {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return false
	}
	return canBeEmpty(re)
}

func canBeEmpty(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpEmptyMatch, syntax.OpStar, syntax.OpQuest,
		syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	case syntax.OpLiteral:
		return len(re.Rune) == 0
	case syntax.OpCapture, syntax.OpPlus:
		return canBeEmpty(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min == 0 || canBeEmpty(re.Sub[0])
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			if !canBeEmpty(sub) {
				return false
			}
		}
		return true
	case syntax.OpAlternate:
		return slices.ContainsFunc(re.Sub, canBeEmpty)
	}
	return false
}

// appendLower appends s to dst with ASCII letters lower-cased. Byte offsets
// stay the same, so an index into the result is an index into s.
func appendLower(dst []byte, s string) []byte {
	n := len(dst)
	dst = slices.Grow(dst, len(s))[:n+len(s)]
	out := dst[n:]
	for i := range len(s) {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return dst
}

var active atomic.Pointer[Set]

// Activate makes s the rules every log read applies from its next line on. A
// nil Set turns redaction off.
func Activate(s *Set) {
	active.Store(s)
}

// Active returns the rules in effect, or nil when redaction is off.
func Active() *Set {
	return active.Load()
}

// Stream redacts one container's lines with whatever rules are active. It
// resolves which rules apply only when the active Set changes, so a settings
// change reaches streams that are already open. A Stream is not safe for
// concurrent use. A nil Stream returns lines unchanged.
type Stream struct {
	target  Target
	set     *Set
	rules   *Rules
	scratch []byte
}

// NewStream returns a Stream for one container's lines.
func NewStream(t Target) *Stream {
	return &Stream{target: t}
}

// Line redacts one physical log line.
func (s *Stream) Line(line string) string {
	if s == nil {
		return line
	}
	if set := active.Load(); set != s.set {
		s.set, s.rules = set, set.For(s.target)
	}
	if s.rules == nil {
		return line
	}
	return s.rules.line(line, &s.scratch)
}
