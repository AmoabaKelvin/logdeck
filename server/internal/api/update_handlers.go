package api

import (
	"encoding/json"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/go-chi/chi/v5"
)

func (ar *APIRouter) GetContainerResources(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host := r.URL.Query().Get("host")

	if host == "" {
		http.Error(w, "host parameter is required", http.StatusBadRequest)
		return
	}

	container, err := ar.registry.Docker().GetContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resources := models.ContainerResources{}
	if container.HostConfig != nil {
		resources.MemoryBytes = container.HostConfig.Memory
		resources.NanoCPUs = container.HostConfig.NanoCPUs
		resources.RestartPolicy = models.RestartPolicySpec{
			Name:              string(container.HostConfig.RestartPolicy.Name),
			MaximumRetryCount: container.HostConfig.RestartPolicy.MaximumRetryCount,
		}
	}

	WriteJsonResponse(w, http.StatusOK, resources)
}

func (ar *APIRouter) UpdateContainerResources(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host := r.URL.Query().Get("host")

	if host == "" {
		http.Error(w, "host parameter is required", http.StatusBadRequest)
		return
	}

	var req models.UpdateResourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ar.registry.Docker().UpdateContainerResources(r.Context(), host, id, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container resources updated",
	})
}
