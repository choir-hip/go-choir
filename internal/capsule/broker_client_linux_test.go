//go:build linux

package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	return serveFakeBroker(t, listener, slow), socket
}

func serveFakeBroker(t *testing.T, listener net.Listener, slow time.Duration) *fakeBroker {
	t.Helper()
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
	return fb
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

// Drill selfdev-jobs-0654 (docs/archive/problems/selfdev-jobs-0654-raced-activation-verified-engineering-budget-2026-10-15.md):
// capsule state sits past sun_path, so the client dialed
// /proc/self/fd/<N>/broker.sock through a directory descriptor that startBrokerLocked
// closed; the first re-dial after a cell timeout failed with ENOTDIR for the
// rest of the capsule's life. Failure modes pinned:
//   - R1 a deep socket path cannot be re-dialed after a dropped connection;
//   - R2 the re-dial leaks a descriptor per dial;
//   - R3 a failed dial names an fd path instead of the real socket path;
//   - R4 the re-dial depends on a particular descriptor number staying open.
func deepBrokerSocket(t *testing.T) string {
	t.Helper()
	deep := filepath.Join(t.TempDir(), strings.Repeat("d", 60), "boot-84c9b456-9074-44cf-a582-5f0b7cbd59b7",
		"capsule-7492efb9-70fa-5ca7-8819-2b0386d5e51b", "root", "run", "capsule")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(deep, "broker.sock")
	if len(full) <= maxUnixSocketPath {
		t.Fatalf("test path is only %d bytes", len(full))
	}
	if conn, err := net.Dial("unix", full); err == nil {
		_ = conn.Close()
		t.Fatal("premise: a plain dial of the deep path succeeded")
	} else if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("premise: dial failed with %v, not a path-length error", err)
	}
	return full
}

func openFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestBrokerClientRedialsADeepSocketPathAfterATimeout(t *testing.T) {
	full := deepBrokerSocket(t)
	listener, err := listenUnixAt(full)
	if err != nil {
		t.Fatalf("listenUnixAt: %v", err)
	}
	fb := serveFakeBroker(t, listener, 300*time.Millisecond)

	// Premise (the bug): an address taken through a directory descriptor that
	// is then closed cannot be dialed.
	staleAddr, releaseStale, err := unixSocketAddress(full)
	if err != nil {
		t.Fatal(err)
	}
	releaseStale()
	if err := NewBrokerClient(staleAddr, nil).Connect(context.Background()); err == nil {
		t.Fatalf("premise: dialing %q after its descriptor closed succeeded", staleAddr)
	}

	// The way startBrokerLocked builds the client: the real path, connected
	// once. Before the fix the stored address was a descriptor path whose
	// descriptor the caller had already closed.
	client := NewBrokerClient(full, nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if got, err := brokerEcho(t, context.Background(), client, "echo", "first"); err != nil || got != "first" {
		t.Fatalf("first call = %q, %v", got, err)
	}

	// R4: churn the low descriptor numbers, as a long-lived process does, so a
	// path naming one particular descriptor would now name something else.
	var churn []*os.File
	for i := 0; i < 8; i++ {
		f, err := os.Open("/")
		if err != nil {
			t.Fatal(err)
		}
		churn = append(churn, f)
	}
	t.Cleanup(func() {
		for _, f := range churn {
			_ = f.Close()
		}
	})

	before := openFDCount(t)
	for round := 0; round < 3; round++ {
		// R1: a cell times out, dropping the connection.
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		_, err := brokerEcho(t, ctx, client, "slow", "late")
		cancel()
		if err == nil {
			t.Fatal("slow call past its deadline succeeded")
		}
		time.Sleep(350 * time.Millisecond)
		id := fmt.Sprintf("after-timeout-%d", round)
		got, err := brokerEcho(t, context.Background(), client, "echo", id)
		if err != nil {
			t.Fatalf("round %d: call after a timeout on a deep path failed: %v", round, err)
		}
		if got != id {
			t.Fatalf("round %d: read %q, want %q", round, got, id)
		}
	}
	if n := fb.accepted.Load(); n != 4 {
		t.Fatalf("broker accepted %d connections, want 1 + 3 re-dials", n)
	}
	// R2: three re-dials leave the descriptor table as they found it (the
	// live connection replaced the dropped one).
	if after := openFDCount(t); after > before {
		t.Fatalf("re-dials leaked descriptors: %d open before, %d after", before, after)
	}
}

func TestBrokerClientDialErrorNamesTheRealPathNotAnFDPath(t *testing.T) {
	full := deepBrokerSocket(t) // the directory exists; no socket is listening
	client := NewBrokerClient(full, nil)
	err := client.Connect(context.Background())
	if err == nil {
		t.Fatal("connect to a missing socket succeeded")
	}
	if strings.Contains(err.Error(), "/proc/self/fd") {
		t.Fatalf("R3: error names a descriptor path: %v", err)
	}
	if !strings.Contains(err.Error(), full) {
		t.Fatalf("R3: error does not name the real socket path: %v", err)
	}
}
