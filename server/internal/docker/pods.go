package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// StackLabel groups Quadlet containers the way a compose project does.
const StackLabel = "io.logdeck.stack"

// Podman's compat list marks pod members with this annotation but leaves out
// the pod name, which only its libpod API reports. Podman before 6.0 omits
// HostConfig from the list, so a systemd unit label also counts as a hint.
const podSandboxAnnotation = "io.kubernetes.cri-o.SandboxID"

// labelPodStacks sets StackLabel to the pod name on pod members. Docker hosts
// never make the extra call.
func labelPodStacks(ctx context.Context, cl *client.Client, containers []container.Summary) {
	mayHavePods := slices.ContainsFunc(containers, func(ctr container.Summary) bool {
		return ctr.HostConfig.Annotations[podSandboxAnnotation] != "" || ctr.Labels[systemdUnitLabel] != ""
	})
	if !mayHavePods {
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

// libpodGet decodes a GET from Podman's libpod API into out. Docker answers
// these paths with 404.
func libpodGet(ctx context.Context, cl *client.Client, path string, out any) error {
	// The transport dials the socket or SSH tunnel itself, so the URL host
	// only matters for tcp:// daemons. client.FromEnv turns on TLS exactly
	// when DOCKER_CERT_PATH is set.
	scheme, host := "http", "podman"
	if u, err := url.Parse(cl.DaemonHost()); err == nil && u.Scheme == "tcp" {
		host = u.Host
		if os.Getenv("DOCKER_CERT_PATH") != "" {
			scheme = "https"
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/v4.0.0/libpod"+path, nil)
	if err != nil {
		return err
	}
	resp, err := cl.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("libpod %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// podNames maps container IDs to pod names through Podman's libpod API.
func podNames(ctx context.Context, cl *client.Client) (map[string]string, error) {
	var list []struct {
		ID      string `json:"Id"`
		PodName string `json:"PodName"`
	}
	if err := libpodGet(ctx, cl, "/containers/json?all=true", &list); err != nil {
		return nil, err
	}
	pods := make(map[string]string, len(list))
	for _, ctr := range list {
		pods[ctr.ID] = ctr.PodName
	}
	return pods, nil
}
