package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectGitRepo returns the repository root if dir is inside a git repository.
func detectGitRepo(dir string) (repoRoot string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// setupWorktree creates a git worktree for the given agent under ~/.ax/worktrees/.
// Returns the worktree path and branch name on success.
func setupWorktree(agentID, repoRoot string) (worktreePath, branchName string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("could not determine home directory: %w", err)
	}

	repoName := filepath.Base(repoRoot)
	worktreePath = filepath.Join(home, ".ax", "worktrees", repoName+"-"+agentID)
	branchName = "ax/" + agentID

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", "", fmt.Errorf("could not create worktrees dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, "HEAD")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git worktree add failed: %w\n%s", err, out)
	}

	return worktreePath, branchName, nil
}
