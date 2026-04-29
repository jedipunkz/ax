package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// DiffWorktree prints the diff between the current HEAD of the repository (at cwd)
// and the tip of the agent's worktree branch.
//
// When unified is false (default), git diff handles colour and paging automatically.
// When unified is true, diff -ru is used to produce plain unified diff output; the
// result is colourised and piped through a pager when stdout is a terminal.
func DiffWorktree(idOrName string, unified bool) error {
	ag, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ag.WorktreeBranch == "" {
		return fmt.Errorf("agent %q has no associated worktree branch", idOrName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory: %w", err)
	}

	repoRoot, ok := detectGitRepo(cwd)
	if !ok {
		return fmt.Errorf("current directory is not inside a git repository")
	}

	if unified {
		return diffUnified(repoRoot, idOrName, ag.WorkDir)
	}

	// Three-dot diff: changes on worktree branch since it diverged from HEAD.
	cmd := exec.Command("git", "-c", "color.ui=always", "diff", "HEAD..."+ag.WorktreeBranch)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func diffUnified(repoRoot, idOrName, workDir string) error {
	if workDir == "" {
		return fmt.Errorf("agent %q has no working directory", idOrName)
	}

	// diff exits 0 (identical), 1 (differ), or 2 (error).
	diffCmd := exec.Command("diff", "-ru", "--exclude=.git", repoRoot, workDir)
	out, err := diffCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// exit 1 means files differ — normal result
		} else {
			return fmt.Errorf("diff failed: %w", err)
		}
	}

	if len(out) == 0 {
		return nil
	}

	colored := colorDiff(string(out))

	if term.IsTerminal(int(os.Stdout.Fd())) {
		return runPager(colored)
	}
	_, err = fmt.Fprint(os.Stdout, colored)
	return err
}

// colorDiff applies ANSI colour codes to unified diff output.
func colorDiff(s string) string {
	const (
		reset = "\033[0m"
		bold  = "\033[1m"
		red   = "\033[31m"
		green = "\033[32m"
		cyan  = "\033[36m"
	)

	lines := strings.Split(s, "\n")
	var buf strings.Builder
	buf.Grow(len(s) + len(lines)*12)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			buf.WriteString(bold + line + reset)
		case strings.HasPrefix(line, "diff "):
			buf.WriteString(bold + line + reset)
		case strings.HasPrefix(line, "@@"):
			buf.WriteString(cyan + line + reset)
		case strings.HasPrefix(line, "-"):
			buf.WriteString(red + line + reset)
		case strings.HasPrefix(line, "+"):
			buf.WriteString(green + line + reset)
		default:
			buf.WriteString(line)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

// runPager pipes content through the user's preferred pager.
// Falls back to less with -R to preserve ANSI colour codes.
func runPager(content string) error {
	pager := os.Getenv("GIT_PAGER")
	if pager == "" {
		pager = os.Getenv("PAGER")
	}
	if pager == "" {
		pager = "less"
	}

	parts := strings.Fields(pager)
	args := parts[1:]

	// Ensure less renders ANSI colour codes.
	if filepath.Base(parts[0]) == "less" {
		hasR := false
		for _, a := range args {
			if strings.HasPrefix(a, "-") && strings.ContainsRune(a, 'R') {
				hasR = true
				break
			}
		}
		if !hasR {
			args = append([]string{"-R"}, args...)
		}
	}

	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
