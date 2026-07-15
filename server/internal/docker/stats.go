package docker

import (
	"context"
	"encoding/json"
	"fmt"
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

// cpuSample is one raw CPU counter reading, kept per container so
// consecutive polls can be diffed. Podman's one-shot stats endpoint leaves
// PreCPUStats zeroed (Docker double-samples server-side), which would turn
// the naive calculation into a since-start average instead of current usage.
type cpuSample struct {
	totalUsage  uint64
	systemUsage uint64
	readAt      time.Time
}

const cpuSampleMaxAge = 2 * time.Minute

var (
	prevCPUMu      sync.Mutex
	prevCPUSamples = map[string]cpuSample{}
)

// swapPrevCPUSample stores the current reading and returns the previous one.
func swapPrevCPUSample(key string, current cpuSample) (cpuSample, bool) {
	prevCPUMu.Lock()
	defer prevCPUMu.Unlock()

	prev, ok := prevCPUSamples[key]
	prevCPUSamples[key] = current

	if len(prevCPUSamples) > 256 {
		for k, s := range prevCPUSamples {
			if time.Since(s.readAt) > cpuSampleMaxAge {
				delete(prevCPUSamples, k)
			}
		}
	}
	return prev, ok
}

// currentCPUPercent returns an instantaneous CPU percentage. When the engine
// pre-populated PreCPUStats (Docker), its window is used directly; otherwise
// (Podman one-shot) the reading is diffed against our previous poll. The very
// first Podman reading falls back to the engine numbers — a since-start
// average — which is better than reporting 0.
func currentCPUPercent(key string, stats *container.StatsResponse) float64 {
	current := cpuSample{
		totalUsage:  stats.CPUStats.CPUUsage.TotalUsage,
		systemUsage: stats.CPUStats.SystemUsage,
		readAt:      time.Now(),
	}
	prev, ok := swapPrevCPUSample(key, current)

	if stats.PreCPUStats.SystemUsage > 0 {
		return calculateCPUPercent(stats)
	}

	if ok && time.Since(prev.readAt) < cpuSampleMaxAge &&
		current.totalUsage >= prev.totalUsage &&
		current.systemUsage > prev.systemUsage {
		cpuDelta := float64(current.totalUsage - prev.totalUsage)
		systemDelta := float64(current.systemUsage - prev.systemUsage)
		return (cpuDelta / systemDelta) * numCPUs(stats) * 100.0
	}

	return calculateCPUPercent(stats)
}

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
		CPUPercent:    currentCPUPercent(cacheKey, &v),
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

func (c *MultiHostClient) GetAllRunningContainerStats(ctx context.Context) ([]models.ContainerStats, error) {
	var runningContainers []ContainerIdentifier

	for hostName, apiClient := range c.clients {
		containers, err := apiClient.ContainerList(ctx, container.ListOptions{All: false})
		if err != nil {
			continue // an unreachable host must not fail the whole call
		}

		for _, ctr := range containers {
			runningContainers = append(runningContainers, ContainerIdentifier{
				ID:   ctr.ID,
				Host: hostName,
			})
		}
	}

	return c.GetBulkContainerStats(ctx, runningContainers), nil
}

func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0.0
	}

	return (cpuDelta / systemDelta) * numCPUs(stats) * 100.0
}

func numCPUs(stats *container.StatsResponse) float64 {
	n := float64(stats.CPUStats.OnlineCPUs)
	if n == 0 {
		if n = float64(len(stats.CPUStats.CPUUsage.PercpuUsage)); n == 0 {
			n = 1
		}
	}
	return n
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
