package docker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
)

// Docker Compose and recent podman-compose both set the com.docker label;
// older podman-compose releases only set the io.podman one.
var composeProjectLabels = []string{
	"com.docker.compose.project",
	"io.podman.compose.project",
}

func inComposeProject(labels map[string]string, project string) bool {
	for _, label := range composeProjectLabels {
		if labels[label] == project {
			return true
		}
	}
	return false
}

// composeTarget identifies one container a compose action applies to.
type composeTarget struct {
	ID   string
	Name string
}

// applyToTargets runs apply on each target concurrently and aggregates
// per-container failures instead of aborting the whole action.
func applyToTargets(ctx context.Context, targets []composeTarget, apply func(ctx context.Context, id string) error) (succeeded int, failed []models.ComposeContainerFailure) {
	failed = []models.ComposeContainerFailure{} // non-nil so the JSON response renders [] rather than null
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(t composeTarget) {
			defer wg.Done()
			err := apply(ctx, t.ID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed = append(failed, models.ComposeContainerFailure{
					ID:    t.ID,
					Name:  t.Name,
					Error: err.Error(),
				})
				return
			}
			succeeded++
		}(target)
	}

	wg.Wait()
	return succeeded, failed
}

// ComposeProjectAction applies start, stop, or restart to every container in a
// compose project on one host.
func (c *MultiHostClient) ComposeProjectAction(ctx context.Context, hostName, project, action string) (models.ComposeActionResult, error) {
	result := models.ComposeActionResult{
		Project: project,
		Host:    hostName,
		Failed:  []models.ComposeContainerFailure{},
	}

	var apply func(ctx context.Context, id string) error
	switch action {
	case "start":
		apply = func(ctx context.Context, id string) error { return c.StartContainer(ctx, hostName, id) }
	case "stop":
		apply = func(ctx context.Context, id string) error { return c.StopContainer(ctx, hostName, id) }
	case "restart":
		apply = func(ctx context.Context, id string) error { return c.RestartContainer(ctx, hostName, id) }
	default:
		return result, fmt.Errorf("unsupported compose action: %s", action)
	}

	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return result, err
	}

	containers, err := apiClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return result, err
	}

	var targets []composeTarget
	for _, ctr := range containers {
		if !inComposeProject(ctr.Labels, project) {
			continue
		}
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}
		targets = append(targets, composeTarget{ID: ctr.ID, Name: name})
	}

	result.Total = len(targets)
	result.Succeeded, result.Failed = applyToTargets(ctx, targets, apply)
	return result, nil
}
