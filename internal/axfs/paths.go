// Package axfs centralizes filesystem layout under ~/.ax/.
//
// Every file in the project that needs to construct a path under the
// user's ax data directory should go through this package so the layout
// has exactly one definition. Adding a new on-disk artifact means adding
// one helper here.
package axfs

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// Root is the directory name under the user's home.
	Root = ".ax"

	socketFile   = "ax.sock"
	stateFile    = "state.json"
	pidFile      = "daemon.pid"
	configFile   = "ax.yaml"
	agentsDir    = "agents"
	worktreesDir = "worktrees"
	outputLog    = "output.log"
)

// Paths is a snapshot of the ax filesystem layout rooted at a specific
// home directory. Construct one with New (or NewForHome in tests) and
// pass it around instead of recomputing paths everywhere.
type Paths struct {
	Home string
	Dir  string
}

// New builds Paths from os.UserHomeDir.
func New() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("could not determine home directory: %w", err)
	}
	return NewForHome(home), nil
}

// NewForHome builds Paths rooted under the given home directory.
// Tests can use this with t.TempDir() to avoid touching the real ~/.ax.
func NewForHome(home string) Paths {
	return Paths{Home: home, Dir: filepath.Join(home, Root)}
}

// EnsureDir creates ~/.ax (and any missing parents).
func (p Paths) EnsureDir() error {
	if err := os.MkdirAll(p.Dir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", p.Dir, err)
	}
	return nil
}

// Socket returns ~/.ax/ax.sock.
func (p Paths) Socket() string { return filepath.Join(p.Dir, socketFile) }

// StateFile returns ~/.ax/state.json.
func (p Paths) StateFile() string { return filepath.Join(p.Dir, stateFile) }

// PIDFile returns ~/.ax/daemon.pid.
func (p Paths) PIDFile() string { return filepath.Join(p.Dir, pidFile) }

// ConfigFile returns ~/.ax/ax.yaml.
func (p Paths) ConfigFile() string { return filepath.Join(p.Dir, configFile) }

// AgentsDir returns ~/.ax/agents.
func (p Paths) AgentsDir() string { return filepath.Join(p.Dir, agentsDir) }

// AgentDir returns ~/.ax/agents/<id>.
func (p Paths) AgentDir(id string) string {
	return filepath.Join(p.AgentsDir(), id)
}

// AgentLog returns ~/.ax/agents/<id>/output.log.
func (p Paths) AgentLog(id string) string {
	return filepath.Join(p.AgentDir(id), outputLog)
}

// WorktreesDir returns ~/.ax/worktrees.
func (p Paths) WorktreesDir() string { return filepath.Join(p.Dir, worktreesDir) }

// WorktreePath returns the canonical worktree path for an agent.
func (p Paths) WorktreePath(repoName, agentID string) string {
	return filepath.Join(p.WorktreesDir(), repoName+"-"+agentID)
}

// Socket returns ~/.ax/ax.sock for the default home — a shortcut for
// the very common case of "I just need the socket path".
func Socket() (string, error) {
	p, err := New()
	if err != nil {
		return "", err
	}
	return p.Socket(), nil
}
