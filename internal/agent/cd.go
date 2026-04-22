package agent

import "fmt"

// PrintWorktreeDir finds an agent by ID or name and prints its working directory
// to stdout. This is intended for use with shell cd: cd $(ax agent cd -n <name>)
func PrintWorktreeDir(idOrName string) error {
	agent, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if agent.WorkDir == "" {
		return fmt.Errorf("agent %q has no working directory", idOrName)
	}
	fmt.Println(agent.WorkDir)
	return nil
}
