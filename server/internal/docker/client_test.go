package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/docker/docker/client"
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

func TestEachHostStopsWaitingForASilentHost(t *testing.T) {
	defer func(d time.Duration) { hostWait = d }(hostWait)
	hostWait = 50 * time.Millisecond

	c := &MultiHostClient{clients: map[string]*client.Client{"up": nil, "down": nil, "broken": nil}}
	release := make(chan struct{})
	defer close(release)

	results, hostErrors := eachHost(context.Background(), c, func(ctx context.Context, name string, _ *client.Client) (string, error) {
		switch name {
		case "down":
			<-release // like a request stuck behind the client's negotiation lock
		case "broken":
			return "", errors.New("refused")
		}
		return name + "-ok", nil
	})

	if len(results) != 1 || results["up"] != "up-ok" {
		t.Fatalf("results = %v, want only the host that answered", results)
	}
	if len(hostErrors) != 2 {
		t.Fatalf("hostErrors = %v, want the failed and the silent host", hostErrors)
	}
}
