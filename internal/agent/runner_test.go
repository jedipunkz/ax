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
