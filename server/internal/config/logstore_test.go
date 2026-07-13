package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeLogStoreConfig(t *testing.T, fc FileConfig) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_PATH", path)
	return NewManager()
}

func TestLogStoreDefaults(t *testing.T) {
	manager := writeLogStoreConfig(t, FileConfig{})

	got := manager.LogStore()
	want := ResolvedLogStoreConfig{
		Enabled:        true,
		PerContainerMB: DefaultLogStorePerContainerMB,
		TotalMB:        DefaultLogStoreTotalMB,
	}
	if got != want {
		t.Fatalf("LogStore() = %+v, want %+v", got, want)
	}
}

func TestLogStoreFileConfig(t *testing.T) {
	disabled := false
	perContainer := 10
	manager := writeLogStoreConfig(t, FileConfig{
		LogStore: &LogStoreConfig{Enabled: &disabled, PerContainerMB: &perContainer},
	})

	got := manager.LogStore()
	if got.Enabled {
		t.Fatal("Enabled = true, want the file's false")
	}
	if got.PerContainerMB != 10 {
		t.Fatalf("PerContainerMB = %d, want 10", got.PerContainerMB)
	}
	if got.TotalMB != DefaultLogStoreTotalMB {
		t.Fatalf("TotalMB = %d, want the default", got.TotalMB)
	}
}

func TestLogStoreEnvOverridesFile(t *testing.T) {
	enabled := true
	perContainer := 10
	total := 20
	manager := writeLogStoreConfig(t, FileConfig{
		LogStore: &LogStoreConfig{Enabled: &enabled, PerContainerMB: &perContainer, TotalMB: &total},
	})

	t.Setenv("LOG_STORE_ENABLED", "false")
	t.Setenv("LOG_STORE_PER_CONTAINER_MB", "5")
	t.Setenv("LOG_STORE_TOTAL_MB", "500")

	got := manager.LogStore()
	want := ResolvedLogStoreConfig{Enabled: false, PerContainerMB: 5, TotalMB: 500}
	if got != want {
		t.Fatalf("LogStore() = %+v, want the env values %+v", got, want)
	}
}

func TestLogStoreIgnoresInvalidEnv(t *testing.T) {
	manager := writeLogStoreConfig(t, FileConfig{})

	t.Setenv("LOG_STORE_ENABLED", "yes please")
	t.Setenv("LOG_STORE_PER_CONTAINER_MB", "-1")

	got := manager.LogStore()
	if !got.Enabled || got.PerContainerMB != DefaultLogStorePerContainerMB {
		t.Fatalf("LogStore() = %+v, want invalid env values ignored", got)
	}
}

func TestUpdateLogStorePersists(t *testing.T) {
	manager := writeLogStoreConfig(t, FileConfig{})

	if err := manager.UpdateLogStore(func(current LogStoreConfig) (LogStoreConfig, error) {
		total := 256
		current.TotalMB = &total
		return current, nil
	}); err != nil {
		t.Fatalf("UpdateLogStore: %v", err)
	}

	if got := manager.LogStore().TotalMB; got != 256 {
		t.Fatalf("TotalMB = %d, want 256", got)
	}

	reloaded := NewManager() // reads the file written above via CONFIG_PATH
	if got := reloaded.LogStore().TotalMB; got != 256 {
		t.Fatalf("after reload TotalMB = %d, want 256", got)
	}
}
