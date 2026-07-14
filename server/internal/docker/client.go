package docker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type MultiHostClient struct {
	clients map[string]*client.Client
	hosts   []config.DockerHost
}

func NewMultiHostClient(hosts []config.DockerHost) (*MultiHostClient, error) {
	clients := make(map[string]*client.Client)

	for _, host := range hosts {
		var (
			apiClient *client.Client
			err       error
		)

		if strings.HasPrefix(host.Host, "ssh://") {
			helper, helperErr := connhelper.GetConnectionHelper(host.Host)
			if helperErr != nil {
				return nil, fmt.Errorf("failed to setup SSH helper for host %s (%s): %w", host.Name, host.Host, helperErr)
			}

			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext: helper.Dialer,
				},
				Timeout: 10 * time.Second,
			}

			apiClient, err = client.NewClientWithOpts(
				client.WithHTTPClient(httpClient),
				client.WithHost(helper.Host),
				client.WithDialContext(helper.Dialer),
				client.WithAPIVersionNegotiation(),
			)
		} else {
			// FromEnv comes first so DOCKER_HOST cannot override a configured
			// host: it applies WithHostFromEnv, and the last option wins. With
			// the order reversed, a single DOCKER_HOST would silently collapse
			// every configured host onto one socket. The TLS and API-version
			// parts of FromEnv still apply.
			apiClient, err = client.NewClientWithOpts(
				client.FromEnv,
				client.WithHost(host.Host),
				client.WithAPIVersionNegotiation(),
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to connect to host %s (%s): %w", host.Name, host.Host, err)
		}
		clients[host.Name] = apiClient
	}

	return &MultiHostClient{
		clients: clients,
		hosts:   hosts,
	}, nil
}

type HostError struct {
	HostName string
	Err      error
}

// healthFromStatus extracts the healthcheck state from a container list Status
// string. Docker embeds "(healthy)", "(unhealthy)", or "(health: starting)" in
// the list Status string, e.g. "Up 3 hours (healthy)"; Podman's Docker-compat
// list API does not, so on Podman the field is absent. Returns "" when no
// health suffix is present.
func healthFromStatus(status string) string {
	switch {
	case strings.HasSuffix(status, "(healthy)"):
		return "healthy"
	case strings.HasSuffix(status, "(unhealthy)"):
		return "unhealthy"
	case strings.HasSuffix(status, "(health: starting)"):
		return "starting"
	default:
		return ""
	}
}

func (c *MultiHostClient) ListContainersAllHosts(ctx context.Context) (map[string][]models.ContainerInfo, []HostError, error) {
	result := make(map[string][]models.ContainerInfo)
	var hostErrors []HostError
	var mu sync.Mutex
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()

			containers, err := cl.ContainerList(ctx, container.ListOptions{All: true})
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				hostErrors = append(hostErrors, HostError{HostName: name, Err: err})
				return
			}

			hostContainers := make([]models.ContainerInfo, 0, len(containers))
			for _, ctr := range containers {
				hostContainers = append(hostContainers, models.ContainerInfo{
					ID:      ctr.ID,
					Names:   ctr.Names,
					Image:   ctr.Image,
					ImageID: ctr.ImageID,
					Command: ctr.Command,
					Created: ctr.Created,
					State:   ctr.State,
					Status:  ctr.Status,
					Health:  healthFromStatus(ctr.Status),
					Labels:  ctr.Labels,
					Host:    name,
				})
			}
			result[name] = hostContainers
		}(hostName, apiClient)
	}

	wg.Wait()
	return result, hostErrors, nil
}

func (c *MultiHostClient) GetClient(hostName string) (*client.Client, error) {
	apiClient, ok := c.clients[hostName]
	if !ok {
		return nil, fmt.Errorf("host %s not found", hostName)
	}
	return apiClient, nil
}

func (c *MultiHostClient) GetHosts() []config.DockerHost {
	return c.hosts
}

// EngineInfo identifies the container engine behind a host. Podman serves the
// Docker-compatible API and reports itself as a "Podman Engine" component in
// the version response; anything else is treated as Docker.
func (c *MultiHostClient) EngineInfo(ctx context.Context, hostName string) (engine, version string, err error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return "", "", err
	}

	v, err := apiClient.ServerVersion(ctx)
	if err != nil {
		return "", "", err
	}

	engine = "Docker"
	for _, component := range v.Components {
		if strings.Contains(component.Name, "Podman") {
			engine = "Podman"
			break
		}
	}
	return engine, v.Version, nil
}

// Close closes all underlying Docker API clients.
func (c *MultiHostClient) Close() {
	for _, cl := range c.clients {
		cl.Close()
	}
}
