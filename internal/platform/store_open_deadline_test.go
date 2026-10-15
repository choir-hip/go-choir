package platform

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"
)

// hungMySQLListener accepts TCP connections and never answers: the shape of
// the 2026-10-15 platform Dolt deadlock (accepting, not responding). It
// returns a DSN that points at it.
func hungMySQLListener(t *testing.T) string {
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
	return "root@tcp(" + ln.Addr().String() + ")/platform?parseTime=true&multiStatements=true"
}

func TestOpenStoreIsBoundedByContextWhenServerNeverAnswers(t *testing.T) {
	dsn := hungMySQLListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	store, err := OpenStore(ctx, dsn, "")
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStore against a server that never answers must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("OpenStore took %s against a hung server; the context deadline did not bound it", elapsed)
	}
}

func TestOpenStoreWithoutBootstrapIsBoundedByContext(t *testing.T) {
	dsn := hungMySQLListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	store, err := OpenStoreWithoutBootstrap(ctx, dsn, "")
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStoreWithoutBootstrap against a server that never answers must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("OpenStoreWithoutBootstrap took %s against a hung server", elapsed)
	}
}

func TestBootstrapHonoursContextDeadline(t *testing.T) {
	dsn := hungMySQLListener(t)
	// Open without any handshake: sql.Open is lazy, so Bootstrap is the first
	// operation to touch the hung server.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := store.Bootstrap(ctx); err == nil {
		t.Fatal("Bootstrap against a server that never answers must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Bootstrap took %s against a hung server", elapsed)
	}
}
