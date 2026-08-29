package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedipunkz/agx/internal/agxfs"
)

func TestAcquireDaemonLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")

	first, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("acquireDaemonLock (first): %v", err)
	}

	if _, err := acquireDaemonLock(path); !errors.Is(err, errDaemonLocked) {
		t.Fatalf("acquireDaemonLock (second) = %v, want errDaemonLocked", err)
	}

	// Closing the file releases the lock so a replacement daemon can start.
	if err := first.Close(); err != nil {
		t.Fatalf("close lock: %v", err)
	}
	second, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("acquireDaemonLock after release: %v", err)
	}
	_ = second.Close()
}

func TestWaitForDaemonLockGivesUpWhenHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")

	held, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("acquireDaemonLock: %v", err)
	}
	defer func() { _ = held.Close() }()

	start := time.Now()
	if _, err := waitForDaemonLock(path, 100*time.Millisecond); !errors.Is(err, errDaemonLocked) {
		t.Fatalf("waitForDaemonLock = %v, want errDaemonLocked", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("waitForDaemonLock returned after %v, want it to retry for the full timeout", elapsed)
	}
}

func TestWaitForDaemonLockSucceedsAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")

	held, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("acquireDaemonLock: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = held.Close()
	}()

	lock, err := waitForDaemonLock(path, 2*time.Second)
	if err != nil {
		t.Fatalf("waitForDaemonLock: %v", err)
	}
	_ = lock.Close()
}

func TestDaemonStartErrorNamesTheLockHolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths := agxfs.NewForHome(home)
	if err := paths.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	// No lock holder: the caller only learns the daemon timed out.
	if err := daemonStartError(time.Second); !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("daemonStartError without holder = %v, want a timeout message", err)
	}

	held, err := acquireDaemonLock(paths.LockFile())
	if err != nil {
		t.Fatalf("acquireDaemonLock: %v", err)
	}
	defer func() { _ = held.Close() }()
	pid := os.Getpid()
	if err := os.WriteFile(paths.PIDFile(), []byte(strconv.Itoa(pid)), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	err = daemonStartError(time.Second)
	if !strings.Contains(err.Error(), strconv.Itoa(pid)) {
		t.Fatalf("daemonStartError = %v, want it to name the holding pid %d", err, pid)
	}
}
