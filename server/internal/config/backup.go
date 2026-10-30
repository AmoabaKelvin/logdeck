package config

// BackupConfig holds the scheduled backup settings persisted in the config
// file. An IntervalHours of zero turns scheduled backups off.
type BackupConfig struct {
	IntervalHours  int  `json:"intervalHours"`
	Keep           int  `json:"keep"`
	IncludeLogs    bool `json:"includeLogs"`
	IncludeSecrets bool `json:"includeSecrets"`
}

// DefaultBackupKeep is how many scheduled backups are kept when none is set.
const DefaultBackupKeep = 7

// Backup returns the scheduled backup settings, with defaults applied.
func (m *Manager) Backup() BackupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := BackupConfig{Keep: DefaultBackupKeep}
	if m.fileConfig.Backup != nil {
		cfg = *m.fileConfig.Backup
		if cfg.Keep <= 0 {
			cfg.Keep = DefaultBackupKeep
		}
	}
	return cfg
}

// UpdateBackup replaces the scheduled backup settings. The scheduler reads
// them through the manager on every tick, so no remerge or callback is needed.
func (m *Manager) UpdateBackup(cfg BackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.fileConfig.Backup
	m.fileConfig.Backup = &cfg
	if err := m.persist(); err != nil {
		m.fileConfig.Backup = old
		return err
	}
	return nil
}
