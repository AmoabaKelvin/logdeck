package docker

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// engineEventActions are the container actions consumed by the alerting
// engine and the shared log-tail hub. Distinct from watchedContainerActions
// (the frontend stream, which also handles health separately): the engine
// needs "oom" to fire rules; start/destroy/rename exist for the hub's tail
// lifecycle.
var engineEventActions = []string{
	"start",
	"die",
	"oom",
	"destroy",
	"rename",
}

// EngineEvent is the richer event shape consumed by the alerting engine. It
// keeps the exit code and the full attribute set (compose labels etc.) that
// the frontend stream deliberately drops.
type EngineEvent struct {
	Host          string
	ContainerID   string
	ContainerName string
	Action        string
	ExitCode      string            // die events, from Actor.Attributes["exitCode"], "" if absent
	Labels        map[string]string // full Actor.Attributes copy
	Timestamp     int64
}

func engineEventFilters() filters.Args {
	args := filters.NewArgs(filters.Arg("type", string(events.ContainerEventType)))
	for _, action := range engineEventActions {
		args.Add("event", action)
	}
	return args
}

// mapEngineEvent converts a Docker SDK event message into an EngineEvent.
// It returns false for events that should be skipped.
func mapEngineEvent(hostName string, msg events.Message) (EngineEvent, bool) {
	if msg.Type != events.ContainerEventType {
		return EngineEvent{}, false
	}

	// Some actions arrive suffixed with detail, e.g. "health_status: healthy";
	// match the watched set against the base action before ": ".
	action := string(msg.Action)
	base, _, _ := strings.Cut(action, ": ")
	if !slices.Contains(engineEventActions, base) {
		return EngineEvent{}, false
	}

	timestamp := msg.Time
	if timestamp == 0 && msg.TimeNano > 0 {
		timestamp = msg.TimeNano / int64(time.Second)
	}

	exitCode := ""
	if base == "die" {
		exitCode = msg.Actor.Attributes["exitCode"]
	}

	return EngineEvent{
		Host:          hostName,
		ContainerID:   msg.Actor.ID,
		ContainerName: msg.Actor.Attributes["name"],
		Action:        action,
		ExitCode:      exitCode,
		Labels:        maps.Clone(msg.Actor.Attributes),
		Timestamp:     timestamp,
	}, true
}

// StreamEngineEvents subscribes to the alerting-relevant container events on
// every configured host and fans them into a single channel. Same lifecycle
// as StreamContainerEvents: per-host failures are retried with backoff, and
// the returned channel closes once all host subscriptions have stopped.
func (c *MultiHostClient) StreamEngineEvents(ctx context.Context) <-chan EngineEvent {
	out := make(chan EngineEvent)
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()
			options := events.ListOptions{Filters: engineEventFilters()}
			watchHostEventStream(ctx, name, cl, options, func(msg events.Message) {
				event, ok := mapEngineEvent(name, msg)
				if !ok {
					return
				}
				select {
				case out <- event:
				case <-ctx.Done():
				}
			})
		}(hostName, apiClient)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
