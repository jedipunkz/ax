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
	ID             string     `json:"id"`
	Name           string     `json:"name,omitempty"`       // optional human-readable name
	AgentType      string     `json:"agent_type,omitempty"` // agent binary name (e.g. "claude", "codex"); defaults to "claude" when empty
	PID            int        `json:"pid"`
	Args           []string   `json:"args"`
	WorkDir        string     `json:"work_dir"`
	Status         Status     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	ExitCode       *int       `json:"exit_code,omitempty"`
	LastOutput     string     `json:"last_output"`
	LogFile        string     `json:"log_file"`
	WaitingUser    bool       `json:"waiting_user,omitempty"`    // true when agent is waiting for user input
	WorktreeBranch string     `json:"worktree_branch,omitempty"` // git worktree branch name, if any
	RepoName       string     `json:"repo_name,omitempty"`       // original repository name where the agent was started
	Commits        []string   `json:"commits,omitempty"`         // git commit hashes made during this session
}

// AgentTypeName returns the agent binary name, defaulting to "claude" for
// sessions created before AgentType was introduced.
func (a AgentState) AgentTypeName() string {
	if a.AgentType == "" {
		return "claude"
	}
	return a.AgentType
}

// IsTerminal returns true if the status is a terminal (non-running) state.
func (s Status) IsTerminal() bool {
	return s == StatusKilled || s == StatusSuccess || s == StatusFailed
}

// Message is the JSON-lines protocol message used over the Unix socket.
//
// Type values currently in use:
//   - Client → Daemon: "update", "remove", "subscribe", "attach", "detach",
//     "metrics"
//   - Daemon → Client: "snapshot", "update", "remove",
//     "attach_ok", "attach_err", "output", "eof", "metrics_result",
//     "metrics_err"
//
// Fields are populated as required by the message type; all are omitempty so
// older payloads remain forward-compatible.
type Message struct {
	Type    string       `json:"type"`
	Agent   *AgentState  `json:"agent,omitempty"`
	Agents  []AgentState `json:"agents,omitempty"`
	AgentID string       `json:"agent_id,omitempty"`
	Data    string       `json:"data,omitempty"`   // base64 payload for "output" (and future "input")
	Error   string       `json:"error,omitempty"`  // populated on "*_err" responses
	Tail    int          `json:"tail,omitempty"`   // attach: max bytes of log history to replay
	Filter  *Filter      `json:"filter,omitempty"` // subscribe: limit fan-out to matching agents

	DurationSec int64 `json:"duration_sec,omitempty"` // metrics_result: elapsed runtime in seconds
	CommitCount int   `json:"commits,omitempty"`      // metrics_result: number of recorded commits
	OutputLines int64 `json:"output_lines,omitempty"` // metrics_result: raw log line count
}

// Filter narrows which state events a subscriber receives. An empty Filter
// matches everything; a nil Filter likewise. Conditions are AND-combined.
type Filter struct {
	AgentIDs []string `json:"agent_ids,omitempty"`
	Statuses []Status `json:"statuses,omitempty"`
}

// Match reports whether msg passes the filter. nil filters accept everything.
func (f *Filter) Match(msg Message) bool {
	if f == nil {
		return true
	}
	id := msg.AgentID
	if msg.Agent != nil {
		id = msg.Agent.ID
	}
	if !f.matchesAgentID(id) {
		return false
	}
	if msg.Agent == nil {
		return len(f.Statuses) == 0
	}
	return f.matchesStatus(msg.Agent.Status)
}

// MatchAgent reports whether the given agent passes the filter's AgentID and
// Status constraints, used when filtering snapshot payloads.
func (f *Filter) MatchAgent(a AgentState) bool {
	if f == nil {
		return true
	}
	return f.matchesAgentID(a.ID) && f.matchesStatus(a.Status)
}

func (f *Filter) matchesAgentID(id string) bool {
	if len(f.AgentIDs) == 0 {
		return true
	}
	if id == "" {
		return false
	}
	for _, want := range f.AgentIDs {
		if want == id {
			return true
		}
	}
	return false
}

func (f *Filter) matchesStatus(status Status) bool {
	if len(f.Statuses) == 0 {
		return true
	}
	for _, want := range f.Statuses {
		if want == status {
			return true
		}
	}
	return false
}
