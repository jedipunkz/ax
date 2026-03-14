package tui

import "syscall"

// killProcess sends SIGTERM to the process and its process group.
func killProcess(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
	_ = syscall.Kill(-pid, syscall.SIGTERM)
}
