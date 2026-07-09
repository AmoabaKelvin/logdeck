package api

import (
	"context"
	"net/http"
	"time"
)

// GetHostsStats returns engine-level info for every configured Docker host.
// Unreachable hosts are included with available=false instead of failing the
// whole response.
func (ar *APIRouter) GetHostsStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hosts := ar.registry.Docker().GetHostsInfo(ctx)

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"hosts": hosts,
	})
}
