package alerts

import (
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func TestCompileRulesNormalizationAndSkips(t *testing.T) {
	cfg := config.AlertsConfig{Rules: []config.AlertRule{
		{ID: "a", Enabled: true, Type: "log"},
		{ID: "b", Enabled: false, Type: "log"},
		{ID: "c", Enabled: true, Type: "log", Pattern: "("},
		{ID: "d", Enabled: true, Type: "weird"},
		{ID: "e", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 5, WindowSeconds: 120, CooldownSeconds: 1},
	}}

	compiled := compileRules(cfg)
	if len(compiled) != 2 {
		t.Fatalf("compiled %d rules, want 2 (disabled, invalid-regex, and unknown-type skipped)", len(compiled))
	}

	a := compiled[0]
	if a.id != "a" || a.threshold != 1 || a.window != 60*time.Second || a.cooldown != 300*time.Second {
		t.Fatalf("rule a defaults not applied: %+v", a)
	}
	e := compiled[1]
	if e.id != "e" || e.threshold != 5 || e.window != 120*time.Second || e.cooldown != time.Second {
		t.Fatalf("rule e values not preserved: %+v", e)
	}
	if !e.hasEvent("die") || e.hasEvent("oom") {
		t.Fatal("rule e event set wrong")
	}
}

func compileOne(t *testing.T, rule config.AlertRule) *compiledRule {
	t.Helper()
	rule.Enabled = true
	compiled := compileRules(config.AlertsConfig{Rules: []config.AlertRule{rule}})
	if len(compiled) != 1 {
		t.Fatalf("rule did not compile: %+v", rule)
	}
	return compiled[0]
}

func TestMatchesTargetMatrix(t *testing.T) {
	dockerLabels := map[string]string{"com.docker.compose.project": "shop"}
	podmanLabels := map[string]string{"io.podman.compose.project": "shop"}

	tests := []struct {
		name   string
		rule   config.AlertRule
		host   string
		cname  string
		labels map[string]string
		want   bool
	}{
		{"all empty matches everything", config.AlertRule{Type: "log"}, "h1", "web", nil, true},
		{"host match", config.AlertRule{Type: "log", Hosts: []string{"h1"}}, "h1", "web", nil, true},
		{"host mismatch", config.AlertRule{Type: "log", Hosts: []string{"h1"}}, "h2", "web", nil, false},
		{"container match", config.AlertRule{Type: "log", Containers: []string{"web"}}, "h1", "web", nil, true},
		{"container mismatch", config.AlertRule{Type: "log", Containers: []string{"web"}}, "h1", "db", nil, false},
		{"project via docker label", config.AlertRule{Type: "log", Projects: []string{"shop"}}, "h1", "db", dockerLabels, true},
		{"project via podman label", config.AlertRule{Type: "log", Projects: []string{"shop"}}, "h1", "db", podmanLabels, true},
		{"project mismatch", config.AlertRule{Type: "log", Projects: []string{"shop"}}, "h1", "db", map[string]string{"com.docker.compose.project": "other"}, false},
		{"container OR project: name miss, project hit", config.AlertRule{Type: "log", Containers: []string{"api"}, Projects: []string{"shop"}}, "h1", "db", dockerLabels, true},
		{"container OR project: both miss", config.AlertRule{Type: "log", Containers: []string{"api"}, Projects: []string{"blog"}}, "h1", "db", dockerLabels, false},
		{"host AND container: host miss", config.AlertRule{Type: "log", Hosts: []string{"h1"}, Containers: []string{"web"}}, "h2", "web", nil, false},
		{"host AND container: both hit", config.AlertRule{Type: "log", Hosts: []string{"h1"}, Containers: []string{"web"}}, "h1", "web", nil, true},
		{"host only: any container on host", config.AlertRule{Type: "log", Hosts: []string{"h1"}}, "h1", "anything", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := compileOne(t, tt.rule)
			if got := rule.spec.Matches(tt.host, tt.cname, tt.labels); got != tt.want {
				t.Fatalf("spec.Matches(%s, %s, %v) = %v, want %v", tt.host, tt.cname, tt.labels, got, tt.want)
			}
		})
	}
}

func entryWith(level models.LogLevel, message string) models.LogEntry {
	return models.LogEntry{Level: level, Message: message, Raw: message}
}

func TestMatchesEntry(t *testing.T) {
	tests := []struct {
		name  string
		rule  config.AlertRule
		entry models.LogEntry
		want  bool
	}{
		{"no filters match everything", config.AlertRule{Type: "log"}, entryWith(models.LogLevelUnknown, "anything"), true},
		{"minLevel pass at level", config.AlertRule{Type: "log", MinLevel: "ERROR"}, entryWith(models.LogLevelError, "boom"), true},
		{"minLevel pass above level", config.AlertRule{Type: "log", MinLevel: "ERROR"}, entryWith(models.LogLevelFatal, "boom"), true},
		{"minLevel fail below level", config.AlertRule{Type: "log", MinLevel: "ERROR"}, entryWith(models.LogLevelWarn, "boom"), false},
		{"UNKNOWN never passes a set minLevel", config.AlertRule{Type: "log", MinLevel: "TRACE"}, entryWith(models.LogLevelUnknown, "boom"), false},
		{"unrecognized minLevel still blocks UNKNOWN", config.AlertRule{Type: "log", MinLevel: "NOPE"}, entryWith(models.LogLevelUnknown, "boom"), false},
		{"unrecognized minLevel passes TRACE", config.AlertRule{Type: "log", MinLevel: "NOPE"}, entryWith(models.LogLevelTrace, "boom"), true},
		{"lowercase minLevel normalized", config.AlertRule{Type: "log", MinLevel: "error"}, entryWith(models.LogLevelError, "boom"), true},
		{"pattern match on message", config.AlertRule{Type: "log", Pattern: "time.?out"}, entryWith(models.LogLevelUnknown, "request timeout"), true},
		{"pattern match on raw only", config.AlertRule{Type: "log", Pattern: "^2026"}, models.LogEntry{Level: models.LogLevelInfo, Message: "started", Raw: "2026-07-10 started"}, true},
		{"pattern no match", config.AlertRule{Type: "log", Pattern: "timeout"}, entryWith(models.LogLevelError, "connection refused"), false},
		{"level and pattern both required", config.AlertRule{Type: "log", MinLevel: "ERROR", Pattern: "timeout"}, entryWith(models.LogLevelError, "connection refused"), false},
		{"level and pattern both pass", config.AlertRule{Type: "log", MinLevel: "ERROR", Pattern: "timeout"}, entryWith(models.LogLevelError, "timeout talking to db"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := compileOne(t, tt.rule)
			if got := rule.matchesEntry(tt.entry); got != tt.want {
				t.Fatalf("matchesEntry(%+v) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}
