package vmmanager

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The host egress floor fails closed (SEC-03,
// docs/problems/host-egress-floor-fails-open-2026-10-21.md).
//
// Ways this could fail, written before the code:
//
//	H1 a filter rule (the guest's egress ACCEPT, the base chains and their
//	   FORWARD jumps, the INPUT service ACCEPTs) fails to install and the VM
//	   starts anyway;
//	H2 a nat rule (service DNAT, the host-to-guest DNAT, MASQUERADE) fails
//	   and the VM starts anyway;
//	H3 the mangle egress policy, its PREROUTING jump or the IPv6 drop fails
//	   and the VM starts anyway;
//	H4 a start refused for a rule leaves its tap or the tap's rules behind;
//	H5 snapshot restore, which shares prepareVMNetwork, starts anyway;
//	H6 reattaching a guest that is already running stops it, or tears its
//	   tap down, because a rule failed (a running computer stays on; the
//	   failure is logged and the next start fails closed);
//	H7 the INPUT service ACCEPTs admit a guest to a host address other than
//	   its own DNAT'd loopback service (another tap's host address);
//	H8 the guest DNS resolver is reached when none is configured, or not
//	   reached (DNAT and INPUT) when one is;
//	H9 a host service a guest needs (the package mirror on 8089, NET-02)
//	   is on the floor's port list but the probe and the hardening check do
//	   not expect it, so the deployed proof flags a capability as a leak, or
//	   the list outgrows the one multiport match that admits it.

type floorStep struct {
	name  string
	ipt   func(args []string) bool
	mangl func(call string) bool
}

func iptCmd(args []string) (table, cmd, chain string) {
	table = "filter"
	if len(args) >= 2 && args[0] == "-t" {
		table, args = args[1], args[2:]
	}
	if len(args) >= 2 {
		cmd, chain = args[0], args[1]
	}
	return table, cmd, chain
}

func failIPT(table, cmd, chain string) func(args []string) bool {
	return func(args []string) bool {
		t, c, ch := iptCmd(args)
		return t == table && c == cmd && ch == chain
	}
}

func floorSteps() []floorStep {
	return []floorStep{
		{name: "H1 guest egress ACCEPT", ipt: failIPT("filter", "-A", guestTapsChain)},
		{name: "H1 guest forward chain", ipt: failIPT("filter", "-A", guestForwardChain)},
		{name: "H1 reply-only chain", ipt: failIPT("filter", "-A", guestReplyChain)},
		{name: "H1 FORWARD jumps", ipt: failIPT("filter", "-I", "FORWARD")},
		{name: "H1 INPUT service ACCEPTs", ipt: failIPT("filter", "-I", "INPUT")},
		{name: "H2 service DNAT", ipt: failIPT("nat", "-A", "PREROUTING")},
		{name: "H2 host-to-guest DNAT", ipt: failIPT("nat", "-A", "OUTPUT")},
		{name: "H2 MASQUERADE", ipt: failIPT("nat", "-A", "POSTROUTING")},
		{name: "H3 egress chain", mangl: func(c string) bool { return strings.HasPrefix(c, "iptables -t mangle -A "+guestEgressChain) }},
		{name: "H3 egress jump", mangl: func(c string) bool { return strings.HasPrefix(c, "iptables -t mangle -I PREROUTING") }},
		{name: "H3 IPv6 drop", mangl: func(c string) bool { return strings.HasPrefix(c, "ip6tables -t mangle -I PREROUTING") }},
	}
}

// H1-H4: every install step, failed, refuses the start before Firecracker
// runs and leaves nothing of the tap behind.
func TestTapRuleFailureFailsTheVMStart(t *testing.T) {
	for _, step := range floorSteps() {
		t.Run(step.name, func(t *testing.T) {
			ipt := newFakeIPTables(t)
			ip := newFakeIP(nil)
			h, nf := newFakeHostNetWithEgress(t, ipt, ip)
			if step.ipt != nil {
				ipt.fail = step.ipt
			}
			if step.mangl != nil {
				nf.fail = step.mangl
			}
			m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
			m.hostNet = h
			started := false
			m.firecrackerStarter = func(string, []string) error { started = true; return nil }
			vmID := "vm-floor-" + strings.ReplaceAll(strings.ToLower(step.name), " ", "-")
			tap := tapNameForVMID(vmID)
			err := m.launchFirecracker(VMConfig{VMID: vmID, GuestPort: 8085}, map[string]interface{}{}, 9300)
			if err == nil {
				t.Fatalf("VM started with %s failing", step.name)
			}
			if started {
				t.Fatal("Firecracker ran although a floor rule failed")
			}
			if ip.has(tap) {
				t.Fatalf("H4: tap %s survived the refused start", tap)
			}
			if dump := ipt.dump(); strings.Contains(dump, tap) {
				t.Fatalf("H4: rules for %s survived the refused start:\n%s", tap, dump)
			}
		})
	}
}

// The control for the test above: with nothing failing the start proceeds.
func TestTapRulesInstalledStartsTheVM(t *testing.T) {
	ipt := newFakeIPTables(t)
	ip := newFakeIP(nil)
	h, _ := newFakeHostNetWithEgress(t, ipt, ip)
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
	m.hostNet = h
	started := false
	m.firecrackerStarter = func(string, []string) error { started = true; return nil }
	if err := m.launchFirecracker(VMConfig{VMID: "vm-floor-ok", GuestPort: 8085}, map[string]interface{}{}, 9300); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if !started {
		t.Fatal("Firecracker did not run")
	}
}

// H5: the restore path's network step reports the failure.
func TestPrepareVMNetworkReturnsRuleFailure(t *testing.T) {
	for _, step := range floorSteps() {
		t.Run(step.name, func(t *testing.T) {
			ipt := newFakeIPTables(t)
			h, nf := newFakeHostNetWithEgress(t, ipt, newFakeIP(nil))
			ipt.fail, nf.fail = step.ipt, step.mangl
			m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
			m.hostNet = h
			if err := m.prepareVMNetwork("vm-floor-restore", 9302, 8085, -1); err == nil {
				t.Fatalf("prepareVMNetwork returned nil with %s failing", step.name)
			}
		})
	}
}

// H6: reattach reconciles a running guest's rules; a failure is logged and
// the guest's tap stays.
func TestReattachRuleFailureKeepsTheRunningGuest(t *testing.T) {
	for _, step := range floorSteps() {
		t.Run(step.name, func(t *testing.T) {
			ipt := newFakeIPTables(t)
			ip := newFakeIP(nil)
			h, nf := newFakeHostNetWithEgress(t, ipt, ip)
			m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
			m.hostNet = h
			vmID := "vm-floor-reattach"
			if err := m.prepareVMNetwork(vmID, 9303, 8085, -1); err != nil {
				t.Fatal(err)
			}
			// A rule lost while the guest runs, then a failing repair.
			for _, chain := range []string{"INPUT", "FORWARD", guestTapsChain, guestForwardChain, guestReplyChain} {
				_, _ = ipt.run("-F", chain)
			}
			nf.chains["iptables/mangle/PREROUTING"] = nil
			nf.chains["iptables/mangle/"+guestEgressChain] = nil
			nf.chains["ip6tables/mangle/PREROUTING"] = nil
			ipt.fail, nf.fail = step.ipt, step.mangl
			m.reconcileTapIsolation(vmID, fmt.Sprintf("http://127.0.0.1:%d", 9303))
			m.ensureGuestEgressPolicy()
			if !ip.has(tapNameForVMID(vmID)) {
				t.Fatal("H6: a failed repair tore down a running guest's tap")
			}
		})
	}
}

// H7: a guest reaches its host services only through its own DNAT to
// loopback; not another tap's host address, not other host ports.
func TestTapServiceInputIsBoundToOwnLoopbackDNAT(t *testing.T) {
	ipt := newFakeIPTables(t)
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	taps := tapsAtOrdinals(t, 3, 300)
	for _, tp := range taps {
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatal(err)
		}
	}
	a, b := taps[0], taps[1]
	for _, port := range tapReachableHostServicePorts() {
		own := fakePacket{in: a.name, src: a.guest, dst: "127.0.0.1", proto: "tcp", dport: port}
		if v := ipt.verdict("INPUT", own); v != "ACCEPT" {
			t.Fatalf("guest to its own service %s: verdict %q, want ACCEPT\n%s", port, v, ipt.dump())
		}
		other := fakePacket{in: a.name, src: a.guest, dst: b.host, proto: "tcp", dport: port}
		if v := ipt.verdict("INPUT", other); v == "ACCEPT" {
			t.Fatalf("H7: guest %s admitted to tap %s's host address on %s", a.name, b.name, port)
		}
	}
	for _, port := range []string{"22", "80", "443", "13306"} {
		p := fakePacket{in: a.name, src: a.guest, dst: "127.0.0.1", proto: "tcp", dport: port}
		if v := ipt.verdict("INPUT", p); v == "ACCEPT" {
			t.Fatalf("H7: guest admitted to host port %s", port)
		}
	}
}

