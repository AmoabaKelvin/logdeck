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
			apiClient, err = client.NewClientWithOpts(
				client.WithHost(host.Host),
				client.WithAPIVersionNegotiation(),
				client.FromEnv,
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
