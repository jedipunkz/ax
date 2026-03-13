package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/thirai/cco/internal/store"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Start the state manager daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory: %w", err)
		}

		ccoDir := filepath.Join(home, ".cco")
		if err := os.MkdirAll(ccoDir, 0755); err != nil {
			return fmt.Errorf("could not create ~/.cco dir: %w", err)
		}

		socketPath := filepath.Join(ccoDir, "cco.sock")
		stateFilePath := filepath.Join(ccoDir, "state.json")
		pidFilePath := filepath.Join(ccoDir, "daemon.pid")

		// Remove stale socket if it exists
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not remove stale socket: %w", err)
		}

		// Write PID file
		pid := os.Getpid()
		_ = os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644)
		defer os.Remove(pidFilePath)

		return store.RunManager(socketPath, stateFilePath)
	},
}
