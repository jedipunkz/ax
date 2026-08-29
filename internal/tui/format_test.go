package tui

import (
	"testing"
	"time"

	"github.com/jedipunkz/agx/internal/store"
)

func TestClampWidth(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: 80},
		{in: 59, want: 80},
		{in: 60, want: 60},
		{in: 120, want: 120},
	}
	for _, tt := range tests {
		if got := clampWidth(tt.in); got != tt.want {
			t.Errorf("clampWidth(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		sec  int64
		want string
	}{
		{sec: 0, want: "0:00:00"},
		{sec: 5, want: "0:00:05"},
		{sec: 65, want: "0:01:05"},
		{sec: 3661, want: "1:01:01"},
		{sec: -10, want: "0:00:00"},
	}
	for _, tt := range tests {
		if got := formatSeconds(tt.sec); got != tt.want {
			t.Errorf("formatSeconds(%d) = %q, want %q", tt.sec, got, tt.want)
		}
	}
}

func TestFormatElapsed(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	finish := start.Add(time.Hour + 2*time.Minute + 3*time.Second)
	agent := store.AgentState{StartedAt: start, FinishedAt: &finish}
	if got := formatElapsed(agent); got != "1:02:03" {
		t.Errorf("formatElapsed() = %q, want %q", got, "1:02:03")
	}
}

func TestRepoName(t *testing.T) {
	if got := repoName("explicit", "/path/to/repo"); got != "explicit" {
		t.Errorf("repoName() with name = %q, want %q", got, "explicit")
	}
	if got := repoName("", "/path/to/repo"); got != "repo" {
		t.Errorf("repoName() from workdir = %q, want %q", got, "repo")
	}
}
