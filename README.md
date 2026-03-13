# cco — Claude Code Orchestrator

Launch and monitor multiple [Claude Code](https://claude.ai/code) agents from a single terminal.

## Installation

```bash
go install github.com/thirai/cco@latest
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

```bash
# Start an interactive agent
cco agent

# Pass a prompt directly
cco agent -- -p "Fix the authentication bug in auth/login.go"

# Use a specific model
cco agent -- --model claude-opus-4-6 -p "Refactor the database layer"

# Open the TUI dashboard
cco status
```

`--dangerously-skip-permissions` is always prepended so agents run without interactive permission prompts.

Run `cco agent` in as many terminals as you like, then open `cco status` in another pane to monitor them all.

## TUI

```
╭─ cco status ─────────────────────────────────── 2 running ─╮
│ RUNNING                                                      │
│  ▶ cco-29514-a3f1   ⏳ waiting you   0:02:31   Fix auth...  │
│    cco-29514-c8e2   ⠋ running        0:01:05   Writing te…  │
├──────────────────────────────────────────────────────────────┤
│ SUCCESS (recent)                                             │
│    cco-29514-b1d0   ✓ success        0:05:10                 │
╰──────────────────────────────────────────────────────────────╯
```

Press `space` to open the detail view for a selected agent (PID, working dir, activity log, etc.).

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
| `⏳ waiting you` | Idle at prompt, waiting for input |
| `✓ success` | Exited with code 0 |
| `✗ failed` | Exited with non-zero code |
| `✕ killed` | Terminated by signal |

Finished agents remain visible for 5 minutes after exit.

## Runtime files

```
~/.cco/
├── cco.sock          # Unix domain socket (daemon IPC)
├── state.json        # Agent state snapshot
└── agents/
    └── <id>/
        └── output.log
```

## License

MIT
