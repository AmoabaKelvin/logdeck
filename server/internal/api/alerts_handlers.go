package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/go-chi/chi/v5"
)

const (
	maxAlertRules          = 50
	maxAlertRuleNameLen    = 64
	maxAlertChannels       = 20
	maxAlertChannelNameLen = 64
	maxAlertThreshold      = 1000
	minAlertWindowSecs     = 5
	maxAlertWindowSecs     = 3600
	maxAlertCooldownSecs   = 86400
	minTickedWindowSecs    = 60

	defaultAlertHistoryLimit = 100
	maxAlertHistoryLimit     = 500
)

var (
	errAlertRuleLimit       = fmt.Errorf("maximum of %d alert rules reached", maxAlertRules)
	errAlertRuleNotFound    = errors.New("alert rule not found")
	errAlertChannelLimit    = fmt.Errorf("maximum of %d alert channels reached", maxAlertChannels)
	errAlertChannelNotFound = errors.New("alert channel not found")
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
	Metric          string   `json:"metric"`
	AbovePercent    float64  `json:"abovePercent"`
	AboveBytes      int64    `json:"aboveBytes"`
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
	"restart":   true,
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
		Metric:          req.Metric,
		AbovePercent:    req.AbovePercent,
		AboveBytes:      req.AboveBytes,
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

	if rule.Type != "resource" && (rule.Metric != "" || rule.AbovePercent != 0 || rule.AboveBytes != 0) {
		return rule, errors.New("metric, abovePercent and aboveBytes only apply to a resource rule")
	}

	switch rule.Type {
	case "event":
		if len(rule.Events) == 0 {
			return rule, errors.New("events is required for an event rule")
		}
		for _, ev := range rule.Events {
			if !validAlertEvents[ev] {
				return rule, fmt.Errorf("events contains invalid value %q (must be \"die\", \"oom\", \"unhealthy\", or \"restart\")", ev)
			}
		}
		// A crash loop is a die and a restart each round, so one rule
		// counting both would reach its threshold twice as fast.
		if len(rule.Events) > 1 && slices.Contains(rule.Events, "restart") {
			return rule, errors.New("restart can't be combined with other events in one rule")
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
	case "collection":
		// A collection failure belongs to a host or to the log store, never to
		// one container, so only host targeting means anything.
		if len(rule.Containers) > 0 || len(rule.Projects) > 0 {
			return rule, errors.New("a collection rule can only target hosts")
		}
		if len(rule.Events) > 0 || rule.MinLevel != "" || rule.Pattern != "" {
			return rule, errors.New("events, minLevel and pattern must be empty for a collection rule")
		}
		// Collection state is checked once a minute, so a shorter delay would
		// read as a promise the engine cannot keep.
		if rule.WindowSeconds != 0 && rule.WindowSeconds < minTickedWindowSecs {
			return rule, fmt.Errorf("windowSeconds must be at least %d for a collection rule", minTickedWindowSecs)
		}
		rule.Threshold = 1 // one sustained failure fires; there is no rate
	case "resource":
		if len(rule.Events) > 0 || rule.MinLevel != "" || rule.Pattern != "" {
			return rule, errors.New("events, minLevel and pattern must be empty for a resource rule")
		}
		switch rule.Metric {
		case "cpu":
			if rule.AbovePercent <= 0 || rule.AboveBytes != 0 {
				return rule, errors.New("a cpu rule requires abovePercent greater than 0 (100 is one core) and no aboveBytes")
			}
		case "memory":
			if (rule.AbovePercent != 0) == (rule.AboveBytes != 0) {
				return rule, errors.New("a memory rule requires exactly one of abovePercent or aboveBytes")
			}
			if rule.AbovePercent < 0 || rule.AbovePercent > 100 || rule.AboveBytes < 0 {
				return rule, errors.New("abovePercent must be between 0 and 100, and aboveBytes positive, for a memory rule")
			}
		default:
			return rule, fmt.Errorf("metric %q is invalid (must be \"cpu\" or \"memory\")", rule.Metric)
		}
		// Usage is sampled once a minute, so a shorter window would read as a
		// promise the engine cannot keep.
		if rule.WindowSeconds != 0 && rule.WindowSeconds < minTickedWindowSecs {
			return rule, fmt.Errorf("windowSeconds must be at least %d for a resource rule", minTickedWindowSecs)
		}
		rule.Threshold = 1 // one sustained breach fires; there is no rate
	case "":
		return rule, errors.New("type is required")
	default:
		return rule, fmt.Errorf("type %q is invalid (must be \"event\", \"log\", \"collection\", or \"resource\")", rule.Type)
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

// generateAlertID returns a random 8-character hex identifier for a rule or
// channel.
func generateAlertID() (string, error) {
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

	id, err := generateAlertID()
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

	setAuditTarget(r, rule.ID)
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

// alertChannelRequest is the client-supplied portion of a channel. Enabled is
// a pointer so an omitted value can default to true.
type alertChannelRequest struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Enabled  *bool  `json:"enabled"`
	URL      string `json:"url"`
	Token    string `json:"token"`
	Target   string `json:"target"`
	Username string `json:"username"`
	From     string `json:"from"`
}

var validAlertChannelTypes = map[string]bool{
	"webhook":   true,
	"ntfy":      true,
	"gotify":    true,
	"telegram":  true,
	"slack":     true,
	"discord":   true,
	"email":     true,
	"pagerduty": true,
	"opsgenie":  true,
}

// validateHTTPURL rejects empty or non-http(s) URLs with a field-named error.
func validateHTTPURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("url must be a valid http or https URL")
	}
	return nil
}

// buildAlertChannel validates and normalizes a channel request into a config
// channel. ID is left empty for the caller to fill in. Fields irrelevant to the
// chosen type are cleared so stored channels stay minimal.
func buildAlertChannel(req alertChannelRequest) (config.AlertChannel, error) {
	ch := config.AlertChannel{
		Type:     strings.TrimSpace(req.Type),
		Name:     strings.TrimSpace(req.Name),
		Enabled:  true,
		URL:      strings.TrimSpace(req.URL),
		Token:    strings.TrimSpace(req.Token),
		Target:   strings.TrimSpace(req.Target),
		Username: strings.TrimSpace(req.Username),
		From:     strings.TrimSpace(req.From),
	}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}

	if !validAlertChannelTypes[ch.Type] {
		return ch, fmt.Errorf("type %q is invalid (must be \"webhook\", \"ntfy\", \"gotify\", \"telegram\", \"slack\", \"discord\", \"email\", \"pagerduty\", or \"opsgenie\")", ch.Type)
	}
	if len(ch.Name) > maxAlertChannelNameLen {
		return ch, fmt.Errorf("name must be at most %d characters", maxAlertChannelNameLen)
	}

	if ch.Type != "email" {
		ch.Username = ""
		ch.From = ""
	}

	switch ch.Type {
	case "webhook", "ntfy", "slack", "discord":
		if err := validateHTTPURL(ch.URL); err != nil {
			return ch, err
		}
		ch.Token = ""
		ch.Target = ""
	case "gotify":
		if err := validateHTTPURL(ch.URL); err != nil {
			return ch, err
		}
		if ch.Token == "" {
			return ch, errors.New("token is required for a gotify channel")
		}
		ch.Target = ""
	case "telegram":
		if ch.Token == "" {
			return ch, errors.New("token is required for a telegram channel")
		}
		if ch.Target == "" {
			return ch, errors.New("target is required for a telegram channel (the chat id)")
		}
		ch.URL = ""
	case "pagerduty", "opsgenie":
		// The URL is optional and only needed for an EU-hosted account.
		if ch.URL != "" {
			if err := validateHTTPURL(ch.URL); err != nil {
				return ch, err
			}
		}
		if ch.Token == "" {
			name := "routing key"
			if ch.Type == "opsgenie" {
				name = "API key"
			}
			return ch, fmt.Errorf("token is required for a %s channel (the %s)", ch.Type, name)
		}
		ch.Target = ""
	case "email":
		parsed, err := url.Parse(ch.URL)
		if err != nil || (parsed.Scheme != "smtp" && parsed.Scheme != "smtps") || parsed.Hostname() == "" {
			return ch, errors.New("url must be an smtp:// or smtps:// server URL for an email channel")
		}
		if _, err := mail.ParseAddress(ch.From); err != nil {
			return ch, errors.New("from must be a valid email address")
		}
		if _, err := mail.ParseAddressList(ch.Target); err != nil {
			return ch, errors.New("target must be one or more comma-separated email addresses")
		}
	}

	return ch, nil
}

