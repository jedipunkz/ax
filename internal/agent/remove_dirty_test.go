package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
)

// newWorktreeUnder creates a real linked worktree under home/.ax/worktrees so
// the removal path exercises git, not just os.RemoveAll.
func newWorktreeUnder(t *testing.T, home string) (paths axfs.Paths, worktree string) {
	t.Helper()
	repo := initDiffTestRepo(t)
	paths = axfs.NewForHome(home)
	worktree = paths.WorktreePath("repo", "ax-1-aaaa")
	if err := os.MkdirAll(paths.WorktreesDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, repo, "worktree", "add", "-q", "-b", "ax/test", worktree, "HEAD")
	return paths, worktree
}

func TestWorktreeIsDirty(t *testing.T) {
	_, worktree := newWorktreeUnder(t, t.TempDir())

	if WorktreeIsDirty(worktree) {
		t.Fatal("WorktreeIsDirty() = true on a freshly created worktree, want false")
	}

	if err := os.WriteFile(filepath.Join(worktree, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !WorktreeIsDirty(worktree) {
		t.Error("WorktreeIsDirty() = false with an untracked file, want true")
	}
}

func TestRemoveWorktreeKeepsDirtyWorktreeWithoutForce(t *testing.T) {
	_, worktree := newWorktreeUnder(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(worktree, "a.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RemoveWorktree(worktree, false)
	if !errors.Is(err, ErrWorktreeDirty) {
		t.Fatalf("RemoveWorktree(force=false) = %v, want ErrWorktreeDirty", err)
	}
	if _, statErr := os.Stat(worktree); statErr != nil {
		t.Fatalf("worktree was removed despite the dirty guard: %v", statErr)
	}

	if err := RemoveWorktree(worktree, true); err != nil {
		t.Fatalf("RemoveWorktree(force=true) = %v, want nil", err)
	}
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("worktree still present after forced removal: %v", statErr)
	}
}

func TestRemoveAgentArtifactsKeepsLogWhenWorktreeIsDirty(t *testing.T) {
	home := t.TempDir()
	paths, worktree := newWorktreeUnder(t, home)
	if err := os.WriteFile(filepath.Join(worktree, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := paths.AgentLog("ax-1-aaaa")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("output\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ag := store.AgentState{ID: "ax-1-aaaa", WorkDir: worktree, LogFile: logPath}

	err := RemoveAgentArtifacts(paths, ag, false)
	if !errors.Is(err, ErrWorktreeDirty) {
		t.Fatalf("RemoveAgentArtifacts(force=false) = %v, want ErrWorktreeDirty", err)
	}
	// Nothing may be removed, so a retry after committing still has everything.
	if _, statErr := os.Stat(worktree); statErr != nil {
		t.Errorf("worktree removed: %v", statErr)
	}
	if _, statErr := os.Stat(logPath); statErr != nil {
		t.Errorf("log file removed even though the worktree was kept: %v", statErr)
	}

	if err := RemoveAgentArtifacts(paths, ag, true); err != nil {
		t.Fatalf("RemoveAgentArtifacts(force=true) = %v, want nil", err)
	}
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Error("worktree still present after forced removal")
	}
	if _, statErr := os.Stat(logPath); !os.IsNotExist(statErr) {
		t.Error("log file still present after forced removal")
	}
}

func TestRemoveWorktreeAllowsCleanWorktreeWithoutForce(t *testing.T) {
	_, worktree := newWorktreeUnder(t, t.TempDir())

	if err := RemoveWorktree(worktree, false); err != nil {
		t.Fatalf("RemoveWorktree(clean, force=false) = %v, want nil", err)
	}
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Error("clean worktree was not removed")
	}
}
