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

## Security Policy

- Write secure code at all times. Security is a first-class concern, not an afterthought.
- Prevent common vulnerabilities: SQL injection, XSS, command injection, path traversal, insecure deserialization, and other OWASP Top 10 issues.
- Never hardcode secrets, credentials, or API keys. Use environment variables or a secrets manager.
- Validate and sanitize all input at system boundaries (user input, external APIs, file reads).
- Apply the principle of least privilege: request only the permissions and access necessary.
- If a security issue is introduced, fix it immediately before proceeding.
