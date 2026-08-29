package tui

// Agent visibility, grouping, and fuzzy filtering. This is the list view's
// data model: it turns the raw agent snapshot from the daemon into the ordered
// groups the renderer draws. No rendering, no terminal widths.

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedipunkz/agx/internal/store"
)

// recentThreshold returns the cutoff time for finished agents based on the configured duration in days.
func recentThreshold(days int) time.Time {
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
}

// visibleAgents returns the agents to display, in order: running (all), then success, then killed.
// When showExpired is true, all finished agents are included regardless of age.
func visibleAgents(agents []store.AgentState, showExpired bool, days int) []store.AgentState {
	threshold := recentThreshold(days)
	recentEnough := func(a store.AgentState) bool {
		return showExpired || (a.FinishedAt != nil && a.FinishedAt.After(threshold))
	}
	var running, success, killed []store.AgentState
	for _, a := range agents {
		switch a.Status {
		case store.StatusRunning:
			running = append(running, a)
		case store.StatusSuccess:
			if recentEnough(a) {
				success = append(success, a)
			}
		case store.StatusKilled, store.StatusFailed:
			if recentEnough(a) {
				killed = append(killed, a)
			}
		}
	}
	return append(append(running, success...), killed...)
}

// AgentGroup represents one or more agents sharing the same name/label (e.g. after -r resume).
type AgentGroup struct {
	Rep  store.AgentState // best representative (running > success > killed, then most recent)
	PIDs []int            // all PIDs across agents in this group
}

// groupLabel returns the display label for this group.
func (g AgentGroup) groupLabel() string {
	return g.Rep.Label()
}

// pidString returns comma-separated PIDs.
func (g AgentGroup) pidString() string {
	parts := make([]string, len(g.PIDs))
	for i, p := range g.PIDs {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ", ")
}

// isBetterRep returns true if candidate should replace current as the group representative.
// Priority order: running > success > killed/failed, then most recently started.
func isBetterRep(candidate, current store.AgentState) bool {
	statusPriority := func(s store.Status) int {
		switch s {
		case store.StatusRunning:
			return 3
		case store.StatusSuccess:
			return 2
		default:
			return 1
		}
	}
	cp := statusPriority(candidate.Status)
	rp := statusPriority(current.Status)
	if cp != rp {
		return cp > rp
	}
	return candidate.StartedAt.After(current.StartedAt)
}

// fuzzyMatch returns true if all runes of pattern appear in text in order (case-insensitive).
func fuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)
	pi := 0
	patRunes := []rune(pattern)
	for _, c := range text {
		if pi < len(patRunes) && patRunes[pi] == c {
			pi++
		}
	}
	return pi == len(patRunes)
}

// fuzzyFilterGroups returns groups whose label matches the fuzzy query.
func fuzzyFilterGroups(groups []AgentGroup, query string) []AgentGroup {
	if query == "" {
		return groups
	}
	var result []AgentGroup
	for _, g := range groups {
		if fuzzyMatch(query, g.groupLabel()) {
			result = append(result, g)
		}
	}
	return result
}

// groupedVisibleAgents groups visible agents by name/label into AgentGroups.
// Agents sharing the same Name (or the same ID when no Name is set) are merged into one group.
func groupedVisibleAgents(agents []store.AgentState, showExpired bool, days int) []AgentGroup {
	visible := visibleAgents(agents, showExpired, days)
	groupMap := map[string]*AgentGroup{}
	var order []string
	for _, a := range visible {
		a := a
		key := a.Label()
		if g, ok := groupMap[key]; ok {
			g.PIDs = append(g.PIDs, a.PID)
			if isBetterRep(a, g.Rep) {
				g.Rep = a
			}
		} else {
			groupMap[key] = &AgentGroup{Rep: a, PIDs: []int{a.PID}}
			order = append(order, key)
		}
	}
	result := make([]AgentGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groupMap[key])
	}
	return result
}

// groupsByStatus splits groups into the three display sections in order.
// Failed groups are shown in the killed section.
func groupsByStatus(groups []AgentGroup) (running, success, killed []AgentGroup) {
	for _, g := range groups {
		switch g.Rep.Status {
		case store.StatusRunning:
			running = append(running, g)
		case store.StatusSuccess:
			success = append(success, g)
		default:
			killed = append(killed, g)
		}
	}
	return running, success, killed
}
