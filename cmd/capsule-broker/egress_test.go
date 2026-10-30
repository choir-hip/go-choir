//go:build linux

package main

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"golang.org/x/sys/unix"
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

const brokerCapsHelperEnv = "CHOIR_BROKER_CAPS_HELPER"

// docs/archive/problems/l2-broker-cannot-bring-up-loopback-2026-10-10.md: a process
// launched into the capsule user namespace holds only its ambient
// capabilities. Failure modes pinned: the L2 ambient set cannot bring up lo
// (every L2 capsule dies at start); CAP_NET_ADMIN survives the broker's
// capability drop, into its serving phase and every workload it spawns.
// Needs root (privileged container); skips otherwise.
func TestL2BrokerBringsUpLoopbackThenDropsNetAdmin(t *testing.T) {
	if os.Getenv(brokerCapsHelperEnv) == "" {
		if os.Geteuid() != 0 {
			t.Skip("needs root to create a user namespace with mapped ids")
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestL2BrokerBringsUpLoopbackThenDropsNetAdmin$", "-test.v")
		cmd.Env = append(os.Environ(), brokerCapsHelperEnv+"=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags:   syscall.CLONE_NEWUSER,
			Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
			UidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65534, Size: 1}},
			GidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: 65534, Size: 1}},
			AmbientCaps:  capsule.BrokerAmbientCaps(true),
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("broker caps helper: %v\n%s", err, output)
		}
		return
	}
	if err := bringLoopbackUp(); err != nil {
		t.Fatalf("L2 ambient set cannot bring up lo: %v", err)
	}
	if err := capsule.DropBrokerCapabilities(); err != nil {
		t.Fatalf("drop broker capabilities: %v", err)
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(status), "\n") {
		field, value, ok := strings.Cut(line, ":")
		if !ok || (field != "CapEff" && field != "CapAmb" && field != "CapPrm" && field != "CapBnd") {
			continue
		}
		bits, err := strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		if bits&(1<<unix.CAP_NET_ADMIN) != 0 {
			t.Fatalf("%s still holds CAP_NET_ADMIN after the broker drop: %s", field, strings.TrimSpace(value))
		}
	}
}
