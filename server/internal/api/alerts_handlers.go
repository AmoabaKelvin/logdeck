package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/go-chi/chi/v5"
)

const (
	maxAlertRules        = 50
	maxAlertRuleNameLen  = 64
	maxAlertThreshold    = 1000
	minAlertWindowSecs   = 5
	maxAlertWindowSecs   = 3600
	maxAlertCooldownSecs = 86400

	defaultAlertHistoryLimit = 100
	maxAlertHistoryLimit     = 500
)

var (
	errAlertRuleLimit    = fmt.Errorf("maximum of %d alert rules reached", maxAlertRules)
	errAlertRuleNotFound = errors.New("alert rule not found")
)

// alertRuleRequest is the client-supplied portion of an alert rule. Enabled is
// a pointer so an omitted value can default to true.
type alertRuleRequest struct {
	Name            string   `json:"name"`
	Enabled         *bool    `json:"enabled"`
	Type            string   `json:"type"`
	Hosts           []string `json:"hosts"`
	Containers      []string `json:"containers"`
	Projects        []string `json:"projects"`
	Events          []string `json:"events"`
	MinLevel        string   `json:"minLevel"`
	Pattern         string   `json:"pattern"`
	Threshold       int      `json:"threshold"`
	WindowSeconds   int      `json:"windowSeconds"`
	CooldownSeconds int      `json:"cooldownSeconds"`
}

// validAlertMinLevels are the log levels accepted for a log rule's minLevel.
// UNKNOWN is deliberately excluded: unclassified lines never satisfy a
// min-level threshold.
var validAlertMinLevels = map[string]bool{
	string(models.LogLevelTrace): true,
	string(models.LogLevelDebug): true,
	string(models.LogLevelInfo):  true,
	string(models.LogLevelWarn):  true,
	string(models.LogLevelError): true,
	string(models.LogLevelFatal): true,
	string(models.LogLevelPanic): true,
}

var validAlertEvents = map[string]bool{
	"die":       true,
	"oom":       true,
	"unhealthy": true,
}

// buildAlertRule validates and normalizes a rule request into a config rule.
// ID and CreatedAt are left empty for the caller to fill in.
func buildAlertRule(req alertRuleRequest) (config.AlertRule, error) {
	rule := config.AlertRule{
		Name:            strings.TrimSpace(req.Name),
		Enabled:         true,
		Type:            req.Type,
		Hosts:           req.Hosts,
		Containers:      req.Containers,
		Projects:        req.Projects,
		Events:          req.Events,
		MinLevel:        strings.ToUpper(strings.TrimSpace(req.MinLevel)),
		Pattern:         strings.TrimSpace(req.Pattern),
		Threshold:       req.Threshold,
		WindowSeconds:   req.WindowSeconds,
		CooldownSeconds: req.CooldownSeconds,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	if rule.Name == "" {
		return rule, errors.New("name is required")
	}
	if len(rule.Name) > maxAlertRuleNameLen {
		return rule, fmt.Errorf("name must be at most %d characters", maxAlertRuleNameLen)
	}

	// Normalize container names: the engine and hub match against names
	// without the Docker API's leading "/", so "/web" would never match.
	for i, c := range rule.Containers {
		rule.Containers[i] = strings.TrimPrefix(strings.TrimSpace(c), "/")
	}

	for _, targets := range []struct {
		field  string
		values []string
	}{
		{"hosts", rule.Hosts},
		{"containers", rule.Containers},
		{"projects", rule.Projects},
	} {
		for _, v := range targets.values {
			if strings.TrimSpace(v) == "" {
				return rule, fmt.Errorf("%s must not contain empty entries", targets.field)
			}
		}
	}

	switch rule.Type {
	case "event":
		if len(rule.Events) == 0 {
			return rule, errors.New("events is required for an event rule")
		}
		for _, ev := range rule.Events {
			if !validAlertEvents[ev] {
				return rule, fmt.Errorf("events contains invalid value %q (must be \"die\", \"oom\", or \"unhealthy\")", ev)
			}
		}
		if rule.MinLevel != "" {
			return rule, errors.New("minLevel must be empty for an event rule")
		}
		if rule.Pattern != "" {
			return rule, errors.New("pattern must be empty for an event rule")
		}
	case "log":
		if len(rule.Events) > 0 {
			return rule, errors.New("events must be empty for a log rule")
		}
		if rule.MinLevel == "" && rule.Pattern == "" {
			return rule, errors.New("a log rule requires at least one of minLevel or pattern")
		}
		if rule.MinLevel != "" && !validAlertMinLevels[rule.MinLevel] {
			return rule, fmt.Errorf("minLevel %q is not a valid log level", rule.MinLevel)
		}
		if rule.Pattern != "" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return rule, fmt.Errorf("pattern is not a valid regular expression: %v", err)
			}
		}
	case "":
		return rule, errors.New("type is required")
	default:
		return rule, fmt.Errorf("type %q is invalid (must be \"event\" or \"log\")", rule.Type)
	}

	if rule.Threshold < 0 {
		return rule, errors.New("threshold must not be negative")
	}
	if rule.Threshold > maxAlertThreshold {
		return rule, fmt.Errorf("threshold must be at most %d", maxAlertThreshold)
	}
	if rule.Threshold == 0 {
		rule.Threshold = 1
	}

	if rule.WindowSeconds < 0 {
		return rule, errors.New("windowSeconds must not be negative")
	}
	if rule.WindowSeconds == 0 {
		rule.WindowSeconds = 60
	} else if rule.WindowSeconds < minAlertWindowSecs || rule.WindowSeconds > maxAlertWindowSecs {
		return rule, fmt.Errorf("windowSeconds must be between %d and %d", minAlertWindowSecs, maxAlertWindowSecs)
	}

	if rule.CooldownSeconds < 0 {
		return rule, errors.New("cooldownSeconds must not be negative")
	}
	if rule.CooldownSeconds > maxAlertCooldownSecs {
		return rule, fmt.Errorf("cooldownSeconds must be at most %d", maxAlertCooldownSecs)
	}

	return rule, nil
}

