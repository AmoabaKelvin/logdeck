package docker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// watchedContainerActions are the container lifecycle actions streamed to the frontend.
var watchedContainerActions = []string{
	"create",
	"start",
	"stop",
	"die",
	"restart",
	"pause",
	"unpause",
	"destroy",
	"rename",
}

const (
	eventsInitialBackoff = 1 * time.Second
	eventsMaxBackoff     = 30 * time.Second
)

func containerEventFilters() filters.Args {
	args := filters.NewArgs(filters.Arg("type", string(events.ContainerEventType)))
	for _, action := range watchedContainerActions {
		args.Add("event", action)
	}
	return args
}

// mapContainerEvent converts a Docker SDK event message into the compact shape
// streamed to the frontend. It returns false for events that should be skipped.
func mapContainerEvent(hostName string, msg events.Message) (models.ContainerEvent, bool) {
	if msg.Type != events.ContainerEventType {
		return models.ContainerEvent{}, false
	}

	action := string(msg.Action)
	watched := false
	for _, a := range watchedContainerActions {
		if action == a {
			watched = true
			break
		}
	}
	if !watched {
		return models.ContainerEvent{}, false
	}

	timestamp := msg.Time
	if timestamp == 0 && msg.TimeNano > 0 {
		timestamp = msg.TimeNano / int64(time.Second)
	}

	return models.ContainerEvent{
		Host:          hostName,
		ContainerID:   msg.Actor.ID,
		ContainerName: msg.Actor.Attributes["name"],
		Action:        action,
		Timestamp:     timestamp,
	}, true
}

// StreamContainerEvents subscribes to container lifecycle events on every
// configured host and fans them into a single channel. Subscriptions live
// until ctx is cancelled; per-host failures are retried with backoff while
// the other hosts keep streaming. The returned channel is closed once all
// host subscriptions have stopped.
func (c *MultiHostClient) StreamContainerEvents(ctx context.Context) <-chan models.ContainerEvent {
	out := make(chan models.ContainerEvent)
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()
			watchHostEvents(ctx, name, cl, out)
		}(hostName, apiClient)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func watchHostEvents(ctx context.Context, hostName string, cl *client.Client, out chan<- models.ContainerEvent) {
	backoff := eventsInitialBackoff
	options := events.ListOptions{Filters: containerEventFilters()}

	for {
		messages, errs := cl.Events(ctx, options)

	receive:
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-messages:
				backoff = eventsInitialBackoff
				event, ok := mapContainerEvent(hostName, msg)
				if !ok {
					continue
				}
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			case err := <-errs:
				if ctx.Err() != nil {
					return
				}
				log.Printf("Warning: event stream error for host %s: %v (resubscribing in %s)", hostName, err, backoff)
				break receive
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, eventsMaxBackoff)
	}
}
