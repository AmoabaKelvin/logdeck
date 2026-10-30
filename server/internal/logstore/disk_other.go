//go:build !linux && !darwin

package logstore

import "errors"

// diskFree is not implemented here; free space goes unreported.
func diskFree(string) (int64, error) {
	return 0, errors.New("free disk space is not available on this platform")
}
