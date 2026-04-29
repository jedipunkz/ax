package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jedipunkz/ax/internal/store"
)

// CleanupOldWorktrees removes git worktrees for agents that finished more than
// removeDurationDays ago. It reads the agent state from statePath and removes
// worktree directories under worktreesDir that belong to sufficiently old agents.
func CleanupOldWorktrees(statePath, worktreesDir string, removeDurationDays int) error {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read state: %w", err)
	}

	var agents []store.AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return fmt.Errorf("could not parse state: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -removeDurationDays)

	for _, a := range agents {
		if !a.Status.IsTerminal() {
			continue
		}
		if a.FinishedAt == nil || a.FinishedAt.After(cutoff) {
			continue
		}
		if a.WorkDir == "" {
			continue
		}
		// Only remove directories that live under ~/.ax/worktrees/ to avoid
		// accidentally deleting the user's actual working directories.
		cleanWorktrees := filepath.Clean(worktreesDir)
		cleanWorkDir := filepath.Clean(a.WorkDir)
		if !strings.HasPrefix(cleanWorkDir, cleanWorktrees+string(filepath.Separator)) {
			continue
		}
		if _, err := os.Stat(cleanWorkDir); os.IsNotExist(err) {
			continue
		}
		if err := RemoveWorktree(cleanWorkDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", cleanWorkDir, err)
		}
	}

	return nil
}

// RemoveWorktree removes the git worktree at the given path.
// It first attempts a clean removal via "git worktree remove --force" so that
// the main repository's worktree admin data is cleaned up properly. If that
// fails (e.g. the admin entry is already gone after a "git worktree prune"),
// it falls back to os.RemoveAll silently, which is safe because git will
// prune the stale admin entry automatically on the next gc or worktree prune.
func RemoveWorktree(worktreePath string) error {
	mainRepo, err := resolveMainRepo(worktreePath)
	if err == nil && mainRepo != "" {
		cmd := exec.Command("git", "-C", mainRepo, "worktree", "remove", "--force", worktreePath)
		if err := cmd.Run(); err == nil {
			return nil
		}
		// git worktree remove failed (e.g. admin entry already pruned); fall through.
	}

	return os.RemoveAll(worktreePath)
}

// resolveMainRepo reads the .git file inside a linked worktree and returns the
// path to the main repository's git directory's parent (i.e. the working tree
// of the main repository).
func resolveMainRepo(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", err
	}
	// The .git file contains a single line like: gitdir: /path/to/main/.git/worktrees/<name>
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected .git file content: %s", line)
	}
	gitDir := strings.TrimPrefix(line, prefix)

	// Walk up from .git/worktrees/<name> to find the main .git directory,
	// then return its parent (the main working tree).
	// Expected structure: <mainRepo>/.git/worktrees/<agentID>
	worktreesDir := filepath.Dir(gitDir)   // <mainRepo>/.git/worktrees
	dotGitDir := filepath.Dir(worktreesDir) // <mainRepo>/.git
	mainRepo := filepath.Dir(dotGitDir)    // <mainRepo>
	return mainRepo, nil
}
