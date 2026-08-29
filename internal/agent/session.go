package agent

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/jedipunkz/agx/internal/store"
	"github.com/jedipunkz/agx/internal/textutil"
)

// sessionMonitor owns the mutable state for an in-flight agent session
// and pushes incremental updates to the daemon. Every field is guarded
// by mu so the PTY reader, the idle watcher, and the commit watcher
// can operate concurrently without races.
type sessionMonitor struct {
	mu           sync.Mutex
	state        store.AgentState
	lastActivity time.Time
	waitingUser  bool

	client *store.Client
}

// newSessionMonitor seeds the monitor with the initial running state.
// lastActivity is set to now so the idle watcher does not declare the
// agent waiting before the first byte ever arrives.
func newSessionMonitor(client *store.Client, initial store.AgentState) *sessionMonitor {
	return &sessionMonitor{
		state:        initial,
		lastActivity: time.Now(),
		client:       client,
	}
}

// setPID records the OS PID of the spawned agent and returns the
// updated snapshot for the caller to send as the initial state.
func (m *sessionMonitor) setPID(pid int) store.AgentState {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.PID = pid
	return m.state
}

// recordActivity bumps lastActivity to "now". If we were marked as
// waiting for user input, transition back to processing and tell the
// daemon — this matches the original runSession behavior where every
// PTY read flips waiting=false the moment output resumes.
func (m *sessionMonitor) recordActivity() {
	m.mu.Lock()
	m.lastActivity = time.Now()
	if !m.waitingUser {
		m.mu.Unlock()
		return
	}
	m.waitingUser = false
	m.state.WaitingUser = false
	snap := m.state
	m.mu.Unlock()
	_ = m.client.SendUpdate(snap)
}

// updateLastOutput pushes a state update when the last meaningful line
// extracted from chunk differs from the one we already reported.
func (m *sessionMonitor) updateLastOutput(chunk []byte) {
	line := textutil.LastMeaningfulLine(chunk)
	if line == "" {
		return
	}
	m.mu.Lock()
	if m.state.LastOutput == line {
		m.mu.Unlock()
		return
	}
	m.state.LastOutput = line
	snap := m.state
	m.mu.Unlock()
	_ = m.client.SendUpdate(snap)
}

// runIdleWatcher periodically checks whether the agent has been quiet
// long enough to count as "waiting for user input" and notifies the
// daemon on transitions. Exits when done is closed.
func (m *sessionMonitor) runIdleWatcher(done <-chan struct{}, threshold time.Duration) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			m.mu.Lock()
			idle := time.Since(m.lastActivity) > threshold
			if idle == m.waitingUser {
				m.mu.Unlock()
				continue
			}
			m.waitingUser = idle
			m.state.WaitingUser = idle
			snap := m.state
			m.mu.Unlock()
			_ = m.client.SendUpdate(snap)
		}
	}
}

// runCommitWatcher periodically refreshes the commit list so the
// dashboard sees in-flight commits without waiting for the session to
// end. Exits when done is closed.
func (m *sessionMonitor) runCommitWatcher(done <-chan struct{}, workDir, initialHead string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			commits := gitNewCommits(workDir, initialHead)
			m.mu.Lock()
			if slices.Equal(m.state.Commits, commits) {
				m.mu.Unlock()
				continue
			}
			m.state.Commits = commits
			snap := m.state
			m.mu.Unlock()
			_ = m.client.SendUpdate(snap)
		}
	}
}

// finalise stamps the terminal state (Status, ExitCode, FinishedAt,
// Commits) based on the result of cmd.Wait and returns the snapshot
// that should be sent as the final update.
func (m *sessionMonitor) finalise(exitErr error, workDir, initialHead string) store.AgentState {
	exitCode, signaled := classifyExit(exitErr)

	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.state.FinishedAt = &now
	m.state.WaitingUser = false
	m.state.ExitCode = &exitCode
	switch {
	case signaled:
		m.state.Status = store.StatusKilled
	case exitCode == 0:
		m.state.Status = store.StatusSuccess
	default:
		m.state.Status = store.StatusFailed
	}
	m.state.Commits = gitNewCommits(workDir, initialHead)
	return m.state
}

// classifyExit maps the cmd.Wait error to (exitCode, signaled).
// A nil error means the process exited cleanly with code 0.
// A non-ExitError (e.g. the wait itself failed) is treated as code 1.
func classifyExit(exitErr error) (int, bool) {
	if exitErr == nil {
		return 0, false
	}
	ee, ok := exitErr.(*exec.ExitError)
	if !ok {
		return 1, false
	}
	code := ee.ExitCode()
	ws, ok := ee.Sys().(syscall.WaitStatus)
	return code, ok && ws.Signaled()
}

// forwardPTYOutput reads PTY output and writes it to stdout + log file,
// invoking the monitor's activity hooks on every non-empty chunk.
// Returns when the PTY signals EOF or an unrecoverable read error.
func forwardPTYOutput(ptmx io.Reader, logFile io.Writer, monitor *sessionMonitor) {
	out := io.MultiWriter(os.Stdout, logFile)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			monitor.recordActivity()
			_, _ = out.Write(buf[:n])
			monitor.updateLastOutput(buf[:n])
		}
		if readErr != nil {
			return
		}
	}
}

// forwardDaemonInput reads "input" messages forwarded by the daemon
// (from `agx agent input` or future remote senders) and writes them to
// the PTY. Returns on connection error so the parent's defer chain can
// run normally.
func forwardDaemonInput(client *store.Client, w io.Writer, agentID string) {
	for {
		msg, err := client.ReadMessage()
		if err != nil {
			return
		}
		if msg.Type != "input" || msg.AgentID != agentID {
			continue
		}
		data, decErr := base64.StdEncoding.DecodeString(msg.Data)
		if decErr != nil {
			continue
		}
		_, _ = w.Write(data)
	}
}

// stripLeadingDoubleDash drops a leading "--" that cobra forwarded so
// the agent process does not see the cobra separator.
func stripLeadingDoubleDash(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// warnf writes a one-line warning to stderr. Centralising the format
// keeps every "warning: …" message consistent.
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}
