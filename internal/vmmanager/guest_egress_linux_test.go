//go:build linux

package vmmanager

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestGuestEgressPolicyOnRealNetfilter builds a guest, the host and "the
// internet" out of network namespaces and veth pairs, then checks what the
// guest can reach before and after the policy. It needs root, iproute2 and
// iptables, and changes the host's netfilter state, so it runs only when
// CHOIR_NETFILTER_TEST=1 (a disposable container or VM):
//
//	docker run --privileged ... CHOIR_NETFILTER_TEST=1 go test ./internal/vmmanager -run TestGuestEgressPolicyOnRealNetfilter
func TestGuestEgressPolicyOnRealNetfilter(t *testing.T) {
	if os.Getenv("CHOIR_NETFILTER_TEST") != "1" {
		t.Skip("set CHOIR_NETFILTER_TEST=1 in a disposable root container")
	}
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	for _, bin := range []string{"ip", "iptables"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s missing", bin)
		}
	}
	const (
		guestNS  = "choir-eg-guest"
		worldNS  = "choir-eg-world"
		tap      = "vm-egtest00001" // matches vm-+, as vmmanager's taps do
		hostIP   = "10.200.250.1"
		guestIP  = "10.200.250.2"
		worldHop = "45.45.45.1"
		worldIP  = "45.45.45.2"
		privIP   = "192.168.77.2"
		metaIP   = "169.254.169.254"
	)
	sh := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	cleanup := func() {
		_ = exec.Command("ip", "netns", "del", guestNS).Run()
		_ = exec.Command("ip", "netns", "del", worldNS).Run()
		_ = exec.Command("ip", "link", "del", tap).Run()
		_ = exec.Command("ip", "link", "del", "eg-world0").Run()
		g := &guestEgressInstaller{run: runCombined, ipt: "iptables", ip6t: "ip6tables"}
		g.remove()
	}
	cleanup()
	t.Cleanup(cleanup)
	sh("sysctl", "-qw", "net.ipv4.ip_forward=1")
	sh("ip", "netns", "add", guestNS)
	sh("ip", "netns", "add", worldNS)
	sh("ip", "link", "add", tap, "type", "veth", "peer", "name", "eg-guest1")
	sh("ip", "link", "set", "eg-guest1", "netns", guestNS)
	sh("ip", "addr", "add", hostIP+"/30", "dev", tap)
	sh("ip", "link", "set", tap, "up")
	sh("ip", "-n", guestNS, "addr", "add", guestIP+"/30", "dev", "eg-guest1")
	sh("ip", "-n", guestNS, "link", "set", "eg-guest1", "up")
	sh("ip", "-n", guestNS, "link", "set", "lo", "up")
	sh("ip", "-n", guestNS, "route", "add", "default", "via", hostIP)
	sh("ip", "link", "add", "eg-world0", "type", "veth", "peer", "name", "eg-world1")
	sh("ip", "link", "set", "eg-world1", "netns", worldNS)
	sh("ip", "addr", "add", worldHop+"/24", "dev", "eg-world0")
	sh("ip", "link", "set", "eg-world0", "up")
	for _, a := range []string{worldIP + "/24", privIP + "/32", metaIP + "/32"} {
		sh("ip", "-n", worldNS, "addr", "add", a, "dev", "eg-world1")
	}
	sh("ip", "-n", worldNS, "link", "set", "eg-world1", "up")
	sh("ip", "-n", worldNS, "link", "set", "lo", "up")
	sh("ip", "-n", worldNS, "route", "add", "default", "via", worldHop)
	sh("ip", "route", "add", privIP+"/32", "via", worldIP)
	sh("ip", "route", "add", metaIP+"/32", "via", worldIP)

	// Listeners: the host's own ports and the world's.
	for _, port := range []string{"22", "8084", "8086", "13306", "443"} {
		listenTCP(t, "", hostIP+":"+port)
	}
	for _, port := range []string{"443", "8443", "25"} {
		listenTCP(t, worldNS, worldIP+":"+port)
	}
	listenTCP(t, worldNS, privIP+":443")
	listenTCP(t, worldNS, metaIP+":80")
	udpSeen := map[string]chan struct{}{}
	for _, port := range []string{"53", "9999"} {
		udpSeen[worldIP+":"+port] = listenUDP(t, worldNS, worldIP+":"+port)
	}
	udpSeen[hostIP+":53"] = listenUDP(t, "", hostIP+":53")
	// The guest's runtime port, reached by the host (vmctl's direction).
	listenTCP(t, guestNS, guestIP+":8085")

	type probe struct {
		name, network, addr string
		allowed             bool
	}
	probes := []probe{
		{"gateway on the tap peer", "tcp", hostIP + ":8084", true},
		{"corpusd on the tap peer", "tcp", hostIP + ":8086", true},
		{"own host resolver", "udp", hostIP + ":53", true},
		{"internet https", "tcp", worldIP + ":443", true},
		{"internet other tcp port", "tcp", worldIP + ":8443", true},
		{"internet dns", "udp", worldIP + ":53", true},
		{"host sshd", "tcp", hostIP + ":22", false},
		{"host dolt", "tcp", hostIP + ":13306", false},
		{"public front door on a host address (SEC-03)", "tcp", hostIP + ":443", false},
		{"internet smtp", "tcp", worldIP + ":25", false},
		{"private range", "tcp", privIP + ":443", false},
		{"metadata", "tcp", metaIP + ":80", false},
		{"internet arbitrary udp", "udp", worldIP + ":9999", false},
	}
	reach := func(p probe) bool {
		if p.network == "udp" {
			seen := udpSeen[p.addr]
			// Drain an earlier datagram.
			select {
			case <-seen:
			default:
			}
			inNetns(t, guestNS, func() {
				c, err := net.Dial("udp", p.addr)
				if err == nil {
					_, _ = c.Write([]byte("probe"))
					c.Close()
				}
			})
			select {
			case <-seen:
				return true
			case <-time.After(700 * time.Millisecond):
				return false
			}
		}
		ok := false
		inNetns(t, guestNS, func() {
			c, err := net.DialTimeout("tcp", p.addr, 700*time.Millisecond)
			if err == nil {
				ok = true
				c.Close()
			}
		})
		return ok
	}

	// Control: with no policy every probe connects, so a "denied" below
	// is the policy's doing, not the topology's.
	for _, p := range probes {
		if !reach(p) {
			t.Fatalf("control: %s (%s) unreachable before the policy; the test topology is broken", p.name, p.addr)
		}
	}

	g := &guestEgressInstaller{run: runCombined, ipt: "iptables", ip6t: "ip6tables"}
	if err := g.ensure(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := g.ensure(); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	for _, p := range probes {
		if got := reach(p); got != p.allowed {
			t.Errorf("%s (%s %s): reachable=%v, want %v", p.name, p.network, p.addr, got, p.allowed)
		}
	}
	// Host-initiated flow to the guest still works (its replies enter
	// from the tap with a random destination port).
	c, err := net.DialTimeout("tcp", guestIP+":8085", time.Second)
	if err != nil {
		t.Errorf("host to guest :8085 after the policy: %v", err)
	} else {
		buf := make([]byte, 2)
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		if n, _ := c.Read(buf); n != 2 || string(buf) != "ok" {
			t.Errorf("host to guest :8085: no reply through the policy (read %q)", buf[:n])
		}
		c.Close()
	}
	out, _ := exec.Command("iptables", "-t", "mangle", "-S", guestEgressChain).CombinedOutput()
	t.Logf("installed:\n%s", out)
}

