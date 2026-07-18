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
	group, ok := m.selectedGroup()
	if !ok {
		return "No agent selected."
	}

	agent := group.Rep
	width := clampWidth(m.width)
	innerWidth := width - 4

	argsStr := strings.Join(agent.Args, " ")

	header := agent.ID
	if agent.Name != "" {
		header = agent.Name + " (" + agent.ID + ")"
	}

	renderFieldLine := func(label, value string) string {
		styledLabel := OverviewLabelStyle.Render(label)
		styledValue := NormalItemStyle.Render(value)
		return frameRow(styledLabel+styledValue, innerWidth)
	}

	var lines []string
	lines = append(lines, frameTop(header, innerWidth))
	lines = append(lines, renderFieldLine("Status : ", formatStatus(agent, m)))
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
	lines = append(lines, renderFieldLine("Elapsed: ", formatElapsed(agent)))
	if agent.LastOutput != "" {
		lines = append(lines, renderFieldLine("Last   : ", truncate(agent.LastOutput, innerWidth-9)))
	}
	lines = append(lines, frameSectionHeader(SectionHeaderStyle.Render("Activity Log"), innerWidth))

	for _, l := range strings.Split(m.viewport.View(), "\n") {
		lines = append(lines, frameRow(l, innerWidth))
	}

	lines = append(lines, frameDivider(innerWidth))
	help := NormalItemStyle.Render("[esc] back  [d] diff  [K] kill  [↑↓/jk] scroll")
	lines = append(lines, frameRow(help, innerWidth))
	lines = append(lines, frameBottom(innerWidth))

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
