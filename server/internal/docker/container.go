package docker

import (
	"context"
	"log"
	"maps"
	"strings"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (c *MultiHostClient) GetContainer(ctx context.Context, hostName, id string) (container.InspectResponse, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return container.InspectResponse{}, err
	}
	result, err := apiClient.ContainerInspect(ctx, id)
	if err != nil {
		return container.InspectResponse{}, err
	}
	return result, nil
}

func (c *MultiHostClient) StartContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *MultiHostClient) StopContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerStop(ctx, id, container.StopOptions{})
}

func (c *MultiHostClient) RestartContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerRestart(ctx, id, container.StopOptions{})
}

func (c *MultiHostClient) RemoveContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerRemove(ctx, id, container.RemoveOptions{})
}

func (c *MultiHostClient) GetEnvVariables(ctx context.Context, hostName, id string) (map[string]string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	inspect, err := apiClient.ContainerInspect(ctx, id)
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

// containerRecreateAPI is the subset of the Docker client used by the
// env-edit recreate flow, extracted so the rollback logic can be tested.
type containerRecreateAPI interface {
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRename(ctx context.Context, containerID, newContainerName string) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
}

// envEditLocks serializes concurrent env edits on the same container,
// keyed by host+containerID. Package-level so locks survive client hot-swaps.
var envEditLocks sync.Map

func envEditLock(hostName, id string) *sync.Mutex {
	lock, _ := envEditLocks.LoadOrStore(hostName+"/"+id, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// SetEnvVariables recreates a container with updated environment variables.
// Returns the new container ID and the original container's labels.
func (c *MultiHostClient) SetEnvVariables(ctx context.Context, hostName, id string, envVariables map[string]string) (string, map[string]string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return "", nil, err
	}

	lock := envEditLock(hostName, id)
	lock.Lock()
	defer lock.Unlock()

	inspect, err := apiClient.ContainerInspect(ctx, id)
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

	newID, err := recreateContainerWithEnv(ctx, apiClient, inspect, envs)
	if err != nil {
		return "", nil, err
	}

	return newID, labels, nil
}

// recreateContainerWithEnv replaces a container with an identical one whose
// env is envs. The original is renamed aside (not removed) until the
// replacement is running, so any failure rolls back to the original.
func recreateContainerWithEnv(ctx context.Context, apiClient containerRecreateAPI, inspect container.InspectResponse, envs []string) (string, error) {
	containerName := strings.TrimPrefix(inspect.Name, "/")
	wasRunning := inspect.State != nil && inspect.State.Running

	shortID := inspect.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	tempName := containerName + "-logdeck-old-" + shortID

	// Rollback and cleanup must proceed even if the request was canceled.
	cleanupCtx := context.WithoutCancel(ctx)

	// rollback removes the partially-created replacement (if any), renames
	// the original back, and restarts it if it was running.
	rollback := func(newID string) {
		if newID != "" {
			_ = apiClient.ContainerRemove(cleanupCtx, newID, container.RemoveOptions{Force: true})
		}
		_ = apiClient.ContainerRename(cleanupCtx, inspect.ID, containerName)
		if wasRunning {
			_ = apiClient.ContainerStart(cleanupCtx, inspect.ID, container.StartOptions{})
		}
	}

	if err := apiClient.ContainerStop(ctx, inspect.ID, container.StopOptions{}); err != nil {
		return "", err
	}

	if err := apiClient.ContainerRename(ctx, inspect.ID, tempName); err != nil {
		// Name unchanged; just restart the original if it was running.
		if wasRunning {
			_ = apiClient.ContainerStart(cleanupCtx, inspect.ID, container.StartOptions{})
		}
		return "", err
	}

	newConfig := inspect.Config
	newConfig.Env = envs

	var networking *network.NetworkingConfig
	if inspect.NetworkSettings != nil {
		networking = &network.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		}
	}

	resp, err := apiClient.ContainerCreate(
		ctx,
		newConfig,
		inspect.HostConfig,
		networking,
		nil,
		containerName,
	)
	if err != nil {
		rollback("")
		return "", err
	}

	// Only start the replacement if the original was running, so editing
	// env vars on a stopped container leaves it stopped.
	if wasRunning {
		if err := apiClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			rollback(resp.ID)
			return "", err
		}
	}

	// Replacement is running; removing the renamed original is best-effort.
	if err := apiClient.ContainerRemove(cleanupCtx, inspect.ID, container.RemoveOptions{}); err != nil {
		log.Printf("Warning: failed to remove old container %s after env update: %v", tempName, err)
	}

	return resp.ID, nil
}