func runCombined(bin string, args ...string) ([]byte, error) {
	return exec.Command(bin, args...).CombinedOutput()
}

// inNetns runs f on a fresh OS thread inside the named network namespace
// ("" = the current one). The thread is never returned to the scheduler.
func inNetns(t *testing.T, ns string, f func()) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if ns != "" {
			fd, err := unix.Open("/var/run/netns/"+ns, unix.O_RDONLY|unix.O_CLOEXEC, 0)
			if err != nil {
				done <- err
				return
			}
			defer unix.Close(fd)
			if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
				done <- err
				return
			}
		}
		f()
		done <- nil
	}()
	if err := <-done; err != nil {
		t.Fatalf("netns %s: %v", ns, err)
	}
}

// listenTCP accepts on addr in ns and answers "ok" to every connection.
func listenTCP(t *testing.T, ns, addr string) {
	t.Helper()
	var ln net.Listener
	inNetns(t, ns, func() {
		var err error
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			t.Errorf("listen %s in %q: %v", addr, ns, err)
		}
	})
	if ln == nil {
		t.FailNow()
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte("ok"))
			c.Close()
		}
	}()
}

func listenUDP(t *testing.T, ns, addr string) chan struct{} {
	t.Helper()
	var pc net.PacketConn
	inNetns(t, ns, func() {
		var err error
		pc, err = net.ListenPacket("udp", addr)
		if err != nil {
			t.Errorf("listen udp %s in %q: %v", addr, ns, err)
		}
	})
	if pc == nil {
		t.FailNow()
	}
	t.Cleanup(func() { pc.Close() })
	seen := make(chan struct{}, 16)
	go func() {
		buf := make([]byte, 64)
		for {
			if _, _, err := pc.ReadFrom(buf); err != nil {
				return
			}
			seen <- struct{}{}
		}
	}()
	return seen
}

