# Fish shell completion for ax - AI coding agent manager

# Helper: list all agent IDs and names from state.json
function __ax_agents
    set -l state_file ~/.ax/state.json
    if test -f $state_file
        command jq -r '.[] | if .name != "" then "\(.id)\t\(.name)" else "\(.id)\tagent" end' $state_file 2>/dev/null
    end
end

function __ax_agent_start_cmd
    __fish_seen_subcommand_from agent
    and __fish_seen_subcommand_from new resume
end

function __ax_agent_arg_flags
    printf '%s\n' \
        '--add-dir	[claude/codex] Add an accessible directory' \
        '--agent	[claude/opencode] Agent to use' \
        '--agents	[claude] JSON object defining custom agents' \
        '--allow-dangerously-skip-permissions	[claude] Enable permission bypass as an option' \
        '--allowedTools	[claude] Tool names to allow' \
        '--allowed-tools	[claude] Tool names to allow' \
        '--append-system-prompt	[claude] Append to the system prompt' \
        '--ask-for-approval	[codex] Approval policy' \
        '--bare	[claude] Minimal mode' \
        '--betas	[claude] Beta headers for API requests' \
        '--brief	[claude] Enable SendUserMessage tool' \
        '--cd	[codex] Working root directory' \
        '--chrome	[claude] Enable Claude in Chrome integration' \
        '--config	[codex] Override config key=value' \
        '--continue	[claude/opencode] Continue the most recent session' \
        '--cors	[opencode] Additional CORS domain' \
        '--dangerously-bypass-approvals-and-sandbox	[codex] Skip approvals and sandbox' \
        '--dangerously-bypass-hook-trust	[codex] Run hooks without persisted trust' \
        '--dangerously-skip-permissions	[claude] Bypass all permission checks' \
        '--debug	[claude] Enable debug mode with optional filter' \
        '--debug-file	[claude] Write debug logs to a file' \
        '--disable	[codex] Disable a feature' \
        '--disable-slash-commands	[claude] Disable all skills' \
        '--disallowedTools	[claude] Tool names to deny' \
        '--disallowed-tools	[claude] Tool names to deny' \
        '--effort	[claude] Effort level' \
        '--enable	[codex] Enable a feature' \
        '--exclude-dynamic-system-prompt-sections	[claude] Move dynamic system prompt sections into first user message' \
        '--fallback-model	[claude] Fallback model for print mode' \
        '--file	[claude] File resource spec to download at startup' \
        '--fork	[opencode] Fork the session when continuing' \
        '--fork-session	[claude] Fork when resuming' \
        '--from-pr	[claude] Resume a session linked to a PR' \
        '--hostname	[opencode] Hostname to listen on' \
        '--ide	[claude] Automatically connect to IDE' \
        '--image	[codex] Attach image to initial prompt' \
        '--include-hook-events	[claude] Include hook events in stream-json output' \
        '--include-partial-messages	[claude] Include partial stream-json message chunks' \
        '--input-format	[claude] Input format for print mode' \
        '--json-schema	[claude] JSON Schema for structured output' \
        '--local-provider	[codex] Local OSS provider' \
        '--log-level	[opencode] Log level' \
        '--max-budget-usd	[claude] Maximum API spend for print mode' \
        '--mcp-config	[claude] MCP config JSON file or string' \
        '--mcp-debug	[claude] Enable MCP debug mode' \
        '--mdns	[opencode] Enable mDNS service discovery' \
        '--mdns-domain	[opencode] Custom mDNS domain name' \
        '--model	[claude/codex/opencode] Model to use' \
        '--name	[claude] Display name for this session' \
        '--no-alt-screen	[codex] Disable alternate screen mode' \
        '--no-chrome	[claude] Disable Claude in Chrome integration' \
        '--no-session-persistence	[claude] Disable session persistence' \
        '--oss	[codex] Use open-source provider' \
        '--output-format	[claude] Output format for print mode' \
        '--permission-mode	[claude] Permission mode' \
        '--plugin-dir	[claude] Plugin directory or zip' \
        '--plugin-url	[claude] Plugin zip URL' \
        '--port	[opencode] Port to listen on' \
        '--print	[claude] Print response and exit' \
        '--print-logs	[opencode] Print logs to stderr' \
        '--profile	[codex] Config profile' \
        '--prompt	[opencode] Prompt to use' \
        '--pure	[opencode] Run without external plugins' \
        '--remote	[codex] Connect TUI to remote app server' \
        '--remote-auth-token-env	[codex] Env var containing remote auth token' \
        '--remote-control	[claude] Start with Remote Control enabled' \
        '--remote-control-session-name-prefix	[claude] Prefix for Remote Control session names' \
        '--replay-user-messages	[claude] Re-emit stream-json user messages' \
        '--resume	[claude] Resume a conversation' \
        '--sandbox	[codex] Sandbox policy' \
        '--search	[codex] Enable live web search' \
        '--session	[opencode] Session ID to continue' \
        '--session-id	[claude] Use a specific session UUID' \
        '--setting-sources	[claude] Setting sources to load' \
        '--settings	[claude] Settings JSON file or string' \
        '--strict-config	[codex] Error on unrecognized config fields' \
        '--strict-mcp-config	[claude] Only use MCP servers from --mcp-config' \
        '--system-prompt	[claude] System prompt for the session' \
        '--tmux	[claude] Create a tmux session for the worktree' \
        '--tools	[claude] Available built-in tools' \
        '--verbose	[claude] Enable verbose mode' \
        '--worktree	[claude] Create a new git worktree'
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

