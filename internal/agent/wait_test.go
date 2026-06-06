package agent

import (
	"testing"

	"github.com/jedipunkz/ax/internal/store"
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
