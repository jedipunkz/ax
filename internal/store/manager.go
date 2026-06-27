package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// agentIDPattern matches the canonical generated agent ID format
// (`ax-<unix-minutes>-<4hex>`) and excludes anything containing path
// separators or `.`/`..` segments that could be abused to redirect the
// expected log-path computation.
var agentIDPattern = regexp.MustCompile(`^ax-[0-9]+-[0-9a-f]+$`)

// RunManager starts the state manager on the given Unix socket path.
// It blocks until it encounters a fatal error.
func RunManager(socketPath, stateFilePath string) error {
	mgr := &manager{
		agents:         make(map[string]AgentState),
		stateFilePath:  stateFilePath,
		dataDir:        filepath.Dir(stateFilePath),
		attachers:      make(map[string]map[*subscriber]bool),
		streams:        make(map[string]*outputStream),
		inputListeners: make(map[string]*subscriber),
	}

	// Load existing state if present.
	if err := mgr.loadState(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: could not load state: %v\n", err)
	}

	// Mark any running agents whose PIDs are no longer alive as failed.
	mgr.reconcileStaleRunning()

	// Periodically recheck running agent PIDs to catch abrupt terminations.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mgr.reconcileStaleRunning()
		}
	}()

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not listen on socket: %w", err)
	}
	defer ln.Close()
	if err := os.Chmod(socketPath, 0600); err != nil {
		return fmt.Errorf("could not secure socket: %w", err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept error: %w", err)
		}
		go mgr.handleConn(conn)
	}
}

type manager struct {
	mu            sync.Mutex
	agents        map[string]AgentState
	subscribers   []*subscriber
	stateFilePath string
	dataDir       string

	// attachers tracks output streaming subscribers per agent ID. A single
	// subscriber can be in both subscribers (state updates) and attachers
	// (output streaming) for any number of agents.
	attachers map[string]map[*subscriber]bool
	// streams tracks the running output tailer for each agent that has at
	// least one attacher.
	streams map[string]*outputStream
	// inputListeners maps an agent ID to the connection (the agent's runner)
	// that owns its PTY input. Only one listener per agent at a time; the
	// most recent register_input wins.
	inputListeners map[string]*subscriber
}

func (m *manager) loadState() error {
	agents, err := ReadAgents(m.stateFilePath)
	if err != nil {
		return err
	}
	for _, a := range agents {
		m.agents[a.ID] = a
	}
	return nil
}

func (m *manager) persistState() {
	if err := WriteAgents(m.stateFilePath, m.agentSlice()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
}

func isResumeRestart(existing, next AgentState) bool {
	return existing.FinishedAt != nil && next.StartedAt.After(*existing.FinishedAt)
}

// reconcileStaleRunning marks running agents whose PIDs are no longer alive as failed.
func (m *manager) reconcileStaleRunning() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	changed := false
	for id, a := range m.agents {
		if a.Status == StatusRunning && !isPIDAlive(a.PID) {
			a.Status = StatusFailed
			a.FinishedAt = &now
			a.WaitingUser = false
			m.agents[id] = a
			changed = true
			m.broadcast(Message{Type: "update", Agent: &a})
		}
	}
	if changed {
		m.persistState()
	}
}