// H8: the guest resolver is DNAT'd and admitted only when configured.
func TestGuestDNSReachesHostResolverOnlyWhenConfigured(t *testing.T) {
	tp := tapsAtOrdinals(t, 7)[0]
	hasDNSDNAT := func(ipt *fakeIPTables) bool {
		for _, r := range ipt.rules("nat", "PREROUTING") {
			if strings.Contains(r, "--dport 53 ") && strings.Contains(r, "-j DNAT") {
				return true
			}
		}
		return false
	}
	t.Run("unset", func(t *testing.T) {
		ipt := newFakeIPTables(t)
		h := newFakeHostNet(t, ipt, newFakeIP(nil))
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatal(err)
		}
		if err := h.ensureNAT(tp.name, tp.host, tp.guest, 9307, 8085); err != nil {
			t.Fatal(err)
		}
		if hasDNSDNAT(ipt) {
			t.Fatalf("DNS DNAT installed with no resolver configured:\n%s", ipt.dump())
		}
	})
	t.Run("configured", func(t *testing.T) {
		ipt := newFakeIPTables(t)
		h := newFakeHostNet(t, ipt, newFakeIP(nil))
		h.guestDNSPort = 8053
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatal(err)
		}
		if err := h.ensureNAT(tp.name, tp.host, tp.guest, 9307, 8085); err != nil {
			t.Fatal(err)
		}
		for _, proto := range []string{"udp", "tcp"} {
			want := fmt.Sprintf("-A PREROUTING -d %s/32 -p %s -m %s --dport 53 -m comment --comment %s -j DNAT --to-destination 127.0.0.1:8053", tp.host, proto, proto, tapComment(tp.name))
			found := false
			for _, r := range ipt.rules("nat", "PREROUTING") {
				found = found || r == want
			}
			if !found {
				t.Fatalf("no %s DNS DNAT %q:\n%s", proto, want, ipt.dump())
			}
			p := fakePacket{in: tp.name, src: tp.guest, dst: "127.0.0.1", proto: proto, dport: "8053"}
			if v := ipt.verdict("INPUT", p); v != "ACCEPT" {
				t.Fatalf("guest %s to the resolver: verdict %q, want ACCEPT", proto, v)
			}
		}
	})
}

