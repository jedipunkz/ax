package store

import (
	"encoding/json"
	"net"
)

// subscriberSendBuf bounds how many messages can be queued for a single
// subscriber before the manager treats it as too slow and drops it. The
// buffer absorbs short bursts (e.g. snapshot followed by a flurry of updates)
// without letting one stalled client block fan-out to the others.
const subscriberSendBuf = 256

type subscriber struct {
	conn   net.Conn
	enc    *json.Encoder
	sendCh chan Message
	done   chan struct{}

	// Fields below are guarded by manager.mu.
	subscribed bool            // true while this sub is in manager.subscribers
	attached   map[string]bool // agent IDs this sub is attached to (output streaming)
	filter     *Filter         // subscribe filter; nil = receive everything
}

func newSubscriber(conn net.Conn, enc *json.Encoder) *subscriber {
	return &subscriber{
		conn:     conn,
		enc:      enc,
		sendCh:   make(chan Message, subscriberSendBuf),
		done:     make(chan struct{}),
		attached: make(map[string]bool),
	}
}

// run drains sendCh and writes to the connection. Returns when sendCh is
// closed (orderly shutdown) or a write fails (the connection's reader loop
// will see EOF shortly and clean up the subscriber).
func (s *subscriber) run() {
	for msg := range s.sendCh {
		if err := s.enc.Encode(msg); err != nil {
			_ = s.conn.Close()
			return
		}
	}
}

// trySend delivers a message non-blockingly. Returns false when the buffer
// is full, signaling that the caller should drop the subscriber.
func (s *subscriber) trySend(msg Message) bool {
	select {
	case <-s.done:
		return false
	case s.sendCh <- msg:
		return true
	default:
		return false
	}
}

// close is idempotent: closes sendCh exactly once so run() exits cleanly.
func (s *subscriber) close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
		close(s.sendCh)
	}
}
