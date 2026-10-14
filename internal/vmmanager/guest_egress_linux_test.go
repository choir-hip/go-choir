//go:build linux

package vmmanager

import (
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
		udpSeen[port] = listenUDP(t, worldNS, worldIP+":"+port)
	}
	// The guest's runtime port, reached by the host (vmctl's direction).
	listenTCP(t, guestNS, guestIP+":8085")

	type probe struct {
		name, network, addr string
		allowed             bool
	}
	probes := []probe{
		{"gateway on the tap peer", "tcp", hostIP + ":8084", true},
		{"corpusd on the tap peer", "tcp", hostIP + ":8086", true},
		{"public front door on a host address", "tcp", hostIP + ":443", true},
		{"internet https", "tcp", worldIP + ":443", true},
		{"internet other tcp port", "tcp", worldIP + ":8443", true},
		{"internet dns", "udp", worldIP + ":53", true},
		{"host sshd", "tcp", hostIP + ":22", false},
		{"host dolt", "tcp", hostIP + ":13306", false},
		{"internet smtp", "tcp", worldIP + ":25", false},
		{"private range", "tcp", privIP + ":443", false},
		{"metadata", "tcp", metaIP + ":80", false},
		{"internet arbitrary udp", "udp", worldIP + ":9999", false},
	}
	reach := func(p probe) bool {
		if p.network == "udp" {
			port := p.addr[strings.LastIndex(p.addr, ":")+1:]
			seen := udpSeen[port]
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
