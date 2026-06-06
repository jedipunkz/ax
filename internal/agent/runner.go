package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
	"github.com/jedipunkz/ax/internal/textutil"
	"golang.org/x/term"
)

// waitingUserThreshold is how long with no output before we consider the agent
// to be waiting for user input rather than processing.
const waitingUserThreshold = 2 * time.Second

// Run starts an interactive agent session and reports agent lifecycle
// state to the store daemon. agentType is the binary to invoke (e.g. "claude",
// "codex", "gemini"); an empty string defaults to "claude".
func Run(args []string, socketPath string, name string, agentType string) error {
	agentType = store.AgentState{AgentType: agentType}.AgentTypeName()

	if name != "" {
		if existing, err := findAgentByIDOrName(name); err == nil {
			return fmt.Errorf(
				"agent %q already exists (status: %s)\nhint: use 'ax agent resume -n %s' to resume it",
				name, existing.Status, name,
			)
		}
	}

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
				warnf("could not create worktree: %v", wtErr)
			} else {
				workDir = wt
				worktreeBranch = branch
			}
		}
	}

	return runSession(args, socketPath, id, name, agentType, workDir, worktreeBranch, repoName)
}

// resumePrefixArgs returns the arguments that should be prepended to resume a
// previous session for the given agent binary. Unknown agents return nil and
// are relaunched fresh in the existing worktree.
func resumePrefixArgs(agentType string) []string {
	return lookupAgent(agentType).ResumeArgs
}

// ResumeByIDOrName finds an existing agent by ID or name and launches it in
// its worktree using the appropriate resume arguments for the agent type.
// agentTypeOverride, when non-empty, replaces the agent type stored in state.
func ResumeByIDOrName(args []string, socketPath string, idOrName string, agentTypeOverride string) error {
	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(existing.WorkDir); err != nil {
		return fmt.Errorf("worktree directory %q no longer exists: %w", existing.WorkDir, err)
	}

	agentType := existing.AgentTypeName()
	if agentTypeOverride != "" {
		agentType = agentTypeOverride
	}
	resumeArgs := buildResumeArgs(agentType, args)
	return runSession(resumeArgs, socketPath, existing.ID, existing.Name, agentType, existing.WorkDir, existing.WorktreeBranch, existing.RepoName)
}

// buildResumeArgs assembles the final argv for resuming an agent session.
//
// User-supplied args (everything after `--` on the ax CLI) are placed BEFORE
// the agent's resume tokens so they act as agent-level global flags rather
// than as arguments to the resume subcommand. For example, with codex:
//
//	ax agent resume -a codex -n NAME -- -a never
//	→ codex -a never resume --last        (correct: -a is codex's approval flag)
//	NOT codex resume --last -- -a never  (wrong: codex parses -a as SESSION_ID)
//
// The leading "--" separator that cobra forwards is stripped before merging.
func buildResumeArgs(agentType string, userArgs []string) []string {
	userArgs = stripLeadingDoubleDash(userArgs)
	prefix := resumePrefixArgs(agentType)
	out := make([]string, 0, len(userArgs)+len(prefix))
	out = append(out, userArgs...)
	out = append(out, prefix...)
	return out
}

// runSession is the shared implementation for Run and Resume. It owns
// the PTY, the log file, and the lifecycle goroutines (idle watcher,
// commit watcher, daemon input pump), and produces the terminal state
// update before returning.
func runSession(args []string, socketPath, id, name, agentType, workDir, worktreeBranch, repoName string) error {
	paths, err := axfs.New()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.AgentDir(id), 0755); err != nil {
		return fmt.Errorf("could not create agent dir: %w", err)
	}
	logPath := paths.AgentLog(id)

	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to store: %w", err)
	}
	defer client.Close()

	agentArgs := stripLeadingDoubleDash(args)
	initialHead := gitHeadCommit(workDir)

	cmd := exec.Command(agentType, agentArgs...)
	cmd.Dir = workDir

	monitor := newSessionMonitor(&client, store.AgentState{
		ID:             id,
		Name:           name,
		AgentType:      agentType,
		Args:           agentArgs,
		WorkDir:        workDir,
		Status:         store.StatusRunning,
		StartedAt:      time.Now(),
		LastOutput:     "interactive session",
		LogFile:        logPath,
		WorktreeBranch: worktreeBranch,
		RepoName:       repoName,
	})

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("could not start %s: %w", agentType, err)
	}
	defer ptmx.Close()

	// done is closed when the PTY read loop finishes so background goroutines exit.
	done := make(chan struct{})
	defer close(done)

	setupWinchHandler(ptmx, done)
	if restore := makeRawStdin(); restore != nil {
		defer restore()
	}

	// Forward our stdin to the PTY (user keystrokes → agent).
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()

	initial := monitor.setPID(cmd.Process.Pid)
	if err := client.SendUpdate(initial); err != nil {
		warnf("could not send initial state: %v", err)
	}
	if err := client.RegisterInput(id); err != nil {
		warnf("could not register input listener: %v", err)
	}

	go forwardDaemonInput(&client, ptmx, id)
	go monitor.runIdleWatcher(done, waitingUserThreshold)
	go monitor.runCommitWatcher(done, workDir, initialHead)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}
	defer logFile.Close()

	forwardPTYOutput(ptmx, logFile, monitor)

	final := monitor.finalise(cmd.Wait(), workDir, initialHead)
	if err := client.SendUpdate(final); err != nil {
		warnf("could not send final state: %v", err)
	}
	return nil
}

// makeRawStdin puts the controlling terminal into raw mode and returns
// a restore function. Returns nil when stdin is not a TTY or raw mode
// could not be entered — the caller treats that as "nothing to undo".
func makeRawStdin() func() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil
	}
	return func() { _ = term.Restore(fd, oldState) }
}

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
	return textutil.LastMeaningfulLine(chunk)
}

func generateID() string {
	ts := time.Now().Unix() / 60
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		b = []byte{0, 0}
	}
	return fmt.Sprintf("ax-%d-%s", ts, hex.EncodeToString(b))
}
