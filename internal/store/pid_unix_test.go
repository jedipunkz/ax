//go:build !windows

package store

import (
	"os/exec"
	"testing"
	"time"
)

func TestIsPIDAliveTreatsZombieAsDead(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = cmd.Wait()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if isPIDZombie(cmd.Process.Pid) {
			if IsPIDAlive(cmd.Process.Pid) {
				t.Fatalf("IsPIDAlive(%d) = true, want false for zombie process", cmd.Process.Pid)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Skip("child process was reaped or disappeared before it could be observed as a zombie")
}
