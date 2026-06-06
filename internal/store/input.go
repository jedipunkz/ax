package store

// handleInput validates and forwards an input payload to the registered
// runner connection for the target agent. Per project policy, input is only
// accepted when the agent is currently waiting for user input.
func (m *manager) handleInput(sender *subscriber, msg Message) {
	if msg.AgentID == "" {
		sender.trySend(Message{Type: "input_err", Error: "missing agent_id"})
		return
	}

	m.mu.Lock()
	agent, ok := m.agents[msg.AgentID]
	listener := m.inputListeners[msg.AgentID]
	m.mu.Unlock()

	if !ok {
		sender.trySend(Message{Type: "input_err", AgentID: msg.AgentID, Error: "no such agent"})
		return
	}
	if agent.Status != StatusRunning {
		sender.trySend(Message{Type: "input_err", AgentID: msg.AgentID, Error: "agent is not running"})
		return
	}
	if listener == nil {
		sender.trySend(Message{Type: "input_err", AgentID: msg.AgentID, Error: "agent has no input listener"})
		return
	}
	if !agent.WaitingUser {
		sender.trySend(Message{Type: "input_err", AgentID: msg.AgentID, Error: "agent is busy (not waiting for input)"})
		return
	}

	forwarded := Message{Type: "input", AgentID: msg.AgentID, Data: msg.Data}
	if !listener.trySend(forwarded) {
		sender.trySend(Message{Type: "input_err", AgentID: msg.AgentID, Error: "listener buffer full"})
		return
	}
	sender.trySend(Message{Type: "input_ok", AgentID: msg.AgentID})
}
