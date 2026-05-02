# Fish shell completion for ax - Claude Code agent manager

# Helper: list all agent IDs and names from state.json
function __ax_agents
    set -l state_file ~/.ax/state.json
    if test -f $state_file
        command jq -r '.[] | if .name != "" then "\(.id)\t\(.name)" else "\(.id)\tagent" end' $state_file 2>/dev/null
    end
end

# Disable file completion globally for ax
complete -c ax -f

# Top-level subcommands
complete -c ax -n 'not __fish_seen_subcommand_from agent dash completion' -a agent      -d 'Manage Claude Code agents'
complete -c ax -n 'not __fish_seen_subcommand_from agent dash completion' -a dash       -d 'Show TUI dashboard of all agents'
complete -c ax -n 'not __fish_seen_subcommand_from agent dash completion' -a completion -d 'Generate autocompletion scripts'

# ax agent subcommands
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a new    -d 'Start a new agent session'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a resume -d 'Resume a previous agent session by ID or name'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a cd     -d 'Print the worktree path of an agent'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a list   -d 'List all agents'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a ls     -d 'List all agents (alias)'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a remove -d 'Remove an agent and its worktree'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a rm     -d 'Remove an agent and its worktree (alias)'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume cd list ls remove rm diff' -a diff   -d 'Show git diff for an agent worktree'

# ax agent new: -a/--agent flag for agent type, -n/--name flag, and agent-specific options
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from new' -s a -s m -l agent -d 'Agent binary to use' -r -a 'claude\tClaude\ Code codex\tOpenAI\ Codex gemini\tGoogle\ Gemini opencode\tOpenCode'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from new' -s n -l name  -d 'Name for the agent' -r

# ax agent new: claude-specific options
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from new' -l dangerously-skip-permissions -d '[claude] Skip permission prompts'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from new' -l enable-auto-mode             -d '[claude] Enable auto mode'

# ax agent resume: -a/--agent flag, required -n/--name with dynamic completions, and agent-specific options
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -s a -s m -l agent -d 'Override agent binary' -r -a 'claude\tClaude\ Code codex\tOpenAI\ Codex gemini\tGoogle\ Gemini opencode\tOpenCode'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -s n -l name  -d 'Agent ID or name to resume' -r -a '(__ax_agents)'

# ax agent resume: claude-specific options
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -l dangerously-skip-permissions -d '[claude] Skip permission prompts'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -l enable-auto-mode             -d '[claude] Enable auto mode'

# ax agent cd, remove/rm, diff: -n/--name with dynamic completions
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from cd'     -s n -l name -d 'Agent ID or name' -r -a '(__ax_agents)'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from remove' -s n -l name -d 'Agent ID or name' -r -a '(__ax_agents)'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from rm'     -s n -l name -d 'Agent ID or name' -r -a '(__ax_agents)'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from diff'   -s n -l name -d 'Agent ID or name' -r -a '(__ax_agents)'

# ax completion subcommands
complete -c ax -n '__fish_seen_subcommand_from completion' -a bash       -d 'Generate bash completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a fish       -d 'Generate fish completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a zsh        -d 'Generate zsh completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a powershell -d 'Generate powershell completion script'

# Global help flag
complete -c ax -s h -l help -d 'Show help'
