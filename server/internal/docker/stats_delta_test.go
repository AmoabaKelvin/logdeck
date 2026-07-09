package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func podmanStatsResponse(totalUsage, systemUsage uint64) *container.StatsResponse {
	// Podman's one-shot stats leave PreCPUStats zeroed.
	resp := &container.StatsResponse{}
	resp.CPUStats.CPUUsage.TotalUsage = totalUsage
	resp.CPUStats.SystemUsage = systemUsage
	resp.CPUStats.OnlineCPUs = 4
	return resp
}

func TestCurrentCPUPercentPodmanUsesPollDeltas(t *testing.T) {
	key := "test-host:podman-delta-container"

	// First reading has no previous sample: falls back to the engine
	// numbers, i.e. a since-start average (total/system * cpus).
	first := currentCPUPercent(key, podmanStatsResponse(50_000, 1_000_000))
	if want := 50_000.0 / 1_000_000.0 * 4 * 100; first != want {
		t.Errorf("first reading = %f, want since-start fallback %f", first, want)
	}

	// Second reading diffs against the first: 10k of 100k system time
	// on 4 CPUs = 40%, regardless of the lifetime totals.
	second := currentCPUPercent(key, podmanStatsResponse(60_000, 1_100_000))
	if want := 10_000.0 / 100_000.0 * 4 * 100; second != want {
		t.Errorf("second reading = %f, want delta-based %f", second, want)
	}
}

func TestCurrentCPUPercentDockerWindowUntouched(t *testing.T) {
	key := "test-host:docker-window-container"

	resp := podmanStatsResponse(60_000, 1_100_000)
	resp.PreCPUStats.CPUUsage.TotalUsage = 55_000
	resp.PreCPUStats.SystemUsage = 1_050_000

	// Engine provided a proper window: use it directly.
	got := currentCPUPercent(key, resp)
	if want := 5_000.0 / 50_000.0 * 4 * 100; got != want {
		t.Errorf("docker reading = %f, want engine-window %f", got, want)
	}
}

func TestCurrentCPUPercentCounterResetFallsBack(t *testing.T) {
	key := "test-host:reset-container"

	currentCPUPercent(key, podmanStatsResponse(60_000, 1_100_000))
	// Counters went backwards (container restarted): fall back to the
	// engine numbers instead of producing a negative delta.
	got := currentCPUPercent(key, podmanStatsResponse(5_000, 90_000))
	if want := 5_000.0 / 90_000.0 * 4 * 100; got != want {
		t.Errorf("post-reset reading = %f, want fallback %f", got, want)
	}
}
