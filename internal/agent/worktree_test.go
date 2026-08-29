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
		t.Errorf("worktreeSetupError() = %q, want it to state that agx refuses to fall back", err)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"keeps a slashed branch name", "feat/Foo Bar", "feat/foo-bar"},
		{"drops characters illegal in refs", "feat:foo?*bar", "featfoobar"},
		{"trims surrounding slashes and dots", "/.feat/foo./", "feat/foo"},
		{"drops a leading dash git would reject", "-foo", "foo"},
		{"drops repeated leading dashes", "---feat/foo", "feat/foo"},
		{"keeps inner dashes", "feat/foo-bar", "feat/foo-bar"},
		{"empty when nothing usable remains", "--", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeBranchName(tt.in); got != tt.want {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSetupWorktreeFailsOnRepoWithoutCommits(t *testing.T) {
	// setupWorktree resolves paths from the home directory; point it at a
	// throwaway one so the test never touches the real ~/.agx.
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// Regression guard for the silent fallback: Run must have an error to
	// propagate here rather than quietly reusing the user's own working tree.
	if _, _, err := setupWorktree("agx-1-abcd", dir, ""); err == nil {
		t.Fatal("setupWorktree() on a repo with no commits = nil error, want failure")
	}
}
