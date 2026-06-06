// Package textutil holds small text-manipulation helpers shared by
// agent and tui packages. Keeping these in one place ensures the same
// ANSI/escape rules apply everywhere — the runner, the daemon stream
// follower, and the dashboard's detail view.
package textutil

import (
	"regexp"
	"strings"
	"unicode"
)

// ansiRe matches ANSI/VT escape sequences produced by PTY output:
//   - CSI: ESC [ ... <final byte>
//   - Charset selection: ESC ( / ESC ) followed by A/B/0/1/2
//   - Two-byte escapes (e.g. ESC c reset): ESC <single uppercase letter>
//   - OSC: ESC ] ... BEL or ESC ] ... ST
var ansiRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// StripANSI removes ANSI escape sequences and normalises CR/LF to LF.
// It returns a UTF-8 string suitable for plain-text consumption.
func StripANSI(b []byte) string {
	s := ansiRe.ReplaceAllString(string(b), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// LastMeaningfulLine returns the most recent line from a raw PTY output
// chunk that contains at least minAlphanumerics letters or digits after
// ANSI stripping. Empty string when nothing qualifies.
//
// The runner uses this to populate the dashboard's "Last Output" column
// without picking up spinner frames or punctuation-only TUI artefacts.
func LastMeaningfulLine(chunk []byte) string {
	const minAlphanumerics = 4
	s := StripANSI(chunk)
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if countAlphaNum(line) >= minAlphanumerics {
			return line
		}
	}
	return ""
}

// CleanLogReadable strips ANSI codes and keeps only lines that contain
// at least minAlphanumerics readable characters. Used by the dashboard
// detail view to discard cursor-movement and decoration-only output.
// Returns placeholder when nothing readable was found.
func CleanLogReadable(data []byte, placeholder string) string {
	const minAlphanumerics = 4
	s := ansiRe.ReplaceAllString(string(data), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if countAlphaNum(strings.TrimSpace(line)) >= minAlphanumerics {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return placeholder
	}
	return strings.Join(out, "\n")
}

func countAlphaNum(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}