// generateAlertRuleID returns a random 8-character hex rule identifier.
func generateAlertRuleID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// reconcileAlerts nudges the alerting engine after a config change. The engine
// is nil in some router configurations (e.g. tests), which is safe to skip.
func (ar *APIRouter) reconcileAlerts() {
	if ar.engine != nil {
		ar.engine.Reconcile()
	}
}

// ListAlertRules handles GET /api/v1/alerts/rules.
func (ar *APIRouter) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	fc := ar.manager.FileConfigSnapshot()
	rules := []config.AlertRule{}
	if fc.Alerts != nil {
		rules = append(rules, fc.Alerts.Rules...)
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"rules": rules})
}

// CreateAlertRule handles POST /api/v1/alerts/rules.
func (ar *APIRouter) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var req alertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rule, err := buildAlertRule(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := generateAlertRuleID()
	if err != nil {
		http.Error(w, "failed to generate rule id", http.StatusInternalServerError)
		return
	}
	rule.ID = id
	rule.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	err = ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		if len(current.Rules) >= maxAlertRules {
			return current, errAlertRuleLimit
		}
		current.Rules = append(current.Rules, rule)
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertRuleLimit) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusCreated, rule)
}

// UpdateAlertRule handles PUT /api/v1/alerts/rules/{id}.
func (ar *APIRouter) UpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req alertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rule, err := buildAlertRule(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		for i, existing := range current.Rules {
			if existing.ID == id {
				rule.ID = existing.ID
				rule.CreatedAt = existing.CreatedAt
				current.Rules[i] = rule
				return current, nil
			}
		}
		return current, errAlertRuleNotFound
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertRuleNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusOK, rule)
}

// DeleteAlertRule handles DELETE /api/v1/alerts/rules/{id}.
func (ar *APIRouter) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		next := current.Rules[:0]
		for _, rule := range current.Rules {
			if rule.ID != id {
				next = append(next, rule)
			}
		}
		if len(next) == len(current.Rules) {
			return current, errAlertRuleNotFound
		}
		current.Rules = next
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertRuleNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "rule deleted"})
}

// GetAlertsWebhook handles GET /api/v1/alerts/webhook.
func (ar *APIRouter) GetAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	fc := ar.manager.FileConfigSnapshot()
	webhookURL := ""
	if fc.Alerts != nil {
		webhookURL = fc.Alerts.WebhookURL
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"url": webhookURL})
}

// UpdateAlertsWebhook handles PUT /api/v1/alerts/webhook.
func (ar *APIRouter) UpdateAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	webhookURL := strings.TrimSpace(req.URL)
	if webhookURL != "" {
		parsed, err := url.Parse(webhookURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			http.Error(w, "url must be a valid http or https URL", http.StatusBadRequest)
			return
		}
	}

	err := ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		current.WebhookURL = webhookURL
		return current, nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ar.reconcileAlerts()
	message := "Webhook updated"
	if webhookURL == "" {
		message = "Webhook cleared"
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": message})
}

// TestAlertsWebhook handles POST /api/v1/alerts/test. The delivery result is
// returned with a 200 whether or not the delivery itself succeeded; the
// result's status field carries the outcome.
func (ar *APIRouter) TestAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	if ar.engine == nil {
		http.Error(w, "alerting engine not available", http.StatusInternalServerError)
		return
	}
	result := ar.engine.TestWebhook(r.Context())
	WriteJsonResponse(w, http.StatusOK, result)
}

// GetAlertHistory handles GET /api/v1/alerts/history.
func (ar *APIRouter) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	if ar.engine == nil {
		http.Error(w, "alerting engine not available", http.StatusInternalServerError)
		return
	}

	limit := defaultAlertHistoryLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "limit must be an integer", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if limit < 1 {
		limit = 1
	}
	if limit > maxAlertHistoryLimit {
		limit = maxAlertHistoryLimit
	}

	alerts := ar.engine.History(limit)
	if alerts == nil {
		alerts = []models.Alert{}
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// ClearAlertHistory handles DELETE /api/v1/alerts/history.
func (ar *APIRouter) ClearAlertHistory(w http.ResponseWriter, r *http.Request) {
	if ar.engine == nil {
		http.Error(w, "alerting engine not available", http.StatusInternalServerError)
		return
	}
	ar.engine.ClearHistory()
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "history cleared"})
}
