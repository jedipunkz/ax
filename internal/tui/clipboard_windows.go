//go:build windows

package tui

import (
	"os/exec"
	"strings"
)

// copyToClipboard writes text to the OS clipboard using the Windows clip command.
func copyToClipboard(text string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
