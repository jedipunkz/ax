//go:build !windows

package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// acquireDaemonLock takes an exclusive advisory lock on path without blocking.
// The lock lives on the returned file's open description, so it is held until
// the file is closed and the kernel releases it automatically if the process
// dies — a crashed daemon never leaves the lock stuck.
//
// Returns errDaemonLocked when another process already holds it.
func acquireDaemonLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errDaemonLocked
		}
		return nil, fmt.Errorf("could not lock %s: %w", path, err)
	}
	return f, nil
}
