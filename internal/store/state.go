package store

import "time"

// Status represents the current state of an agent.
type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusKilled  Status = "killed"
)

// AgentState holds all information about a running or completed agent.
type AgentState struct {
	ID          string     `json:"id"`
	Name        string     `json:"name,omitempty"`            // optional human-readable name
	AgentType   string     `json:"agent_type,omitempty"`      // agent binary name (e.g. "claude", "codex"); defaults to "claude" when empty
	PID         int        `json:"pid"`
	Args        []string   `json:"args"`
	WorkDir     string     `json:"work_dir"`
	Status      Status     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	LastOutput  string     `json:"last_output"`
	LogFile     string     `json:"log_file"`
	WaitingUser    bool     `json:"waiting_user,omitempty"`    // true when agent is waiting for user input
	WorktreeBranch string   `json:"worktree_branch,omitempty"` // git worktree branch name, if any
	RepoName       string   `json:"repo_name,omitempty"`       // original repository name where the agent was started
	Commits        []string `json:"commits,omitempty"`         // git commit hashes made during this session
}

// IsTerminal returns true if the status is a terminal (non-running) state.
func (s Status) IsTerminal() bool {
	return s == StatusKilled || s == StatusSuccess || s == StatusFailed
}

// Message is the JSON-lines protocol message used over the Unix socket.
type Message struct {
	Type    string       `json:"type"`
	Agent   *AgentState  `json:"agent,omitempty"`
	Agents  []AgentState `json:"agents,omitempty"`
	AgentID string       `json:"agent_id,omitempty"`
}
