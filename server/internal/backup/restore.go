package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/audit"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
)

const stampLayout = "20060102T150405Z"

// paths locates the restorable state, which all lives next to the config file.
type paths struct {
	dataDir, config, history, audit, logs string
}

func pathsFor(configPath string) paths {
	return paths{
		dataDir: filepath.Dir(configPath),
		config:  configPath,
		history: alerts.HistoryPath(configPath),
		audit:   audit.Path(configPath),
		logs:    logstore.DBPath(configPath),
	}
}

// staging holds a verified upload until it is confirmed; pending holds a
// confirmed restore until the next start applies it.
func (p paths) staging() string { return filepath.Join(p.dataDir, "restore-staging") }
func (p paths) pending() string { return filepath.Join(p.dataDir, "restore-pending") }

// targets lists the live files an archive file replaces, the one it replaces
// last. Replacing the log database also moves its WAL and shared-memory files
// aside: SQLite would otherwise replay the old WAL onto the restored file.
func (p paths) targets(name string) []string {
	switch name {
	case configName:
		return []string{p.config}
	case historyName:
		return []string{p.history}
	case auditName:
		return []string{p.audit}
	case logsName:
		return []string{p.logs + "-wal", p.logs + "-shm", p.logs}
	}
	return nil
}

// ApplyPending finishes a confirmed restore. It runs at startup, before
// anything opens the files it replaces, which is why restoring needs a
// restart rather than swapping state under a running instance.
//
// Each live file is moved into a pre-restore directory before its restored
// copy is renamed into place. A crash part-way is finished by the next start,
// since a file whose restored copy is still pending has not been swapped. A
// failed step undoes every swap, including those made before a crash,
// leaving the instance as it was. It returns the directory holding the
// previous state, or "" when there was nothing to restore.
func ApplyPending(configPath string) (string, error) {
	p := pathsFor(configPath)
	// An upload that was never confirmed is not wanted after a restart.
	if err := os.RemoveAll(p.staging()); err != nil {
		log.Printf("backup: removing an unconfirmed restore failed: %v", err)
	}

	pending := p.pending()
	if _, err := os.Stat(pending); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	prev, err := previousDir(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(prev, 0o700); err != nil {
		return "", err
	}

	err = keepLiveCredentials(p)
	if err == nil {
		err = keepLiveAudit(p)
	}
	for _, name := range restoredNames {
		if err != nil {
			break
		}
		staged := filepath.Join(pending, name)
		if _, statErr := os.Stat(staged); statErr != nil {
			continue // not in this backup, or swapped before a crash
		}
		err = swap(p, staged, name, prev)
	}
	if err != nil {
		rollback(p, pending, prev)
		failed := filepath.Join(p.dataDir, "restore-failed-"+time.Now().UTC().Format(stampLayout))
		_ = os.Rename(pending, failed)
		return "", fmt.Errorf("the previous state was kept and the backup moved to %s: %w", failed, err)
	}
	if err := os.RemoveAll(pending); err != nil {
		log.Printf("backup: removing the applied restore failed: %v", err)
	}
	return prev, nil
}

// restoredNames are the archive files a restore can replace, in the order it
// replaces them.
var restoredNames = []string{configName, historyName, auditName, logsName}

var previousNameRegexp = regexp.MustCompile(`^pre-restore-\d{8}T\d{6}Z$`)

// previousDir names the directory the live files move into, remembered in
// the pending directory so a restore resumed after a crash uses the same one.
// Anything but a well-formed name, such as a marker emptied by a crash, is
// ignored: it must never resolve to the data directory itself.
func previousDir(p paths) (string, error) {
	marker := filepath.Join(p.pending(), "previous")
	if name, err := os.ReadFile(marker); err == nil && previousNameRegexp.Match(name) {
		return filepath.Join(p.dataDir, string(name)), nil
	}
	name := "pre-restore-" + time.Now().UTC().Format(stampLayout)
	if err := os.WriteFile(marker, []byte(name), 0o600); err != nil {
		return "", err
	}
	return filepath.Join(p.dataDir, name), nil
}

// keepLiveCredentials completes a backup made without credentials from the
// live config file. It runs at apply time rather than when the backup was
// checked, so a token revoked or a password changed while the restore waited
// for a restart stays revoked or changed.
func keepLiveCredentials(p paths) error {
	staged := filepath.Join(p.pending(), configName)
	if _, err := os.Stat(staged); err != nil {
		return nil // swapped before a crash, already merged
	}
	var preview Preview
	if err := readJSON(filepath.Join(p.pending(), "preview.json"), &preview); err != nil {
		return err
	}
	if preview.Manifest.IncludesSecrets {
		return nil
	}
	var restored, live config.FileConfig
	if err := readJSON(staged, &restored); err != nil {
		return err
	}
	if err := readJSON(p.config, &live); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	keepCredentials(&restored, live)
	data, err := json.MarshalIndent(restored, "", "  ")
	if err != nil {
		return err
	}
	tmp := staged + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, staged)
}

// keepLiveAudit adds the live audit log to the restored one, so changes made
// since the backup was taken, the restore among them, stay on record. Like
// keepLiveCredentials it runs at apply time, and running it again after a
// crash gives the same file.
func keepLiveAudit(p paths) error {
	staged := filepath.Join(p.pending(), auditName)
	if _, err := os.Stat(staged); err != nil {
		return nil // not in this backup, or swapped before a crash
	}
	return audit.MergeInto(staged, p.audit)
}

func swap(p paths, staged, name, prev string) error {
	targets := p.targets(name)
	for _, t := range targets {
		if err := os.Rename(t, filepath.Join(prev, filepath.Base(t))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return os.Rename(staged, targets[len(targets)-1])
}

// rollback puts every restored copy back into pending and the previous files
// back in place. A name counts as swapped when its staged copy is gone and
// its live file was moved aside, which also catches swaps made before a
// crash. It is best effort: it only runs after a step has already failed.
func rollback(p paths, pending, prev string) {
	for _, name := range restoredNames {
		targets := p.targets(name)
		live := targets[len(targets)-1]
		staged := filepath.Join(pending, name)
		if _, err := os.Stat(staged); errors.Is(err, fs.ErrNotExist) {
			if _, err := os.Stat(filepath.Join(prev, filepath.Base(live))); err == nil {
				_ = os.Rename(live, staged)
			}
		}
		for _, t := range targets {
			if err := os.Rename(filepath.Join(prev, filepath.Base(t)), t); err != nil && !errors.Is(err, fs.ErrNotExist) {
				log.Printf("backup: rolling back %s failed: %v", t, err)
			}
		}
	}
}
