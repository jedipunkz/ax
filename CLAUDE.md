# CLAUDE.md

This file defines guidelines for Claude Code to follow when working in this repository.

## Language

- All communication, code comments, commit messages, and documentation must be written in **English**.

## Pull Requests

All PRs must be written in English and include the following sections in the body:

```
## Context
<Background and motivation for this change>

## Summary
<High-level overview of what this PR does>

## What I Did
<Bulleted list of specific changes made>
```

## Workflow After Task Completion

After completing any implementation task, automatically perform the following steps without waiting for user instruction:

1. **Commit** — Stage the relevant changed files and create a descriptive commit message in English.
2. **Push** — Push the branch to the remote repository.
3. **Create a Pull Request** — Open a PR against the main branch using `gh pr create`, following the PR format defined in the Pull Requests section above.

## Data Compatibility

The `~/.ax/` directory is the persistent data store for ax. All changes to its layout or schema must maintain backward compatibility with existing data.

### Directory Layout

```
~/.ax/
├── state.json          # JSON array of AgentState; the source of truth for all agents
├── ax.sock             # Unix domain socket for daemon IPC
├── daemon.pid          # Plain-text daemon PID
├── agents/
│   └── <agent-id>/
│       └── output.log  # Raw PTY output (may contain ANSI escape codes)
└── worktrees/
    └── <repo>-<agent-id>/  # Git worktree for the agent's isolated branch
```

### `state.json` Schema (AgentState)

Each element in the JSON array has the following fields:

| Field | Type | JSON key | Notes |
|---|---|---|---|
| ID | string | `id` | Format: `ax-<unix-ts>-<4hex>` |
| Name | string | `name` | Optional; omitted when empty |
| PID | int | `pid` | OS process ID |
| Args | []string | `args` | CLI args passed to claude |
| WorkDir | string | `work_dir` | Absolute path |
| Status | string | `status` | `"running"` \| `"success"` \| `"failed"` \| `"killed"` |
| StartedAt | time.Time | `started_at` | RFC3339 timestamp |
| FinishedAt | *time.Time | `finished_at` | Optional; omitted while running |
| ExitCode | *int | `exit_code` | Optional; omitted while running |
| LastOutput | string | `last_output` | Last meaningful output line |
| LogFile | string | `log_file` | Absolute path to `output.log` |
| WaitingUser | bool | `waiting_user` | Optional; omitted when false |
| WorktreeBranch | string | `worktree_branch` | Optional; omitted when no worktree |

### Compatibility Goal

The bar for compatibility is simple: **if a user restores their `~/.ax/` data, `ax dash` should display their agents correctly.** As long as that holds, the implementation is compatible.

### Rules

- **Adding new fields to `AgentState` is always fine.** Use `omitempty` so that existing `state.json` files missing the field remain valid.
- **Removing or renaming a field that `ax dash` depends on for display requires a migration** (see below).
- **Writes are atomic.** The daemon writes `state.json` via a `.tmp` + `os.Rename` pattern. Always preserve this to avoid corrupt reads.

### Migration

When a breaking schema change is necessary (e.g. renaming a field, changing a Status string value, or altering the agent ID format), implement a migration rather than leaving existing data broken. A migration reads the old format on daemon startup, transforms the data to the new format, and writes it back. Document the migration clearly in the commit message.

## Security Policy

- Write secure code at all times. Security is a first-class concern, not an afterthought.
- Prevent common vulnerabilities: SQL injection, XSS, command injection, path traversal, insecure deserialization, and other OWASP Top 10 issues.
- Never hardcode secrets, credentials, or API keys. Use environment variables or a secrets manager.
- Validate and sanitize all input at system boundaries (user input, external APIs, file reads).
- Apply the principle of least privilege: request only the permissions and access necessary.
- If a security issue is introduced, fix it immediately before proceeding.
