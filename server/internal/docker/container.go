package docker

import (
	"context"
	"maps"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// container life cycle methods, start, stop, restart, remove

func (c *Client) StartContainer(id string) error {
	err := c.apiClient.ContainerStart(context.Background(), id, container.StartOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) StopContainer(id string) error {
	err := c.apiClient.ContainerStop(context.Background(), id, container.StopOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RestartContainer(id string) error {
	err := c.apiClient.ContainerRestart(context.Background(), id, container.StopOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RemoveContainer(id string) error {
	err := c.apiClient.ContainerRemove(context.Background(), id, container.RemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}

// GetEnvVariables returns the environment variables for a container
func (c *Client) GetEnvVariables(id string) (map[string]string, error) {
	inspect, err := c.apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	envMap := make(map[string]string)
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap, nil
}

func (c *Client) SetEnvVariables(id string, envVariables map[string]string) (string, error) {
	// we cannot directly update the env variables of a container, we have to first get the
	// previous commands and things used to create the container, then stop it.
	// create a new container with the same configuration but updated env vars and then start it.
	inspect, err := c.apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", err
	}

	envMap := make(map[string]string)
	// First, load all existing env vars from the container config
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	maps.Copy(envMap, envVariables)

	envs := make([]string, 0, len(envMap))
	for key, value := range envMap {
		envs = append(envs, key+"="+value)
	}

	containerName := inspect.Name
	containerName = strings.TrimPrefix(containerName, "/")

	err = c.apiClient.ContainerStop(context.Background(), id, container.StopOptions{})
	if err != nil {
		return "", err
	}

	err = c.apiClient.ContainerRemove(context.Background(), id, container.RemoveOptions{})
	if err != nil {
		return "", err
	}

	newConfig := inspect.Config
	newConfig.Env = envs

	resp, err := c.apiClient.ContainerCreate(
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
		return "", err
	}

	err = c.apiClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}
