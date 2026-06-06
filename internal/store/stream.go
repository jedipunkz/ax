package store

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/fsnotify/fsnotify"
)

// outputStream tails an agent's log file and fans new bytes out to every
// subscriber attached to that agent. It owns the file handle and watcher;
// callers interact with it only via the manager.
type outputStream struct {
	agentID string
	logPath string
	file    *os.File
	watcher *fsnotify.Watcher

	// stopCh is closed by the manager when the stream should shut down
	// without sending eof (e.g. last attacher detached).
	stopCh chan struct{}
	// eofCh is closed by the manager when the agent has reached a terminal
	// state; the run loop performs a final flush, emits "eof", then exits.
	eofCh chan struct{}
}

// streamReadChunk is the buffer size used per read from the log file.
const streamReadChunk = 32 * 1024

// startStream opens the log file at the end (so attachers see only new bytes)
// and begins watching it. Callers that want history should send a "tail"
// payload via emitTail before relying on the watcher.
func startStream(agentID, logPath string) (*outputStream, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("seek log end: %w", err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("watcher: %w", err)
	}
	if err := w.Add(logPath); err != nil {
		_ = f.Close()
		_ = w.Close()
		return nil, fmt.Errorf("watch log: %w", err)
	}
	return &outputStream{
		agentID: agentID,
		logPath: logPath,
		file:    f,
		watcher: w,
		stopCh:  make(chan struct{}),
		eofCh:   make(chan struct{}),
	}, nil
}

// emitTail reads the last n bytes of the log and sends them as one "output"
// message to dst. Called once at attach time before the stream takes over.
func emitTail(logPath string, n int, dst *subscriber, agentID string) {
	if n <= 0 {
		return
	}
	f, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return
	}
	size := info.Size()
	start := int64(0)
	if size > int64(n) {
		start = size - int64(n)
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return
	}
	buf := make([]byte, size-start)
	read, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return
	}
	if read == 0 {
		return
	}
	dst.trySend(Message{
		Type:    "output",
		AgentID: agentID,
		Data:    base64.StdEncoding.EncodeToString(buf[:read]),
	})
}

// run drains new bytes and fans them out via the manager. It exits on stop
// (no eof) or on eofCh (final flush + eof). Background events on the watcher
// that signal file removal/rename also terminate the stream with eof.
func (s *outputStream) run(m *manager) {
	defer func() {
		_ = s.watcher.Close()
		_ = s.file.Close()
	}()

	flush := func() {
		buf := make([]byte, streamReadChunk)
		for {
			n, err := s.file.Read(buf)
			if n > 0 {
				m.fanOutOutput(s.agentID, buf[:n])
			}
			if err != nil {
				// EOF is expected — we'll wait for the next event.
				return
			}
		}
	}

	for {
		select {
		case <-s.stopCh:
			return

		case <-s.eofCh:
			flush()
			m.fanOutEOF(s.agentID)
			return

		case ev, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Write != 0 {
				flush()
			}
			if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				flush()
				m.fanOutEOF(s.agentID)
				return
			}

		case _, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			// Best-effort: keep going on transient watcher errors.
		}
	}
}
