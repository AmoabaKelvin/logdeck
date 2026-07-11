package alerts

import (
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	defaultWindow = 60 * time.Second
	// defaultCooldown applies when CooldownSeconds is 0. The JSON schema
	// cannot distinguish an absent field from an explicit 0, so 0 means "use
	// the default"; a rule that wants effectively no cooldown sets 1.
	defaultCooldown = 300 * time.Second
)

// Docker Compose and recent podman-compose both set the com.docker label;
// older podman-compose releases only set the io.podman one. Mirrors the
// helper in internal/logstream.
var composeProjectLabels = []string{
	"com.docker.compose.project",
	"io.podman.compose.project",
}

// compiledRule is an immutable, normalized snapshot of one enabled alert
// rule. Sink closures capture it by pointer; reconciles build fresh ones and
// never mutate rules already handed out.
type compiledRule struct {
	id   string
	name string
	typ  string // "event" | "log"

	hosts      []string
	containers []string
	projects   []string

	events []string // event rules: "die" | "oom"

	minLevel    string // normalized upper-case, "" when unset
	minSeverity int    // >= 1 when minLevel is set; UNKNOWN (0) never passes
	pattern     *regexp.Regexp

	threshold int
	window    time.Duration
	cooldown  time.Duration

	// src is the original rule as configured, kept for change detection
	// across reconciles.
	src config.AlertRule
}

// compileRules turns the configured rules into compiled ones. Disabled rules,
// rules with an unknown type, and rules with an invalid pattern are skipped
// (the latter two with a log line). Defaults are normalized here so the rest
// of the engine never re-checks them.
func compileRules(cfg config.AlertsConfig) []*compiledRule {
	compiled := make([]*compiledRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.Type != "event" && rule.Type != "log" {
			log.Printf("alerts: rule %q (%s): unknown type %q, skipping", rule.Name, rule.ID, rule.Type)
			continue
		}

		c := &compiledRule{
			id:         rule.ID,
			name:       rule.Name,
			typ:        rule.Type,
			hosts:      rule.Hosts,
			containers: rule.Containers,
			projects:   rule.Projects,
			events:     rule.Events,
			threshold:  rule.Threshold,
			window:     time.Duration(rule.WindowSeconds) * time.Second,
			cooldown:   time.Duration(rule.CooldownSeconds) * time.Second,
			src:        rule,
		}
		if c.threshold < 1 {
			c.threshold = 1
		}
		if c.window <= 0 {
			c.window = defaultWindow
		}
		if c.cooldown <= 0 {
			c.cooldown = defaultCooldown
		}

		if level := strings.ToUpper(strings.TrimSpace(rule.MinLevel)); level != "" {
			c.minLevel = level
			c.minSeverity = models.LevelSeverity(models.LogLevel(level))
			if c.minSeverity < 1 {
				// Unrecognized level: require at least TRACE so UNKNOWN
				// entries (severity 0) still never pass a set MinLevel.
				c.minSeverity = 1
			}
		}
		if rule.Pattern != "" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				log.Printf("alerts: rule %q (%s): invalid pattern: %v, skipping", rule.Name, rule.ID, err)
				continue
			}
			c.pattern = re
		}

		compiled = append(compiled, c)
	}
	return compiled
}

// matchesTarget reports whether a container (host, name without the leading
// "/", labels) is selected by the rule's targeting. All dimensions are
// optional; hosts are ANDed with the container/project dimension, which is an
// OR between exact names and compose projects.
func (r *compiledRule) matchesTarget(host, name string, labels map[string]string) bool {
	if len(r.hosts) > 0 && !slices.Contains(r.hosts, host) {
		return false
	}
	if len(r.containers) == 0 && len(r.projects) == 0 {
		return true
	}
	if slices.Contains(r.containers, name) {
		return true
	}
	for _, project := range r.projects {
		for _, label := range composeProjectLabels {
			if labels[label] == project {
				return true
			}
		}
	}
	return false
}

// matchesEntry reports whether a log entry satisfies the rule's level and
// pattern filters. Both filters are optional and ANDed.
func (r *compiledRule) matchesEntry(entry models.LogEntry) bool {
	if r.minSeverity > 0 && models.LevelSeverity(entry.Level) < r.minSeverity {
		return false
	}
	if r.pattern != nil && !r.pattern.MatchString(entry.Message) && !r.pattern.MatchString(entry.Raw) {
		return false
	}
	return true
}

// hasEvent reports whether the rule watches the given event action.
func (r *compiledRule) hasEvent(action string) bool {
	return slices.Contains(r.events, action)
}
