package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
)

// Resolve platform ownership before choosing an editor. Managed workloads
// never fall back to direct recreation when their integration fails.
func (ar *APIRouter) environmentOwner(w http.ResponseWriter, r *http.Request, host, id string) (*coolify.Client, *coolify.ResourceInfo, bool) {
	inspect, err := ar.registry.Docker().GetContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, nil, false
	}
	if inspect.Config == nil {
		http.Error(w, "Container configuration is unavailable", http.StatusConflict)
		return nil, nil, false
	}
	if ar.handleDokployEnvironment(w, r, host, id, inspect) {
		return nil, nil, false
	}
	if inspect.Config.Labels[coolify.LabelManaged] != "true" {
		return nil, nil, true
	}
	resource := coolify.ExtractResourceInfo(inspect.Config.Labels)
	if err := resource.ValidateEnvSupport(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return nil, nil, false
	}
	client := ar.registry.Coolify().GetClient(host)
	if client == nil {
		http.Error(w, "This container is managed by Coolify. Connect this host's Coolify integration in Settings to edit its saved environment", http.StatusConflict)
		return nil, nil, false
	}
	return client, resource, true
}

func (ar *APIRouter) GetEnvVariables(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	client, resource, ok := ar.environmentOwner(w, r, host, id)
	if !ok {
		return
	}
	if client != nil {
		vars, err := client.ListEnvVars(r.Context(), resource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// Keep the legacy map for CLI readers. Unknown and preview values are
		// represented only in the records, never invented as empty strings.
		env := make(map[string]string)
		for _, v := range vars {
			if !v.IsPreview && v.Value != nil {
				env[v.Key] = *v.Value
			}
		}
		WriteJsonResponse(w, http.StatusOK, map[string]any{"source": "coolify", "env": env, "variables": vars, "resource_type": resource.Type})
		return
	}
	env, err := ar.registry.Docker().GetEnvVariables(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Tells the UI this runtime editor was chosen over a Dokploy mapping, which
	// only means something while the host still has a Dokploy connection.
	plainCompose := r.URL.Query().Get("platform") == "docker" && ar.registry.Dokploy().GetClient(host) != nil
	WriteJsonResponse(w, http.StatusOK, map[string]any{"source": "docker", "env": env, "plain_compose": plainCompose})
}

var envKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\-\.]*$`)

func (ar *APIRouter) UpdateEnvVariables(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	client, resource, ok := ar.environmentOwner(w, r, host, id)
	if !ok {
		return
	}
	var req struct {
		Env     map[string]string   `json:"env"`
		Changes []coolify.EnvChange `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if client != nil {
		if req.Env != nil || req.Changes == nil {
			http.Error(w, "Coolify requires explicit variable changes, not a replacement runtime environment; read the saved variables first", http.StatusBadRequest)
			return
		}
		completed, err := client.SaveEnvChanges(r.Context(), resource, req.Changes)
		if err != nil {
			WriteJsonResponse(w, http.StatusBadGateway, map[string]any{"message": err.Error(), "saved": false, "completed": completed, "reload_required": true})
			return
		}
		WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Environment saved in Coolify. Deploy to apply the changes", "saved": true, "applied": false, "completed": completed})
		return
	}
	if req.Env == nil || req.Changes != nil {
		http.Error(w, "env is required for direct container editing", http.StatusBadRequest)
		return
	}
	for key := range req.Env {
		if !envKeyRegex.MatchString(key) {
			http.Error(w, fmt.Sprintf("invalid environment variable key: %s", key), http.StatusBadRequest)
			return
		}
	}
	newID, _, err := ar.registry.Docker().SetEnvVariables(r.Context(), host, id, req.Env)
	if err != nil {
		http.Error(w, err.Error(), actionErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Environment variables updated", "new_container_id": newID})
}

func (ar *APIRouter) DeployEnvironment(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	client, resource, ok := ar.environmentOwner(w, r, host, id)
	if !ok {
		return
	}
	if client == nil {
		http.Error(w, "This container has no Coolify deployment", http.StatusConflict)
		return
	}
	if err := client.Deploy(r.Context(), resource); err != nil {
		http.Error(w, "Coolify deployment request failed; saved configuration is unchanged: "+err.Error(), http.StatusBadGateway)
		return
	}
	WriteJsonResponse(w, http.StatusAccepted, map[string]any{"message": "Coolify deployment requested. Check Coolify for progress", "applied": false})
}
