package cmd

import (
	"fmt"
	"strings"

	"github.com/jedipunkz/ax/internal/agent"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:                "resume -n <id|name> [-- <claude-args>...]",
	Short:              "Resume a previous agent session by ID or name",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		idOrName, claudeArgs, err := parseFlagsForResume(args)
		if err != nil {
			return err
		}
		return agent.ResumeByIDOrName(claudeArgs, socketPath, idOrName)
	},
}

// parseFlagsForResume extracts -n/--name from args (before any -- separator).
// Returns an error if -n/--name is not provided.
func parseFlagsForResume(args []string) (idOrName string, rest []string, err error) {
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		switch {
		case (args[i] == "-n" || args[i] == "--name") && i+1 < len(args):
			idOrName = args[i+1]
			i += 2
		case strings.HasPrefix(args[i], "--name="):
			idOrName = strings.TrimPrefix(args[i], "--name=")
			i++
		default:
			rest = append(rest, args[i])
			i++
		}
	}
	if idOrName == "" {
		err = fmt.Errorf("resume requires -n/--name to specify the agent ID or name")
	}
	return
}
