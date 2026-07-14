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
			resolved.PerContainerMB = positiveMB("logStore.perContainerMB",
				*file.PerContainerMB, DefaultLogStorePerContainerMB)
		}
		if file.TotalMB != nil {
			resolved.TotalMB = positiveMB("logStore.totalMB", *file.TotalMB, DefaultLogStoreTotalMB)
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

// LogStoreSources tracks where each effective log store value came from. The
// three fields are overridden independently, so unlike the other categories
// they carry a source each rather than one for the whole section.
type LogStoreSources struct {
	Enabled        Source `json:"enabled"`
	PerContainerMB Source `json:"perContainerMB"`
	TotalMB        Source `json:"totalMB"`
}

// LogStoreSources reports which log store values are pinned by an environment
// variable. A field only counts as env-sourced when its variable parses — an
// invalid value is ignored by LogStore, so it must not be reported as env
// either. Everything else reads as file (defaults included), matching the
// convention the other settings use.
func (m *Manager) LogStoreSources() LogStoreSources {
	sources := LogStoreSources{
		Enabled:        SourceFile,
		PerContainerMB: SourceFile,
		TotalMB:        SourceFile,
	}
	if _, ok := envBool("LOG_STORE_ENABLED"); ok {
		sources.Enabled = SourceEnv
	}
	if _, ok := envPositiveInt("LOG_STORE_PER_CONTAINER_MB"); ok {
		sources.PerContainerMB = SourceEnv
	}
	if _, ok := envPositiveInt("LOG_STORE_TOTAL_MB"); ok {
		sources.TotalMB = SourceEnv
	}
	return sources
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

// positiveMB rejects a non-positive retention cap from the config file. A cap of
// zero or less would make every janitor pass evict the whole store, so it falls
// back to the default rather than quietly deleting the user's logs.
func positiveMB(field string, value, fallback int) int {
	if value <= 0 {
		log.Printf("Warning: ignoring %s=%d (expected a positive integer), using %d", field, value, fallback)
		return fallback
	}
	return value
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
