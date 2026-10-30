package hostlink

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// docs/archive/problems/host-deploy-restart-fails-in-flight-guest-turns-2026-10-10.md:
// a deploy stopped the host services for 1 min 48 s and every guest call in
// flight failed "connection refused". Failure modes pinned: a call sent
// while the host is down fails instead of waiting; the resent POST arrives
// without its body; a cancelled context keeps resending; a call that
// reached the host and failed is resent; the per-attempt timeout stops
// covering the body read.
func TestTransportWaitsOutAHostRestart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	var hits atomic.Int32
	var body atomic.Value
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))
		if r.URL.Path == "/fail" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		restarted, listenErr := net.Listen("tcp", addr)
		if listenErr != nil {
			t.Errorf("relisten: %v", listenErr)
			return
		}
		_ = server.Serve(restarted)
	}()
	defer server.Close()

	client := NewClient(5 * time.Second)
	resp, err := client.Post("http://"+addr+"/write", "text/plain", bytes.NewReader([]byte("event-1")))
	if err != nil {
		t.Fatalf("POST across the restart: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(got) != "ok" || body.Load() != "event-1" {
		t.Fatalf("response %q, body at host %q", got, body.Load())
	}

	before := hits.Load()
	resp, err = client.Post("http://"+addr+"/fail", "text/plain", bytes.NewReader([]byte("x")))
	if err != nil || resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("failing call: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	if hits.Load()-before != 1 {
		t.Fatalf("a call that reached the host was resent (%d hits)", hits.Load()-before)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/", nil)
	start := time.Now()
	if _, err := client.Do(req); err == nil {
		t.Fatal("a cancelled call to a dead host succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelled call kept resending for %s", elapsed)
	}
}

func TestTransportAttemptTimeoutCoversTheBody(t *testing.T) {
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte("late"))
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	resp, err := NewClient(300 * time.Millisecond).Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatalf("headers within the timeout: %v", err)
	}
	defer resp.Body.Close()
	start := time.Now()
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("a body slower than the attempt timeout was read in full")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("body read ran %s past a 300ms attempt timeout", elapsed)
	}
}
