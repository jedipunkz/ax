package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/creack/pty"
	"github.com/jedipunkz/ax/internal/store"
	"golang.org/x/term"
)

// waitingUserThreshold is how long with no output before we consider the agent
// to be waiting for user input rather than processing.
const waitingUserThreshold = 2 * time.Second

// normalizeAgentType returns agentType unchanged if non-empty, or "claude" as default.
// Delegates to store.AgentState.AgentTypeName for consistent behavior.
func normalizeAgentType(agentType string) string {
	return (store.AgentState{AgentType: agentType}).AgentTypeName()
}

// Run starts an interactive agent session and reports agent lifecycle
// state to the store daemon. agentType is the binary to invoke (e.g. "claude",
// "codex", "gemini"); an empty string defaults to "claude".
func Run(args []string, socketPath string, name string, agentType string) error {
	agentType = normalizeAgentType(agentType)
	id := generateID()

	workDir, err := os.Getwd()
	if err != nil {
		workDir = ""
	}

	var worktreeBranch, repoName string
	if workDir != "" {
		if repoRoot, ok := detectGitRepo(workDir); ok {
			repoName = filepath.Base(repoRoot)
			wt, branch, wtErr := setupWorktree(id, repoRoot, name)
			if wtErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not create worktree: %v\n", wtErr)
			} else {
				workDir = wt
				worktreeBranch = branch
			}
		}
	}

	return runSession(args, socketPath, id, name, agentType, workDir, worktreeBranch, repoName)
}

// resumePrefixArgs returns the arguments that should be prepended to resume a
// previous session for the given agent binary. The mapping reflects each tool's
// own session-continuation interface:
//
//	claude    --resume
//	gemini    --resume
//	codex     resume --last
//	opencode  --continue
//
// For unknown agent types no prefix is added; the agent is launched fresh in
// the existing worktree.
func resumePrefixArgs(agentType string) []string {
	switch agentType {
	case "claude", "gemini":
		return []string{"--resume"}
	case "codex":
		return []string{"resume", "--last"}
	case "opencode":
		return []string{"--continue"}
	default:
		return nil
	}
}

// ResumeByIDOrName finds an existing agent by ID or name and launches it in
// its worktree using the appropriate resume arguments for the agent type.
func ResumeByIDOrName(args []string, socketPath string, idOrName string) error {
	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(existing.WorkDir); err != nil {
		return fmt.Errorf("worktree directory %q no longer exists: %w", existing.WorkDir, err)
	}

	agentType := normalizeAgentType(existing.AgentType)
	id := generateID()
	resumeArgs := append(resumePrefixArgs(agentType), args...)
	return runSession(resumeArgs, socketPath, id, existing.Name, agentType, existing.WorkDir, existing.WorktreeBranch, existing.RepoName)
}

// findAgentByIDOrName reads state.json and returns the agent matching the given ID exactly,
// or falls back to the most recent agent matching by name.
func findAgentByIDOrName(idOrName string) (store.AgentState, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return store.AgentState{}, fmt.Errorf("could not determine home directory: %w", err)
	}
	stateFile := filepath.Join(home, ".ax", "state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return store.AgentState{}, fmt.Errorf("could not read state file: %w", err)
	}
	var agents []store.AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return store.AgentState{}, fmt.Errorf("could not parse state file: %w", err)
	}

	// Search by ID first (exact match).
	for i := range agents {
		if agents[i].ID == idOrName {
			return agents[i], nil
		}
	}

	// Fall back to name search (most recent match).
	sanitized := sanitizeBranchName(idOrName)
	var best *store.AgentState
	for i := range agents {
		a := &agents[i]
		if a.Name == idOrName || (sanitized != "" && a.WorktreeBranch == sanitized) {
			if best == nil || a.StartedAt.After(best.StartedAt) {
				best = a
			}
		}
	}
	if best == nil {
		return store.AgentState{}, fmt.Errorf("no agent found with ID or name %q", idOrName)
	}
	return *best, nil
}

// runSession is the shared implementation for Run and Resume.
func runSession(args []string, socketPath, id, name, agentType, workDir, worktreeBranch, repoName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	agentDir := filepath.Join(home, ".ax", "agents", id)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("could not create agent dir: %w", err)
	}

	logPath := filepath.Join(agentDir, "output.log")

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

	// Record the HEAD commit before the session so we can diff afterwards.
	initialHead := gitHeadCommit(workDir)

	cmd := exec.Command(agentType, claudeArgs...)
	cmd.Dir = workDir

	now := time.Now()
	state := store.AgentState{
		ID:             id,
		Name:           name,
		AgentType:      agentType,
		Args:           claudeArgs,
		WorkDir:        workDir,
		Status:         store.StatusRunning,
		StartedAt:      now,
		LastOutput:     "interactive session",
		LogFile:        logPath,
		WorktreeBranch: worktreeBranch,
		RepoName:       repoName,
	}

	// Start the agent inside a PTY so it sees a real terminal while we can also
	// monitor its output.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("could not start %s: %w", agentType, err)
	}
	defer ptmx.Close()

	// done is closed when the PTY read loop finishes so background goroutines exit.
	done := make(chan struct{})
	defer close(done)

	// Propagate terminal resize events to the PTY (Unix only).
	setupWinchHandler(ptmx, done)

	// Put our own stdin in raw mode so keystrokes go straight through.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd()))
		if rawErr == nil {
			defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
		}
	}

	// Forward our stdin to the PTY (user keystrokes → Claude).
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()

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

	// Periodically collect new git commits made during the session so that
	// ax dash reflects them even while the agent is still running.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				commits := gitNewCommits(workDir, initialHead)
				mu.Lock()
				changed := !stringSliceEqual(state.Commits, commits)
				if changed {
					state.Commits = commits
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

	// Collect any git commits made during the session by comparing HEAD now
	// against the HEAD recorded before the session started.
	state.Commits = gitNewCommits(workDir, initialHead)

	if err := client.SendUpdate(state); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not send final state: %v\n", err)
	}

	return nil
}

var outputCleanRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|[)(][AB012]|[A-Z\\^_@]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// gitHeadCommit returns the full SHA of HEAD in workDir, or "" if not a git repo.
func gitHeadCommit(workDir string) string {
	if workDir == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", workDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitNewCommits returns the full SHAs of commits reachable from HEAD but not
// from before (oldest-first). Returns nil when workDir is not a git repo or
// before is empty (e.g. the repo had no commits before the session).
func gitNewCommits(workDir, before string) []string {
	if workDir == "" || before == "" {
		return nil
	}
	out, err := exec.Command(
		"git", "-C", workDir,
		"log", "--format=%H", "--reverse",
		before+"..HEAD",
	).Output()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

// lastMeaningfulLine extracts the last readable text line from a raw PTY output chunk.
func lastMeaningfulLine(chunk []byte) string {
	s := outputCleanRe.ReplaceAllString(string(chunk), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		alpha := 0
		for _, r := range line {
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

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func generateID() string {
	ts := time.Now().Unix() / 60
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		b = []byte{0, 0}
	}
	return fmt.Sprintf("ax-%d-%s", ts, hex.EncodeToString(b))
}
