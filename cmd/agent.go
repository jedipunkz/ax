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

	"github.com/jedipunkz/ax/internal/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI coding agents",
}

var agentNewCmd = &cobra.Command{
	Use:                "new [-a <agent>] [-n <name>] [-- <agent-args>...]",
	Short:              "Start a new agent session (e.g. -a claude, -a codex, -a gemini, -a opencode)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		agentType, name, rest, err := parseAgentTypeAndNameFlag(args)
		if err != nil {
			return err
		}
		return agent.Run(rest, socketPath, name, agentType)
	},
}

var agentCdCmd = &cobra.Command{
	Use:                "cd -n <id|name>",
	Short:              "Open a new shell in the agent's worktree directory",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, _, err := parseNameFlagRequired(args)
		if err != nil {
			return err
		}
		return agent.CdToWorktreeDir(idOrName)
	},
}

var agentRmCmd = &cobra.Command{
	Use:                "remove -n <id|name>",
	Aliases:            []string{"rm"},
	Short:              "Remove a terminated agent's worktree and state entry",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, _, err := parseNameFlagRequired(args)
		if err != nil {
			return err
		}
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}
		return agent.RemoveAgent(idOrName, socketPath)
	},
}

var agentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all agent worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		return agent.ListWorktrees()
	},
}

var agentResumeCmd = &cobra.Command{
	Use:                "resume [-a <agent>] -n <id|name> [-- <agent-args>...]",
	Short:              "Resume a previous agent session by ID or name (-a overrides stored agent type)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		// agentType is "" when not explicitly provided; ResumeByIDOrName falls
		// back to the agent type stored in state when override is empty.
		agentType, idOrName, rest, err := parseAgentTypeAndNameFlag(args)
		if err != nil {
			return err
		}
		if idOrName == "" {
			return fmt.Errorf("requires -n/--name to specify the agent ID or name")
		}
		return agent.ResumeByIDOrName(rest, socketPath, idOrName, agentType)
	},
}

var agentDiffCmd = &cobra.Command{
	Use:                "diff -n <id|name>",
	Short:              "Show git diff for all commits recorded by the agent",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, _, err := parseNameFlagRequired(args)
		if err != nil {
			return err
		}
		return agent.ShowDiff(idOrName)
	},
}

func init() {
	agentCmd.AddCommand(agentNewCmd)
	agentCmd.AddCommand(agentCdCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentResumeCmd)
	agentCmd.AddCommand(agentRmCmd)
	agentCmd.AddCommand(agentDiffCmd)
}

// parseAgentTypeAndNameFlag extracts -a/-m/--agent and -n/--name from args.
// agentType is empty when neither flag is given; callers apply their own default.
// Returns an error if the agent type contains path separators or spaces.
func parseAgentTypeAndNameFlag(args []string) (agentType string, name string, rest []string, err error) {
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
		case (args[i] == "-a" || args[i] == "-m" || args[i] == "--agent") && i+1 < len(args):
			candidate := args[i+1]
			if strings.ContainsAny(candidate, "/ \\") {
				err = fmt.Errorf("invalid agent type %q: must be a plain binary name", candidate)
				return
			}
			agentType = candidate
			i += 2
		case strings.HasPrefix(args[i], "--agent="):
			candidate := strings.TrimPrefix(args[i], "--agent=")
			if strings.ContainsAny(candidate, "/ \\") {
				err = fmt.Errorf("invalid agent type %q: must be a plain binary name", candidate)
				return
			}
			agentType = candidate
			i++
		default:
			rest = append(rest, args[i])
			i++
		}
	}
	return
}

// parseNameFlag extracts -n/--name from args (before any -- separator).
// Unrecognised flags and positional arguments are returned in rest.
func parseNameFlag(args []string) (name string, rest []string) {
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

// parseNameFlagRequired is like parseNameFlag but returns an error if -n/--name is absent.
func parseNameFlagRequired(args []string) (name string, rest []string, err error) {
	name, rest = parseNameFlag(args)
	if name == "" {
		err = fmt.Errorf("requires -n/--name to specify the agent ID or name")
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
	conn, err := net.DialTimeout("unix", socketPath, 50*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
