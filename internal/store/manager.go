package store

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// subscriberSendBuf bounds how many messages can be queued for a single
// subscriber before the manager treats it as too slow and drops it. The
// buffer absorbs short bursts (e.g. snapshot followed by a flurry of updates)
// without letting one stalled client block fan-out to the others.
const subscriberSendBuf = 256

type subscriber struct {
	conn   net.Conn
	enc    *json.Encoder
	sendCh chan Message
	done   chan struct{}

	// Fields below are guarded by manager.mu.
	subscribed bool            // true while this sub is in manager.subscribers
	attached   map[string]bool // agent IDs this sub is attached to (output streaming)
	filter     *Filter         // subscribe filter; nil = receive everything
}

func newSubscriber(conn net.Conn, enc *json.Encoder) *subscriber {
	return &subscriber{
		conn:     conn,
		enc:      enc,
		sendCh:   make(chan Message, subscriberSendBuf),
		done:     make(chan struct{}),
		attached: make(map[string]bool),
	}
}

// run drains sendCh and writes to the connection. Returns when sendCh is
// closed (orderly shutdown) or a write fails (the connection's reader loop
// will see EOF shortly and clean up the subscriber).
func (s *subscriber) run() {
	for msg := range s.sendCh {
		if err := s.enc.Encode(msg); err != nil {
			_ = s.conn.Close()
			return
		}
	}
}

// trySend delivers a message non-blockingly. Returns false when the buffer
// is full, signaling that the caller should drop the subscriber.
func (s *subscriber) trySend(msg Message) bool {
	select {
	case <-s.done:
		return false
	case s.sendCh <- msg:
		return true
	default:
		return false
	}
}

// close is idempotent: closes sendCh exactly once so run() exits cleanly.
func (s *subscriber) close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
		close(s.sendCh)
	}
}

// RunManager starts the state manager on the given Unix socket path.
// It blocks until it encounters a fatal error.
func RunManager(socketPath, stateFilePath string) error {
	mgr := &manager{
		agents:        make(map[string]AgentState),
		stateFilePath: stateFilePath,
		attachers:     make(map[string]map[*subscriber]bool),
		streams:       make(map[string]*outputStream),
	}

	// Load existing state if present
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

	// attachers tracks output streaming subscribers per agent ID. A single
	// subscriber can be in both subscribers (state updates) and attachers
	// (output streaming) for any number of agents.
	attachers map[string]map[*subscriber]bool
	// streams tracks the running output tailer for each agent that has at
	// least one attacher.
	streams map[string]*outputStream
}

func (m *manager) loadState() error {
	data, err := os.ReadFile(m.stateFilePath)
	if err != nil {
		return err
	}
	var agents []AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return err
	}
	for _, a := range agents {
		m.agents[a.ID] = a
	}
	return nil
}

func (m *manager) persistState() {
	agents := m.agentSlice()
	data, err := json.Marshal(agents)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not marshal state: %v\n", err)
		return
	}
	tmp := m.stateFilePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write state: %v\n", err)
		return
	}
	if err := os.Rename(tmp, m.stateFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not rename state file: %v\n", err)
	}
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
			broadcastMsg := Message{Type: "update", Agent: &a}
			m.broadcast(broadcastMsg)
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
	// Remove dead subscribers (in reverse order)
	for i := len(dead) - 1; i >= 0; i-- {
		idx := dead[i]
		m.subscribers = append(m.subscribers[:idx], m.subscribers[idx+1:]...)
	}
}

