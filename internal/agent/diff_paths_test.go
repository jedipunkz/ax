package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Paths git C-quotes unless told otherwise. Each one used to disappear from the
// worktree diff, which is the surface a user reviews an agent's work through —
// a missing file there reads as "the agent did not touch anything".
var quotedPathNames = []string{
	"plain.txt",
	"with space.txt",
	"日本語.txt",
	"quote\"name.txt",
}

func writeAll(t *testing.T, dir string, names []string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write %q: %v", n, err)
		}
	}
}

func TestUntrackedFilesReturnsUnquotedPaths(t *testing.T) {
	dir := initDiffTestRepo(t)
	writeAll(t, dir, quotedPathNames)

	got := untrackedFiles(dir)
	for _, want := range quotedPathNames {
		if !slices.Contains(got, want) {
			t.Errorf("untrackedFiles() = %q, missing %q", got, want)
		}
	}
}

func TestUntrackedDiffRendersEveryName(t *testing.T) {
	dir := initDiffTestRepo(t)
	writeAll(t, dir, quotedPathNames)

	// Every listed name must produce a diff section. Previously the quoted name
	// git reported could not be opened again, so the section came back empty.
	for _, n := range untrackedFiles(dir) {
		if untrackedDiff(dir, n) == "" {
			t.Errorf("untrackedDiff(%q) = empty, want a new-file diff", n)
		}
	}
}

func TestWorktreeDiffShowsNonASCIIUntrackedPathsLiterally(t *testing.T) {
	dir := initDiffTestRepo(t)
	writeAll(t, dir, quotedPathNames)

	diff, err := WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}

	// core.quotePath=false keeps non-ASCII readable. A name containing a double
	// quote is escaped by git no matter what, so it is only checked for presence
	// via TestUntrackedDiffRendersEveryName above.
	for _, want := range []string{"plain.txt", "with space.txt", "日本語.txt"} {
		if !strings.Contains(diff, want) {
			t.Errorf("WorktreeDiff() omits untracked file %q", want)
		}
	}
}

func TestWorktreeDiffKeepsNonASCIIPathInHeader(t *testing.T) {
	dir := initDiffTestRepo(t)

	name := "日本語.txt"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", ".")
	gitInDir(t, dir, "commit", "-q", "-m", "add non-ascii")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := WorktreeDiff(dir, nil)
	if err != nil {
		t.Fatalf("WorktreeDiff: %v", err)
	}

	// The header must stay parseable, otherwise the section gets no mtime and
	// sorts last no matter how recently it changed.
	var found bool
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "diff --git ") && diffHeaderPath(l) == name {
			found = true
		}
	}
	if !found {
		t.Errorf("no diff --git header resolved to %q; diff was:\n%s", name, diff)
	}
}
