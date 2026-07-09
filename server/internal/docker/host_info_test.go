package docker

import (
	"testing"

	"github.com/docker/docker/api/types/system"
)

func TestHostInfoFromEngine(t *testing.T) {
	engineInfo := system.Info{
		Name:              "daemon-host",
		OperatingSystem:   "Ubuntu 24.04 LTS",
		Architecture:      "x86_64",
		ServerVersion:     "28.0.1",
		NCPU:              8,
		MemTotal:          16 * 1024 * 1024 * 1024,
		ContainersRunning: 3,
		ContainersPaused:  1,
		ContainersStopped: 2,
		Images:            10,
	}

	got := hostInfoFromEngine("production", engineInfo)

	if got.Host != "production" {
		t.Errorf("Host = %q, want %q", got.Host, "production")
	}
	if !got.Available {
		t.Error("Available = false, want true")
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want empty", got.Error)
	}
	if got.Name != engineInfo.Name {
		t.Errorf("Name = %q, want %q", got.Name, engineInfo.Name)
	}
	if got.OperatingSystem != engineInfo.OperatingSystem {
		t.Errorf("OperatingSystem = %q, want %q", got.OperatingSystem, engineInfo.OperatingSystem)
	}
	if got.Architecture != engineInfo.Architecture {
		t.Errorf("Architecture = %q, want %q", got.Architecture, engineInfo.Architecture)
	}
	if got.ServerVersion != engineInfo.ServerVersion {
		t.Errorf("ServerVersion = %q, want %q", got.ServerVersion, engineInfo.ServerVersion)
	}
	if got.NCPU != engineInfo.NCPU {
		t.Errorf("NCPU = %d, want %d", got.NCPU, engineInfo.NCPU)
	}
	if got.MemTotal != engineInfo.MemTotal {
		t.Errorf("MemTotal = %d, want %d", got.MemTotal, engineInfo.MemTotal)
	}
	if got.ContainersRunning != engineInfo.ContainersRunning {
		t.Errorf("ContainersRunning = %d, want %d", got.ContainersRunning, engineInfo.ContainersRunning)
	}
	if got.ContainersPaused != engineInfo.ContainersPaused {
		t.Errorf("ContainersPaused = %d, want %d", got.ContainersPaused, engineInfo.ContainersPaused)
	}
	if got.ContainersStopped != engineInfo.ContainersStopped {
		t.Errorf("ContainersStopped = %d, want %d", got.ContainersStopped, engineInfo.ContainersStopped)
	}
	if got.Images != engineInfo.Images {
		t.Errorf("Images = %d, want %d", got.Images, engineInfo.Images)
	}
}
