package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newManagerWithFile writes fc to a temp config file, points CONFIG_PATH at it,
// and returns a fresh Manager that loaded it.
func newManagerWithFile(t *testing.T, fc FileConfig) *Manager {
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

func TestMergeDefaultsWhenNothingConfigured(t *testing.T) {
	m := newManagerWithFile(t, FileConfig{})

	cfg := m.Config()
	if len(cfg.DockerHosts) != 1 || cfg.DockerHosts[0].Name != "local" {
		t.Fatalf("expected a single default local host, got %+v", cfg.DockerHosts)
	}
	sources := m.Sources()
	if sources.DockerHosts != SourceDefault {
		t.Fatalf("DockerHosts source = %q, want %q", sources.DockerHosts, SourceDefault)
	}
	if sources.CoolifyHosts != SourceDefault {
		t.Fatalf("CoolifyHosts source = %q, want %q", sources.CoolifyHosts, SourceDefault)
	}
	if sources.ReadOnly != SourceDefault {
		t.Fatalf("ReadOnly source = %q, want %q", sources.ReadOnly, SourceDefault)
	}
	if cfg.ReadOnly {
		t.Fatal("ReadOnly should default to false")
	}
}

func TestMergeFileOnly(t *testing.T) {
	m := newManagerWithFile(t, FileConfig{
		DockerHosts: []DockerHost{{Name: "file-host", Host: "tcp://filehost:2375"}},
	})

	cfg := m.Config()
	if len(cfg.DockerHosts) != 1 || cfg.DockerHosts[0].Name != "file-host" {
		t.Fatalf("expected the file host, got %+v", cfg.DockerHosts)
	}
	if got := m.Sources().DockerHosts; got != SourceFile {
		t.Fatalf("DockerHosts source = %q, want %q", got, SourceFile)
	}
}

func TestMergeEnvOnly(t *testing.T) {
	t.Setenv("DOCKER_HOSTS", "env-host=tcp://envhost:2375")
	m := newManagerWithFile(t, FileConfig{})

	cfg := m.Config()
	if len(cfg.DockerHosts) != 1 || cfg.DockerHosts[0].Name != "env-host" {
		t.Fatalf("expected the env host, got %+v", cfg.DockerHosts)
	}
	if got := m.Sources().DockerHosts; got != SourceEnv {
		t.Fatalf("DockerHosts source = %q, want %q", got, SourceEnv)
	}
}

// Env and file hosts combine; on a name collision the env host wins and the
// section is reported as mixed.
func TestMergeEnvWinsOnCollision(t *testing.T) {
	t.Setenv("DOCKER_HOSTS", "local=tcp://envhost:2375")
	m := newManagerWithFile(t, FileConfig{
		DockerHosts: []DockerHost{
			{Name: "local", Host: "tcp://filehost:2375"}, // collides with env "local"
			{Name: "extra", Host: "tcp://extra:2375"},
		},
	})

	cfg := m.Config()
	byName := map[string]string{}
	for _, h := range cfg.DockerHosts {
		byName[h.Name] = h.Host
	}
	if len(byName) != 2 {
		t.Fatalf("expected env local + file extra, got %+v", cfg.DockerHosts)
	}
	if byName["local"] != "tcp://envhost:2375" {
		t.Fatalf("expected env host to win the collision, got %q", byName["local"])
	}
	if byName["extra"] != "tcp://extra:2375" {
		t.Fatalf("expected file-only host to survive, got %q", byName["extra"])
	}
	if got := m.Sources().DockerHosts; got != SourceMixed {
		t.Fatalf("DockerHosts source = %q, want %q", got, SourceMixed)
	}
}

func TestUpdateDockerHostsRejectsEnvCollision(t *testing.T) {
	t.Setenv("DOCKER_HOSTS", "local=tcp://envhost:2375")
	m := newManagerWithFile(t, FileConfig{})

	err := m.UpdateDockerHosts([]DockerHost{{Name: "local", Host: "tcp://x:2375"}})
	if err == nil {
		t.Fatal("expected an error updating a host that collides with an env host")
	}

	// The rejected update must not be persisted.
	if got := len(m.FileConfigSnapshot().DockerHosts); got != 0 {
		t.Fatalf("expected no file hosts after a rejected update, got %d", got)
	}
}

func TestUpdateDockerHostsPersistsAndRemerges(t *testing.T) {
	m := newManagerWithFile(t, FileConfig{})

	hosts := []DockerHost{{Name: "new", Host: "tcp://new:2375"}}
	if err := m.UpdateDockerHosts(hosts); err != nil {
		t.Fatalf("UpdateDockerHosts: %v", err)
	}

	// The merged config reflects the update immediately.
	cfg := m.Config()
	if len(cfg.DockerHosts) != 1 || cfg.DockerHosts[0].Name != "new" {
		t.Fatalf("merged config not updated, got %+v", cfg.DockerHosts)
	}
	if got := m.Sources().DockerHosts; got != SourceFile {
		t.Fatalf("DockerHosts source = %q, want %q", got, SourceFile)
	}

	// A fresh manager reads the persisted host back from disk.
	reloaded := NewManager()
	rc := reloaded.Config()
	if len(rc.DockerHosts) != 1 || rc.DockerHosts[0].Host != "tcp://new:2375" {
		t.Fatalf("persisted host did not round-trip, got %+v", rc.DockerHosts)
	}
}

func TestUpdateCoolifyHostsRejectsEnvCollision(t *testing.T) {
	t.Setenv("COOLIFY_CONFIGS", "local|https://coolify.example|token")
	m := newManagerWithFile(t, FileConfig{})

	err := m.UpdateCoolifyHosts([]CoolifyHostConfig{{HostName: "local", APIURL: "https://x", APIToken: "t"}})
	if err == nil {
		t.Fatal("expected an error updating a coolify host that collides with an env host")
	}
}

// OnChange callbacks fire with the freshly merged config after an update.
func TestOnChangeFiresAfterUpdate(t *testing.T) {
	m := newManagerWithFile(t, FileConfig{})

	var got *Config
	m.OnChange(func(cfg *Config) { got = cfg })

	if err := m.UpdateReadOnly(true); err != nil {
		t.Fatalf("UpdateReadOnly: %v", err)
	}
	if got == nil {
		t.Fatal("expected OnChange callback to fire")
	}
	if !got.ReadOnly {
		t.Fatal("expected callback to receive the updated (read-only) config")
	}
}

func TestUpdateReadOnlyRejectedWhenEnvControlled(t *testing.T) {
	t.Setenv("READONLY_MODE", "true")
	m := newManagerWithFile(t, FileConfig{})

	if err := m.UpdateReadOnly(false); err == nil {
		t.Fatal("expected an error changing read-only mode that is pinned by env")
	}
	if !m.Config().ReadOnly {
		t.Fatal("env-pinned read-only mode must remain true")
	}
}
