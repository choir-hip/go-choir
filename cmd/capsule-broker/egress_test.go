//go:build linux

package main

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const egressForwarderHelperEnv = "CHOIR_EGRESS_FORWARDER_HELPER"

// docs/design/engineering-network-grants-2026-10-10.md §4.0b. In a fresh
// network namespace lo is down, so nothing can reach the proxy address.
// Failure modes pinned: lo is not brought up; the forwarder does not listen
// on the proxy address; bytes are not spliced both ways to the egress
// socket; the client's half-close does not reach the proxy. Needs root to
// create the namespace (privileged container); skips otherwise.
func TestEgressForwarderSplicesLoopbackToTheEgressSocket(t *testing.T) {
	if os.Getenv(egressForwarderHelperEnv) == "" {
		if os.Geteuid() != 0 {
			t.Skip("needs root to create a network namespace")
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestEgressForwarderSplicesLoopbackToTheEgressSocket$", "-test.v")
		cmd.Env = append(os.Environ(), egressForwarderHelperEnv+"=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{Unshareflags: syscall.CLONE_NEWNET}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("forwarder helper: %v\n%s", err, output)
		}
		return
	}
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:3128", 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("fresh namespace already reaches the proxy address")
	}
	socketPath := filepath.Join(t.TempDir(), "egress.sock")
	upstream, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		for {
			conn, err := upstream.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				line, _ := bufio.NewReader(c).ReadString('\n')
				c.Write([]byte("proxy saw " + line))
			}(conn)
		}
	}()
	listener, err := startEgressForwarder("127.0.0.1:3128", socketPath)
	if err != nil {
		t.Fatalf("start forwarder: %v", err)
	}
	defer listener.Close()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:3128", 2*time.Second)
	if err != nil {
		t.Fatalf("dial proxy address: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("CONNECT pypi.org:443\n")); err != nil {
		t.Fatal(err)
	}
	conn.(*net.TCPConn).CloseWrite()
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || reply != "proxy saw CONNECT pypi.org:443\n" {
		t.Fatalf("reply = %q, %v", reply, err)
	}
}
