package provider

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Stream stall watchdog (docs/problems/model-stream-hangs-ten-minutes-2026-11-04.md).
//
// A model stream that sends no bytes holds the whole gateway call open until
// the 10 minute inference deadline. The reader below aborts a stream that has
// been silent for a window and surfaces "stream stalled: no bytes for Ns" as
// the read error, so the gateway answers 502 and the guest's existing
// transient-failure retry (gatewayruntime call, 3 attempts) re-issues it.
//
// Windows, from the gateway journal of 2026-11-02..04 (chatgpt xhigh, 388
// calls; call-to-next-call gaps bound time-to-first-byte from above): p50 7 s,
// p90 44 s, p95 91 s. A reasoning model may think silently before the first
// token, so the first-byte window is 180 s. Once bytes flow, the window is 75 s.
// Any byte counts, SSE comments and pings included.
var (
	streamFirstByteWindow = 180 * time.Second
	streamIdleWindow      = 75 * time.Second
)

// errStreamStalled is the sentinel under every stall error.
var errStreamStalled = errors.New("stream stalled")

type stallReader struct {
	src   io.Reader
	close func() error
	first time.Duration
	idle  time.Duration

	mu      sync.Mutex
	timer   *time.Timer
	started time.Time
	gotByte bool
	stalled error
	ttfb    time.Duration
}

// withStallWatchdog wraps body so a silent stream is aborted. A body that
// cannot be closed cannot be unblocked and is returned unwrapped.
func withStallWatchdog(body io.Reader) io.Reader {
	c, ok := body.(io.Closer)
	if !ok {
		return body
	}
	s := &stallReader{src: body, close: c.Close, first: streamFirstByteWindow, idle: streamIdleWindow, started: time.Now()}
	s.timer = time.AfterFunc(s.first, s.fire)
	return s
}

func (s *stallReader) fire() {
	s.mu.Lock()
	window := s.idle
	if !s.gotByte {
		window = s.first
	}
	s.stalled = fmt.Errorf("%w: no bytes for %ds", errStreamStalled, int(window.Seconds()))
	s.mu.Unlock()
	_ = s.close() // unblocks the pending Read
}

func (s *stallReader) Read(p []byte) (int, error) {
	n, err := s.src.Read(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stalled != nil {
		return n, s.stalled
	}
	if err != nil {
		s.timer.Stop()
		return n, err
	}
	if n > 0 {
		if !s.gotByte {
			s.gotByte = true
			s.ttfb = time.Since(s.started)
		}
		s.timer.Reset(s.idle)
	}
	return n, err
}

// firstByte reports the time to the first byte, for the stream log lines.
func firstByteOf(r io.Reader) time.Duration {
	if s, ok := r.(*stallReader); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.ttfb
	}
	return 0
}

func stopStallWatchdog(r io.Reader) {
	if s, ok := r.(*stallReader); ok {
		s.timer.Stop()
	}
}
