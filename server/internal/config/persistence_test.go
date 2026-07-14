package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A bare-metal install must never be warned: a false alarm about data loss on a
// machine that has no such problem is worse than staying quiet.
func TestWarnIfDataIsEphemeralStaysQuietOutsideAContainer(t *testing.T) {
	if inContainer() {
		t.Skip("this test asserts the non-container path; running inside a container")
	}

	dir := t.TempDir()
	if warning := WarnIfDataIsEphemeral(filepath.Join(dir, "config.json")); warning != "" {
		t.Fatalf("warned on a host install: %s", warning)
	}
}

// The mount table is the source of truth: a mounted volume appears in it, a
// directory in the image's writable layer does not.
func TestIsMountPoint(t *testing.T) {
	if _, err := os.Stat("/proc/self/mountinfo"); err != nil {
		t.Skip("no /proc/self/mountinfo on this platform")
	}

	mounted, err := isMountPoint("/proc")
	if err != nil {
		t.Fatalf("isMountPoint(/proc): %v", err)
	}
	if !mounted {
		t.Error("expected /proc to be reported as a mount point")
	}

	mounted, err = isMountPoint(t.TempDir())
	if err != nil {
		t.Fatalf("isMountPoint(tempdir): %v", err)
	}
	if mounted {
		t.Error("expected a plain directory not to be reported as a mount point")
	}
}
