package api

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/audit"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/go-chi/chi/v5"
)

const (
	defaultAuditLimit = 50
	maxAuditLimit     = 500
)

// auditActions names the recorded routes. A route missing here is recorded
// under its method and pattern; one mapped to "" changes nothing and is not
// recorded.
var auditActions = map[string]string{
	"POST /api/v1/containers/{id}/start":            "container.start",
	"POST /api/v1/containers/{id}/stop":             "container.stop",
	"POST /api/v1/containers/{id}/restart":          "container.restart",
	"POST /api/v1/containers/{id}/remove":           "container.remove",
	"PUT /api/v1/containers/{id}/env":               "container.env.update",
	"POST /api/v1/containers/{id}/env/deploy":       "container.env.deploy",
	"PUT /api/v1/containers/{id}/resources":         "container.resources.update",
	"POST /api/v1/containers/{id}/resources/deploy": "container.resources.deploy",
	"POST /api/v1/containers/{id}/image/check":      "",
	"POST /api/v1/containers/{id}/image/update":     "container.image.update",
	"GET /api/v1/containers/{id}/exec":              "container.terminal",
	"POST /api/v1/containers/{id}/exec/run":         "container.exec",
	"POST /api/v1/compose/{project}/{action}":       "compose.",
	"DELETE /api/v1/history/containers":             "history.purge_all",
	"DELETE /api/v1/history/containers/{name}":      "history.purge",
	"DELETE /api/v1/history/removed":                "history.purge_removed",
	"PUT /api/v1/settings/docker-hosts":             "settings.docker_hosts",
	"PUT /api/v1/settings/coolify-hosts":            "settings.coolify_hosts",
	"PUT /api/v1/settings/dokploy-hosts":            "settings.dokploy_hosts",
	"PUT /api/v1/settings/read-only":                "settings.read_only",
	"PUT /api/v1/settings/auth":                     "settings.auth",
	"PUT /api/v1/settings/oidc":                     "settings.oidc",
	"POST /api/v1/settings/users":                   "user.create",
	"PUT /api/v1/settings/users/{username}":         "user.update",
	"DELETE /api/v1/settings/users/{username}":      "user.delete",
	"PUT /api/v1/settings/log-storage":              "settings.log_storage",
	"PUT /api/v1/settings/redaction":                "settings.redaction",
	"POST /api/v1/settings/redaction/preview":       "",
	"POST /api/v1/settings/api-tokens":              "api_token.create",
	"DELETE /api/v1/settings/api-tokens/{prefix}":   "api_token.delete",
	"POST /api/v1/settings/test/docker-host":        "",
	"POST /api/v1/settings/test/coolify-host":       "",
	"POST /api/v1/settings/test/dokploy-host":       "",
	"POST /api/v1/settings/backups":                 "backup.create",
	"PUT /api/v1/settings/backups/schedule":         "backup.schedule",
	"DELETE /api/v1/settings/backups/{name}":        "backup.delete",
	"POST /api/v1/settings/backups/restore":         "backup.restore.upload",
	"POST /api/v1/settings/backups/{name}/restore":  "backup.restore.stage",
	"POST /api/v1/settings/backups/restore/confirm": "backup.restore.confirm",
	"DELETE /api/v1/settings/backups/restore":       "backup.restore.cancel",
	"POST /api/v1/alerts/rules":                     "alert.rule.create",
	"PUT /api/v1/alerts/rules/{id}":                 "alert.rule.update",
	"DELETE /api/v1/alerts/rules/{id}":              "alert.rule.delete",
	"POST /api/v1/alerts/channels":                  "alert.channel.create",
	"PUT /api/v1/alerts/channels/{id}":              "alert.channel.update",
	"DELETE /api/v1/alerts/channels/{id}":           "alert.channel.delete",
	"POST /api/v1/alerts/channels/{id}/test":        "alert.channel.test",
	"POST /api/v1/alerts/silences":                  "alert.silence.create",
	"DELETE /api/v1/alerts/silences/{id}":           "alert.silence.delete",
	"DELETE /api/v1/alerts/history":                 "alert.history.clear",
}

// auditTargetParams are the path parameters that name what a route changes,
// in the order they are looked for.
var auditTargetParams = []string{"id", "project", "name", "prefix", "username"}

