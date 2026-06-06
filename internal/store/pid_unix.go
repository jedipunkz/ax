//go:build !windows

package store

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// isPIDAlive returns true if the given PID refers to a running process.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}
	return !isPIDZombie(pid)
}

// IsPIDAlive returns true if the given PID refers to a running process.
func IsPIDAlive(pid int) bool {
	return isPIDAlive(pid)
}

func isPIDZombie(pid int) bool {
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	stat := strings.TrimSpace(string(out))
	return strings.HasPrefix(stat, "Z")
}
