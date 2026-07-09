package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// shortTempDir returns a temp dir with a short path; unix socket paths are
// limited to ~104 chars on macOS, which t.TempDir() subtest paths can exceed.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// listenUnix creates a real unix socket at path and cleans it up with the test.
func listenUnix(t *testing.T, path string) {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("failed to create unix socket %s: %v", path, err)
	}
	t.Cleanup(func() { l.Close() })
}

func TestFirstExistingSocket(t *testing.T) {
	t.Run("returns first candidate that is a socket", func(t *testing.T) {
		dir := shortTempDir(t)
		missing := filepath.Join(dir, "docker.sock")
		podmanSock := filepath.Join(dir, "podman.sock")
		listenUnix(t, podmanSock)

		if got := firstExistingSocket([]string{missing, podmanSock}); got != podmanSock {
			t.Errorf("expected %s, got %s", podmanSock, got)
		}
	})

	t.Run("prefers earlier candidates", func(t *testing.T) {
		dir := shortTempDir(t)
		first := filepath.Join(dir, "first.sock")
		second := filepath.Join(dir, "second.sock")
		listenUnix(t, first)
		listenUnix(t, second)

		if got := firstExistingSocket([]string{first, second}); got != first {
			t.Errorf("expected %s, got %s", first, got)
		}
	})

	t.Run("skips regular files", func(t *testing.T) {
		dir := shortTempDir(t)
		notASocket := filepath.Join(dir, "not-a-socket")
		if err := os.WriteFile(notASocket, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		sock := filepath.Join(dir, "real.sock")
		listenUnix(t, sock)

		if got := firstExistingSocket([]string{notASocket, sock}); got != sock {
			t.Errorf("expected %s, got %s", sock, got)
		}
	})

	t.Run("falls back to docker socket path when nothing exists", func(t *testing.T) {
		missing := filepath.Join(shortTempDir(t), "missing.sock")
		if got := firstExistingSocket([]string{missing}); got != dockerSocketPath {
			t.Errorf("expected %s, got %s", dockerSocketPath, got)
		}
	})
}

func TestLocalSocketCandidatesIncludesRootlessPodman(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	candidates := localSocketCandidates()
	want := []string{
		"/var/run/docker.sock",
		"/run/user/1000/podman/podman.sock",
		"/run/podman/podman.sock",
	}
	if len(candidates) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %v", len(want), len(candidates), candidates)
	}
	for i, w := range want {
		if candidates[i] != w {
			t.Errorf("candidate %d: expected %s, got %s", i, w, candidates[i])
		}
	}
}
