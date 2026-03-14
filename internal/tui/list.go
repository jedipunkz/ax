package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedipunkz/ax/internal/store"
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
	title := TitleStyle.Render("ax dash")
	titleLine := title + " " + strings.Repeat("─", max(0, innerWidth-utf8.RuneCountInString("ax dash")-utf8.RuneCountInString(agentCount)-3)) + " " + agentCount

	topBorder := fr("╭─ ") + titleLine + fr("─╮")

	var lines []string
	lines = append(lines, topBorder)

	divider := fr("├" + strings.Repeat("─", innerWidth+2) + "┤")

	// Fixed column widths: cursor(2) id(17) sp(1) status(16) sp(1) elapsed(9) sp(1) ended(5)
	// ID format: "ax-{unix_minutes}-{4hex}" = 17 chars
	const (
		idWidth      = 17
		statusWidth  = 11
		elapsedWidth = 9
		endedWidth   = 11
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
		endedAt := "           "
		if agent.FinishedAt != nil {
			endedAt = agent.FinishedAt.Format("01/02 15:04")
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

	// Compute available rows for agent entries.
	// Fixed frame lines: topBorder + colHeader + 3 section headers + 2 section dividers + bottom divider + help + bottomBorder = 10.
	emptyCount := 0
	if len(running) == 0 {
		emptyCount++
	}
	if len(success) == 0 {
		emptyCount++
	}
	if len(killed) == 0 {
		emptyCount++
	}
	availableRows := max(0, height-10-emptyCount)

	// Compute per-section slice bounds based on scroll offset.
	// Flat visible list order: running[0..], success[0..], killed[0..]
	offset := m.scrollOffset
	windowEnd := offset + availableRows

	// Running: global indices [0, len(running))
	runSliceStart, runSliceLen := 0, 0
	if len(running) > 0 {
		s := max(0, offset)
		e := min(len(running), windowEnd)
		if s < e {
			runSliceStart = s
			runSliceLen = e - s
		}
	}

	// Success: global indices [len(running), len(running)+len(success))
	sucBase := len(running)
	sucSliceStart, sucSliceLen := 0, 0
	if len(success) > 0 {
		s := max(0, offset-sucBase)
		e := min(len(success), windowEnd-sucBase)
		if s < e {
			sucSliceStart = s
			sucSliceLen = e - s
		}
	}

	// Killed: global indices [len(running)+len(success), ...)
	kilBase := len(running) + len(success)
	kilSliceStart, kilSliceLen := 0, 0
	if len(killed) > 0 {
		s := max(0, offset-kilBase)
		e := min(len(killed), windowEnd-kilBase)
		if s < e {
			kilSliceStart = s
			kilSliceLen = e - s
		}
	}

	// renderSection renders a section header + agent rows into the outer frame.
	// baseGlobalIdx is the global index of agents[0] in the flat visible list.
	// sliceStart/sliceLen control which agents within the section to show.
	renderSection := func(title string, headerStyle lipgloss.Style, agents []store.AgentState, baseGlobalIdx int, sliceStart int, sliceLen int) {
		lines = append(lines, fr("│ ")+padRight(headerStyle.Render(title), innerWidth)+fr(" │"))
		if len(agents) == 0 {
			lines = append(lines, fr("│ ")+padRight(NormalItemStyle.Render("  (none)"), innerWidth)+fr(" │"))
			return
		}
		end := sliceStart + sliceLen
		if end > len(agents) {
			end = len(agents)
		}
		for i := sliceStart; i < end; i++ {
			globalIdx := baseGlobalIdx + i
			lines = append(lines, fr("│ ")+padRight(renderRow(agents[i], globalIdx), innerWidth)+fr(" │"))
		}
	}

	renderSection("RUNNING", RunningHeaderStyle, running, 0, runSliceStart, runSliceLen)
	lines = append(lines, divider)
	renderSection(successTitle, SuccessHeaderStyle, success, len(running), sucSliceStart, sucSliceLen)
	lines = append(lines, divider)
	renderSection(killedTitle, KilledHeaderStyle, killed, len(running)+len(success), kilSliceStart, kilSliceLen)

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
