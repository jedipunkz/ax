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