func (m *manager) handleConn(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // allow up to 4 MB messages
	enc := json.NewEncoder(conn)

	sub := newSubscriber(conn, enc)
	go sub.run()

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "update":
			if msg.Agent == nil {
				continue
			}
			m.mu.Lock()
			if existing, exists := m.agents[msg.Agent.ID]; exists {
				// StatusKilled is final: user explicitly terminated the agent.
				if existing.Status == StatusKilled && msg.Agent.Status != StatusKilled {
					m.mu.Unlock()
					continue
				}
				// Don't allow a terminal state to regress to running.
				if existing.Status.IsTerminal() && msg.Agent.Status == StatusRunning {
					m.mu.Unlock()
					continue
				}
			}
			m.agents[msg.Agent.ID] = *msg.Agent
			m.persistState()
			broadcastMsg := Message{Type: "update", Agent: msg.Agent}
			m.broadcast(broadcastMsg)
			// If the agent just reached a terminal state, signal its output
			// stream (if any) to flush + send eof so attached clients can exit.
			if msg.Agent.Status.IsTerminal() {
				if s, ok := m.streams[msg.Agent.ID]; ok {
					m.signalStreamEOFLocked(s)
				}
			}
			m.mu.Unlock()

		case "remove":
			if msg.AgentID == "" {
				continue
			}
			m.mu.Lock()
			delete(m.agents, msg.AgentID)
			m.persistState()
			broadcastMsg := Message{Type: "remove", AgentID: msg.AgentID}
			m.broadcast(broadcastMsg)
			// If the agent had an active stream, tear it down quietly.
			if s, ok := m.streams[msg.AgentID]; ok {
				m.stopStreamLocked(s)
			}
			m.mu.Unlock()

		case "subscribe":
			m.mu.Lock()
			if sub.subscribed {
				m.mu.Unlock()
				continue
			}
			sub.filter = msg.Filter
			snapshot := Message{
				Type:   "snapshot",
				Agents: m.filteredSnapshotLocked(sub.filter),
			}
			if !sub.trySend(snapshot) {
				m.mu.Unlock()
				goto cleanup
			}
			sub.subscribed = true
			m.subscribers = append(m.subscribers, sub)
			m.mu.Unlock()

		case "attach":
			m.handleAttach(sub, msg)

		case "detach":
			m.handleDetach(sub, msg.AgentID)
		}
	}

cleanup:
	m.mu.Lock()
	// Detach from every agent this conn was streaming.
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
	m.mu.Unlock()
	sub.close()
	_ = conn.Close()
}

// handleAttach validates the request, ensures an output stream exists for the
// agent, optionally replays the most recent N bytes, and registers sub as an
// attacher.
func (m *manager) handleAttach(sub *subscriber, msg Message) {
	if msg.AgentID == "" {
		sub.trySend(Message{Type: "attach_err", Error: "missing agent_id"})
		return
	}

	m.mu.Lock()
	agent, ok := m.agents[msg.AgentID]
	if !ok {
		m.mu.Unlock()
		sub.trySend(Message{Type: "attach_err", AgentID: msg.AgentID, Error: "no such agent"})
		return
	}
	if sub.attached[msg.AgentID] {
		m.mu.Unlock()
		sub.trySend(Message{Type: "attach_ok", AgentID: msg.AgentID})
		return
	}

	logPath := agent.LogFile
	needStream := m.streams[msg.AgentID] == nil
	// Only start a new tailer if the agent is still running. For terminal
	// agents we just replay history and send eof — no fsnotify needed.
	if needStream && agent.Status == StatusRunning {
		stream, err := startStream(msg.AgentID, logPath)
		if err != nil {
			m.mu.Unlock()
			sub.trySend(Message{
				Type:    "attach_err",
				AgentID: msg.AgentID,
				Error:   fmt.Sprintf("could not start stream: %v", err),
			})
			return
		}
		m.streams[msg.AgentID] = stream
		go stream.run(m)
	}

	if m.attachers[msg.AgentID] == nil {
		m.attachers[msg.AgentID] = make(map[*subscriber]bool)
	}
	m.attachers[msg.AgentID][sub] = true
	sub.attached[msg.AgentID] = true
	m.mu.Unlock()

	sub.trySend(Message{Type: "attach_ok", AgentID: msg.AgentID})

	if msg.Tail > 0 && logPath != "" {
		emitTail(logPath, msg.Tail, sub, msg.AgentID)
	}

	// For agents that are already terminal, deliver eof immediately so the
	// client can exit without waiting on a stream that will never produce.
	if agent.Status.IsTerminal() {
		sub.trySend(Message{Type: "eof", AgentID: msg.AgentID})
		m.mu.Lock()
		m.detachLocked(sub, msg.AgentID)
		m.mu.Unlock()
	}
}

