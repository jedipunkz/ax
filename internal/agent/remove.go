package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
)

// RemoveAgent removes the worktree, log file, and state entry for the agent
// identified by idOrName. The agent must be in a terminal state.
// If socketPath is non-empty and the daemon is reachable, state removal is
// delegated to the daemon; otherwise state.json is updated directly.
func RemoveAgent(idOrName, socketPath string) error {
	paths, err := axfs.New()
	if err != nil {
		return err
	}
	stateFile := paths.StateFile()

	agents, err := readAgents(stateFile)
	if err != nil {
		return err
	}

	target, idx := FindAgent(agents, idOrName)
	if idx < 0 {
		return fmt.Errorf("agent %q not found", idOrName)
	}
	if !target.Status.IsTerminal() {
		return fmt.Errorf("agent %s is still running; stop it before removing", target.ID)
	}

	if err := RemoveAgentArtifacts(paths, target); err != nil {
		// Match prior CLI behavior: artifact failures are surfaced as
		// warnings but never block state removal.
		warnf("%v", err)
	}

	// Remove agent from state
	if socketPath != "" {
		var c store.Client
		if err := c.Connect(socketPath); err == nil {
			defer c.Close()
			if err := c.SendRemove(target.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not notify daemon: %v\n", err)
			}
			fmt.Printf("removed agent %s\n", target.ID)
			return nil
		}
	}

	// Daemon not reachable — update state.json directly.
	updated := append(agents[:idx], agents[idx+1:]...)
	if err := writeAgents(stateFile, updated); err != nil {
		return err
	}

	fmt.Printf("removed agent %s\n", target.ID)
	return nil
}

// RemoveAgentArtifacts deletes the worktree (if it lives under
// ~/.ax/worktrees/) and the log file/directory for the given agent.
// Returns the first failure (worktree errors take precedence over log
// errors) so callers can decide whether to surface it; cleanup of the
// remaining artifacts proceeds regardless.
//
// Shared by `ax agent remove` and the dashboard's deletion flow.
func RemoveAgentArtifacts(paths axfs.Paths, ag store.AgentState) error {
	var firstErr error
	if ag.WorkDir != "" && IsUnderWorktreesDir(paths.WorktreesDir(), ag.WorkDir) {
		cleanWorkDir := filepath.Clean(ag.WorkDir)
		if _, err := os.Stat(cleanWorkDir); err == nil {
			if err := RemoveWorktree(cleanWorkDir); err != nil {
				firstErr = fmt.Errorf("worktree remove %s: %w", cleanWorkDir, err)
			}
		}
	}
	if ag.LogFile != "" {
		if err := os.Remove(ag.LogFile); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = fmt.Errorf("log remove %s: %w", ag.LogFile, err)
		}
		_ = os.Remove(filepath.Dir(ag.LogFile))
	}
	return firstErr
}

// readAgents wraps store.ReadAgents with the package's "no agents found"
// error for missing files. Other I/O failures bubble up unchanged.
func readAgents(stateFile string) ([]store.AgentState, error) {
	agents, err := store.ReadAgents(stateFile)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no agents found")
	}
	if err != nil {
		return nil, fmt.Errorf("could not read state file: %w", err)
	}
	return agents, nil
}

// writeAgents is a thin alias kept so the package's read/write helpers
// remain a matched pair at the call sites.
func writeAgents(stateFile string, agents []store.AgentState) error {
	return store.WriteAgents(stateFile, agents)
}
