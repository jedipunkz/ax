# cco — Claude Code Orchestrator

Launch and monitor multiple [Claude Code](https://claude.ai/code) agents from a single terminal.

## Overview

`cco` lets you run several Claude Code sessions in parallel and watch all of them from a real-time TUI dashboard. Each agent runs in its own PTY, forwards your keystrokes directly to Claude, and reports its lifecycle state (running, waiting, done) to a shared state store.

```
cco agent [-- <claude-args>...]   # Start a Claude Code agent
cco status                        # Open the TUI dashboard
```

## Installation

```bash
go install github.com/thirai/cco@latest
```

Or build from source:

```bash
git clone https://github.com/thirai/cco
cd cco
go build -o cco .
```

**Requirements**: `claude` CLI must be on your `$PATH`.

## Usage

### Start an agent

```bash
# Open an interactive Claude Code session
cco agent

# Pass a prompt directly
cco agent -- -p "Fix the authentication bug in auth/login.go"

# Use a specific model
cco agent -- --model claude-opus-4-6 -p "Refactor the database layer"
```

`--dangerously-skip-permissions` is always prepended so agents run without interactive permission prompts.

### Watch the dashboard

```bash
cco status
```

Open as many `cco agent` sessions as you like in separate terminals, then run `cco status` in another pane to monitor them all.

## TUI

### List view

```
╭─ cco status ─────────────────────────────────── 2 running ─╮
│ RUNNING                                                      │
│  ▶ cco-29514-a3f1   ⏳ waiting you   0:02:31   Fix auth...  │
│    cco-29514-c8e2   ⠋ running        0:01:05   Writing te…  │
├──────────────────────────────────────────────────────────────┤
│ SUCCESS (recent)                                             │
│    cco-29514-b1d0   ✓ success        0:05:10                 │
├──────────────────────────────────────────────────────────────┤
│ KILLED (recent)                                              │
│    (none)                                                    │
├──────────────────────────────────────────────────────────────┤
│ [↑↓/jk] select  [space] detail  [K] kill  [q] quit          │
╰──────────────────────────────────────────────────────────────╯
```

### Detail view (press `space`)

```
╭─ cco-29514-a3f1 ─────────────────────────────────────────────╮
│ Status : ⏳ waiting you                                       │
│ PID    : 83201                                                │
│ Dir    : /Users/you/projects/myapp                            │
│ Args   : --dangerously-skip-permissions -p "Fix auth..."      │
│ Started: 2025-06-01 10:22:31                                  │
│ Elapsed: 0:02:31                                              │
│ Last   : Writing tests for the auth module                    │
│ ── Activity Log ──────────────────────────────────────────── │
│  (scrollable filtered log)                                    │
╰───────────────────────────────────────────────────────────────╯
[esc] back  [K] kill  [↑↓/jk/pgup/pgdn] scroll
```

### Key bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `space` | Open detail view |
| `esc` / `q` | Back to list (in detail) or quit |
| `K` | Send SIGTERM to selected running agent |

### Status indicators

| Symbol | Meaning |
|--------|---------|
| `⠋ running` | Claude is actively processing |
| `⏳ waiting you` | Claude is idle at the prompt, waiting for input |
| `✓ success` | Exited with code 0 |
| `✗ failed` | Exited with non-zero code |
| `✕ killed` | Terminated by signal |

Finished agents (success / killed) remain visible for 5 minutes after they exit.

## How it works

```
cco agent                    cco status
    │                             │
    ▼                             ▼
Agent Process              Status TUI (bubbletea)
 - fork claude               - list view
 - PTY monitor               - detail view
 - stream LastOutput
    │                             │
    └──────────┬──────────────────┘
               ▼
        State Store (IPC)
   ~/.cco/cco.sock  +  ~/.cco/state.json
```

The **daemon** (`cco daemon`, started automatically) acts as a central hub. Agents push state updates over a Unix socket; the TUI subscribes and receives broadcasts in real time. The state is also persisted to `~/.cco/state.json` so it survives restarts.

**WaitingUser detection**: Claude Code only writes to stdout when it has something to render. While processing, it produces a steady byte stream. When it shows the input prompt, stdout goes idle. `cco agent` uses a 2-second idle threshold to flip `WaitingUser` between `true` and `false`.

**Activity Log**: Each agent's full PTY output is written to `~/.cco/agents/<id>/output.log`. The detail view filters this down to lines with readable text content, stripping TUI chrome and ANSI control sequences.

## Runtime files

```
~/.cco/
├── cco.sock                   # Unix Domain Socket
├── state.json                 # Agent state snapshot
└── agents/
    └── cco-29514-a3f1/
        └── output.log         # Full PTY output log
```

## License

MIT
