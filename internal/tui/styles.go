package tui

import "github.com/charmbracelet/lipgloss"

// Tokyo Night color palette
// bg: #1a1b26, bg_highlight: #292e42
// fg: #c0caf5, comment: #565f89
// blue: #7aa2f7, cyan: #7dcfff
// purple: #bb9af7, magenta: #9d7cd8
// green: #9ece6a, yellow: #e0af68
// red: #f7768e, orange: #ff9e64
// dark3: #545c7e

var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	FrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#545c7e"))

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#bb9af7"))

	StatusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e0af68"))

	StatusSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a"))

	StatusFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f7768e"))

	StatusKilledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89"))

	StatusWaitingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7dcfff"))

	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7aa2f7"))

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5"))

	SectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#565f89"))

	RunningHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#e0af68"))

	SuccessHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#9ece6a"))

	KilledHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#565f89"))

	ColHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#545c7e"))

	ElapsedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7dcfff"))

	EndedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0af68"))

	LastOutputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff9e64"))
)

func fr(s string) string { return FrameStyle.Render(s) }
