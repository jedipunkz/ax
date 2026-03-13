package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/thirai/cco/internal/agent"
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

		name, rest := parseNameFlag(args)
		return agent.Run(rest, socketPath, name)
	},
}

// parseNameFlag extracts -n / --name from args (before any -- separator).
func parseNameFlag(args []string) (name string, rest []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if (args[i] == "-n" || args[i] == "--name") && i+1 < len(args) {
			name = args[i+1]
			rest = append(append([]string{}, args[:i]...), args[i+2:]...)
			return
		}
		if strings.HasPrefix(args[i], "--name=") {
			name = strings.TrimPrefix(args[i], "--name=")
			rest = append(append([]string{}, args[:i]...), args[i+1:]...)
			return
		}
	}
	return "", args
}

func getSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".cco", "cco.sock"), nil
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
	// Setsid puts the daemon in a new session so it is detached from the
	// controlling terminal and will not be killed when the user's shell exits.
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
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
		pidFile := filepath.Join(home, ".cco", "daemon.pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGTERM)
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
