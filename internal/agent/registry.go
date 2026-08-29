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

// ResumeCommand returns a copy of the arguments used to resume an agent.
// Keeping the copy at the registry boundary prevents callers from modifying
// the shared definitions in knownAgents by appending to the returned slice.
func (d AgentDef) ResumeCommand() []string {
	return append([]string(nil), d.ResumeArgs...)
}

// knownAgents is the registry of supported agent binaries.
//
// Each entry documents the agent's own session-continuation interface.
// agx has already identified the session by ID or name before this point, and
// every agent runs in its own worktree, so each entry resolves to the most
// recent session in that directory rather than opening the agent's picker:
//
//	claude    --continue         (most recent conversation in the cwd)
//	gemini    --resume latest    (resumes most recent session; v0.20.0+)
//	codex     resume --last      (picker without --last)
//	opencode  --continue
var knownAgents = map[string]AgentDef{
	"claude":   {ResumeArgs: []string{"--continue"}},
	"gemini":   {ResumeArgs: []string{"--resume", "latest"}},
	"codex":    {ResumeArgs: []string{"resume", "--last"}},
	"opencode": {ResumeArgs: []string{"--continue"}},
}

// lookupAgent returns the AgentDef for the named agent binary.
// Unknown agents return a zero-value AgentDef (ResumeArgs == nil).
func lookupAgent(name string) AgentDef {
	return knownAgents[name]
}
