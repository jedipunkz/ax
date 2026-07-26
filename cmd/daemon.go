package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
	"github.com/spf13/cobra"
)

// errDaemonLocked reports that another daemon process already owns the data
// directory. It is not a failure: the incumbent keeps serving on the existing
// socket, so the newcomer exits quietly.
var errDaemonLocked = errors.New("another ax daemon owns this data directory")

// daemonLockWait bounds how long a starting daemon waits for a predecessor to
// release the lock. ensureDaemon kills the old daemon and spawns the new one
// immediately, so the lock can still be held for a moment while the old
// process winds down.
const daemonLockWait = 2 * time.Second

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

		// Take the lock before touching the socket. Every daemon writes
		// state.json from its own in-memory agent map, so two daemons sharing
		// a data directory silently erase each other's agents. Holding the
		// lock for the whole process lifetime makes that impossible.
		lock, err := waitForDaemonLock(paths.LockFile(), daemonLockWait)
		if err != nil {
			if errors.Is(err, errDaemonLocked) {
				return nil
			}
			return err
		}
		defer func() { _ = lock.Close() }()

		socketPath := paths.Socket()
		stateFilePath := paths.StateFile()
		pidFilePath := paths.PIDFile()

		// Remove stale socket if it exists. Safe now that we own the lock:
		// any socket file still present belongs to a daemon that is gone.
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

// waitForDaemonLock retries acquireDaemonLock until timeout elapses, then
// reports errDaemonLocked so the caller can stand down.
func waitForDaemonLock(path string, timeout time.Duration) (*os.File, error) {
	deadline := time.Now().Add(timeout)
	for {
		lock, err := acquireDaemonLock(path)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, errDaemonLocked) || !time.Now().Before(deadline) {
			return nil, err
		}
		time.Sleep(25 * time.Millisecond)
	}
}
