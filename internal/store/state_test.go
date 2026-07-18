package store

import "testing"

func TestAgentTypeName(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		want      string
	}{
		{name: "empty defaults to claude", agentType: "", want: "claude"},
		{name: "explicit claude", agentType: "claude", want: "claude"},
		{name: "codex preserved", agentType: "codex", want: "codex"},
		{name: "gemini preserved", agentType: "gemini", want: "gemini"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := AgentState{AgentType: tt.agentType}
			if got := a.AgentTypeName(); got != tt.want {
				t.Errorf("AgentTypeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{status: StatusRunning, want: false},
		{status: StatusSuccess, want: true},
		{status: StatusFailed, want: true},
		{status: StatusKilled, want: true},
		{status: Status("unknown"), want: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Errorf("IsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestFilterMatchAndMatchAgentAgree(t *testing.T) {
	filter := &Filter{
		AgentIDs: []string{"ax-1", "ax-2"},
		Statuses: []Status{StatusRunning},
	}
	cases := []struct {
		name  string
		agent AgentState
		want  bool
	}{
		{name: "matching agent", agent: AgentState{ID: "ax-1", Status: StatusRunning}, want: true},
		{name: "different agent", agent: AgentState{ID: "ax-3", Status: StatusRunning}, want: false},
		{name: "different status", agent: AgentState{ID: "ax-1", Status: StatusSuccess}, want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter.MatchAgent(tt.agent); got != tt.want {
				t.Errorf("MatchAgent(%+v) = %t, want %t", tt.agent, got, tt.want)
			}
			if got := filter.Match(Message{Agent: &tt.agent}); got != tt.want {
				t.Errorf("Match(%+v) = %t, want %t", tt.agent, got, tt.want)
			}
		})
	}
}

func TestFilterMatchRejectsStatusFilterWithoutAgent(t *testing.T) {
	filter := &Filter{Statuses: []Status{StatusRunning}}
	if filter.Match(Message{AgentID: "ax-1"}) {
		t.Error("Match() accepted a message without an agent for a status filter")
	}
}
