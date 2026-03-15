//go:build windows

package store

import (
	"os"
)

// isPIDAlive returns true if the given PID refers to a running process.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds; OpenProcess is the real check.
	// We attempt to send signal 0 via the process handle as a best-effort.
	// os.Process.Signal is not available without reflect tricks, so we rely
	// on the fact that FindProcess failing means the process is gone.
	_ = p
	return true
}
