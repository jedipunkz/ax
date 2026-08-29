package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jedipunkz/agx/internal/agxfs"
)

// detectGitRepo returns the repository root if dir is inside a git repository.
func detectGitRepo(dir string) (repoRoot string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
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
	// git refuses refs beginning with "-", and every git command that receives
	// the branch name would parse it as a flag. Dropping the dashes keeps a name
	// like "-foo" usable instead of failing the whole session; a name made only
	// of dashes becomes empty and the caller falls back to "agx/<agentID>".
	s = strings.TrimLeft(s, "-")
	return s
}

// setupWorktree creates a git worktree for the given agent under ~/.agx/worktrees/.
// branchHint, if non-empty, is used as the branch name (after sanitization);
// otherwise the branch defaults to "agx/<agentID>".
// Returns the worktree path and branch name on success.
func setupWorktree(agentID, repoRoot, branchHint string) (worktreePath, branchName string, err error) {
	paths, err := agxfs.New()
	if err != nil {
		return "", "", err
	}

	repoName := filepath.Base(repoRoot)
	worktreePath = paths.WorktreePath(repoName, agentID)
	if branchHint != "" {
		if s := sanitizeBranchName(branchHint); s != "" {
			branchName = s
		}
	}
	if branchName == "" {
		branchName = "agx/" + agentID
	}

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", "", fmt.Errorf("could not create worktrees dir: %w", err)
	}

	// If the desired branch already exists, fall back to a unique branch name.
	if branchExists(repoRoot, branchName) {
		branchName = "agx/" + agentID
	}

	// checkout.workers=0 enables parallel checkout (one worker per CPU core)
	// when populating the worktree. On large repositories the working-tree
	// write is I/O bound and serial by default, so this is the dominant cost
	// of `git worktree add`; parallelising it overlaps the I/O waits and cuts
	// wall time substantially. Scoped to this command only (no global config
	// change); older git (<2.32) silently ignores the unknown key.
	cmd := exec.Command("git", "-c", "checkout.workers=0",
		"worktree", "add", "-b", branchName, worktreePath, "HEAD")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git worktree add failed: %w\n%s", err, out)
	}

	return worktreePath, branchName, nil
}