func (m *manager) agentSlice() []AgentState {
	result := make([]AgentState, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

// filteredSnapshotLocked returns a snapshot of all agents that pass the given
// filter. Caller must hold m.mu.
func (m *manager) filteredSnapshotLocked(f *Filter) []AgentState {
	if f == nil {
		return m.agentSlice()
	}
	result := make([]AgentState, 0, len(m.agents))
	for _, a := range m.agents {
		if f.MatchAgent(a) {
			result = append(result, a)
		}
	}
	return result
}

func (m *manager) broadcast(msg Message) {
	dead := make([]int, 0)
	for i, sub := range m.subscribers {
		if !sub.filter.Match(msg) {
			continue
		}
		if !sub.trySend(msg) {
			dead = append(dead, i)
			sub.close()
			_ = sub.conn.Close()
		}
	}
	for i := len(dead) - 1; i >= 0; i-- {
		idx := dead[i]
		m.subscribers = append(m.subscribers[:idx], m.subscribers[idx+1:]...)
	}
}

// handleConn owns one client connection. It reads JSON-lines messages,
// dispatches them, and unwinds all subscription/attachment state when the
// connection drops.
func (m *manager) handleConn(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // allow up to 4 MB messages
	enc := json.NewEncoder(conn)

	sub := newSubscriber(conn, enc)
	go sub.run()

	defer m.cleanupConn(sub, conn)

	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		m.dispatch(sub, msg)
	}
}

// dispatch routes a single decoded message to its handler. Kept separate
// from handleConn so each message type's flow reads top-to-bottom.
func (m *manager) dispatch(sub *subscriber, msg Message) {
	switch msg.Type {
	case "update":
		m.handleUpdate(msg)
	case "remove":
		m.handleRemove(msg)
	case "subscribe":
		m.handleSubscribe(sub, msg)
	case "attach":
		m.handleAttach(sub, msg)
	case "detach":
		m.handleDetach(sub, msg.AgentID)
	case "register_input":
		m.handleRegisterInput(sub, msg)
	case "input":
		m.handleInput(sub, msg)
	case "metrics":
		m.handleMetrics(sub, msg)
	}
}

func (m *manager) handleUpdate(msg Message) {
	if msg.Agent == nil {
		return
	}
	if !m.validAgentState(*msg.Agent) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.agents[msg.Agent.ID]; exists {
		// StatusKilled is final: user explicitly terminated the agent.
		if existing.Status == StatusKilled && msg.Agent.Status != StatusKilled {
			return
		}
		// Don't allow stale updates to regress a terminal state to running.
		// A resumed session reuses the same agent ID with a fresh StartedAt
		// after FinishedAt, so that transition is valid.
		if existing.Status.IsTerminal() && msg.Agent.Status == StatusRunning && !isResumeRestart(existing, *msg.Agent) {
			return
		}
	}
	m.agents[msg.Agent.ID] = *msg.Agent
	m.persistState()
	m.broadcast(Message{Type: "update", Agent: msg.Agent})

	// If the agent just reached a terminal state, signal its output stream
	// (if any) to flush + send eof so attached clients can exit.
	if msg.Agent.Status.IsTerminal() {
		if s, ok := m.streams[msg.Agent.ID]; ok {
			m.signalStreamEOFLocked(s)
		}
	}
}

func (m *manager) validAgentState(agent AgentState) bool {
	if !agentIDPattern.MatchString(agent.ID) {
		return false
	}
	if agent.LogFile == "" || m.dataDir == "" {
		return true
	}
	want := filepath.Clean(filepath.Join(m.dataDir, "agents", agent.ID, "output.log"))
	got := filepath.Clean(agent.LogFile)
	return got == want
}

func (m *manager) handleRemove(msg Message) {
	if msg.AgentID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, msg.AgentID)
	m.persistState()
	m.broadcast(Message{Type: "remove", AgentID: msg.AgentID})
	if s, ok := m.streams[msg.AgentID]; ok {
		m.stopStreamLocked(s)
	}
}

func (m *manager) handleSubscribe(sub *subscriber, msg Message) {
	m.mu.Lock()
	if sub.subscribed {
		m.mu.Unlock()
		return
	}
	sub.filter = msg.Filter
	snapshot := Message{
		Type:   "snapshot",
		Agents: m.filteredSnapshotLocked(sub.filter),
	}
	if !sub.trySend(snapshot) {
		m.mu.Unlock()
		return
	}
	sub.subscribed = true
	m.subscribers = append(m.subscribers, sub)
	m.mu.Unlock()
}

func (m *manager) handleRegisterInput(sub *subscriber, msg Message) {
	if msg.AgentID == "" {
		return
	}
	m.mu.Lock()
	m.inputListeners[msg.AgentID] = sub
	m.mu.Unlock()
}

// cleanupConn detaches sub from every agent, removes it from subscribers,
// drops any input-listener ownership, and closes the connection. Idempotent.
func (m *manager) cleanupConn(sub *subscriber, conn net.Conn) {
	m.mu.Lock()
	for agentID := range sub.attached {
		m.detachLocked(sub, agentID)
	}
	if sub.subscribed {
		for i, s := range m.subscribers {
			if s == sub {
				m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
				break
			}
		}
		sub.subscribed = false
	}
	for agentID, listener := range m.inputListeners {
		if listener == sub {
			delete(m.inputListeners, agentID)
		}
	}
	m.mu.Unlock()
	sub.close()
	_ = conn.Close()
}
