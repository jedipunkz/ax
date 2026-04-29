package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/jedipunkz/ax/internal/store"
)

// ListWorktrees prints all agents with their name/id, repo, ended time, and directory name.
func ListWorktrees() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	stateFile := filepath.Join(home, ".ax", "state.json")
	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		fmt.Println("no agents found")
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read state file: %w", err)
	}

	var agents []store.AgentState
	if err := json.Unmarshal(data, &agents); err != nil {
		return fmt.Errorf("could not parse state file: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("no agents found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME/ID\tREPO\tSTATUS\tENDED\tDIRECTORY")
	for _, a := range agents {
		nameID := a.ID
		if a.Name != "" {
			nameID = a.Name + " (" + a.ID + ")"
		}

		repo := a.RepoName
		if repo == "" {
			repo = "-"
		}

		ended := "-"
		if a.FinishedAt != nil {
			ended = a.FinishedAt.Local().Format(time.DateTime)
		}

		dir := filepath.Base(a.WorkDir)
		if a.WorkDir == "" {
			dir = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", nameID, repo, string(a.Status), ended, dir)
	}
	return w.Flush()
}
