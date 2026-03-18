package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

// Model is the main bubbletea model for ax status.
type Model struct {
	agents        []store.AgentState
	cursor        int
	scrollOffset  int
	view          ViewMode
	client        *store.Client
	sub           chan store.Message
	spinner       spinner.Model
	spinnerActive bool
	viewport      viewport.Model
	width         int
	height        int
	logContent    string
	showExpired   bool
	statusMsg     string
	searchMode    bool
	searchQuery   string
	workDir       string
}

func newModel(client *store.Client, sub chan store.Message) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	workDir, _ := os.Getwd()

	return Model{
		agents:  []store.AgentState{},
		client:  client,
		sub:     sub,
		spinner: sp,
		view:    viewList,
		workDir: workDir,
	}
}

func waitForMsg(sub chan store.Message) tea.Cmd {
	return func() tea.Msg {
		return agentUpdateMsg{<-sub}
	}
}

func (m Model) Init() tea.Cmd {
	// Spinner is started lazily when running agents are first detected.
	return waitForMsg(m.sub)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// In search mode, handle text input specially
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchQuery = ""
				m.cursor = 0
				m.scrollOffset = 0
			case "enter":
				m.searchMode = false
			case "backspace", "ctrl+h":
				if len(m.searchQuery) > 0 {
					runes := []rune(m.searchQuery)
					m.searchQuery = string(runes[:len(runes)-1])
					m.cursor = firstMatchIndex(m.agents, m.showExpired, m.searchQuery)
					m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
				}
			default:
				if len(msg.Runes) > 0 {
					m.searchQuery += string(msg.Runes)
					m.cursor = firstMatchIndex(m.agents, m.showExpired, m.searchQuery)
					m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
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
				m.viewport.LineUp(1)
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
				m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
			}

		case "down", "j":
			if m.view == viewDetail {
				m.viewport.LineDown(1)
			} else {
				groups := groupedVisibleAgents(m.agents, m.showExpired)
				if m.cursor < len(groups)-1 {
					m.cursor++
				}
				m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
			}

		case "enter":
			if m.view == viewDetail {
				m.view = viewList
			} else {
				groups := groupedVisibleAgents(m.agents, m.showExpired)
				if len(groups) > 0 && m.cursor < len(groups) {
					m.view = viewDetail
					agent := groups[m.cursor].Rep
					m.viewport = viewport.New(m.width-4, m.height-13)
					cmds = append(cmds, loadLog(agent.LogFile))
				}
			}

		case "o":
			m.showExpired = !m.showExpired
			// Clamp cursor after toggling visibility
			groups := groupedVisibleAgents(m.agents, m.showExpired)
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
			groups := groupedVisibleAgents(m.agents, m.showExpired)
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
			groups := groupedVisibleAgents(m.agents, m.showExpired)
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
		groups := groupedVisibleAgents(m.agents, m.showExpired)
		if m.cursor >= len(groups) && len(groups) > 0 {
			m.cursor = len(groups) - 1
		}
		m.scrollOffset = clampScroll(m.cursor, m.scrollOffset, m.listAvailableRows())
		// If in detail view, reload log for selected group's representative
		if m.view == viewDetail && len(groups) > 0 && m.cursor < len(groups) {
			cmds = append(cmds, loadLog(groups[m.cursor].Rep.LogFile))
		}
		// Start spinner when running agents appear; stop tracking when they disappear.
		if hasRunningAgents(m.agents) {
			if !m.spinnerActive {
				m.spinnerActive = true
				cmds = append(cmds, m.spinner.Tick)
			}
		} else {
			m.spinnerActive = false
		}
		cmds = append(cmds, waitForMsg(m.sub))

	case clearStatusMsg:
		m.statusMsg = ""

	case logLoadedMsg:
		m.logContent = msg.content
		m.viewport.SetContent(m.logContent)
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		if hasRunningAgents(m.agents) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			m.spinnerActive = false
		}

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

// firstMatchIndex returns the index of the first visible group whose label contains query.
// Returns 0 if no match is found or query is empty.
func firstMatchIndex(agents []store.AgentState, showExpired bool, query string) int {
	if query == "" {
		return 0
	}
	q := strings.ToLower(query)
	groups := groupedVisibleAgents(agents, showExpired)
	for i, g := range groups {
		if strings.Contains(strings.ToLower(g.groupLabel()), q) {
			return i
		}
	}
	return 0
}

// hasRunningAgents reports whether any agent in the list is currently running.
func hasRunningAgents(agents []store.AgentState) bool {
	for _, a := range agents {
		if a.Status == store.StatusRunning {
			return true
		}
	}
	return false
}

// listAvailableRows returns the number of rows available for agent entries in the list view.
func (m Model) listAvailableRows() int {
	groups := groupedVisibleAgents(m.agents, m.showExpired)
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
