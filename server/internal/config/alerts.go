package config

// AlertsConfig holds the alerting settings persisted in the config file:
// the webhook destination and the list of alert rules.
type AlertsConfig struct {
	WebhookURL string      `json:"webhookUrl,omitempty"`
	Rules      []AlertRule `json:"rules,omitempty"`
}

// AlertRule describes one alerting rule.
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
