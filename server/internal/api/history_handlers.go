package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp/syntax"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// validHistoryLevels are the level names accepted by the history log filter.
// UNKNOWN is included: unclassified lines are stored under it and a user may
// legitimately want to look at them.
var validHistoryLevels = map[string]bool{
	string(models.LogLevelTrace):   true,
	string(models.LogLevelDebug):   true,
	string(models.LogLevelInfo):    true,
	string(models.LogLevelWarn):    true,
	string(models.LogLevelError):   true,
	string(models.LogLevelFatal):   true,
	string(models.LogLevelPanic):   true,
	string(models.LogLevelUnknown): true,
}

// GetHistoryStatus reports whether log persistence is available. The frontend
// hides History mode when it is not.
func (ar *APIRouter) GetHistoryStatus(w http.ResponseWriter, r *http.Request) {
	WriteJsonResponse(w, http.StatusOK, map[string]bool{
		"enabled": ar.logStore != nil,
	})
}

// GetHistoryContainers lists every logical container the store knows about.
// With persistence disabled the list is simply empty.
func (ar *APIRouter) GetHistoryContainers(w http.ResponseWriter, r *http.Request) {
	containers := []logstore.StoredContainer{}
	if ar.logStore != nil {
		stored, err := ar.logStore.ListContainers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		containers = stored
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"containers": containers,
	})
}

// GetHistoryLogs returns one page of stored logs for a logical container.
// Pages walk backwards through history: follow nextCursor for older lines.
func (ar *APIRouter) GetHistoryLogs(w http.ResponseWriter, r *http.Request) {
	if ar.logStore == nil {
		WriteJsonResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "log persistence is disabled",
		})
		return
	}

	query, ok := parseHistoryQuery(w, r)
	if !ok {
		return
	}

	page, err := ar.logStore.Query(r.Context(), query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, logstore.ErrInvalidCursor) || isInvalidSearchPattern(err) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	logs := page.Entries
	if logs == nil {
		logs = []models.LogEntry{}
	}

	response := map[string]any{
		"logs":  logs,
		"count": len(logs),
	}
	if page.NextCursor != "" {
		response["nextCursor"] = page.NextCursor
	}
	WriteJsonResponse(w, http.StatusOK, response)
}

// parseHistoryQuery validates the query parameters and writes the 400 itself
// when one is bad, reporting ok=false.
func parseHistoryQuery(w http.ResponseWriter, r *http.Request) (logstore.LogQuery, bool) {
	params := r.URL.Query()

	container := strings.TrimSpace(params.Get("container"))
	if container == "" {
		http.Error(w, "container is required", http.StatusBadRequest)
		return logstore.LogQuery{}, false
	}

	query := logstore.LogQuery{
		Host:      params.Get("host"),
		Container: container,
		Search:    params.Get("search"),
		Cursor:    params.Get("cursor"),
	}

	for _, param := range []struct {
		name string
		dest *time.Time
	}{{"since", &query.Since}, {"until", &query.Until}} {
		value := params.Get(param.name)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid %s: expected an RFC3339 timestamp", param.name), http.StatusBadRequest)
			return logstore.LogQuery{}, false
		}
		*param.dest = parsed
	}

	if levels := params.Get("levels"); levels != "" {
		for _, level := range strings.Split(levels, ",") {
			level = strings.ToUpper(strings.TrimSpace(level))
			if level == "" {
				continue
			}
			if !validHistoryLevels[level] {
				http.Error(w, fmt.Sprintf("invalid level: %s", level), http.StatusBadRequest)
				return logstore.LogQuery{}, false
			}
			query.Levels = append(query.Levels, level)
		}
	}

	if regex := params.Get("regex"); regex != "" {
		parsed, err := strconv.ParseBool(regex)
		if err != nil {
			http.Error(w, "invalid regex: expected a boolean", http.StatusBadRequest)
			return logstore.LogQuery{}, false
		}
		query.Regex = parsed
	}

	if limit := params.Get("limit"); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil {
			http.Error(w, "invalid limit: expected an integer", http.StatusBadRequest)
			return logstore.LogQuery{}, false
		}
		query.Limit = parsed
	}

	return query, true
}

// isInvalidSearchPattern reports whether the store rejected the query because
// its regex would not compile — a client error, not a server one. The store
// wraps regexp's own error, so it surfaces as a syntax error.
func isInvalidSearchPattern(err error) bool {
	var syntaxErr *syntax.Error
	return errors.As(err, &syntaxErr)
}
