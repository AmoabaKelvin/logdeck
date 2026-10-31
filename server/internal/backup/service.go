package backup

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/audit"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// scheduleTick is how often the scheduler checks whether a backup is due.
const scheduleTick = time.Minute

var (
	ErrBusy          = errors.New("a backup is already running")
	ErrRestoreBusy   = errors.New("another restore is being checked")
	ErrNoStaged      = errors.New("there is no checked backup to restore; check it again")
	ErrNotFound      = errors.New("backup not found")
	backupNameRegexp = regexp.MustCompile(`^logdeck-(manual|scheduled)-(\d{8}T\d{6}Z)\.tar(\.enc)?$`)
)

// Service creates, lists and schedules backups in one directory, and stages
// restores for the next start to apply.
type Service struct {
	dir     string
	paths   paths
	version string
	// passphrase seals scheduled backups when BACKUP_PASSPHRASE is set. It is
	// read from the environment so it is never written into a backup itself.
	passphrase string
	manager    *config.Manager
	history    func() []models.Alert
	auditLog   func() []audit.Entry
	// logs is nil when log persistence is disabled.
	logs *logstore.Store

	mu            sync.Mutex
	ctx           context.Context
	running       bool
	staging       bool
	last          *Result
	lastScheduled time.Time
}

// Result is the outcome of the latest backup run.
type Result struct {
	Name      string    `json:"name,omitempty"`
	At        time.Time `json:"at"`
	SizeBytes int64     `json:"sizeBytes,omitempty"`
	Scheduled bool      `json:"scheduled"`
	Error     string    `json:"error,omitempty"`
}

// Options choose what one backup holds.
type Options struct {
	IncludeLogs    bool
	IncludeSecrets bool
	Passphrase     string
	Scheduled      bool
}

// Dir returns the backup directory: BACKUP_DIR, or backups/ next to the
// config file. Pointing BACKUP_DIR at a mounted network share or synced
// folder is what takes backups off the host.
func Dir(configPath string) string {
	if dir := os.Getenv("BACKUP_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(filepath.Dir(configPath), "backups")
}

// NewService prepares backups for the instance manager configures. history
// and auditLog may be nil (no alert engine or audit log), and logs is nil
// when persistence is off.
func NewService(manager *config.Manager, history func() []models.Alert, auditLog func() []audit.Entry, logs *logstore.Store, version string) *Service {
	s := &Service{
		dir:        Dir(manager.ConfigFilePath()),
		paths:      pathsFor(manager.ConfigFilePath()),
		version:    version,
		passphrase: os.Getenv("BACKUP_PASSPHRASE"),
		manager:    manager,
		history:    history,
		auditLog:   auditLog,
		logs:       logs,
		ctx:        context.Background(),
	}
	// A backup cut short by a crash leaves its partial archive and snapshot.
	_ = os.Remove(s.snapshotPath())
	partials, _ := filepath.Glob(filepath.Join(s.dir, "logdeck-*.tar*.partial"))
	for _, p := range partials {
		_ = os.Remove(p)
	}
	// The schedule resumes from the newest scheduled backup on disk, so a
	// restart does not take an extra one.
	if entries, err := s.List(); err == nil {
		for _, e := range entries {
			if e.Scheduled {
				s.lastScheduled = e.CreatedAt
				break
			}
		}
	}
	return s
}

// Start runs the scheduler until ctx ends. Backups started afterwards stop
// with ctx too, so shutdown never waits on a snapshot.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(scheduleTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runScheduled()
			}
		}
	}()
}

func (s *Service) runScheduled() {
	cfg := s.manager.Backup()
	if cfg.IntervalHours <= 0 {
		return
	}
	s.mu.Lock()
	due := !time.Now().Before(s.nextRunLocked(cfg))
	if due {
		// Counted from the attempt, so a failing backup retries next interval
		// rather than every tick.
		s.lastScheduled = time.Now()
	}
	s.mu.Unlock()
	if !due {
		return
	}
	err := s.StartBackup(Options{
		IncludeLogs:    cfg.IncludeLogs,
		IncludeSecrets: cfg.IncludeSecrets,
		Passphrase:     s.passphrase,
		Scheduled:      true,
	})
	if err != nil {
		log.Printf("backup: scheduled backup skipped: %v", err)
	}
}

func (s *Service) nextRunLocked(cfg config.BackupConfig) time.Time {
	return s.lastScheduled.Add(time.Duration(cfg.IntervalHours) * time.Hour)
}

// StartBackup begins a backup in the background; Status reports how it went.
func (s *Service) StartBackup(opts Options) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.running = true
	ctx := s.ctx
	s.mu.Unlock()

	go func() {
		res := Result{At: time.Now().UTC(), Scheduled: opts.Scheduled}
		name, size, err := s.create(ctx, opts)
		if err != nil {
			res.Error = err.Error()
			log.Printf("backup: failed: %v", err)
		} else {
			res.Name, res.SizeBytes = name, size
			log.Printf("backup: wrote %s (%d bytes)", name, size)
			if opts.Scheduled {
				s.prune(s.manager.Backup().Keep)
			}
		}
		s.mu.Lock()
		s.running = false
		s.last = &res
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) snapshotPath() string {
	return filepath.Join(s.paths.dataDir, "backup-logs.db.tmp")
}

