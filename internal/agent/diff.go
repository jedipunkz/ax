package agent

import (
	"fmt"
	"os"
	"os/exec"
)

// DiffWorktree prints the diff between the current HEAD of the repository (at cwd)
// and the tip of the agent's worktree branch, using git's unified diff format with colour.
func DiffWorktree(idOrName string) error {
	ag, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ag.WorktreeBranch == "" {
		return fmt.Errorf("agent %q has no associated worktree branch", idOrName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory: %w", err)
	}

	repoRoot, ok := detectGitRepo(cwd)
	if !ok {
		return fmt.Errorf("current directory is not inside a git repository")
	}

	// Three-dot diff: changes on worktree branch since it diverged from HEAD.
	cmd := exec.Command("git", "-c", "color.ui=always", "diff", "HEAD..."+ag.WorktreeBranch)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
