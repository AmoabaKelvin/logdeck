package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// AlertsConfig holds the alerting settings persisted in the config file:
// the notification channels, the list of alert rules, and the silences that
// mute them.
type AlertsConfig struct {
	// WebhookURL is the pre-channels single-webhook field. It is migrated into
	// a webhook channel on load and cleared; it remains only so old configs
	// keep parsing.
	WebhookURL string         `json:"webhookUrl,omitempty"`
	Channels   []AlertChannel `json:"channels,omitempty"`
	Rules      []AlertRule    `json:"rules,omitempty"`
	Silences   []AlertSilence `json:"silences,omitempty"`
}

// AlertSilence mutes the alerts it matches while it is active: they are kept
// in history as muted and sent nowhere. A silence is either one-off (StartsAt
// to EndsAt) or a weekly Schedule.
type AlertSilence struct {
	ID      string `json:"id"`
	Comment string `json:"comment,omitempty"`

	// Matchers: all optional, empty = match-all in that dimension, ANDed. A
	// silence with none mutes every alert.
	RuleID    string `json:"ruleId,omitempty"`
	Host      string `json:"host,omitempty"`
	Container string `json:"container,omitempty"` // exact container name
	Project   string `json:"project,omitempty"`   // compose project

	// One-off: RFC3339 times. An empty StartsAt starts at once, an empty
	// EndsAt lasts until the silence is deleted.
	StartsAt string `json:"startsAt,omitempty"`
	EndsAt   string `json:"endsAt,omitempty"`

	// Schedule makes the silence a recurring maintenance window instead.
	Schedule *SilenceSchedule `json:"schedule,omitempty"`

	CreatedAt string `json:"createdAt"`
}

// SilenceSchedule is a weekly maintenance window: it starts at Start on each
// of Days and lasts DurationMinutes (at most a day).
type SilenceSchedule struct {
	Days            []string `json:"days,omitempty"` // "mon".."sun"; empty = every day
	Start           string   `json:"start"`          // "15:04"
	DurationMinutes int      `json:"durationMinutes"`
	Timezone        string   `json:"timezone,omitempty"` // IANA name; empty = UTC
}

// AlertChannel is one notification destination. A fired alert is delivered to
// every enabled channel.
type AlertChannel struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "webhook" | "ntfy" | "gotify" | "telegram"
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`    // webhook: full URL; ntfy: topic URL; gotify: server base URL
	Token   string `json:"token,omitempty"`  // gotify: app token; telegram: bot token
	Target  string `json:"target,omitempty"` // telegram: chat_id
}

type AlertRule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"` // "event" | "log" | "collection" | "resource"

	// Targeting: all optional, empty = match-all in that dimension, ANDed.
	// Collection rules target hosts only.
	Hosts      []string `json:"hosts,omitempty"`
	Containers []string `json:"containers,omitempty"` // exact container names
	Projects   []string `json:"projects,omitempty"`   // compose projects

	// Event rules.
	Events []string `json:"events,omitempty"` // "die" | "oom" | "unhealthy" | "restart"

	// Log rules.
	MinLevel string `json:"minLevel,omitempty"`
	Pattern  string `json:"pattern,omitempty"` // RE2

	// Resource rules: the metric must stay above the limit for WindowSeconds.
	// AbovePercent is CPU as docker stats shows it (100 = one core), or memory
	// as a share of the container's memory limit. Memory can set AboveBytes
	// instead.
	Metric       string  `json:"metric,omitempty"` // "cpu" | "memory"
	AbovePercent float64 `json:"abovePercent,omitempty"`
	AboveBytes   int64   `json:"aboveBytes,omitempty"`

	// Rate + cooldown (event and log rules). Collection and resource rules use
	// WindowSeconds as how long the condition must last before it fires.
	Threshold       int    `json:"threshold"`
	WindowSeconds   int    `json:"windowSeconds,omitempty"`
	CooldownSeconds int    `json:"cooldownSeconds,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// migrateAlertChannels folds a legacy single WebhookURL into a webhook channel
// (enabled) at the front of the channel list and clears WebhookURL. It reports
// whether it changed the config so the caller can persist the result once.
func migrateAlertChannels(a *AlertsConfig) bool {
	if a.WebhookURL == "" {
		return false
	}
	a.Channels = append([]AlertChannel{{
		ID:      newChannelID(),
		Type:    "webhook",
		Name:    "Webhook",
		Enabled: true,
		URL:     a.WebhookURL,
	}}, a.Channels...)
	a.WebhookURL = ""
	return true
}

// newChannelID returns a random 8-hex-char channel ID.
func newChannelID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%08x", uint32(time.Now().UnixNano()))
	}
	return hex.EncodeToString(b[:])
}

// UpdateAlerts applies a mutation function to the stored alerts config
// atomically. Alerts live only in the file config (never env), so no remerge
// is needed — readers access them through FileConfigSnapshot.
func (m *Manager) UpdateAlerts(mutate func(current AlertsConfig) (AlertsConfig, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clone to prevent mutate from modifying the live config in-place.
	current := AlertsConfig{}
	if m.fileConfig.Alerts != nil {
		current.WebhookURL = m.fileConfig.Alerts.WebhookURL
		current.Channels = make([]AlertChannel, len(m.fileConfig.Alerts.Channels))
		copy(current.Channels, m.fileConfig.Alerts.Channels)
		current.Rules = make([]AlertRule, len(m.fileConfig.Alerts.Rules))
		copy(current.Rules, m.fileConfig.Alerts.Rules)
		current.Silences = make([]AlertSilence, len(m.fileConfig.Alerts.Silences))
		copy(current.Silences, m.fileConfig.Alerts.Silences)
	}

	updated, err := mutate(current)
	if err != nil {
		return err
	}

	old := m.fileConfig.Alerts
	m.fileConfig.Alerts = &updated
	if err := m.persist(); err != nil {
		m.fileConfig.Alerts = old
		return err
	}
	return nil
}
