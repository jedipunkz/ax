package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/jedipunkz/ax/internal/config"
)

var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	FrameStyle         lipgloss.Style
	TitleStyle         lipgloss.Style
	StatusRunningStyle lipgloss.Style
	StatusSuccessStyle lipgloss.Style
	StatusFailedStyle  lipgloss.Style
	StatusKilledStyle  lipgloss.Style
	StatusWaitingStyle lipgloss.Style
	SelectedItemStyle  lipgloss.Style
	NormalItemStyle    lipgloss.Style
	SectionHeaderStyle lipgloss.Style
	RunningHeaderStyle lipgloss.Style
	SuccessHeaderStyle lipgloss.Style
	KilledHeaderStyle  lipgloss.Style
	ColHeaderStyle     lipgloss.Style
	OverviewLabelStyle lipgloss.Style
	RepoStyle          lipgloss.Style
	EndedStyle         lipgloss.Style
	LastOutputStyle    lipgloss.Style
)

func init() {
	// Apply the default theme at startup so styles are never nil.
	// cmd/dash.go calls ApplyTheme again once the user config is loaded.
	cfg := &config.Config{Theme: config.DefaultTheme}
	ApplyTheme(cfg.Palette())
}

// ApplyTheme sets all package-level style variables to match the given palette.
func ApplyTheme(p config.ThemePalette) {
	FrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Frame))

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Title))

	StatusRunningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.StatusRunning))

	StatusSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.StatusSuccess))

	StatusFailedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.StatusFailed))

	StatusKilledStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.StatusKilled))

	StatusWaitingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.StatusWaiting))

	SelectedItemStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Selected))

	NormalItemStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Normal))

	SectionHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.SectionHeader))

	RunningHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.RunningHeader))

	SuccessHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.SuccessHeader))

	KilledHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.KilledHeader))

	ColHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.ColHeader))

	OverviewLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.OverviewLabel))

	RepoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Repo))

	EndedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Ended))

	LastOutputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.LastOutput))
}

func fr(s string) string { return FrameStyle.Render(s) }
