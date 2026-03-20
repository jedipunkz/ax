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
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume' -a new    -d 'Start a new Claude Code agent'
complete -c ax -n '__fish_seen_subcommand_from agent; and not __fish_seen_subcommand_from new resume' -a resume -d 'Resume a previous agent session by ID or name'

# ax agent new: optional -n/--name flag
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from new' -s n -l name -d 'Name for the agent' -r

# ax agent resume: required -n/--name flag with dynamic agent completions
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -s n -l name -d 'Agent ID or name' -r -a '(__ax_agents)'

# ax completion subcommands
complete -c ax -n '__fish_seen_subcommand_from completion' -a bash       -d 'Generate bash completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a fish       -d 'Generate fish completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a zsh        -d 'Generate zsh completion script'
complete -c ax -n '__fish_seen_subcommand_from completion' -a powershell -d 'Generate powershell completion script'

# Global help flag
complete -c ax -s h -l help -d 'Show help'
