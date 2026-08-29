// Package agxfs centralizes filesystem layout under ~/.agx/.
//
// Every file in the project that needs to construct a path under the
// user's agx data directory should go through this package so the layout
// has exactly one definition. Adding a new on-disk artifact means adding
// one helper here.
package agxfs

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// Root is the directory name under the user's home.
	Root = ".agx"

	socketFile   = "agx.sock"
	stateFile    = "state.json"
	pidFile      = "daemon.pid"
	lockFile     = "daemon.lock"
	configFile   = "agx.yaml"
	agentsDir    = "agents"
	worktreesDir = "worktrees"
	outputLog    = "output.log"
)

// Paths is a snapshot of the agx filesystem layout rooted at a specific
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
// Tests can use this with t.TempDir() to avoid touching the real ~/.agx.
func NewForHome(home string) Paths {
	return Paths{Home: home, Dir: filepath.Join(home, Root)}
}

// EnsureDir creates ~/.agx (and any missing parents).
func (p Paths) EnsureDir() error {
	if err := os.MkdirAll(p.Dir, 0700); err != nil {
		return fmt.Errorf("could not create %s: %w", p.Dir, err)
	}
	if err := os.Chmod(p.Dir, 0700); err != nil {
		return fmt.Errorf("could not secure %s: %w", p.Dir, err)
	}
	return nil
}

// Socket returns ~/.agx/agx.sock.
func (p Paths) Socket() string { return filepath.Join(p.Dir, socketFile) }

// StateFile returns ~/.agx/state.json.
func (p Paths) StateFile() string { return filepath.Join(p.Dir, stateFile) }

// PIDFile returns ~/.agx/daemon.pid.
func (p Paths) PIDFile() string { return filepath.Join(p.Dir, pidFile) }

// LockFile returns ~/.agx/daemon.lock, the advisory lock that guarantees at
// most one daemon owns this data directory.
func (p Paths) LockFile() string { return filepath.Join(p.Dir, lockFile) }

// ConfigFile returns ~/.agx/agx.yaml.
func (p Paths) ConfigFile() string { return filepath.Join(p.Dir, configFile) }

// AgentsDir returns ~/.agx/agents.
func (p Paths) AgentsDir() string { return filepath.Join(p.Dir, agentsDir) }

// AgentDir returns ~/.agx/agents/<id>.
func (p Paths) AgentDir(id string) string {
	return filepath.Join(p.AgentsDir(), id)
}

// AgentLog returns ~/.agx/agents/<id>/output.log.
func (p Paths) AgentLog(id string) string {
	return filepath.Join(p.AgentDir(id), outputLog)
}

// WorktreesDir returns ~/.agx/worktrees.
func (p Paths) WorktreesDir() string { return filepath.Join(p.Dir, worktreesDir) }

// WorktreePath returns the canonical worktree path for an agent.
func (p Paths) WorktreePath(repoName, agentID string) string {
	return filepath.Join(p.WorktreesDir(), repoName+"-"+agentID)
}

// Socket returns ~/.agx/agx.sock for the default home — a shortcut for
// the very common case of "I just need the socket path".
func Socket() (string, error) {
	p, err := New()
	if err != nil {
		return "", err
	}
	return p.Socket(), nil
}
