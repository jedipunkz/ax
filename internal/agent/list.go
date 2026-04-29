package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	fmt.Fprintln(w, "ID\tNAME\tREPO\tENDED\tDIR")
	for _, a := range agents {
		name := a.Name
		if name == "" {
			name = "-"
		}

		repo := a.RepoName
		if repo == "" {
			repo = "-"
		}

		ended := "-"
		if a.FinishedAt != nil {
			ended = a.FinishedAt.Local().Format(time.DateTime)
		}

		dir := "-"
		if a.WorkDir != "" {
			dir = a.WorkDir
			if home != "" && strings.HasPrefix(dir, home) {
				dir = "~" + dir[len(home):]
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, name, repo, ended, dir)
	}
	return w.Flush()
}