// create writes one backup into the backup directory under a temporary name
// and renames it into place once it is complete and synced.
func (s *Service) create(ctx context.Context, opts Options) (string, int64, error) {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return "", 0, fmt.Errorf("create backup directory: %w", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	kind := "manual"
	if opts.Scheduled {
		kind = "scheduled"
	}
	name := fmt.Sprintf("logdeck-%s-%s.tar", kind, now.Format(stampLayout))
	if opts.Passphrase != "" {
		name += ".enc"
	}

	st := state{createdAt: now, version: s.version, config: s.manager.FileConfigSnapshot()}
	if s.history != nil {
		st.history = s.history()
	}
	if s.auditLog != nil {
		st.audit = s.auditLog()
	}
	if opts.IncludeLogs && s.logs != nil {
		// The snapshot goes next to the live database rather than into the
		// backup directory: SQLite on a network share is slow and unsafe.
		st.logsPath = s.snapshotPath()
		_ = os.Remove(st.logsPath)
		defer os.Remove(st.logsPath)
		if err := s.logs.Snapshot(ctx, st.logsPath); err != nil {
			return "", 0, err
		}
	}

	final := filepath.Join(s.dir, name)
	if _, err := os.Stat(final); err == nil {
		return "", 0, errors.New("a backup with this name was just made; try again in a second")
	}
	tmp := final + ".partial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("write backup: %w", err)
	}
	bw := bufio.NewWriterSize(f, 256<<10)
	_, err = writeArchive(bw, st, opts.IncludeSecrets, opts.Passphrase)
	if err == nil {
		err = bw.Flush()
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, final)
	}
	if err != nil {
		os.Remove(tmp)
		return "", 0, fmt.Errorf("write backup: %w", err)
	}
	info, err := os.Stat(final)
	if err != nil {
		return name, 0, nil
	}
	return name, info.Size(), nil
}

// Entry is one backup file in the backup directory.
type Entry struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
	Scheduled bool      `json:"scheduled"`
	Encrypted bool      `json:"encrypted"`
	// Manifest is read from unencrypted backups only: an encrypted one shows
	// what it holds only with its passphrase.
	Manifest *Manifest `json:"manifest,omitempty"`
}

