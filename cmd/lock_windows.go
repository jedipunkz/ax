//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// acquireDaemonLock takes an exclusive lock on path without blocking. The lock
// is tied to the returned file's handle, so Windows releases it automatically
// when the process exits — a crashed daemon never leaves the lock stuck.
//
// Returns errDaemonLocked when another process already holds it.
func acquireDaemonLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open lock file: %w", err)
	}
	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &overlapped,
	)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, errDaemonLocked
		}
		return nil, fmt.Errorf("could not lock %s: %w", path, err)
	}
	return f, nil
}
