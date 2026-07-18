package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/jedipunkz/ax/internal/store"
)

// handleKey routes a key press to the appropriate mode-specific handler.
// Returns the updated model and any commands the handler scheduled.
//
// While a removal is in progress, all keys are ignored to avoid races
// between the user and the (potentially slow) deletion goroutine.
func (m Model) handleKey(msg tea.KeyPressMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.removing {
		return m, cmds
	}
	if m.confirmRemove {
		return m.handleConfirmRemoveKey(msg, cmds)
	}
	if m.searchMode {
		return m.handleSearchKey(msg, cmds)
	}
	return m.handleListKey(msg, cmds)
}

func (m Model) handleConfirmRemoveKey(msg tea.KeyPressMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		ag := m.confirmTarget
		m.confirmRemove = false
		m.removing = true
		m.removingTarget = ag
		m.removingDots = 1
		cmds = append(cmds, removeAgentCmd(ag, m.client), removingTickCmd())
	case "n", "esc", "q":
		m.confirmRemove = false
	}
	return m, cmds
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		m.cursor = 0
		m.scrollOffset = 0
	case "enter":
		// Map filtered cursor back to full groups cursor before exiting search mode.
		allGroups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
		filtered := fuzzyFilterGroups(allGroups, m.searchQuery)
		if len(filtered) > 0 && m.cursor < len(filtered) {
			selectedID := filtered[m.cursor].Rep.ID
			for i, g := range allGroups {
				if g.Rep.ID == selectedID {
					m.cursor = i
					break
				}
			}
		}
		m.searchMode = false
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	case "ctrl+n":
		allGroups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
		filtered := fuzzyFilterGroups(allGroups, m.searchQuery)
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	case "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			m.cursor = 0
			m.scrollOffset = 0
		}
	default:
		if msg.Text != "" {
			m.searchQuery += msg.Text
			m.cursor = 0
			m.scrollOffset = 0
		}
	}
	return m, cmds
}

func (m Model) handleListKey(msg tea.KeyPressMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.view != viewList {
			m.view = viewList
			return m, nil
		}
		return m, []tea.Cmd{tea.Quit}

	case "esc":
		if m.view != viewList {
			m.view = viewList
		}
		return m, nil

	case "up", "k":
		return m.moveCursor(-1, cmds)

	case "down", "j":
		return m.moveCursor(+1, cmds)

	case "enter":
		return m.openDetailView(cmds)

	case "d":
		return m.openDiffView(cmds)

	case "o":
		return m.toggleExpired(cmds)

	case "/":
		if m.view == viewList {
			m.searchMode = true
			m.searchQuery = ""
		}
		return m, cmds

	case "K":
		return m.killSelectedGroup(cmds)

	case "y":
		return m.yankCdCommand(cmds)

	case "r":
		return m.requestRemove(cmds)
	}
	return m, cmds
}

// moveCursor moves the cursor (or scrolls the viewport in detail view)
// by delta and triggers a metrics refresh for the new selection.
func (m Model) moveCursor(delta int, cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.view != viewList {
		if delta < 0 {
			m.viewport.ScrollUp(-delta)
		} else {
			m.viewport.ScrollDown(delta)
		}
		return m, cmds
	}

	if delta < 0 {
		if m.cursor > 0 {
			m.cursor--
		}
	} else {
		groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
		if m.cursor < len(groups)-1 {
			m.cursor++
		}
	}
	m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	if cmd := m.pollSelectedMetrics(m.now, true); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, cmds
}

func (m Model) openDetailView(cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.view != viewList {
		m.view = viewList
		return m, cmds
	}
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if len(groups) == 0 || m.cursor >= len(groups) {
		return m, cmds
	}
	m.view = viewDetail
	ag := groups[m.cursor].Rep
	m.viewport = viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(m.height-13))
	cmds = append(cmds, loadLog(ag.LogFile))
	return m, cmds
}

// openDiffView switches to the live diff view for the currently selected
// agent. Pressing d again (or esc/q) returns to the list. The diff is
// reloaded automatically while the agent is running.
func (m Model) openDiffView(cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.view == viewDiff {
		m.view = viewList
		return m, cmds
	}
	ag, ok := m.selectedAgent()
	if !ok {
		return m, cmds
	}
	if ag.WorkDir == "" {
		m.statusMsg = "agent has no working directory"
		cmds = append(cmds, clearStatusAfter(2*time.Second))
		return m, cmds
	}
	m.view = viewDiff
	m.diffAgentID = ag.ID
	m.diffContent = ""
	m.diffErr = ""
	m.diffLoaded = false
	m.diffPolledAt = m.now
	m.viewport = viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(diffViewportHeight(m.height)))
	m.viewport.SetContent(diffPlaceholder(m))
	cmds = append(cmds, loadDiff(ag))
	return m, cmds
}

func (m Model) toggleExpired(cmds []tea.Cmd) (Model, []tea.Cmd) {
	m.showExpired = !m.showExpired
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if m.cursor >= len(groups) && len(groups) > 0 {
		m.cursor = len(groups) - 1
	}
	m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	if cmd := m.pollSelectedMetrics(m.now, true); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, cmds
}

// killSelectedGroup sends SIGTERM to every running agent that shares a
// label with the currently selected group (handles the case where a
// name was reused across resumed sessions).
func (m Model) killSelectedGroup(cmds []tea.Cmd) (Model, []tea.Cmd) {
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if len(groups) == 0 || m.cursor >= len(groups) {
		return m, cmds
	}
	target := groups[m.cursor].groupLabel()

	for _, ag := range m.agents {
		if ag.Status != store.StatusRunning {
			continue
		}
		label := ag.ID
		if ag.Name != "" {
			label = ag.Name
		}
		if label != target || ag.PID <= 0 {
			continue
		}
		killProcess(ag.PID)
		now := time.Now()
		ag.Status = store.StatusKilled
		ag.FinishedAt = &now
		for i, a := range m.agents {
			if a.ID == ag.ID {
				m.agents[i] = ag
				break
			}
		}
		_ = m.client.SendUpdate(ag) // persist to daemon (best-effort)
	}
	return m, cmds
}

func (m Model) yankCdCommand(cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.view != viewList {
		return m, cmds
	}
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if len(groups) == 0 || m.cursor >= len(groups) {
		return m, cmds
	}
	ag := groups[m.cursor].Rep
	if ag.WorkDir == "" {
		return m, cmds
	}
	cdCmd := "cd " + ag.WorkDir
	if err := copyToClipboard(cdCmd); err != nil {
		m.statusMsg = fmt.Sprintf("clipboard error: %v", err)
	} else {
		m.statusMsg = fmt.Sprintf("yanked: %s", cdCmd)
	}
	cmds = append(cmds, clearStatusAfter(2*time.Second))
	return m, cmds
}

func (m Model) requestRemove(cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.view != viewList {
		return m, cmds
	}
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if len(groups) == 0 || m.cursor >= len(groups) {
		return m, cmds
	}
	ag := groups[m.cursor].Rep
	if !ag.Status.IsTerminal() {
		m.statusMsg = "cannot remove running agent; stop it first"
		cmds = append(cmds, clearStatusAfter(2*time.Second))
		return m, cmds
	}
	m.confirmRemove = true
	m.confirmTarget = ag
	return m, cmds
}

// clearStatusAfter returns a tea.Cmd that schedules a clearStatusMsg
// after d. Used by yank, remove errors, and the running-agent guard.
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
