package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jedipunkz/ax/internal/store"
	"github.com/jedipunkz/ax/internal/textutil"
)

const sampleDiff = `diff --git a/main.go b/main.go
index 1234567..89abcde 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
-func old() {}
+func new() {}
+func extra() {}
`

func TestDiffStat(t *testing.T) {
	added, deleted, files := diffStat(sampleDiff)
	if files != 1 {
		t.Errorf("files = %d, want 1", files)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

func TestDiffStatEmpty(t *testing.T) {
	added, deleted, files := diffStat("")
	if added != 0 || deleted != 0 || files != 0 {
		t.Errorf("got %d/%d/%d, want 0/0/0", added, deleted, files)
	}
}

func TestColorizeDiffPreservesContent(t *testing.T) {
	got := colorizeDiff(sampleDiff)

	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(sampleDiff, "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count changed: got %d, want %d", len(gotLines), len(wantLines))
	}
	for i, want := range wantLines {
		stripped := textutil.StripANSI([]byte(gotLines[i]))
		if stripped != want {
			t.Errorf("line %d: got %q, want %q", i, stripped, want)
		}
	}
}

func TestOpenDiffViewAndLoad(t *testing.T) {
	m := newModel(&store.Client{}, "", nil, 7)
	m.width, m.height = 100, 30
	m.agents = []store.AgentState{{
		ID:        "ax-1",
		Status:    store.StatusRunning,
		WorkDir:   "/tmp/x",
		StartedAt: time.Now(),
	}}

	m2, cmds := m.openDiffView(nil)
	if m2.view != viewDiff {
		t.Fatalf("view = %v, want viewDiff", m2.view)
	}
	if m2.diffAgentID != "ax-1" {
		t.Errorf("diffAgentID = %q, want ax-1", m2.diffAgentID)
	}
	if len(cmds) == 0 {
		t.Error("expected a loadDiff command to be scheduled")
	}

	m3, _ := m2.handleDiffLoaded(diffLoadedMsg{agentID: "ax-1", content: sampleDiff}, nil)
	if !m3.diffLoaded {
		t.Error("diffLoaded should be true after load")
	}
	if m3.diffContent != sampleDiff {
		t.Errorf("diffContent not stored, got %q", m3.diffContent)
	}

	// A result for a different agent must be ignored.
	m4, _ := m3.handleDiffLoaded(diffLoadedMsg{agentID: "other", content: "x"}, nil)
	if m4.diffContent != sampleDiff {
		t.Error("diff result for another agent should be ignored")
	}

	// Pressing d again returns to the list.
	m5, _ := m4.openDiffView(nil)
	if m5.view != viewList {
		t.Errorf("view = %v, want viewList after toggling", m5.view)
	}
}

func TestPollDiffOnlyWhileRunning(t *testing.T) {
	m := newModel(&store.Client{}, "", nil, 7)
	m.width, m.height = 100, 30
	now := time.Now()
	m.now = now
	m.agents = []store.AgentState{{
		ID:        "ax-1",
		Status:    store.StatusRunning,
		WorkDir:   "/tmp/x",
		StartedAt: now,
	}}
	m, _ = m.openDiffView(nil)

	// Within the poll interval: no reload.
	if cmd := m.pollDiff(now.Add(time.Second)); cmd != nil {
		t.Error("expected no poll before interval elapsed")
	}
	// After the interval: reload scheduled.
	if cmd := m.pollDiff(now.Add(3 * time.Second)); cmd == nil {
		t.Error("expected poll after interval elapsed")
	}
	// Finished agents are not re-polled.
	m.agents[0].Status = store.StatusSuccess
	if cmd := m.pollDiff(now.Add(10 * time.Second)); cmd != nil {
		t.Error("expected no poll for finished agent")
	}
}

func TestPollDiffFinalReloadAfterFinish(t *testing.T) {
	m := newModel(&store.Client{}, "", nil, 7)
	m.width, m.height = 100, 30
	now := time.Now()
	m.now = now
	m.agents = []store.AgentState{{
		ID:        "ax-1",
		Status:    store.StatusRunning,
		WorkDir:   "/tmp/x",
		StartedAt: now,
	}}
	m, _ = m.openDiffView(nil)

	// The agent finishes after the last poll: one final reload is scheduled.
	finished := now.Add(time.Second)
	m.agents[0].Status = store.StatusSuccess
	m.agents[0].FinishedAt = &finished
	if cmd := m.pollDiff(now.Add(2 * time.Second)); cmd == nil {
		t.Error("expected one final poll after agent finished")
	}
	// The final state was already loaded: no further polls.
	if cmd := m.pollDiff(now.Add(10 * time.Second)); cmd != nil {
		t.Error("expected no poll after the final reload")
	}
}

func TestDiffLoadErrorShownInViewport(t *testing.T) {
	m := newModel(&store.Client{}, "", nil, 7)
	m.width, m.height = 100, 30
	m.agents = []store.AgentState{{
		ID:        "ax-1",
		Status:    store.StatusRunning,
		WorkDir:   "/tmp/x",
		StartedAt: time.Now(),
	}}
	m, _ = m.openDiffView(nil)

	m, _ = m.handleDiffLoaded(diffLoadedMsg{agentID: "ax-1", err: "boom"}, nil)
	if m.diffErr != "boom" {
		t.Fatalf("diffErr = %q, want boom", m.diffErr)
	}
	if !strings.Contains(m.viewport.View(), "diff failed: boom") {
		t.Errorf("viewport should show the load error, got:\n%s", m.viewport.View())
	}

	// A successful reload clears the error placeholder.
	m, _ = m.handleDiffLoaded(diffLoadedMsg{agentID: "ax-1", content: sampleDiff}, nil)
	if m.diffErr != "" {
		t.Errorf("diffErr should be cleared, got %q", m.diffErr)
	}
}

func TestDiffViewportHeight(t *testing.T) {
	if h := diffViewportHeight(24); h != 24-diffChromeRows {
		t.Errorf("height for 24 = %d, want %d", h, 24-diffChromeRows)
	}
	// Tiny terminals fall back to the default height of 24.
	if h := diffViewportHeight(5); h != 24-diffChromeRows {
		t.Errorf("height for 5 = %d, want %d", h, 24-diffChromeRows)
	}
	if h := diffViewportHeight(10); h != 3 {
		t.Errorf("height floor = %d, want 3", h)
	}
}
