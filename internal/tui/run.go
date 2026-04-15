package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jedipunkz/ax/internal/agent"
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

	// Start background goroutine to periodically remove old worktrees.
	go runWorktreeCleanup(cfg.RemoveDurationDays)

	m := newModel(client, sub, cfg.DurationDays)
	p := tea.NewProgram(m, tea.WithFPS(30))
	_, err := p.Run()
	client.Close()
	return err
}

// runWorktreeCleanup runs worktree cleanup immediately and then every 24 hours.
func runWorktreeCleanup(removeDurationDays int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	statePath := filepath.Join(home, ".ax", "state.json")
	worktreesDir := filepath.Join(home, ".ax", "worktrees")

	doCleanup := func() {
		if err := agent.CleanupOldWorktrees(statePath, worktreesDir, removeDurationDays); err != nil {
			fmt.Fprintf(os.Stderr, "warning: worktree cleanup error: %v\n", err)
		}
	}

	doCleanup()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		doCleanup()
	}
}
