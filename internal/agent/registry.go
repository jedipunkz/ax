package agent

// AgentDef holds the static configuration for a known agent binary.
// Add a new entry to knownAgents to register additional agent types; no
// other code changes are required.
type AgentDef struct {
	// ResumeArgs are the arguments prepended to the command line when
	// resuming a previous session. Nil means the agent does not natively
	// support session resume and will simply be relaunched in the existing
	// worktree.
	ResumeArgs []string
}

// knownAgents is the registry of supported agent binaries.
//
// Each entry documents the agent's own session-continuation interface:
//
//	claude    --resume           (opens interactive session picker)
//	gemini    --resume latest    (resumes most recent session; v0.20.0+)
//	codex     resume --last
//	opencode  --continue
var knownAgents = map[string]AgentDef{
	"claude":   {ResumeArgs: []string{"--resume"}},
	"gemini":   {ResumeArgs: []string{"--resume", "latest"}},
	"codex":    {ResumeArgs: []string{"resume", "--last"}},
	"opencode": {ResumeArgs: []string{"--continue"}},
}

// lookupAgent returns the AgentDef for the named agent binary.
// Unknown agents return a zero-value AgentDef (ResumeArgs == nil).
func lookupAgent(name string) AgentDef {
	return knownAgents[name]
}
