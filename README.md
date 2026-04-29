# ax - agent cross

[![CI](https://github.com/jedipunkz/ax/actions/workflows/ci.yml/badge.svg)](https://github.com/jedipunkz/ax/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedipunkz/ax)](https://goreportcard.com/report/github.com/jedipunkz/ax)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Go version](https://img.shields.io/badge/go-1.25-blue)

Run multiple [Claude Code](https://claude.ai/code) agents in parallel, each isolated in its own git worktree, and monitor them all from a single terminal dashboard.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap jedipunkz/ax
brew install ax
```

To upgrade to the latest version:

```bash
brew upgrade ax
```

### Go

```bash
go install github.com/jedipunkz/ax@latest
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

### Start an agent

**Important**: `cd` into your git repository before running `ax agent new`. ax uses the current directory to detect the git repo and automatically creates an isolated worktree for the agent.

```bash
cd /path/to/your/repo
ax agent new
```

You can optionally give the agent a name:

```bash
ax agent new -n my-feature
```

You can also pass Claude Code options directly:

```bash
ax agent new -n my-feature --model sonnet --dangerously-skip-permissions
```

### Resume an agent

To resume a previous session by ID or name:

```bash
ax agent resume -n <id|name>
```

You can also pass Claude Code options directly:

```bash
ax agent resume -n my-feature --model opus --enable-auto-mode
```

### Change to an agent's worktree

To open a new shell in the agent's worktree directory:

```bash
ax agent cd -n <id|name>
```

This spawns a subshell (`$SHELL`) with the working directory set to the agent's worktree. Type `exit` to return to your original shell.

### List agents

To list all agents with their ID, name, repo, ended time, and worktree directory:

```bash
ax agent list
```

### Open the dashboard

```bash
ax dash
```

### Key bindings

#### List view

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `enter` | Open agent log (detail view) |
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
| `enter` / `esc` / `q` | Back to list |

### Status indicators

| Symbol | Meaning |
|--------|---------|
| `⠋ running` | Claude is actively processing |
| `waiting` | Idle at prompt, waiting for input |
| `success` | Exited with code 0 |
| `failed` | Exited with non-zero code |
| `killed` | Terminated by signal |

Finished agents are visible for the configured duration after exit (default: 7 days). Press `o` to toggle their visibility.

## Runtime files

```
~/.ax/
├── ax.yaml               # Configuration (Optional)
├── ax.sock               # Unix domain socket (daemon IPC)
├── state.json            # Agent state snapshot
├── agents/
│   └── <id>/
│       └── output.log    # Claude output log for each agent
└── worktrees/
    └── <repo>-<id>/      # Git worktree per agent (branch: ax/<id>)
```

When `ax agent` is run inside a git repository, a dedicated worktree is automatically created at `~/.ax/worktrees/<repo>-<id>/` on a new branch `ax/<id>` branched from `HEAD`. Claude Code runs inside this isolated worktree so each agent's changes stay separate from the main working tree.

## Configuration (Optional)

ax can be configured via `~/.ax/ax.yaml`.

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

When `ax dash` is running, it automatically removes old git worktrees in the background. Set the `remove_duration_days` key to control how many days after an agent finishes before its worktree is deleted. The value must be a positive integer followed by `d` (e.g. `7d`, `30d`, `90d`). Default is `30d`.

```yaml
remove_duration_days: 30d
```

Cleanup runs once when the dashboard starts and then every 24 hours. Only worktrees under `~/.ax/worktrees/` belonging to finished agents (success/failed/killed) older than the configured threshold are removed.

## License

MIT
