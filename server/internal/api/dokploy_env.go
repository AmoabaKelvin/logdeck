package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/deployment"
	"github.com/AmoabaKelvin/logdeck/internal/dokploy"
	"github.com/docker/docker/api/types/container"
)

// handleDokployEnvironment returns false only for containers whose environment
// should follow the existing Docker or Coolify path.
func (ar *APIRouter) handleDokployEnvironment(w http.ResponseWriter, r *http.Request, host, id string, inspect container.InspectResponse) bool {
	labels := inspect.Config.Labels
	if labels[coolify.LabelManaged] == "true" {
		return false
	}
	client := ar.registry.Dokploy().GetClient(host)
	hinted := deployment.DokployHint(labels)
	swarm := deployment.SwarmManaged(labels)
	compose := labels["com.docker.compose.project"] != ""
	selected := r.URL.Query().Get("resource")
	direct := r.URL.Query().Get("platform") == "docker" && r.URL.Query().Get("confirmed") == "true"
	if direct && !hinted && !swarm {
		return false
	}
	if client == nil {
		if hinted {
			http.Error(w, "This container is managed by Dokploy. Connect its instance and server in Settings to edit the saved environment", 409)
			return true
		}
		if swarm {
			http.Error(w, "This container belongs to a Swarm service. Connect its deployment platform to edit the saved environment", 409)
			return true
		}
		if selected != "" {
			http.Error(w, "No Dokploy integration is configured for this host", 409)
			return true
		}
		return false
	}
	// Standalone containers on a connected host retain direct editing. Known
	// Compose/Swarm workloads require an explicit platform mapping instead.
	if !hinted && !swarm && !compose && selected == "" {
		return false
	}
	for _, entry := range inspect.Config.Env {
		if strings.HasPrefix(entry, "DOKPLOY_DEPLOY_URL=") {
			http.Error(w, "Dokploy preview environments must be edited and deployed in Dokploy", 409)
			return true
		}
	}
	resources, err := client.Resources(r.Context())
	if err != nil {
		if hinted || swarm {
			http.Error(w, err.Error()+". Direct container recreation is unavailable for this workload", 502)
			return true
		}
		// Ownership cannot be checked, but nothing marks this Compose workload
		// as Dokploy's, so the caller may still choose the runtime editor.
		if selected == "" && r.Method == http.MethodGet {
			WriteJsonResponse(w, 200, map[string]any{"source": "dokploy", "env": map[string]string{}, "resources": []dokploy.Resource{}, "mapping_required": true, "plain_compose_allowed": true, "inventory_error": err.Error(), "instance_url": ar.dokployInstanceURL(host), "workload": inspect.Name})
			return true
		}
		http.Error(w, err.Error(), 502)
		return true
	}
	for _, res := range resources {
		for _, preview := range res.PreviewNames {
			if preview != "" && (preview == labels["com.docker.swarm.service.name"] || preview == labels["com.docker.compose.project"]) {
				http.Error(w, "Dokploy preview environments are unsupported here. Edit and deploy the preview in Dokploy; production configuration will not be changed", 409)
				return true
			}
		}
	}
	if selected == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "Confirm the owning Dokploy application or Compose deployment before editing", 409)
			return true
		}
		suggested := make([]string, 0)
		for _, res := range resources {
			if res.AppName != "" && (res.AppName == labels["com.docker.compose.project"] || res.AppName == labels["com.docker.swarm.service.name"] || res.AppName == labels["com.docker.stack.namespace"]) {
				suggested = append(suggested, string(res.Type)+"/"+res.ID)
			}
		}
		WriteJsonResponse(w, 200, map[string]any{"source": "dokploy", "env": map[string]string{}, "suggested_resources": suggested, "resources": resources, "mapping_required": true, "plain_compose_allowed": !hinted && !swarm, "instance_url": ar.dokployInstanceURL(host), "workload": inspect.Name})
		return true
	}
	var owner *dokploy.Resource
	for i := range resources {
		if resources[i].ID == selected && string(resources[i].Type) == r.URL.Query().Get("resource_type") {
			owner = &resources[i]
			break
		}
	}
	if owner == nil {
		http.Error(w, "The selected Dokploy resource is unavailable on this server. Choose the deployment again", 409)
		return true
	}
	if r.URL.Query().Get("mapping_revision") != owner.MappingRevision {
		http.Error(w, "The Dokploy connection or deployment name changed. Choose the deployment again", 409)
		return true
	}
	if r.URL.Query().Get("confirmed") != "true" {
		http.Error(w, "Explicit confirmation of this container's Dokploy mapping is required", 409)
		return true
	}
	var store dokploy.EnvironmentStore = client
	if r.Method == http.MethodGet {
		env, err := store.ReadEnvironment(r.Context(), *owner)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return true
		}
		WriteJsonResponse(w, 200, map[string]any{"source": "dokploy", "env": map[string]string{}, "resource": owner, "configuration": env, "instance_url": ar.dokployInstanceURL(host)})
		return true
	}
	if r.Method == http.MethodPut {
		var req struct {
			Text     *string `json:"text"`
			Revision string  `json:"revision"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == nil {
			http.Error(w, "Saved environment text and revision are required", 400)
			return true
		}
		env, err := store.SaveEnvironment(r.Context(), *owner, *req.Text, req.Revision)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, dokploy.ErrStaleEnvironment) {
				status = http.StatusConflict
			}
			WriteJsonResponse(w, status, map[string]any{"message": err.Error(), "saved": nil, "applied": false, "reload_required": true})
			return true
		}
		WriteJsonResponse(w, 200, map[string]any{"saved": true, "applied": false, "configuration": env, "message": "Environment saved in Dokploy. Deploy separately to apply it"})
		return true
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/deploy") {
		if err := store.Deploy(r.Context(), *owner); err != nil {
			http.Error(w, err.Error()+". Saved configuration remains in Dokploy; check deployment progress before retrying", 502)
			return true
		}
		WriteJsonResponse(w, http.StatusAccepted, map[string]any{"message": "Dokploy deployment requested. Check Dokploy for progress", "applied": false})
		return true
	}
	http.Error(w, "Unsupported Dokploy environment operation", 405)
	return true
}
func (ar *APIRouter) dokployInstanceURL(host string) string {
	for _, h := range ar.registry.Config().DokployHosts {
		if h.HostName == host {
			return h.APIURL
		}
	}
	return ""
}
