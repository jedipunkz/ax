package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/jedipunkz/ax/internal/store"
	"golang.org/x/term"
)

// attachStripRe matches ANSI/VT escape sequences for the ANSI-stripped logs
// flow. It mirrors the regex used in internal/agent/runner.go.
var attachStripRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

var signalExitCleanup = struct {
	sync.Mutex
	id uint64
	fn func()
}{}

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

	restoreInput := installControlCByteExit(os.Stdin, os.Exit)
	clearSignalCleanup := setSignalExitCleanup(restoreInput)
	defer clearSignalCleanup()
	defer restoreInput()

	return followStream(socketPath, existing.ID)
}

// installSignalExit registers a goroutine that immediately terminates the
// process on SIGINT/SIGTERM. We use a hard os.Exit instead of a context to
// guarantee termination even if a goroutine is blocked in a syscall (slow
// stdout write, large file read, daemon stuck) at the moment the signal
// arrives. The exit code follows the shell convention 128 + signal number.
func installSignalExit() {
	installSignalExitWith(os.Exit)
}

func installSignalExitWith(exit func(int)) {
	var once sync.Once
	exitWith := func(code int) {
		once.Do(func() {
			runSignalExitCleanup()
			exit(code)
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		code := 130 // SIGINT
		if sig == syscall.SIGTERM {
			code = 143
		}
		exitWith(code)
	}()
}

func setSignalExitCleanup(fn func()) func() {
	signalExitCleanup.Lock()
	prevID := signalExitCleanup.id
	prevFn := signalExitCleanup.fn
	signalExitCleanup.id++
	id := signalExitCleanup.id
	signalExitCleanup.fn = fn
	signalExitCleanup.Unlock()

	return func() {
		signalExitCleanup.Lock()
		if signalExitCleanup.id == id {
			signalExitCleanup.id = prevID
			signalExitCleanup.fn = prevFn
		}
		signalExitCleanup.Unlock()
	}
}

func runSignalExitCleanup() {
	signalExitCleanup.Lock()
	fn := signalExitCleanup.fn
	signalExitCleanup.fn = nil
	signalExitCleanup.Unlock()

	if fn != nil {
		fn()
	}
}

func installControlCByteExit(stdin *os.File, exit func(int)) func() {
	restore := func() {}
	if stdin != nil && term.IsTerminal(int(stdin.Fd())) {
		oldState, err := term.MakeRaw(int(stdin.Fd()))
		if err == nil {
			var restoreOnce sync.Once
			restore = func() {
				restoreOnce.Do(func() {
					_ = term.Restore(int(stdin.Fd()), oldState)
				})
			}
		}
	}

	if stdin != nil {
		go exitOnControlCByte(stdin, func() {
			restore()
			exit(130)
		})
	}
	return restore
}

func exitOnControlCByte(r io.Reader, onInterrupt func()) {
	var buf [256]byte
	recent := make([]byte, 0, 64)
	for {
		n, err := r.Read(buf[:])
		for _, b := range buf[:n] {
			recent = append(recent, b)
			if len(recent) > 64 {
				recent = recent[len(recent)-64:]
			}
			if isControlCInput(recent) {
				onInterrupt()
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func isControlCInput(input []byte) bool {
	if bytes.Contains(input, []byte{0x03}) {
		return true
	}
	for _, seq := range [][]byte{
		[]byte("\x1b[99;5:1u"), // kitty keyboard protocol: Ctrl-c key press
		[]byte("\x1b[99;5u"),   // common variant without event type
		[]byte("[99;5:1u"),     // same sequence when the ESC byte was eaten
		[]byte("[99;5u"),
		[]byte("\x1b[67;5:1u"), // Ctrl-Shift-c key press
		[]byte("\x1b[67;5u"),   // common variant without event type
		[]byte("[67;5:1u"),
		[]byte("[67;5u"),
	} {
		if bytes.Contains(input, seq) {
			return true
		}
	}
	return false
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
