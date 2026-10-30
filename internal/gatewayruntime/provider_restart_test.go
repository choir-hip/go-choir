package gatewayruntime

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// docs/archive/problems/host-deploy-restart-fails-in-flight-guest-turns-2026-10-10.md
// (second instance): a deploy restarted the host gateway and a busy
// computer's model call failed "connection refused", failing its run.
// Failure modes pinned: a call sent while the gateway is down fails instead
// of waiting for it; the retried request arrives without its body; a
// cancelled context keeps retrying.
func TestGatewayCallWaitsOutAGatewayRestart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	var gotModel atomic.Value
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel.Store(body["model"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
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

	p := New("http://"+addr, "token")
	resp, err := p.do(context.Background(), llmRequest{Model: "m-restart"}, "")
	if err != nil {
		t.Fatalf("call across the gateway restart: %v", err)
	}
	_ = resp.Body.Close()
	if got, _ := gotModel.Load().(string); got != "m-restart" {
		t.Fatalf("retried request body model = %q", got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	start := time.Now()
	if _, err := New("http://127.0.0.1:1", "token").do(cancelled, llmRequest{Model: "m"}, ""); err == nil {
		t.Fatal("a cancelled call to a dead gateway succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelled call kept retrying for %s", elapsed)
	}
}
