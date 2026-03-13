package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/creack/pty"
	"github.com/thirai/cco/internal/store"
	"golang.org/x/term"
)

// waitingUserThreshold is how long with no output before we consider Claude
// to be waiting for user input rather than processing.
const waitingUserThreshold = 2 * time.Second

// Run starts an interactive Claude Code session with all permissions allowed,
// and reports agent lifecycle state to the store daemon.
func Run(args []string, socketPath string) error {
	id := generateID()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	agentDir := filepath.Join(home, ".cco", "agents", id)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("could not create agent dir: %w", err)
	}

	logPath := filepath.Join(agentDir, "output.log")

	workDir, err := os.Getwd()
	if err != nil {
		workDir = ""
	}

	var worktreeBranch string
	if workDir != "" {
		if repoRoot, ok := detectGitRepo(workDir); ok {
			wt, branch, wtErr := setupWorktree(id, repoRoot)
			if wtErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not create worktree: %v\n", wtErr)
			} else {
				workDir = wt
				worktreeBranch = branch
			}
		}
	}

	// Connect to store
	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to store: %w", err)
	}
	defer client.Close()

	// Strip leading "--" separator if present (cobra passes it through)
	claudeArgs := args
	if len(claudeArgs) > 0 && claudeArgs[0] == "--" {
		claudeArgs = claudeArgs[1:]
	}

	// Always run with all permissions allowed (interactive mode)
	claudeArgs = append([]string{"--dangerously-skip-permissions"}, claudeArgs...)

	cmd := exec.Command("claude", claudeArgs...)
	cmd.Dir = workDir

	now := time.Now()
	state := store.AgentState{
		ID:             id,
		Args:           claudeArgs,
		WorkDir:        workDir,
		Status:         store.StatusRunning,
		StartedAt:      now,
		LastOutput:     "interactive session",
		LogFile:        logPath,
		WorktreeBranch: worktreeBranch,
	}

	// Start claude inside a PTY so it sees a real terminal while we can also
	// monitor its output.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("could not start claude: %w", err)
	}
	defer ptmx.Close()

	// Propagate terminal resize events to the PTY.
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	defer signal.Stop(sigWinch)

	// done is closed when the PTY read loop finishes so background goroutines exit.
	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-sigWinch:
				_ = pty.InheritSize(os.Stdin, ptmx)
			}
		}
	}()
	_ = pty.InheritSize(os.Stdin, ptmx) // set initial size

	// Put our own stdin in raw mode so keystrokes go straight through.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd()))
		if rawErr == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}

	// Forward our stdin to the PTY (user keystrokes → Claude).
	go io.Copy(ptmx, os.Stdin)

	state.PID = cmd.Process.Pid
	if err := client.SendUpdate(state); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not send initial state: %v\n", err)
	}

	// --- activity monitoring ---
	var (
		mu           sync.Mutex
		lastActivity = time.Now()
		waitingUser  bool
	)

	// Periodically check whether Claude has been idle long enough to be
	// considered "waiting for user input".
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				idle := time.Since(lastActivity) > waitingUserThreshold
				changed := idle != waitingUser
				if changed {
					waitingUser = idle
					state.WaitingUser = waitingUser
					s := state
					mu.Unlock()
					_ = client.SendUpdate(s)
				} else {
					mu.Unlock()
				}
			}
		}
	}()

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}
	defer logFile.Close()

	out := io.MultiWriter(os.Stdout, logFile)

	// Forward PTY output to our stdout while tracking activity time.
	buf := make([]byte, 32*1024)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			mu.Lock()
			lastActivity = time.Now()
			if waitingUser {
				waitingUser = false
				state.WaitingUser = false
				s := state
				mu.Unlock()
				_ = client.SendUpdate(s)
			} else {
				mu.Unlock()
			}
			_, _ = out.Write(buf[:n])

			if line := lastMeaningfulLine(buf[:n]); line != "" {
				mu.Lock()
				changed := state.LastOutput != line
				if changed {
					state.LastOutput = line
				}
				s := state
				mu.Unlock()
				if changed {
					_ = client.SendUpdate(s)
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	// Wait for the process to finish.
	exitErr := cmd.Wait()

	finishedAt := time.Now()
	state.FinishedAt = &finishedAt
	state.WaitingUser = false

	exitCode := 0
	signaled := false
	if exitErr != nil {
		if ee, ok := exitErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				signaled = true
			}
		} else {
			exitCode = 1
		}
	}
	state.ExitCode = &exitCode

	switch {
	case signaled:
		state.Status = store.StatusKilled
	case exitCode == 0:
		state.Status = store.StatusSuccess
	default:
		state.Status = store.StatusFailed
	}

	if err := client.SendUpdate(state); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not send final state: %v\n", err)
	}

	return nil
}

var outputCleanRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// lastMeaningfulLine extracts the last readable text line from a raw PTY output chunk.
func lastMeaningfulLine(chunk []byte) string {
	s := outputCleanRe.ReplaceAllString(string(chunk), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		alpha := 0
		for _, r := range []rune(line) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				alpha++
			}
		}
		if alpha >= 4 {
			return line
		}
	}
	return ""
}

func generateID() string {
	ts := time.Now().Unix() / 60
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		b = []byte{0, 0}
	}
	return fmt.Sprintf("cco-%d-%s", ts, hex.EncodeToString(b))
}
