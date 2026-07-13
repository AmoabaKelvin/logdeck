package config

import (
	"log"
	"os"
	"strconv"
)

// Default log store limits. Retention is generous enough to be useful out of
// the box while keeping the SQLite file bounded on a small VPS.
const (
	DefaultLogStorePerContainerMB = 50
	DefaultLogStoreTotalMB        = 1024
)

// LogStoreConfig holds the log persistence settings. Persisted in the config
// file under "logStore"; each field can be overridden by an environment
// variable, following the manager's env-over-file pattern.
type LogStoreConfig struct {
	Enabled        *bool `json:"enabled,omitempty"`
	PerContainerMB *int  `json:"perContainerMB,omitempty"`
	TotalMB        *int  `json:"totalMB,omitempty"`
}

// ResolvedLogStoreConfig is the effective log store configuration after the
// env-over-file merge, with defaults applied.
type ResolvedLogStoreConfig struct {
	Enabled        bool `json:"enabled"`
	PerContainerMB int  `json:"perContainerMB"`
	TotalMB        int  `json:"totalMB"`
}

// LogStore returns the effective log store settings: environment variables
// win over the config file, which wins over the defaults (enabled, 50 MB per
// container, 1024 MB total).
func (m *Manager) LogStore() ResolvedLogStoreConfig {
	m.mu.RLock()
	file := m.fileConfig.LogStore
	m.mu.RUnlock()

	resolved := ResolvedLogStoreConfig{
		Enabled:        true,
		PerContainerMB: DefaultLogStorePerContainerMB,
		TotalMB:        DefaultLogStoreTotalMB,
	}

	if file != nil {
		if file.Enabled != nil {
			resolved.Enabled = *file.Enabled
		}
		if file.PerContainerMB != nil {
			resolved.PerContainerMB = *file.PerContainerMB
		}
		if file.TotalMB != nil {
			resolved.TotalMB = *file.TotalMB
		}
	}

	if v, ok := envBool("LOG_STORE_ENABLED"); ok {
		resolved.Enabled = v
	}
	if v, ok := envPositiveInt("LOG_STORE_PER_CONTAINER_MB"); ok {
		resolved.PerContainerMB = v
	}
	if v, ok := envPositiveInt("LOG_STORE_TOTAL_MB"); ok {
		resolved.TotalMB = v
	}

	return resolved
}

// UpdateLogStore applies a mutation function to the stored log store config
// atomically. The store reads its limits through the manager on every janitor
// pass, so no remerge or callback is needed.
func (m *Manager) UpdateLogStore(mutate func(current LogStoreConfig) (LogStoreConfig, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := LogStoreConfig{}
	if m.fileConfig.LogStore != nil {
		current = *m.fileConfig.LogStore
	}

	updated, err := mutate(current)
	if err != nil {
		return err
	}

	old := m.fileConfig.LogStore
	m.fileConfig.LogStore = &updated
	if err := m.persist(); err != nil {
		m.fileConfig.LogStore = old
		return err
	}
	return nil
}

func envBool(key string) (bool, bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("Warning: ignoring %s=%q (expected a boolean)", key, raw)
		return false, false
	}
	return value, true
}

func envPositiveInt(key string) (int, bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		log.Printf("Warning: ignoring %s=%q (expected a positive integer)", key, raw)
		return 0, false
	}
	return value, true
}
