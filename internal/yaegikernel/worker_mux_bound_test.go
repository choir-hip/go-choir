package yaegikernel

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// The worker's wait for a broker result
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, class c
// row "yaegikernel/worker_mux.go"). The worker evaluates a cell under
// context.Background, so its wait has no deadline of its own. Its bound is
// the host: the host's eval ends at its context (the activation budget, or
// the cell's bound), kills the worker, and the socket closes. This pins
// that a waiting call returns when the socket closes, which is the bound.
func TestWorkerBrokerCallReturnsWhenTheHostClosesTheSocket(t *testing.T) {
	host, worker := net.Pipe()
	defer host.Close()
	mux := newWorkerMux(NewFramedConn(worker), newSearchStore())
	mux.beginCell()
	hostFC := NewFramedConn(host)
	go func() {
		// The host reads the request and never answers; then it kills the
		// worker, which closes the socket.
		_, _, _ = hostFC.ReadFrame()
		time.Sleep(50 * time.Millisecond)
		_ = host.Close()
	}()
	done := make(chan error, 1)
	go func() {
		_, err := mux.roundTrip(context.Background(), StreamBrokerEgress, StreamBrokerEgressResult, BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r-1"})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "session socket closed") {
			t.Fatalf("err = %v, want the closed socket named", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a broker call outlived the socket's close")
	}
}
