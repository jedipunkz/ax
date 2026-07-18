package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/ax/internal/textutil"
)

// cleanLog strips ANSI codes, normalizes line endings, and keeps only
// lines that contain readable text (at least 4 alphanumeric characters).
func cleanLog(data []byte) string {
	return textutil.CleanLogReadable(data, "(no readable output yet)")
}

func detailView(m Model) string {
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if len(groups) == 0 || m.cursor >= len(groups) {
		return "No agent selected."
	}

	agent := groups[m.cursor].Rep
	width := clampWidth(m.width)
	innerWidth := width - 4

	statusStr := formatStatus(agent, m)
	elapsed := formatElapsed(agent)

	argsStr := strings.Join(agent.Args, " ")

	header := agent.ID
	if agent.Name != "" {
		header = agent.Name + " (" + agent.ID + ")"
	}

	renderFieldLine := func(label, value string) string {
		styledLabel := OverviewLabelStyle.Render(label)
		styledValue := NormalItemStyle.Render(value)
		content := styledLabel + styledValue
		return fr("│ ") + padRight(content, innerWidth) + fr(" │")
	}

	renderSectionDivider := func(title string) string {
		styledTitle := SectionHeaderStyle.Render(title)
		d := max(0, innerWidth-lipgloss.Width(styledTitle)-1)
		return fr("├─ ") + styledTitle + fr(" "+strings.Repeat("─", d)+"┤")
	}

	var lines []string
	lines = append(lines, fr("╭─ "+header+" "+strings.Repeat("─", max(0, innerWidth-lipgloss.Width(header)-2))+"╮"))
	lines = append(lines, renderFieldLine("Status : ", statusStr))
	if agent.Name != "" {
		lines = append(lines, renderFieldLine("Name   : ", agent.Name))
	}
	lines = append(lines, renderFieldLine("PID    : ", fmt.Sprintf("%d", agent.PID)))
	lines = append(lines, renderFieldLine("Dir    : ", agent.WorkDir))
	if agent.WorktreeBranch != "" {
		lines = append(lines, renderFieldLine("Branch : ", agent.WorktreeBranch))
	}
	lines = append(lines, renderFieldLine("Args   : ", truncate(argsStr, innerWidth-9)))
	lines = append(lines, renderFieldLine("Started: ", agent.StartedAt.Format("2006-01-02 15:04:05")))
	lines = append(lines, renderFieldLine("Elapsed: ", elapsed))
	if agent.LastOutput != "" {
		lines = append(lines, renderFieldLine("Last   : ", truncate(agent.LastOutput, innerWidth-9)))
	}
	lines = append(lines, renderSectionDivider("Activity Log"))

	// Viewport content
	vpLines := strings.Split(m.viewport.View(), "\n")
	for _, l := range vpLines {
		lines = append(lines, fr("│ ")+padRight(l, innerWidth)+fr(" │"))
	}

	divider := fr("├" + strings.Repeat("─", innerWidth+2) + "┤")
	lines = append(lines, divider)
	help := NormalItemStyle.Render("[esc] back  [d] diff  [K] kill  [↑↓/jk] scroll")
	lines = append(lines, fr("│ ")+padRight(help, innerWidth)+fr(" │"))
	lines = append(lines, fr("╰"+strings.Repeat("─", innerWidth+2)+"╯"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// loadLog returns a tea.Cmd that reads the log file and sends a logLoadedMsg.
func loadLog(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return logLoadedMsg{content: fmt.Sprintf("(could not read log: %v)", err)}
		}
		return logLoadedMsg{content: cleanLog(data)}
	}
}
