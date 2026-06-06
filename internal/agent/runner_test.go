package agent

import (
	"reflect"
	"testing"
)

func TestResumePrefixArgs(t *testing.T) {
	tests := []struct {
		agentType string
		want      []string
	}{
		{"claude", []string{"--resume"}},
		{"codex", []string{"resume", "--last"}},
		{"opencode", []string{"--continue"}},
		{"gemini", []string{"--resume", "latest"}}, // gemini v0.20.0+
		{"unknown", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			got := resumePrefixArgs(tt.agentType)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resumePrefixArgs(%q) = %v, want %v", tt.agentType, got, tt.want)
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
			want:      []string{"--model", "opus", "--resume"},
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
