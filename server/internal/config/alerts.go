package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// AlertsConfig holds the alerting settings persisted in the config file:
// the notification channels and the list of alert rules.
type AlertsConfig struct {
	// WebhookURL is the pre-channels single-webhook field. It is migrated into
	// a webhook channel on load and cleared; it remains only so old configs
	// keep parsing.
	WebhookURL string         `json:"webhookUrl,omitempty"`
	Channels   []AlertChannel `json:"channels,omitempty"`
	Rules      []AlertRule    `json:"rules,omitempty"`
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
	Type    string `json:"type"` // "event" | "log"

	// Targeting: all optional, empty = match-all in that dimension, ANDed.
	Hosts      []string `json:"hosts,omitempty"`
	Containers []string `json:"containers,omitempty"` // exact container names
	Projects   []string `json:"projects,omitempty"`   // compose projects

	// Event rules.
	Events []string `json:"events,omitempty"` // "die" | "oom" | "unhealthy"

	// Log rules.
	MinLevel string `json:"minLevel,omitempty"`
	Pattern  string `json:"pattern,omitempty"` // RE2

	// Rate + cooldown (both rule types).
	Threshold       int    `json:"threshold"`
	WindowSeconds   int    `json:"windowSeconds,omitempty"`
	CooldownSeconds int    `json:"cooldownSeconds,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// migrateAlertChannels folds a legacy single WebhookURL into a webhook channel
// (enabled) at the front of the channel list and clears WebhookURL. It reports
// whether it changed the config so the caller can persist the result once.
func migrateAlertChannels(a *AlertsConfig) bool {
	if a == nil || a.WebhookURL == "" {
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
