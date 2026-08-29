package cmd

// Definitions of the `agx agent` subcommands. Each RunE only parses its flags
// and hands off: argument scanning lives in flags.go, daemon startup in
// daemon_client.go, and the actual work in internal/agent.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedipunkz/agx/internal/agent"
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
		socketPath, err := socketWithDaemon()
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
		socketPath, err := getSocketPath()
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
		socketPath, err := socketWithDaemon()
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
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}
		if follow {
			if err := ensureDaemon(socketPath); err != nil {
				return fmt.Errorf("could not start daemon: %w", err)
			}
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
		socketPath, err := socketWithDaemon()
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
		socketPath, err := socketWithDaemon()
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
