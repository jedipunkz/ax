package agent

// Every git invocation in this package goes through here, so the "build a
// command, point it at a directory, read its stdout" boilerplate exists once
// and the set of git commands agx runs can be read off a single file.

import (
	"os/exec"
	"strings"
)

// gitCmd builds a git command that runs inside dir. Callers that need
// CombinedOutput, a bare Run, or raw (untrimmed) stdout use this directly.
func gitCmd(dir string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	c.Dir = dir
	return c
}

// gitOut runs git inside dir and returns its stdout with surrounding
// whitespace trimmed. A failed command yields "" — callers here always treat
// "git could not answer" and "git answered nothing" the same way.
func gitOut(dir string, args ...string) string {
	out, err := gitCmd(dir, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
