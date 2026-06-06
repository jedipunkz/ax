package store

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

// handleMetrics computes a point-in-time metrics summary for one agent. It
// reads only a copy of AgentState while holding the manager lock so potentially
// slow log file scans do not block state updates or subscribers.
func (m *manager) handleMetrics(sender *subscriber, msg Message) {
	if msg.AgentID == "" {
		sender.trySend(Message{Type: "metrics_err", Error: "missing agent_id"})
		return
	}

	m.mu.Lock()
	agent, ok := m.agents[msg.AgentID]
	m.mu.Unlock()

	if !ok {
		sender.trySend(Message{Type: "metrics_err", AgentID: msg.AgentID, Error: "no such agent"})
		return
	}

	outputLines, err := countLogLines(agent.LogFile)
	if err != nil {
		sender.trySend(Message{Type: "metrics_err", AgentID: msg.AgentID, Error: err.Error()})
		return
	}

	sender.trySend(Message{
		Type:        "metrics_result",
		AgentID:     agent.ID,
		DurationSec: durationSec(agent, time.Now()),
		CommitCount: len(agent.Commits),
		OutputLines: outputLines,
	})
}

func durationSec(agent AgentState, now time.Time) int64 {
	end := now
	if agent.FinishedAt != nil {
		end = *agent.FinishedAt
	}
	if end.Before(agent.StartedAt) {
		return 0
	}
	return int64(end.Sub(agent.StartedAt).Seconds())
}

func countLogLines(path string) (int64, error) {
	if path == "" {
		return 0, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("could not open log: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var lines int64
	var last byte
	var sawBytes bool
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			lines += int64(bytes.Count(chunk, []byte{'\n'}))
			last = chunk[n-1]
			sawBytes = true
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, fmt.Errorf("could not read log: %w", readErr)
		}
	}
	if sawBytes && last != '\n' {
		lines++
	}
	return lines, nil
}
