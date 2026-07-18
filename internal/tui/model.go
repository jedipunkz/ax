package tui

import (
	"os"
	"sort"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/jedipunkz/ax/internal/agent"
	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
)

// ViewMode represents which view is active.
type ViewMode int

const (
	viewList   ViewMode = iota
	viewDetail ViewMode = iota
	viewDiff   ViewMode = iota
)

// diffPollInterval controls how often the diff view reloads the worktree
// diff while the target agent is still running.
const diffPollInterval = 2 * time.Second

// agentUpdateMsg wraps a store.Message received from the socket.
type agentUpdateMsg struct {
	store.Message
}

// logLoadedMsg carries the content of a loaded log file.
type logLoadedMsg struct {
	content string
}

// metricsLoadedMsg carries the result of a daemon metrics request.
type metricsLoadedMsg struct {
	agentID string
	metrics store.Message
	err     string
}

// diffLoadedMsg carries the result of an asynchronous worktree diff load.
type diffLoadedMsg struct {
	agentID string
	content string
	err     string
}

// clearStatusMsg clears the status message after a short delay.
type clearStatusMsg struct{}

// tickMsg is sent every second to update the clock display.
type tickMsg time.Time

// removingTickMsg is sent every second while a worktree removal is in
// progress to advance the "..." progress indicator.
type removingTickMsg struct{}

// removeDoneMsg is sent when the asynchronous worktree removal completes.
type removeDoneMsg struct {
	err error
}

// Model is the main bubbletea model for ax status.
type Model struct {
	agents         []store.AgentState
	cursor         int
	scrollOffset   int
	view           ViewMode
	client         *store.Client
	socketPath     string
	sub            chan store.Message
	spinner        spinner.Model
	viewport       viewport.Model
	width          int
	height         int
	logContent     string
	showExpired    bool
	statusMsg      string
	searchMode     bool
	searchQuery    string
	workDir        string
	confirmRemove  bool
	confirmTarget  store.AgentState
	removing       bool
	removingTarget store.AgentState
	removingDots   int
	now            time.Time
	durationDays   int
	metrics        map[string]store.Message
	metricsErr     map[string]string
	metricsPolled  map[string]time.Time
	diffAgentID    string
	diffContent    string // raw (uncolored) diff, used to detect changes
	diffErr        string
	diffLoaded     bool
	diffPolledAt   time.Time
}

func newModel(client *store.Client, socketPath string, sub chan store.Message, durationDays int) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	workDir, _ := os.Getwd()

	return Model{
		agents:        []store.AgentState{},
		client:        client,
		socketPath:    socketPath,
		sub:           sub,
		spinner:       sp,
		view:          viewList,
		workDir:       workDir,
		now:           time.Now(),
		durationDays:  durationDays,
		metrics:       map[string]store.Message{},
		metricsErr:    map[string]string{},
		metricsPolled: map[string]time.Time{},
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

func removingTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return removingTickMsg{}
	})
}

func requestMetrics(socketPath, agentID string) tea.Cmd {
	return func() tea.Msg {
		if agentID == "" {
			return metricsLoadedMsg{}
		}
		client := &store.Client{}
		if err := client.Connect(socketPath); err != nil {
			return metricsLoadedMsg{agentID: agentID, err: err.Error()}
		}
		defer client.Close()
		if err := client.Metrics(agentID); err != nil {
			return metricsLoadedMsg{agentID: agentID, err: err.Error()}
		}
		msg, err := client.ReadMessage()
		if err != nil {
			return metricsLoadedMsg{agentID: agentID, err: err.Error()}
		}
		if msg.Type == "metrics_err" {
			return metricsLoadedMsg{agentID: agentID, err: msg.Error}
		}
		if msg.Type != "metrics_result" {
			return metricsLoadedMsg{agentID: agentID, err: "unexpected metrics response: " + msg.Type}
		}
		return metricsLoadedMsg{agentID: agentID, metrics: msg}
	}
}

// loadDiff computes the agent's worktree diff off the UI thread and
// delivers it as a diffLoadedMsg.
func loadDiff(ag store.AgentState) tea.Cmd {
	return func() tea.Msg {
		content, err := agent.WorktreeDiff(ag.WorkDir, ag.Commits)
		if err != nil {
			return diffLoadedMsg{agentID: ag.ID, err: err.Error()}
		}
		return diffLoadedMsg{agentID: ag.ID, content: content}
	}
}