# ax agent resume: -a/--agent flag, required -n/--name with dynamic completions, and agent-specific options
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -s a -s m -l agent -d 'Override agent binary' -r -a 'claude\tClaude\ Code codex\tOpenAI\ Codex gemini\tGoogle\ Gemini opencode\tOpenCode'
complete -c ax -n '__fish_seen_subcommand_from agent; and __fish_seen_subcommand_from resume' -s n -l name  -d 'Agent ID or name to resume' -r -a '(__ax_agents)'

# Fish suppresses option completions after `--`; ax forwards that segment to
# the selected agent, so expose the same agent flags as argument candidates too.
complete -c ax -n '__ax_agent_start_cmd' -a '(__ax_agent_arg_flags)'

# ax agent new/resume: Claude Code options
complete -c ax -n '__ax_agent_start_cmd' -l add-dir -d '[claude] Add additional directories for tool access' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l agent -d '[claude] Agent for the current session' -r
complete -c ax -n '__ax_agent_start_cmd' -l agents -d '[claude] JSON object defining custom agents' -r
complete -c ax -n '__ax_agent_start_cmd' -l allow-dangerously-skip-permissions -d '[claude] Enable permission bypass as an option'
complete -c ax -n '__ax_agent_start_cmd' -l allowedTools -l allowed-tools -d '[claude] Tool names to allow' -r
complete -c ax -n '__ax_agent_start_cmd' -l append-system-prompt -d '[claude] Append to the system prompt' -r
complete -c ax -n '__ax_agent_start_cmd' -l bare -d '[claude] Minimal mode'
complete -c ax -n '__ax_agent_start_cmd' -l betas -d '[claude] Beta headers for API requests' -r
complete -c ax -n '__ax_agent_start_cmd' -l brief -d '[claude] Enable SendUserMessage tool'
complete -c ax -n '__ax_agent_start_cmd' -l chrome -d '[claude] Enable Claude in Chrome integration'
complete -c ax -n '__ax_agent_start_cmd' -s c -l continue -d '[claude] Continue the most recent conversation'
complete -c ax -n '__ax_agent_start_cmd' -l dangerously-skip-permissions -d '[claude] Bypass all permission checks'
complete -c ax -n '__ax_agent_start_cmd' -s d -l debug -d '[claude] Enable debug mode with optional filter' -r
complete -c ax -n '__ax_agent_start_cmd' -l debug-file -d '[claude] Write debug logs to a file' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l disable-slash-commands -d '[claude] Disable all skills'
complete -c ax -n '__ax_agent_start_cmd' -l disallowedTools -l disallowed-tools -d '[claude] Tool names to deny' -r
complete -c ax -n '__ax_agent_start_cmd' -l effort -d '[claude] Effort level' -r -a 'low medium high xhigh max'
complete -c ax -n '__ax_agent_start_cmd' -l exclude-dynamic-system-prompt-sections -d '[claude] Move dynamic system prompt sections into first user message'
complete -c ax -n '__ax_agent_start_cmd' -l fallback-model -d '[claude] Fallback model for print mode' -r
complete -c ax -n '__ax_agent_start_cmd' -l file -d '[claude] File resource spec to download at startup' -r
complete -c ax -n '__ax_agent_start_cmd' -l fork-session -d '[claude] Fork when resuming'
complete -c ax -n '__ax_agent_start_cmd' -l from-pr -d '[claude] Resume a session linked to a PR' -r
complete -c ax -n '__ax_agent_start_cmd' -l ide -d '[claude] Automatically connect to IDE'
complete -c ax -n '__ax_agent_start_cmd' -l include-hook-events -d '[claude] Include hook events in stream-json output'
complete -c ax -n '__ax_agent_start_cmd' -l include-partial-messages -d '[claude] Include partial stream-json message chunks'
complete -c ax -n '__ax_agent_start_cmd' -l input-format -d '[claude] Input format for print mode' -r -a 'text stream-json'
complete -c ax -n '__ax_agent_start_cmd' -l json-schema -d '[claude] JSON Schema for structured output' -r
complete -c ax -n '__ax_agent_start_cmd' -l max-budget-usd -d '[claude] Maximum API spend for print mode' -r
complete -c ax -n '__ax_agent_start_cmd' -l mcp-config -d '[claude] MCP config JSON file or string' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l mcp-debug -d '[claude] Enable MCP debug mode'
complete -c ax -n '__ax_agent_start_cmd' -l model -d '[claude] Model for the session' -r
complete -c ax -n '__ax_agent_start_cmd' -l name -d '[claude] Display name for this session' -r
complete -c ax -n '__ax_agent_start_cmd' -l no-chrome -d '[claude] Disable Claude in Chrome integration'
complete -c ax -n '__ax_agent_start_cmd' -l no-session-persistence -d '[claude] Disable session persistence'
complete -c ax -n '__ax_agent_start_cmd' -l output-format -d '[claude] Output format for print mode' -r -a 'text json stream-json'
complete -c ax -n '__ax_agent_start_cmd' -l permission-mode -d '[claude] Permission mode' -r -a 'acceptEdits auto bypassPermissions default dontAsk plan'
complete -c ax -n '__ax_agent_start_cmd' -l plugin-dir -d '[claude] Plugin directory or zip' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l plugin-url -d '[claude] Plugin zip URL' -r
complete -c ax -n '__ax_agent_start_cmd' -s p -l print -d '[claude] Print response and exit'
complete -c ax -n '__ax_agent_start_cmd' -l remote-control -d '[claude] Start with Remote Control enabled' -r
complete -c ax -n '__ax_agent_start_cmd' -l remote-control-session-name-prefix -d '[claude] Prefix for Remote Control session names' -r
complete -c ax -n '__ax_agent_start_cmd' -l replay-user-messages -d '[claude] Re-emit stream-json user messages'
complete -c ax -n '__ax_agent_start_cmd' -s r -l resume -d '[claude] Resume a conversation' -r
complete -c ax -n '__ax_agent_start_cmd' -l session-id -d '[claude] Use a specific session UUID' -r
complete -c ax -n '__ax_agent_start_cmd' -l setting-sources -d '[claude] Setting sources to load' -r -a 'user project local'
complete -c ax -n '__ax_agent_start_cmd' -l settings -d '[claude] Settings JSON file or string' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l strict-mcp-config -d '[claude] Only use MCP servers from --mcp-config'
complete -c ax -n '__ax_agent_start_cmd' -l system-prompt -d '[claude] System prompt for the session' -r
complete -c ax -n '__ax_agent_start_cmd' -l tmux -d '[claude] Create a tmux session for the worktree' -r -a 'classic'
complete -c ax -n '__ax_agent_start_cmd' -l tools -d '[claude] Available built-in tools' -r
complete -c ax -n '__ax_agent_start_cmd' -l verbose -d '[claude] Enable verbose mode'
complete -c ax -n '__ax_agent_start_cmd' -s w -l worktree -d '[claude] Create a new git worktree' -r

