package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/dokploy"
	"github.com/docker/docker/api/types/swarm"
)

// Rollout phases. Dokploy reports a deployment done once it has asked Swarm
// to update the service, before the new task is healthy, and Swarm may still
// roll it back. Only "completed" means the saved configuration is running.
const (
	rolloutQueued      = "queued"
	rolloutBuilding    = "building"
	rolloutFailed      = "failed"
	rolloutUpdating    = "updating"
	rolloutCompleted   = "completed"
	rolloutRollingBack = "rolling_back"
	rolloutRolledBack  = "rolled_back"
	rolloutPaused      = "paused"
)

// unchangedGrace is how long after Dokploy finishes a service may go without
// an update before it counts as already matching. Swarm starts the update of
// a changed service at once; an unchanged stack service never gets one. It
// also absorbs ordinary clock skew between LogDeck and Dokploy.
const unchangedGrace = time.Minute

type rolloutStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message,omitempty"`
	// ContainerID is the container now running the workload, once completed.
	ContainerID string `json:"container_id,omitempty"`
}

// rolloutPhase combines Dokploy's deployment with the Swarm update it caused.
// Every service update clears the service's update status, and Dokploy marks
// a deployment done only after its update call returned, so once it is done
// the status belongs to this deployment or a later one: nil means Swarm has
// not started on it yet, or has nothing to change. Without swarmService,
// Dokploy's result is final.
func rolloutPhase(deployment *dokploy.Deployment, swarmService bool, update *swarm.UpdateStatus, now time.Time) rolloutStatus {
	switch {
	case deployment == nil:
		return rolloutStatus{Phase: rolloutQueued}
	case deployment.Status == dokploy.DeploymentError:
		return rolloutStatus{Phase: rolloutFailed, Message: deployment.ErrorMessage}
	case deployment.Status == dokploy.DeploymentCancelled:
		return rolloutStatus{Phase: rolloutFailed, Message: "The deployment was cancelled in Dokploy"}
	case deployment.Status != dokploy.DeploymentDone:
		// Running, or a state a newer Dokploy adds; it has not ended.
		return rolloutStatus{Phase: rolloutBuilding}
	case !swarmService:
		return rolloutStatus{Phase: rolloutCompleted}
	case update == nil:
		if !deployment.FinishedAt.IsZero() && now.Sub(deployment.FinishedAt) > unchangedGrace {
			return rolloutStatus{Phase: rolloutCompleted, Message: "Swarm had nothing to change"}
		}
		return rolloutStatus{Phase: rolloutUpdating}
	}
	status := rolloutStatus{Message: update.Message}
	switch update.State {
	case swarm.UpdateStateCompleted:
		status.Phase = rolloutCompleted
	case swarm.UpdateStateRollbackStarted:
		status.Phase = rolloutRollingBack
	case swarm.UpdateStateRollbackCompleted:
		status.Phase = rolloutRolledBack
	case swarm.UpdateStatePaused, swarm.UpdateStateRollbackPaused:
		status.Phase = rolloutPaused
	default:
		status.Phase = rolloutUpdating
	}
	return status
}

// rolloutWorkload is the Swarm service a deployment updates, if any, and the
// labels of the containers it runs. An application is one service named after
// its appName. A Compose deployment's service comes from the caller and must
// belong to it: a project service, or a stack service named <appName>_<name>.
func rolloutWorkload(resourceType dokploy.ResourceType, appName, service string) (swarmService string, labels map[string]string) {
	switch {
	case resourceType == dokploy.Application:
		return appName, map[string]string{"com.docker.swarm.service.name": appName}
	case service == "":
		return "", nil
	case strings.HasPrefix(service, appName+"_"):
		return service, map[string]string{"com.docker.stack.namespace": appName, "com.docker.swarm.service.name": service}
	default:
		return "", map[string]string{"com.docker.compose.project": appName, "com.docker.compose.service": service}
	}
}

// GetDokployRollout handles GET /api/v1/dokploy/rollout. It follows the
// deployments that started after "after", the start of the newest deployment
// before the request, without needing the container the request came from,
// which the deployment replaces.
func (ar *APIRouter) GetDokployRollout(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	host := query.Get("host")
	client := ar.registry.Dokploy().GetClient(host)
	if client == nil {
		http.Error(w, "No Dokploy integration is configured for this host", http.StatusConflict)
		return
	}
	var after time.Time
	if raw := query.Get("after"); raw != "" {
		var err error
		if after, err = time.Parse(time.RFC3339Nano, raw); err != nil {
			http.Error(w, "after must be the deploy response's after_deployment", http.StatusBadRequest)
			return
		}
	}
	resource := dokploy.Resource{Type: dokploy.ResourceType(query.Get("resource_type")), ID: query.Get("resource"), MappingRevision: query.Get("mapping_revision")}
	appName, deployment, err := client.LatestDeployment(r.Context(), resource, after)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, dokploy.ErrMappingChanged) {
			status = http.StatusConflict
		}
		http.Error(w, dokployMessage(err), status)
		return
	}
	swarmService, labels := rolloutWorkload(resource.Type, appName, query.Get("service"))
	var update *swarm.UpdateStatus
	if swarmService != "" && deployment != nil && deployment.Status == dokploy.DeploymentDone {
		update, err = ar.registry.Docker().ServiceUpdateStatus(r.Context(), host, swarmService)
		if err != nil {
			http.Error(w, "Dokploy finished, but Swarm service "+swarmService+" could not be read: "+err.Error(), http.StatusBadGateway)
			return
		}
	}
	status := rolloutPhase(deployment, swarmService != "", update, time.Now())
	if status.Phase == rolloutCompleted && labels != nil {
		status.ContainerID, err = ar.registry.Docker().NewestRunningContainer(r.Context(), host, labels)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// A service is done only once its new task's container runs here.
		if status.ContainerID == "" && swarmService != "" {
			status = rolloutStatus{Phase: rolloutUpdating}
		}
	}
	WriteJsonResponse(w, http.StatusOK, status)
}