// handleDetach removes sub from the agent's attacher set. If sub was not
// attached, the call is a no-op.
func (m *manager) handleDetach(sub *subscriber, agentID string) {
	if agentID == "" {
		return
	}
	m.mu.Lock()
	m.detachLocked(sub, agentID)
	m.mu.Unlock()
}

// detachLocked removes sub from attachers[agentID] and stops the stream when
// no attachers remain. Caller must hold m.mu.
func (m *manager) detachLocked(sub *subscriber, agentID string) {
	if !sub.attached[agentID] {
		return
	}
	delete(sub.attached, agentID)
	if set, ok := m.attachers[agentID]; ok {
		delete(set, sub)
		if len(set) == 0 {
			delete(m.attachers, agentID)
			if s, ok := m.streams[agentID]; ok {
				m.stopStreamLocked(s)
			}
		}
	}
}

// stopStreamLocked closes the stream's stop channel and deregisters it.
// Caller must hold m.mu. Safe to call once per stream.
func (m *manager) stopStreamLocked(s *outputStream) {
	delete(m.streams, s.agentID)
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// signalStreamEOFLocked asks the stream to flush remaining bytes and emit
// "eof". Caller must hold m.mu.
func (m *manager) signalStreamEOFLocked(s *outputStream) {
	delete(m.streams, s.agentID)
	select {
	case <-s.eofCh:
	default:
		close(s.eofCh)
	}
}

// fanOutOutput sends a base64-encoded chunk of agent output to every attacher.
// Slow attachers are dropped (their connection is closed and they are removed
// from subscribers/attachers).
func (m *manager) fanOutOutput(agentID string, data []byte) {
	msg := Message{
		Type:    "output",
		AgentID: agentID,
		Data:    base64.StdEncoding.EncodeToString(data),
	}
	m.fanOutToAttachers(agentID, msg)
}

// fanOutEOF notifies every attacher that the agent's output stream has ended,
// then removes them from the attacher set.
func (m *manager) fanOutEOF(agentID string) {
	msg := Message{Type: "eof", AgentID: agentID}
	m.fanOutToAttachers(agentID, msg)

	m.mu.Lock()
	defer m.mu.Unlock()
	if set, ok := m.attachers[agentID]; ok {
		for s := range set {
			delete(s.attached, agentID)
		}
		delete(m.attachers, agentID)
	}
}

func (m *manager) fanOutToAttachers(agentID string, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	set := m.attachers[agentID]
	if len(set) == 0 {
		return
	}
	var dead []*subscriber
	for sub := range set {
		if !sub.trySend(msg) {
			dead = append(dead, sub)
		}
	}
	for _, sub := range dead {
		for id := range sub.attached {
			if id == agentID {
				delete(set, sub)
			} else if s, ok := m.attachers[id]; ok {
				delete(s, sub)
				if len(s) == 0 {
					delete(m.attachers, id)
					if st, ok := m.streams[id]; ok {
						m.stopStreamLocked(st)
					}
				}
			}
		}
		sub.attached = map[string]bool{}
		if sub.subscribed {
			for i, s := range m.subscribers {
				if s == sub {
					m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
					break
				}
			}
			sub.subscribed = false
		}
		sub.close()
		_ = sub.conn.Close()
	}
	if len(set) == 0 {
		delete(m.attachers, agentID)
		if s, ok := m.streams[agentID]; ok {
			m.stopStreamLocked(s)
		}
	}
}
