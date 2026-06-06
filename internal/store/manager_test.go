package store

import (
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
