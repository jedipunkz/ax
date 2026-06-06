package agent

import (
	"context"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}

	if existing.LogFile != "" {
		if data, err := os.ReadFile(existing.LogFile); err == nil {
			_, _ = os.Stdout.WriteString(stripANSI(data))
		}
	}

	if ctx.Err() != nil {
		return nil
	}
	if !follow {
		return nil
	}
	if existing.Status.IsTerminal() {
		return nil
	}

	return runAttachLoop(ctx, socketPath, existing.ID, 0, true /* strip */)
}

// AttachAgent implements `ax agent attach -n <id|name>`. The connection
// streams raw PTY bytes (ANSI preserved) so callers see colours, prompts and
// progress bars exactly as they were produced.
func AttachAgent(socketPath, idOrName string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}

	return runAttachLoop(ctx, socketPath, existing.ID, 8192, false /* strip */)
}

// runAttachLoop opens a daemon connection, attaches to the given agent and
// writes incoming "output" payloads to stdout until "eof" arrives or ctx is
// cancelled (e.g. SIGINT). When strip is true, ANSI escapes are removed
// before writing.
func runAttachLoop(ctx context.Context, socketPath, agentID string, tail int, strip bool) error {
	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer client.Close()

	if err := client.Attach(agentID, tail); err != nil {
		return fmt.Errorf("could not send attach: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		for {
			msg, err := client.ReadMessage()
			if err != nil {
				// Non-blocking send so we don't deadlock if main has already
				// returned via the ctx.Done() path.
				select {
				case done <- err:
				default:
				}
				return
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
				if strip {
					_, _ = os.Stdout.WriteString(stripANSI(raw))
				} else {
					_, _ = os.Stdout.Write(raw)
				}
			case "attach_err":
				select {
				case done <- fmt.Errorf("attach failed: %s", msg.Error):
				default:
				}
				return
			case "eof":
				if msg.AgentID == agentID {
					select {
					case done <- nil:
					default:
					}
					return
				}
			}
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Best-effort detach (we don't wait for an ack), then close the
		// connection so the reader goroutine unblocks immediately.
		_ = client.Detach(agentID)
		client.Close()
		return nil
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