// ListAlertChannels handles GET /api/v1/alerts/channels.
func (ar *APIRouter) ListAlertChannels(w http.ResponseWriter, r *http.Request) {
	fc := ar.manager.FileConfigSnapshot()
	channels := []config.AlertChannel{}
	if fc.Alerts != nil {
		channels = append(channels, fc.Alerts.Channels...)
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"channels": channels})
}

// CreateAlertChannel handles POST /api/v1/alerts/channels.
func (ar *APIRouter) CreateAlertChannel(w http.ResponseWriter, r *http.Request) {
	var req alertChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	channel, err := buildAlertChannel(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := generateAlertID()
	if err != nil {
		http.Error(w, "failed to generate channel id", http.StatusInternalServerError)
		return
	}
	channel.ID = id

	err = ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		if len(current.Channels) >= maxAlertChannels {
			return current, errAlertChannelLimit
		}
		current.Channels = append(current.Channels, channel)
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertChannelLimit) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	setAuditTarget(r, channel.ID)
	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusCreated, channel)
}

// UpdateAlertChannel handles PUT /api/v1/alerts/channels/{id}.
func (ar *APIRouter) UpdateAlertChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req alertChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	channel, err := buildAlertChannel(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		for i, existing := range current.Channels {
			if existing.ID == id {
				channel.ID = existing.ID
				current.Channels[i] = channel
				return current, nil
			}
		}
		return current, errAlertChannelNotFound
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertChannelNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusOK, channel)
}

