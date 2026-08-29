package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/agx/internal/store"
)

// recentThreshold returns the cutoff time for finished agents based on the configured duration in days.
func recentThreshold(days int) time.Time {
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
}

// visibleAgents returns the agents to display, in order: running (all), then success, then killed.
// When showExpired is true, all finished agents are included regardless of age.
func visibleAgents(agents []store.AgentState, showExpired bool, days int) []store.AgentState {
	threshold := recentThreshold(days)
	recentEnough := func(a store.AgentState) bool {
		return showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold))
	}
	var running, success, killed []store.AgentState
	for _, a := range agents {
		switch a.Status {
		case store.StatusRunning:
			running = append(running, a)
		case store.StatusSuccess:
			if recentEnough(a) {
				success = append(success, a)
			}
		case store.StatusKilled, store.StatusFailed:
			if recentEnough(a) {
				killed = append(killed, a)
			}
		}
	}
	return append(append(running, success...), killed...)
}

// AgentGroup represents one or more agents sharing the same name/label (e.g. after -r resume).
type AgentGroup struct {
	Rep  store.AgentState // best representative (running > success > killed, then most recent)
	PIDs []int            // all PIDs across agents in this group
}

// groupLabel returns the display label for this group.
func (g AgentGroup) groupLabel() string {
	return g.Rep.Label()
}

// pidString returns comma-separated PIDs.
func (g AgentGroup) pidString() string {
	parts := make([]string, len(g.PIDs))
	for i, p := range g.PIDs {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ", ")
}

// isBetterRep returns true if candidate should replace current as the group representative.
// Priority order: running > success > killed/failed, then most recently started.
func isBetterRep(candidate, current store.AgentState) bool {
	statusPriority := func(s store.Status) int {
		switch s {
		case store.StatusRunning:
			return 3
		case store.StatusSuccess:
			return 2
		default:
			return 1
		}
	}
	cp := statusPriority(candidate.Status)
	rp := statusPriority(current.Status)
	if cp != rp {
		return cp > rp
	}
	return candidate.StartedAt.After(current.StartedAt)
}

// fuzzyMatch returns true if all runes of pattern appear in text in order (case-insensitive).
func fuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)
	pi := 0
	patRunes := []rune(pattern)
	for _, c := range text {
		if pi < len(patRunes) && patRunes[pi] == c {
			pi++
		}
	}
	return pi == len(patRunes)
}

// fuzzyFilterGroups returns groups whose label matches the fuzzy query.
func fuzzyFilterGroups(groups []AgentGroup, query string) []AgentGroup {
	if query == "" {
		return groups
	}
	var result []AgentGroup
	for _, g := range groups {
		if fuzzyMatch(query, g.groupLabel()) {
			result = append(result, g)
		}
	}
	return result
}

// groupedVisibleAgents groups visible agents by name/label into AgentGroups.
// Agents sharing the same Name (or the same ID when no Name is set) are merged into one group.
func groupedVisibleAgents(agents []store.AgentState, showExpired bool, days int) []AgentGroup {
	visible := visibleAgents(agents, showExpired, days)
	groupMap := map[string]*AgentGroup{}
	var order []string
	for _, a := range visible {
		a := a
		key := a.Label()
		if g, ok := groupMap[key]; ok {
			g.PIDs = append(g.PIDs, a.PID)
			if isBetterRep(a, g.Rep) {
				g.Rep = a
			}
		} else {
			groupMap[key] = &AgentGroup{Rep: a, PIDs: []int{a.PID}}
			order = append(order, key)
		}
	}
	result := make([]AgentGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groupMap[key])
	}
	return result
}

// groupsByStatus splits groups into the three display sections in order.
// Failed groups are shown in the killed section.
func groupsByStatus(groups []AgentGroup) (running, success, killed []AgentGroup) {
	for _, g := range groups {
		switch g.Rep.Status {
		case store.StatusRunning:
			running = append(running, g)
		case store.StatusSuccess:
			success = append(success, g)
		default:
			killed = append(killed, g)
		}
	}
	return running, success, killed
}

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

func formatStatus(agent store.AgentState, m Model) string {
	switch agent.Status {
	case store.StatusRunning:
		if agent.WaitingUser {
			return StatusWaitingStyle.Render("waiting")
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

func repoName(name, workDir string) string {
	if name != "" {
		return name
	}
	return filepath.Base(workDir)
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

func formatMetricsSummary(m Model, agent store.AgentState) string {
	if err := m.metricsErr[agent.ID]; err != "" {
		return "metrics error: " + err
	}
	metrics, ok := m.metrics[agent.ID]
	if !ok {
		return "loading..."
	}
	return fmt.Sprintf(
		"%s  commits %d  output %d lines",
		formatSeconds(metrics.DurationSec),
		metrics.CommitCount,
		metrics.OutputLines,
	)
}

func formatSeconds(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	mn := (sec / 60) % 60
	s := sec % 60
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
