package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initDiffTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-q", "-m", "init")
	return dir
}

func gitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestWorktreeDiffEmpty(t *testing.T) {
	dir := initDiffTestRepo(t)
	got, err := WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty diff for clean worktree, got %q", got)
	}
}

func TestWorktreeDiffUncommittedAndUntracked(t *testing.T) {
	dir := initDiffTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("brand new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	if !strings.Contains(got, "+world") {
		t.Errorf("expected uncommitted change in diff, got:\n%s", got)
	}
	if !strings.Contains(got, "b.txt") || !strings.Contains(got, "+brand new") {
		t.Errorf("expected untracked file rendered as new-file diff, got:\n%s", got)
	}
}

func TestWorktreeDiffWithCommits(t *testing.T) {
	dir := initDiffTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\ncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "a.txt")
	gitInDir(t, dir, "commit", "-q", "-m", "change")
	hash := gitInDir(t, dir, "rev-parse", "HEAD")

	got, err := WorktreeDiff(dir, []string{hash})
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	if !strings.Contains(got, "+committed") {
		t.Errorf("expected committed change in diff, got:\n%s", got)
	}
}

func TestWorktreeDiffNewestFileFirst(t *testing.T) {
	dir := initDiffTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", "b.txt")
	gitInDir(t, dir, "commit", "-q", "-m", "add b")

	// Modify both files, making a.txt the most recently touched one.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("second\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "b.txt"), old, old); err != nil {
		t.Fatal(err)
	}

	got, err := WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	aPos := strings.Index(got, "diff --git a/a.txt")
	bPos := strings.Index(got, "diff --git a/b.txt")
	if aPos < 0 || bPos < 0 {
		t.Fatalf("expected both files in diff, got:\n%s", got)
	}
	if aPos > bPos {
		t.Errorf("most recently modified file should come first; a.txt at %d, b.txt at %d", aPos, bPos)
	}

	// An untracked file touched last must float to the top.
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("newest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}
	cPos := strings.Index(got, "c.txt")
	if cPos < 0 || cPos > strings.Index(got, "diff --git a/a.txt") {
		t.Errorf("newest untracked file should be first, got:\n%s", got)
	}
}

func TestSplitDiffSections(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\n+one\ndiff --git a/y.go b/y.go\n+two\n"
	sections := splitDiffSections(diff)
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].path != "x.go" || sections[1].path != "y.go" {
		t.Errorf("paths = %q, %q; want x.go, y.go", sections[0].path, sections[1].path)
	}
	joined := sections[0].body + sections[1].body
	if joined != diff {
		t.Errorf("sections do not reassemble input:\n%q", joined)
	}
}

func TestWorktreeDiffNoWorkDir(t *testing.T) {
	if _, err := WorktreeDiff("", nil); err == nil {
		t.Error("expected error for empty workDir")
	}
}
