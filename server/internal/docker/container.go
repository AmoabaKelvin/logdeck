package docker

import (
	"context"
	"maps"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func (c *MultiHostClient) GetContainer(hostName, id string) (container.InspectResponse, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return container.InspectResponse{}, err
	}
	result, err := apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return container.InspectResponse{}, err
	}
	return result, nil
}

func (c *MultiHostClient) StartContainer(hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerStart(context.Background(), id, container.StartOptions{})
}

func (c *MultiHostClient) StopContainer(hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerStop(context.Background(), id, container.StopOptions{})
}

func (c *MultiHostClient) RestartContainer(hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerRestart(context.Background(), id, container.StopOptions{})
}

func (c *MultiHostClient) RemoveContainer(hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerRemove(context.Background(), id, container.RemoveOptions{})
}

func (c *MultiHostClient) GetEnvVariables(hostName, id string) (map[string]string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	inspect, err := apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	isCoolifyManaged := inspect.Config.Labels[coolify.LabelManaged] == "true"

	envMap := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			if isCoolifyManaged && coolify.IsCoolifyDefaultEnvVar(parts[0]) {
				continue
			}
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap, nil
}

// SetEnvVariables recreates a container with updated environment variables.
// Returns the new container ID and the original container's labels.
func (c *MultiHostClient) SetEnvVariables(hostName, id string, envVariables map[string]string) (string, map[string]string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return "", nil, err
	}

	inspect, err := apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", nil, err
	}

	labels := inspect.Config.Labels
	isCoolifyManaged := labels[coolify.LabelManaged] == "true"

	// Split existing env vars into user-defined and Coolify-injected defaults.
	// Coolify defaults are kept aside so the user cannot accidentally delete or
	// overwrite them — they get merged back unconditionally before recreation.
	envMap := make(map[string]string)
	coolifyDefaults := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			if isCoolifyManaged && coolify.IsCoolifyDefaultEnvVar(parts[0]) {
				coolifyDefaults[parts[0]] = parts[1]
			} else {
				envMap[parts[0]] = parts[1]
			}
		}
	}

	// Delete keys from envMap that are not present in envVariables
	for key := range envMap {
		if _, exists := envVariables[key]; !exists {
			delete(envMap, key)
		}
	}

	// Apply user-supplied values, then merge Coolify defaults back in
	maps.Copy(envMap, envVariables)
	maps.Copy(envMap, coolifyDefaults)

	envs := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envs = append(envs, key+"="+value)
	}

	containerName := strings.TrimPrefix(inspect.Name, "/")

	err = apiClient.ContainerStop(context.Background(), id, container.StopOptions{})
	if err != nil {
		return "", nil, err
	}

	err = apiClient.ContainerRemove(context.Background(), id, container.RemoveOptions{})
	if err != nil {
		return "", nil, err
	}

	newConfig := inspect.Config
	newConfig.Env = envs

	resp, err := apiClient.ContainerCreate(
		context.Background(),
		newConfig,
		inspect.HostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		},
		nil,
		containerName,
	)
	if err != nil {
		return "", nil, err
	}

	err = apiClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
	if err != nil {
		return "", nil, err
	}

	return resp.ID, labels, nil
}
