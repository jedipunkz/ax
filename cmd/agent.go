package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jedipunkz/agx/internal/agent"
	"github.com/jedipunkz/agx/internal/agxfs"
	"github.com/jedipunkz/agx/internal/store"
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
		socketPath, err := daemonSocket()
		if err != nil {
			return err
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
	Use:                "remove [-f] -n <id|name>",
	Aliases:            []string{"rm"},
	Short:              "Remove a terminated agent's worktree and state entry (-f discards uncommitted changes)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, force, _, err := parseNameAndForceFlags(args)
		if err != nil {
			return err
		}
		// No ensureDaemon: RemoveAgent falls back to editing state.json directly
		// when the daemon is not already running.
		socketPath, err := agxfs.Socket()
		if err != nil {
			return err
		}
		return agent.RemoveAgent(idOrName, socketPath, force)
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
		socketPath, err := daemonSocket()
		if err != nil {
			return err
		}

		// agentType is "" when not explicitly provided; ResumeByIDOrName falls
		// back to the agent type stored in state when override is empty.
		agentType, idOrName, rest, err := parseAgentTypeAndNameFlag(args)
		if err != nil {
			return err
		}
		if idOrName == "" {
			return errNameRequired()
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

var agentLogsCmd = &cobra.Command{
	Use:                "logs -n <id|name> [-f]",
	Short:              "Show agent output (use -f to follow new output, ANSI stripped)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, follow, rest, err := parseNameAndFollowFlags(args)
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return fmt.Errorf("unexpected arguments: %v", rest)
		}
		// Only following needs a live daemon; a one-shot read comes from the log file.
		socketPath, err := agxfs.Socket()
		if follow {
			socketPath, err = daemonSocket()
		}
		if err != nil {
			return err
		}
		return agent.StreamLogs(socketPath, idOrName, follow)
	},
}

var agentWaitCmd = &cobra.Command{
	Use:                "wait -n <id|name>",
	Short:              "Block until the agent finishes or waits for input; exit with its code",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, _, err := parseNameFlagRequired(args)
		if err != nil {
			return err
		}
		socketPath, err := daemonSocket()
		if err != nil {
			return err
		}
		result, err := agent.Wait(socketPath, idOrName)
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}
		return nil
	},
}

var agentInputCmd = &cobra.Command{
	Use:                "input -n <id|name> [text]",
	Short:              "Send text to a waiting agent's stdin (reads from stdin if text is omitted)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName, rest, err := parseNameFlagRequired(args)
		if err != nil {
			return err
		}
		var data string
		if len(rest) > 0 {
			data = strings.Join(rest, " ")
		} else {
			buf, readErr := io.ReadAll(os.Stdin)
			if readErr != nil {
				return fmt.Errorf("could not read stdin: %w", readErr)
			}
			data = string(buf)
		}
		if data == "" {
			return fmt.Errorf("no input data provided")
		}
		socketPath, err := daemonSocket()
		if err != nil {
			return err
		}
		return agent.SendInput(socketPath, idOrName, data)
	},
}

func init() {
	agentCmd.AddCommand(agentNewCmd)
	agentCmd.AddCommand(agentCdCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentResumeCmd)
	agentCmd.AddCommand(agentRmCmd)
	agentCmd.AddCommand(agentDiffCmd)
	agentCmd.AddCommand(agentLogsCmd)
	agentCmd.AddCommand(agentWaitCmd)
	agentCmd.AddCommand(agentInputCmd)
}

// The agent subcommands disable cobra flag parsing so that arbitrary flags
// can be forwarded to the agent binary. parseAgentFlags is the shared scanner
// behind the per-command wrappers below: it always extracts -n/--name, and
// optionally -a/-m/--agent and -f/--follow depending on spec. Everything else
// (including a literal "--" and all tokens after it) is left in rest.

// agentFlagSpec selects which optional flags parseAgentFlags recognises.
// Flags outside the spec pass through to rest untouched.
type agentFlagSpec struct {
	agentType bool // -a / -m / --agent / --agent=
	follow    bool // -f / --follow
	force     bool // -f / --force
}

// agentFlags holds the values extracted by parseAgentFlags.
type agentFlags struct {
	name      string
	agentType string
	follow    bool
	force     bool
	rest      []string
}

