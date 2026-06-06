package agent

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestExitOnControlCByte(t *testing.T) {
	var got int
	done := make(chan struct{})

	exitOnControlCByte(bytes.NewReader([]byte("log output\x03more")), func() {
		got++
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("control-c byte did not trigger interrupt")
	}
	if got != 1 {
		t.Fatalf("interrupt calls = %d, want 1", got)
	}
}

func TestExitOnControlCByteHandlesKittyKeyboardProtocol(t *testing.T) {
	var got int
	done := make(chan struct{})

	exitOnControlCByte(bytes.NewReader([]byte("log output\x1b[99;5:1u")), func() {
		got++
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("kitty ctrl-c sequence did not trigger interrupt")
	}
	if got != 1 {
		t.Fatalf("interrupt calls = %d, want 1", got)
	}
}

func TestExitOnControlCByteHandlesKittyKeyboardProtocolWithoutEsc(t *testing.T) {
	var got int
	done := make(chan struct{})

	exitOnControlCByte(bytes.NewReader([]byte("log output[99;5:1u")), func() {
		got++
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("kitty ctrl-c sequence without esc did not trigger interrupt")
	}
	if got != 1 {
		t.Fatalf("interrupt calls = %d, want 1", got)
	}
}

func TestExitOnControlCByteHandlesSplitKittyKeyboardProtocol(t *testing.T) {
	var got int

	exitOnControlCByte(&chunkReader{chunks: [][]byte{
		[]byte("log output\x1b[99"),
		[]byte(";5:1u"),
	}}, func() {
		got++
	})

	if got != 1 {
		t.Fatalf("interrupt calls = %d, want 1", got)
	}
}

func TestExitOnControlCByteIgnoresRegularInput(t *testing.T) {
	called := false

	exitOnControlCByte(bytes.NewReader([]byte("regular input")), func() {
		called = true
	})

	if called {
		t.Fatal("regular input unexpectedly triggered interrupt")
	}
}

func TestExitOnControlCByteStopsAfterInterrupt(t *testing.T) {
	var got int

	exitOnControlCByte(bytes.NewReader([]byte{0x03, 0x03}), func() {
		got++
	})

	if got != 1 {
		t.Fatalf("interrupt calls = %d, want 1", got)
	}
}

func TestExitOnControlCByteStopsOnReadError(t *testing.T) {
	called := false

	exitOnControlCByte(errReader{}, func() {
		called = true
	})

	if called {
		t.Fatal("read error unexpectedly triggered interrupt")
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

type chunkReader struct {
	chunks [][]byte
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	return copy(p, chunk), nil
}
