package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/go-chi/chi/v5"
)

const (
	maxAlertSilences        = 50
	maxAlertSilenceComment  = 200
	maxSilenceWindowMinutes = 24 * 60
)

var (
	errAlertSilenceLimit    = fmt.Errorf("maximum of %d silences reached", maxAlertSilences)
	errAlertSilenceNotFound = errors.New("silence not found")
)

// alertSilenceRequest is the client-supplied portion of a silence.
type alertSilenceRequest struct {
	Comment   string                  `json:"comment"`
	RuleID    string                  `json:"ruleId"`
	Host      string                  `json:"host"`
	Container string                  `json:"container"`
	Project   string                  `json:"project"`
	StartsAt  string                  `json:"startsAt"`
	EndsAt    string                  `json:"endsAt"`
	Schedule  *config.SilenceSchedule `json:"schedule"`
}

// alertSilenceView is a stored silence plus whether it is muting right now
// and, if so, until when ("" = until deleted).
type alertSilenceView struct {
	config.AlertSilence
	Active bool   `json:"active"`
	Until  string `json:"until,omitempty"`
}

// buildAlertSilence validates and normalizes a silence request. ID and
// CreatedAt are left empty for the caller to fill in.
func buildAlertSilence(req alertSilenceRequest, now time.Time) (config.AlertSilence, error) {
	s := config.AlertSilence{
		Comment:   strings.TrimSpace(req.Comment),
		RuleID:    strings.TrimSpace(req.RuleID),
		Host:      strings.TrimSpace(req.Host),
		Container: strings.TrimPrefix(strings.TrimSpace(req.Container), "/"),
		Project:   strings.TrimSpace(req.Project),
		StartsAt:  strings.TrimSpace(req.StartsAt),
		EndsAt:    strings.TrimSpace(req.EndsAt),
		Schedule:  req.Schedule,
	}
	if len(s.Comment) > maxAlertSilenceComment {
		return s, fmt.Errorf("comment must be at most %d characters", maxAlertSilenceComment)
	}

	if sch := s.Schedule; sch != nil {
		if s.StartsAt != "" || s.EndsAt != "" {
			return s, errors.New("a scheduled silence takes no startsAt or endsAt")
		}
		for _, day := range sch.Days {
			if !alerts.ValidSilenceDay(day) {
				return s, fmt.Errorf("schedule days contains invalid value %q (must be mon, tue, wed, thu, fri, sat or sun)", day)
			}
		}
		if _, err := time.Parse("15:04", sch.Start); err != nil {
			return s, errors.New("schedule start must be a time of day like 02:30")
		}
		if sch.DurationMinutes < 1 || sch.DurationMinutes > maxSilenceWindowMinutes {
			return s, fmt.Errorf("schedule durationMinutes must be between 1 and %d", maxSilenceWindowMinutes)
		}
		if _, err := time.LoadLocation(sch.Timezone); err != nil {
			return s, fmt.Errorf("schedule timezone %q is not a known time zone", sch.Timezone)
		}
		return s, nil
	}

	var start, end time.Time
	var err error
	if s.StartsAt != "" {
		if start, err = time.Parse(time.RFC3339, s.StartsAt); err != nil {
			return s, errors.New("startsAt must be an RFC 3339 time")
		}
	}
	if s.EndsAt != "" {
		if end, err = time.Parse(time.RFC3339, s.EndsAt); err != nil {
			return s, errors.New("endsAt must be an RFC 3339 time")
		}
		if !end.After(now) || (!start.IsZero() && !end.After(start)) {
			return s, errors.New("endsAt must be in the future and after startsAt")
		}
	}
	return s, nil
}

// expired reports whether a one-off silence has ended for good.
func expired(s config.AlertSilence, now time.Time) bool {
	if s.Schedule != nil || s.EndsAt == "" {
		return false
	}
	end, err := time.Parse(time.RFC3339, s.EndsAt)
	return err == nil && !now.Before(end)
}

// ListAlertSilences handles GET /api/v1/alerts/silences. Ended one-off
// silences are left out.
func (ar *APIRouter) ListAlertSilences(w http.ResponseWriter, r *http.Request) {
	fc := ar.manager.FileConfigSnapshot()
	now := time.Now()
	views := []alertSilenceView{}
	if fc.Alerts != nil {
		for _, s := range fc.Alerts.Silences {
			if expired(s, now) {
				continue
			}
			view := alertSilenceView{AlertSilence: s}
			var until time.Time
			if view.Active, until = alerts.SilenceActive(s, now); !until.IsZero() {
				view.Until = until.UTC().Format(time.RFC3339)
			}
			views = append(views, view)
		}
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"silences": views})
}

// CreateAlertSilence handles POST /api/v1/alerts/silences. Ended one-off
// silences are dropped from the stored list on the way.
func (ar *APIRouter) CreateAlertSilence(w http.ResponseWriter, r *http.Request) {
	var req alertSilenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	now := time.Now()
	silence, err := buildAlertSilence(req, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := generateAlertID()
	if err != nil {
		http.Error(w, "failed to generate silence id", http.StatusInternalServerError)
		return
	}
	silence.ID = id
	silence.CreatedAt = now.UTC().Format(time.RFC3339)

	err = ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		kept := current.Silences[:0]
		for _, s := range current.Silences {
			if !expired(s, now) {
				kept = append(kept, s)
			}
		}
		if len(kept) >= maxAlertSilences {
			return current, errAlertSilenceLimit
		}
		current.Silences = append(kept, silence)
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertSilenceLimit) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	WriteJsonResponse(w, http.StatusCreated, silence)
}

// DeleteAlertSilence handles DELETE /api/v1/alerts/silences/{id}, which ends
// the silence at once.
func (ar *APIRouter) DeleteAlertSilence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		next := current.Silences[:0]
		for _, s := range current.Silences {
			if s.ID != id {
				next = append(next, s)
			}
		}
		if len(next) == len(current.Silences) {
			return current, errAlertSilenceNotFound
		}
		current.Silences = next
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertSilenceNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "silence deleted"})
}