# ax agent new/resume: Codex CLI options
complete -c ax -n '__ax_agent_start_cmd' -s c -l config -d '[codex] Override config key=value' -r
complete -c ax -n '__ax_agent_start_cmd' -l enable -d '[codex] Enable a feature' -r
complete -c ax -n '__ax_agent_start_cmd' -l disable -d '[codex] Disable a feature' -r
complete -c ax -n '__ax_agent_start_cmd' -l remote -d '[codex] Connect TUI to remote app server' -r
complete -c ax -n '__ax_agent_start_cmd' -l remote-auth-token-env -d '[codex] Env var containing remote auth token' -r
complete -c ax -n '__ax_agent_start_cmd' -l strict-config -d '[codex] Error on unrecognized config fields'
complete -c ax -n '__ax_agent_start_cmd' -s i -l image -d '[codex] Attach image to initial prompt' -r -F
complete -c ax -n '__ax_agent_start_cmd' -s m -l model -d '[codex] Model the agent should use' -r
complete -c ax -n '__ax_agent_start_cmd' -l oss -d '[codex] Use open-source provider'
complete -c ax -n '__ax_agent_start_cmd' -l local-provider -d '[codex] Local OSS provider' -r -a 'lmstudio ollama'
complete -c ax -n '__ax_agent_start_cmd' -s p -l profile -d '[codex] Config profile' -r
complete -c ax -n '__ax_agent_start_cmd' -s s -l sandbox -d '[codex] Sandbox policy' -r -a 'read-only workspace-write danger-full-access'
complete -c ax -n '__ax_agent_start_cmd' -l dangerously-bypass-approvals-and-sandbox -d '[codex] Skip approvals and sandbox'
complete -c ax -n '__ax_agent_start_cmd' -l dangerously-bypass-hook-trust -d '[codex] Run hooks without persisted trust'
complete -c ax -n '__ax_agent_start_cmd' -s C -l cd -d '[codex] Working root directory' -r -F
complete -c ax -n '__ax_agent_start_cmd' -l add-dir -d '[codex] Additional writable directory' -r -F
complete -c ax -n '__ax_agent_start_cmd' -s a -l ask-for-approval -d '[codex] Approval policy' -r -a 'untrusted on-failure on-request never'
complete -c ax -n '__ax_agent_start_cmd' -l search -d '[codex] Enable live web search'
complete -c ax -n '__ax_agent_start_cmd' -l no-alt-screen -d '[codex] Disable alternate screen mode'

