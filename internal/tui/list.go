package tui

// The dashboard list view: turns the grouped agents from group.go into framed
// rows, section headers, and the help line. Layout only — the data shaping
// lives in group.go and the value formatting in format.go.

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/agx/internal/store"
)

// Fixed column widths: cursor(2) id(24) sp(1) agent(8) sp(1) repo(12) sp(1) status(9) sp(1) ended(11)
// ID format: "agx-{unix_minutes}-{4hex}" = 17 chars; name can be longer so give extra room
const (
	idColWidth     = 24
	agentColWidth  = 8
	repoColWidth   = 12
	statusColWidth = 9
	endedColWidth  = 11
	fixedColTotal  = 2 + idColWidth + 1 + agentColWidth + 1 + repoColWidth + 1 + statusColWidth + 1 + endedColWidth
)

// lastOutputWidth returns the width available for the trailing Last Output
// column, or 0 when the terminal is too narrow to show it.
func lastOutputWidth(innerWidth int) int {
	remaining := max(0, innerWidth-fixedColTotal-2)
	if remaining > 8 {
		return remaining
	}
	return 0
}

// listView renders the main dashboard: a framed list of agent groups split
// into Running / Success / Killed sections, with an overview of the selected
// group at the top and a context-sensitive help line at the bottom.
func listView(m Model) string {
	width := clampWidth(m.width)
	height := m.height
	if height < 10 {
		height = 24
	}

	innerWidth := width - 4 // outer frame: "│ " + content(innerWidth) + " │"

	groups := m.selectedGroups()
	running, success, killed := groupsByStatus(groups)

	var lines []string
	lines = append(lines, listTitleBorder(m, innerWidth))
	lines = append(lines, overviewLines(m, groups, innerWidth)...)
	lines = append(lines, sectionLines(m, running, success, killed, height, innerWidth)...)

	// Fill remaining height with blank lines (divider + help + bottom = 3 lines)
	for len(lines) < height-3 {
		lines = append(lines, frameRow("", innerWidth))
	}

	lines = append(lines, frameDivider(innerWidth))
	lines = append(lines, frameRow(NormalItemStyle.Render(listHelpText(m)), innerWidth))
	lines = append(lines, frameBottom(innerWidth))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// listTitleBorder renders the top border embedding the clock and the
// unfiltered agent status counts: ╭─ 2006-01-02 15:04 ──── Running: … ─╮
func listTitleBorder(m Model, innerWidth int) string {
	clockStr := m.now.Format("2006-01-02 15:04")
	var totalRunning, totalSuccess, totalKilled int
	for _, a := range m.agents {
		switch a.Status {
		case store.StatusRunning:
			totalRunning++
		case store.StatusSuccess:
			totalSuccess++
		default:
			totalKilled++
		}
	}
	countsStr := fmt.Sprintf("Running: %d  Success: %d  Killed: %d", totalRunning, totalSuccess, totalKilled)
	clockStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	dashes := max(0, innerWidth-utf8.RuneCountInString(clockStr)-utf8.RuneCountInString(countsStr)-3)
	titleLine := clockStyle.Render(clockStr) + fr(" "+strings.Repeat("─", dashes)+" ") + countsStr
	return fr("╭─ ") + titleLine + fr("─╮")
}

// overviewLines renders the detail overview section for the selected group:
// Name, Agent, PID(s), Dir, Branch, Args, and Stats.
func overviewLines(m Model, groups []AgentGroup, innerWidth int) []string {
	name, agentType, pid, dir, branch, args, stats := "-", "-", "-", "-", "-", "-", "-"
	if len(groups) > 0 && m.cursor < len(groups) {
		g := groups[m.cursor]
		name = g.groupLabel()
		agentType = g.Rep.AgentTypeName()
		pid = g.pidString()
		dir = g.Rep.WorkDir
		branch = g.Rep.WorktreeBranch
		if branch == "" {
			branch = "-"
		}
		args = strings.Join(g.Rep.Args, " ")
		if args == "" {
			args = "-"
		}
		stats = formatMetricsSummary(m, g.Rep)
	}
	renderLine := func(label, value string) string {
		styledLabel := OverviewLabelStyle.Render(label + " ")
		prefix := "  " // align with cursor column in agent rows
		maxVal := max(0, innerWidth-len(prefix)-lipgloss.Width(styledLabel))
		styledValue := NormalItemStyle.Render(truncate(value, maxVal))
		return frameRow(prefix+styledLabel+styledValue, innerWidth)
	}
	return []string{
		renderLine("Name: ", name),
		renderLine("Agent:", agentType),
		renderLine("PID:  ", pid),
		renderLine("Dir:  ", dir),
		renderLine("Branch:", branch),
		renderLine("Args: ", args),
		renderLine("Stats:", stats),
	}
}

// columnHeaderLine renders the column header row shown under the Running
// section header.
func columnHeaderLine(innerWidth int) string {
	header := "  " +
		padRight("Name/Id", idColWidth) + " " +
		padRight("Agent", agentColWidth) + " " +
		padRight("Repo", repoColWidth) + " " +
		padRight("Status", statusColWidth) + " " +
		padRight("Ended", endedColWidth)
	if lastOutputWidth(innerWidth) > 0 {
		header += "  " + "Last Output"
	}
	return frameRow(OverviewLabelStyle.Render(header), innerWidth)
}

// agentRowLine renders one agent group as a table row, highlighting it when
// it is under the cursor.
func agentRowLine(m Model, group AgentGroup, selected bool, innerWidth int) string {
	cursor := "  "
	if selected {
		cursor = "▶ "
	}

	endedAt := "           "
	if group.Rep.FinishedAt != nil {
		endedAt = group.Rep.FinishedAt.Format("01/02 15:04")
	}
	repo := repoName(group.Rep.RepoName, group.Rep.WorkDir)
	row := cursor +
		padRight(truncate(group.groupLabel(), idColWidth), idColWidth) + " " +
		padRight(truncate(group.Rep.AgentTypeName(), agentColWidth), agentColWidth) + " " +
		padRight(RepoStyle.Render(truncate(repo, repoColWidth)), repoColWidth) + " " +
		padRight(formatStatus(group.Rep, m), statusColWidth) + " " +
		EndedStyle.Render(endedAt)

	if w := lastOutputWidth(innerWidth); w > 0 && group.Rep.LastOutput != "" {
		row += "  " + LastOutputStyle.Render(truncate(group.Rep.LastOutput, w))
	}

	if selected {
		return SelectedItemStyle.Render(row)
	}
	return NormalItemStyle.Render(row)
}

// sectionWindow returns the visible sub-range [start, start+length) of a
// section whose entries occupy global indices [base, base+sectionLen) in the
// flattened list, given the scroll window [offset, windowEnd).
func sectionWindow(sectionLen, base, offset, windowEnd int) (start, length int) {
	if sectionLen == 0 {
		return 0, 0
	}
	s := max(0, offset-base)
	e := min(sectionLen, windowEnd-base)
	if s >= e {
		return 0, 0
	}
	return s, e - s
}

// sectionLines renders the three agent sections (Running / Success / Killed),
// windowed by the current scroll offset so the flat cursor stays visible.
func sectionLines(m Model, running, success, killed []AgentGroup, height, innerWidth int) []string {
	successTitle, killedTitle := sectionTitles(m)

	// Compute available rows for agent entries.
	// Fixed frame lines: topBorder + 7 overview + colHeader + 3 section divider-headers + bottom divider + help + bottomBorder = 15.
	emptyCount := 0
	for _, section := range [][]AgentGroup{running, success, killed} {
		if len(section) == 0 {
			emptyCount++
		}
	}
	availableRows := max(0, height-15-emptyCount)

	// Flat visible list order: running[0..], success[0..], killed[0..]
	offset := m.scrollOffset
	windowEnd := offset + availableRows

	// renderSection renders a divider-style section header + optional pre-rows
	// line + group rows. preRow is appended immediately after the section
	// header (used for the column header under Running).
	renderSection := func(title string, headerStyle lipgloss.Style, section []AgentGroup, base int, preRow string) []string {
		lines := []string{frameSectionHeader(headerStyle.Render(title), innerWidth)}
		if preRow != "" {
			lines = append(lines, preRow)
		}
		if len(section) == 0 {
			return append(lines, frameRow(NormalItemStyle.Render("  (none)"), innerWidth))
		}
		start, length := sectionWindow(len(section), base, offset, windowEnd)
		for i := start; i < start+length; i++ {
			selected := base+i == m.cursor
			lines = append(lines, frameRow(agentRowLine(m, section[i], selected, innerWidth), innerWidth))
		}
		return lines
	}

	var lines []string
	lines = append(lines, renderSection("Running", RunningHeaderStyle, running, 0, columnHeaderLine(innerWidth))...)
	lines = append(lines, renderSection(successTitle, SuccessHeaderStyle, success, len(running), "")...)
	lines = append(lines, renderSection(killedTitle, KilledHeaderStyle, killed, len(running)+len(success), "")...)
	return lines
}

// sectionTitles returns the Success / Killed section titles, reflecting
// whether expired agents are shown.
func sectionTitles(m Model) (successTitle, killedTitle string) {
	if m.showExpired {
		return "Success (all)", "Killed / Failed (all)"
	}
	durationLabel := fmt.Sprintf("%dd", m.durationDays)
	return "Success (" + durationLabel + ")", "Killed (" + durationLabel + ")"
}

// listHelpText returns the bottom help line, which doubles as the display
// area for the remove confirmation / progress and search prompts.
func listHelpText(m Model) string {
	switch {
	case m.removing:
		label := m.removingTarget.Label()
		dots := m.removingDots
		if dots < 1 {
			dots = 1
		}
		return fmt.Sprintf("Removing \"%s\"%s", label, strings.Repeat(".", dots))
	case m.confirmRemove:
		label := m.confirmTarget.Label()
		return fmt.Sprintf("Remove \"%s\"? [y] yes  [n] no", label)
	case m.searchMode:
		return "search: " + m.searchQuery + "█  [ctrl-n/p] select  [esc] cancel  [enter] confirm"
	case m.statusMsg != "":
		return m.statusMsg
	default:
		historyLabel := "[o] history"
		if m.showExpired {
			historyLabel = "[o] hide history"
		}
		return "[jk] select  [enter] detail  [d] diff  [y] yank  [K] kill  [r] remove  [/] search  " + historyLabel + "  [q] quit"
	}
}
