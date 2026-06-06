package textutil

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain text", "plain text"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"a\r\nb", "a\nb"},
		{"a\rb", "a\nb"},
	}
	for _, tc := range cases {
		if got := StripANSI([]byte(tc.in)); got != tc.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLastMeaningfulLine(t *testing.T) {
	cases := []struct {
		name  string
		chunk string
		want  string
	}{
		{"returns last readable line", "first line\nsecond line\n", "second line"},
		{"skips trailing noise lines", "real content\n>>>\n--\n", "real content"},
		{"strips ANSI", "\x1b[32mgreen output\x1b[0m\n", "green output"},
		{"normalizes carriage returns", "line one\r\nline two\r", "line two"},
		{"empty when nothing meaningful", ">>>\n--\n   \n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LastMeaningfulLine([]byte(tc.chunk)); got != tc.want {
				t.Errorf("LastMeaningfulLine() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCleanLogReadable(t *testing.T) {
	t.Run("strips ANSI escapes and keeps readable lines", func(t *testing.T) {
		input := []byte("\x1b[31mHello world\x1b[0m\r\nsecond line\r\n")
		got := CleanLogReadable(input, "<placeholder>")
		if strings.Contains(got, "\x1b") {
			t.Errorf("escapes leaked: %q", got)
		}
		if !strings.Contains(got, "Hello world") || !strings.Contains(got, "second line") {
			t.Errorf("readable lines missing: %q", got)
		}
	})
	t.Run("placeholder when nothing readable", func(t *testing.T) {
		if got := CleanLogReadable([]byte(">>>\n--\n"), "(empty)"); got != "(empty)" {
			t.Errorf("placeholder = %q", got)
		}
	})
}
