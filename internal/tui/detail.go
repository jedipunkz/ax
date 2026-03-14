package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ansiRe matches ANSI/VT escape sequences produced by PTY output.
var ansiRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// cleanLog strips ANSI codes, normalizes line endings, and keeps only
// lines that contain readable text (at least 4 alphanumeric characters).
func cleanLog(data []byte) string {
	s := ansiRe.ReplaceAllString(string(data), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		alpha := 0
		for _, r := range trimmed {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				alpha++
			}
		}
		if alpha >= 4 {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return "(no readable output yet)"
	}
	return strings.Join(out, "\n")
}

func detailView(m Model) string {
	visible := visibleAgents(m.agents, m.showExpired)
	if len(visible) == 0 || m.cursor >= len(visible) {
		return "No agent selected."
	}

	agent := visible[m.cursor]
	width := clampWidth(m.width)
	innerWidth := width - 4

	statusStr := formatStatus(agent, m)
	elapsed := formatElapsed(agent)

	argsStr := strings.Join(agent.Args, " ")

	header := agent.ID
	if agent.Name != "" {
		header = agent.Name + " (" + agent.ID + ")"
	}
	var lines []string
	lines = append(lines, fr("╭─ "+header+" "+strings.Repeat("─", max(0, innerWidth-lipgloss.Width(header)-2))+"╮"))
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Status : %s", statusStr), innerWidth)+fr(" │"))
	if agent.Name != "" {
		lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Name   : %s", agent.Name), innerWidth)+fr(" │"))
	}
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("PID    : %d", agent.PID), innerWidth)+fr(" │"))
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Dir    : %s", agent.WorkDir), innerWidth)+fr(" │"))
	if agent.WorktreeBranch != "" {
		lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Branch : %s", agent.WorktreeBranch), innerWidth)+fr(" │"))
	}
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Args   : %s", truncate(argsStr, innerWidth-9)), innerWidth)+fr(" │"))
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Started: %s", agent.StartedAt.Format("2006-01-02 15:04:05")), innerWidth)+fr(" │"))
	lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Elapsed: %s", elapsed), innerWidth)+fr(" │"))
	if agent.LastOutput != "" {
		lines = append(lines, fr("│ ")+padRight(fmt.Sprintf("Last   : %s", truncate(agent.LastOutput, innerWidth-9)), innerWidth)+fr(" │"))
	}
	lines = append(lines, fr("│ ")+padRight("── Activity Log ──", innerWidth)+fr(" │"))

	// Viewport content
	vpLines := strings.Split(m.viewport.View(), "\n")
	for _, l := range vpLines {
		lines = append(lines, fr("│ ")+padRight(l, innerWidth)+fr(" │"))
	}

	lines = append(lines, fr("╰"+strings.Repeat("─", innerWidth+2)+"╯"))

	help := NormalItemStyle.Render("[esc] back  [K] kill  [↑↓/jk/pgup/pgdn] scroll")
	lines = append(lines, help)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// loadLog returns a tea.Cmd that reads the log file and sends a logLoadedMsg.
func loadLog(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return logLoadedMsg{content: fmt.Sprintf("(could not read log: %v)", err)}
		}
		return logLoadedMsg{content: cleanLog(data)}
	}
}
