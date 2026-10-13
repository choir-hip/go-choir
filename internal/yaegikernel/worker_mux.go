package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// errCallAfterCell is what a host-mediated verb returns when a goroutine
// calls it after its cell has returned. The host answers broker frames only
// while a cell runs, so such a call would otherwise wait for the next cell
// and be served under that cell's identity.
var errCallAfterCell = errors.New("this call came after its cell returned; host calls run only while a cell runs")

// workerMux owns the worker end of the session socket. One reader loop
// reads every frame: broker results go to their waiter by request id, and
// cell frames go to the session loop in order. Before it, each verb call read
// the next frame itself, so parallel calls in one cell got each other's
// answers (problems/parallel-choir-calls-cross-wire-results-2026-10-12.md).
type workerMux struct {
	fc     *FramedConn
	frames chan muxFrame

	mu      sync.Mutex
	waiters map[string]muxWaiter
	readErr error

	// cellMu orders calls against the cell boundary: a call holds the read
	// lock from its in-cell check through its request write, and the cell
	// end takes the write lock, so every request a cell sends is on the wire
	// before its result frame.
	cellMu sync.RWMutex
	inCell bool
}

type muxFrame struct {
	stream  byte
	payload []byte
	err     error
}

type muxWaiter struct {
	stream byte
	ch     chan []byte
}

func newWorkerMux(fc *FramedConn) *workerMux {
	m := &workerMux{fc: fc, frames: make(chan muxFrame, 8), waiters: map[string]muxWaiter{}}
	go m.readLoop()
	return m
}

func (m *workerMux) readLoop() {
	for {
		stream, payload, err := m.fc.ReadFrame()
		if err != nil {
			m.mu.Lock()
			m.readErr = err
			waiters := m.waiters
			m.waiters = map[string]muxWaiter{}
			m.mu.Unlock()
			for _, w := range waiters {
				close(w.ch)
			}
			m.frames <- muxFrame{err: err}
			return
		}
		switch stream {
		case StreamCell, StreamCancel:
			m.frames <- muxFrame{stream: stream, payload: payload}
		case StreamBrokerEgressResult, StreamBrokerEmitResult:
			var head struct {
				RequestID string `json:"request_id"`
			}
			if json.Unmarshal(payload, &head) != nil {
				continue
			}
			m.mu.Lock()
			w, ok := m.waiters[head.RequestID]
			if ok && w.stream == stream {
				delete(m.waiters, head.RequestID)
			}
			m.mu.Unlock()
			if ok && w.stream == stream {
				w.ch <- payload
			}
		}
	}
}

// next returns the next cell or cancel frame for the session loop.
func (m *workerMux) next() (byte, []byte, error) {
	f := <-m.frames
	return f.stream, f.payload, f.err
}

func (m *workerMux) beginCell() {
	m.cellMu.Lock()
	m.inCell = true
	m.cellMu.Unlock()
}

func (m *workerMux) endCell() {
	m.cellMu.Lock()
	m.inCell = false
	m.cellMu.Unlock()
}

// roundTrip sends one broker request and waits for the result carrying the
// same request id on respStream.
func (m *workerMux) roundTrip(ctx context.Context, reqStream, respStream byte, req BrokerRequest) (BrokerResponse, error) {
	enc, err := json.Marshal(req)
	if err != nil {
		return BrokerResponse{}, fmt.Errorf("request marshal: %w", err)
	}
	ch := make(chan []byte, 1)
	m.cellMu.RLock()
	if !m.inCell {
		m.cellMu.RUnlock()
		return BrokerResponse{}, errCallAfterCell
	}
	m.mu.Lock()
	if m.readErr != nil {
		err := m.readErr
		m.mu.Unlock()
		m.cellMu.RUnlock()
		return BrokerResponse{}, fmt.Errorf("session socket closed: %w", err)
	}
	m.waiters[req.RequestID] = muxWaiter{stream: respStream, ch: ch}
	m.mu.Unlock()
	err = m.fc.WriteFrame(reqStream, enc)
	m.cellMu.RUnlock()
	if err != nil {
		m.drop(req.RequestID)
		return BrokerResponse{}, fmt.Errorf("write frame: %w", err)
	}
	select {
	case raw, ok := <-ch:
		if !ok {
			return BrokerResponse{}, fmt.Errorf("session socket closed before the result arrived")
		}
		var resp BrokerResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return BrokerResponse{}, fmt.Errorf("decode result: %w", err)
		}
		return resp, nil
	case <-ctx.Done():
		m.drop(req.RequestID)
		return BrokerResponse{}, ctx.Err()
	}
}

func (m *workerMux) drop(requestID string) {
	m.mu.Lock()
	delete(m.waiters, requestID)
	m.mu.Unlock()
}
