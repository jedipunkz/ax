package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/jedipunkz/ax/internal/tui"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Show TUI dashboard of all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		if err := ensureDaemon(socketPath); err != nil {
			return fmt.Errorf("could not start daemon: %w", err)
		}

		return tui.Run(socketPath)
	},
}
