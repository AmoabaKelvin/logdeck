package docker

import (
	"testing"

	"github.com/docker/docker/api/types/events"
)

func TestMapContainerEventMapsWatchedActions(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "die",
		Actor: events.Actor{
			ID:         "abc123",
			Attributes: map[string]string{"name": "web", "exitCode": "137"},
		},
		Time: 1700000000,
	}

	event, ok := mapContainerEvent("local", msg)
	if !ok {
		t.Fatal("expected watched container event to be mapped")
	}
	if event.Host != "local" {
		t.Fatalf("expected host %q, got %q", "local", event.Host)
	}
	if event.ContainerID != "abc123" {
		t.Fatalf("expected container id %q, got %q", "abc123", event.ContainerID)
	}
	if event.ContainerName != "web" {
		t.Fatalf("expected container name %q, got %q", "web", event.ContainerName)
	}
	if event.Action != "die" {
		t.Fatalf("expected action %q, got %q", "die", event.Action)
	}
	if event.Timestamp != 1700000000 {
		t.Fatalf("expected timestamp 1700000000, got %d", event.Timestamp)
	}
}

func TestMapContainerEventSkipsNonContainerTypes(t *testing.T) {
	msg := events.Message{
		Type:   events.NetworkEventType,
		Action: "create",
		Actor:  events.Actor{ID: "net1"},
	}

	if _, ok := mapContainerEvent("local", msg); ok {
		t.Fatal("expected non-container event to be skipped")
	}
}

func TestMapContainerEventSkipsUnwatchedActions(t *testing.T) {
	for _, action := range []events.Action{"exec_start: bash", "attach", "health_status: healthy"} {
		msg := events.Message{
			Type:   events.ContainerEventType,
			Action: action,
			Actor:  events.Actor{ID: "abc123"},
		}

		if _, ok := mapContainerEvent("local", msg); ok {
			t.Fatalf("expected action %q to be skipped", action)
		}
	}
}

func TestMapContainerEventFallsBackToTimeNano(t *testing.T) {
	msg := events.Message{
		Type:     events.ContainerEventType,
		Action:   "start",
		Actor:    events.Actor{ID: "abc123"},
		TimeNano: 1700000000_500000000,
	}

	event, ok := mapContainerEvent("local", msg)
	if !ok {
		t.Fatal("expected watched container event to be mapped")
	}
	if event.Timestamp != 1700000000 {
		t.Fatalf("expected timestamp 1700000000, got %d", event.Timestamp)
	}
}

func TestMapContainerEventHandlesMissingNameAttribute(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "destroy",
		Actor:  events.Actor{ID: "abc123"},
		Time:   1700000000,
	}

	event, ok := mapContainerEvent("local", msg)
	if !ok {
		t.Fatal("expected watched container event to be mapped")
	}
	if event.ContainerName != "" {
		t.Fatalf("expected empty container name, got %q", event.ContainerName)
	}
}
