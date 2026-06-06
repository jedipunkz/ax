package store

import (
	"sync"
	"testing"
)

// TestSubscriberCloseIsIdempotentUnderConcurrency exercises the panic
// hazard called out in PR review: two goroutines calling close at the
// same time previously raced on close(done) / close(sendCh) and could
// panic. With sync.Once + done-only close, this must be safe.
func TestSubscriberCloseIsIdempotentUnderConcurrency(t *testing.T) {
	sub := newSubscriber(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub.close()
		}()
	}
	wg.Wait()

	select {
	case <-sub.done:
	default:
		t.Fatal("done channel should be closed after close()")
	}

	// trySend after close must report failure without panic.
	if sub.trySend(Message{Type: "ping"}) {
		t.Fatal("trySend after close must return false")
	}
}

// TestSubscriberTrySendDoesNotPanicWhenClosedConcurrently models the
// fan-out path that previously could panic when one goroutine closed
// sendCh while another was still pushing to it.
func TestSubscriberTrySendDoesNotPanicWhenClosedConcurrently(t *testing.T) {
	sub := newSubscriber(nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = sub.trySend(Message{Type: "x"})
		}
	}()

	go func() {
		defer wg.Done()
		sub.close()
	}()

	wg.Wait()

	if sub.trySend(Message{Type: "after"}) {
		t.Fatal("trySend after close must return false")
	}
}
