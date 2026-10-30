package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

// CheckImageUpdate asks the registry about one container's image now, for
// when the cached answer in the container list is too old to trust.
func (ar *APIRouter) CheckImageUpdate(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	update, err := ar.registry.Docker().CheckImageUpdate(ctx, host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"image_update": update})
}

// UpdateContainerImage pulls and recreates a standalone container. Platform
// and Compose containers are refused with how to update them instead.
func (ar *APIRouter) UpdateContainerImage(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	newID, err := ar.registry.Docker().UpdateContainerImage(r.Context(), host, id)
	if err != nil {
		status := http.StatusInternalServerError
		var refused *docker.ImageUpdateRefusedError
		var systemd *docker.SystemdManagedError
		var pull *docker.ImagePullError
		switch {
		case errors.As(err, &refused), errors.As(err, &systemd):
			status = http.StatusConflict
		case errors.As(err, &pull):
			status = http.StatusBadGateway
		}
		http.Error(w, dokployMessage(err), status)
		return
	}
	if newID == "" {
		WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "The container already runs the newest image", "updated": false})
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Image updated and container recreated", "updated": true, "new_container_id": newID})
}
