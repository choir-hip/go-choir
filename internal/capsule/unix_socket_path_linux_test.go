//go:build linux

package capsule

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Staging 2026-10-10 (capsule-socket-path-too-long-on-data-disk-2026-10-10.md):
// with capsule state on the data disk the broker socket sat 145 bytes deep
// and every spawn failed with "bind: invalid argument". Failure modes
// pinned: a deep socket cannot be bound; it binds somewhere other than the
// requested directory; the host cannot dial it; closing the listener
// unlinks a file through a stale descriptor path.
func TestUnixSocketsBindAndDialPastSunPathLimit(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("d", 60), "boot-84c9b456-9074-44cf-a582-5f0b7cbd59b7",
		"capsule-7492efb9-70fa-5ca7-8819-2b0386d5e51b", "root", "run", "capsule")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(deep, "broker.sock")
	if len(full) <= maxUnixSocketPath {
		t.Fatalf("test path is only %d bytes", len(full))
	}
	if l, err := net.ListenUnix("unix", &net.UnixAddr{Name: full, Net: "unix"}); err == nil {
		_ = l.Close()
		t.Fatal("premise: a plain bind of a deep path succeeded")
	}
	listener, err := listenUnixAt(full)
	if err != nil {
		t.Fatalf("listenUnixAt: %v", err)
	}
	if info, err := os.Stat(full); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket not at the requested path: %v %v", info, err)
	}
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_, _ = conn.Write([]byte("ok"))
			_ = conn.Close()
		}
	}()
	addr, release, err := unixSocketAddress(full)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", addr)
	release()
	if err != nil {
		t.Fatalf("dial through %q: %v", addr, err)
	}
	got, _ := io.ReadAll(conn)
	_ = conn.Close()
	if string(got) != "ok" {
		t.Fatalf("read %q", got)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("listener close removed the socket through a descriptor path: %v", err)
	}
}
