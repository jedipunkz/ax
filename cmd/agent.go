package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/jedipunkz/ax/internal/agent"
)

var agentCmd = &cobra.Command{
	Use:                "agent [-n <name>] [-- <claude-args>...]",
	Short:              "Start a Claude Code agent",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		name, rest := parseFlagsForAgent(args)
		return agent.Run(rest, socketPath, name)
	},
}

// parseFlagsForAgent extracts -n/--name from args (before any -- separator).
// Unrecognised flags and positional arguments are returned in rest.
func parseFlagsForAgent(args []string) (name string, rest []string) {
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		switch {
		case (args[i] == "-n" || args[i] == "--name") && i+1 < len(args):
			name = args[i+1]
			i += 2
		case strings.HasPrefix(args[i], "--name="):
			name = strings.TrimPrefix(args[i], "--name=")
			i++
		default:
			rest = append(rest, args[i])
			i++
		}
	}
	return
}

func getSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".ax", "ax.sock"), nil
}

func ensureDaemon(socketPath string) error {
	// Check if socket exists and is connectable
	if isSocketAlive(socketPath) {
		// Restart daemon if binary has been updated since daemon started
		if isBinaryNewerThanSocket(socketPath) {
			killDaemon(socketPath)
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

	// Wait up to 3 seconds for socket to appear
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if isSocketAlive(socketPath) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
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

// killDaemon kills the running daemon process using the PID file and removes the socket.
func killDaemon(socketPath string) {
	home, err := os.UserHomeDir()
	if err == nil {
		pidFile := filepath.Join(home, ".ax", "daemon.pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				killPID(pid)
			}
		}
	}
	_ = os.Remove(socketPath)
	// Give the old daemon a moment to exit
	time.Sleep(200 * time.Millisecond)
}

func isSocketAlive(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
