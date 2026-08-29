package store

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnauthenticatedSocketClientCannotInjectArbitraryLogPath(t *testing.T) {
	// Use a short prefix in the system temp dir instead of t.TempDir(), whose
	// generated path embeds the long test name and can exceed the platform
	// Unix-socket path length limit (~104 bytes on macOS).
	dir, err := os.MkdirTemp("", "agx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "agx.sock")
	statePath := filepath.Join(dir, "state.json")
	victimPath := filepath.Join(dir, "victim.txt")
	victimContent := "non-secret proof content\n"
	if err := os.WriteFile(victimPath, []byte(victimContent), 0600); err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunManager(socketPath, statePath)
	}()
	waitForSocket(t, socketPath, errCh)
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("socket permissions = %o, want 0600", got)
	}

	now := time.Now()
	sendRawMessage(t, socketPath, Message{
		Type: "update",
		Agent: &AgentState{
			ID:         "attacker-controlled",
			PID:        os.Getpid(),
			WorkDir:    dir,
			Status:     StatusFailed,
			StartedAt:  now,
			FinishedAt: &now,
			LogFile:    victimPath,
		},
	})
	assertNoAgentState(t, statePath, "attacker-controlled")

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(conn).Encode(Message{
		Type:    "attach",
		AgentID: "attacker-controlled",
		Tail:    1024,
	}); err != nil {
		t.Fatal(err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("expected attach_err response")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	var msg Message
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "attach_err" {
		t.Fatalf("response type = %q, want attach_err", msg.Type)
	}
}

func waitForSocket(t *testing.T, socketPath string, errCh <-chan error) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			t.Fatalf("manager exited early: %v", err)
		default:
		}
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket was not ready: %s", socketPath)
}

func assertNoAgentState(t *testing.T, statePath, agentID string) {
	t.Helper()
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		agents, err := ReadAgents(statePath)
		if err == nil {
			for _, agent := range agents {
				if agent.ID == agentID {
					t.Fatalf("agent state was persisted unexpectedly: %s", agentID)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func sendRawMessage(t *testing.T, socketPath string, msg Message) {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		t.Fatal(err)
	}
}
