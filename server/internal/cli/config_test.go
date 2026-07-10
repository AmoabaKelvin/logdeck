package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPathRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	path, err := configPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != filepath.Join("/custom/config", "logdeck", "config.json") {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if cfg.CurrentContext != "" || len(cfg.Contexts) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}

	want := cliConfig{
		CurrentContext: "prod",
		Contexts: map[string]contextConfig{
			"prod":    {URL: "https://logdeck.example.com", Token: "ldk_secret123456"},
			"staging": {URL: "http://staging:8080"},
		},
	}
	if err := saveConfig(path, want); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	got, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got.CurrentContext != "prod" || len(got.Contexts) != 2 {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if got.Contexts["prod"] != want.Contexts["prod"] || got.Contexts["staging"] != want.Contexts["staging"] {
		t.Errorf("contexts mismatch: %+v", got.Contexts)
	}

	// Tokens live in this file: check restrictive permissions.
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file permissions = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir permissions = %o, want 700", perm)
	}
}

func TestContextSemantics(t *testing.T) {
	var cfg cliConfig

	cfg.setContext("prod", contextConfig{URL: "https://prod", Token: "ldk_prod12345"})
	cfg.setContext("staging", contextConfig{URL: "https://staging"})
	if cfg.CurrentContext != "staging" {
		t.Errorf("setContext should make the context current, got %q", cfg.CurrentContext)
	}

	if err := cfg.useContext("prod"); err != nil {
		t.Fatalf("useContext: %v", err)
	}
	if cfg.CurrentContext != "prod" {
		t.Errorf("useContext did not switch, got %q", cfg.CurrentContext)
	}
	if err := cfg.useContext("nope"); err == nil {
		t.Error("useContext with unknown name should error")
	}

	if err := cfg.clearToken("prod"); err != nil {
		t.Fatalf("clearToken: %v", err)
	}
	if cfg.Contexts["prod"].Token != "" || cfg.Contexts["prod"].URL != "https://prod" {
		t.Errorf("clearToken should drop the token and keep the URL, got %+v", cfg.Contexts["prod"])
	}
	if err := cfg.clearToken("nope"); err == nil {
		t.Error("clearToken with unknown name should error")
	}

	if err := cfg.removeContext("prod"); err != nil {
		t.Fatalf("removeContext: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("removing the current context should clear currentContext, got %q", cfg.CurrentContext)
	}
	if _, ok := cfg.Contexts["prod"]; ok {
		t.Error("context not removed")
	}
	if err := cfg.removeContext("prod"); err == nil {
		t.Error("removeContext twice should error")
	}
	// Removing a non-current context keeps currentContext.
	cfg.setContext("other", contextConfig{URL: "https://other"})
	cfg.setContext("keep", contextConfig{URL: "https://keep"})
	if err := cfg.removeContext("other"); err != nil {
		t.Fatalf("removeContext: %v", err)
	}
	if cfg.CurrentContext != "keep" {
		t.Errorf("currentContext should survive removing another context, got %q", cfg.CurrentContext)
	}
}

func TestResolveConnectionPrecedence(t *testing.T) {
	cfg := cliConfig{
		CurrentContext: "prod",
		Contexts: map[string]contextConfig{
			"prod":    {URL: "https://ctx", Token: "ctx-token"},
			"staging": {URL: "https://staging", Token: "staging-token"},
		},
	}

	// Default: nothing set, no config.
	conn, err := resolveConnection("", false, "", false, "", "", cliConfig{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.url != defaultServerURL || conn.urlSource != "default" || conn.token != "" || conn.tokenSource != "none" {
		t.Errorf("default resolution wrong: %+v", conn)
	}

	// Context supplies both.
	conn, _ = resolveConnection("", false, "", false, "", "", cfg, "")
	if conn.url != "https://ctx" || conn.urlSource != `context "prod"` || conn.token != "ctx-token" {
		t.Errorf("context resolution wrong: %+v", conn)
	}

	// Env beats context.
	conn, _ = resolveConnection("", false, "", false, "https://env", "env-token", cfg, "")
	if conn.url != "https://env" || conn.urlSource != "env" || conn.token != "env-token" || conn.tokenSource != "env" {
		t.Errorf("env should beat context: %+v", conn)
	}

	// Flags beat env and context.
	conn, _ = resolveConnection("https://flag", true, "flag-token", true, "https://env", "env-token", cfg, "")
	if conn.url != "https://flag" || conn.urlSource != "flag" || conn.token != "flag-token" || conn.tokenSource != "flag" {
		t.Errorf("flags should beat everything: %+v", conn)
	}

	// URL and token resolve independently (flag url + context token).
	conn, _ = resolveConnection("https://flag", true, "", false, "", "", cfg, "")
	if conn.urlSource != "flag" || conn.token != "ctx-token" || conn.tokenSource != `context "prod"` {
		t.Errorf("mixed sources wrong: %+v", conn)
	}

	// --context override selects a non-current context.
	conn, _ = resolveConnection("", false, "", false, "", "", cfg, "staging")
	if conn.url != "https://staging" || conn.token != "staging-token" || conn.urlSource != `context "staging"` {
		t.Errorf("context override wrong: %+v", conn)
	}

	// Unknown --context errors.
	if _, err := resolveConnection("", false, "", false, "", "", cfg, "nope"); err == nil {
		t.Error("unknown context override should error")
	}

	// Dangling currentContext falls back to defaults.
	conn, err = resolveConnection("", false, "", false, "", "", cliConfig{CurrentContext: "gone"}, "")
	if err != nil || conn.url != defaultServerURL {
		t.Errorf("dangling currentContext should fall back to default: %+v, err %v", conn, err)
	}
}

func TestTokenPreview(t *testing.T) {
	if got := tokenPreview(""); got != "none" {
		t.Errorf("empty token preview = %q", got)
	}
	if got := tokenPreview("short"); got != "****" {
		t.Errorf("short token preview = %q", got)
	}
	got := tokenPreview("ldk_abcdef123456789")
	if got != "ldk_abcd..." {
		t.Errorf("token preview = %q, want ldk_abcd...", got)
	}
}
