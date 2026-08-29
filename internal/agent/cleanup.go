package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jedipunkz/agx/internal/store"
)

// IsUnderWorktreesDir reports whether workDir lives under worktreesDir.
// Used as a safety check before removing a directory so the user's real
// working directories cannot be deleted by accident.
func IsUnderWorktreesDir(worktreesDir, workDir string) bool {
	if workDir == "" {
		return false
	}
	cleanWorktrees := filepath.Clean(worktreesDir)
	cleanWorkDir := filepath.Clean(workDir)
	return strings.HasPrefix(cleanWorkDir, cleanWorktrees+string(filepath.Separator))
}

// CleanupOldWorktrees removes git worktrees for agents that finished
// more than removeDurationDays ago. It reads the agent state from
// statePath, removes worktree directories under worktreesDir for
// sufficiently old agents, and clears their worktree metadata.
//
// When update is non-nil it is invoked for each cleared state entry so
// the caller can forward the change to the daemon; in that case the
// state file is *not* rewritten directly. When update is nil the
// function persists the updated state file itself.
func CleanupOldWorktrees(statePath, worktreesDir string, removeDurationDays int, update func(store.AgentState) error) error {
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
	changed := false
	clearWorktreeState := func(i int, a store.AgentState) error {
		if a.WorkDir == "" && a.WorktreeBranch == "" {
			return nil
		}

		a.WorkDir = ""
		a.WorktreeBranch = ""
		agents[i] = a
		changed = true

		if update != nil {
			if err := update(a); err != nil {
				return fmt.Errorf("could not update cleaned worktree state for agent %s: %w", a.ID, err)
			}
		}
		return nil
	}

	for i, a := range agents {
		if !a.Status.IsTerminal() {
			continue
		}
		if a.FinishedAt == nil || a.FinishedAt.After(cutoff) {
			continue
		}
		if a.WorkDir == "" {
			continue
		}
		// Only remove directories that live under ~/.agx/worktrees/ to avoid
		// accidentally deleting the user's actual working directories.
		if !IsUnderWorktreesDir(worktreesDir, a.WorkDir) {
			continue
		}
		cleanWorkDir := filepath.Clean(a.WorkDir)
		if _, err := os.Stat(cleanWorkDir); err != nil {
			if os.IsNotExist(err) {
				if err := clearWorktreeState(i, a); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintf(os.Stderr, "warning: could not stat worktree %s: %v\n", cleanWorkDir, err)
			continue
		}
		// Aging cleanup forces removal: these agents finished at least
		// removeDurationDays ago, and skipping dirty worktrees would let them
		// accumulate forever with no way for the user to notice or act.
		if err := RemoveWorktree(cleanWorkDir, true); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", cleanWorkDir, err)
			continue
		}

		if err := clearWorktreeState(i, a); err != nil {
			return err
		}
	}

	if changed && update == nil {
		if err := writeAgents(statePath, agents); err != nil {
			return err
		}
	}

	return nil
}

// ErrWorktreeDirty reports that a worktree still holds changes that removing it
// would destroy: modifications to tracked files, or untracked files. Committed
// work is not at risk — removing a worktree leaves its branch behind — so this
// is specifically about work the agent never committed.
var ErrWorktreeDirty = errors.New("worktree has uncommitted changes")

// WorktreeIsDirty reports whether the worktree at the given path holds tracked
// modifications or untracked files.
//
// A path that is not a readable git worktree counts as clean: there is nothing
// identifiable to preserve, and callers already restrict removal to paths under
// ~/.agx/worktrees/.
func WorktreeIsDirty(worktreePath string) bool {
	c := exec.Command("git", "status", "--porcelain")
	c.Dir = worktreePath
	out, err := c.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// RemoveWorktree removes the git worktree at the given path.
//
// When force is false and the worktree still holds uncommitted changes, it
// returns ErrWorktreeDirty and removes nothing — "git worktree remove --force"
// discards those changes irrecoverably, so the caller has to say it means it.
//
// It first attempts a clean removal via "git worktree remove --force" so that
// the main repository's worktree admin data is cleaned up properly. If that
// fails (e.g. the admin entry is already gone after a "git worktree prune"),
// it falls back to os.RemoveAll silently, which is safe because git will
// prune the stale admin entry automatically on the next gc or worktree prune.
func RemoveWorktree(worktreePath string, force bool) error {
	if !force && WorktreeIsDirty(worktreePath) {
		return fmt.Errorf("%s: %w", worktreePath, ErrWorktreeDirty)
	}

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
	worktreesDir := filepath.Dir(gitDir)    // <mainRepo>/.git/worktrees
	dotGitDir := filepath.Dir(worktreesDir) // <mainRepo>/.git
	mainRepo := filepath.Dir(dotGitDir)     // <mainRepo>
	return mainRepo, nil
}
