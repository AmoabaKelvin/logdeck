package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCalculateMemoryStatsSubtractsCache(t *testing.T) {
	var stats container.StatsResponse
	stats.MemoryStats.Usage = 100
	stats.MemoryStats.Limit = 200
	stats.MemoryStats.Stats = map[string]uint64{"cache": 30}

	percent, usage, limit := calculateMemoryStats(&stats)
	if usage != 70 {
		t.Fatalf("usage = %d, want 70 (cache subtracted)", usage)
	}
	if limit != 200 {
		t.Fatalf("limit = %d, want 200", limit)
	}
	if percent != 35.0 { // 70/200
		t.Fatalf("percent = %f, want 35.0", percent)
	}
}

// A cache value larger than usage must not underflow the unsigned usage.
func TestCalculateMemoryStatsCacheLargerThanUsageIgnored(t *testing.T) {
	var stats container.StatsResponse
	stats.MemoryStats.Usage = 20
	stats.MemoryStats.Limit = 200
	stats.MemoryStats.Stats = map[string]uint64{"cache": 50}

	_, usage, _ := calculateMemoryStats(&stats)
	if usage != 20 {
		t.Fatalf("usage = %d, want 20 (cache subtraction skipped to avoid underflow)", usage)
	}
}

func TestCalculateMemoryStatsZeroLimit(t *testing.T) {
	var stats container.StatsResponse
	stats.MemoryStats.Usage = 100
	stats.MemoryStats.Limit = 0

	percent, usage, limit := calculateMemoryStats(&stats)
	if percent != 0 {
		t.Fatalf("percent = %f, want 0 with a zero limit (no divide by zero)", percent)
	}
	if usage != 100 || limit != 0 {
		t.Fatalf("usage/limit = %d/%d, want 100/0", usage, limit)
	}
}

func TestNumCPUs(t *testing.T) {
	online := &container.StatsResponse{}
	online.CPUStats.OnlineCPUs = 8
	if got := numCPUs(online); got != 8 {
		t.Fatalf("numCPUs with OnlineCPUs = %f, want 8", got)
	}

	percpu := &container.StatsResponse{}
	percpu.CPUStats.CPUUsage.PercpuUsage = []uint64{1, 2, 3, 4}
	if got := numCPUs(percpu); got != 4 {
		t.Fatalf("numCPUs falling back to PercpuUsage = %f, want 4", got)
	}

	none := &container.StatsResponse{}
	if got := numCPUs(none); got != 1 {
		t.Fatalf("numCPUs with no info = %f, want 1", got)
	}
}
