package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WarnIfDataIsEphemeral reports a warning when the directory holding the config
// file (and with it the log database and the alert history) lives inside the
// container's own writable layer rather than on a mounted volume. Everything in
// it — Docker hosts, API tokens, alert rules, stored logs — is then discarded
// the moment the container is recreated, which is exactly what an upgrade does.
//
// It returns an empty string when the data is safe, when the check cannot be
// made, or when LogDeck is not running in a container at all: a false alarm on
// a bare-metal install would be worse than no warning.
func WarnIfDataIsEphemeral(configFilePath string) string {
	if !inContainer() {
		return ""
	}

	dir := filepath.Dir(configFilePath)
	mounted, err := isMountPoint(dir)
	if err != nil || mounted {
		return ""
	}

	return fmt.Sprintf(
		"WARNING: %s is not a mounted volume. Your configuration (Docker hosts, "+
			"API tokens, alert rules), stored logs, and alert history are written "+
			"there and will be LOST when this container is recreated, including on "+
			"every upgrade. Mount a volume, e.g. -v logdeck-data:%s",
		dir, dir,
	)
}

// inContainer reports whether the process looks containerized. /.dockerenv is
// written by Docker; Podman writes /run/.containerenv.
func inContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

// isMountPoint reports whether dir is a mount point, read from the kernel's
// mount table. A bind mount and a named volume both appear there; a plain
// directory in the image's writable layer does not.
func isMountPoint(dir string) (bool, error) {
	clean := filepath.Clean(dir)

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// mountinfo: id parent major:minor root mount-point ...
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if filepath.Clean(fields[4]) == clean {
			return true, nil
		}
	}
	return false, scanner.Err()
}
