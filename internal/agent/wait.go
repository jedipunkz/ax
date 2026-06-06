package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jedipunkz/ax/internal/store"
)

// WaitResult reports how the wait ended. ExitCode is what the caller should
// propagate to the shell.
type WaitResult struct {
	Status   store.Status
	ExitCode int
}

// Wait blocks until the agent identified by idOrName reaches a terminal
// status, then returns the result. The exit code reflects the agent's own
// exit_code when available; Killed agents return 130 (SIGINT convention).
func Wait(socketPath, idOrName string) (WaitResult, error) {
	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return WaitResult{}, err
	}

	if existing.Status.IsTerminal() {
		return resultFromAgent(existing), nil
	}

	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return WaitResult{}, fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer client.Close()

	filter := &store.Filter{AgentIDs: []string{existing.ID}}
	if err := client.SubscribeWithFilter(filter); err != nil {
		return WaitResult{}, fmt.Errorf("could not subscribe: %w", err)
	}

	done := make(chan struct{})
	defer close(done)

	msgCh := make(chan store.Message)
	errCh := make(chan error, 1)
	go func() {
		for {
			msg, err := client.ReadMessage()
			if err != nil {
				select {
				case errCh <- err:
				case <-done:
				}
				return
			}
			select {
			case msgCh <- msg:
			case <-done:
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-msgCh:
			switch msg.Type {
			case "snapshot":
				for _, a := range msg.Agents {
					if a.ID == existing.ID && a.Status.IsTerminal() {
						return resultFromAgent(a), nil
					}
				}
			case "update":
				if msg.Agent != nil && msg.Agent.ID == existing.ID && msg.Agent.Status.IsTerminal() {
					return resultFromAgent(*msg.Agent), nil
				}
			case "remove":
				if msg.AgentID == existing.ID {
					return WaitResult{}, fmt.Errorf("agent %q was removed before reaching a terminal state", existing.ID)
				}
			}
		case err := <-errCh:
			if latest, latestErr := findAgentByExactID(existing.ID); latestErr == nil {
				if latest.Status == store.StatusRunning && !store.IsPIDAlive(latest.PID) {
					latest = staleAgentResult(latest)
				}
				if latest.Status.IsTerminal() {
					return resultFromAgent(latest), nil
				}
			}
			return WaitResult{}, fmt.Errorf("subscription closed: %w", err)
		case <-ticker.C:
			latest, latestErr := findAgentByExactID(existing.ID)
			if latestErr != nil {
				return WaitResult{}, latestErr
			}
			if latest.Status == store.StatusRunning && !store.IsPIDAlive(latest.PID) {
				latest = staleAgentResult(latest)
				_ = client.SendUpdate(latest)
			}
			if latest.Status.IsTerminal() {
				return resultFromAgent(latest), nil
			}
		}
	}
}

func staleAgentResult(a store.AgentState) store.AgentState {
	now := time.Now()
	exitCode := 1
	a.Status = store.StatusFailed
	a.FinishedAt = &now
	a.WaitingUser = false
	a.ExitCode = &exitCode
	return a
}

func findAgentByExactID(id string) (store.AgentState, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return store.AgentState{}, fmt.Errorf("could not determine home directory: %w", err)
	}
	stateFile := filepath.Join(home, ".ax", "state.json")
	agents, err := readAgents(stateFile)
	if err != nil {
		return store.AgentState{}, err
	}
	for _, a := range agents {
		if a.ID == id {
			return a, nil
		}
	}
	return store.AgentState{}, fmt.Errorf("agent %q was removed before reaching a terminal state", id)
}

// resultFromAgent maps an AgentState to its shell exit code:
//   - StatusSuccess → ExitCode (0 in practice, but we honour what was recorded)
//   - StatusFailed  → ExitCode if non-zero, else 1
//   - StatusKilled  → 130 (SIGINT convention) when no code was recorded
func resultFromAgent(a store.AgentState) WaitResult {
	r := WaitResult{Status: a.Status}
	switch a.Status {
	case store.StatusSuccess:
		if a.ExitCode != nil {
			r.ExitCode = *a.ExitCode
		}
	case store.StatusFailed:
		if a.ExitCode != nil && *a.ExitCode != 0 {
			r.ExitCode = *a.ExitCode
		} else {
			r.ExitCode = 1
		}
	case store.StatusKilled:
		if a.ExitCode != nil && *a.ExitCode != 0 {
			r.ExitCode = *a.ExitCode
		} else {
			r.ExitCode = 130
		}
	}
	return r
}
