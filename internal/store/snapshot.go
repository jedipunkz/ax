package store

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadAgents loads the AgentState slice persisted at path. It is used by
// CLI commands (list, remove, wait, resume) that read state directly
// instead of talking to the daemon.
//
// A missing file is reported via os.IsNotExist on the returned error so
// callers can distinguish "no agents yet" from real I/O failures.
func ReadAgents(path string) ([]AgentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var agents []AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("could not parse state file: %w", err)
	}
	return agents, nil
}

// WriteAgents persists the AgentState slice to path atomically using
// a sibling .tmp file followed by os.Rename so partial writes are not
// observable by readers.
func WriteAgents(path string, agents []AgentState) error {
	data, err := json.Marshal(agents)
	if err != nil {
		return fmt.Errorf("could not marshal state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("could not write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("could not rename state file: %w", err)
	}
	return nil
}
