package agent

import (
	"fmt"
	"os"
	"os/exec"
)

// CdToWorktreeDir finds an agent by ID or name and spawns a new interactive
// shell session in its working directory.
func CdToWorktreeDir(idOrName string) error {
	ag, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ag.WorkDir == "" {
		return fmt.Errorf("agent %q has no working directory", idOrName)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Dir = ag.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "entering %s (exit to return)\n", ag.WorkDir)
	return cmd.Run()
}
