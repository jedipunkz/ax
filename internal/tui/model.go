package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/thirai/cco/internal/store"
)

// ViewMode represents which view is active.
type ViewMode int

const (
	viewList   ViewMode = iota
	viewDetail ViewMode = iota
)


// agentUpdateMsg wraps a store.Message received from the socket.
type agentUpdateMsg struct {
	store.Message
}

// logLoadedMsg carries the content of a loaded log file.
type logLoadedMsg struct {
	content string
}

// clearStatusMsg clears the status message after a short delay.
type clearStatusMsg struct{}

// Model is the main bubbletea model for cco status.
type Model struct {
	agents      []store.AgentState
	cursor      int
	view        ViewMode
	client      *store.Client
	sub         chan store.Message
	spinner     spinner.Model
	viewport    viewport.Model
	width       int
	height      int
	logContent  string
	showExpired bool
	statusMsg   string
}

func newModel(client *store.Client, sub chan store.Message) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		agents:  []store.AgentState{},
		client:  client,
		sub:     sub,
		spinner: sp,
		view:    viewList,
	}
}

func waitForMsg(sub chan store.Message) tea.Cmd {
	return func() tea.Msg {
		return agentUpdateMsg{<-sub}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForMsg(m.sub),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.view == viewDetail {
				m.view = viewList
				return m, nil
			}
			return m, tea.Quit

		case "esc":
			if m.view == viewDetail {
				m.view = viewList
			}
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			visible := visibleAgents(m.agents, m.showExpired)
			if m.cursor < len(visible)-1 {
				m.cursor++
			}

		case " ":
			visible := visibleAgents(m.agents, m.showExpired)
			if m.view == viewList && len(visible) > 0 && m.cursor < len(visible) {
				m.view = viewDetail
				agent := visible[m.cursor]
				m.viewport = viewport.New(m.width-4, m.height-13)
				cmds = append(cmds, loadLog(agent.LogFile))
			}

		case "h":
			m.showExpired = !m.showExpired
			// Clamp cursor after toggling visibility
			visible := visibleAgents(m.agents, m.showExpired)
			if m.cursor >= len(visible) && len(visible) > 0 {
				m.cursor = len(visible) - 1
			}

		case "K":
			visible := visibleAgents(m.agents, m.showExpired)
			if len(visible) > 0 && m.cursor < len(visible) {
				ag := visible[m.cursor]
				if ag.PID > 0 && ag.Status == store.StatusRunning {
					killProcess(ag.PID)
					// Optimistic update: mark killed immediately in local model
					now := time.Now()
					ag.Status = store.StatusKilled
					ag.FinishedAt = &now
					// Update in m.agents by ID
					for i, a := range m.agents {
						if a.ID == ag.ID {
							m.agents[i] = ag
							break
						}
					}
					_ = m.client.SendUpdate(ag) // persist to daemon (best-effort)
				}
			}

		case "y":
			visible := visibleAgents(m.agents, m.showExpired)
			if m.view == viewList && len(visible) > 0 && m.cursor < len(visible) {
				ag := visible[m.cursor]
				if ag.WorkDir != "" {
					cdCmd := fmt.Sprintf("cd %s", ag.WorkDir)
					if err := copyToClipboard(cdCmd); err != nil {
						m.statusMsg = fmt.Sprintf("clipboard error: %v", err)
					} else {
						m.statusMsg = fmt.Sprintf("yanked: %s", cdCmd)
					}
					cmds = append(cmds, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
						return clearStatusMsg{}
					}))
				}
			}
		}

	case agentUpdateMsg:
		switch msg.Type {
		case "snapshot":
			m.agents = msg.Agents
			sortAgents(m.agents)
		case "update":
			if msg.Agent != nil {
				updated := false
				for i, a := range m.agents {
					if a.ID == msg.Agent.ID {
						m.agents[i] = *msg.Agent
						updated = true
						break
					}
				}
				if !updated {
					m.agents = append(m.agents, *msg.Agent)
				}
				sortAgents(m.agents)
			}
		}
		// Clamp cursor to visible agents
		visible := visibleAgents(m.agents, m.showExpired)
		if m.cursor >= len(visible) && len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
		// If in detail view, reload log for selected agent
		if m.view == viewDetail && len(visible) > 0 && m.cursor < len(visible) {
			cmds = append(cmds, loadLog(visible[m.cursor].LogFile))
		}
		cmds = append(cmds, waitForMsg(m.sub))

	case clearStatusMsg:
		m.statusMsg = ""

	case logLoadedMsg:
		m.logContent = msg.content
		m.viewport.SetContent(m.logContent)
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewDetail {
			m.viewport = viewport.New(m.width-4, m.height-13)
			m.viewport.SetContent(m.logContent)
			m.viewport.GotoBottom()
		}
	}

	// Update viewport in detail view
	if m.view == viewDetail {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	switch m.view {
	case viewDetail:
		return detailView(m)
	default:
		return listView(m)
	}
}

func sortAgents(agents []store.AgentState) {
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].StartedAt.Before(agents[j].StartedAt)
	})
}
