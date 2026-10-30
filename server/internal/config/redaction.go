package config

// RedactionConfig holds the log redaction settings persisted in the config
// file under "redaction". Like alerts, it lives only in the file config.
type RedactionConfig struct {
	Enabled bool `json:"enabled"`
	// Replacement stands in for every redacted value unless a rule sets its
	// own. Empty means the redact package's default.
	Replacement string `json:"replacement,omitempty"`
	// Builtins lists the enabled built-in patterns by ID. Nil means the
	// settings were never saved, so the defaults are offered.
	Builtins []string        `json:"builtins"`
	Rules    []RedactionRule `json:"rules"`
}

// RedactionRule is one custom pattern. Rules run in list order, after the
// built-in patterns.
type RedactionRule struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Pattern string `json:"pattern"` // RE2
	// Replacement overrides the config's replacement; $1 expands a capture
	// group, so a rule can keep a key and hide only its value.
	Replacement string `json:"replacement,omitempty"`

	// Targeting: all optional, empty = match-all in that dimension, ANDed,
	// with the same semantics as alert rules.
	Hosts      []string `json:"hosts,omitempty"`
	Containers []string `json:"containers,omitempty"` // exact container names
	Projects   []string `json:"projects,omitempty"`   // compose projects
}

// Redaction returns the stored redaction settings, or the zero value when
// none were saved.
func (m *Manager) Redaction() RedactionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.fileConfig.Redaction == nil {
		return RedactionConfig{}
	}
	return *m.fileConfig.Redaction
}

// SetRedaction replaces the stored redaction settings. The caller validates
// them and activates the compiled rules; nothing here is re-read per line.
func (m *Manager) SetRedaction(cfg RedactionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.fileConfig.Redaction
	m.fileConfig.Redaction = &cfg
	if err := m.persist(); err != nil {
		m.fileConfig.Redaction = old
		return err
	}
	return nil
}
