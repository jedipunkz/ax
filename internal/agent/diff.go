package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// DiffWorktree shows the diff between the current HEAD and the agent's worktree branch.
//
// Both modes use the same git branch comparison (HEAD...<worktree-branch>) so the
// scope of changes is identical. The -u flag only changes the output format:
//   - without -u: git diff with colour and paging handled by git
//   - with -u:    same diff, colourised and piped through a pager in plain unified format
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
		return diffUnified(repoRoot, ag.WorktreeBranch)
	}

	// Three-dot diff: changes on worktree branch since it diverged from HEAD.
	// git handles colour and paging automatically.
	cmd := exec.Command("git", "-c", "color.ui=always", "diff", "HEAD..."+ag.WorktreeBranch)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// diffUnified fetches the same git diff without colour, then colourises it and
// pipes it through a pager — producing plain unified diff style output.
func diffUnified(repoRoot, worktreeBranch string) error {
	cmd := exec.Command("git", "--no-pager", "diff", "--no-color", "HEAD..."+worktreeBranch)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git diff failed: %w", err)
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
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
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
