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

	successTitle := "Success (24h)"
	killedTitle := "Killed (24h)"
	if m.showExpired {
		successTitle = "Success (all)"
		killedTitle = "Killed / Failed (all)"
	}

	// Title line with full working directory path
	agentCount := fmt.Sprintf("%d running", len(running))
	pathPart := ""
	if m.workDir != "" {
		pathPart = " " + m.workDir
	}
	titleText := "ax dash" + pathPart
	dashes := max(0, innerWidth-utf8.RuneCountInString(titleText)-utf8.RuneCountInString(agentCount)-3)
	titleLine := TitleStyle.Render("ax dash") + fr(pathPart) + fr(" "+strings.Repeat("─", dashes)+" ") + agentCount

	topBorder := fr("╭─ ") + titleLine + fr("─╮")

	// Helper to render a section divider line: ├─ Title ──────┤
	renderSectionHeader := func(label string, style lipgloss.Style) string {
		styledLabel := style.Render(label)
		labelWidth := lipgloss.Width(styledLabel)
		d := max(0, innerWidth-labelWidth-1)
		return fr("├─ ") + styledLabel + fr(" "+strings.Repeat("─", d)+"┤")
	}

	var lines []string
	lines = append(lines, topBorder)

	divider := fr("├" + strings.Repeat("─", innerWidth+2) + "┤")

	// Overview section divider header
	lines = append(lines, renderSectionHeader("Overview", OverviewLabelStyle))

	// Detail overview section: show selected agent's Name, PID, Dir, Branch, Args.
	{
		visible := visibleAgents(m.agents, m.showExpired)
		var name, pid, dir, branch, args string
		if len(visible) > 0 && m.cursor < len(visible) {
			ag := visible[m.cursor]
			name = ag.Name
			if name == "" {
				name = ag.ID
			}
			pid = fmt.Sprintf("%d", ag.PID)
			dir = ag.WorkDir
			branch = ag.WorktreeBranch
			if branch == "" {
				branch = "-"
			}
			args = strings.Join(ag.Args, " ")
			if args == "" {
				args = "-"
			}
		} else {
			name, pid, dir, branch, args = "-", "-", "-", "-", "-"
		}
		renderOverviewLine := func(label, value string) string {
			styledLabel := OverviewLabelStyle.Render(label + " ")
			maxVal := max(0, innerWidth-lipgloss.Width(styledLabel))
			styledValue := NormalItemStyle.Render(truncate(value, maxVal))
			return fr("│ ") + padRight(styledLabel+styledValue, innerWidth) + fr(" │")
		}
		lines = append(lines, renderOverviewLine("Name:", name))
		lines = append(lines, renderOverviewLine("PID: ", pid))
		lines = append(lines, renderOverviewLine("Dir: ", dir))
		lines = append(lines, renderOverviewLine("Branch:", branch))
		lines = append(lines, renderOverviewLine("Args:", args))
	}
	lines = append(lines, divider)

	// Fixed column widths: cursor(2) id(24) sp(1) status(11) sp(1) elapsed(9) sp(1) ended(11)
	// ID format: "ax-{unix_minutes}-{4hex}" = 17 chars; name can be longer so give extra room
	const (
		idWidth      = 24
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
			padRight(ElapsedStyle.Render(formatElapsed(agent)), elapsedWidth) + " " +
			EndedStyle.Render(endedAt)

		if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 && agent.LastOutput != "" {
			row += "  " + LastOutputStyle.Render(truncate(agent.LastOutput, remaining))
		}

		if idx == m.cursor {
			return SelectedItemStyle.Render(row)
		}
		return NormalItemStyle.Render(row)
	}

	// Compute available rows for agent entries.
	// Fixed frame lines: topBorder + Overview divider + 5 overview + divider + colHeader + 3 section divider-headers + bottom divider + help + bottomBorder = 15.
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
	availableRows := max(0, height-15-emptyCount)

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

	// renderSection renders a divider-style section header + agent rows into the outer frame.
	// baseGlobalIdx is the global index of agents[0] in the flat visible list.
	// sliceStart/sliceLen control which agents within the section to show.
	renderSection := func(title string, headerStyle lipgloss.Style, agents []store.AgentState, baseGlobalIdx int, sliceStart int, sliceLen int) {
		lines = append(lines, renderSectionHeader(title, headerStyle))
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

	renderSection("Running", RunningHeaderStyle, running, 0, runSliceStart, runSliceLen)
	renderSection(successTitle, SuccessHeaderStyle, success, len(running), sucSliceStart, sucSliceLen)
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
	var helpText string
	switch {
	case m.searchMode:
		helpText = "search: " + m.searchQuery + "█  [esc] cancel  [enter] confirm"
	case m.statusMsg != "":
		helpText = m.statusMsg
	default:
		helpText = "[↑↓/jk] select  [enter] detail  [y] yank  [K] kill  [/] search  " + historyLabel + "  [q] quit"
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
