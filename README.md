# ax - Agent Cross

[![CI](https://github.com/jedipunkz/ax/actions/workflows/ci.yml/badge.svg)](https://github.com/jedipunkz/ax/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedipunkz/ax)](https://goreportcard.com/report/github.com/jedipunkz/ax)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Go version](https://img.shields.io/badge/go-1.25-blue)

Launch and monitor multiple [Claude Code](https://claude.ai/code) agents from a single terminal.

## Installation

```bash
go install github.com/jedipunkz/ax@latest
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

Open TUI Dashboard

```bash
ax dash
```

Start an interactive agent
```bash
ax agent
```

### Key bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `enter` | Open detail view |
| `esc` / `q` | Back to list or quit |
| `y` | Yank — copy `cd <worktree-path>` to clipboard |
| `K` | Kill selected agent (SIGTERM) |
| `o` | Toggle showing full agent history |
| `/` | Search agents by ID or name |

### Status indicators

| Symbol | Meaning |
|--------|---------|
| `⠋ running` | Claude is actively processing |
| `waiting` | Idle at prompt, waiting for input |
| `success` | Exited with code 0 |
| `failed` | Exited with non-zero code |
| `killed` | Terminated by signal |

Finished agents are visible for 24 hours after exit. Press `h` to show the full history.

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

## License

MIT
