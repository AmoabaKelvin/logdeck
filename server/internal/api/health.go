package api

import "net/http"

// handleHealthz reports server liveness. Public, no auth.
func (ar *APIRouter) handleHealthz(w http.ResponseWriter, r *http.Request) {
	WriteJsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleVersion returns the server version. Public, no auth.
func (ar *APIRouter) handleVersion(w http.ResponseWriter, r *http.Request) {
	WriteJsonResponse(w, http.StatusOK, map[string]string{"version": ar.version})
}