# ax agent new/resume: OpenCode TUI options
complete -c ax -n '__ax_agent_start_cmd' -l print-logs -d '[opencode] Print logs to stderr'
complete -c ax -n '__ax_agent_start_cmd' -l log-level -d '[opencode] Log level' -r -a 'DEBUG INFO WARN ERROR'
complete -c ax -n '__ax_agent_start_cmd' -l pure -d '[opencode] Run without external plugins'
complete -c ax -n '__ax_agent_start_cmd' -l port -d '[opencode] Port to listen on' -r
complete -c ax -n '__ax_agent_start_cmd' -l hostname -d '[opencode] Hostname to listen on' -r
complete -c ax -n '__ax_agent_start_cmd' -l mdns -d '[opencode] Enable mDNS service discovery'
complete -c ax -n '__ax_agent_start_cmd' -l mdns-domain -d '[opencode] Custom mDNS domain name' -r
complete -c ax -n '__ax_agent_start_cmd' -l cors -d '[opencode] Additional CORS domain' -r
complete -c ax -n '__ax_agent_start_cmd' -s m -l model -d '[opencode] Model in provider/model format' -r
complete -c ax -n '__ax_agent_start_cmd' -s c -l continue -d '[opencode] Continue the last session'
complete -c ax -n '__ax_agent_start_cmd' -s s -l session -d '[opencode] Session ID to continue' -r
complete -c ax -n '__ax_agent_start_cmd' -l fork -d '[opencode] Fork the session when continuing'
complete -c ax -n '__ax_agent_start_cmd' -l prompt -d '[opencode] Prompt to use' -r
complete -c ax -n '__ax_agent_start_cmd' -l agent -d '[opencode] Agent to use' -r

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
