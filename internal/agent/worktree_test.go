package agent

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWorktreeSetupErrorNamesUnbornHead(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	err := worktreeSetupError(dir, errors.New("fatal: invalid reference: HEAD"))
	if err == nil {
		t.Fatal("worktreeSetupError() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "no commits yet") {
		t.Errorf("worktreeSetupError() = %q, want it to name the unborn HEAD as the cause", err)
	}
	if !strings.Contains(err.Error(), "initial commit") {
		t.Errorf("worktreeSetupError() = %q, want an actionable hint", err)
	}
}

func TestWorktreeSetupErrorRefusesToRunInRepo(t *testing.T) {
	dir := initDiffTestRepo(t) // has one commit, so HEAD resolves

	err := worktreeSetupError(dir, errors.New("boom"))
	if err == nil {
		t.Fatal("worktreeSetupError() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("worktreeSetupError() = %q, want the underlying git error preserved", err)
	}
	if !strings.Contains(err.Error(), "will not run an agent directly in your repository") {
		t.Errorf("worktreeSetupError() = %q, want it to state that ax refuses to fall back", err)
	}
}

func TestSetupWorktreeFailsOnRepoWithoutCommits(t *testing.T) {
	// setupWorktree resolves paths from the home directory; point it at a
	// throwaway one so the test never touches the real ~/.ax.
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// Regression guard for the silent fallback: Run must have an error to
	// propagate here rather than quietly reusing the user's own working tree.
	if _, _, err := setupWorktree("ax-1-abcd", dir, ""); err == nil {
		t.Fatal("setupWorktree() on a repo with no commits = nil error, want failure")
	}
}
