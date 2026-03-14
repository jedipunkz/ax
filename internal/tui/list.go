package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/thirai/cco/internal/store"
)

// recentThreshold returns the cutoff time for "recent" finished agents (24 hours ago).
func recentThreshold() time.Time {
	return time.Now().Add(-24 * time.Hour)
}

// visibleAgents returns the agents to display, in order: running (all), then success, then killed.
// When showExpired is true, all finished agents are included regardless of age.
func visibleAgents(agents []store.AgentState, showExpired bool) []store.AgentState {
	threshold := recentThreshold()
	var running, success, killed []store.AgentState
	for _, a := range agents {
		switch a.Status {
		case store.StatusRunning:
			running = append(running, a)
		case store.StatusSuccess:
			if showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
				success = append(success, a)
			}
		case store.StatusKilled:
			if showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
				killed = append(killed, a)
			}
		case store.StatusFailed:
			if showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
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
			if m.showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
				success = append(success, a)
			}
		case store.StatusKilled:
			if m.showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
				killed = append(killed, a)
			}
		case store.StatusFailed:
			if m.showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold)) {
				killed = append(killed, a)
			}
		}
	}

	successTitle := "SUCCESS (24h)"
	killedTitle := "KILLED (24h)"
	if m.showExpired {
		successTitle = "SUCCESS (all)"
		killedTitle = "KILLED / FAILED (all)"
	}

	// Title line
	agentCount := fmt.Sprintf("%d running", len(running))
	title := TitleStyle.Render("cco dash")
	titleLine := title + " " + strings.Repeat("─", max(0, innerWidth-utf8.RuneCountInString("cco dash")-utf8.RuneCountInString(agentCount)-3)) + " " + agentCount

	topBorder := fr("╭─ ") + titleLine + fr("─╮")

	var lines []string
	lines = append(lines, topBorder)

	divider := fr("├" + strings.Repeat("─", innerWidth+2) + "┤")

	// Fixed column widths: cursor(2) id(17) sp(1) status(16) sp(1) elapsed(9) sp(1) ended(5)
	// ID format: "cco-{unix_minutes}-{4hex}" = 17 chars
	const (
		idWidth      = 17
		statusWidth  = 11
		elapsedWidth = 9
		endedWidth   = 5
		fixedTotal   = 2 + idWidth + 1 + statusWidth + 1 + elapsedWidth + 1 + endedWidth
	)

	// Column header row
	colHeader := "  " +
		padRight("NAME/ID", idWidth) + " " +
		padRight("STATUS", statusWidth) + " " +
		padRight("ELAPSED", elapsedWidth) + " " +
		padRight("ENDED", endedWidth)
	if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 {
		colHeader += "  " + "LAST OUTPUT"
	}
	lines = append(lines, fr("│ ")+padRight(ColHeaderStyle.Render(colHeader), innerWidth)+fr(" │"))

	renderRow := func(agent store.AgentState, idx int) string {
		cursor := "  "
		if idx == m.cursor {
			cursor = "▶ "
		}

		label := agent.ID
		if agent.Name != "" {
			label = agent.Name
		}
		endedAt := "     "
		if agent.FinishedAt != nil {
			endedAt = agent.FinishedAt.Format("15:04")
		}
		row := cursor +
			padRight(truncate(label, idWidth), idWidth) + " " +
			padRight(formatStatus(agent, m), statusWidth) + " " +
			padRight(formatElapsed(agent), elapsedWidth) + " " +
			endedAt

		if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 && agent.LastOutput != "" {
			row += "  " + truncate(agent.LastOutput, remaining)
		}

		if idx == m.cursor {
			return SelectedItemStyle.Render(row)
		}
		return NormalItemStyle.Render(row)
	}

	// Priority-based row allocation: RUNNING > SUCCESS > KILLED.
	// Fixed frame lines: topBorder + colHeader + 3 section headers + 2 section dividers + bottom divider + help + bottomBorder = 10.
	const fixedFrameLines = 10
	availableRows := height - fixedFrameLines
	if availableRows < 0 {
		availableRows = 0
	}
	runningNeed := len(running)
	if runningNeed == 0 {
		runningNeed = 1 // "(none)" line
	}
	runningAlloc := min(runningNeed, availableRows)
	availableRows -= runningAlloc

	successNeed := len(success)
	if successNeed == 0 {
		successNeed = 1 // "(none)" line
	}
	successAlloc := min(successNeed, availableRows)
	availableRows -= successAlloc

	killedNeed := len(killed)
	if killedNeed == 0 {
		killedNeed = 1 // "(none)" line
	}
	killedAlloc := min(killedNeed, availableRows)

	// renderSection renders a section header + agent rows into the outer frame,
	// showing at most maxRows data lines. Returns the next available visible index.
	renderSection := func(title string, headerStyle lipgloss.Style, agents []store.AgentState, startIdx int, maxRows int) int {
		lines = append(lines, fr("│ ")+padRight(headerStyle.Render(title), innerWidth)+fr(" │"))
		if len(agents) == 0 {
			if maxRows > 0 {
				lines = append(lines, fr("│ ")+padRight(NormalItemStyle.Render("  (none)"), innerWidth)+fr(" │"))
			}
			return startIdx
		}
		shown := agents
		if maxRows >= 0 && len(shown) > maxRows {
			shown = shown[:maxRows]
		}
		for _, agent := range shown {
			lines = append(lines, fr("│ ")+padRight(renderRow(agent, startIdx), innerWidth)+fr(" │"))
			startIdx++
		}
		return startIdx
	}

	idx := 0
	idx = renderSection("RUNNING", RunningHeaderStyle, running, idx, runningAlloc)
	lines = append(lines, divider)
	idx = renderSection(successTitle, SuccessHeaderStyle, success, idx, successAlloc)
	lines = append(lines, divider)
	renderSection(killedTitle, KilledHeaderStyle, killed, idx, killedAlloc)

	// Fill remaining height with blank lines (divider + help + bottom = 3 lines)
	for len(lines) < height-3 {
		lines = append(lines, fr("│ ")+padRight("", innerWidth)+fr(" │"))
	}

	// Help line at bottom
	lines = append(lines, divider)
	historyLabel := "[o] show history"
	if m.showExpired {
		historyLabel = "[o] hide history"
	}
	helpText := "[↑↓/jk] select  [space] detail  [y] yank  [K] kill  " + historyLabel + "  [q] quit"
	if m.statusMsg != "" {
		helpText = m.statusMsg
	}
	help := NormalItemStyle.Render(helpText)
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
		return StatusSuccessStyle.Render("success")
	case store.StatusFailed:
		return StatusFailedStyle.Render("failed")
	case store.StatusKilled:
		return StatusKilledStyle.Render("killed")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
