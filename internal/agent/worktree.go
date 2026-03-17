package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// detectGitRepo returns the repository root if dir is inside a git repository.
func detectGitRepo(dir string) (repoRoot string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// sanitizeBranchName converts a human-readable name into a valid git branch name.
// Spaces become hyphens; characters invalid in branch names are dropped.
// If the result is empty, an empty string is returned (caller should fall back).
func sanitizeBranchName(name string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range name {
		switch {
		case r == ' ' || r == '_':
			r = '-'
		case r == '~' || r == '^' || r == ':' || r == '?' ||
			r == '*' || r == '[' || r == '\\' ||
			unicode.IsControl(r):
			continue
		}
		// Skip consecutive hyphens
		if r == '-' && prev == '-' {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return strings.Trim(b.String(), "-/.")
}

// setupWorktree creates a git worktree for the given agent under ~/.ax/worktrees/.
// branchHint, when non-empty, is used as the branch name (sanitized); otherwise
// the branch is named "ax/<agentID>".
// Returns the worktree path and branch name on success.
func setupWorktree(agentID, repoRoot, branchHint string) (worktreePath, branchName string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("could not determine home directory: %w", err)
	}

	repoName := filepath.Base(repoRoot)
	worktreePath = filepath.Join(home, ".ax", "worktrees", repoName+"-"+agentID)

	if hint := sanitizeBranchName(branchHint); hint != "" {
		branchName = hint
	} else {
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
