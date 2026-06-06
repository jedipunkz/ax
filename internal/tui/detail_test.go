package tui

import (
	"strings"
	"testing"
)

func TestCleanLog(t *testing.T) {
	t.Run("strips ANSI escapes and keeps readable lines", func(t *testing.T) {
		input := []byte("\x1b[31mHello world\x1b[0m\r\nsecond line\r\n")
		got := cleanLog(input)
		if strings.Contains(got, "\x1b") {
			t.Errorf("output still contains escape sequences: %q", got)
		}
		if !strings.Contains(got, "Hello world") || !strings.Contains(got, "second line") {
			t.Errorf("expected readable lines preserved, got %q", got)
		}
		if strings.Contains(got, "\r") {
			t.Errorf("carriage returns should be removed, got %q", got)
		}
	})

	t.Run("drops lines with too few alphanumerics", func(t *testing.T) {
		input := []byte("ok\n>>> \nreal content here\n--\n")
		got := cleanLog(input)
		if strings.Contains(got, ">>>") || strings.Contains(got, "--") {
			t.Errorf("noise lines should be dropped, got %q", got)
		}
		if !strings.Contains(got, "real content here") {
			t.Errorf("expected meaningful line kept, got %q", got)
		}
	})

	t.Run("placeholder when no readable output", func(t *testing.T) {
		got := cleanLog([]byte(">>>\n--\n   \n"))
		if got != "(no readable output yet)" {
			t.Errorf("got %q, want placeholder", got)
		}
	})
}
