package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Start the state manager daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := axfs.New()
		if err != nil {
			return err
		}
		if err := paths.EnsureDir(); err != nil {
			return err
		}

		socketPath := paths.Socket()
		stateFilePath := paths.StateFile()
		pidFilePath := paths.PIDFile()

		// Remove stale socket if it exists
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not remove stale socket: %w", err)
		}

		// Write PID file
		pid := os.Getpid()
		if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0600); err != nil {
			return fmt.Errorf("could not write pid file: %w", err)
		}
		defer os.Remove(pidFilePath)

		return store.RunManager(socketPath, stateFilePath)
	},
}
