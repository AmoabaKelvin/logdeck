package system

import (
	"context"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemStats struct {
	HostInfo HostInfo `json:"hostInfo"`
	Usage    Usage    `json:"usage"`
}

type HostInfo struct {
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
	KernelVersion   string `json:"kernelVersion"`
	Arch            string `json:"arch"`
	Uptime          uint64 `json:"uptime"`
}

type Usage struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	MemoryTotal   uint64  `json:"memoryTotal"`
	MemoryUsed    uint64  `json:"memoryUsed"`
}

// Init points gopsutil at /host/proc when it is mounted, so a containerized
// server reports host metrics instead of its own namespace's.
func Init() {
	if _, err := os.Stat("/host/proc"); err == nil {
		os.Setenv("HOST_PROC", "/host/proc")
	}
}

func GetStats(ctx context.Context) (*SystemStats, error) {
	hInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	// A zero interval returns usage since the previous call without blocking
	// the request; the very first call may report 0.
	cpuPercents, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, err
	}

	var cpuPercent float64
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	return &SystemStats{
		HostInfo: HostInfo{
			Hostname:        hInfo.Hostname,
			Platform:        hInfo.Platform,
			PlatformVersion: hInfo.PlatformVersion,
			KernelVersion:   hInfo.KernelVersion,
			Arch:            runtime.GOARCH,
			Uptime:          hInfo.Uptime,
		},
		Usage: Usage{
			CPUPercent:    cpuPercent,
			MemoryPercent: vMem.UsedPercent,
			MemoryTotal:   vMem.Total,
			MemoryUsed:    vMem.Used,
		},
	}, nil
}
