package cmd

import (
	"fmt"

	"github.com/jedipunkz/ax/internal/agent"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:                "resume <id|name> [-- <claude-args>...]",
	Short:              "Resume a previous agent session by ID or name",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("resume requires an agent ID or name")
		}

		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		idOrName, claudeArgs := parseResumeArgs(args)
		return agent.ResumeByIDOrName(claudeArgs, socketPath, idOrName)
	},
}

// parseResumeArgs extracts the agent ID/name (first arg) and any pass-through
// claude args (everything after a "--" separator or remaining positional args).
func parseResumeArgs(args []string) (idOrName string, rest []string) {
	idOrName = args[0]
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		rest = append(rest, args[i])
	}
	return
}
