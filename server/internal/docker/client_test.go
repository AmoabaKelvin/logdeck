package docker

import (
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

func TestHealthFromStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"healthy", "Up 3 hours (healthy)", "healthy"},
		{"unhealthy", "Up 2 minutes (unhealthy)", "unhealthy"},
		{"starting", "Up 5 seconds (health: starting)", "starting"},
		{"no healthcheck running", "Up 3 hours", ""},
		{"no healthcheck exited", "Exited (0) 2 hours ago", ""},
		{"empty status", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthFromStatus(tt.status); got != tt.want {
				t.Errorf("healthFromStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// TestConfiguredHostWinsOverDockerHostEnv guards a footgun: the Docker SDK's
// FromEnv option applies DOCKER_HOST, and the last option wins. Applied after
// the configured host it would silently point every non-SSH host at the same
// socket, collapsing a multi-host setup — and the shipped compose file sets
// DOCKER_HOST.
func TestConfiguredHostWinsOverDockerHostEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")

	hosts := []config.DockerHost{
		{Name: "a", Host: "tcp://10.0.0.1:2375"},
		{Name: "b", Host: "tcp://10.0.0.2:2375"},
	}
	multi, err := NewMultiHostClient(hosts)
	if err != nil {
		t.Fatalf("NewMultiHostClient: %v", err)
	}
	defer multi.Close()

	for _, host := range hosts {
		cl, err := multi.GetClient(host.Name)
		if err != nil {
			t.Fatalf("GetClient(%s): %v", host.Name, err)
		}
		if got := cl.DaemonHost(); got != host.Host {
			t.Errorf("host %s: DOCKER_HOST overrode the configured host: got %s, want %s",
				host.Name, got, host.Host)
		}
	}
}
