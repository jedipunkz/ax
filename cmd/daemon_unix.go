package cmd

import (
	"os/exec"
	"syscall"
)

// setDaemonSysProcAttr puts the daemon in a new session so it is detached from
// the controlling terminal and will not be killed when the user's shell exits.
func setDaemonSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// killPID sends SIGTERM to the given process ID.
func killPID(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
}
