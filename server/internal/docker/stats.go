package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
)

const statsCacheTTL = 3 * time.Second

// statsCache holds cached container stats with TTL
type statsCache struct {
	sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	stats     *models.ContainerStats
	timestamp time.Time
}

var cache = &statsCache{
	entries: make(map[string]*cacheEntry),
}

func getCachedStats(key string) *models.ContainerStats {
	cache.RLock()
	defer cache.RUnlock()

	if entry, ok := cache.entries[key]; ok && time.Since(entry.timestamp) < statsCacheTTL {
		return entry.stats
	}
	return nil
}

func setCachedStats(key string, stats *models.ContainerStats) {
	cache.Lock()
	defer cache.Unlock()

	cache.entries[key] = &cacheEntry{
		stats:     stats,
		timestamp: time.Now(),
	}

	if len(cache.entries) > 100 {
		pruneCache()
	}
}

func pruneCache() {
	for key, entry := range cache.entries {
		if time.Since(entry.timestamp) > 10*time.Second {
			delete(cache.entries, key)
		}
	}
}

// ContainerIdentifier represents a container on a specific host
type ContainerIdentifier struct {
	ID   string
	Host string
}

func (c *MultiHostClient) GetContainerStats(ctx context.Context, hostName, containerID string) (*models.ContainerStats, error) {
	cacheKey := fmt.Sprintf("%s:%s", hostName, containerID)

	if stats := getCachedStats(cacheKey); stats != nil {
		return stats, nil
	}

	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}

	statsResp, err := apiClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	var v container.StatsResponse
	if err := json.NewDecoder(statsResp.Body).Decode(&v); err != nil {
		return nil, err
	}

	memPercent, memUsed, memLimit := calculateMemoryStats(&v)

	stats := &models.ContainerStats{
		ID:            containerID,
		Host:          hostName,
		CPUPercent:    calculateCPUPercent(&v),
		MemoryPercent: memPercent,
		MemoryUsed:    memUsed,
		MemoryLimit:   memLimit,
	}

	setCachedStats(cacheKey, stats)
	return stats, nil
}

func (c *MultiHostClient) GetBulkContainerStats(ctx context.Context, containers []ContainerIdentifier) []models.ContainerStats {
	var results []models.ContainerStats
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cont := range containers {
		wg.Add(1)
		go func(identifier ContainerIdentifier) {
			defer wg.Done()

			if ctx.Err() != nil {
				return
			}

			stats, err := c.GetContainerStats(ctx, identifier.Host, identifier.ID)
			if err != nil {
				log.Printf("Failed to get stats for container %s on host %s: %v", identifier.ID[:12], identifier.Host, err)
				return
			}

			mu.Lock()
			results = append(results, *stats)
			mu.Unlock()
		}(cont)
	}

	wg.Wait()
	return results
}

// GetAllRunningContainerStats fetches stats for all running containers across all hosts
func (c *MultiHostClient) GetAllRunningContainerStats(ctx context.Context) ([]models.ContainerStats, error) {
	// First, get list of all running containers
	var runningContainers []ContainerIdentifier

	for hostName, apiClient := range c.clients {
		containers, err := apiClient.ContainerList(ctx, container.ListOptions{
			All: false, // Only running containers
		})
		if err != nil {
			// Log error but continue with other hosts
			continue
		}

		for _, ctr := range containers {
			runningContainers = append(runningContainers, ContainerIdentifier{
				ID:   ctr.ID,
				Host: hostName,
			})
		}
	}

	// Fetch stats for all running containers in parallel
	return c.GetBulkContainerStats(ctx, runningContainers), nil
}

func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0.0
	}

	numCPUs := float64(stats.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		if numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage)); numCPUs == 0 {
			numCPUs = 1
		}
	}

	return (cpuDelta / systemDelta) * numCPUs * 100.0
}

func calculateMemoryStats(stats *container.StatsResponse) (percent float64, usage uint64, limit uint64) {
	usage = stats.MemoryStats.Usage
	limit = stats.MemoryStats.Limit

	if cache, ok := stats.MemoryStats.Stats["cache"]; ok && usage > cache {
		usage -= cache
	}

	if limit > 0 {
		percent = (float64(usage) / float64(limit)) * 100.0
	}

	return
}
