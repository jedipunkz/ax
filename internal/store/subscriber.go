package store

import (
	"encoding/json"
	"net"
	"sync"
)

// subscriberSendBuf bounds how many messages can be queued for a single
// subscriber before the manager treats it as too slow and drops it. The
// buffer absorbs short bursts (e.g. snapshot followed by a flurry of updates)
// without letting one stalled client block fan-out to the others.
const subscriberSendBuf = 256

type subscriber struct {
	conn      net.Conn
	enc       *json.Encoder
	sendCh    chan Message
	done      chan struct{}
	closeOnce sync.Once

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

// run drains sendCh and writes to the connection. Exits when done is
// closed (orderly shutdown) or a write fails. sendCh is never closed,
// so concurrent trySend calls cannot panic on a closed channel.
func (s *subscriber) run() {
	for {
		select {
		case <-s.done:
			return
		case msg := <-s.sendCh:
			if err := s.enc.Encode(msg); err != nil {
				_ = s.conn.Close()
				return
			}
		}
	}
}

// trySend delivers a message non-blockingly. Returns false when the
// subscriber has been closed or the buffer is full, signaling that the
// caller should drop the subscriber.
//
// The two-stage select guarantees that a trySend issued after close
// observes done and returns false: a single combined select could pick
// the sendCh case at random when both cases are simultaneously ready.
// A trySend racing concurrently with close may still succeed in
// enqueuing, but run() will exit on done and drop it; sendCh is never
// closed so there is no send-on-closed-channel panic hazard.
func (s *subscriber) trySend(msg Message) bool {
	select {
	case <-s.done:
		return false
	default:
	}
	select {
	case s.sendCh <- msg:
		return true
	default:
		return false
	}
}

// close is idempotent and safe to call from multiple goroutines. It
// only closes the done channel — sendCh is left open so an in-flight
// trySend can finish without risking a send-on-closed-channel panic.
// run() observes done and exits, leaving any queued messages to be GCed
// together with the subscriber.
func (s *subscriber) close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}
