package cmd

import (
	"testing"
)

func TestParseAgentTypeAndNameFlag(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantAgentType string
		wantName      string
		wantRest      []string
		wantErr       bool
	}{
		{
			name:          "no args returns empty agent type",
			args:          []string{},
			wantAgentType: "",
		},
		{
			name:          "-a flag sets agent type",
			args:          []string{"-a", "codex"},
			wantAgentType: "codex",
		},
		{
			name:          "-m flag sets agent type",
			args:          []string{"-m", "codex"},
			wantAgentType: "codex",
		},
		{
			name:          "--agent flag sets agent type",
			args:          []string{"--agent", "gemini"},
			wantAgentType: "gemini",
		},
		{
			name:          "--agent= syntax",
			args:          []string{"--agent=opencode"},
			wantAgentType: "opencode",
		},
		{
			name:          "-a and -n together",
			args:          []string{"-a", "codex", "-n", "myjob"},
			wantAgentType: "codex",
			wantName:      "myjob",
		},
		{
			name:          "-n without -a returns empty agent type",
			args:          []string{"-n", "myjob"},
			wantAgentType: "",
			wantName:      "myjob",
		},
		{
			name:          "--name= syntax",
			args:          []string{"-a", "gemini", "--name=myjob"},
			wantAgentType: "gemini",
			wantName:      "myjob",
		},
		{
			name:          "-a before double-dash separator",
			args:          []string{"-a", "opencode", "--", "--some-flag"},
			wantAgentType: "opencode",
			wantRest:      []string{"--", "--some-flag"},
		},
		{
			name:          "double-dash only returns empty agent type",
			args:          []string{"--", "--some-flag"},
			wantAgentType: "",
			wantRest:      []string{"--", "--some-flag"},
		},
		{
			name:          "-a and -n then double-dash",
			args:          []string{"-a", "claude", "-n", "foo", "--", "--verbose"},
			wantAgentType: "claude",
			wantName:      "foo",
			wantRest:      []string{"--", "--verbose"},
		},
		{
			name:          "unknown flag goes to rest",
			args:          []string{"--unknown-flag"},
			wantAgentType: "",
			wantRest:      []string{"--unknown-flag"},
		},
		{
			name:    "path separator in agent type returns error",
			args:    []string{"-a", "../evil"},
			wantErr: true,
		},
		{
			name:    "space in agent type returns error",
			args:    []string{"-a", "rm -rf"},
			wantErr: true,
		},
		{
			name:    "absolute path in agent type returns error",
			args:    []string{"-a", "/usr/bin/rm"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentType, name, rest, err := parseAgentTypeAndNameFlag(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if agentType != tt.wantAgentType {
				t.Errorf("agentType = %q, want %q", agentType, tt.wantAgentType)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if len(rest) != len(tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
				return
			}
			for i := range rest {
				if rest[i] != tt.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q", i, rest[i], tt.wantRest[i])
				}
			}
		})
	}
}
