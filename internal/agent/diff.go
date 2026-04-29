package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
			return fmt.Errorf("could not compute diff: %w", err)
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
		cmd.Wait()
		return fmt.Errorf("could not write to pager: %w", err)
	}
	pipe.Close()

	return cmd.Wait()
}
