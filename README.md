# cco - claude code o11y

Launch and monitor multiple [Claude Code](https://claude.ai/code) agents from a single terminal.

## Installation

```bash
go install github.com/jedipunkz/cco@latest
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

Open TUI Dashboard

```bash
cco status
```

Start an interactive agent
```bash
cco agent
```

### Key bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `space` | Open detail view |
| `esc` / `q` | Back to list or quit |
| `K` | Kill selected agent (SIGTERM) |

### Status indicators

| Symbol | Meaning |
|--------|---------|
| `⠋ running` | Claude is actively processing |
| `waiting you` | Idle at prompt, waiting for input |
| `success` | Exited with code 0 |
| `failed` | Exited with non-zero code |
| `killed` | Terminated by signal |

Finished agents remain visible for 5 minutes after exit.

## Runtime files

```
~/.cco/
├── cco.sock              # Unix domain socket (daemon IPC)
├── state.json            # Agent state snapshot
├── agents/
│   └── <id>/
│       └── output.log    # Claude output log for each agent
└── worktrees/
    └── <repo>-<id>/      # Git worktree per agent (branch: cco/<id>)
```

When `cco agent` is run inside a git repository, a dedicated worktree is automatically created at `~/.cco/worktrees/<repo>-<id>/` on a new branch `cco/<id>` branched from `HEAD`. Claude Code runs inside this isolated worktree so each agent's changes stay separate from the main working tree.

## License

MIT
