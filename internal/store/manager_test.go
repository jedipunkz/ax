package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsResumeRestart(t *testing.T) {
	finishedAt := time.Now()

	if !isResumeRestart(
		AgentState{FinishedAt: &finishedAt},
		AgentState{StartedAt: finishedAt.Add(time.Second)},
	) {
		t.Fatal("newer start after finished should be treated as resume restart")
	}

	if isResumeRestart(
		AgentState{FinishedAt: &finishedAt},
		AgentState{StartedAt: finishedAt.Add(-time.Second)},
	) {
		t.Fatal("older start before finished should not be treated as resume restart")
	}

	if isResumeRestart(
		AgentState{},
		AgentState{StartedAt: finishedAt.Add(time.Second)},
	) {
		t.Fatal("missing FinishedAt should not be treated as resume restart")
	}
}

func TestDurationSec(t *testing.T) {
	startedAt := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2*time.Minute + 3*time.Second)

	if got := durationSec(AgentState{StartedAt: startedAt}, startedAt.Add(5*time.Second)); got != 5 {
		t.Fatalf("running duration = %d, want 5", got)
	}
	if got := durationSec(AgentState{StartedAt: startedAt, FinishedAt: &finishedAt}, time.Now()); got != 123 {
		t.Fatalf("finished duration = %d, want 123", got)
	}
	if got := durationSec(AgentState{StartedAt: startedAt}, startedAt.Add(-time.Second)); got != 0 {
		t.Fatalf("negative duration = %d, want 0", got)
	}
}

func TestCountLogLines(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		data string
		want int64
	}{
		{name: "empty", data: "", want: 0},
		{name: "one partial line", data: "hello", want: 1},
		{name: "one newline line", data: "hello\n", want: 1},
		{name: "two lines with partial", data: "hello\nworld", want: 2},
		{name: "two newline lines", data: "hello\nworld\n", want: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".log")
			if err := os.WriteFile(path, []byte(tc.data), 0644); err != nil {
				t.Fatal(err)
			}
			got, err := countLogLines(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("line count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCountLogLinesMissingPath(t *testing.T) {
	got, err := countLogLines(filepath.Join(t.TempDir(), "missing.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("line count = %d, want 0", got)
	}
}

func TestHandleMetrics(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "output.log")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree"), 0644); err != nil {
		t.Fatal(err)
	}

	startedAt := time.Now().Add(-10 * time.Second)
	mgr := &manager{
		agents: map[string]AgentState{
			"agx-test": {
				ID:        "agx-test",
				StartedAt: startedAt,
				LogFile:   logPath,
				Commits:   []string{"a", "b"},
			},
		},
	}
	sub := newSubscriber(nil, nil)

	mgr.handleMetrics(sub, Message{Type: "metrics", AgentID: "agx-test"})

	got := <-sub.sendCh
	if got.Type != "metrics_result" {
		t.Fatalf("response type = %q, want metrics_result", got.Type)
	}
	if got.AgentID != "agx-test" {
		t.Fatalf("agent id = %q, want agx-test", got.AgentID)
	}
	if got.CommitCount != 2 {
		t.Fatalf("commit count = %d, want 2", got.CommitCount)
	}
	if got.OutputLines != 3 {
		t.Fatalf("output lines = %d, want 3", got.OutputLines)
	}
	if got.DurationSec <= 0 {
		t.Fatalf("duration = %d, want positive", got.DurationSec)
	}
}

func TestHandleMetricsNoSuchAgent(t *testing.T) {
	mgr := &manager{agents: map[string]AgentState{}}
	sub := newSubscriber(nil, nil)

	mgr.handleMetrics(sub, Message{Type: "metrics", AgentID: "missing"})

	got := <-sub.sendCh
	if got.Type != "metrics_err" {
		t.Fatalf("response type = %q, want metrics_err", got.Type)
	}
	if got.Error == "" {
		t.Fatal("expected error message")
	}
}
