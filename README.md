# ax - Agent Cross

[![CI](https://github.com/jedipunkz/ax/actions/workflows/ci.yml/badge.svg)](https://github.com/jedipunkz/ax/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedipunkz/ax)](https://goreportcard.com/report/github.com/jedipunkz/ax)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Go version](https://img.shields.io/badge/go-1.25-blue)

Run multiple [Claude Code](https://claude.ai/code) agents in parallel, each isolated in its own git worktree, and monitor them all from a single terminal dashboard.

## Installation

```bash
go install github.com/jedipunkz/ax@latest
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

### Start an agent

**Important**: `cd` into your git repository before running `ax agent`. ax uses the current directory to detect the git repo and automatically creates an isolated worktree for the agent.

```bash
cd /path/to/your/repo
ax agent
```

You can optionally give the agent a name:

```bash
ax agent -n my-feature
```

To resume a previous session by name:

```bash
ax agent -n my-feature --resume
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

Finished agents are visible for 24 hours after exit. Press `o` to toggle their visibility.

## Runtime files

```
~/.ax/
├── ax.sock              # Unix domain socket (daemon IPC)
├── state.json            # Agent state snapshot
├── agents/
│   └── <id>/
│       └── output.log    # Claude output log for each agent
└── worktrees/
    └── <repo>-<id>/      # Git worktree per agent (branch: ax/<id>)
```

When `ax agent` is run inside a git repository, a dedicated worktree is automatically created at `~/.ax/worktrees/<repo>-<id>/` on a new branch `ax/<id>` branched from `HEAD`. Claude Code runs inside this isolated worktree so each agent's changes stay separate from the main working tree.

## Configuration

ax can be configured via `~/.ax/ax.yaml`.

### Color theme

Set the `theme` key to choose a color theme for the dashboard:

```yaml
theme: tokyonight
```

Available themes:

| Theme | Description |
|-------|-------------|
| `tokyonight` | Tokyo Night (default) |
| `kanagawa-wave` | Kanagawa Wave |
| `solarized-dark` | Solarized Dark |
| `catppuccin` | Catppuccin |

## License

MIT
