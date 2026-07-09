package docker

import (
	"context"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/system"
)

// GetHostsInfo fetches engine-level info for every configured host
// concurrently. Unreachable hosts are reported as unavailable instead of
// failing the whole call.
func (c *MultiHostClient) GetHostsInfo(ctx context.Context) []models.HostInfo {
	result := make([]models.HostInfo, len(c.hosts))
	var wg sync.WaitGroup

	for i, host := range c.hosts {
		wg.Add(1)
		go func(idx int, hostName string) {
			defer wg.Done()

			apiClient, err := c.GetClient(hostName)
			if err != nil {
				result[idx] = models.HostInfo{Host: hostName, Error: err.Error()}
				return
			}

			engineInfo, err := apiClient.Info(ctx)
			if err != nil {
				result[idx] = models.HostInfo{Host: hostName, Error: err.Error()}
				return
			}

			result[idx] = hostInfoFromEngine(hostName, engineInfo)
		}(i, host.Name)
	}

	wg.Wait()
	return result
}

// hostInfoFromEngine maps the engine's Info response to the API model.
func hostInfoFromEngine(hostName string, info system.Info) models.HostInfo {
	return models.HostInfo{
		Host:              hostName,
		Available:         true,
		Name:              info.Name,
		OperatingSystem:   info.OperatingSystem,
		Architecture:      info.Architecture,
		ServerVersion:     info.ServerVersion,
		NCPU:              info.NCPU,
		MemTotal:          info.MemTotal,
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
	}
}
