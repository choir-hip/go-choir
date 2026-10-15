//go:build linux

package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Drill selfdev-jobs-0101 (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md):
// a cell ran into its timeout, the client gave up at the same instant the
// broker answered "session worker eval timed out", and every later RPC on the
// shared connection read the previous call's answer. Failure modes pinned:
//   - F1 a response that arrives after the caller's deadline is read by the
//     next RPC;
//   - F2 a decode error leaves a partial frame the next RPC decodes from;
//   - F4 after an error the next RPC fails too instead of reconnecting;
//   - F5 concurrent callers interleave on the reconnected connection;
//   - F6 a healthy connection is re-dialed on every call.
// (F3, a write error, takes the same drop-and-redial path as F1 and F2.)

// fakeBroker answers each request on a connection in order, as the capsule
// broker does: {"id": x} echoes x; "slow" answers after a delay; "garble"
// writes a frame that is not JSON.
type fakeBroker struct {
	listener net.Listener
	accepted atomic.Int32
	slow     time.Duration
}

func newFakeBroker(t *testing.T, slow time.Duration) (*fakeBroker, string) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "broker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	fb := &fakeBroker{listener: listener, slow: slow}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fb.accepted.Add(1)
			go fb.serve(conn)
		}
	}()
	return fb, socket
}

func (fb *fakeBroker) serve(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	for {
		var req BrokerRPCRequest
		if err := decoder.Decode(&req); err != nil {
			return
		}
		var params struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(req.Params, &params)
		switch req.Verb {
		case "slow":
			time.Sleep(fb.slow)
		case "garble":
			// The front of a frame and the start of the next, as a reader
			// that gave up mid-message would leave them.
			if _, err := conn.Write([]byte(`{"result":"` + params.ID + `"}` + "\n" + `{"result":fal7`)); err != nil {
				return
			}
			continue
		}
		result, _ := json.Marshal(params.ID)
		if err := encoder.Encode(BrokerRPCResponse{Result: result}); err != nil {
			return
		}
	}
}

func brokerEcho(t *testing.T, ctx context.Context, client *BrokerClient, verb, id string) (string, error) {
	t.Helper()
	resp, err := client.call(ctx, verb, nil, map[string]string{"id": id})
	if err != nil {
		return "", err
	}
	var got string
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		return "", fmt.Errorf("decode result: %w", err)
	}
	return got, nil
}

func connectedBrokerClient(t *testing.T, socket string) *BrokerClient {
	t.Helper()
	client := NewBrokerClient(socket, nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestBrokerClientLateResponseIsNotReadByTheNextCall(t *testing.T) {
	fb, socket := newFakeBroker(t, 300*time.Millisecond)
	client := connectedBrokerClient(t, socket)

	// F6: a healthy connection is reused.
	for _, id := range []string{"a", "b"} {
		got, err := brokerEcho(t, context.Background(), client, "echo", id)
		if err != nil || got != id {
			t.Fatalf("echo %s = %q, %v", id, got, err)
		}
	}
	if n := fb.accepted.Load(); n != 1 {
		t.Fatalf("healthy calls dialed %d connections, want 1", n)
	}

	// F1: the slow call's answer lands after its deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err := brokerEcho(t, ctx, client, "slow", "late")
	cancel()
	if err == nil {
		t.Fatal("slow call past its deadline succeeded")
	}
	time.Sleep(400 * time.Millisecond) // the late answer is written by now

	// F4: the next call reconnects and reads its own answer.
	got, err := brokerEcho(t, context.Background(), client, "echo", "after-timeout")
	if err != nil {
		t.Fatalf("call after a timeout failed: %v", err)
	}
	if got != "after-timeout" {
		t.Fatalf("call after a timeout read %q: another call's response", got)
	}
	if n := fb.accepted.Load(); n != 2 {
		t.Fatalf("after one timeout the client dialed %d connections, want 2", n)
	}
}

func TestBrokerClientPartialFrameIsNotReadByTheNextCall(t *testing.T) {
	_, socket := newFakeBroker(t, 0)
	client := connectedBrokerClient(t, socket)

	// The garbled call reads its first frame; the bytes behind it are not a
	// response to anything the client sent next.
	if got, err := brokerEcho(t, context.Background(), client, "garble", "g"); err != nil || got != "g" {
		t.Fatalf("garble = %q, %v", got, err)
	}
	// F2: the next call must not decode the leftover partial frame.
	got, err := brokerEcho(t, context.Background(), client, "echo", "next")
	if err == nil && got != "next" {
		t.Fatalf("next call read %q: a leftover frame", got)
	}
	if err != nil {
		// A decode error is acceptable once; it must drop the connection.
		got, err = brokerEcho(t, context.Background(), client, "echo", "next-again")
		if err != nil || got != "next-again" {
			t.Fatalf("call after a decode error = %q, %v; want a reconnect", got, err)
		}
	}
}

func TestBrokerClientConcurrentCallsAfterReconnectGetTheirOwnResponses(t *testing.T) {
	_, socket := newFakeBroker(t, 200*time.Millisecond)
	client := connectedBrokerClient(t, socket)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if _, err := brokerEcho(t, ctx, client, "slow", "late"); err == nil {
		t.Fatal("slow call past its deadline succeeded")
	}
	cancel()

	// F5: a cell and the job watcher share the client.
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("call-%d", i)
			got, err := brokerEcho(t, context.Background(), client, "echo", id)
			if err != nil {
				errs <- fmt.Errorf("%s: %w", id, err)
				return
			}
			if got != id {
				errs <- fmt.Errorf("%s read %q", id, got)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	if len(all) > 0 {
		t.Fatal(errors.Join(all...))
	}
}
