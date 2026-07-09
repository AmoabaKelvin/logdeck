package api

import (
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api/middleware"
	"github.com/go-chi/chi/v5"
)

var validComposeActions = map[string]bool{
	"start":   true,
	"stop":    true,
	"restart": true,
}

func (ar *APIRouter) registerComposeRoutes(r chi.Router) {
	// Mutating routes (blocked in read-only mode)
	r.Group(func(mutating chi.Router) {
		mutating.Use(middleware.ReadOnly(func() bool {
			return ar.registry.Config().ReadOnly
		}))
		mutating.Post("/compose/{project}/{action}", ar.ComposeAction)
	})
}

// ComposeAction applies start/stop/restart to every container in a compose
// project on one host. Responds 200 when all containers succeed, 500 with the
// per-container failures in the body when any fail, and 404 when the project
// has no containers on the host.
func (ar *APIRouter) ComposeAction(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	action := chi.URLParam(r, "action")
	host := r.URL.Query().Get("host")

	if host == "" {
		http.Error(w, "host parameter is required", http.StatusBadRequest)
		return
	}

	if !validComposeActions[action] {
		http.Error(w, "invalid action: must be one of start, stop, restart", http.StatusBadRequest)
		return
	}

	result, err := ar.registry.Docker().ComposeProjectAction(r.Context(), host, project, action)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.Total == 0 {
		http.Error(w, "no containers found for compose project "+project, http.StatusNotFound)
		return
	}

	status := http.StatusOK
	if len(result.Failed) > 0 {
		status = http.StatusInternalServerError
	}
	WriteJsonResponse(w, status, result)
}
