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

func TestParseNameFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantRest []string
	}{
		{name: "no flag", args: []string{"foo", "bar"}, wantName: "", wantRest: []string{"foo", "bar"}},
		{name: "-n flag", args: []string{"-n", "myname", "rest"}, wantName: "myname", wantRest: []string{"rest"}},
		{name: "--name flag", args: []string{"--name", "myname"}, wantName: "myname", wantRest: nil},
		{name: "--name= syntax", args: []string{"--name=myname", "x"}, wantName: "myname", wantRest: []string{"x"}},
		{name: "double dash passthrough", args: []string{"-n", "n1", "--", "-n", "n2"}, wantName: "n1", wantRest: []string{"--", "-n", "n2"}},
		{name: "-n without value is passthrough", args: []string{"-n"}, wantName: "", wantRest: []string{"-n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, rest := parseNameFlag(tt.args)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if !equalStringSlice(rest, tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
		})
	}
}

func TestParseNameFlagRequired(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		name, rest, err := parseNameFlagRequired([]string{"-n", "x", "extra"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "x" {
			t.Errorf("name = %q, want x", name)
		}
		if !equalStringSlice(rest, []string{"extra"}) {
			t.Errorf("rest = %v, want [extra]", rest)
		}
	})
	t.Run("missing returns error", func(t *testing.T) {
		_, _, err := parseNameFlagRequired([]string{"foo"})
		if err == nil {
			t.Error("expected error when name absent, got nil")
		}
	})
}

func TestParseNameAndFollowFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantName   string
		wantFollow bool
		wantRest   []string
		wantErr    bool
	}{
		{name: "name and follow", args: []string{"-n", "x", "-f"}, wantName: "x", wantFollow: true, wantRest: nil},
		{name: "long flags", args: []string{"--name", "x", "--follow", "y"}, wantName: "x", wantFollow: true, wantRest: []string{"y"}},
		{name: "name only", args: []string{"-n", "x"}, wantName: "x", wantFollow: false, wantRest: nil},
		{name: "missing name errors", args: []string{"-f"}, wantErr: true},
		{name: "double dash passthrough", args: []string{"-n", "x", "--", "-f"}, wantName: "x", wantFollow: false, wantRest: []string{"-f"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, follow, rest, err := parseNameAndFollowFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if follow != tt.wantFollow {
				t.Errorf("follow = %v, want %v", follow, tt.wantFollow)
			}
			if !equalStringSlice(rest, tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
		})
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
