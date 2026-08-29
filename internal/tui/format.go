package tui

// Value formatting and string padding shared by every view. These are pure
// functions of their arguments (plus the Model's spinner/metrics caches for
// status and stats), so they carry no layout knowledge.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/agx/internal/store"
)

func formatStatus(agent store.AgentState, m Model) string {
	switch agent.Status {
	case store.StatusRunning:
		if agent.WaitingUser {
			return StatusWaitingStyle.Render("waiting")
		}
		return StatusRunningStyle.Render(m.spinner.View() + " running")
	case store.StatusSuccess:
		return StatusSuccessStyle.Render("success")
	case store.StatusFailed:
		return StatusFailedStyle.Render("failed")
	case store.StatusKilled:
		return StatusKilledStyle.Render("killed")
	default:
		return string(agent.Status)
	}
}

func repoName(name, workDir string) string {
	if name != "" {
		return name
	}
	return filepath.Base(workDir)
}

func formatElapsed(agent store.AgentState) string {
	var d time.Duration
	if agent.FinishedAt != nil {
		d = agent.FinishedAt.Sub(agent.StartedAt)
	} else {
		d = time.Since(agent.StartedAt)
	}

	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, mn, s)
}

func formatMetricsSummary(m Model, agent store.AgentState) string {
	if err := m.metrics.errs[agent.ID]; err != "" {
		return "metrics error: " + err
	}
	metrics, ok := m.metrics.byAgent[agent.ID]
	if !ok {
		return "loading..."
	}
	return fmt.Sprintf(
		"%s  commits %d  output %d lines",
		formatSeconds(metrics.DurationSec),
		metrics.CommitCount,
		metrics.OutputLines,
	)
}

func formatSeconds(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	mn := (sec / 60) % 60
	s := sec % 60
	return fmt.Sprintf("%d:%02d:%02d", h, mn, s)
}

func clampWidth(w int) int {
	if w < 60 {
		return 80
	}
	return w
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