// List returns the backups in the backup directory, newest first. A missing
// directory is an empty list.
func (s *Service) List() ([]Entry, error) {
	dirEntries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	for _, de := range dirEntries {
		match := backupNameRegexp.FindStringSubmatch(de.Name())
		if match == nil || !de.Type().IsRegular() {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		created, _ := time.Parse(stampLayout, match[2])
		e := Entry{
			Name:      de.Name(),
			SizeBytes: info.Size(),
			CreatedAt: created,
			Scheduled: match[1] == "scheduled",
			Encrypted: match[3] != "",
		}
		if !e.Encrypted {
			e.Manifest = readManifest(filepath.Join(s.dir, de.Name()))
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b Entry) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return entries, nil
}

// readManifest reads only the first archive entry, so listing large backups
// stays cheap. It returns nil for anything that is not a readable manifest.
func readManifest(path string) *Manifest {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	tr := tar.NewReader(f)
	hdr, err := tr.Next()
	if err != nil || hdr.Name != manifestName || hdr.Size > maxManifestSize {
		return nil
	}
	var m Manifest
	if err := json.NewDecoder(tr).Decode(&m); err != nil {
		return nil
	}
	return &m
}

// prune deletes scheduled backups beyond the newest keep. Manual backups are
// left for the user to delete.
func (s *Service) prune(keep int) {
	entries, err := s.List()
	if err != nil {
		log.Printf("backup: listing backups for retention failed: %v", err)
		return
	}
	kept := 0
	for _, e := range entries {
		if !e.Scheduled {
			continue
		}
		if kept++; kept > keep {
			if err := os.Remove(filepath.Join(s.dir, e.Name)); err != nil {
				log.Printf("backup: removing old backup %s failed: %v", e.Name, err)
			}
		}
	}
}

// Path returns the file of a listed backup. Only names the backup directory
// could hold are accepted, so a name can never reach outside it.
func (s *Service) Path(name string) (string, error) {
	if !backupNameRegexp.MatchString(name) {
		return "", ErrNotFound
	}
	path := filepath.Join(s.dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

func (s *Service) Delete(name string) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// Status is everything the settings page shows about backups.
type Status struct {
	Dir               string              `json:"dir"`
	Running           bool                `json:"running"`
	Last              *Result             `json:"last,omitempty"`
	Schedule          config.BackupConfig `json:"schedule"`
	NextRunAt         *time.Time          `json:"nextRunAt,omitempty"`
	ScheduleEncrypted bool                `json:"scheduleEncrypted"`
	LogsAvailable     bool                `json:"logsAvailable"`
	Backups           []Entry             `json:"backups"`
	ListError         string              `json:"listError,omitempty"`
	PendingRestore    *Preview            `json:"pendingRestore,omitempty"`
}

func (s *Service) Status() Status {
	cfg := s.manager.Backup()
	st := Status{
		Dir:               s.dir,
		Schedule:          cfg,
		ScheduleEncrypted: s.passphrase != "",
		LogsAvailable:     s.logs != nil,
		Backups:           []Entry{},
	}
	s.mu.Lock()
	st.Running = s.running
	st.Last = s.last
	if cfg.IntervalHours > 0 {
		next := s.nextRunLocked(cfg)
		st.NextRunAt = &next
	}
	s.mu.Unlock()

	if entries, err := s.List(); err != nil {
		st.ListError = err.Error()
	} else {
		st.Backups = entries
	}
	var pending Preview
	if err := readJSON(filepath.Join(s.paths.pending(), "preview.json"), &pending); err == nil {
		st.PendingRestore = &pending
	}
	return st
}

// Preview is what a checked backup would replace, shown before confirming.
type Preview struct {
	Manifest  Manifest `json:"manifest"`
	Encrypted bool     `json:"encrypted"`
	// Current counts what this instance holds now, for comparison.
	Current Contents `json:"current"`
	// MissingCredentials counts platform hosts and alert channels that come
	// back without a secret because the backup was made without credentials.
	MissingCredentials int      `json:"missingCredentials"`
	Warnings           []string `json:"warnings"`
}

// StageRestore reads a backup from r into the staging directory and verifies
// it. Nothing live is touched: ConfirmRestore marks it for the next start.
func (s *Service) StageRestore(r io.Reader, passphrase string) (*Preview, error) {
	s.mu.Lock()
	if s.staging {
		s.mu.Unlock()
		return nil, ErrRestoreBusy
	}
	s.staging = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.staging = false
		s.mu.Unlock()
	}()

	dir := s.paths.staging()
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	preview, err := s.stage(r, passphrase, dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return preview, nil
}

func (s *Service) stage(r io.Reader, passphrase, dir string) (*Preview, error) {
	m, encrypted, err := stage(r, passphrase, dir)
	if err != nil {
		return nil, err
	}

	current := s.manager.FileConfigSnapshot()
	var historyCount, auditCount int
	if s.history != nil {
		historyCount = len(s.history())
	}
	if s.auditLog != nil {
		auditCount = len(s.auditLog())
	}
	var logsBytes int64
	if s.logs != nil {
		logsBytes, _ = s.logs.DBSize()
	}
	p := &Preview{
		Manifest:  m,
		Encrypted: encrypted,
		Current:   countContents(current, historyCount, auditCount, logsBytes),
		Warnings:  []string{},
	}

	if !m.IncludesSecrets {
		// Only counted here: the merge itself happens when the restore is
		// applied, against the credentials the instance has by then.
		var restored config.FileConfig
		if err := readJSON(filepath.Join(dir, configName), &restored); err != nil {
			return nil, err
		}
		p.MissingCredentials = keepCredentials(&restored, current)
		p.Warnings = append(p.Warnings, "This backup was made without credentials, so sign-in and API tokens stay as they are on this instance.")
		if p.MissingCredentials > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%d platform connections or alert channels have no matching credential here and need it entered again; those channels come back disabled.", p.MissingCredentials))
		}
	}
	if !m.IncludesLogs {
		p.Warnings = append(p.Warnings, "This backup has no stored logs, so the logs on this instance are kept.")
	}
	if m.LogDeckVersion != s.version {
		p.Warnings = append(p.Warnings, fmt.Sprintf("This backup was made by LogDeck %s; this instance runs %s. Settings this version does not know are dropped.", m.LogDeckVersion, s.version))
	}

	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	// Written last: its presence is what marks the staging directory complete.
	if err := os.WriteFile(filepath.Join(dir, "preview.json"), data, 0o600); err != nil {
		return nil, err
	}
	return p, nil
}

// StageStored stages a backup from the backup directory.
func (s *Service) StageStored(name, passphrase string) (*Preview, error) {
	path, err := s.Path(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.StageRestore(f, passphrase)
}

// ConfirmRestore hands the checked backup to the next start. Moving the
// staging directory into place is a single rename, so a restore is either
// pending in full or not at all.
func (s *Service) ConfirmRestore() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staging {
		return ErrRestoreBusy
	}
	staging := s.paths.staging()
	if _, err := os.Stat(filepath.Join(staging, "preview.json")); err != nil {
		return ErrNoStaged
	}
	if err := os.RemoveAll(s.paths.pending()); err != nil {
		return err
	}
	return os.Rename(staging, s.paths.pending())
}

// CancelRestore drops a checked or pending restore.
func (s *Service) CancelRestore() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staging {
		return ErrRestoreBusy
	}
	return errors.Join(os.RemoveAll(s.paths.staging()), os.RemoveAll(s.paths.pending()))
}
