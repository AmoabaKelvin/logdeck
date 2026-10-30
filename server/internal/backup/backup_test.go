package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// newInstance points a config manager at a fresh data directory.
func newInstance(t *testing.T) (*config.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_PATH", configPath)
	for _, key := range []string{"BACKUP_DIR", "BACKUP_PASSPHRASE", "JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD", "DOCKER_HOSTS", "COOLIFY_CONFIGS", "DOKPLOY_CONFIGS"} {
		t.Setenv(key, "")
	}
	return config.NewManager(), configPath
}

// populate gives an instance every kind of state a backup carries.
func populate(t *testing.T, m *config.Manager) {
	t.Helper()
	if err := m.UpdateDockerHosts([]config.DockerHost{{Name: "remote", Host: "ssh://root@10.0.0.2"}}, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateCoolifyHosts([]config.CoolifyHostConfig{{HostName: "remote", APIURL: "https://coolify.example", APIToken: "coolify-secret"}}, ""); err != nil {
		t.Fatal(err)
	}
	err := m.UpdateAlerts(func(config.AlertsConfig) (config.AlertsConfig, error) {
		return config.AlertsConfig{
			Channels: []config.AlertChannel{{ID: "ch1", Type: "webhook", Enabled: true, URL: "https://hooks.example/secret"}},
			Rules:    []config.AlertRule{{ID: "r1", Name: "Crashes", Enabled: true, Type: "event", Events: []string{"die"}, Threshold: 1}},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = m.UpdateAPITokens(func([]config.APIToken) ([]config.APIToken, error) {
		return []config.APIToken{{Name: "ci", Hash: "abc", Prefix: "ld_abc", Scope: "read"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = m.UpdateAuth(func(*config.FileAuthConfig) (*config.FileAuthConfig, error) {
		return &config.FileAuthConfig{Enabled: true, JWTSecret: "jwt", AdminUsername: "admin", AdminPasswordHash: "hash", AdminPasswordSalt: "salt"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T, configPath string, m *config.Manager) *logstore.Store {
	t.Helper()
	store, err := logstore.Open(logstore.DBPath(configPath), m.LogStore)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func storedContainerNames(t *testing.T, dbPath string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name FROM containers ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func takeBackup(t *testing.T, s *Service, opts Options) string {
	t.Helper()
	name, _, err := s.create(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(s.dir, name)
}

func TestBackupRestoresIntoAFreshInstance(t *testing.T) {
	source, sourceConfig := newInstance(t)
	populate(t, source)
	store := openStore(t, sourceConfig, source)
	// Write a stored container through a second connection, as WAL allows
	// while the store's own pool is open.
	db, err := sql.Open("sqlite", logstore.DBPath(sourceConfig))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO containers (host, container_id, name, first_seen_ms, last_seen_ms) VALUES ('local', 'c1', 'web', 1, 2)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	history := []models.Alert{{ID: "a1", RuleName: "Crashes", ContainerName: "web"}, {ID: "a2", RuleName: "Crashes"}}
	src := NewService(source, func() []models.Alert { return history }, store, "1.2.0")
	archive := takeBackup(t, src, Options{IncludeLogs: true, IncludeSecrets: true, Passphrase: "correct horse battery"})

	target, targetConfig := newInstance(t)
	dst := NewService(target, nil, nil, "1.2.0")
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := dst.StageRestore(f, "correct horse battery")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	want := Contents{DockerHosts: 1, CoolifyHosts: 1, AlertRules: 1, AlertChannels: 1, AlertHistory: 2, APITokens: 1, LogsBytes: preview.Manifest.Contents.LogsBytes}
	if preview.Manifest.Contents != want || !preview.Encrypted || preview.Manifest.Contents.LogsBytes == 0 {
		t.Fatalf("preview = %+v", preview)
	}
	// Nothing live changes before the confirmed restore is applied at start.
	if _, err := os.Stat(targetConfig); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging wrote the live config: %v", err)
	}
	if err := dst.ConfirmRestore(); err != nil {
		t.Fatal(err)
	}
	if dst.Status().PendingRestore == nil {
		t.Fatal("confirmed restore is not reported as pending")
	}
	prev, err := ApplyPending(targetConfig)
	if err != nil || prev == "" {
		t.Fatalf("ApplyPending = %q, %v", prev, err)
	}

	restored := config.NewManager()
	gotJSON, _ := json.Marshal(restored.FileConfigSnapshot())
	wantJSON, _ := json.Marshal(source.FileConfigSnapshot())
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("restored config differs:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	var gotHistory []models.Alert
	if err := readJSON(alerts.HistoryPath(targetConfig), &gotHistory); err != nil || len(gotHistory) != 2 || gotHistory[0].ID != "a1" {
		t.Fatalf("restored history = %+v, %v", gotHistory, err)
	}
	if names := storedContainerNames(t, logstore.DBPath(targetConfig)); len(names) != 1 || names[0] != "web" {
		t.Fatalf("restored containers = %v", names)
	}
	if _, err := os.Stat(pathsFor(targetConfig).pending()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("pending restore left behind after applying")
	}
}

func TestApplyPendingMovesTheOldDatabaseAndItsWALAside(t *testing.T) {
	source, sourceConfig := newInstance(t)
	src := NewService(source, nil, openStore(t, sourceConfig, source), "1.2.0")
	archive := takeBackup(t, src, Options{IncludeLogs: true})

	target, targetConfig := newInstance(t)
	logs := logstore.DBPath(targetConfig)
	for _, p := range []string{logs, logs + "-wal", logs + "-shm"} {
		if err := os.WriteFile(p, []byte("old "+filepath.Base(p)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dst := NewService(target, nil, nil, "1.2.0")
	f, _ := os.Open(archive)
	_, err := dst.StageRestore(f, "")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.ConfirmRestore(); err != nil {
		t.Fatal(err)
	}
	prev, err := ApplyPending(targetConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"logs.db-wal", "logs.db-shm"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(logs), name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still next to the restored database", name)
		}
		if data, err := os.ReadFile(filepath.Join(prev, name)); err != nil || string(data) != "old "+name {
			t.Errorf("%s was not kept in %s: %q, %v", name, prev, data, err)
		}
	}
	if err := logstore.CheckSnapshot(logs); err != nil {
		t.Fatalf("restored database: %v", err)
	}
}

func TestFailedApplyRollsBackSwapsMadeBeforeACrash(t *testing.T) {
	source, sourceConfig := newInstance(t)
	populate(t, source)
	src := NewService(source, nil, openStore(t, sourceConfig, source), "1.2.0")
	archive := takeBackup(t, src, Options{IncludeLogs: true, IncludeSecrets: true})

	target, targetConfig := newInstance(t)
	if err := target.UpdateReadOnly(true); err != nil {
		t.Fatal(err)
	}
	oldConfig, _ := os.ReadFile(targetConfig)
	logs := logstore.DBPath(targetConfig)
	if err := os.WriteFile(logs, []byte("old logs"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := NewService(target, nil, nil, "1.2.0")
	f, _ := os.Open(archive)
	_, err := dst.StageRestore(f, "")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.ConfirmRestore(); err != nil {
		t.Fatal(err)
	}

	// A first start swapped the config and then crashed.
	p := pathsFor(targetConfig)
	prev, err := previousDir(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(prev, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := swap(p, filepath.Join(p.pending(), configName), configName, prev); err != nil {
		t.Fatal(err)
	}
	// On the next start the log database can't be moved aside.
	if err := os.MkdirAll(filepath.Join(prev, "logs.db", "blocker"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := ApplyPending(targetConfig); err == nil {
		t.Fatal("ApplyPending succeeded with a blocked swap")
	}
	if got, _ := os.ReadFile(targetConfig); !bytes.Equal(got, oldConfig) {
		t.Fatalf("config was not rolled back:\n%s", got)
	}
	if got, _ := os.ReadFile(logs); string(got) != "old logs" {
		t.Fatalf("logs.db = %q, want the old file", got)
	}
	failed, _ := filepath.Glob(filepath.Join(p.dataDir, "restore-failed-*", configName))
	if len(failed) != 1 {
		t.Fatalf("the backup was not kept for another try: %v", failed)
	}
}

func TestRestoreRejectsDamagedAndUnsupportedBackups(t *testing.T) {
	source, _ := newInstance(t)
	populate(t, source)
	src := NewService(source, nil, nil, "1.2.0")
	plain, err := os.ReadFile(takeBackup(t, src, Options{IncludeSecrets: true}))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(takeBackup(t, src, Options{IncludeSecrets: true, Passphrase: "correct horse battery"}))
	if err != nil {
		t.Fatal(err)
	}

	flipped := bytes.Clone(plain)
	i := bytes.Index(flipped, []byte("ssh://root@10.0.0.2"))
	flipped[i] = 'S'

	newer := bytes.Replace(plain, []byte(`"format": 1`), []byte(`"format": 9`), 1)

	cases := map[string]struct {
		data       []byte
		passphrase string
		want       string
	}{
		"not a backup":             {[]byte("hello"), "", "not a LogDeck backup"},
		"flipped byte":             {flipped, "", "does not match its checksum"},
		"truncated":                {plain[:len(plain)/2], "", "incomplete or corrupted"},
		"unsupported format":       {newer, "", "format 9 is not supported"},
		"encrypted, no passphrase": {sealed, "", "encrypted"},
		"wrong passphrase":         {sealed, "wrong passphrase!", "passphrase is wrong"},
		"encrypted, truncated":     {sealed[:len(sealed)-100], "correct horse battery", "passphrase is wrong or the backup is corrupted"},
	}
	for name, tc := range cases {
		target, targetConfig := newInstance(t)
		dst := NewService(target, nil, nil, "1.2.0")
		_, err := dst.StageRestore(bytes.NewReader(tc.data), tc.passphrase)
		var invalid *InvalidError
		if !errors.As(err, &invalid) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want an invalid-backup error containing %q", name, err, tc.want)
		}
		if _, err := os.Stat(pathsFor(targetConfig).staging()); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: rejected backup left a staging directory", name)
		}
		if err := dst.ConfirmRestore(); !errors.Is(err, ErrNoStaged) {
			t.Errorf("%s: a rejected backup could still be confirmed: %v", name, err)
		}
	}
}

func TestBackupWithoutCredentialsKeepsTheTargetsAccess(t *testing.T) {
	source, _ := newInstance(t)
	populate(t, source)
	src := NewService(source, nil, nil, "1.2.0")
	archive := takeBackup(t, src, Options{})
	data, _ := os.ReadFile(archive)
	for _, secret := range []string{"coolify-secret", "hooks.example/secret", "jwt", `"hash"`} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("backup without credentials contains %s", secret)
		}
	}

	target, targetConfig := newInstance(t)
	err := target.UpdateAuth(func(*config.FileAuthConfig) (*config.FileAuthConfig, error) {
		return &config.FileAuthConfig{Enabled: true, JWTSecret: "target-jwt", AdminUsername: "root", AdminPasswordHash: "h", AdminPasswordSalt: "s"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = target.UpdateAlerts(func(config.AlertsConfig) (config.AlertsConfig, error) {
		return config.AlertsConfig{Channels: []config.AlertChannel{{ID: "ch1", Type: "webhook", Enabled: true, URL: "https://hooks.example/target"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	dst := NewService(target, nil, nil, "1.2.0")
	f, _ := os.Open(archive)
	preview, err := dst.StageRestore(f, "")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	// The Coolify host has no counterpart here; the channel takes the
	// target's URL by ID.
	if preview.MissingCredentials != 1 {
		t.Fatalf("MissingCredentials = %d, want 1", preview.MissingCredentials)
	}
	if err := dst.ConfirmRestore(); err != nil {
		t.Fatal(err)
	}
	// A password changed while the restore waits for a restart must hold.
	err = target.UpdateAuth(func(*config.FileAuthConfig) (*config.FileAuthConfig, error) {
		return &config.FileAuthConfig{Enabled: true, JWTSecret: "rotated-jwt", AdminUsername: "root", AdminPasswordHash: "h2", AdminPasswordSalt: "s2"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPending(targetConfig); err != nil {
		t.Fatal(err)
	}
	fc := config.NewManager().FileConfigSnapshot()
	if fc.Auth == nil || fc.Auth.AdminUsername != "root" || fc.Auth.JWTSecret != "rotated-jwt" || len(fc.APITokens) != 0 {
		t.Fatalf("sign-in changed: auth=%+v tokens=%+v", fc.Auth, fc.APITokens)
	}
	if ch := fc.Alerts.Channels[0]; ch.URL != "https://hooks.example/target" || !ch.Enabled {
		t.Fatalf("channel = %+v", ch)
	}
	if len(fc.Alerts.Rules) != 1 || len(fc.DockerHosts) != 1 {
		t.Fatalf("restored config lost rules or hosts: %+v", fc)
	}
}

func TestScheduledRetentionLeavesManualBackups(t *testing.T) {
	m, _ := newInstance(t)
	s := NewService(m, nil, nil, "1.2.0")
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		t.Fatal(err)
	}
	names := []string{
		"logdeck-scheduled-20260101T000000Z.tar",
		"logdeck-scheduled-20260102T000000Z.tar.enc",
		"logdeck-scheduled-20260103T000000Z.tar",
		"logdeck-manual-20250101T000000Z.tar",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(s.dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.prune(2)
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name)
	}
	want := []string{names[2], names[1], names[3]}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("after pruning: %v, want %v", got, want)
	}
}
