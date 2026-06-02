package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jedipunkz/ax/internal/store"
)

func TestCleanupOldWorktreesClearsRemovedWorktreeState(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "state.json")
	worktreesDir := filepath.Join(tmp, "worktrees")
	oldWorktree := filepath.Join(worktreesDir, "repo-old")
	recentWorktree := filepath.Join(worktreesDir, "repo-recent")
	missingWorktree := filepath.Join(worktreesDir, "repo-missing")

	if err := os.MkdirAll(oldWorktree, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recentWorktree, 0755); err != nil {
		t.Fatal(err)
	}

	oldFinished := time.Now().AddDate(0, 0, -31)
	recentFinished := time.Now().AddDate(0, 0, -1)
	agents := []store.AgentState{
		{
			ID:             "old",
			WorkDir:        oldWorktree,
			WorktreeBranch: "ax/old",
			Status:         store.StatusSuccess,
			FinishedAt:     &oldFinished,
		},
		{
			ID:             "recent",
			WorkDir:        recentWorktree,
			WorktreeBranch: "ax/recent",
			Status:         store.StatusSuccess,
			FinishedAt:     &recentFinished,
		},
		{
			ID:             "missing",
			WorkDir:        missingWorktree,
			WorktreeBranch: "ax/missing",
			Status:         store.StatusSuccess,
			FinishedAt:     &oldFinished,
		},
	}
	writeState(t, statePath, agents)

	if err := CleanupOldWorktrees(statePath, worktreesDir, 30); err != nil {
		t.Fatalf("CleanupOldWorktrees() error = %v", err)
	}

	if _, err := os.Stat(oldWorktree); !os.IsNotExist(err) {
		t.Fatalf("old worktree still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(recentWorktree); err != nil {
		t.Fatalf("recent worktree stat error = %v", err)
	}

	var got []store.AgentState
	readState(t, statePath, &got)
	if got[0].WorkDir != "" {
		t.Fatalf("old WorkDir = %q, want empty", got[0].WorkDir)
	}
	if got[0].WorktreeBranch != "" {
		t.Fatalf("old WorktreeBranch = %q, want empty", got[0].WorktreeBranch)
	}
	if got[1].WorkDir != recentWorktree {
		t.Fatalf("recent WorkDir = %q, want %q", got[1].WorkDir, recentWorktree)
	}
	if got[1].WorktreeBranch != "ax/recent" {
		t.Fatalf("recent WorktreeBranch = %q, want ax/recent", got[1].WorktreeBranch)
	}
	if got[2].WorkDir != "" {
		t.Fatalf("missing WorkDir = %q, want empty", got[2].WorkDir)
	}
	if got[2].WorktreeBranch != "" {
		t.Fatalf("missing WorktreeBranch = %q, want empty", got[2].WorktreeBranch)
	}
}

func writeState(t *testing.T, statePath string, agents []store.AgentState) {
	t.Helper()
	data, err := json.Marshal(agents)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func readState(t *testing.T, statePath string, dest any) {
	t.Helper()
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatal(err)
	}
}
