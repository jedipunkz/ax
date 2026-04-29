package agent

import (
	"fmt"
	"os"
	"os/exec"
)

// DiffWorktree prints the diff between the current HEAD of the repository (at cwd)
// and the tip of the agent's worktree branch.
//
// When unified is false (default), it runs git diff with colour.
// When unified is true, it runs diff -ru on the two directory trees, matching
// the plain unified diff format produced by the diff(1) command.
func DiffWorktree(idOrName string, unified bool) error {
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

	var cmd *exec.Cmd
	if unified {
		if ag.WorkDir == "" {
			return fmt.Errorf("agent %q has no working directory", idOrName)
		}
		// diff -ru exits with 1 when files differ (normal) and 2 on error.
		cmd = exec.Command("diff", "-ru", "--exclude=.git", repoRoot, ag.WorkDir)
	} else {
		// Three-dot diff: changes on worktree branch since it diverged from HEAD.
		cmd = exec.Command("git", "-c", "color.ui=always", "diff", "HEAD..."+ag.WorktreeBranch)
		cmd.Dir = repoRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if unified {
			// Exit code 1 from diff means files differ — that is the expected result.
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return nil
			}
		}
		return err
	}
	return nil
}
