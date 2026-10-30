package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/deployment"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/dokploy"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// limitsStore saves a workload's limits in its deployment platform. Saving
// and deploying are separate, so a save never claims to have applied.
type limitsStore interface {
	ReadLimits(context.Context) (models.ResourceLimits, error)
	SaveLimits(context.Context, models.UpdateResourcesRequest) error
	Deploy(context.Context) error
}

type coolifyLimits struct {
	client   *coolify.Client
	resource *coolify.ResourceInfo
}

func (s coolifyLimits) ReadLimits(ctx context.Context) (models.ResourceLimits, error) {
	return s.client.ReadLimits(ctx, s.resource)
}
func (s coolifyLimits) SaveLimits(ctx context.Context, req models.UpdateResourcesRequest) error {
	return s.client.SaveLimits(ctx, s.resource, req)
}
func (s coolifyLimits) Deploy(ctx context.Context) error { return s.client.Deploy(ctx, s.resource) }

type dokployLimits struct {
	client   *dokploy.Client
	resource dokploy.Resource
}

func (s dokployLimits) ReadLimits(ctx context.Context) (models.ResourceLimits, error) {
	return s.client.ReadLimits(ctx, s.resource)
}
func (s dokployLimits) SaveLimits(ctx context.Context, req models.UpdateResourcesRequest) error {
	return s.client.SaveLimits(ctx, s.resource, req)
}
func (s dokployLimits) Deploy(ctx context.Context) error { return s.client.Deploy(ctx, s.resource) }

// limitsOwner is the platform that recreates a container from saved settings,
// so a live update would last only until its next deployment. Plain Compose
// keeps live updates; the UI points at the Compose file to persist them.
type limitsOwner struct {
	Platform   string                 `json:"platform"`
	Persistent bool                   `json:"persistent"`
	Configured *models.ResourceLimits `json:"configured,omitempty"`
	// Reason says why the limits cannot be saved through LogDeck.
	Reason string `json:"reason,omitempty"`
	URL    string `json:"url,omitempty"`
	store  limitsStore
}

func (o *limitsOwner) live() bool { return o == nil || o.Platform == "compose" }

func (o *limitsOwner) name() string {
	return strings.ToUpper(o.Platform[:1]) + o.Platform[1:]
}

// resourcesOwner finds who owns a container's limits without calling the
// platform: Coolify by its labels, Dokploy by the deployment the user mapped
// in the environment editor, which the platform read then checks.
func (ar *APIRouter) resourcesOwner(r *http.Request, host string, labels map[string]string) *limitsOwner {
	if labels[coolify.LabelManaged] == "true" {
		owner := &limitsOwner{Platform: "coolify", URL: ar.coolifyInstanceURL(host)}
		resource := coolify.ExtractResourceInfo(labels)
		client := ar.registry.Coolify().GetClient(host)
		switch {
		case resource == nil:
			owner.Reason = "Coolify's labels on this container are incomplete. Change its limits in Coolify."
		case resource.Type == coolify.ResourceTypeService:
			owner.Reason = "Coolify services set their limits in the service's Compose file. Change them in Coolify, then restart the service."
		case resource.Type != coolify.ResourceTypeApplication:
			owner.Reason = "Only Coolify application limits can be saved here. Change this resource's limits in Coolify."
		case client == nil:
			owner.Reason = "This container is managed by Coolify. Connect this host's Coolify integration in Settings to save its limits."
		default:
			owner.store = coolifyLimits{client, resource}
		}
		return owner
	}
	// A cold Dokploy name cache is worth a short wait, as in the container list.
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if ar.ownedByDokploy(ctx, host, labels) {
		owner := &limitsOwner{Platform: "dokploy", URL: ar.dokployInstanceURL(host)}
		client := ar.registry.Dokploy().GetClient(host)
		service := labels["com.docker.swarm.service.name"]
		selected := r.URL.Query().Get("resource")
		dokployID := deployment.DokployResourceID(labels)
		switch {
		case client == nil:
			owner.Reason = "This container is managed by Dokploy. Connect its instance and server in Settings to save its limits."
		case service == "" || labels["com.docker.stack.namespace"] != "" || r.URL.Query().Get("resource_type") == string(dokploy.Compose):
			owner.Reason = "Dokploy Compose deployments set their limits in the Compose file. Change them in Dokploy, then deploy."
		case selected == "":
			owner.Reason = "Choose this container's Dokploy deployment in the Environment panel to save its limits in Dokploy."
		case dokployID != "" && dokployID != selected:
			owner.Reason = "Dokploy deployed this container from a different resource. Choose its deployment again in the Environment panel."
		default:
			// An application runs as a Swarm service named after its appName,
			// which the record read checks the mapped deployment against.
			owner.store = dokployLimits{client, dokploy.Resource{Type: dokploy.Application, ID: selected, AppName: service}}
		}
		return owner
	}
	if deployment.SwarmManaged(labels) {
		return &limitsOwner{Platform: "swarm", Reason: "Swarm replaces this container from its service's settings. Change the limits on the service, for example with docker service update."}
	}
	if labels["com.docker.compose.project"] != "" {
		return &limitsOwner{Platform: "compose"}
	}
	return nil
}

