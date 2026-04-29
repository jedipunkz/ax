package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// DiffWorktree compares the repository root directory against the agent's worktree
// directory using git diff --no-index, so both modes cover exactly the same set of files.
//
// Without -u: git handles colour and paging (git-style output).
// With -u:    output is captured, colourised, and piped through a pager (plain diff -u style).
func DiffWorktree(idOrName string, unified bool) error {
	ag, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ag.WorkDir == "" {
		return fmt.Errorf("agent %q has no working directory", idOrName)
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
		return diffUnified(repoRoot, ag.WorkDir)
	}

	// git diff --no-index recursively compares the two directory trees using
	// git's engine; colour and paging are handled by git itself.
	// Exit code 1 means files differ and is not an error.
	cmd := exec.Command("git", "-c", "color.ui=always", "diff", "--no-index", repoRoot, ag.WorkDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}
	return nil
}

// diffUnified runs diff -ru on the two directory trees, then colourises the output
// and pipes it through a pager when stdout is a terminal.
func diffUnified(repoRoot, workDir string) error {
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
