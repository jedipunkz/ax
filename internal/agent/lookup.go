package agent

import (
	"fmt"
	"strings"

	"github.com/jedipunkz/agx/internal/agxfs"
	"github.com/jedipunkz/agx/internal/store"
)

// FindAgent locates an agent in the given slice using the following
// priority order. The first non-empty bucket wins, and within a bucket
// the most recently started agent is returned when more than one
// candidate matches.
//
//  1. Exact ID match
//  2. Exact Name match
//  3. ID prefix match
//  4. WorktreeBranch == sanitizeBranchName(idOrName)
//
// Returns -1 as the index when nothing matched.
func FindAgent(agents []store.AgentState, idOrName string) (store.AgentState, int) {
	if idOrName == "" {
		return store.AgentState{}, -1
	}

	// 1. Exact ID — IDs are unique so any match is the answer.
	for i := range agents {
		if agents[i].ID == idOrName {
			return agents[i], i
		}
	}

	// 2. Exact Name — pick the most recently started.
	if state, idx, ok := pickMostRecent(agents, func(a store.AgentState) bool {
		return a.Name == idOrName
	}); ok {
		return state, idx
	}

	// 3. ID prefix — pick the most recently started; takes precedence over
	//    branch-name matching because prefix matches are more intentional.
	if state, idx, ok := pickMostRecent(agents, func(a store.AgentState) bool {
		return strings.HasPrefix(a.ID, idOrName)
	}); ok {
		return state, idx
	}

	// 4. Sanitized branch name — last-chance fallback so users can refer to
	//    an agent by the branch the runner created from their --name input.
	if sanitized := sanitizeBranchName(idOrName); sanitized != "" {
		if state, idx, ok := pickMostRecent(agents, func(a store.AgentState) bool {
			return a.WorktreeBranch == sanitized
		}); ok {
			return state, idx
		}
	}

	return store.AgentState{}, -1
}

// pickMostRecent returns the most recently started agent for which pred
// returns true, along with its index in the slice.
func pickMostRecent(agents []store.AgentState, pred func(store.AgentState) bool) (store.AgentState, int, bool) {
	bestIdx := -1
	for i := range agents {
		if !pred(agents[i]) {
			continue
		}
		if bestIdx < 0 || agents[i].StartedAt.After(agents[bestIdx].StartedAt) {
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return store.AgentState{}, -1, false
	}
	return agents[bestIdx], bestIdx, true
}

// findAgentByIDOrName reads state.json and returns the first match for
// idOrName using FindAgent's priority order.
func findAgentByIDOrName(idOrName string) (store.AgentState, error) {
	paths, err := agxfs.New()
	if err != nil {
		return store.AgentState{}, err
	}
	agents, err := store.ReadAgents(paths.StateFile())
	if err != nil {
		return store.AgentState{}, fmt.Errorf("could not read state file: %w", err)
	}
	state, idx := FindAgent(agents, idOrName)
	if idx < 0 {
		return store.AgentState{}, fmt.Errorf("no agent found with ID or name %q", idOrName)
	}
	return state, nil
}
