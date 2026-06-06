package agent

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jedipunkz/ax/internal/axfs"
	"github.com/jedipunkz/ax/internal/store"
)

// ListWorktrees prints all agents with their name/id, repo, ended time, and directory name.
func ListWorktrees() error {
	paths, err := axfs.New()
	if err != nil {
		return err
	}
	agents, err := store.ReadAgents(paths.StateFile())
	if os.IsNotExist(err) {
		fmt.Println("no agents found")
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read state file: %w", err)
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
			if paths.Home != "" && strings.HasPrefix(dir, paths.Home) {
				dir = "~" + dir[len(paths.Home):]
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, name, repo, ended, dir)
	}
	return w.Flush()
}
