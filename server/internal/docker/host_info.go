package docker

import (
	"context"
	"strings"
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

// LocalEngineHostname returns the hostname reported by the engine behind the
// first unix-socket host, which is the machine LogDeck itself runs on. It
// returns "" when there is no such host or the engine cannot be reached.
func (c *MultiHostClient) LocalEngineHostname(ctx context.Context) string {
	for _, host := range c.hosts {
		if !strings.HasPrefix(host.Host, "unix://") {
			continue
		}
		apiClient, err := c.GetClient(host.Name)
		if err != nil {
			continue
		}
		if info, err := apiClient.Info(ctx); err == nil {
			return info.Name
		}
	}
	return ""
}

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
