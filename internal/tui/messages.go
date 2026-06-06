package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/jedipunkz/ax/internal/store"
)

// handleAgentMsg applies a snapshot / update / remove event from the
// daemon, clamps the cursor, refreshes the log when the detail view is
// open, and re-arms the listener for the next message.
func (m Model) handleAgentMsg(msg agentUpdateMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	switch msg.Type {
	case "snapshot":
		m.agents = msg.Agents
		sortAgents(m.agents)
		if cmd := m.pollSelectedMetrics(m.now, true); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case "update":
		if msg.Agent != nil {
			m.applyAgentUpdate(*msg.Agent)
			if selected, ok := m.selectedAgent(); ok && selected.ID == msg.Agent.ID {
				if cmd := m.pollSelectedMetrics(m.now, false); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	case "remove":
		m.removeAgent(msg.AgentID)
	}

	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if m.cursor >= len(groups) && len(groups) > 0 {
		m.cursor = len(groups) - 1
	}
	m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
	if m.view == viewDetail && len(groups) > 0 && m.cursor < len(groups) {
		cmds = append(cmds, loadLog(groups[m.cursor].Rep.LogFile))
	}
	cmds = append(cmds, waitForMsg(m.sub))
	return m, cmds
}

// applyAgentUpdate merges an updated agent into the slice — replacing
// the existing entry when the ID matches, appending otherwise — and
// re-sorts so the dashboard order stays consistent.
func (m *Model) applyAgentUpdate(updated store.AgentState) {
	for i, a := range m.agents {
		if a.ID == updated.ID {
			m.agents[i] = updated
			sortAgents(m.agents)
			return
		}
	}
	m.agents = append(m.agents, updated)
	sortAgents(m.agents)
}

// removeAgent deletes the agent and any cached metrics for it.
func (m *Model) removeAgent(agentID string) {
	if agentID == "" {
		return
	}
	for i, a := range m.agents {
		if a.ID == agentID {
			m.agents = append(m.agents[:i], m.agents[i+1:]...)
			break
		}
	}
	delete(m.metrics, agentID)
	delete(m.metricsErr, agentID)
	delete(m.metricsPolled, agentID)
}

func (m Model) handleTick(msg tickMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	m.now = time.Time(msg)
	cmds = append(cmds, tickEverySecond())
	if cmd := m.pollSelectedMetrics(m.now, false); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, cmds
}

func (m Model) handleMetricsLoaded(msg metricsLoadedMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	if msg.agentID == "" {
		return m, cmds
	}
	if msg.err != "" {
		m.metricsErr[msg.agentID] = msg.err
		return m, cmds
	}
	m.metrics[msg.agentID] = msg.metrics
	delete(m.metricsErr, msg.agentID)
	return m, cmds
}

func (m Model) handleRemovingTick(cmds []tea.Cmd) (Model, []tea.Cmd) {
	if m.removing {
		m.removingDots = (m.removingDots % 3) + 1
		cmds = append(cmds, removingTickCmd())
	}
	return m, cmds
}

func (m Model) handleRemoveDone(msg removeDoneMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	m.removing = false
	m.removingDots = 0
	m.removingTarget = store.AgentState{}
	if msg.err != nil {
		m.statusMsg = fmt.Sprintf("remove error: %v", msg.err)
		cmds = append(cmds, clearStatusAfter(3*time.Second))
	}
	return m, cmds
}

func (m Model) handleLogLoaded(msg logLoadedMsg) Model {
	m.logContent = msg.content
	m.viewport.SetContent(m.logContent)
	m.viewport.GotoBottom()
	return m
}

func (m Model) handleSpinnerTick(msg spinner.TickMsg, cmds []tea.Cmd) (Model, []tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	return m, cmds
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	if m.view == viewDetail {
		m.viewport = viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(m.height-13))
		m.viewport.SetContent(m.logContent)
		m.viewport.GotoBottom()
	}
	return m
}
