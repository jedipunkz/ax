package agent

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/jedipunkz/ax/internal/store"
)

// AttachToAgent connects to a running agent's PTY output stream via the daemon
// and forwards local stdin as raw keystrokes. The session ends when the agent
// exits (daemon sends eof) or the connection drops.
//
// Keystrokes — including ^C — are forwarded to the agent rather than
// interpreted locally, matching the behaviour of `docker attach`. SIGINT and
// SIGTERM received as OS signals (e.g. `kill`) restore the terminal before
// exiting.
func AttachToAgent(socketPath, idOrName string) error {
	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if existing.Status != store.StatusRunning {
		return fmt.Errorf(
			"agent %q is not running (status: %s)\nhint: use 'ax agent resume -n %s' to restart a finished agent",
			idOrName, existing.Status, idOrName,
		)
	}

	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer client.Close()

	// Replay the last 50 KB of output so context is visible on attach.
	if err := client.Attach(existing.ID, 50*1024); err != nil {
		return fmt.Errorf("could not send attach: %w", err)
	}

	// Drain messages until the handshake completes.
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			return fmt.Errorf("attach handshake failed: %w", err)
		}
		if msg.Type == "attach_ok" {
			break
		}
		if msg.Type == "attach_err" {
			return fmt.Errorf("attach failed: %s", msg.Error)
		}
	}

	// Raw mode: keystrokes (including ^C) are forwarded to the agent, not
	// interpreted locally. Register terminal restore as the signal cleanup so
	// it runs before os.Exit when SIGINT/SIGTERM arrives as an OS signal.
	restore := makeRawStdin()
	if restore != nil {
		clearCleanup := setSignalExitCleanup(restore)
		defer clearCleanup()
		defer restore()
	}
	installSignalExit()

	// Forward stdin as raw_input; the goroutine exits when the connection
	// is closed by the deferred client.Close() after the output loop returns.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if sendErr := client.SendRawInput(existing.ID, buf[:n]); sendErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Stream agent output to stdout until eof or connection drop.
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			return nil
		}
		switch msg.Type {
		case "output":
			if msg.AgentID != existing.ID {
				continue
			}
			data, decErr := base64.StdEncoding.DecodeString(msg.Data)
			if decErr != nil {
				continue
			}
			_, _ = os.Stdout.Write(data)
		case "eof":
			if msg.AgentID == existing.ID {
				return nil
			}
		}
	}
}
