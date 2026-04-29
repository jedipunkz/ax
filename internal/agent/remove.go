package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedipunkz/ax/internal/store"
)

// RemoveAgent removes the worktree, log file, and state entry for the agent
// identified by idOrName. The agent must be in a terminal state.
// If socketPath is non-empty and the daemon is reachable, state removal is
// delegated to the daemon; otherwise state.json is updated directly.
func RemoveAgent(idOrName, socketPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	stateFile := filepath.Join(home, ".ax", "state.json")

	agents, err := readAgents(stateFile)
	if err != nil {
		return err
	}

	target, idx := findAgent(agents, idOrName)
	if idx < 0 {
		return fmt.Errorf("agent %q not found", idOrName)
	}
	if !target.Status.IsTerminal() {
		return fmt.Errorf("agent %s is still running; stop it before removing", target.ID)
	}

	// Remove worktree directory if it lives under ~/.ax/worktrees/
	worktreesDir := filepath.Join(home, ".ax", "worktrees")
	if target.WorkDir != "" {
		cleanWorktrees := filepath.Clean(worktreesDir)
		cleanWorkDir := filepath.Clean(target.WorkDir)
		if strings.HasPrefix(cleanWorkDir, cleanWorktrees+string(filepath.Separator)) {
			if _, err := os.Stat(cleanWorkDir); err == nil {
				if err := RemoveWorktree(cleanWorkDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not remove worktree %s: %v\n", cleanWorkDir, err)
				}
			}
		}
	}

	// Remove log file
	if target.LogFile != "" {
		if err := os.Remove(target.LogFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: could not remove log file %s: %v\n", target.LogFile, err)
		}
		// Remove parent agents/<id> directory if empty
		logDir := filepath.Dir(target.LogFile)
		_ = os.Remove(logDir)
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

// findAgent returns the agent matching idOrName (by ID prefix or exact name) and its index.
func findAgent(agents []store.AgentState, idOrName string) (store.AgentState, int) {
	// Exact ID match first
	for i, a := range agents {
		if a.ID == idOrName {
			return a, i
		}
	}
	// Exact name match
	for i, a := range agents {
		if a.Name == idOrName {
			return a, i
		}
	}
	// ID prefix match
	for i, a := range agents {
		if strings.HasPrefix(a.ID, idOrName) {
			return a, i
		}
	}
	return store.AgentState{}, -1
}

func readAgents(stateFile string) ([]store.AgentState, error) {
	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no agents found")
	}
	if err != nil {
		return nil, fmt.Errorf("could not read state file: %w", err)
	}
	var agents []store.AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("could not parse state file: %w", err)
	}
	return agents, nil
}

func writeAgents(stateFile string, agents []store.AgentState) error {
	data, err := json.Marshal(agents)
	if err != nil {
		return fmt.Errorf("could not marshal state: %w", err)
	}
	tmp := stateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("could not write state: %w", err)
	}
	if err := os.Rename(tmp, stateFile); err != nil {
		return fmt.Errorf("could not rename state file: %w", err)
	}
	return nil
}
