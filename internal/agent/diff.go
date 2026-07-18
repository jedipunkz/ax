package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ShowDiff finds an agent by ID or name and displays a colorized git diff
// of all commits recorded in the agent's state, piped through a pager.
func ShowDiff(idOrName string) error {
	ag, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ag.WorkDir == "" {
		return fmt.Errorf("agent %q has no working directory", idOrName)
	}
	if len(ag.Commits) == 0 {
		return showBranchDiff(ag.WorkDir)
	}

	var gitArgs []string
	if len(ag.Commits) == 1 {
		gitArgs = []string{"show", "--color=always", ag.Commits[0]}
	} else {
		first := ag.Commits[0]
		last := ag.Commits[len(ag.Commits)-1]
		gitArgs = []string{"diff", "--color=always", first + "^", last}
	}

	gitCmd := exec.Command("git", gitArgs...)
	gitCmd.Dir = ag.WorkDir

	output, err := gitCmd.Output()
	if err != nil {
		// For root commits, `first^` fails; fall back to showing each commit.
		return showCommitsIndividually(ag.WorkDir, ag.Commits)
	}

	return runPager(output)
}

// maxUntrackedDiffs caps how many untracked files are rendered as new-file
// diffs so a stray build directory cannot blow up the diff output.
const maxUntrackedDiffs = 50

// fileDiff is one per-file section of a unified diff, tagged with the
// file's modification time so sections can be ordered newest-first.
type fileDiff struct {
	path  string
	mtime time.Time
	body  string
}

// WorktreeDiff returns the plain-text unified diff of everything the agent
// changed in its worktree: commits recorded in the agent state plus
// uncommitted (staged and unstaged) changes, with untracked files rendered
// as new-file diffs. Per-file sections are ordered by modification time,
// newest first, so a live view always surfaces the most recently updated
// file at the top. The output carries no ANSI colors so callers can apply
// their own styling.
func WorktreeDiff(workDir string, commits []string) (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("agent has no working directory")
	}

	base := diffBase(workDir, commits)
	c := exec.Command("git", "diff", base, "--")
	c.Dir = workDir
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", base, err)
	}

	sections := splitDiffSections(string(out))

	var truncNote string
	untracked := untrackedFiles(workDir)
	for i, f := range untracked {
		if i >= maxUntrackedDiffs {
			truncNote = fmt.Sprintf("\n(%d more untracked files not shown)\n", len(untracked)-maxUntrackedDiffs)
			break
		}
		if body := untrackedDiff(workDir, f); body != "" {
			sections = append(sections, fileDiff{path: f, body: body})
		}
	}

	for i := range sections {
		if sections[i].path == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(workDir, sections[i].path)); err == nil {
			sections[i].mtime = info.ModTime()
		}
	}
	sort.SliceStable(sections, func(i, j int) bool {
		return sections[i].mtime.After(sections[j].mtime)
	})

	var b strings.Builder
	for _, s := range sections {
		b.WriteString(s.body)
	}
	b.WriteString(truncNote)
	return b.String(), nil
}

// splitDiffSections splits a unified diff into per-file sections on
// "diff --git" boundaries, extracting the b-side path of each section.
// Content before the first header (normally none) is kept as a pathless
// section so nothing is ever dropped.
func splitDiffSections(diff string) []fileDiff {
	if diff == "" {
		return nil
	}
	var sections []fileDiff
	lines := strings.SplitAfter(diff, "\n")
	var cur strings.Builder
	curPath := ""
	flush := func() {
		if cur.Len() > 0 {
			sections = append(sections, fileDiff{path: curPath, body: cur.String()})
			cur.Reset()
		}
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "diff --git ") {
			flush()
			curPath = diffHeaderPath(l)
		}
		cur.WriteString(l)
	}
	flush()
	return sections
}

// diffHeaderPath extracts the b-side path from a "diff --git a/X b/Y"
// header line. Returns "" for headers it cannot parse (e.g. quoted paths);
// those sections simply sort last.
func diffHeaderPath(header string) string {
	header = strings.TrimSuffix(header, "\n")
	idx := strings.LastIndex(header, " b/")
	if idx < 0 {
		return ""
	}
	return header[idx+len(" b/"):]
}

// diffBase picks the revision the worktree is diffed against: the parent of
// the first recorded commit when available, otherwise the merge base with
// the default branch, otherwise HEAD (uncommitted changes only).
func diffBase(workDir string, commits []string) string {
	if len(commits) > 0 {
		parent := commits[0] + "^"
		c := exec.Command("git", "rev-parse", "--verify", "--quiet", parent)
		c.Dir = workDir
		if c.Run() == nil {
			return parent
		}
	}
	if base, err := findMergeBase(workDir); err == nil {
		return base
	}
	return "HEAD"
}

// untrackedFiles lists files that are not tracked by git and not ignored.
func untrackedFiles(workDir string) []string {
	c := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	c.Dir = workDir
	out, err := c.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

// untrackedDiff renders an untracked file as a new-file diff. `git diff
// --no-index` exits with status 1 when the files differ, which is the
// expected outcome here, so exit errors are not propagated.
func untrackedDiff(workDir, file string) string {
	c := exec.Command("git", "diff", "--no-index", "--", os.DevNull, file)
	c.Dir = workDir
	out, _ := c.Output()
	return string(out)
}

// showBranchDiff falls back to `git diff <merge-base>..HEAD` when no commits
// were recorded in the agent state (e.g. the agent has not yet committed).
func showBranchDiff(workDir string) error {
	base, err := findMergeBase(workDir)
	if err != nil {
		// Last resort: show uncommitted working-tree changes.
		c := exec.Command("git", "diff", "--color=always")
		c.Dir = workDir
		out, err2 := c.Output()
		if err2 != nil {
			return fmt.Errorf("could not compute diff: %w", err2)
		}
		return runPager(out)
	}
	c := exec.Command("git", "diff", "--color=always", base, "HEAD")
	c.Dir = workDir
	out, err := c.Output()
	if err != nil {
		return fmt.Errorf("git diff %s HEAD: %w", base, err)
	}
	return runPager(out)
}

func findMergeBase(workDir string) (string, error) {
	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		c := exec.Command("git", "merge-base", "HEAD", ref)
		c.Dir = workDir
		out, err := c.Output()
		if err == nil {
			base := strings.TrimSpace(string(out))
			if base != "" {
				return base, nil
			}
		}
	}
	return "", fmt.Errorf("could not find merge base")
}

func showCommitsIndividually(workDir string, commits []string) error {
	var combined []byte
	for _, commit := range commits {
		c := exec.Command("git", "show", "--color=always", commit)
		c.Dir = workDir
		out, err := c.Output()
		if err != nil {
			return fmt.Errorf("git show %s: %w", commit, err)
		}
		combined = append(combined, out...)
	}
	return runPager(combined)
}

func runPager(content []byte) error {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	cmd := exec.Command(pager, "-R")
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open pager stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Pager not available; write directly to stdout.
		_, writeErr := os.Stdout.Write(content)
		return writeErr
	}

	if _, err := pipe.Write(content); err != nil {
		pipe.Close()
		_ = cmd.Wait()
		return fmt.Errorf("could not write to pager: %w", err)
	}
	pipe.Close()

	return cmd.Wait()
}
