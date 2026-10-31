// Package backup writes and restores LogDeck backups. A backup is one tar
// archive: a manifest first, then the config file, the alert history, the
// audit log and, optionally, a snapshot of the stored logs. It can be sealed
// with a passphrase; see crypt.go.
package backup

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/audit"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// FormatVersion is the archive layout this build writes and reads.
const FormatVersion = 1

const (
	manifestName = "manifest.json"
	configName   = "config.json"
	historyName  = "alerts-history.json"
	auditName    = "audit.jsonl"
	logsName     = "logs.db"

	maxManifestSize = 1 << 20
	// maxSmallFile caps the config file, alert history and audit log, which
	// restore parses in memory.
	maxSmallFile = 64 << 20
)

var (
	ErrNotBackup          = errors.New("this file is not a LogDeck backup")
	ErrPassphraseRequired = errors.New("this backup is encrypted; enter its passphrase")
)

// Manifest describes a backup. It is the archive's first entry, so the
// format and version are checked before anything else is read.
type Manifest struct {
	Format          int       `json:"format"`
	LogDeckVersion  string    `json:"logdeckVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	IncludesLogs    bool      `json:"includesLogs"`
	IncludesSecrets bool      `json:"includesSecrets"`
	Contents        Contents  `json:"contents"`
	Files           []File    `json:"files"`
}

type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Contents counts what a backup, or the running instance, holds.
type Contents struct {
	DockerHosts   int   `json:"dockerHosts"`
	CoolifyHosts  int   `json:"coolifyHosts"`
	DokployHosts  int   `json:"dokployHosts"`
	AlertRules    int   `json:"alertRules"`
	AlertChannels int   `json:"alertChannels"`
	AlertHistory  int   `json:"alertHistory"`
	AuditEntries  int   `json:"auditEntries"`
	APITokens     int   `json:"apiTokens"`
	LogsBytes     int64 `json:"logsBytes"`
}

func countContents(fc config.FileConfig, history, auditEntries int, logsBytes int64) Contents {
	c := Contents{
		DockerHosts:  len(fc.DockerHosts),
		CoolifyHosts: len(fc.CoolifyHosts),
		DokployHosts: len(fc.DokployHosts),
		AlertHistory: history,
		AuditEntries: auditEntries,
		APITokens:    len(fc.APITokens),
		LogsBytes:    logsBytes,
	}
	if fc.Alerts != nil {
		c.AlertRules = len(fc.Alerts.Rules)
		c.AlertChannels = len(fc.Alerts.Channels)
	}
	return c
}

// state is what one backup captures.
type state struct {
	createdAt time.Time
	version   string
	config    config.FileConfig
	history   []models.Alert
	audit     []audit.Entry
	// logsPath is a finished log store snapshot, or "" for none.
	logsPath string
}

// writeArchive writes a backup of st to w. The log snapshot is streamed from
// disk, never held in memory.
func writeArchive(w io.Writer, st state, includeSecrets bool, passphrase string) (Manifest, error) {
	fc, history, logsPath, createdAt := st.config, st.history, st.logsPath, st.createdAt
	if !includeSecrets {
		fc = redactCredentials(fc)
	}
	if history == nil {
		history = []models.Alert{}
	}
	configJSON, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	historyJSON, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return Manifest{}, err
	}

	m := Manifest{
		Format:          FormatVersion,
		LogDeckVersion:  st.version,
		CreatedAt:       createdAt,
		IncludesLogs:    logsPath != "",
		IncludesSecrets: includeSecrets,
		Files:           []File{fileOf(configName, configJSON), fileOf(historyName, historyJSON)},
	}
	entries := []archiveEntry{{manifestName, nil}, {configName, configJSON}, {historyName, historyJSON}}
	// The audit log is optional so that backups made before it existed still
	// restore; restoring one of them leaves the live audit log as it is.
	if st.audit != nil {
		auditData := audit.Encode(st.audit)
		m.Files = append(m.Files, fileOf(auditName, auditData))
		entries = append(entries, archiveEntry{auditName, auditData})
	}
	var logsSize int64
	if logsPath != "" {
		// Hashing first costs one extra sequential read of a file that was
		// just written, and lets the manifest lead the archive.
		f, err := hashFile(logsPath)
		if err != nil {
			return Manifest{}, err
		}
		logsSize = f.Size
		m.Files = append(m.Files, f)
	}
	m.Contents = countContents(fc, len(history), len(st.audit), logsSize)
	manifestJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	entries[0].data = manifestJSON

	out := w
	var enc *encryptWriter
	if passphrase != "" {
		if enc, err = newEncryptWriter(w, passphrase); err != nil {
			return Manifest{}, err
		}
		out = enc
	}
	tw := tar.NewWriter(out)
	for _, entry := range entries {
		if err := writeEntry(tw, entry.name, int64(len(entry.data)), m.CreatedAt); err != nil {
			return Manifest{}, err
		}
		if _, err := tw.Write(entry.data); err != nil {
			return Manifest{}, err
		}
	}
	if logsPath != "" {
		if err := writeEntry(tw, logsName, logsSize, m.CreatedAt); err != nil {
			return Manifest{}, err
		}
		f, err := os.Open(logsPath)
		if err != nil {
			return Manifest{}, err
		}
		_, err = io.Copy(tw, f)
		f.Close()
		if err != nil {
			return Manifest{}, err
		}
	}
	if err := tw.Close(); err != nil {
		return Manifest{}, err
	}
	if enc != nil {
		if err := enc.Close(); err != nil {
			return Manifest{}, err
		}
	}
	return m, nil
}

type archiveEntry struct {
	name string
	data []byte
}

func writeEntry(tw *tar.Writer, name string, size int64, modTime time.Time) error {
	return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: size, Mode: 0o600, ModTime: modTime})
}

func fileOf(name string, data []byte) File {
	sum := sha256.Sum256(data)
	return File{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

func hashFile(path string) (File, error) {
	f, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return File{}, err
	}
	return File{Name: logsName, Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// stage reads the archive in r into dir, checking every file against the
// manifest as it is written, then checks that each file is one this version
// can load. It touches nothing outside dir; on error the caller removes dir.
func stage(r io.Reader, passphrase, dir string) (Manifest, bool, error) {
	br := bufio.NewReaderSize(r, chunkSize)
	encrypted := isEncrypted(br)
	var in io.Reader = br
	if encrypted {
		if passphrase == "" {
			return Manifest{}, true, invalid(ErrPassphraseRequired)
		}
		dr, err := newDecryptReader(br, passphrase)
		if err != nil {
			return Manifest{}, true, invalid(err)
		}
		in = dr
	}

	tr := tar.NewReader(in)
	hdr, err := tr.Next()
	if err != nil {
		if errors.Is(err, ErrBadPassphrase) {
			return Manifest{}, encrypted, invalid(err)
		}
		return Manifest{}, encrypted, invalid(ErrNotBackup)
	}
	if hdr.Name != manifestName || hdr.Size > maxManifestSize {
		return Manifest{}, encrypted, invalid(ErrNotBackup)
	}
	var m Manifest
	if err := json.NewDecoder(tr).Decode(&m); err != nil {
		return Manifest{}, encrypted, corrupt(err)
	}
	if m.Format != FormatVersion {
		return m, encrypted, invalid(fmt.Errorf("backup format %d is not supported by this LogDeck, which reads format %d", m.Format, FormatVersion))
	}

	expected := make(map[string]File, len(m.Files))
	for _, f := range m.Files {
		switch f.Name {
		case configName, historyName, auditName, logsName:
		default:
			return m, encrypted, invalid(fmt.Errorf("backup lists an unknown file %q", f.Name))
		}
		if _, dup := expected[f.Name]; dup {
			return m, encrypted, invalid(fmt.Errorf("backup lists %q twice", f.Name))
		}
		expected[f.Name] = f
	}
	if _, ok := expected[configName]; !ok {
		return m, encrypted, invalid(errors.New("backup has no config file"))
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, encrypted, corrupt(err)
		}
		f, ok := expected[hdr.Name]
		if !ok || hdr.Typeflag != tar.TypeReg || hdr.Size != f.Size {
			return m, encrypted, corrupt(fmt.Errorf("unexpected entry %q", hdr.Name))
		}
		if hdr.Name != logsName && hdr.Size > maxSmallFile {
			return m, encrypted, corrupt(fmt.Errorf("%s is too large", hdr.Name))
		}
		if err := extract(tr, filepath.Join(dir, hdr.Name), f.SHA256); err != nil {
			return m, encrypted, err
		}
		delete(expected, hdr.Name)
	}
	if len(expected) > 0 {
		return m, encrypted, corrupt(errors.New("files listed in the manifest are missing"))
	}
	// The tar end marker can arrive before the final encrypted chunk is
	// read; reading to the end authenticates it, so a cut-short file fails.
	if _, err := io.Copy(io.Discard, in); err != nil {
		return m, encrypted, corrupt(err)
	}

	if err := checkStaged(dir, m); err != nil {
		return m, encrypted, invalid(err)
	}
	return m, encrypted, nil
}

func extract(r io.Reader, path, wantSHA string) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), r); err != nil {
		out.Close()
		return corrupt(err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != wantSHA {
		return corrupt(fmt.Errorf("%s does not match its checksum", filepath.Base(path)))
	}
	return nil
}

// checkStaged parses every staged file the way LogDeck will load it, so a
// backup that passes its checksums but would not start is refused here.
func checkStaged(dir string, m Manifest) error {
	var fc config.FileConfig
	if err := readJSON(filepath.Join(dir, configName), &fc); err != nil {
		return fmt.Errorf("the backup's config file is invalid: %w", err)
	}
	for _, f := range m.Files {
		switch f.Name {
		case historyName:
			var history []models.Alert
			if err := readJSON(filepath.Join(dir, historyName), &history); err != nil {
				return fmt.Errorf("the backup's alert history is invalid: %w", err)
			}
		case auditName:
			if err := audit.Check(filepath.Join(dir, auditName)); err != nil {
				return fmt.Errorf("the backup's audit log is invalid: %w", err)
			}
		case logsName:
			if err := logstore.CheckSnapshot(filepath.Join(dir, logsName)); err != nil {
				return err
			}
		}
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// InvalidError reports a problem with the backup itself, as opposed to a
// failure on this side, such as a full disk.
type InvalidError struct{ Err error }

func (e *InvalidError) Error() string { return e.Err.Error() }
func (e *InvalidError) Unwrap() error { return e.Err }

func invalid(err error) error {
	var ie *InvalidError
	if errors.As(err, &ie) {
		return err
	}
	return &InvalidError{Err: err}
}

func corrupt(err error) error {
	if errors.Is(err, ErrBadPassphrase) {
		return invalid(err)
	}
	return invalid(fmt.Errorf("the backup is incomplete or corrupted: %w", err))
}
