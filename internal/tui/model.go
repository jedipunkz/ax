package tui

import (
	"fmt"
	"os"
	"sort"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/jedipunkz/ax/internal/store"
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

// tickMsg is sent every second to update the clock display.
type tickMsg time.Time

// Model is the main bubbletea model for ax status.
type Model struct {
	agents       []store.AgentState
	cursor       int
	scrollOffset int
	view         ViewMode
	client       *store.Client
	sub          chan store.Message
	spinner      spinner.Model
	viewport     viewport.Model
	width        int
	height       int
	logContent   string
	showExpired  bool
	statusMsg    string
	searchMode   bool
	searchQuery  string
	workDir      string
	now          time.Time
	durationDays int
}

func newModel(client *store.Client, sub chan store.Message, durationDays int) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	workDir, _ := os.Getwd()

	return Model{
		agents:       []store.AgentState{},
		client:       client,
		sub:          sub,
		spinner:      sp,
		view:         viewList,
		workDir:      workDir,
		now:          time.Now(),
		durationDays: durationDays,
	}
}

func waitForMsg(sub chan store.Message) tea.Cmd {
	return func() tea.Msg {
		return agentUpdateMsg{<-sub}
	}
}

func tickEverySecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForMsg(m.sub),
		tickEverySecond(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// In search mode, handle text input specially
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchQuery = ""
				m.cursor = 0
				m.scrollOffset = 0
			case "enter":
				// Map filtered cursor back to full groups cursor before exiting search mode
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
			return m, tea.Batch(cmds...)
		}

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
			if m.view == viewDetail {
				m.viewport.ScrollUp(1)
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
				m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
			}

		case "down", "j":
			if m.view == viewDetail {
				m.viewport.ScrollDown(1)
			} else {
				groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
				if m.cursor < len(groups)-1 {
					m.cursor++
				}
				m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
			}

		case "enter":
			if m.view == viewDetail {
				m.view = viewList
			} else {
				groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
				if len(groups) > 0 && m.cursor < len(groups) {
					m.view = viewDetail
					agent := groups[m.cursor].Rep
					m.viewport = viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(m.height-13))
					cmds = append(cmds, loadLog(agent.LogFile))
				}
			}

		case "o":
			m.showExpired = !m.showExpired
			// Clamp cursor after toggling visibility
			groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
			if m.cursor >= len(groups) && len(groups) > 0 {
				m.cursor = len(groups) - 1
			}
			m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())

		case "/":
			if m.view == viewList {
				m.searchMode = true
				m.searchQuery = ""
			}

		case "K":
			groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
			if len(groups) > 0 && m.cursor < len(groups) {
				g := groups[m.cursor]
				// Kill all running agents in the group
				for _, ag := range m.agents {
					ag := ag
					if ag.Status != store.StatusRunning {
						continue
					}
					label := ag.ID
					if ag.Name != "" {
						label = ag.Name
					}
					if label != g.groupLabel() {
						continue
					}
					if ag.PID > 0 {
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
				}
			}

		case "y":
			groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
			if m.view == viewList && len(groups) > 0 && m.cursor < len(groups) {
				ag := groups[m.cursor].Rep
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
		// Clamp cursor to visible groups
		groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
		if m.cursor >= len(groups) && len(groups) > 0 {
			m.cursor = len(groups) - 1
		}
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
		// If in detail view, reload log for selected group's representative
		if m.view == viewDetail && len(groups) > 0 && m.cursor < len(groups) {
			cmds = append(cmds, loadLog(groups[m.cursor].Rep.LogFile))
		}
		cmds = append(cmds, waitForMsg(m.sub))

	case tickMsg:
		m.now = time.Time(msg)
		cmds = append(cmds, tickEverySecond())

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
			m.viewport = viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(m.height-13))
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

func (m Model) View() tea.View {
	var content string
	switch m.view {
	case viewDetail:
		content = detailView(m)
	default:
		content = listView(m)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func sortAgents(agents []store.AgentState) {
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].StartedAt.After(agents[j].StartedAt)
	})
}

// clampScroll adjusts the scroll offset so that cursor remains in the visible window.
func clampScroll(cursor, offset, availRows int) int {
	if cursor < offset {
		offset = cursor
	}
	if availRows > 0 && cursor >= offset+availRows {
		offset = cursor - availRows + 1
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// listAvailableRows returns the number of rows available for agent entries in the list view.
func (m Model) listAvailableRows() int {
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if m.searchMode {
		groups = fuzzyFilterGroups(groups, m.searchQuery)
	}
	runCount, sucCount, kilCount := 0, 0, 0
	for _, g := range groups {
		switch g.Rep.Status {
		case store.StatusRunning:
			runCount++
		case store.StatusSuccess:
			sucCount++
		default:
			kilCount++
		}
	}
	emptyCount := 0
	if runCount == 0 {
		emptyCount++
	}
	if sucCount == 0 {
		emptyCount++
	}
	if kilCount == 0 {
		emptyCount++
	}
	h := m.height
	if h < 10 {
		h = 24
	}
	avail := h - 13 - emptyCount
	if avail < 0 {
		avail = 0
	}
	return avail
}
