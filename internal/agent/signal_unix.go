package agent

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
)

// setupWinchHandler propagates SIGWINCH terminal resize events to the PTY.
// It sets the initial PTY size and exits when done is closed.
func setupWinchHandler(ptmx *os.File, done <-chan struct{}) {
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	_ = pty.InheritSize(os.Stdin, ptmx)
	go func() {
		defer signal.Stop(sigWinch)
		for {
			select {
			case <-done:
				return
			case <-sigWinch:
				_ = pty.InheritSize(os.Stdin, ptmx)
			}
		}
	}()
}
