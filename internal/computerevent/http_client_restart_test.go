package computerevent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// docs/archive/problems/host-deploy-restart-fails-in-flight-guest-turns-2026-10-10.md:
// a deploy restarted corpusd for about two seconds and every guest write in
// flight failed "connection refused", including the one recording the run's
// own failure. Failure modes pinned: a request sent while corpusd is down
// fails instead of waiting for it; the body is lost on the retry (a POST
// arrives empty); a cancelled context keeps retrying; a server that answers
// with an error is retried (only refused connections never reached it).
func TestHTTPClientWaitsOutACorpusdRestart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	var posted atomic.Value
	var hits atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			posted.Store(body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
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

	client, err := NewGuestHTTPClient("http://"+addr, func(context.Context) (string, error) { return "t", nil })
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	status, err := client.do(context.Background(), http.MethodPost, "/internal/computers/events/cas", map[string]string{"event": "e1"}, &out)
	if err != nil || status != http.StatusOK {
		t.Fatalf("request across the restart: status=%d err=%v", status, err)
	}
	if got, _ := posted.Load().(map[string]any); got["event"] != "e1" {
		t.Fatalf("retried POST body = %v", got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	dead, err := NewGuestHTTPClient("http://127.0.0.1:1", func(context.Context) (string, error) { return "t", nil })
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	start := time.Now()
	if _, err := dead.do(cancelled, http.MethodGet, "/internal/computers/events/head", nil, &out); err == nil {
		t.Fatal("a cancelled request to a dead host succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancelled request kept retrying for %s", elapsed)
	}
}
