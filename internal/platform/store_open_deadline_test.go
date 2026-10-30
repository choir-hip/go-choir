package platform

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// hungPGListener accepts TCP connections and never answers: the shape of
// the 2026-10-15 platform database deadlock (accepting, not responding). It
// returns a DSN that points at it.
func hungPGListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return fmt.Sprintf("host=127.0.0.1 port=%d dbname=control user=choir sslmode=disable", addr.Port)
}

func TestOpenStoreIsBoundedByContextWhenServerNeverAnswers(t *testing.T) {
	dsn := hungPGListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	store, err := OpenStore(ctx, dsn, dsn)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStore against a server that never answers must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("OpenStore took %s against a hung server; the context deadline did not bound it", elapsed)
	}
}

func TestOpenStoreWithoutBootstrapIsBoundedByContext(t *testing.T) {
	dsn := hungPGListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	store, err := OpenStoreWithoutBootstrap(ctx, dsn, dsn)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStoreWithoutBootstrap against a server that never answers must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("OpenStoreWithoutBootstrap took %s against a hung server", elapsed)
	}
}
