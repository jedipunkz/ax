//go:build windows

package store

import "os"

// IsPIDAlive returns true if the given PID refers to a running process.
// On Windows we rely on os.FindProcess; a real liveness check would
// require OpenProcess which we avoid here.
func IsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if _, err := os.FindProcess(pid); err != nil {
		return false
	}
	return true
}

// isPIDAlive is the internal alias used by the manager.
func isPIDAlive(pid int) bool { return IsPIDAlive(pid) }
