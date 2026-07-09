package config

import (
	"os"
	"path/filepath"
)

// dockerSocketPath is the conventional Docker daemon socket. It is probed
// first and is also the fallback when no local socket exists, so connection
// errors point at the location most users expect.
const dockerSocketPath = "/var/run/docker.sock"

// DefaultLocalHost returns the host used when no Docker hosts are configured.
// It probes for a local Docker socket first, then Podman's API sockets
// (rootless, then rootful), so LogDeck works out of the box on Podman-only
// machines. Podman's socket speaks the Docker-compatible API, so no other
// changes are needed to talk to it.
func DefaultLocalHost() DockerHost {
	return DockerHost{Name: "local", Host: "unix://" + firstExistingSocket(localSocketCandidates())}
}

// localSocketCandidates lists local engine sockets in preference order:
// Docker, rootless Podman, rootful Podman.
func localSocketCandidates() []string {
	candidates := []string{dockerSocketPath}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, filepath.Join(runtimeDir, "podman", "podman.sock"))
	}
	return append(candidates, "/run/podman/podman.sock")
}

// firstExistingSocket returns the first candidate that exists and is a unix
// socket, or dockerSocketPath when none match.
func firstExistingSocket(candidates []string) string {
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return path
		}
	}
	return dockerSocketPath
}
