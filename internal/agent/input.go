package agent

import (
	"fmt"

	"github.com/jedipunkz/agx/internal/store"
)

// SendInput delivers data to the agent identified by idOrName via the daemon.
// The daemon only forwards input when the agent is in a waiting_user state;
// any other state results in an error returned here.
func SendInput(socketPath, idOrName, data string) error {
	existing, err := findAgentByIDOrName(idOrName)
	if err != nil {
		return err
	}
	if existing.Status != store.StatusRunning {
		return fmt.Errorf("agent is not running (status: %s)", existing.Status)
	}

	var client store.Client
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to daemon: %w", err)
	}
	defer client.Close()

	if err := client.SendInput(existing.ID, []byte(data)); err != nil {
		return fmt.Errorf("could not send input: %w", err)
	}

	msg, err := client.ReadMessage()
	if err != nil {
		return fmt.Errorf("no response from daemon: %w", err)
	}
	switch msg.Type {
	case "input_ok":
		return nil
	case "input_err":
		return fmt.Errorf("%s", msg.Error)
	default:
		return fmt.Errorf("unexpected response from daemon: %s", msg.Type)
	}
}
