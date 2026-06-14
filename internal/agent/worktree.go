package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jedipunkz/ax/internal/axfs"
)

// detectGitRepo returns the repository root if dir is inside a git repository.
func detectGitRepo(dir string) (repoRoot string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// detectGitRepoAndHead returns the repository root and HEAD commit SHA in a
// single git invocation, avoiding a second subprocess call for gitHeadCommit.
func detectGitRepoAndHead(dir string) (repoRoot, headCommit string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel", "HEAD").Output()
	if err != nil {
		return "", "", false
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) != 2 {
		return "", "", false
	}
	return lines[0], lines[1], true
}

// branchExists reports whether a branch with the given name exists in the repo.
func branchExists(repoRoot, branchName string) bool {
	out, err := exec.Command("git", "-C", repoRoot, "branch", "--list", branchName).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// sanitizeBranchName converts a human-readable name into a valid git branch name.
// Spaces are replaced with hyphens; characters illegal in git refs are removed.
func sanitizeBranchName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '/' || r == '-' || r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		}
	}
	s := b.String()
	// Trim leading/trailing slashes and dots.
	s = strings.Trim(s, "/.")
	return s
}

// setupWorktree creates a git worktree for the given agent under ~/.ax/worktrees/.
// branchHint, if non-empty, is used as the branch name (after sanitization);
// otherwise the branch defaults to "ax/<agentID>".
// Returns the worktree path and branch name on success.
func setupWorktree(agentID, repoRoot, branchHint string) (worktreePath, branchName string, err error) {
	paths, err := axfs.New()
	if err != nil {
		return "", "", err
	}

	repoName := filepath.Base(repoRoot)
	worktreePath = paths.WorktreePath(repoName, agentID)
	// Use the hint-based name only when it's available and not already taken;
	// agentID-based names are always unique so they never need a collision check.
	if branchHint != "" {
		if s := sanitizeBranchName(branchHint); s != "" && !branchExists(repoRoot, s) {
			branchName = s
		}
	}
	if branchName == "" {
		branchName = "ax/" + agentID
	}

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
