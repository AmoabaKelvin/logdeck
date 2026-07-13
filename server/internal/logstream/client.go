package logstream

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// engineClient is the slice of the Docker client the hub needs. Tests inject
// fakes; production wraps *docker.MultiHostClient. Implementations must be
// comparable so the hub can detect hot-swapped clients by equality.
type engineClient interface {
	listContainers(ctx context.Context) (map[string][]models.ContainerInfo, []docker.HostError, error)
	streamEvents(ctx context.Context) <-chan docker.EngineEvent
	openTail(ctx context.Context, host, containerID string, opts models.LogOptions, emit func(models.LogEntry)) error
}

// dockerAdapter adapts one *docker.MultiHostClient snapshot. It is a
// comparable single-pointer struct: two adapters are equal exactly when they
// wrap the same client, which is what the hot-swap check compares.
type dockerAdapter struct {
	c *docker.MultiHostClient
}

func (a dockerAdapter) listContainers(ctx context.Context) (map[string][]models.ContainerInfo, []docker.HostError, error) {
	return a.c.ListContainersAllHosts(ctx)
}

func (a dockerAdapter) streamEvents(ctx context.Context) <-chan docker.EngineEvent {
	return a.c.StreamEngineEvents(ctx)
}

func (a dockerAdapter) openTail(ctx context.Context, host, containerID string, opts models.LogOptions, emit func(models.LogEntry)) error {
	return a.c.TailContainerLogs(ctx, host, containerID, opts, emit)
}