func parseAgentFlags(args []string, spec agentFlagSpec) (agentFlags, error) {
	var p agentFlags
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			p.rest = append(p.rest, args[i:]...)
			break
		}
		switch {
		case (args[i] == "-n" || args[i] == "--name") && i+1 < len(args):
			p.name = args[i+1]
			i += 2
		case strings.HasPrefix(args[i], "--name="):
			p.name = strings.TrimPrefix(args[i], "--name=")
			i++
		case spec.agentType && (args[i] == "-a" || args[i] == "-m" || args[i] == "--agent") && i+1 < len(args):
			if err := validateAgentType(args[i+1]); err != nil {
				return agentFlags{}, err
			}
			p.agentType = args[i+1]
			i += 2
		case spec.agentType && strings.HasPrefix(args[i], "--agent="):
			candidate := strings.TrimPrefix(args[i], "--agent=")
			if err := validateAgentType(candidate); err != nil {
				return agentFlags{}, err
			}
			p.agentType = candidate
			i++
		case spec.follow && (args[i] == "-f" || args[i] == "--follow"):
			p.follow = true
			i++
		case spec.force && (args[i] == "-f" || args[i] == "--force"):
			p.force = true
			i++
		default:
			p.rest = append(p.rest, args[i])
			i++
		}
	}
	return p, nil
}

// validateAgentType rejects agent types containing path separators or spaces;
// the value is later executed as a bare binary name.
func validateAgentType(candidate string) error {
	if strings.ContainsAny(candidate, "/ \\") {
		return fmt.Errorf("invalid agent type %q: must be a plain binary name", candidate)
	}
	return nil
}

func errNameRequired() error {
	return fmt.Errorf("requires -n/--name to specify the agent ID or name")
}

// parseAgentTypeAndNameFlag extracts -a/-m/--agent and -n/--name from args.
// agentType is empty when neither flag is given; callers apply their own default.
func parseAgentTypeAndNameFlag(args []string) (agentType string, name string, rest []string, err error) {
	p, err := parseAgentFlags(args, agentFlagSpec{agentType: true})
	return p.agentType, p.name, p.rest, err
}

// parseNameFlag extracts -n/--name from args (before any -- separator).
// Unrecognised flags and positional arguments are returned in rest.
func parseNameFlag(args []string) (name string, rest []string) {
	p, _ := parseAgentFlags(args, agentFlagSpec{}) // no validated flags → cannot fail
	return p.name, p.rest
}

// parseNameFlagRequired is like parseNameFlag but returns an error if -n/--name is absent.
func parseNameFlagRequired(args []string) (name string, rest []string, err error) {
	name, rest = parseNameFlag(args)
	if name == "" {
		err = errNameRequired()
	}
	return
}

// parseNameAndForceFlags extracts -n/--name and -f/--force from args.
// The name is required.
func parseNameAndForceFlags(args []string) (name string, force bool, rest []string, err error) {
	p, _ := parseAgentFlags(args, agentFlagSpec{force: true})
	if p.name == "" {
		return "", false, nil, errNameRequired()
	}
	return p.name, p.force, p.rest, nil
}

// parseNameAndFollowFlags extracts -n/--name and -f/--follow from args. The
// name is required. Unlike the other wrappers, the "--" separator itself is
// dropped from rest (its tail is kept) so `agx agent logs` can reject any
// leftover arguments without tripping over the separator.
func parseNameAndFollowFlags(args []string) (name string, follow bool, rest []string, err error) {
	p, _ := parseAgentFlags(args, agentFlagSpec{follow: true})
	if p.name == "" {
		return "", false, nil, errNameRequired()
	}
	rest = p.rest
	if i := slices.Index(rest, "--"); i >= 0 {
		rest = append(rest[:i], rest[i+1:]...)
	}
	if len(rest) == 0 {
		rest = nil
	}
	return p.name, p.follow, rest, nil
}

// daemonSocket returns the daemon socket path, starting the daemon first when
// it is not already running.
func daemonSocket() (string, error) {
	socketPath, err := agxfs.Socket()
	if err != nil {
		return "", err
	}
	if err := ensureDaemon(socketPath); err != nil {
		return "", fmt.Errorf("could not start daemon: %w", err)
	}
	return socketPath, nil
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
