package axfs

import (
	"path/filepath"
	"testing"
)

func TestPathsLayout(t *testing.T) {
	p := NewForHome("/home/u")

	want := map[string]string{
		"Dir":          "/home/u/.ax",
		"Socket":       "/home/u/.ax/ax.sock",
		"StateFile":    "/home/u/.ax/state.json",
		"PIDFile":      "/home/u/.ax/daemon.pid",
		"LockFile":     "/home/u/.ax/daemon.lock",
		"ConfigFile":   "/home/u/.ax/ax.yaml",
		"AgentsDir":    "/home/u/.ax/agents",
		"WorktreesDir": "/home/u/.ax/worktrees",
	}
	got := map[string]string{
		"Dir":          p.Dir,
		"Socket":       p.Socket(),
		"StateFile":    p.StateFile(),
		"PIDFile":      p.PIDFile(),
		"LockFile":     p.LockFile(),
		"ConfigFile":   p.ConfigFile(),
		"AgentsDir":    p.AgentsDir(),
		"WorktreesDir": p.WorktreesDir(),
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %q, want %q", key, got[key], w)
		}
	}
}

func TestAgentPaths(t *testing.T) {
	p := NewForHome("/h")
	if got := p.AgentDir("ax-123"); got != filepath.Join("/h/.ax/agents/ax-123") {
		t.Errorf("AgentDir = %q", got)
	}
	if got := p.AgentLog("ax-123"); got != filepath.Join("/h/.ax/agents/ax-123/output.log") {
		t.Errorf("AgentLog = %q", got)
	}
	if got := p.WorktreePath("repo", "ax-123"); got != filepath.Join("/h/.ax/worktrees/repo-ax-123") {
		t.Errorf("WorktreePath = %q", got)
	}
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	p := NewForHome(dir)
	if err := p.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := p.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir (idempotent): %v", err)
	}
}