// TestHostFloorOnRealNetfilter runs the whole host side of a tap
// (setupHostNetworking: filter, nat, mangle, IPv6) against the kernel's
// netfilter, with a guest, a second guest and "the internet" in network
// namespaces, and checks SEC-03 by connection: the guest reaches its own
// host services through the loopback DNAT and its host resolver, the
// public internet over TCP, and nothing else on the host, another tap's
// host address, private ranges or SMTP; the host reaches the guest; the
// internet cannot open a connection to the guest. Same gate as above; it
// also runs unprivileged in a user and network namespace
// (scripts/vmmanager-netfilter-test.sh).
func TestHostFloorOnRealNetfilter(t *testing.T) {
	if os.Getenv("CHOIR_NETFILTER_TEST") != "1" {
		t.Skip("set CHOIR_NETFILTER_TEST=1 in a disposable root container")
	}
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	for _, bin := range []string{"ip", "iptables", "ip6tables"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s missing", bin)
		}
	}
	const (
		guestNS  = "choir-fl-guest"
		otherNS  = "choir-fl-other"
		worldNS  = "choir-fl-world"
		worldHop = "45.45.46.1"
		worldIP  = "45.45.46.2"
		privIP   = "192.168.78.2"
		dnsPort  = 8053
	)
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
	m.hostNet = newSystemHostNet()
	m.hostNet.guestDNSPort = dnsPort
	m.hostNetOnce.Do(func() {})
	const portA, portB = 9000 + 77, 9000 + 300
	guestA, hostA := m.guestAndHostIP(portA)
	guestB, hostB := m.guestAndHostIP(portB)
	tapA, tapB := tapNameForVMID("vm-floor-real-a"), tapNameForVMID("vm-floor-real-b")

	sh := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	cleanup := func() {
		for _, ns := range []string{guestNS, otherNS, worldNS} {
			_ = exec.Command("ip", "netns", "del", ns).Run()
		}
		for _, l := range []string{tapA, tapB, "fl-world0"} {
			_ = exec.Command("ip", "link", "del", l).Run()
		}
		m.hostNet.removeTap(tapA)
		m.hostNet.removeTap(tapB)
		m.hostNet.egress.remove()
	}
	cleanup()
	t.Cleanup(cleanup)
	sh("ip", "link", "set", "lo", "up")
	for _, g := range []struct{ ns, tap, peer, guest, host string }{
		{guestNS, tapA, "fl-ga", guestA, hostA},
		{otherNS, tapB, "fl-gb", guestB, hostB},
	} {
		sh("ip", "netns", "add", g.ns)
		sh("ip", "link", "add", g.tap, "type", "veth", "peer", "name", g.peer)
		sh("ip", "link", "set", g.peer, "netns", g.ns)
		sh("ip", "link", "set", g.tap, "up")
		sh("ip", "addr", "add", g.host+"/30", "dev", g.tap)
		sh("ip", "-n", g.ns, "addr", "add", g.guest+"/30", "dev", g.peer)
		sh("ip", "-n", g.ns, "link", "set", g.peer, "up")
		sh("ip", "-n", g.ns, "link", "set", "lo", "up")
		sh("ip", "-n", g.ns, "route", "add", "default", "via", g.host)
	}
	sh("ip", "netns", "add", worldNS)
	sh("ip", "link", "add", "fl-world0", "type", "veth", "peer", "name", "fl-world1")
	sh("ip", "link", "set", "fl-world1", "netns", worldNS)
	sh("ip", "addr", "add", worldHop+"/24", "dev", "fl-world0")
	sh("ip", "link", "set", "fl-world0", "up")
	for _, a := range []string{worldIP + "/24", privIP + "/32"} {
		sh("ip", "-n", worldNS, "addr", "add", a, "dev", "fl-world1")
	}
	sh("ip", "-n", worldNS, "link", "set", "fl-world1", "up")
	sh("ip", "-n", worldNS, "link", "set", "lo", "up")
	sh("ip", "-n", worldNS, "route", "add", "default", "via", worldHop)
	sh("ip", "route", "add", privIP+"/32", "via", worldIP)
	sh("sysctl", "-qw", "net.ipv4.ip_forward=1")

	// The host: services on loopback, sshd and Caddy on every address, the
	// resolver on loopback.
	listenTCP(t, "", "127.0.0.1:8084")
	listenTCP(t, "", "127.0.0.1:8089") // the package mirror (NET-02)
	listenTCP(t, "", "0.0.0.0:8086")   // a service bound too widely
	listenTCP(t, "", "0.0.0.0:22")
	listenTCP(t, "", "0.0.0.0:443")
	listenTCP(t, "", fmt.Sprintf("127.0.0.1:%d", dnsPort))
	resolverSeen := listenUDP(t, "", fmt.Sprintf("127.0.0.1:%d", dnsPort))
	// The internet.
	for _, port := range []string{"443", "25"} {
		listenTCP(t, worldNS, worldIP+":"+port)
	}
	listenTCP(t, worldNS, privIP+":443")
	// The guest's runtime port.
	listenTCP(t, guestNS, guestA+":8085")

	dial := func(ns, addr string) bool {
		ok := false
		inNetns(t, ns, func() {
			c, err := net.DialTimeout("tcp", addr, 700*time.Millisecond)
			if err == nil {
				buf := make([]byte, 2)
				_ = c.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
				n, _ := c.Read(buf)
				ok = n == 2
				c.Close()
			}
		})
		return ok
	}
	udpToResolver := func() bool {
		select {
		case <-resolverSeen:
		default:
		}
		inNetns(t, guestNS, func() {
			if c, err := net.Dial("udp", hostA+":53"); err == nil {
				_, _ = c.Write([]byte("probe"))
				c.Close()
			}
		})
		select {
		case <-resolverSeen:
			return true
		case <-time.After(700 * time.Millisecond):
			return false
		}
	}
	type probe struct {
		name    string
		ns      string
		addr    string
		control bool // reachable with no rules at all
		allowed bool
	}
	probes := []probe{
		{"guest: own gateway through DNAT", guestNS, hostA + ":8084", false, true},
		{"guest: own package mirror through DNAT", guestNS, hostA + ":8089", false, true},
		{"guest: another tap's package mirror", guestNS, hostB + ":8089", false, false},
		{"guest: own resolver over TCP through DNAT", guestNS, fmt.Sprintf("%s:53", hostA), false, true},
		{"guest: internet https", guestNS, worldIP + ":443", true, true},
		{"guest: host sshd on its tap address", guestNS, hostA + ":22", true, false},
		{"guest: host front door on its tap address", guestNS, hostA + ":443", true, false},
		{"guest: another tap's host address", guestNS, hostB + ":8086", true, false},
		{"guest: internet smtp", guestNS, worldIP + ":25", true, false},
		{"guest: private range", guestNS, privIP + ":443", true, false},
		{"internet: new connection to the guest", worldNS, guestA + ":8085", true, false},
		// vmctl's direction: HostURL is the guest address (manager.go).
		{"host: guest runtime", "", guestA + ":8085", true, true},
	}
	// The world needs a route to the guest for the inbound probe.
	sh("ip", "-n", worldNS, "route", "add", guestA+"/32", "via", worldHop)
	for _, p := range probes {
		if p.control && !dial(p.ns, p.addr) {
			t.Fatalf("control: %s (%s) unreachable before the floor; the topology is broken", p.name, p.addr)
		}
	}

	if err := m.setupHostNetworking(tapA, hostA, portA, guestA, 8085); err != nil {
		t.Fatalf("setup tap A: %v", err)
	}
	if err := m.setupHostNetworking(tapB, hostB, portB, guestB, 8085); err != nil {
		t.Fatalf("setup tap B: %v", err)
	}
	for _, p := range probes {
		got := dial(p.ns, p.addr)
		if got != p.allowed {
			t.Errorf("%s (%s): reachable=%v, want %v", p.name, p.addr, got, p.allowed)
			continue
		}
		t.Logf("verdict ok: %s (%s) reachable=%v", p.name, p.addr, got)
	}
	if !udpToResolver() {
		t.Error("guest: own resolver over UDP through DNAT: unreachable")
	}
	for _, table := range []string{"filter", "nat", "mangle"} {
		out, _ := exec.Command("iptables", "-t", table, "-S").CombinedOutput()
		t.Logf("%s:\n%s", table, out)
	}
}
