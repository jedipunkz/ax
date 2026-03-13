package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/thirai/cco/internal/store"
)

// recentThreshold returns the cutoff time for "recent" finished agents (5 minutes ago).
func recentThreshold() time.Time {
	return time.Now().Add(-5 * time.Minute)
}

// visibleAgents returns the agents to display, in order: running (all), then recent success, then recent killed.
func visibleAgents(agents []store.AgentState) []store.AgentState {
	threshold := recentThreshold()
	var running, success, killed []store.AgentState
	for _, a := range agents {
		switch a.Status {
		case store.StatusRunning:
			running = append(running, a)
		case store.StatusSuccess:
			if a.FinishedAt != nil && a.FinishedAt.After(threshold) {
				success = append(success, a)
			}
		case store.StatusKilled:
			if a.FinishedAt != nil && a.FinishedAt.After(threshold) {
				killed = append(killed, a)
			}
		}
	}
	return append(append(running, success...), killed...)
}

func listView(m Model) string {
	width := clampWidth(m.width)
	height := m.height
	if height < 10 {
		height = 24
	}

	innerWidth := width - 4 // outer frame: "│ " + content(innerWidth) + " │"

	// Single pass: build sections.
	threshold := recentThreshold()
	var running, success, killed []store.AgentState
	for _, a := range m.agents {
		switch a.Status {
		case store.StatusRunning:
			running = append(running, a)
		case store.StatusSuccess:
			if a.FinishedAt != nil && a.FinishedAt.After(threshold) {
				success = append(success, a)
			}
		case store.StatusKilled:
			if a.FinishedAt != nil && a.FinishedAt.After(threshold) {
				killed = append(killed, a)
			}
		}
	}

	// Title line
	agentCount := fmt.Sprintf("%d running", len(running))
	title := TitleStyle.Render("cco status")
	titleLine := title + " " + strings.Repeat("─", max(0, innerWidth-utf8.RuneCountInString("cco status")-utf8.RuneCountInString(agentCount)-3)) + " " + agentCount

	topBorder := fr("╭─ ") + titleLine + fr("─╮")

	var lines []string
	lines = append(lines, topBorder)

	divider := fr("├" + strings.Repeat("─", innerWidth+2) + "┤")

	renderRow := func(agent store.AgentState, idx int) string {
		cursor := "  "
		if idx == m.cursor {
			cursor = "▶ "
		}

		// Fixed columns: cursor(2) id(15+1) status(16+1) elapsed(9)
		const (
			idWidth      = 15
			statusWidth  = 16
			elapsedWidth = 9
			fixedTotal   = 2 + idWidth + 1 + statusWidth + 1 + elapsedWidth
		)
		row := cursor +
			padRight(truncate(agent.ID, idWidth), idWidth) + " " +
			padRight(formatStatus(agent, m), statusWidth) + " " +
			padRight(formatElapsed(agent), elapsedWidth)

		if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 && agent.LastOutput != "" {
			row += "  " + truncate(agent.LastOutput, remaining)
		}

		if idx == m.cursor {
			return SelectedItemStyle.Render(row)
		}
		return NormalItemStyle.Render(row)
	}

	// renderSection renders a section header + agent rows into the outer frame.
	// Returns the next available visible index.
	renderSection := func(title string, headerStyle lipgloss.Style, agents []store.AgentState, startIdx int) int {
		lines = append(lines, fr("│ ")+padRight(headerStyle.Render(title), innerWidth)+fr(" │"))
		if len(agents) == 0 {
			lines = append(lines, fr("│ ")+padRight(NormalItemStyle.Render("  (none)"), innerWidth)+fr(" │"))
			return startIdx
		}
		for _, agent := range agents {
			lines = append(lines, fr("│ ")+padRight(renderRow(agent, startIdx), innerWidth)+fr(" │"))
			startIdx++
		}
		return startIdx
	}

	idx := 0
	idx = renderSection("RUNNING", RunningHeaderStyle, running, idx)
	lines = append(lines, divider)
	idx = renderSection("SUCCESS (recent)", SuccessHeaderStyle, success, idx)
	lines = append(lines, divider)
	renderSection("KILLED (recent)", KilledHeaderStyle, killed, idx)

	// Fill remaining height with blank lines (divider + help + bottom = 3 lines)
	for len(lines) < height-3 {
		lines = append(lines, fr("│ ")+padRight("", innerWidth)+fr(" │"))
	}

	// Help line at bottom
	lines = append(lines, divider)
	help := NormalItemStyle.Render("[↑↓/jk] select  [space] detail  [K] kill  [q] quit")
	lines = append(lines, fr("│ ")+padRight(help, innerWidth)+fr(" │"))
	lines = append(lines, fr("╰"+strings.Repeat("─", innerWidth+2)+"╯"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatStatus(agent store.AgentState, m Model) string {
	switch agent.Status {
	case store.StatusRunning:
		if agent.WaitingUser {
			return StatusWaitingStyle.Render("waiting you")
		}
		return StatusRunningStyle.Render(m.spinner.View() + " running")
	case store.StatusSuccess:
		return StatusSuccessStyle.Render("✓ success")
	case store.StatusFailed:
		return StatusFailedStyle.Render("✗ failed")
	case store.StatusKilled:
		return StatusKilledStyle.Render("✕ killed")
	default:
		return string(agent.Status)
	}
}

func formatElapsed(agent store.AgentState) string {
	var d time.Duration
	if agent.FinishedAt != nil {
		d = agent.FinishedAt.Sub(agent.StartedAt)
	} else {
		d = time.Since(agent.StartedAt)
	}

	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, mn, s)
}

func clampWidth(w int) int {
	if w < 60 {
		return 80
	}
	return w
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
