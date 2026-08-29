package cmd

// Client-side daemon control: locating the socket, starting the daemon on
// demand, and replacing a stale one. The daemon process itself lives in
// daemon.go.

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jedipunkz/agx/internal/agxfs"
	"github.com/jedipunkz/agx/internal/store"
)

// socketWithDaemon returns the daemon socket path, starting the daemon first
// if it is not already serving. Every subcommand that talks to the daemon
// opens with this.
func socketWithDaemon() (string, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return "", err
	}
	if err := ensureDaemon(socketPath); err != nil {
		return "", fmt.Errorf("could not start daemon: %w", err)
	}
	return socketPath, nil
}

func getSocketPath() (string, error) {
	return agxfs.Socket()
}

func ensureDaemon(socketPath string) error {
	// Check if socket exists and is connectable
	if isSocketAlive(socketPath) {
		// Restart daemon if binary has been updated since daemon started
		if isBinaryNewerThanSocket(socketPath) {
			killDaemon()
			// Fall through to start a new daemon
		} else {
			return nil
		}
	}

	// Fork daemon process
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	daemonCmd := exec.Command(exe, "daemon")
	daemonCmd.Stdout = nil
	daemonCmd.Stderr = nil
	daemonCmd.Stdin = nil
	setDaemonSysProcAttr(daemonCmd)
	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("could not start daemon: %w", err)
	}

	// Wait up to 3 seconds for socket to appear using exponential backoff.
	wait := 10 * time.Millisecond
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if isSocketAlive(socketPath) {
			return nil
		}
		time.Sleep(wait)
		if wait < 500*time.Millisecond {
			wait *= 2
		}
	}

	return fmt.Errorf("daemon did not start within 3 seconds")
}

// isBinaryNewerThanSocket returns true if the current executable was modified
// after the socket file was created, indicating the daemon is stale.
func isBinaryNewerThanSocket(socketPath string) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exeInfo, err := os.Stat(exe)
	if err != nil {
		return false
	}
	sockInfo, err := os.Stat(socketPath)
	if err != nil {
		return false
	}
	return exeInfo.ModTime().After(sockInfo.ModTime())
}

// killDaemon terminates the running daemon via its PID file and waits for the
// process to actually exit, because the replacement daemon can only take the
// lock once the kernel has released the old one's.
//
// The socket file is deliberately left in place: the replacement removes it
// after acquiring the lock. Removing it here would open a window in which no
// socket exists even though no replacement is guaranteed to start.
func killDaemon() {
	paths, err := agxfs.New()
	if err != nil {
		return
	}
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return
	}
	killPID(pid)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && store.IsPIDAlive(pid) {
		time.Sleep(20 * time.Millisecond)
	}
}

func isSocketAlive(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 50*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
