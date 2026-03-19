package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedipunkz/ax/internal/config"
	"github.com/jedipunkz/ax/internal/store"
)

// Run connects to the store daemon, subscribes for updates, and starts the TUI.
// cfg is used to apply the user's chosen theme before rendering begins.
func Run(socketPath string, cfg *config.Config) error {
	ApplyTheme(cfg.Palette())

	client := &store.Client{}
	if err := client.Connect(socketPath); err != nil {
		return fmt.Errorf("could not connect to store: %w", err)
	}

	if err := client.Subscribe(); err != nil {
		client.Close()
		return fmt.Errorf("could not subscribe: %w", err)
	}

	sub := make(chan store.Message, 64)

	// Start background goroutine to read messages from socket
	go func() {
		for {
			msg, err := client.ReadMessage()
			if err != nil {
				return
			}
			sub <- msg
		}
	}()

	m := newModel(client, sub, cfg.DurationDays)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithFPS(30))
	_, err := p.Run()
	client.Close()
	return err
}
