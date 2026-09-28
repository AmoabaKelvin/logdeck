package docker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	// stops holds when LogDeck last stopped a container, by host|id.
	stops sync.Map
}

func NewMultiHostClient(hosts []config.DockerHost) (*MultiHostClient, error) {
	clients := make(map[string]*client.Client)

	for _, host := range hosts {
		var (
			apiClient *client.Client
			err       error
		)

		if strings.HasPrefix(host.Host, "ssh://") {
			helper, helperErr := connhelper.GetConnectionHelperWithSSHOpts(host.Host, sshFlags())
			if helperErr != nil {
				return nil, fmt.Errorf("failed to setup SSH helper for host %s (%s): %w", host.Name, host.Host, helperErr)
			}
			dial := serialSSHDialer(helper.Dialer)

			// No http.Client.Timeout: it would also cut off followed log,
			// stats, and event streams.
			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext:           dial,
					MaxIdleConnsPerHost:   32,
					ResponseHeaderTimeout: 10 * time.Second,
				},
			}

			apiClient, err = client.NewClientWithOpts(
				client.WithHTTPClient(httpClient),
				client.WithHost(helper.Host),
				client.WithDialContext(dial),
				client.WithAPIVersionNegotiation(),
			)
		} else {
			// FromEnv comes first so DOCKER_HOST cannot override a configured
			// host: it applies WithHostFromEnv, and the last option wins. With
			// the order reversed, a single DOCKER_HOST would silently collapse
			// every configured host onto one socket. The TLS and API-version
			// parts of FromEnv still apply.
			opts := []client.Opt{
				client.FromEnv,
				client.WithHost(host.Host),
				client.WithAPIVersionNegotiation(),
			}
			// The client negotiates its API version under a lock that ignores
			// request contexts, so an unreachable tcp host would hold every
			// request to it for the OS connect timeout (~75s on macOS).
			if strings.HasPrefix(host.Host, "tcp://") {
				dialer := &net.Dialer{Timeout: 2 * time.Second}
				opts = append(opts, client.WithDialContext(dialer.DialContext))
			}
			apiClient, err = client.NewClientWithOpts(opts...)
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

// sshFlags reuses a master connection where the server's session limit allows.
// serialSSHDialer prevents concurrent handshakes when no master is available.
func sshFlags() []string {
	return []string{
		"-o ConnectTimeout=10",
		"-o ControlMaster=auto",
		"-o ControlPath=" + filepath.Join(os.TempDir(), "logdeck-ssh-%C"),
		"-o ControlPersist=10m",
	}
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

// hostWait bounds how long a call across all hosts waits for any one of them.
// The Docker client negotiates its API version under a lock that ignores
// contexts, so a request to an unreachable host can outlive its own deadline.
var hostWait = 5 * time.Second

type hostResult[T any] struct {
	host string
	val  T
	err  error
}

// eachHost calls fn for every host in parallel and returns the results by host,
// with an error for each host that failed or did not answer within hostWait.
// A late answer is dropped.
func eachHost[T any](ctx context.Context, c *MultiHostClient, fn func(ctx context.Context, name string, cl *client.Client) (T, error)) (map[string]T, []HostError) {
	ctx, cancel := context.WithTimeout(ctx, hostWait)
	defer cancel()

	ch := make(chan hostResult[T], len(c.clients))
	pending := make(map[string]bool, len(c.clients))
	for name, cl := range c.clients {
		pending[name] = true
		go func() {
			val, err := fn(ctx, name, cl)
			ch <- hostResult[T]{host: name, val: val, err: err}
		}()
	}

	results := make(map[string]T, len(c.clients))
	var hostErrors []HostError
	for len(pending) > 0 {
		select {
		case r := <-ch:
			delete(pending, r.host)
			if r.err != nil {
				hostErrors = append(hostErrors, HostError{HostName: r.host, Err: r.err})
				continue
			}
			results[r.host] = r.val
		case <-ctx.Done():
			for name := range pending {
				hostErrors = append(hostErrors, HostError{HostName: name, Err: fmt.Errorf("no answer within %s", hostWait)})
			}
			return results, hostErrors
		}
	}
	return results, hostErrors
}

func (c *MultiHostClient) ListContainersAllHosts(ctx context.Context) (map[string][]models.ContainerInfo, []HostError, error) {
	result, hostErrors := eachHost(ctx, c, func(ctx context.Context, name string, cl *client.Client) ([]models.ContainerInfo, error) {
		containers, err := cl.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return nil, err
		}
		labelPodStacks(ctx, cl, containers)

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
				Ports:   publishedPorts(ctr.Ports),
			})
		}
		return hostContainers, nil
	})
	return result, hostErrors, nil
}

// Docker reports one entry per IP binding, so a port published on both IPv4 and
// IPv6 arrives twice. Deduplicate, drop the unpublished ones, and sort so the
// order is stable between polls.
func publishedPorts(ports []container.Port) []models.ContainerPort {
	seen := make(map[models.ContainerPort]struct{}, len(ports))
	mapped := make([]models.ContainerPort, 0, len(ports))

	for _, port := range ports {
		if port.PublicPort == 0 {
			continue
		}
		entry := models.ContainerPort{
			PublicPort:  port.PublicPort,
			PrivatePort: port.PrivatePort,
			Type:        port.Type,
		}
		if _, duplicate := seen[entry]; duplicate {
			continue
		}
		seen[entry] = struct{}{}
		mapped = append(mapped, entry)
	}

	sort.Slice(mapped, func(i, j int) bool {
		if mapped[i].PublicPort != mapped[j].PublicPort {
			return mapped[i].PublicPort < mapped[j].PublicPort
		}
		return mapped[i].PrivatePort < mapped[j].PrivatePort
	})
	return mapped
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

func (c *MultiHostClient) Close() {
	for _, cl := range c.clients {
		cl.Close()
	}
}
