package agent

import (
	"encoding/hex"
	"reflect"
	"strings"
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

func TestLastMeaningfulLine(t *testing.T) {
	tests := []struct {
		name  string
		chunk string
		want  string
	}{
		{
			name:  "returns last readable line",
			chunk: "first line\nsecond line\n",
			want:  "second line",
		},
		{
			name:  "skips trailing noise lines",
			chunk: "real content\n>>>\n--\n",
			want:  "real content",
		},
		{
			name:  "strips ANSI escapes",
			chunk: "\x1b[32mgreen output\x1b[0m\n",
			want:  "green output",
		},
		{
			name:  "normalizes carriage returns",
			chunk: "line one\r\nline two\r",
			want:  "line two",
		},
		{
			name:  "empty when nothing meaningful",
			chunk: ">>>\n--\n   \n",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastMeaningfulLine([]byte(tt.chunk)); got != tt.want {
				t.Errorf("lastMeaningfulLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{name: "both nil", a: nil, b: nil, want: true},
		{name: "equal slices", a: []string{"x", "y"}, b: []string{"x", "y"}, want: true},
		{name: "different length", a: []string{"x"}, b: []string{"x", "y"}, want: false},
		{name: "different content", a: []string{"x", "y"}, b: []string{"x", "z"}, want: false},
		{name: "empty vs nil", a: []string{}, b: nil, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringSliceEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("stringSliceEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if !strings.HasPrefix(id, "ax-") {
		t.Errorf("generateID() = %q, want ax- prefix", id)
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
