package agent

import (
	"testing"
	"time"

	"github.com/jedipunkz/ax/internal/store"
)

func TestFindAgentExactID(t *testing.T) {
	agents := []store.AgentState{
		{ID: "ax-1-aaaa", Name: "feat"},
		{ID: "ax-2-bbbb"},
	}
	got, idx := FindAgent(agents, "ax-1-aaaa")
	if idx != 0 || got.ID != "ax-1-aaaa" {
		t.Errorf("exact ID match = (%v,%d), want (ax-1-aaaa,0)", got.ID, idx)
	}
}

func TestFindAgentExactNamePrefersMostRecent(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	agents := []store.AgentState{
		{ID: "older", Name: "feat", StartedAt: base},
		{ID: "newer", Name: "feat", StartedAt: base.Add(time.Hour)},
	}
	got, _ := FindAgent(agents, "feat")
	if got.ID != "newer" {
		t.Errorf("expected most recent name match, got %q", got.ID)
	}
}

func TestFindAgentIDPrefix(t *testing.T) {
	agents := []store.AgentState{
		{ID: "ax-100-abcd"},
		{ID: "ax-200-efgh"},
	}
	got, _ := FindAgent(agents, "ax-1")
	if got.ID != "ax-100-abcd" {
		t.Errorf("prefix match = %q, want ax-100-abcd", got.ID)
	}
}

func TestFindAgentSanitizedBranch(t *testing.T) {
	agents := []store.AgentState{
		{ID: "ax-1", WorktreeBranch: "my-branch"},
	}
	got, _ := FindAgent(agents, "My Branch")
	if got.ID != "ax-1" {
		t.Errorf("sanitized-branch match = %q, want ax-1", got.ID)
	}
}

func TestFindAgentNotFound(t *testing.T) {
	_, idx := FindAgent([]store.AgentState{{ID: "ax-1"}}, "nope")
	if idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestFindAgentEmptyQuery(t *testing.T) {
	_, idx := FindAgent([]store.AgentState{{ID: "ax-1"}}, "")
	if idx != -1 {
		t.Errorf("empty query should return -1, got %d", idx)
	}
}
