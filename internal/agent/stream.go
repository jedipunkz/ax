package agent

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/jedipunkz/ax/internal/store"
)

// attachStripRe matches ANSI/VT escape sequences for the ANSI-stripped logs
// flow. It mirrors the regex used in internal/agent/runner.go.
var attachStripRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// StreamLogs implements `ax agent logs [-f] -n <id|name>`.
//
// Without follow: dumps the agent's log file with ANSI escapes stripped.
// With follow: dumps the existing log, then keeps streaming new output from
// the daemon (also stripped) until the agent terminates or the user
// interrupts.
func StreamLogs(socketPath, idOrName string, follow bool) error {
	installSignalExit()

	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}

	if existing.LogFile != "" {
		if data, err := os.ReadFile(existing.LogFile); err == nil {
			_, _ = os.Stdout.WriteString(stripANSI(data))
		}
	}

	if !follow {
		return nil
	}
	if existing.Status.IsTerminal() {
		return nil
	}

	return followStream(socketPath, existing.ID)
}

// installSignalExit registers a goroutine that immediately terminates the
// process on SIGINT/SIGTERM. We use a hard os.Exit instead of a context to
// guarantee termination even if a goroutine is blocked in a syscall (slow
// stdout write, large file read, daemon stuck) at the moment the signal
// arrives. The exit code follows the shell convention 128 + signal number.
func installSignalExit() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		code := 130 // SIGINT
		if sig == syscall.SIGTERM {
			code = 143
		}
		os.Exit(code)
	}()
}

// followStream connects to the daemon, attaches to the given agent and writes
// each ANSI-stripped output chunk to stdout until "eof" arrives or the
// connection drops. The signal watcher installed by installSignalExit ends
// the process on Ctrl-C without needing extra plumbing here.
func followStream(socketPath, agentID string) error {
	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer client.Close()

	if err := client.Attach(agentID, 0); err != nil {
		return fmt.Errorf("could not send attach: %w", err)
	}

	for {
		msg, err := client.ReadMessage()
		if err != nil {
			// Daemon closed the connection or an I/O error occurred — treat
			// it as end of stream.
			return nil
		}
		switch msg.Type {
		case "output":
			if msg.AgentID != agentID {
				continue
			}
			raw, decErr := base64.StdEncoding.DecodeString(msg.Data)
			if decErr != nil {
				continue
			}
			_, _ = os.Stdout.WriteString(stripANSI(raw))
		case "attach_err":
			return fmt.Errorf("attach failed: %s", msg.Error)
		case "eof":
			if msg.AgentID == agentID {
				return nil
			}
		}
	}
}

// stripANSI removes ANSI escape sequences and normalises CR/LF, returning a
// UTF-8 string suitable for plain-text consumption.
func stripANSI(b []byte) string {
	s := attachStripRe.ReplaceAllString(string(b), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
