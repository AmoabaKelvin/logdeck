//go:build linux || darwin

package logstore

import "syscall"

// diskFree reports the bytes available to unprivileged writers on the
// filesystem holding dir.
func diskFree(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