func (ar *APIRouter) GetContainerResources(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
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
		resources.NanoCPUs = docker.NanoCPUsFromHostConfig(
			container.HostConfig.NanoCPUs,
			container.HostConfig.CPUQuota,
			container.HostConfig.CPUPeriod,
		)
		resources.RestartPolicy = models.RestartPolicySpec{
			Name:              string(container.HostConfig.RestartPolicy.Name),
			MaximumRetryCount: container.HostConfig.RestartPolicy.MaximumRetryCount,
		}
	}

	var labels map[string]string
	if container.Config != nil {
		labels = container.Config.Labels
	}
	owner := ar.resourcesOwner(r, host, labels)
	if owner != nil && owner.store != nil {
		configured, err := owner.store.ReadLimits(r.Context())
		if err != nil {
			owner.Reason = dokployMessage(err)
		} else {
			owner.Persistent = true
			owner.Configured = &configured
		}
	}

	// The top-level values are what the container runs with; a platform's
	// saved values, which its next deployment applies, sit under owner.
	WriteJsonResponse(w, http.StatusOK, struct {
		models.ContainerResources
		Owner *limitsOwner `json:"owner,omitempty"`
	}{resources, owner})
}

// managedLimitsOwner resolves the owner for a mutating request, answering
// with the reason when a platform owns the limits but LogDeck cannot save them.
func (ar *APIRouter) managedLimitsOwner(w http.ResponseWriter, r *http.Request, host, id string) (*limitsOwner, bool) {
	container, err := ar.registry.Docker().GetContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	var labels map[string]string
	if container.Config != nil {
		labels = container.Config.Labels
	}
	owner := ar.resourcesOwner(r, host, labels)
	if !owner.live() && owner.store == nil {
		http.Error(w, owner.Reason, http.StatusConflict)
		return nil, false
	}
	return owner, true
}

func (ar *APIRouter) UpdateContainerResources(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
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

	// Platform-managed limits never fall back to a live update, which the
	// next deployment would undo.
	owner, ok := ar.managedLimitsOwner(w, r, host, id)
	if !ok {
		return
	}
	if !owner.live() {
		if req.RestartPolicy != nil {
			http.Error(w, owner.name()+" manages this container's restart policy; change it in "+owner.name(), http.StatusBadRequest)
			return
		}
		if err := owner.store.SaveLimits(r.Context(), req); err != nil {
			WriteJsonResponse(w, http.StatusBadGateway, map[string]any{"message": dokployMessage(err), "saved": false, "applied": false, "reload_required": true})
			return
		}
		WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Limits saved in " + owner.name() + ". Deploy to apply them", "saved": true, "applied": false})
		return
	}

	if err := ar.registry.Docker().UpdateContainerResources(r.Context(), host, id, req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container resources updated",
		"applied": true,
	})
}

func (ar *APIRouter) DeployContainerResources(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}
	owner, ok := ar.managedLimitsOwner(w, r, host, id)
	if !ok {
		return
	}
	if owner.live() {
		http.Error(w, "This container has no platform deployment; its limits apply live", http.StatusConflict)
		return
	}
	if err := owner.store.Deploy(r.Context()); err != nil {
		http.Error(w, owner.name()+" deployment request failed; saved limits are unchanged: "+dokployMessage(err), http.StatusBadGateway)
		return
	}
	WriteJsonResponse(w, http.StatusAccepted, map[string]any{"message": owner.name() + " deployment requested. Check " + owner.name() + " for progress", "applied": false})
}

func (ar *APIRouter) coolifyInstanceURL(host string) string {
	for _, h := range ar.registry.Config().CoolifyHosts {
		if h.HostName == host {
			return strings.TrimRight(h.APIURL, "/")
		}
	}
	return ""
}