// auditMutations records every request that can change something, after it
// has been handled. Reads pass straight through without touching the audit
// path; the terminal, a GET that opens a shell, is wrapped with audited
// directly.
func (ar *APIRouter) auditMutations(next http.Handler) http.Handler {
	recorded := ar.audited(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
		default:
			recorded.ServeHTTP(w, r)
		}
	})
}

// audited records the request it wraps. It must run after the auth
// middleware, which places the identity on the request. The record is made
// once the handler returns, or when the connection is taken over for a
// websocket, so a terminal session is on record as soon as it opens.
func (ar *APIRouter) audited(next http.Handler) http.Handler {
	if ar.audit == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var target string
		ctx := context.WithValue(r.Context(), auditTargetKey{}, &target)
		r = r.WithContext(ctx)
		aw := &auditWriter{ResponseWriter: w}
		aw.onHijack = func() { ar.recordAudit(r, start, http.StatusSwitchingProtocols, target) }
		defer func() {
			if aw.hijacked {
				return
			}
			// A handler that panicked wrote nothing; Recoverer above sends 500.
			status := aw.status
			if status == 0 {
				status = http.StatusInternalServerError
			}
			ar.recordAudit(r, start, status, target)
		}()
		next.ServeHTTP(aw, r)
	})
}

type auditTargetKey struct{}

// setAuditTarget names the target of a change for routes whose path does
// not, such as creating an API token. Never pass a secret.
func setAuditTarget(r *http.Request, target string) {
	if p, ok := r.Context().Value(auditTargetKey{}).(*string); ok {
		*p = target
	}
}

func (ar *APIRouter) recordAudit(r *http.Request, start time.Time, status int, target string) {
	rctx := chi.RouteContext(r.Context())
	route := rctx.RoutePattern()
	action, known := auditActions[r.Method+" "+route]
	if known && action == "" {
		return
	}
	if !known {
		action = r.Method + " " + route
	}
	if action == "compose." {
		action += chi.URLParam(r, "action")
	}
	if target == "" {
		for _, key := range auditTargetParams {
			if v := chi.URLParam(r, key); v != "" {
				target = v
				break
			}
		}
	}
	actor, role := requestIdentity(r)
	ar.audit.Record(audit.Entry{
		Time:       start.UTC(),
		Actor:      actor,
		Role:       role,
		Action:     action,
		Method:     r.Method,
		Route:      route,
		Host:       r.URL.Query().Get("host"),
		Target:     target,
		Status:     status,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

// requestIdentity returns who made a request, as the auth middleware placed
// it on the request: the username and role for a session, "token:<name>" and
// the token's scope for an API token, and nothing when auth is disabled.
func requestIdentity(r *http.Request) (actor, role string) {
	user, _ := r.Context().Value(auth.UserContextKey).(models.User)
	return user.Username, user.Role
}

// auditWriter captures the response status and notices a websocket takeover.
type auditWriter struct {
	http.ResponseWriter
	status   int
	hijacked bool
	onHijack func()
}

func (w *auditWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *auditWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Hijack is called by the websocket upgrader directly, which needs the
// interface on the writer itself.
func (w *auditWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked = true
		w.onHijack()
	}
	return conn, rw, err
}

// GetAuditLog handles GET /api/v1/settings/audit. It sits in the settings
// group, so read-scoped tokens are denied.
func (ar *APIRouter) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := audit.Filter{
		Actor:  strings.TrimSpace(q.Get("actor")),
		Action: strings.TrimSpace(q.Get("action")),
		Target: strings.TrimSpace(q.Get("target")),
		Host:   strings.TrimSpace(q.Get("host")),
		Result: q.Get("result"),
		Limit:  defaultAuditLimit,
	}
	switch f.Result {
	case "", "success", "failure":
	default:
		http.Error(w, "result must be success or failure", http.StatusBadRequest)
		return
	}
	for key, dst := range map[string]*time.Time{"since": &f.Since, "until": &f.Until} {
		if raw := q.Get(key); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				http.Error(w, key+" must be an RFC 3339 time", http.StatusBadRequest)
				return
			}
			*dst = t
		}
	}
	for key, dst := range map[string]*int{"limit": &f.Limit, "offset": &f.Offset} {
		if raw := q.Get(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				http.Error(w, key+" must be a non-negative integer", http.StatusBadRequest)
				return
			}
			*dst = n
		}
	}
	if f.Limit < 1 {
		f.Limit = 1
	}
	if f.Limit > maxAuditLimit {
		f.Limit = maxAuditLimit
	}

	entries, total := ar.audit.List(f)
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
	})
}
