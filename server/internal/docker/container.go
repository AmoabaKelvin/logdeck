package docker

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (c *MultiHostClient) GetContainer(ctx context.Context, hostName, id string) (container.InspectResponse, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return container.InspectResponse{}, err
	}
	return apiClient.ContainerInspect(ctx, id)
}

func (c *MultiHostClient) StartContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	return apiClient.ContainerStart(ctx, id, container.StartOptions{})
}

// Quadlet runs containers with --rm under a systemd unit. Stopping or
// restarting one through the API deletes it and leaves the unit failed.
const systemdUnitLabel = "PODMAN_SYSTEMD_UNIT"

// systemdUnit returns the unit that owns the container, or "". podman-compose
// stamps its own unit name on every container, systemd or not.
func systemdUnit(labels map[string]string) string {
	unit := labels[systemdUnitLabel]
	if strings.HasPrefix(unit, "podman-compose@") {
		return ""
	}
	return unit
}

// SystemdManagedError refuses an action that has to go through systemctl.
type SystemdManagedError struct {
	Unit   string
	Action string
}

func (e *SystemdManagedError) Error() string {
	if e.Action == "edit" {
		return fmt.Sprintf("container is managed by systemd unit %s; change its environment in the Quadlet file, then run \"systemctl restart %s\" on the host (with --user for rootless Podman)", e.Unit, e.Unit)
	}
	return fmt.Sprintf("container is managed by systemd unit %s; run \"systemctl %s %s\" on the host instead (with --user for rootless Podman)", e.Unit, e.Action, e.Unit)
}

func checkNotSystemdManaged(labels map[string]string, action string) error {
	if unit := systemdUnit(labels); unit != "" {
		return &SystemdManagedError{Unit: unit, Action: action}
	}
	return nil
}

func (c *MultiHostClient) StopContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	inspect, err := apiClient.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}
	if err := checkNotSystemdManaged(inspect.Config.Labels, "stop"); err != nil {
		return err
	}
	c.stops.Store(hostName+"|"+inspect.ID, time.Now())
	return apiClient.ContainerStop(ctx, id, container.StopOptions{})
}

// StoppedByUser reports whether LogDeck just stopped the container, or Podman
// recorded its last exit as a user stop. Podman's die event can arrive after
// LogDeck has already removed the container, so its own stops are remembered
// here, only until their die event is due. Docker hosts announce stops with a
// kill event instead.
func (c *MultiHostClient) StoppedByUser(ctx context.Context, hostName, id string) bool {
	if at, ok := c.stops.LoadAndDelete(hostName + "|" + id); ok && time.Since(at.(time.Time)) < 15*time.Second {
		return true
	}
	cl, err := c.GetClient(hostName)
	if err != nil {
		return false
	}
	var inspect struct {
		State struct {
			StoppedByUser bool `json:"StoppedByUser"`
		} `json:"State"`
	}
	if err := libpodGet(ctx, cl, "/containers/"+url.PathEscape(id)+"/json", &inspect); err != nil {
		return false
	}
	return inspect.State.StoppedByUser
}

func (c *MultiHostClient) RestartContainer(ctx context.Context, hostName, id string) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}
	inspect, err := apiClient.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}
	if err := checkNotSystemdManaged(inspect.Config.Labels, "restart"); err != nil {
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
	if err := checkNotSystemdManaged(labels, "edit"); err != nil {
		return "", nil, err
	}
	isCoolifyManaged := labels[coolify.LabelManaged] == "true"
	c.stops.Store(hostName+"|"+inspect.ID, time.Now())

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

	for key := range envMap {
		if _, exists := envVariables[key]; !exists {
			delete(envMap, key)
		}
	}

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

	// Copy the config so we don't mutate the shared InspectResponse in place.
	newConfig := *inspect.Config
	newConfig.Env = envs

	var networking *network.NetworkingConfig
	if inspect.NetworkSettings != nil {
		networking = &network.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		}
	}

	// Podman reports a CPU limit as both NanoCpus and CpuQuota/CpuPeriod, but
	// rejects a create that carries both ("NanoCpus conflicts with CpuPeriod and
	// CpuQuota"). They express the same limit, and the quota/period pair is what
	// the resource-update path writes on Podman, so NanoCpus is the one to drop.
	// Copy the host config rather than mutate the shared InspectResponse.
	hostConfig := inspect.HostConfig
	if hostConfig != nil {
		clone := *hostConfig
		if clone.NanoCPUs > 0 && clone.CPUQuota > 0 {
			clone.NanoCPUs = 0
		}
		keepAnonymousVolumes(&clone, inspect.Mounts)
		hostConfig = &clone
	}

	resp, err := apiClient.ContainerCreate(
		ctx,
		&newConfig,
		hostConfig,
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

// keepAnonymousVolumes mounts the old container's anonymous volumes into the
// replacement, which would otherwise start with new empty ones. Compose lists
// them as volume mounts with no source; `-v /path` and image VOLUMEs are not
// listed at all.
func keepAnonymousVolumes(hostConfig *container.HostConfig, mounts []container.MountPoint) {
	names := make(map[string]string)
	for _, m := range mounts {
		if m.Type == mount.TypeVolume && m.Name != "" {
			names[m.Destination] = m.Name
		}
	}

	used := make(map[string]bool)
	for _, bind := range hostConfig.Binds {
		if parts := strings.Split(bind, ":"); len(parts) >= 2 {
			used[parts[1]] = true
		}
	}

	hostConfig.Mounts = slices.Clone(hostConfig.Mounts)
	for i, m := range hostConfig.Mounts {
		if m.Type == mount.TypeVolume && m.Source == "" {
			hostConfig.Mounts[i].Source = names[m.Target]
		}
		used[m.Target] = true
	}

	for _, m := range mounts {
		if m.Type != mount.TypeVolume || m.Name == "" || used[m.Destination] {
			continue
		}
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   m.Name,
			Target:   m.Destination,
			ReadOnly: !m.RW,
		})
	}
}