// removeAgentCmd performs the (potentially slow) worktree removal and
// associated cleanup off the UI thread, returning a removeDoneMsg when done.
func removeAgentCmd(ag store.AgentState, client *store.Client) tea.Cmd {
	return func() tea.Msg {
		var firstErr error
		if paths, err := axfs.New(); err == nil {
			if err := agent.RemoveAgentArtifacts(paths, ag); err != nil {
				firstErr = err
			}
		}
		if err := client.SendRemove(ag.ID); err != nil && firstErr == nil {
			firstErr = err
		}
		return removeDoneMsg{err: firstErr}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForMsg(m.sub),
		tickEverySecond(),
	)
}

// selectedGroups returns the groups currently shown in the list view,
// applying the fuzzy filter while search mode is active.
func (m Model) selectedGroups() []AgentGroup {
	groups := groupedVisibleAgents(m.agents, m.showExpired, m.durationDays)
	if m.searchMode {
		groups = fuzzyFilterGroups(groups, m.searchQuery)
	}
	return groups
}

// selectedGroup returns the group under the cursor, if any.
func (m Model) selectedGroup() (AgentGroup, bool) {
	groups := m.selectedGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		return AgentGroup{}, false
	}
	return groups[m.cursor], true
}

// selectedAgent returns the representative agent of the group under the
// cursor, if any.
func (m Model) selectedAgent() (store.AgentState, bool) {
	g, ok := m.selectedGroup()
	if !ok {
		return store.AgentState{}, false
	}
	return g.Rep, true
}

// findAgent returns the agent with the given ID from the current snapshot.
func (m Model) findAgent(id string) (store.AgentState, bool) {
	for _, a := range m.agents {
		if a.ID == id {
			return a, true
		}
	}
	return store.AgentState{}, false
}

// pollDiff schedules a diff reload while the diff view is open and the
// target agent is still running, so the view tracks changes in near real
// time. When the agent finishes, one final reload is scheduled if the last
// poll predates FinishedAt, so the view always shows the final worktree
// state; after that finished agents are not re-polled.
func (m *Model) pollDiff(now time.Time) tea.Cmd {
	ag, ok := m.findAgent(m.diffAgentID)
	if !ok {
		return nil
	}
	if ag.Status != store.StatusRunning {
		if ag.FinishedAt == nil || !m.diffPolledAt.Before(*ag.FinishedAt) {
			return nil
		}
		m.diffPolledAt = now
		return loadDiff(ag)
	}
	if now.Sub(m.diffPolledAt) < diffPollInterval {
		return nil
	}
	m.diffPolledAt = now
	return loadDiff(ag)
}

func (m *Model) pollSelectedMetrics(now time.Time, force bool) tea.Cmd {
	agent, ok := m.selectedAgent()
	if !ok {
		return nil
	}
	if !force {
		if last, ok := m.metricsPolled[agent.ID]; ok && now.Sub(last) < 5*time.Second {
			return nil
		}
	}
	m.metricsPolled[agent.ID] = now
	return requestMetrics(m.socketPath, agent.ID)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m, cmds = m.handleKey(msg, cmds)
	case agentUpdateMsg:
		m, cmds = m.handleAgentMsg(msg, cmds)
	case tickMsg:
		m, cmds = m.handleTick(msg, cmds)
	case metricsLoadedMsg:
		m, cmds = m.handleMetricsLoaded(msg, cmds)
	case diffLoadedMsg:
		m, cmds = m.handleDiffLoaded(msg, cmds)
	case removingTickMsg:
		m, cmds = m.handleRemovingTick(cmds)
	case removeDoneMsg:
		m, cmds = m.handleRemoveDone(msg, cmds)
	case clearStatusMsg:
		m.statusMsg = ""
	case logLoadedMsg:
		m = m.handleLogLoaded(msg)
	case spinner.TickMsg:
		m, cmds = m.handleSpinnerTick(msg, cmds)
	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)
	}

	if m.view == viewDetail || m.view == viewDiff {
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
	case viewDiff:
		content = diffView(m)
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
	running, success, killed := groupsByStatus(m.selectedGroups())
	emptyCount := 0
	for _, section := range [][]AgentGroup{running, success, killed} {
		if len(section) == 0 {
			emptyCount++
		}
	}
	h := m.height
	if h < 10 {
		h = 24
	}
	return max(0, h-14-emptyCount)
}
