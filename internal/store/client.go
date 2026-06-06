package store

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// Client is a connection to the state manager over a Unix socket. Writes are
// serialised internally so callers may safely invoke Send* methods from
// multiple goroutines (e.g. the runner sends state updates from several
// monitoring tickers concurrently).
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	encMu   sync.Mutex
	scanner *bufio.Scanner
}

// Connect dials the Unix socket at socketPath.
func (c *Client) Connect(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not connect to socket: %w", err)
	}
	c.conn = conn
	c.encoder = json.NewEncoder(conn)
	c.scanner = bufio.NewScanner(conn)
	c.scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // allow up to 4 MB messages
	return nil
}

// send serialises encoder access so concurrent callers don't interleave
// partial JSON-Lines payloads on the socket.
func (c *Client) send(msg Message) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()
	return c.encoder.Encode(msg)
}

// SendUpdate sends an agent state update to the manager.
func (c *Client) SendUpdate(agent AgentState) error {
	return c.send(Message{Type: "update", Agent: &agent})
}

// SendRemove sends a remove request for the agent with the given ID.
func (c *Client) SendRemove(agentID string) error {
	return c.send(Message{Type: "remove", AgentID: agentID})
}

// Subscribe sends a subscribe message so the client receives snapshots and updates.
func (c *Client) Subscribe() error {
	return c.send(Message{Type: "subscribe"})
}

// SubscribeWithFilter is like Subscribe but instructs the daemon to deliver
// only events matching the filter (nil = same as Subscribe).
func (c *Client) SubscribeWithFilter(f *Filter) error {
	return c.send(Message{Type: "subscribe", Filter: f})
}

// Attach asks the daemon to stream the agent's PTY output. When tail > 0, the
// daemon first replays the last tail bytes of the log file.
func (c *Client) Attach(agentID string, tail int) error {
	return c.send(Message{Type: "attach", AgentID: agentID, Tail: tail})
}

// Detach asks the daemon to stop streaming for the given agent.
func (c *Client) Detach(agentID string) error {
	return c.send(Message{Type: "detach", AgentID: agentID})
}

// Metrics asks the daemon to compute a point-in-time metrics summary for the
// given agent. The response is a "metrics_result" or "metrics_err" message.
func (c *Client) Metrics(agentID string) error {
	return c.send(Message{Type: "metrics", AgentID: agentID})
}

// RegisterInput announces that this connection owns the input PTY for the
// given agent. The daemon routes incoming "input" payloads for that agent
// back over this connection.
func (c *Client) RegisterInput(agentID string) error {
	return c.send(Message{Type: "register_input", AgentID: agentID})
}

// SendInput delivers stdin data to the agent identified by agentID. The
// daemon forwards it to the registered runner only when the agent is in a
// waiting_user state; otherwise it responds with an input_err.
func (c *Client) SendInput(agentID string, data []byte) error {
	return c.send(Message{
		Type:    "input",
		AgentID: agentID,
		Data:    base64.StdEncoding.EncodeToString(data),
	})
}

// ReadMessage reads the next JSON-lines message from the socket.
func (c *Client) ReadMessage() (Message, error) {
	if !c.scanner.Scan() {
		err := c.scanner.Err()
		if err == nil {
			err = fmt.Errorf("connection closed")
		}
		return Message{}, err
	}
	var msg Message
	if err := json.Unmarshal(c.scanner.Bytes(), &msg); err != nil {
		return Message{}, fmt.Errorf("could not parse message: %w", err)
	}
	return msg, nil
}

// Close closes the underlying connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
