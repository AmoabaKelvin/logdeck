package api

import "net/http"

// handleHealthz reports server liveness. Public, no auth.
//
// "collection" says whether LogDeck can still observe every host and
// container. A degraded collection still answers 200 by default, so an
// orchestrator's liveness check never restarts LogDeck over an unreachable
// host; ?strict=1 answers 503 instead, for external monitors that only look at
// the status code.
func (ar *APIRouter) handleHealthz(w http.ResponseWriter, r *http.Request) {
	collection := "ok"
	if ar.health.Degraded() {
		collection = "degraded"
	}
	status := http.StatusOK
	if collection == "degraded" && r.URL.Query().Get("strict") == "1" {
		status = http.StatusServiceUnavailable
	}
	WriteJsonResponse(w, status, map[string]string{"status": "ok", "collection": collection})
}

// GetCollectionHealth reports, per configured host and per tracked container,
// whether LogDeck can collect logs, plus the log store's write and disk state
// when persistence is enabled.
func (ar *APIRouter) GetCollectionHealth(w http.ResponseWriter, r *http.Request) {
	hosts := ar.registry.Docker().GetHosts()
	names := make([]string, 0, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Name)
	}

	report := ar.health.Report(names)
	if ar.logStore != nil {
		storage := ar.logStore.Health()
		report.Storage = &storage
	}
	WriteJsonResponse(w, http.StatusOK, report)
}

// handleVersion returns the server version. Public, no auth.
func (ar *APIRouter) handleVersion(w http.ResponseWriter, r *http.Request) {
	WriteJsonResponse(w, http.StatusOK, map[string]string{"version": ar.version})
}
