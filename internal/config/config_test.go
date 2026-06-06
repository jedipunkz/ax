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
