package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the ax configuration loaded from ~/.ax/ax.yaml.
type Config struct {
	Theme              string
	DurationDays       int
	RemoveDurationDays int
}

// DefaultDurationDays is the default number of days to show finished agents.
const DefaultDurationDays = 7

// DefaultRemoveDurationDays is the default number of days before old worktrees are removed.
const DefaultRemoveDurationDays = 30

// ThemePalette defines the color codes (hex) used by the TUI.
type ThemePalette struct {
	Frame         string
	Title         string
	StatusRunning string
	StatusSuccess string
	StatusFailed  string
	StatusKilled  string
	StatusWaiting string
	Selected      string
	Normal        string
	SectionHeader string
	RunningHeader string
	SuccessHeader string
	KilledHeader  string
	ColHeader     string
	OverviewLabel string
	Repo          string
	Ended         string
	LastOutput    string
}

// themes contains all built-in color palettes.
var themes = map[string]ThemePalette{
	// Tokyo Night — https://github.com/folke/tokyonight.nvim
	"tokyonight": {
		Frame:         "#545c7e",
		Title:         "#bb9af7",
		StatusRunning: "#e0af68",
		StatusSuccess: "#9ece6a",
		StatusFailed:  "#f7768e",
		StatusKilled:  "#565f89",
		StatusWaiting: "#7dcfff",
		Selected:      "#7aa2f7",
		Normal:        "#c0caf5",
		SectionHeader: "#565f89",
		RunningHeader: "#e0af68",
		SuccessHeader: "#9ece6a",
		KilledHeader:  "#565f89",
		ColHeader:     "#545c7e",
		OverviewLabel: "#7aa2f7",
		Repo:          "#7dcfff",
		Ended:         "#e0af68",
		LastOutput:    "#a9b1d6",
	},

	// Catppuccin Mocha — https://github.com/catppuccin/catppuccin
	"catppuccin": {
		Frame:         "#585b70",
		Title:         "#cba6f7",
		StatusRunning: "#fab387",
		StatusSuccess: "#a6e3a1",
		StatusFailed:  "#f38ba8",
		StatusKilled:  "#6c7086",
		StatusWaiting: "#89dceb",
		Selected:      "#89b4fa",
		Normal:        "#cdd6f4",
		SectionHeader: "#6c7086",
		RunningHeader: "#fab387",
		SuccessHeader: "#a6e3a1",
		KilledHeader:  "#6c7086",
		ColHeader:     "#585b70",
		OverviewLabel: "#89b4fa",
		Repo:          "#89dceb",
		Ended:         "#fab387",
		LastOutput:    "#bac2de",
	},

	// Solarized Dark — https://ethanschoonover.com/solarized/
	"solarized-dark": {
		Frame:         "#586e75",
		Title:         "#6c71c4",
		StatusRunning: "#b58900",
		StatusSuccess: "#859900",
		StatusFailed:  "#dc322f",
		StatusKilled:  "#586e75",
		StatusWaiting: "#2aa198",
		Selected:      "#268bd2",
		Normal:        "#839496",
		SectionHeader: "#586e75",
		RunningHeader: "#b58900",
		SuccessHeader: "#859900",
		KilledHeader:  "#586e75",
		ColHeader:     "#586e75",
		OverviewLabel: "#268bd2",
		Repo:          "#2aa198",
		Ended:         "#b58900",
		LastOutput:    "#657b83",
	},

	// Kanagawa Wave — https://github.com/rebelot/kanagawa.nvim
	"kanagawa-wave": {
		Frame:         "#54546d",
		Title:         "#957fb8",
		StatusRunning: "#dca561",
		StatusSuccess: "#98bb6c",
		StatusFailed:  "#e46876",
		StatusKilled:  "#54546d",
		StatusWaiting: "#7fb4ca",
		Selected:      "#7e9cd8",
		Normal:        "#dcd7ba",
		SectionHeader: "#54546d",
		RunningHeader: "#dca561",
		SuccessHeader: "#98bb6c",
		KilledHeader:  "#54546d",
		ColHeader:     "#54546d",
		OverviewLabel: "#7e9cd8",
		Repo:          "#7fb4ca",
		Ended:         "#dca561",
		LastOutput:    "#c8c093",
	},
}

// DefaultTheme is the theme used when none is specified.
const DefaultTheme = "tokyonight"

// Load reads ~/.ax/ax.yaml and returns a Config.
// Missing file or unknown keys are silently ignored; defaults apply.
func Load() (*Config, error) {
	cfg := &Config{Theme: DefaultTheme, DurationDays: DefaultDurationDays, RemoveDurationDays: DefaultRemoveDurationDays}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	path := filepath.Join(home, ".ax", "ax.yaml")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "theme":
			cfg.Theme = val
		case "duration_days":
			s := strings.TrimSuffix(val, "d")
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				cfg.DurationDays = n
			}
		case "remove_duration_days":
			s := strings.TrimSuffix(val, "d")
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				cfg.RemoveDurationDays = n
			}
		}
	}

	return cfg, scanner.Err()
}

// Palette returns the ThemePalette for the configured theme.
// Falls back to the default theme if the configured one is unknown.
func (c *Config) Palette() ThemePalette {
	if p, ok := themes[c.Theme]; ok {
		return p
	}
	return themes[DefaultTheme]
}

