package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

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

	m := NewManager()
	err := m.UpdateAlerts(func(current AlertsConfig) (AlertsConfig, error) {
		current.WebhookURL = "https://example.com/hook"
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
	if fc.Alerts.WebhookURL != "https://example.com/hook" {
		t.Fatalf("expected webhook URL to round-trip, got %q", fc.Alerts.WebhookURL)
	}
	if !reflect.DeepEqual(fc.Alerts.Rules, rules) {
		t.Fatalf("expected rules to round-trip, got %+v", fc.Alerts.Rules)
	}
}

func TestUpdateAlertsMutateErrorLeavesConfigUntouched(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	m := NewManager()
	if err := m.UpdateAlerts(func(current AlertsConfig) (AlertsConfig, error) {
		current.WebhookURL = "https://example.com/hook"
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
	if fc.Alerts == nil || fc.Alerts.WebhookURL != "https://example.com/hook" {
		t.Fatalf("expected alerts config to be untouched after mutate error, got %+v", fc.Alerts)
	}
}
