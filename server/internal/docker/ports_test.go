package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestPublishedPorts(t *testing.T) {
	// Dual-stack bindings are the common real-world duplicate: the same
	// mapping arrives once for 0.0.0.0 and once for ::.
	got := publishedPorts([]container.Port{
		{IP: "0.0.0.0", PublicPort: 5432, PrivatePort: 5432, Type: "tcp"},
		{IP: "::", PublicPort: 5432, PrivatePort: 5432, Type: "tcp"},
		{PrivatePort: 9000, Type: "tcp"},
		{IP: "0.0.0.0", PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 published ports, got %d: %+v", len(got), got)
	}
	if got[0].PublicPort != 5432 {
		t.Errorf("expected 5432 first (ascending host port), got %+v", got[0])
	}
	if got[1].PublicPort != 8080 || got[1].PrivatePort != 80 {
		t.Errorf("expected 8080->80 second, got %+v", got[1])
	}
}

func TestPublishedPortsDropsExposedOnly(t *testing.T) {
	got := publishedPorts([]container.Port{{PrivatePort: 80, Type: "tcp"}})
	if len(got) != 0 {
		t.Fatalf("exposed-only ports should be dropped, got %+v", got)
	}
}
