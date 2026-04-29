package agent

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedipunkz/ax/internal/store"
)

// readAgents reads and parses the agent state file.
// Returns (nil, nil) if the file does not exist.
func readAgents(stateFile string) ([]store.AgentState, error) {
	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return nil, nil
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

// writeAgents atomically writes the agent list to stateFile via a temp file.
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
