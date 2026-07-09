package docker

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
)

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

	if req.NanoCPUs != nil {
		cfg.NanoCPUs = *req.NanoCPUs
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

	_, err = apiClient.ContainerUpdate(ctx, id, buildUpdateConfig(req))
	return err
}
