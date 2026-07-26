package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPalette(t *testing.T) {
	t.Run("known theme returns its palette", func(t *testing.T) {
		c := &Config{Theme: "catppuccin"}
		if got := c.Palette(); got != themes["catppuccin"] {
			t.Errorf("Palette() did not return catppuccin palette")
		}
	})

	t.Run("unknown theme falls back to default", func(t *testing.T) {
		c := &Config{Theme: "does-not-exist"}
		if got := c.Palette(); got != themes[DefaultTheme] {
			t.Errorf("Palette() did not fall back to default theme")
		}
	})
}

func TestLoadDefaultsWhenMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != DefaultTheme {
		t.Errorf("Theme = %q, want default %q", cfg.Theme, DefaultTheme)
	}
	if cfg.DurationDays != DefaultDurationDays {
		t.Errorf("DurationDays = %d, want %d", cfg.DurationDays, DefaultDurationDays)
	}
	if cfg.RemoveDurationDays != DefaultRemoveDurationDays {
		t.Errorf("RemoveDurationDays = %d, want %d", cfg.RemoveDurationDays, DefaultRemoveDurationDays)
	}
}

func TestLoadParsesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	axDir := filepath.Join(home, ".ax")
	if err := os.MkdirAll(axDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "" +
		"# comment line\n" +
		"theme: catppuccin\n" +
		"duration_days: 14d\n" +
		"remove_duration_days: 60\n" +
		"unknown_key: ignored\n"
	if err := os.WriteFile(filepath.Join(axDir, "ax.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != "catppuccin" {
		t.Errorf("Theme = %q, want catppuccin", cfg.Theme)
	}
	if cfg.DurationDays != 14 {
		t.Errorf("DurationDays = %d, want 14", cfg.DurationDays)
	}
	if cfg.RemoveDurationDays != 60 {
		t.Errorf("RemoveDurationDays = %d, want 60", cfg.RemoveDurationDays)
	}
}

func TestParseScalar(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain value", " catppuccin ", "catppuccin"},
		{"double quoted", ` "catppuccin"`, "catppuccin"},
		{"single quoted", ` 'catppuccin'`, "catppuccin"},
		{"trailing comment", " catppuccin # my favourite", "catppuccin"},
		{"hash inside quotes is kept", ` "cat#puccin"`, "cat#puccin"},
		{"unterminated quote falls through", ` "catppuccin`, `"catppuccin`},
		{"quoted number", ` "14"`, "14"},
		{"empty", "  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseScalar(tt.in); got != tt.want {
				t.Errorf("parseScalar(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A .yaml file invites YAML habits: quoted strings and trailing comments. Those
// used to be stored verbatim and then silently discarded by the theme lookup or
// the integer parse, leaving a config that looks right and does nothing.
func TestLoadAcceptsQuotedAndCommentedValues(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		wantTheme      string
		wantDurationD  int
		wantRemoveDays int
	}{
		{
			name:           "double quoted values",
			body:           "theme: \"catppuccin\"\nduration_days: \"14\"\nremove_duration_days: \"60\"\n",
			wantTheme:      "catppuccin",
			wantDurationD:  14,
			wantRemoveDays: 60,
		},
		{
			name:           "single quoted values",
			body:           "theme: 'solarized-dark'\nduration_days: '3'\n",
			wantTheme:      "solarized-dark",
			wantDurationD:  3,
			wantRemoveDays: DefaultRemoveDurationDays,
		},
		{
			name:           "trailing comments",
			body:           "theme: kanagawa-wave # nice one\nduration_days: 21 # three weeks\n",
			wantTheme:      "kanagawa-wave",
			wantDurationD:  21,
			wantRemoveDays: DefaultRemoveDurationDays,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			axDir := filepath.Join(home, ".ax")
			if err := os.MkdirAll(axDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(axDir, "ax.yaml"), []byte(tt.body), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Theme != tt.wantTheme {
				t.Errorf("Theme = %q, want %q", cfg.Theme, tt.wantTheme)
			}
			if _, ok := themes[cfg.Theme]; !ok {
				t.Errorf("Theme %q does not resolve to a palette, so it silently falls back", cfg.Theme)
			}
			if cfg.DurationDays != tt.wantDurationD {
				t.Errorf("DurationDays = %d, want %d", cfg.DurationDays, tt.wantDurationD)
			}
			if cfg.RemoveDurationDays != tt.wantRemoveDays {
				t.Errorf("RemoveDurationDays = %d, want %d", cfg.RemoveDurationDays, tt.wantRemoveDays)
			}
		})
	}
}

func TestLoadIgnoresInvalidValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	axDir := filepath.Join(home, ".ax")
	if err := os.MkdirAll(axDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-numeric and non-positive values must be ignored, keeping defaults.
	content := "duration_days: abc\nremove_duration_days: 0\n"
	if err := os.WriteFile(filepath.Join(axDir, "ax.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DurationDays != DefaultDurationDays {
		t.Errorf("DurationDays = %d, want default %d", cfg.DurationDays, DefaultDurationDays)
	}
	if cfg.RemoveDurationDays != DefaultRemoveDurationDays {
		t.Errorf("RemoveDurationDays = %d, want default %d", cfg.RemoveDurationDays, DefaultRemoveDurationDays)
	}
}
