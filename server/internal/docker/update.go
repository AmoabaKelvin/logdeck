package docker

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
)

// cpuPeriod is the standard CFS scheduler period (100ms in microseconds).
const cpuPeriod = 100000

// NanoCPUsFromHostConfig returns the effective CPU limit in nano-CPUs,
// whether the engine stored it as NanoCPUs or as quota/period.
func NanoCPUsFromHostConfig(nanoCPUs, quota, period int64) int64 {
	if nanoCPUs > 0 {
		return nanoCPUs
	}
	if quota > 0 && period > 0 {
		return quota * 1e9 / period
	}
	return 0
}

// buildUpdateConfig maps an update request onto Docker's UpdateConfig.
// The daemon treats zero-valued resource fields as "unchanged".
func buildUpdateConfig(req models.UpdateResourcesRequest) container.UpdateConfig {
	var cfg container.UpdateConfig

	if req.MemoryBytes != nil {
		cfg.Memory = *req.MemoryBytes
		if *req.MemoryBytes > 0 {
			// Unlimited swap avoids the "memoryswap must be >= memory"
			// daemon error when raising the memory limit.
			cfg.MemorySwap = -1
		}
	}

	if req.NanoCPUs != nil && *req.NanoCPUs > 0 {
		// Express the CPU limit as quota/period instead of NanoCPUs:
		// Podman's compat update endpoint silently ignores NanoCpus,
		// while quota/period works on both Docker and Podman.
		cfg.CPUPeriod = cpuPeriod
		cfg.CPUQuota = *req.NanoCPUs * cpuPeriod / 1e9
	}

	if req.RestartPolicy != nil {
		cfg.RestartPolicy = container.RestartPolicy{
			Name:              container.RestartPolicyMode(req.RestartPolicy.Name),
			MaximumRetryCount: req.RestartPolicy.MaximumRetryCount,
		}
	}

	return cfg
}

// UpdateContainerResources applies resource limits and restart policy via the
// Docker update API — a live update with no container recreate and no downtime.
func (c *MultiHostClient) UpdateContainerResources(ctx context.Context, hostName, id string, req models.UpdateResourcesRequest) error {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return err
	}

	cfg := buildUpdateConfig(req)

	// Podman resets the restart policy to "no" when an update omits it
	// (Docker leaves it unchanged), so carry the current policy over
	// explicitly when the request doesn't set one.
	if req.RestartPolicy == nil {
		inspect, err := apiClient.ContainerInspect(ctx, id)
		if err != nil {
			return err
		}
		if inspect.HostConfig != nil {
			cfg.RestartPolicy = inspect.HostConfig.RestartPolicy
		}
	}

	_, err = apiClient.ContainerUpdate(ctx, id, cfg)
	return err
}
