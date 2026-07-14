package docker

import (
	"maps"
	"testing"

	"github.com/docker/docker/api/types/events"
)

func TestMapEngineEventDieCarriesExitCodeAndLabels(t *testing.T) {
	attributes := map[string]string{
		"name":                       "web",
		"exitCode":                   "137",
		"com.docker.compose.project": "demostack",
	}
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "die",
		Actor: events.Actor{
			ID:         "abc123",
			Attributes: attributes,
		},
		Time: 1700000000,
	}

	event, ok := mapEngineEvent("remote", msg)
	if !ok {
		t.Fatal("expected die event to be mapped")
	}
	if event.Host != "remote" {
		t.Fatalf("expected host %q, got %q", "remote", event.Host)
	}
	if event.ContainerID != "abc123" {
		t.Fatalf("expected container id %q, got %q", "abc123", event.ContainerID)
	}
	if event.ContainerName != "web" {
		t.Fatalf("expected container name %q, got %q", "web", event.ContainerName)
	}
	if event.ExitCode != "137" {
		t.Fatalf("expected exit code %q, got %q", "137", event.ExitCode)
	}
	if event.Timestamp != 1700000000 {
		t.Fatalf("expected timestamp 1700000000, got %d", event.Timestamp)
	}
	if !maps.Equal(event.Labels, attributes) {
		t.Fatalf("expected full attribute copy, got %v", event.Labels)
	}
	// The copy must be independent of the source map.
	event.Labels["name"] = "mutated"
	if attributes["name"] != "web" {
		t.Fatal("expected labels to be a copy, not an alias of Actor.Attributes")
	}
}

func TestMapEngineEventDieWithoutExitCode(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "die",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"name": "web"}},
		Time:   1700000000,
	}

	event, ok := mapEngineEvent("local", msg)
	if !ok {
		t.Fatal("expected die event to be mapped")
	}
	if event.ExitCode != "" {
		t.Fatalf("expected empty exit code, got %q", event.ExitCode)
	}
}

func TestMapEngineEventOOMPassesThrough(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "oom",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"name": "web"}},
		Time:   1700000000,
	}

	event, ok := mapEngineEvent("local", msg)
	if !ok {
		t.Fatal("expected oom event to be mapped")
	}
	if event.Action != "oom" {
		t.Fatalf("expected action %q, got %q", "oom", event.Action)
	}
	if event.ExitCode != "" {
		t.Fatalf("expected empty exit code on oom, got %q", event.ExitCode)
	}
}

func TestMapEngineEventHealthStatusDockerSuffix(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "health_status: unhealthy",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"name": "web"}},
		Time:   1700000000,
	}

	event, ok := mapEngineEvent("local", msg)
	if !ok {
		t.Fatal("expected health_status event to be mapped")
	}
	if event.HealthStatus != "unhealthy" {
		t.Fatalf("expected health status %q, got %q", "unhealthy", event.HealthStatus)
	}
}

func TestMapEngineEventHealthStatusPodmanAttribute(t *testing.T) {
	// Podman emits the bare "health_status" action and carries the state in
	// Actor.Attributes instead of the action suffix.
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "health_status",
		Actor: events.Actor{
			ID:         "abc123",
			Attributes: map[string]string{"name": "web", "health_status": "unhealthy"},
		},
		Time: 1700000000,
	}

	event, ok := mapEngineEvent("local", msg)
	if !ok {
		t.Fatal("expected health_status event to be mapped")
	}
	if event.HealthStatus != "unhealthy" {
		t.Fatalf("expected health status %q, got %q", "unhealthy", event.HealthStatus)
	}
}

func TestMapEngineEventSkipsUnwatchedActions(t *testing.T) {
	for _, action := range []events.Action{"create", "pause", "exec_start: bash"} {
		msg := events.Message{
			Type:   events.ContainerEventType,
			Action: action,
			Actor:  events.Actor{ID: "abc123"},
		}
		if _, ok := mapEngineEvent("local", msg); ok {
			t.Fatalf("expected action %q to be skipped", action)
		}
	}
}

func TestMapEngineEventSkipsNonContainerTypes(t *testing.T) {
	msg := events.Message{
		Type:   events.NetworkEventType,
		Action: "die",
		Actor:  events.Actor{ID: "net1"},
	}
	if _, ok := mapEngineEvent("local", msg); ok {
		t.Fatal("expected non-container event to be skipped")
	}
}

func TestMapEngineEventPerHostTagging(t *testing.T) {
	msg := events.Message{
		Type:   events.ContainerEventType,
		Action: "start",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"name": "web"}},
		Time:   1700000000,
	}

	for _, host := range []string{"local", "prod-1"} {
		event, ok := mapEngineEvent(host, msg)
		if !ok {
			t.Fatal("expected start event to be mapped")
		}
		if event.Host != host {
			t.Fatalf("expected host %q, got %q", host, event.Host)
		}
	}
}
