package agxfs

import (
	"path/filepath"
	"testing"
)

func TestPathsLayout(t *testing.T) {
	p := NewForHome("/home/u")

	want := map[string]string{
		"Dir":          "/home/u/.agx",
		"Socket":       "/home/u/.agx/agx.sock",
		"StateFile":    "/home/u/.agx/state.json",
		"PIDFile":      "/home/u/.agx/daemon.pid",
		"LockFile":     "/home/u/.agx/daemon.lock",
		"ConfigFile":   "/home/u/.agx/agx.yaml",
		"AgentsDir":    "/home/u/.agx/agents",
		"WorktreesDir": "/home/u/.agx/worktrees",
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
	if got := p.AgentDir("agx-123"); got != filepath.Join("/h/.agx/agents/agx-123") {
		t.Errorf("AgentDir = %q", got)
	}
	if got := p.AgentLog("agx-123"); got != filepath.Join("/h/.agx/agents/agx-123/output.log") {
		t.Errorf("AgentLog = %q", got)
	}
	if got := p.WorktreePath("repo", "agx-123"); got != filepath.Join("/h/.agx/worktrees/repo-agx-123") {
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
