package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// StackLabel groups Quadlet containers the way a compose project does.
const StackLabel = "io.logdeck.stack"

// Podman's compat list marks pod members with this annotation but leaves out
// the pod name, which only its libpod API reports.
const podSandboxAnnotation = "io.kubernetes.cri-o.SandboxID"

// labelPodStacks sets StackLabel to the pod name on pod members. Hosts without
// pods (every Docker host) never make the extra call.
func labelPodStacks(ctx context.Context, cl *client.Client, containers []container.Summary) {
	hasPods := false
	for _, ctr := range containers {
		if ctr.HostConfig.Annotations[podSandboxAnnotation] != "" {
			hasPods = true
			break
		}
	}
	if !hasPods {
		return
	}

	pods, err := podNames(ctx, cl)
	if err != nil {
		log.Printf("docker: listing pod names failed: %v", err)
		return
	}
	for i, ctr := range containers {
		pod := pods[ctr.ID]
		if pod == "" || ComposeProject(ctr.Labels) != "" {
			continue
		}
		if ctr.Labels == nil {
			containers[i].Labels = map[string]string{}
		}
		containers[i].Labels[StackLabel] = pod
	}
}

// podNames maps container IDs to pod names through Podman's libpod API.
func podNames(ctx context.Context, cl *client.Client) (map[string]string, error) {
	// The transport dials the socket or SSH tunnel itself, so the URL host
	// only matters for tcp:// daemons.
	scheme, host := "http", "podman"
	if u, err := url.Parse(cl.DaemonHost()); err == nil && u.Scheme == "tcp" {
		host = u.Host
		if t, ok := cl.HTTPClient().Transport.(*http.Transport); ok && t.TLSClientConfig != nil {
			scheme = "https"
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/v4.0.0/libpod/containers/json?all=true", nil)
	if err != nil {
		return nil, err
	}
	resp, err := cl.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libpod container list: %s", resp.Status)
	}

	var list []struct {
		ID      string `json:"Id"`
		PodName string `json:"PodName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	pods := make(map[string]string, len(list))
	for _, ctr := range list {
		pods[ctr.ID] = ctr.PodName
	}
	return pods, nil
}
