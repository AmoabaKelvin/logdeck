package alerts

import (
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	defaultWindow = 60 * time.Second
	// defaultCooldown applies when CooldownSeconds is 0. The JSON schema
	// cannot distinguish an absent field from an explicit 0, so 0 means "use
	// the default"; a rule that wants effectively no cooldown sets 1.
	defaultCooldown = 300 * time.Second
)

// compiledRule is an immutable, normalized snapshot of one enabled alert
// rule. Sink closures capture it by pointer; reconciles build fresh ones and
// never mutate rules already handed out.
type compiledRule struct {
	id   string
	name string
	typ  string // "event" | "log" | "collection" | "resource"

	// spec is the rule's container targeting; matching is delegated to
	// logstream.ContainerSpec.Matches.
	spec logstream.ContainerSpec

	events []string // event rules: "die" | "oom" | "unhealthy" | "restart"

	minLevel    string // normalized upper-case, "" when unset
	minSeverity int    // >= 1 when minLevel is set; UNKNOWN (0) never passes
	pattern     *regexp.Regexp

	// Resource rules: metric must stay above abovePercent, or for memory
	// aboveBytes when set.
	metric       string // "cpu" | "memory"
	abovePercent float64
	aboveBytes   uint64

	threshold int
	// window is the rate window, or for collection and resource rules how
	// long the condition must last before it fires.
	window   time.Duration
	cooldown time.Duration

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
		if rule.Type != "event" && rule.Type != "log" && rule.Type != "collection" && rule.Type != "resource" {
			log.Printf("alerts: rule %q (%s): unknown type %q, skipping", rule.Name, rule.ID, rule.Type)
			continue
		}

		c := &compiledRule{
			id:           rule.ID,
			name:         rule.Name,
			typ:          rule.Type,
			spec:         logstream.ContainerSpec{Hosts: rule.Hosts, Containers: rule.Containers, Projects: rule.Projects},
			events:       rule.Events,
			metric:       rule.Metric,
			abovePercent: rule.AbovePercent,
			threshold:    rule.Threshold,
			window:       time.Duration(rule.WindowSeconds) * time.Second,
			cooldown:     time.Duration(rule.CooldownSeconds) * time.Second,
			src:          rule,
		}
		if rule.AboveBytes > 0 {
			c.aboveBytes = uint64(rule.AboveBytes)
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

func (r *compiledRule) hasEvent(action string) bool {
	return slices.Contains(r.events, action)
}

// usage reports a resource rule's sampled value, formatted for the alert, and
// whether it is over the rule's limit.
func (r *compiledRule) usage(stats *models.ContainerStats) (string, bool) {
	switch {
	case r.metric == "cpu":
		return fmt.Sprintf("%.0f%%", stats.CPUPercent), stats.CPUPercent > r.abovePercent
	case r.aboveBytes > 0:
		return formatBytes(stats.MemoryUsed), stats.MemoryUsed > r.aboveBytes
	default:
		return fmt.Sprintf("%.0f%%", stats.MemoryPercent), stats.MemoryPercent > r.abovePercent
	}
}

// limit renders a resource rule's metric and limit for alert reasons, e.g.
// "CPU" and "150%".
func (r *compiledRule) limit() (metric, value string) {
	if r.metric == "cpu" {
		return "CPU", fmt.Sprintf("%g%%", r.abovePercent)
	}
	if r.aboveBytes > 0 {
		return "memory", formatBytes(r.aboveBytes)
	}
	return "memory", fmt.Sprintf("%g%%", r.abovePercent)
}

// formatBytes renders a byte count with binary units, e.g. "1.5 GiB".
func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TiB", value/unit)
}