// DeleteAlertChannel handles DELETE /api/v1/alerts/channels/{id}.
func (ar *APIRouter) DeleteAlertChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.manager.UpdateAlerts(func(current config.AlertsConfig) (config.AlertsConfig, error) {
		next := current.Channels[:0]
		for _, ch := range current.Channels {
			if ch.ID != id {
				next = append(next, ch)
			}
		}
		if len(next) == len(current.Channels) {
			return current, errAlertChannelNotFound
		}
		current.Channels = next
		return current, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errAlertChannelNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	ar.reconcileAlerts()
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "channel deleted"})
}

// TestAlertChannel handles POST /api/v1/alerts/channels/{id}/test. It delivers
// a synthetic alert to the one channel and returns the delivery result with a
// 200 whether or not the delivery itself succeeded; the result's status field
// carries the outcome.
func (ar *APIRouter) TestAlertChannel(w http.ResponseWriter, r *http.Request) {
	if ar.engine == nil {
		http.Error(w, "alerting engine not available", http.StatusInternalServerError)
		return
	}
	id := chi.URLParam(r, "id")

	fc := ar.manager.FileConfigSnapshot()
	var channel config.AlertChannel
	found := false
	if fc.Alerts != nil {
		for _, ch := range fc.Alerts.Channels {
			if ch.ID == id {
				channel = ch
				found = true
				break
			}
		}
	}
	if !found {
		http.Error(w, errAlertChannelNotFound.Error(), http.StatusNotFound)
		return
	}

	result := ar.engine.TestChannel(r.Context(), channel)
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
