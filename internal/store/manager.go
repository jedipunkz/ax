package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type subscriber struct {
	conn    net.Conn
	encoder *json.Encoder
}

// RunManager starts the state manager on the given Unix socket path.
// It blocks until it encounters a fatal error.
func RunManager(socketPath, stateFilePath string) error {
	mgr := &manager{
		agents:        make(map[string]AgentState),
		stateFilePath: stateFilePath,
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

func (m *manager) broadcast(msg Message) {
	dead := make([]int, 0)
	for i, sub := range m.subscribers {
		if err := sub.encoder.Encode(msg); err != nil {
			dead = append(dead, i)
			sub.conn.Close()
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
				if existing.Status == StatusKilled {
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
			m.mu.Unlock()

		case "subscribe":
			m.mu.Lock()
			sub := &subscriber{conn: conn, encoder: enc}
			// Send initial snapshot
			snapshot := Message{
				Type:   "snapshot",
				Agents: m.agentSlice(),
			}
			if err := enc.Encode(snapshot); err != nil {
				conn.Close()
				m.mu.Unlock()
				return
			}
			m.subscribers = append(m.subscribers, sub)
			m.mu.Unlock()
			// Keep connection open — the scanner loop will handle further messages
		}
	}

	// Clean up subscriber on disconnect
	m.mu.Lock()
	for i, sub := range m.subscribers {
		if sub.conn == conn {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	conn.Close()
}
