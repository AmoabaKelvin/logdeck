package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestMigrateLegacyWebhookURL verifies a pre-channels config with a WebhookURL
// is folded into a webhook channel on load, the WebhookURL is cleared, and the
// migration is persisted once so a second load is a no-op.
func TestMigrateLegacyWebhookURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CONFIG_PATH", path)

	legacy := FileConfig{Alerts: &AlertsConfig{
		WebhookURL: "https://example.com/hook",
		Rules:      []AlertRule{{ID: "r1", Name: "boom", Enabled: true, Type: "log", MinLevel: "ERROR", Threshold: 1, CreatedAt: "2026-07-11T00:00:00Z"}},
	}}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	m := NewManager()
	fc := m.FileConfigSnapshot()
	if fc.Alerts == nil {
		t.Fatal("expected alerts config after load")
	}
	if fc.Alerts.WebhookURL != "" {
		t.Errorf("expected WebhookURL cleared after migration, got %q", fc.Alerts.WebhookURL)
	}
	if len(fc.Alerts.Channels) != 1 {
		t.Fatalf("expected 1 migrated channel, got %d", len(fc.Alerts.Channels))
	}
	ch := fc.Alerts.Channels[0]
	if ch.Type != "webhook" || !ch.Enabled || ch.URL != "https://example.com/hook" || ch.ID == "" {
		t.Errorf("migrated channel wrong: %+v", ch)
	}
	if len(fc.Alerts.Rules) != 1 {
		t.Errorf("rules must survive migration, got %+v", fc.Alerts.Rules)
	}

	// The migration must be persisted: the on-disk file no longer has webhookUrl.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk FileConfig
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Alerts.WebhookURL != "" {
		t.Errorf("persisted config still has webhookUrl: %q", onDisk.Alerts.WebhookURL)
	}
	if len(onDisk.Alerts.Channels) != 1 {
		t.Errorf("persisted config missing migrated channel: %+v", onDisk.Alerts.Channels)
	}

	// A second load is a no-op: the channel id is stable (not re-migrated).
	reloaded := NewManager()
	rc := reloaded.FileConfigSnapshot()
	if len(rc.Alerts.Channels) != 1 || rc.Alerts.Channels[0].ID != ch.ID {
		t.Errorf("second load must not re-migrate, got %+v", rc.Alerts.Channels)
	}
}

func TestUpdateAlertsRoundTrip(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rules := []AlertRule{
		{
			ID:              "rule-1",
			Name:            "OOM kills",
			Enabled:         true,
			Type:            "event",
			Hosts:           []string{"local"},
			Events:          []string{"oom", "die"},
			Threshold:       1,
			CooldownSeconds: 300,
			CreatedAt:       "2026-07-11T00:00:00Z",
		},
		{
			ID:            "rule-2",
			Name:          "API errors",
			Enabled:       true,
			Type:          "log",
			Containers:    []string{"api"},
			Projects:      []string{"demostack"},
			MinLevel:      "ERROR",
			Pattern:       "timeout",
			Threshold:     5,
			WindowSeconds: 60,
			CreatedAt:     "2026-07-11T00:00:00Z",
		},
	}

	channels := []AlertChannel{{ID: "c1", Type: "webhook", Enabled: true, URL: "https://example.com/hook"}}

	m := NewManager()
	err := m.UpdateAlerts(func(current AlertsConfig) (AlertsConfig, error) {
		current.Channels = append(current.Channels, channels...)
		current.Rules = append(current.Rules, rules...)
		return current, nil
	})
	if err != nil {
		t.Fatalf("UpdateAlerts failed: %v", err)
	}

	// A fresh manager must read the persisted alerts back from disk.
	reloaded := NewManager()
	fc := reloaded.FileConfigSnapshot()
	if fc.Alerts == nil {
		t.Fatal("expected alerts config to survive a reload")
	}
	if !reflect.DeepEqual(fc.Alerts.Channels, channels) {
		t.Fatalf("expected channels to round-trip, got %+v", fc.Alerts.Channels)
	}
	if !reflect.DeepEqual(fc.Alerts.Rules, rules) {
		t.Fatalf("expected rules to round-trip, got %+v", fc.Alerts.Rules)
	}
}

func TestUpdateAlertsMutateErrorLeavesConfigUntouched(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	m := NewManager()
	if err := m.UpdateAlerts(func(current AlertsConfig) (AlertsConfig, error) {
		current.Channels = append(current.Channels, AlertChannel{ID: "c1", Type: "webhook", Enabled: true, URL: "https://example.com/hook"})
		return current, nil
	}); err != nil {
		t.Fatalf("UpdateAlerts failed: %v", err)
	}

	wantErr := errors.New("nope")
	err := m.UpdateAlerts(func(current AlertsConfig) (AlertsConfig, error) {
		return AlertsConfig{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected mutate error to propagate, got %v", err)
	}

	fc := m.FileConfigSnapshot()
	if fc.Alerts == nil || len(fc.Alerts.Channels) != 1 || fc.Alerts.Channels[0].URL != "https://example.com/hook" {
		t.Fatalf("expected alerts config to be untouched after mutate error, got %+v", fc.Alerts)
	}
}
