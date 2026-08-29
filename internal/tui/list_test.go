package tui

import (
	"testing"
	"time"

	"github.com/jedipunkz/agx/internal/store"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{name: "empty pattern matches anything", pattern: "", text: "anything", want: true},
		{name: "exact match", pattern: "abc", text: "abc", want: true},
		{name: "subsequence match", pattern: "ac", text: "abc", want: true},
		{name: "case insensitive", pattern: "AbC", text: "aXbXc", want: true},
		{name: "out of order fails", pattern: "ca", text: "abc", want: false},
		{name: "missing rune fails", pattern: "abcd", text: "abc", want: false},
		{name: "empty text non-empty pattern fails", pattern: "a", text: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fuzzyMatch(tt.pattern, tt.text); got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
			}
		})
	}
}

func TestFuzzyFilterGroups(t *testing.T) {
	groups := []AgentGroup{
		{Rep: store.AgentState{Name: "alpha"}},
		{Rep: store.AgentState{Name: "beta"}},
		{Rep: store.AgentState{ID: "agx-1-abcd"}},
	}

	if got := fuzzyFilterGroups(groups, ""); len(got) != 3 {
		t.Errorf("empty query returned %d groups, want 3", len(got))
	}

	got := fuzzyFilterGroups(groups, "alpha")
	if len(got) != 1 || got[0].Rep.Name != "alpha" {
		t.Errorf("query 'alpha' = %+v, want single alpha group", got)
	}

	if got := fuzzyFilterGroups(groups, "zzz"); len(got) != 0 {
		t.Errorf("non-matching query returned %d groups, want 0", len(got))
	}
}

func TestGroupLabel(t *testing.T) {
	withName := AgentGroup{Rep: store.AgentState{Name: "myname", ID: "agx-1-aaaa"}}
	if got := withName.groupLabel(); got != "myname" {
		t.Errorf("groupLabel() with name = %q, want %q", got, "myname")
	}

	noName := AgentGroup{Rep: store.AgentState{ID: "agx-1-aaaa"}}
	if got := noName.groupLabel(); got != "agx-1-aaaa" {
		t.Errorf("groupLabel() without name = %q, want %q", got, "agx-1-aaaa")
	}
}

func TestPidString(t *testing.T) {
	g := AgentGroup{PIDs: []int{10, 20, 30}}
	if got := g.pidString(); got != "10, 20, 30" {
		t.Errorf("pidString() = %q, want %q", got, "10, 20, 30")
	}

	empty := AgentGroup{}
	if got := empty.pidString(); got != "" {
		t.Errorf("pidString() empty = %q, want empty string", got)
	}
}

func TestIsBetterRep(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(s store.Status, offset time.Duration) store.AgentState {
		return store.AgentState{Status: s, StartedAt: base.Add(offset)}
	}

	tests := []struct {
		name      string
		candidate store.AgentState
		current   store.AgentState
		want      bool
	}{
		{
			name:      "running beats success",
			candidate: mk(store.StatusRunning, 0),
			current:   mk(store.StatusSuccess, time.Hour),
			want:      true,
		},
		{
			name:      "success beats killed",
			candidate: mk(store.StatusSuccess, 0),
			current:   mk(store.StatusKilled, time.Hour),
			want:      true,
		},
		{
			name:      "lower priority loses",
			candidate: mk(store.StatusFailed, time.Hour),
			current:   mk(store.StatusRunning, 0),
			want:      false,
		},
		{
			name:      "same status newer wins",
			candidate: mk(store.StatusRunning, time.Hour),
			current:   mk(store.StatusRunning, 0),
			want:      true,
		},
		{
			name:      "same status older loses",
			candidate: mk(store.StatusRunning, 0),
			current:   mk(store.StatusRunning, time.Hour),
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBetterRep(tt.candidate, tt.current); got != tt.want {
				t.Errorf("isBetterRep() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func TestVisibleAgents(t *testing.T) {
	now := time.Now()
	recent := now.Add(-1 * time.Hour)
	old := now.Add(-100 * 24 * time.Hour)

	agents := []store.AgentState{
		{ID: "run", Status: store.StatusRunning},
		{ID: "succ-recent", Status: store.StatusSuccess, FinishedAt: &recent},
		{ID: "succ-old", Status: store.StatusSuccess, FinishedAt: &old},
		{ID: "kill-recent", Status: store.StatusKilled, FinishedAt: &recent},
		{ID: "fail-old", Status: store.StatusFailed, FinishedAt: &old},
	}

	got := visibleAgents(agents, false, 7)
	ids := map[string]bool{}
	for _, a := range got {
		ids[a.ID] = true
	}
	if !ids["run"] || !ids["succ-recent"] || !ids["kill-recent"] {
		t.Errorf("recent agents missing from visible set: %v", ids)
	}
	if ids["succ-old"] || ids["fail-old"] {
		t.Errorf("expired agents should be hidden: %v", ids)
	}

	// Running agents always come first.
	if len(got) == 0 || got[0].Status != store.StatusRunning {
		t.Errorf("running agent should be ordered first, got %+v", got)
	}

	all := visibleAgents(agents, true, 7)
	if len(all) != 5 {
		t.Errorf("showExpired should include all agents, got %d want 5", len(all))
	}
}

func TestGroupedVisibleAgents(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	agents := []store.AgentState{
		{ID: "a1", Name: "shared", PID: 1, Status: store.StatusRunning, StartedAt: base},
		{ID: "a2", Name: "shared", PID: 2, Status: store.StatusRunning, StartedAt: base.Add(time.Hour)},
		{ID: "solo", PID: 3, Status: store.StatusRunning, StartedAt: base},
	}

	groups := groupedVisibleAgents(agents, true, 7)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	var shared *AgentGroup
	for i := range groups {
		if groups[i].Rep.Name == "shared" {
			shared = &groups[i]
		}
	}
	if shared == nil {
		t.Fatal("shared group not found")
	}
	if len(shared.PIDs) != 2 {
		t.Errorf("shared group PIDs = %v, want 2 entries", shared.PIDs)
	}
	// Representative should be the most recently started (a2).
	if shared.Rep.ID != "a2" {
		t.Errorf("shared group rep = %q, want a2", shared.Rep.ID)
	}
}
