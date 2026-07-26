package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ax",
	Short: "Manage multiple Claude Code agents",
}

func Execute() {
	rootCmd.SilenceUsage = true
	// Cobra prints "Error: <err>" itself unless silenced, which would duplicate
	// the message we print below — noticeably so for multi-line errors.
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(dashCmd)
}
