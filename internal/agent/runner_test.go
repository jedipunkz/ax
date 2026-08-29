package agent

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/jedipunkz/agx/internal/store"
)

func TestResumeCommand(t *testing.T) {
	tests := []struct {
		agentType string
		want      []string
	}{
		{"claude", []string{"--continue"}},
		{"codex", []string{"resume", "--last"}},
		{"opencode", []string{"--continue"}},
		{"gemini", []string{"--resume", "latest"}}, // gemini v0.20.0+
		{"unknown", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			got := lookupAgent(tt.agentType).ResumeCommand()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResumeCommand(%q) = %v, want %v", tt.agentType, got, tt.want)
			}
		})
	}
}

func TestBuildResumeArgs(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		userArgs  []string
		want      []string
	}{
		{
			name:      "codex with global flag after --",
			agentType: "codex",
			userArgs:  []string{"--", "-a", "never"},
			want:      []string{"-a", "never", "resume", "--last"},
		},
		{
			name:      "codex with no user args",
			agentType: "codex",
			userArgs:  nil,
			want:      []string{"resume", "--last"},
		},
		{
			name:      "claude with global flag after --",
			agentType: "claude",
			userArgs:  []string{"--", "--model", "opus"},
			want:      []string{"--model", "opus", "--continue"},
		},
		{
			name:      "gemini preserves multi-token resume prefix",
			agentType: "gemini",
			userArgs:  []string{"--", "--debug"},
			want:      []string{"--debug", "--resume", "latest"},
		},
		{
			name:      "unknown agent has no resume prefix",
			agentType: "unknown",
			userArgs:  []string{"--", "--flag"},
			want:      []string{"--flag"},
		},
		{
			name:      "user args without leading --",
			agentType: "codex",
			userArgs:  []string{"-a", "never"},
			want:      []string{"-a", "never", "resume", "--last"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildResumeArgs(tt.agentType, tt.userArgs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildResumeArgs(%q, %v) = %v, want %v", tt.agentType, tt.userArgs, got, tt.want)
			}
		})
	}
}

func TestLookupAgent(t *testing.T) {
	for name, want := range knownAgents {
		t.Run(name, func(t *testing.T) {
			got := lookupAgent(name)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("lookupAgent(%q) = %v, want %v", name, got, want)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		got := lookupAgent("unknown")
		if got.ResumeArgs != nil {
			t.Errorf("lookupAgent(unknown).ResumeArgs = %v, want nil", got.ResumeArgs)
		}
	})
}

func TestAgentDefResumeCommandReturnsCopy(t *testing.T) {
	def := AgentDef{ResumeArgs: []string{"resume", "--last"}}

	got := def.ResumeCommand()
	got[0] = "modified"

	if def.ResumeArgs[0] != "resume" {
		t.Fatalf("ResumeCommand modified AgentDef.ResumeArgs: %v", def.ResumeArgs)
	}
}

func TestSessionConfigInitialState(t *testing.T) {
	config := sessionConfig{
		args:           []string{"--model", "fast"},
		id:             "agx-1234",
		name:           "feature/refactor",
		agentType:      "codex",
		workDir:        "/work/repo",
		worktreeBranch: "agx/agx-1234",
		repoName:       "repo",
	}

	state := config.initialState("/tmp/agent.log")
	if state.ID != config.id || state.AgentType != config.agentType || state.WorkDir != config.workDir {
		t.Fatalf("initialState() = %+v, does not reflect config", state)
	}
	if state.Status != store.StatusRunning || state.LastOutput != "interactive session" {
		t.Fatalf("initialState() = %+v, want running interactive state", state)
	}
	if !reflect.DeepEqual(state.Args, config.args) || state.LogFile != "/tmp/agent.log" {
		t.Fatalf("initialState() = %+v, want args and log path from config", state)
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if !strings.HasPrefix(id, "agx-") {
		t.Errorf("generateID() = %q, want agx- prefix", id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		t.Fatalf("generateID() = %q, want 3 dash-separated parts", id)
	}
	if len(parts[2]) != 4 {
		t.Errorf("generateID() suffix = %q, want 4 hex chars", parts[2])
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		t.Errorf("generateID() suffix %q is not valid hex: %v", parts[2], err)
	}
	if id == generateID() {
		t.Error("generateID() produced identical IDs on consecutive calls")
	}
}
