package store

import (
	"encoding/base64"
	"fmt"
)

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
		m.dropAttacherLocked(sub, agentID, set)
	}
	if len(set) == 0 {
		delete(m.attachers, agentID)
		if s, ok := m.streams[agentID]; ok {
			m.stopStreamLocked(s)
		}
	}
}

// dropAttacherLocked removes a misbehaving subscriber from every agent it
// was attached to, deletes its subscribe registration, and tears down its
// connection. Caller must hold m.mu.
func (m *manager) dropAttacherLocked(sub *subscriber, currentAgentID string, currentSet map[*subscriber]bool) {
	for id := range sub.attached {
		if id == currentAgentID {
			delete(currentSet, sub)
			continue
		}
		if s, ok := m.attachers[id]; ok {
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
