package cmd

import (
	"fmt"

	"github.com/jedipunkz/agx/internal/config"
	"github.com/jedipunkz/agx/internal/tui"
	"github.com/spf13/cobra"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Show TUI dashboard of all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := daemonSocket()
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("could not load config: %w", err)
		}

		return tui.Run(socketPath, cfg)
	},
}
