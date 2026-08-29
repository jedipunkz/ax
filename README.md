# agx - agent cross

[![CI](https://github.com/jedipunkz/agx/actions/workflows/ci.yml/badge.svg)](https://github.com/jedipunkz/agx/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedipunkz/agx)](https://goreportcard.com/report/github.com/jedipunkz/agx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Go version](https://img.shields.io/badge/go-1.25-blue)

Run multiple AI coding agents in parallel, each isolated in its own git worktree, and monitor them all from a single terminal dashboard.

Supported agents: [Claude Code](https://claude.ai/code), [Codex CLI](https://github.com/openai/codex), [Gemini CLI](https://github.com/google-gemini/gemini-cli) , [OpenCode](https://opencode.ai/).

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap jedipunkz/agx && brew install agx
```

## Usage

### Start an agent

`cd` into your git repository before running `agx agent new`. agx uses the current directory to detect the git repo and automatically creates an isolated worktree for the agent.

```bash
cd /path/to/your/repo
agx agent new
```

By default agx uses Claude Code. Use `-a` to choose a different agent:

```bash
agx agent new -a claude      # Claude Code (default)
agx agent new -a codex       # OpenAI Codex CLI
agx agent new -a gemini      # Gemini CLI
agx agent new -a opencode    # OpenCode
```

You can optionally give the agent a name(branch name):

```bash
agx agent new -n feat/foo
agx agent new -a codex -n feat/foo
```

You can also pass agent-specific options after `--`:

```bash
agx agent new -n feat/foo -- --model sonnet --dangerously-skip-permissions
agx agent new -a codex -n feat/foo -- --sandbox workspace-write --ask-for-approval never
```

### Resume an agent

To resume a previous session by ID or name:

```bash
agx agent resume -a gemini -n feat/foo
```

### Change to an agent's worktree

To open a new shell in the agent's worktree directory:

```bash
agx agent cd -n <name|id>
```


### View an agent's output

Dump the full output log with ANSI escapes stripped:

```bash
agx agent logs -n <name|id>
```

Follow new output in real time (the daemon streams it from any terminal). Press Ctrl-C to stop following:

```bash
agx agent logs -f -n <name|id>
```

### Wait for an agent to finish or pause

Block until the agent reaches a terminal state or pauses at a prompt. Terminal states exit with the agent's own exit code (`130` for killed agents); a paused `waiting` agent exits with `0`. Useful for shell pipelines and follow-up commands:

```bash
agx agent wait -n <name|id> && ./deploy.sh
```

### Send input to a waiting agent

When an agent is paused at a prompt (`waiting` status), you can answer it from any terminal without returning to the runner's session:

```bash
agx agent input -n <name|id> "y\n"
```

Input is only accepted while the agent is actually waiting for user input; the daemon rejects requests sent during active processing to avoid interleaving with local keystrokes.

### List agents

To list all agents with their ID, name, repo, ended time, and worktree directory:

```bash
agx agent list   # or: agx agent ls
```

### Remove an agent

To remove a terminated agent's worktree, log file, and state entry:

```bash
agx agent remove -n <name|id>   # or: agx agent rm -n <id|name>
```

If the agent's worktree still has uncommitted changes or untracked files, the
removal is refused and nothing is deleted. Commit the work (its branch is kept
either way) or discard it explicitly:

```bash
agx agent rm -f -n <name|id>
```


### Open the dashboard

```bash
agx dash
```

### Key bindings

#### List view

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `enter` | Open agent log (detail view) |
| `d` | Open live diff view of the agent's worktree |
| `o` | Toggle showing finished agents |
| `/` | Search agents by ID or name |
| `y` | Copy `cd <worktree-path>` to clipboard |
| `K` | Kill selected agent (SIGTERM) |
| `q` / `ctrl+c` | Quit |

#### Detail view

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll log down |
| `k` / `↑` | Scroll log up |
| `d` | Open live diff view |
| `enter` / `esc` / `q` | Back to list |

#### Diff view

Press `d` on an agent to see everything it has changed in its worktree — recorded commits, uncommitted edits, and untracked files — as a colorized unified diff. While the agent is running the diff reloads automatically every 2 seconds, and files are ordered by modification time with the most recently updated file at the top, so you can watch changes land in real time. The scroll position is preserved across refreshes.

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `d` / `esc` / `q` | Back to list |

### Status indicators

| Symbol | Meaning |
|--------|---------|
| `⠋ running` | Agent is actively processing |
| `waiting` | Idle at prompt, waiting for input |
| `success` | Exited with code 0 |
| `failed` | Exited with non-zero code |
| `killed` | Terminated by signal |

Finished agents are visible for the configured duration after exit (default: 7 days). Press `o` to toggle their visibility.

## Runtime files

```
~/.agx/
├── agx.yaml              # Configuration (Optional)
├── agx.sock              # Unix domain socket (daemon IPC)
├── state.json            # Agent state snapshot
├── agents/
│   └── <id>/
│       └── output.log    # Agent output log for each session
└── worktrees/
    └── <repo>-<id>/      # Git worktree per agent (branch: agx/<id>)
```

When `agx agent` is run inside a git repository, a dedicated worktree is automatically created at `~/.agx/worktrees/<repo>-<id>/` on a new branch `agx/<id>` branched from `HEAD`. The agent runs inside this isolated worktree so each agent's changes stay separate from the main working tree.

## Configuration (Optional)

agx can be configured via `~/.agx/agx.yaml`.

### Color theme

Set the `theme` key to choose a color theme for the dashboard. You can choose theme from tokyonight(default), kanagawa-wave, solarized-dark, catppussin.

```yaml
theme: tokyonight
```

### Finished agent display duration

Set the `duration_days` key to control how many days of finished (success/killed/failed) agents are shown in the dashboard. The value must be a positive integer followed by `d` (e.g. `1d`, `7d`, `30d`). Default is `7d`.

```yaml
duration_days: 7d
```

### Automatic worktree cleanup

When `agx dash` is running, it automatically removes old git worktrees in the background. Set the `remove_duration_days` key to control how many days after an agent finishes before its worktree is deleted. The value must be a positive integer followed by `d` (e.g. `7d`, `30d`, `90d`). Default is `30d`.

```yaml
remove_duration_days: 30d
```

Cleanup runs once when the dashboard starts and then every 24 hours. Only worktrees under `~/.agx/worktrees/` belonging to finished agents (success/failed/killed) older than the configured threshold are removed.

## License

MIT
