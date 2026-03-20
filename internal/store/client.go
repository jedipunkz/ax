package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

// Client is a connection to the state manager over a Unix socket.
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
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
	return nil
}

// SendUpdate sends an agent state update to the manager.
func (c *Client) SendUpdate(agent AgentState) error {
	msg := Message{Type: "update", Agent: &agent}
	return c.encoder.Encode(msg)
}

// Subscribe sends a subscribe message so the client receives snapshots and updates.
func (c *Client) Subscribe() error {
	msg := Message{Type: "subscribe"}
	return c.encoder.Encode(msg)
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
