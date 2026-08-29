package agent

import (
	"testing"

	"github.com/jedipunkz/agx/internal/store"
)

func TestStaleAgentResult(t *testing.T) {
	got := staleAgentResult(store.AgentState{
		ID:          "agent-1",
		Status:      store.StatusRunning,
		WaitingUser: true,
	})

	if got.Status != store.StatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, store.StatusFailed)
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt = nil, want timestamp")
	}
	if got.WaitingUser {
		t.Fatal("WaitingUser = true, want false")
	}
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Fatalf("ExitCode = %v, want 1", got.ExitCode)
	}
}

func TestIsWaitComplete(t *testing.T) {
	cases := []struct {
		name string
		a    store.AgentState
		want bool
	}{
		{
			name: "running",
			a:    store.AgentState{Status: store.StatusRunning},
			want: false,
		},
		{
			name: "waiting user",
			a:    store.AgentState{Status: store.StatusRunning, WaitingUser: true},
			want: true,
		},
		{
			name: "success",
			a:    store.AgentState{Status: store.StatusSuccess},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWaitComplete(tc.a); got != tc.want {
				t.Fatalf("isWaitComplete() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResultFromWaitingAgent(t *testing.T) {
	got := resultFromAgent(store.AgentState{
		Status:      store.StatusRunning,
		WaitingUser: true,
	})

	if got.Status != store.StatusRunning {
		t.Fatalf("Status = %q, want %q", got.Status, store.StatusRunning)
	}
	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
}
