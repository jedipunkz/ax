//go:build !windows

package store

import "syscall"

// isPIDAlive returns true if the given PID refers to a running process.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
