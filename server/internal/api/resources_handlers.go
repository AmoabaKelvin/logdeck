package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

func hostErrorMessages(hostErrors []docker.HostError) []map[string]string {
	messages := make([]map[string]string, 0, len(hostErrors))
	for _, he := range hostErrors {
		messages = append(messages, map[string]string{
			"host":    he.HostName,
			"message": he.Err.Error(),
		})
	}
	return messages
}

func (ar *APIRouter) GetImages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	images, hostErrors := ar.registry.Docker().ListImagesAllHosts(ctx)
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created > images[j].Created
	})

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"images":     images,
		"hostErrors": hostErrorMessages(hostErrors),
	})
}

func (ar *APIRouter) GetVolumes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	volumes, hostErrors := ar.registry.Docker().ListVolumesAllHosts(ctx)
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].Name < volumes[j].Name
	})

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"volumes":    volumes,
		"hostErrors": hostErrorMessages(hostErrors),
	})
}

func (ar *APIRouter) GetNetworks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	networks, hostErrors := ar.registry.Docker().ListNetworksAllHosts(ctx)
	sort.Slice(networks, func(i, j int) bool {
		return networks[i].Name < networks[j].Name
	})

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"networks":   networks,
		"hostErrors": hostErrorMessages(hostErrors),
	})
}
