package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedipunkz/ax/internal/store"
)

// spinnerFrames are the animation frames for the running-agent indicator.
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// spinnerFrame returns the current spinner character derived from wall-clock time.
// No TickMsg is needed; the frame advances naturally on every re-render.
func spinnerFrame() string {
	idx := time.Now().UnixMilli() / 100 % int64(len(spinnerFrames))
	return spinnerFrames[idx]
}

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

// AgentGroup represents one or more agents sharing the same name/label (e.g. after -r resume).
type AgentGroup struct {
	Rep  store.AgentState // best representative (running > success > killed, then most recent)
	PIDs []int            // all PIDs across agents in this group
}

// groupLabel returns the display label for this group.
func (g AgentGroup) groupLabel() string {
	if g.Rep.Name != "" {
		return g.Rep.Name
	}
	return g.Rep.ID
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

// groupedVisibleAgents groups visible agents by name/label into AgentGroups.
// Agents sharing the same Name (or the same ID when no Name is set) are merged into one group.
func groupedVisibleAgents(agents []store.AgentState, showExpired bool) []AgentGroup {
	visible := visibleAgents(agents, showExpired)
	groupMap := map[string]*AgentGroup{}
	var order []string
	for _, a := range visible {
		a := a
		key := a.ID
		if a.Name != "" {
			key = a.Name
		}
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

func listView(m Model) string {
	width := clampWidth(m.width)
	height := m.height
	if height < 10 {
		height = 24
	}

	innerWidth := width - 4 // outer frame: "│ " + content(innerWidth) + " │"

	// Build grouped sections.
	groups := groupedVisibleAgents(m.agents, m.showExpired)
	var running, success, killed []AgentGroup
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

	successTitle := "Success (24h)"
	killedTitle := "Killed (24h)"
	if m.showExpired {
		successTitle = "Success (all)"
		killedTitle = "Killed / Failed (all)"
	}

	// Title line with full working directory path
	agentCount := fmt.Sprintf("%d running", len(running))
	pathStr := m.workDir
	dashes := max(0, innerWidth-utf8.RuneCountInString(pathStr)-utf8.RuneCountInString(agentCount)-3)
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	titleLine := pathStyle.Render(pathStr) + fr(" "+strings.Repeat("─", dashes)+" ") + agentCount

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

	// Detail overview section: show selected group's Name, PID(s), Dir, Branch, Args.
	{
		var name, pid, dir, branch, args string
		if len(groups) > 0 && m.cursor < len(groups) {
			g := groups[m.cursor]
			name = g.groupLabel()
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
		} else {
			name, pid, dir, branch, args = "-", "-", "-", "-", "-"
		}
		renderOverviewLine := func(label, value string) string {
			styledLabel := OverviewLabelStyle.Render(label + " ")
			prefix := "  " // align with cursor column in agent rows
			maxVal := max(0, innerWidth-len(prefix)-lipgloss.Width(styledLabel))
			styledValue := NormalItemStyle.Render(truncate(value, maxVal))
			return fr("│ ") + padRight(prefix+styledLabel+styledValue, innerWidth) + fr(" │")
		}
		lines = append(lines, renderOverviewLine("Name:", name))
		lines = append(lines, renderOverviewLine("PID: ", pid))
		lines = append(lines, renderOverviewLine("Dir: ", dir))
		lines = append(lines, renderOverviewLine("Branch:", branch))
		lines = append(lines, renderOverviewLine("Args:", args))
	}

	// Fixed column widths: cursor(2) id(24) sp(1) repo(12) sp(1) status(9) sp(1) ended(11)
	// ID format: "ax-{unix_minutes}-{4hex}" = 17 chars; name can be longer so give extra room
	const (
		idWidth     = 24
		repoWidth   = 12
		statusWidth = 9
		endedWidth  = 11
		fixedTotal  = 2 + idWidth + 1 + repoWidth + 1 + statusWidth + 1 + endedWidth
	)

	// Column header row (rendered under the Running section header)
	colHeader := "  " +
		padRight("Name/Id", idWidth) + " " +
		padRight("Repo", repoWidth) + " " +
		padRight("Status", statusWidth) + " " +
		padRight("Ended", endedWidth)
	if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 {
		colHeader += "  " + "Last Output"
	}
	colHeaderLine := fr("│ ") + padRight(OverviewLabelStyle.Render(colHeader), innerWidth) + fr(" │")

	renderRow := func(group AgentGroup, idx int) string {
		cursor := "  "
		if idx == m.cursor {
			cursor = "▶ "
		}

		label := group.groupLabel()
		endedAt := "           "
		if group.Rep.FinishedAt != nil {
			endedAt = group.Rep.FinishedAt.Format("01/02 15:04")
		}
		repo := repoName(group.Rep.RepoName, group.Rep.WorkDir)
		row := cursor +
			padRight(truncate(label, idWidth), idWidth) + " " +
			padRight(RepoStyle.Render(truncate(repo, repoWidth)), repoWidth) + " " +
			padRight(formatStatus(group.Rep, m), statusWidth) + " " +
			EndedStyle.Render(endedAt)

		if remaining := max(0, innerWidth-fixedTotal-2); remaining > 8 && group.Rep.LastOutput != "" {
			row += "  " + LastOutputStyle.Render(truncate(group.Rep.LastOutput, remaining))
		}

		if idx == m.cursor {
			return SelectedItemStyle.Render(row)
		}
		return NormalItemStyle.Render(row)
	}

	// Compute available rows for agent entries.
	// Fixed frame lines: topBorder + 5 overview + colHeader + 3 section divider-headers + bottom divider + help + bottomBorder = 13.
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
	availableRows := max(0, height-13-emptyCount)

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

	// renderSection renders a divider-style section header + optional pre-rows line + group rows.
	// preRows is an optional line appended immediately after the section header (e.g. column headers).
	renderSection := func(title string, headerStyle lipgloss.Style, groupSlice []AgentGroup, baseGlobalIdx int, sliceStart int, sliceLen int, preRows string) {
		lines = append(lines, renderSectionHeader(title, headerStyle))
		if preRows != "" {
			lines = append(lines, preRows)
		}
		if len(groupSlice) == 0 {
			lines = append(lines, fr("│ ")+padRight(NormalItemStyle.Render("  (none)"), innerWidth)+fr(" │"))
			return
		}
		end := sliceStart + sliceLen
		if end > len(groupSlice) {
			end = len(groupSlice)
		}
		for i := sliceStart; i < end; i++ {
			globalIdx := baseGlobalIdx + i
			lines = append(lines, fr("│ ")+padRight(renderRow(groupSlice[i], globalIdx), innerWidth)+fr(" │"))
		}
	}

	renderSection("Running", RunningHeaderStyle, running, 0, runSliceStart, runSliceLen, colHeaderLine)
	renderSection(successTitle, SuccessHeaderStyle, success, len(running), sucSliceStart, sucSliceLen, "")
	renderSection(killedTitle, KilledHeaderStyle, killed, len(running)+len(success), kilSliceStart, kilSliceLen, "")

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
			return StatusWaitingStyle.Render("waiting")
		}
		return StatusRunningStyle.Render(spinnerFrame() + " running")
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
