//go:build !windows

package store

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// IsPIDAlive returns true if the given PID refers to a running, non-zombie process.
func IsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	return !isPIDZombie(pid)
}

// isPIDAlive is the internal alias used by the manager so existing
// callers keep working without exposing the wrapper.
func isPIDAlive(pid int) bool { return IsPIDAlive(pid) }

func isPIDZombie(pid int) bool {
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	stat := strings.TrimSpace(string(out))
	return strings.HasPrefix(stat, "Z")
}
