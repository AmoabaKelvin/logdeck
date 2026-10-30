package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/api/middleware"
	"github.com/AmoabaKelvin/logdeck/internal/backup"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/go-chi/chi/v5"
)

const (
	minBackupPassphrase = 8
	maxBackupKeep       = 100
	maxBackupInterval   = 24 * 30
)

// registerBackupRoutes sits inside the settings group, so it inherits its auth
// and its denial of read-scoped tokens. Restoring and deleting destroy data,
// so like purging stored logs they are blocked in read-only mode; taking and
// downloading a backup change nothing and are not.
func (ar *APIRouter) registerBackupRoutes(r chi.Router) {
	if ar.backups == nil {
		return
	}
	r.Get("/backups", ar.GetBackups)
	r.Post("/backups", ar.CreateBackup)
	r.Put("/backups/schedule", ar.UpdateBackupSchedule)
	r.Get("/backups/{name}", ar.DownloadBackup)

	destructive := r.With(middleware.ReadOnly(func() bool { return ar.registry.Config().ReadOnly }))
	destructive.Delete("/backups/{name}", ar.DeleteBackup)
	destructive.Post("/backups/restore", ar.UploadRestore)
	destructive.Post("/backups/{name}/restore", ar.StageStoredRestore)
	destructive.Post("/backups/restore/confirm", ar.ConfirmRestore)
	destructive.Delete("/backups/restore", ar.CancelRestore)
}

// GetBackups handles GET /api/v1/settings/backups.
func (ar *APIRouter) GetBackups(w http.ResponseWriter, r *http.Request) {
	WriteJsonResponse(w, http.StatusOK, ar.backups.Status())
}

// CreateBackup handles POST /api/v1/settings/backups. The backup runs in the
// background, since snapshotting a large log store can outlast a proxy's
// timeout; the status endpoint reports when it is done.
func (ar *APIRouter) CreateBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IncludeLogs    bool   `json:"includeLogs"`
		IncludeSecrets bool   `json:"includeSecrets"`
		Passphrase     string `json:"passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Passphrase != "" && len(req.Passphrase) < minBackupPassphrase {
		http.Error(w, fmt.Sprintf("the passphrase must be at least %d characters", minBackupPassphrase), http.StatusBadRequest)
		return
	}
	err := ar.backups.StartBackup(backup.Options{
		IncludeLogs:    req.IncludeLogs,
		IncludeSecrets: req.IncludeSecrets,
		Passphrase:     req.Passphrase,
	})
	if err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusAccepted, map[string]any{"message": "backup started"})
}

// UpdateBackupSchedule handles PUT /api/v1/settings/backups/schedule.
func (ar *APIRouter) UpdateBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var req config.BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.IntervalHours < 0 || req.IntervalHours > maxBackupInterval {
		http.Error(w, fmt.Sprintf("intervalHours must be between 0 (off) and %d", maxBackupInterval), http.StatusBadRequest)
		return
	}
	if req.Keep < 1 || req.Keep > maxBackupKeep {
		http.Error(w, fmt.Sprintf("keep must be between 1 and %d", maxBackupKeep), http.StatusBadRequest)
		return
	}
	if err := ar.manager.UpdateBackup(req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "backup schedule updated"})
}

// DownloadBackup handles GET /api/v1/settings/backups/{name}. The browser
// downloads through a plain link, so the session token arrives as the token
// query parameter the auth middleware already accepts.
func (ar *APIRouter) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	path, err := ar.backups.Path(name)
	if err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// DeleteBackup handles DELETE /api/v1/settings/backups/{name}.
func (ar *APIRouter) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := ar.backups.Delete(chi.URLParam(r, "name")); err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "backup deleted"})
}

// UploadRestore handles POST /api/v1/settings/backups/restore: the request
// body is the backup file itself, streamed to disk and verified, and the
// passphrase of an encrypted one arrives in the X-Backup-Passphrase header.
// Nothing changes until the restore is confirmed.
func (ar *APIRouter) UploadRestore(w http.ResponseWriter, r *http.Request) {
	// The server's ReadTimeout covers the whole body; a backup with stored
	// logs takes longer than that to upload.
	if err := http.NewResponseController(w).SetReadDeadline(time.Time{}); err != nil {
		log.Printf("backup: lifting the upload deadline failed: %v", err)
	}
	preview, err := ar.backups.StageRestore(r.Body, r.Header.Get("X-Backup-Passphrase"))
	if err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, preview)
}

// StageStoredRestore handles POST /api/v1/settings/backups/{name}/restore,
// checking a backup from the backup directory the way UploadRestore does.
func (ar *APIRouter) StageStoredRestore(w http.ResponseWriter, r *http.Request) {
	preview, err := ar.backups.StageStored(chi.URLParam(r, "name"), r.Header.Get("X-Backup-Passphrase"))
	if err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, preview)
}

// ConfirmRestore handles POST /api/v1/settings/backups/restore/confirm. The
// restore is applied on the next start, before anything opens the files.
func (ar *APIRouter) ConfirmRestore(w http.ResponseWriter, r *http.Request) {
	if err := ar.backups.ConfirmRestore(); err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "restore scheduled; restart LogDeck to apply it"})
}

// CancelRestore handles DELETE /api/v1/settings/backups/restore.
func (ar *APIRouter) CancelRestore(w http.ResponseWriter, r *http.Request) {
	if err := ar.backups.CancelRestore(); err != nil {
		http.Error(w, err.Error(), backupErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "restore cancelled"})
}

func backupErrorStatus(err error) int {
	var invalid *backup.InvalidError
	switch {
	case errors.As(err, &invalid):
		return http.StatusBadRequest
	case errors.Is(err, backup.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, backup.ErrBusy), errors.Is(err, backup.ErrRestoreBusy), errors.Is(err, backup.ErrNoStaged):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
