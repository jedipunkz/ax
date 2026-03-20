package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/jedipunkz/ax/internal/store"
	"github.com/spf13/cobra"
)

var wtCmd = &cobra.Command{
	Use:   "wt",
	Short: "Manage git worktrees",
}

var wtListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agent worktrees",
	RunE:  runWtList,
}

var wtDeleteForce bool

var wtDeleteCmd = &cobra.Command{
	Use:   "delete <id|name>",
	Short: "Delete an agent worktree and its branch",
	Args:  cobra.ExactArgs(1),
	RunE:  runWtDelete,
}

func runWtList(_ *cobra.Command, _ []string) error {
	agents, err := readStateFile()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	hasAny := false
	for _, a := range agents {
		if a.WorktreeBranch == "" {
			continue
		}
		if !hasAny {
			fmt.Fprintln(w, "ID\tNAME\tBRANCH\tSTATUS\tPATH")
			hasAny = true
		}
		name := a.Name
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, name, a.WorktreeBranch, string(a.Status), a.WorkDir)
	}
	w.Flush()

	if !hasAny {
		fmt.Println("No worktrees found.")
	}
	return nil
}

func runWtDelete(_ *cobra.Command, args []string) error {
	idOrName := args[0]
	agents, err := readStateFile()
	if err != nil {
		return err
	}

	a, ok := findWorktreeAgent(agents, idOrName)
	if !ok {
		return fmt.Errorf("no agent with a worktree found for %q", idOrName)
	}
	if a.Status == store.StatusRunning && !wtDeleteForce {
		return fmt.Errorf("agent %q is still running; use --force to delete anyway", idOrName)
	}

	worktreePath := a.WorkDir
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		fmt.Printf("Worktree directory %s does not exist, skipping removal.\n", worktreePath)
		return nil
	}

	mainRepo, err := worktreeMainRepo(worktreePath)
	if err != nil {
		return fmt.Errorf("could not determine main repository: %w", err)
	}

	if out, err := exec.Command("git", "-C", mainRepo, "worktree", "remove", "--force", worktreePath).CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w\n%s", err, out)
	}
	fmt.Printf("Removed worktree: %s\n", worktreePath)

	if out, err := exec.Command("git", "-C", mainRepo, "branch", "-D", a.WorktreeBranch).CombinedOutput(); err != nil {
		fmt.Printf("Warning: could not delete branch %q: %v\n%s\n", a.WorktreeBranch, err, out)
	} else {
		fmt.Printf("Deleted branch: %s\n", a.WorktreeBranch)
	}

	return nil
}

// worktreeMainRepo returns the main repository root for a linked git worktree.
func worktreeMainRepo(worktreePath string) (string, error) {
	out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	commonDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktreePath, commonDir)
	}
	// commonDir is the main .git directory; its parent is the repo root.
	return filepath.Dir(commonDir), nil
}

// readStateFile reads ~/.ax/state.json and returns all agent states.
func readStateFile() ([]store.AgentState, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".ax", "state.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read state file: %w", err)
	}
	var agents []store.AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("could not parse state file: %w", err)
	}
	return agents, nil
}

// findWorktreeAgent finds the most recent agent by ID or name that has a worktree.
func findWorktreeAgent(agents []store.AgentState, idOrName string) (store.AgentState, bool) {
	// Exact ID match first.
	for _, a := range agents {
		if a.ID == idOrName && a.WorktreeBranch != "" {
			return a, true
		}
	}
	// Name match — most recent wins.
	var best *store.AgentState
	for i := range agents {
		a := &agents[i]
		if a.WorktreeBranch == "" {
			continue
		}
		if a.Name == idOrName {
			if best == nil || a.StartedAt.After(best.StartedAt) {
				best = a
			}
		}
	}
	if best != nil {
		return *best, true
	}
	return store.AgentState{}, false
}

func init() {
	wtDeleteCmd.Flags().BoolVarP(&wtDeleteForce, "force", "f", false, "Delete worktree even if agent is still running")
	wtCmd.AddCommand(wtListCmd)
	wtCmd.AddCommand(wtDeleteCmd)
}
