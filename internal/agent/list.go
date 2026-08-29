package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// ListWorktrees prints all agents with their name/id, repo, ended time, and directory name.
func ListWorktrees() error {
	paths, agents, err := loadState()
	if errors.Is(err, errNoAgents) || (err == nil && len(agents) == 0) {
		fmt.Println("no agents found")
		return nil
	}
	if err != nil {
		return err
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
