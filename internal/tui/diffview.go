package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/ax/internal/store"
)

// diffChromeRows is the number of fixed frame lines around the diff
// viewport: top border, three field lines, section divider, bottom divider,
// help line, and bottom border.
const diffChromeRows = 8

// diffViewportHeight returns the viewport height for the diff view given
// the terminal height.
func diffViewportHeight(h int) int {
	if h < 10 {
		h = 24
	}
	n := h - diffChromeRows
	if n < 3 {
		n = 3
	}
	return n
}

// diffPlaceholder returns the text shown in the viewport before the first
// diff load completes or when the worktree has no changes.
func diffPlaceholder(m Model) string {
	if !m.diffLoaded {
		return "(loading diff...)"
	}
	return "(no changes yet)"
}

// diffViewportContent returns the colorized diff, or a placeholder when
// there is nothing to show.
func diffViewportContent(m Model) string {
	if m.diffContent == "" {
		return diffPlaceholder(m)
	}
	return colorizeDiff(m.diffContent)
}

// diffView renders the live worktree diff view for the selected agent.
func diffView(m Model) string {
	ag, ok := m.findAgent(m.diffAgentID)
	if !ok {
		return "No agent selected."
	}

	width := clampWidth(m.width)
	innerWidth := width - 4

	header := "diff: " + ag.ID
	if ag.Name != "" {
		header = "diff: " + ag.Name + " (" + ag.ID + ")"
	}

	branch := ag.WorktreeBranch
	if branch == "" {
		branch = "-"
	}

	var changes string
	switch {
	case m.diffErr != "":
		changes = "error: " + m.diffErr
	case !m.diffLoaded:
		changes = "loading..."
	default:
		added, deleted, files := diffStat(m.diffContent)
		changes = fmt.Sprintf("%d files  +%d -%d", files, added, deleted)
		if ag.Status == store.StatusRunning {
			changes += fmt.Sprintf("  (auto-refresh %ds)", int(diffPollInterval.Seconds()))
		}
	}

	renderFieldLine := func(label, value string) string {
		styledLabel := OverviewLabelStyle.Render(label)
		styledValue := NormalItemStyle.Render(truncate(value, max(0, innerWidth-lipgloss.Width(styledLabel))))
		return fr("│ ") + padRight(styledLabel+styledValue, innerWidth) + fr(" │")
	}

	var lines []string
	lines = append(lines, fr("╭─ "+header+" "+strings.Repeat("─", max(0, innerWidth-lipgloss.Width(header)-2))+"╮"))
	lines = append(lines, renderFieldLine("Branch : ", branch))
	lines = append(lines, renderFieldLine("Dir    : ", ag.WorkDir))
	lines = append(lines, renderFieldLine("Changes: ", changes))

	title := SectionHeaderStyle.Render("Diff")
	d := max(0, innerWidth-lipgloss.Width(title)-1)
	lines = append(lines, fr("├─ ")+title+fr(" "+strings.Repeat("─", d)+"┤"))

	for _, l := range strings.Split(m.viewport.View(), "\n") {
		lines = append(lines, fr("│ ")+padRight(l, innerWidth)+fr(" │"))
	}

	lines = append(lines, fr("├"+strings.Repeat("─", innerWidth+2)+"┤"))
	help := NormalItemStyle.Render("[esc] back  [↑↓/jk] scroll")
	lines = append(lines, fr("│ ")+padRight(help, innerWidth)+fr(" │"))
	lines = append(lines, fr("╰"+strings.Repeat("─", innerWidth+2)+"╯"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// colorizeDiff applies theme-aware styles to a plain unified diff.
func colorizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	for i, l := range lines {
		lines[i] = colorizeDiffLine(l)
	}
	return strings.Join(lines, "\n")
}

func colorizeDiffLine(l string) string {
	switch {
	case strings.HasPrefix(l, "diff --git"),
		strings.HasPrefix(l, "+++"),
		strings.HasPrefix(l, "---"):
		return DiffFileStyle.Render(l)
	case strings.HasPrefix(l, "@@"):
		return DiffHunkStyle.Render(l)
	case strings.HasPrefix(l, "+"):
		return DiffAddStyle.Render(l)
	case strings.HasPrefix(l, "-"):
		return DiffDelStyle.Render(l)
	case strings.HasPrefix(l, "index "),
		strings.HasPrefix(l, "new file"),
		strings.HasPrefix(l, "deleted file"),
		strings.HasPrefix(l, "old mode"),
		strings.HasPrefix(l, "new mode"),
		strings.HasPrefix(l, "rename "),
		strings.HasPrefix(l, "similarity "),
		strings.HasPrefix(l, "Binary files"):
		return DiffMetaStyle.Render(l)
	default:
		return l
	}
}

// diffStat counts added/deleted lines and changed files in a unified diff.
func diffStat(diff string) (added, deleted, files int) {
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git"):
			files++
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			deleted++
		}
	}
	return added, deleted, files
}