// H9: every port on the floor's service list (tapReachableHostServicePorts,
// the one list the egress chain, the INPUT ACCEPTs and the DNAT read) is
// one the deployed probes expect open, with the package mirror (8089,
// NET-02, go-choir-pkgmirror) named; and the list fits one multiport match.
func TestHostFloorScriptsExpectEveryTapServicePort(t *testing.T) {
	ports := tapReachableHostServicePorts()
	if !slices.Contains(ports, "8089") {
		t.Fatalf("the package mirror 8089 is not on the floor's service list %v", ports)
	}
	if len(ports) > 15 {
		t.Fatalf("%d service ports: iptables multiport takes at most 15", len(ports))
	}
	tp := tapsAtOrdinals(t, 9)[0]
	ipt := newFakeIPTables(t)
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	if err := h.ensureTap(tp.name, tp.guest); err != nil {
		t.Fatal(err)
	}
	if v := ipt.verdict("INPUT", fakePacket{in: tp.name, src: tp.guest, dst: "127.0.0.1", proto: "tcp", dport: "8089"}); v != "ACCEPT" {
		t.Fatalf("guest to its own package mirror: verdict %q, want ACCEPT", v)
	}
	dnat := false
	for _, r := range tapNATSpecs(tp.name, tp.host, tp.guest, 9309, 8085, 0) {
		if r.chain == "PREROUTING" && strings.Contains(strings.Join(r.spec, " "), "--dport 8089 ") && slices.Contains(r.spec, "127.0.0.1:8089") {
			dnat = true
		}
	}
	if !dnat {
		t.Fatalf("no DNAT of the tap host address :8089 to the loopback mirror")
	}

	probe, err := os.ReadFile("../../scripts/host_floor_probe.py")
	if err != nil {
		t.Fatal(err)
	}
	hardening, err := os.ReadFile("../../scripts/verify-host-hardening.py")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^TAP_SERVICE_PORTS = \(([^)]*)\)`).FindSubmatch(hardening)
	if m == nil {
		t.Fatal("verify-host-hardening.py has no TAP_SERVICE_PORTS tuple")
	}
	for _, port := range ports {
		open := regexp.MustCompile(`f"\{host\}:` + port + `", True,`)
		if !open.Match(probe) {
			t.Errorf("host_floor_probe.py does not expect own-tap :%s open", port)
		}
		if !strings.Contains(string(m[1]), `"`+port+`"`) {
			t.Errorf("verify-host-hardening.py TAP_SERVICE_PORTS lacks %s", port)
		}
	}
	if !regexp.MustCompile(`"own pkgmirror", f"\{host\}:8089", True, "[^"]*NET-02`).Match(probe) {
		t.Errorf("host_floor_probe.py does not name 8089's owner (pkgmirror, NET-02)")
	}
}
