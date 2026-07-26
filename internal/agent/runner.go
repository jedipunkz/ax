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

// sessionConfig contains the immutable inputs required to launch and monitor
// one agent process. Keeping them together makes both new and resumed
// sessions use the same execution path without a long, order-sensitive list
// of function parameters.
type sessionConfig struct {
	args           []string
	socketPath     string
	id             string
	name           string
	agentType      string
	workDir        string
	worktreeBranch string
	repoName       string
}

func (c sessionConfig) initialState(logPath string) store.AgentState {
	return store.AgentState{
		ID:             c.id,
		Name:           c.name,
		AgentType:      c.agentType,
		Args:           append([]string(nil), c.args...),
		WorkDir:        c.workDir,
		Status:         store.StatusRunning,
		StartedAt:      time.Now(),
		LastOutput:     "interactive session",
		LogFile:        logPath,
		WorktreeBranch: c.worktreeBranch,
		RepoName:       c.repoName,
	}
}

func (c sessionConfig) command() *exec.Cmd {
	cmd := exec.Command(c.agentType, c.args...)
	cmd.Dir = c.workDir
	return cmd
}

// Run starts an interactive agent session and reports agent lifecycle
// state to the store daemon. agentType is the binary to invoke (e.g. "claude",
// "codex", "gemini"); an empty string defaults to "claude".
func Run(args []string, socketPath string, name string, agentType string) error {
	agentType = store.AgentState{AgentType: agentType}.AgentTypeName()

	if name != "" {
		if existing, err := findAgentByIDOrName(name); err == nil {
			if existing.Status == store.StatusRunning {
				return fmt.Errorf(
					"agent %q is already running\nhint: use 'ax agent resume -n %s' to resume it",
					name, name,
				)
			}
			// Terminal state (killed/failed/success): allow reuse of the name.
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
				return worktreeSetupError(repoRoot, wtErr)
			}
			workDir = wt
			worktreeBranch = branch
		}
	}

	return runSession(sessionConfig{
		args:           stripLeadingDoubleDash(args),
		socketPath:     socketPath,
		id:             id,
		name:           name,
		agentType:      agentType,
		workDir:        workDir,
		worktreeBranch: worktreeBranch,
		repoName:       repoName,
	})
}

// worktreeSetupError explains why the session was aborted instead of falling
// back to repoRoot. Isolation is the point of starting an agent through ax, and
// a warning printed here is immediately overwritten by the agent's full-screen
// UI — so a silent fallback would leave an agent committing directly to the
// branch the user is sitting on without them ever seeing why.
//
// An unborn HEAD is called out separately because it is the common cause: a
// freshly initialised repository has no commit for the worktree to branch from.
func worktreeSetupError(repoRoot string, err error) error {
	if gitHeadCommit(repoRoot) == "" {
		return fmt.Errorf(
			"could not create an isolated worktree for %s: the repository has no commits yet\n"+
				"hint: create an initial commit, then run 'ax agent new' again\nunderlying error: %w",
			repoRoot, err,
		)
	}
	return fmt.Errorf(
		"could not create an isolated worktree for %s: %w\n"+
			"ax will not run an agent directly in your repository; resolve the error above and retry",
		repoRoot, err,
	)
}

// resumePrefixArgs returns the arguments that should be prepended to resume a
// previous session for the given agent binary. Unknown agents return nil and
// are relaunched fresh in the existing worktree.
func resumePrefixArgs(agentType string) []string {
	return lookupAgent(agentType).ResumeCommand()
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
	return runSession(sessionConfig{
		args:           resumeArgs,
		socketPath:     socketPath,
		id:             existing.ID,
		name:           existing.Name,
		agentType:      agentType,
		workDir:        existing.WorkDir,
		worktreeBranch: existing.WorktreeBranch,
		repoName:       existing.RepoName,
	})
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
func runSession(config sessionConfig) error {
	paths, err := axfs.New()
	if err != nil {
		return err
	}
	agentDir := paths.AgentDir(config.id)
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return fmt.Errorf("could not create agent dir: %w", err)
	}
	// MkdirAll leaves existing directory permissions untouched, so enforce
	// owner-only mode explicitly to cover dirs created by prior versions.
	if err := os.Chmod(agentDir, 0700); err != nil {
		return fmt.Errorf("could not secure agent dir: %w", err)
	}
	logPath := paths.AgentLog(config.id)

	var client store.Client
	if err := client.Connect(config.socketPath); err != nil {
		return fmt.Errorf("could not connect to store: %w", err)
	}
	defer client.Close()

	initialHead := gitHeadCommit(config.workDir)
	cmd := config.command()
	monitor := newSessionMonitor(&client, config.initialState(logPath))

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("could not start %s: %w", config.agentType, err)
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
	if err := client.RegisterInput(config.id); err != nil {
		warnf("could not register input listener: %v", err)
	}

	go forwardDaemonInput(&client, ptmx, config.id)
	go monitor.runIdleWatcher(done, waitingUserThreshold)
	go monitor.runCommitWatcher(done, config.workDir, initialHead)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}
	// OpenFile only applies the mode on creation, so chmod existing logs
	// (e.g. carried over from prior versions) to owner-only as well.
	if err := os.Chmod(logPath, 0600); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("could not secure log file: %w", err)
	}
	defer logFile.Close()

	forwardPTYOutput(ptmx, logFile, monitor)

	final := monitor.finalise(cmd.Wait(), config.workDir, initialHead)
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
