package agent

import (
	"fmt"
	"os/exec"
)

// gitOutput runs git with args in dir and returns its stdout. dir may be
// empty to run in the caller's current working directory.
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// gitRun runs git with args in dir when only success or failure matters.
// A failure carries git's combined output so callers can surface it.
func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}
