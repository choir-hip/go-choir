package vmmanager

import (
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Guest isolation on the host firewall
// (docs/archive/problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md).
//
// Failure modes, written before the code:
//
//	F1  the inter-guest deny misses part of the allocator's range (the
//	    10.200.0.0/16 deny vs. guests in 10.201-10.215);
//	F2  the deny over-reaches into non-guest space (a /12 would also cover
//	    10.192-10.199, blocking guest egress to LANs/VPNs there);
//	F3  the pool deny sits after the per-tap ACCEPT, so a guest's own
//	    ACCEPT carries cross-guest packets;
//	F4  a guest interface with no per-tap rule forwards freely (fail open
//	    under FORWARD policy ACCEPT);
//	F5  installing the default-deny at vmctl start drops egress of live
//	    guests that have not been reattached yet;
//	F6  rebuilding a stale base chain leaves it empty for a moment, so
//	    cross-guest traffic falls through to policy ACCEPT;
//	F7  the orphan sweep deletes rules this code does not own (NixOS
//	    firewall, Docker, operator rules, look-alike interface names);
//	F8  the sweep deletes rules of live taps;
//	F9  the sweep runs when the link list is unknown and so treats every
//	    tap as vanished;
//	F10 a tap's rules survive teardown (stop, crash, failed boot);
//	F11 tearing down tap A removes tap B's rules (substring matching);
//	F12 re-running setup duplicates rules (not idempotent);
//	F13 migrating a live tap's legacy rules opens a gap in its access or in
//	    isolation;
//	F14 the INPUT service ACCEPTs admit a forged source (they sat ahead of
//	    the anti-spoof DROP on Node B);
//	F15 rule-text parsing disagrees with the real `iptables -S` format, so
//	    a wanted rule is judged stale and deleted;
//	F16 a crash teardown races a relaunch and deletes the new VM's tap;
//	F17 hostNet changes the host's FORWARD policy or deletes chains it does
//	    not own.
//	F18 (SEC-03) a new connection is forwarded towards a guest: `-o <tap>`
//	    must admit replies (ESTABLISHED,RELATED) only.

const nodeBFixture = "testdata/node-b-iptables-2026-10-14.txt"

var nodeBLiveTaps = map[string]string{
	"vm-dhdbzl4tzqi3": "10.200.112.1/30",
	"vm-vh3spxieztky": "10.200.113.1/30",
	"vm-lackej5pmie2": "10.200.114.1/30",
}

func guestIPForOrdinal(t *testing.T, ordinal int) (guest, host string) {
	t.Helper()
	m := &Manager{cfg: ManagerConfig{HostBasePort: 9000}}
	return m.guestAndHostIP(9000 + ordinal)
}

// F1, F2.
func TestGuestPoolCIDRsCoverExactlyTheAllocatorRange(t *testing.T) {
	got := guestPoolCIDRs()
	if want := []string{"10.200.0.0/13", "10.208.0.0/13"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("guestPoolCIDRs() = %v, want %v", got, want)
	}
	nets := make([]*net.IPNet, 0, len(got))
	for _, c := range got {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatal(err)
		}
		nets = append(nets, n)
	}
	in := func(ip string) bool {
		for _, n := range nets {
			if n.Contains(net.ParseIP(ip)) {
				return true
			}
		}
		return false
	}
	for ordinal := 0; ordinal < vmSubnetCapacity; ordinal++ {
		guest, host := guestIPForOrdinal(t, ordinal)
		if !in(guest) || !in(host) {
			t.Fatalf("ordinal %d: guest %s / host %s outside deny %v", ordinal, guest, host, got)
		}
	}
	for _, outside := range []string{"10.199.255.255", "10.216.0.0", "10.192.0.1", "10.0.0.1", "192.168.1.1"} {
		if in(outside) {
			t.Fatalf("%s is not a guest address but is inside the deny %v", outside, got)
		}
	}
}

func TestCIDRsForSecondOctetRange(t *testing.T) {
	for _, tc := range []struct {
		base, count int
		want        []string
	}{
		{200, 16, []string{"10.200.0.0/13", "10.208.0.0/13"}},
		{0, 256, []string{"10.0.0.0/8"}},
		{100, 1, []string{"10.100.0.0/16"}},
		{3, 5, []string{"10.3.0.0/16", "10.4.0.0/14"}},
		{128, 128, []string{"10.128.0.0/9"}},
		{250, 6, []string{"10.250.0.0/15", "10.252.0.0/14"}},
	} {
		if got := cidrsForSecondOctetRange(tc.base, tc.count); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("cidrsForSecondOctetRange(%d, %d) = %v, want %v", tc.base, tc.count, got, tc.want)
		}
	}
}

type testTap struct {
	name, guest, host string
}

func tapsAtOrdinals(t *testing.T, ordinals ...int) []testTap {
	t.Helper()
	var taps []testTap
	for _, o := range ordinals {
		guest, host := guestIPForOrdinal(t, o)
		taps = append(taps, testTap{name: tapNameForVMID(fmt.Sprintf("vm-ordinal-%d", o)), guest: guest, host: host})
	}
	return taps
}

func crossGuest(a, b testTap) fakePacket {
	return fakePacket{in: a.name, out: b.name, src: a.guest, dst: b.guest, proto: "tcp", dport: "8085"}
}

func egress(a testTap) fakePacket {
	return fakePacket{in: a.name, out: "eno1", src: a.guest, dst: "1.1.1.1", proto: "tcp", dport: "443"}
}

// assertIsolated checks every ordered pair of taps and the egress/spoof cases.
func assertIsolated(t *testing.T, ipt *fakeIPTables, taps []testTap) {
	t.Helper()
	for _, a := range taps {
		for _, b := range taps {
			if a == b {
				continue
			}
			if v := ipt.verdict("FORWARD", crossGuest(a, b)); v != "DROP" {
				t.Fatalf("guest %s (%s) -> guest %s (%s): verdict %q, want DROP\n%s", a.name, a.guest, b.name, b.guest, v, ipt.dump())
			}
		}
		if v := ipt.verdict("FORWARD", egress(a)); v != "ACCEPT" {
			t.Fatalf("guest %s egress: verdict %q, want ACCEPT\n%s", a.name, v, ipt.dump())
		}
		spoof := egress(a)
		spoof.src = "10.200.250.2"
		if v := ipt.verdict("FORWARD", spoof); v != "DROP" {
			t.Fatalf("guest %s forged-source egress: verdict %q, want DROP", a.name, v)
		}
		reply := fakePacket{in: "eno1", out: a.name, src: "1.1.1.1", dst: a.guest, proto: "tcp", dport: "50000", state: "ESTABLISHED"}
		if v := ipt.verdict("FORWARD", reply); v != "ACCEPT" {
			t.Fatalf("reply to guest %s: verdict %q, want ACCEPT", a.name, v)
		}
		// SEC-03 (F8): towards a guest, only replies are forwarded.
		inbound := reply
		inbound.state = ""
		if v := ipt.verdict("FORWARD", inbound); v != "DROP" {
			t.Fatalf("new connection forwarded to guest %s: verdict %q, want DROP\n%s", a.name, v, ipt.dump())
		}
	}
}

// F1, F3, F4, F17: isolation holds for the whole pool, by packet verdict.
func TestHostFirewallIsolatesGuestsAcrossTheWholePool(t *testing.T) {
	ipt := newFakeIPTables(t)
	ipt.loadString(`*filter
-A FORWARD -i br0 -o eno1 -j ACCEPT
`)
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	taps := tapsAtOrdinals(t, 0, 255, 256, 1000, vmSubnetCapacity-1)
	for _, tp := range taps {
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatalf("ensureTap(%s): %v", tp.name, err)
		}
	}
	assertIsolated(t, ipt, taps)

	// F4: a guest interface with no per-tap rule fails closed.
	unknown := fakePacket{in: "vm-zzzzzzzzzzzz", out: "eno1", src: "10.203.0.2", dst: "1.1.1.1", proto: "tcp", dport: "443"}
	if v := ipt.verdict("FORWARD", unknown); v != "DROP" {
		t.Fatalf("forward from a tap with no rules: verdict %q, want DROP", v)
	}
	// Non-guest forwarding is not ours: still the host's policy.
	foreign := fakePacket{in: "eno1", out: "eno2", src: "192.168.1.5", dst: "192.168.2.5", proto: "tcp", dport: "22"}
	if v := ipt.verdict("FORWARD", foreign); v != "policy ACCEPT" {
		t.Fatalf("non-guest forward: verdict %q, want the host policy", v)
	}
	if !strings.Contains(strings.Join(ipt.rules("filter", "FORWARD"), "\n"), "-A FORWARD -i br0 -o eno1 -j ACCEPT") {
		t.Fatal("foreign FORWARD rule was disturbed")
	}
	// The legacy 10.200.0.0/16 deny is what failed: prove the regression case
	// is in the set (ordinal 256 lives in 10.201).
	if taps[2].guest != "10.201.0.2" {
		t.Fatalf("ordinal 256 guest = %s, want 10.201.0.2", taps[2].guest)
	}
}

// The old rule set, replayed against the same verdict check, leaks: this is
// the defect the fix closes (kept so the test proves it can fail).
func TestLegacyPerTapDenyLeaksAcrossSlash16(t *testing.T) {
	ipt := newFakeIPTables(t)
	taps := tapsAtOrdinals(t, 0, 256)
	for _, tp := range taps {
		for _, spec := range [][]string{
			{"-I", "FORWARD", "1", "-i", tp.name, "-d", "10.200.0.0/16", "-j", "DROP"},
			{"-I", "FORWARD", "1", "-i", tp.name, "!", "-s", tp.guest + "/32", "-j", "DROP"},
			{"-A", "FORWARD", "-i", tp.name, "-j", "ACCEPT"},
			{"-A", "FORWARD", "-o", tp.name, "-j", "ACCEPT"},
		} {
			if _, err := ipt.run(spec...); err != nil {
				t.Fatal(err)
			}
		}
	}
	if v := ipt.verdict("FORWARD", crossGuest(taps[0], taps[1])); v != "ACCEPT" {
		t.Fatalf("legacy rules: 10.200 -> 10.201 verdict %q; the harness no longer reproduces the leak", v)
	}
}

// F5, F7, F8, F15, F17 against the real Node B rule set.
func TestReconcileAtStartSweepsOnlyOrphansAndKeepsLiveGuestsOnline(t *testing.T) {
	ipt := newFakeIPTables(t)
	ipt.loadFixture(nodeBFixture)
	// Foreign and look-alike rules that must survive.
	foreign := `*filter
-A FORWARD -i br0 -o eno1 -j ACCEPT
-A FORWARD -i vm-custom-iface -j ACCEPT
-A FORWARD -i vmbr0 -j ACCEPT
-A FORWARD -i vm-abcdefghijkl -m comment --comment operator-debug -j ACCEPT
-A FORWARD -m comment --comment go-choir-vmx -j ACCEPT
-A INPUT -i vmbr0 -p tcp -m tcp --dport 8084 -j ACCEPT
*nat
-A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE
`
	ipt.loadString(foreign)
	before := ipt.dump()
	links := map[string]string{"eno1": "203.0.113.10/24", "br0": "", "vmbr0": ""}
	for name, addr := range nodeBLiveTaps {
		links[name] = addr
	}
	ip := newFakeIP(links)
	h := newFakeHostNet(t, ipt, ip)

	live := []testTap{
		{name: "vm-dhdbzl4tzqi3", guest: "10.200.112.2"},
		{name: "vm-vh3spxieztky", guest: "10.200.113.2"},
		{name: "vm-lackej5pmie2", guest: "10.200.114.2"},
	}
	// F5: at every step, every live guest keeps egress.
	ipt.afterMutation = func(step []string) {
		for _, tp := range live {
			if v := ipt.verdict("FORWARD", egress(tp)); v != "ACCEPT" {
				t.Fatalf("after %v live guest %s egress verdict %q", step, tp.name, v)
			}
		}
	}
	h.reconcileAtStart()
	ipt.afterMutation = nil

	after := ipt.dump()
	for _, line := range strings.Split(before, "\n") {
		if !strings.HasPrefix(line, "-A ") {
			continue
		}
		owner := ruleOwnerTap(iptablesRuleFields(line)[2:])
		_, isLive := nodeBLiveTaps[owner]
		switch {
		case owner == "":
			if !strings.Contains(after, line+"\n") {
				t.Errorf("foreign rule removed: %s", line)
			}
		case isLive:
			if !strings.Contains(after, line+"\n") {
				t.Errorf("live tap rule removed by the sweep: %s", line)
			}
		default:
			if strings.Contains(after, line+"\n") {
				t.Errorf("orphan rule survived the sweep: %s", line)
			}
		}
	}
	deleted := 0
	for _, c := range ipt.mutations() {
		if (c[0] == "-D") || (len(c) > 2 && c[2] == "-D") {
			deleted++
		}
	}
	if deleted != 157 {
		t.Errorf("sweep deleted %d rules, want the 157 stale rules found on Node B", deleted)
	}
	// Live guests are isolated from each other once the base is in.
	assertIsolated(t, ipt, live)
	if pol := ipt.policy("filter", "FORWARD"); pol != "ACCEPT" {
		t.Fatalf("FORWARD policy changed: %q", pol)
	}
}

// F9.
func TestSweepDoesNothingWhenLinksAreUnknown(t *testing.T) {
	ipt := newFakeIPTables(t)
	ipt.loadFixture(nodeBFixture)
	ip := newFakeIP(nil)
	ip.failAll = true
	h := newFakeHostNet(t, ipt, ip)
	h.reconcileAtStart()
	for _, c := range ipt.mutations() {
		if c[0] == "-D" || (len(c) > 2 && c[2] == "-D") {
			t.Fatalf("sweep deleted %v without knowing which links exist", c)
		}
	}
	if n := h.sweepOrphans(map[string]bool{}); n != 0 {
		t.Fatalf("sweepOrphans(empty) deleted %d rules; an empty link set must be refused", n)
	}
}

// F6: rebuilding a stale base chain never opens a window.
func TestEnsureBaseRebuildIsFailClosed(t *testing.T) {
	ipt := newFakeIPTables(t)
	// A stale base from an older build: /16 deny only, and in the wrong place.
	ipt.loadString(`*filter
-N CHOIR-GUEST-FWD
-N CHOIR-GUEST-TAPS
-A FORWARD -i vm-+ -m comment --comment go-choir-guest -j CHOIR-GUEST-FWD
-A CHOIR-GUEST-FWD -m comment --comment go-choir-guest -j CHOIR-GUEST-TAPS
-A CHOIR-GUEST-FWD -d 10.200.0.0/16 -m comment --comment go-choir-guest -j DROP
-A CHOIR-GUEST-FWD -m comment --comment go-choir-guest -j DROP
`)
	taps := tapsAtOrdinals(t, 3, 300)
	for _, tp := range taps {
		if _, err := ipt.run("-A", "CHOIR-GUEST-TAPS", "-s", tp.guest+"/32", "-i", tp.name, "-m", "comment", "--comment", tapCommentPrefix+tp.name, "-j", "ACCEPT"); err != nil {
			t.Fatal(err)
		}
	}
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	ipt.afterMutation = func(step []string) {
		for _, a := range taps {
			for _, b := range taps {
				if a != b {
					if v := ipt.verdict("FORWARD", crossGuest(a, b)); v == "ACCEPT" || v == "policy ACCEPT" {
						t.Fatalf("after %v cross-guest %s->%s verdict %q (window opened)", step, a.guest, b.guest, v)
					}
				}
			}
		}
	}
	if err := h.ensureBase(); err != nil {
		t.Fatal(err)
	}
	ipt.afterMutation = nil
	assertIsolated(t, ipt, taps)
	got := ipt.rules("filter", guestForwardChain)
	want := []string{
		"-A CHOIR-GUEST-FWD -d 10.200.0.0/13 -m comment --comment go-choir-guest -j DROP",
		"-A CHOIR-GUEST-FWD -d 10.208.0.0/13 -m comment --comment go-choir-guest -j DROP",
		"-A CHOIR-GUEST-FWD -m comment --comment go-choir-guest -j CHOIR-GUEST-TAPS",
		"-A CHOIR-GUEST-FWD -m comment --comment go-choir-guest -j DROP",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("base chain = %#v, want %#v", got, want)
	}
}

// F12.
func TestHostNetSetupIsIdempotent(t *testing.T) {
	ipt := newFakeIPTables(t)
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	tp := tapsAtOrdinals(t, 300)[0]
	setup := func() {
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatal(err)
		}
		if err := h.ensureNAT(tp.name, tp.host, tp.guest, 9300, 8085); err != nil {
			t.Fatal(err)
		}
	}
	setup()
	first := ipt.dump()
	mutationsAfterFirst := len(ipt.mutations())
	setup()
	if second := ipt.dump(); second != first {
		t.Fatalf("second setup changed the rules:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if extra := ipt.mutations()[mutationsAfterFirst:]; len(extra) != 0 {
		t.Fatalf("second setup issued mutations: %v", extra)
	}
}

// F10, F11.
func TestRemoveTapDeletesOnlyThatTapsRules(t *testing.T) {
	ipt := newFakeIPTables(t)
	ipt.loadString(`*filter
-A INPUT -i vmbr0 -p tcp -m tcp --dport 8084 -j ACCEPT
-A FORWARD -i br0 -o eno1 -j ACCEPT
`)
	ip := newFakeIP(nil)
	h := newFakeHostNet(t, ipt, ip)
	taps := tapsAtOrdinals(t, 1, 2, 700)
	for i, tp := range taps {
		if err := h.ensureTap(tp.name, tp.guest); err != nil {
			t.Fatal(err)
		}
		if err := h.ensureNAT(tp.name, tp.host, tp.guest, 9000+i, 8085); err != nil {
			t.Fatal(err)
		}
	}
	gone := taps[1]
	h.removeTap(gone.name)
	dump := ipt.dump()
	if strings.Contains(dump, gone.name) || strings.Contains(dump, gone.host+"/32") || strings.Contains(dump, gone.guest+"/32") {
		t.Fatalf("rules for %s survived teardown:\n%s", gone.name, dump)
	}
	kept := []testTap{taps[0], taps[2]}
	for _, tp := range kept {
		if !strings.Contains(dump, tapCommentPrefix+tp.name) {
			t.Fatalf("teardown of %s removed %s's rules", gone.name, tp.name)
		}
	}
	assertIsolated(t, ipt, kept)
	for _, line := range []string{"-A INPUT -i vmbr0 -p tcp -m tcp --dport 8084 -j ACCEPT", "-A FORWARD -i br0 -o eno1 -j ACCEPT"} {
		if !strings.Contains(dump, line) {
			t.Fatalf("foreign rule removed by teardown: %s", line)
		}
	}
	// The removed tap's guest now has no ACCEPT: if the interface lingers,
	// it fails closed.
	if v := ipt.verdict("FORWARD", egress(gone)); v != "DROP" {
		t.Fatalf("torn-down tap egress verdict %q, want DROP", v)
	}
}

// F13, F14, F15: a live tap carrying the pre-fix rule set from Node B is
// migrated without a gap, and spoofed INPUT is refused afterwards.
func TestEnsureTapMigratesLegacyLiveTapRules(t *testing.T) {
	ipt := newFakeIPTables(t)
	ipt.loadFixture(nodeBFixture)
	h := newFakeHostNet(t, ipt, newFakeIP(nil))
	tp := testTap{name: "vm-lackej5pmie2", guest: "10.200.114.2", host: "10.200.114.1"}
	other := testTap{name: "vm-vh3spxieztky", guest: "10.200.113.2", host: "10.200.113.1"}
	toGateway := fakePacket{in: tp.name, src: tp.guest, dst: "127.0.0.1", proto: "tcp", dport: "8084"}
	spoofed := toGateway
	spoofed.src = other.guest

	// Before: the forged source reaches the gateway port (F14 on Node B).
	if v := ipt.verdict("INPUT", spoofed); v != "ACCEPT" {
		t.Fatalf("fixture no longer shows the INPUT ordering defect: %q", v)
	}
	ipt.afterMutation = func(step []string) {
		if v := ipt.verdict("INPUT", toGateway); v != "ACCEPT" {
			t.Fatalf("after %v guest lost the gateway port: %q", step, v)
		}
		if v := ipt.verdict("FORWARD", egress(tp)); v != "ACCEPT" {
			t.Fatalf("after %v guest lost egress: %q", step, v)
		}
		if v := ipt.verdict("FORWARD", crossGuest(tp, other)); v != "DROP" {
			t.Fatalf("after %v cross-guest verdict %q", step, v)
		}
		reply := fakePacket{in: "eno1", out: tp.name, src: "1.1.1.1", dst: tp.guest, proto: "tcp", dport: "50000", state: "ESTABLISHED"}
		if v := ipt.verdict("FORWARD", reply); v != "ACCEPT" && v != "policy ACCEPT" {
			t.Fatalf("after %v replies to the guest dropped: %q", step, v)
		}
	}
	if err := h.ensureTap(tp.name, tp.guest); err != nil {
		t.Fatal(err)
	}
	ipt.afterMutation = nil
	if v := ipt.verdict("INPUT", spoofed); v != "DROP" {
		t.Fatalf("forged source to the gateway port after migration: %q, want DROP", v)
	}
	var forward []string
	for _, line := range ipt.rules("filter", "FORWARD") {
		if ruleOwnerTap(iptablesRuleFields(line)[2:]) == tp.name {
			forward = append(forward, line)
		}
	}
	// SEC-03: replies go through CHOIR-GUEST-RET; no per-tap FORWARD rule
	// is left.
	if len(forward) != 0 {
		t.Fatalf("FORWARD rules for the migrated tap = %#v, want none", forward)
	}
	for _, line := range ipt.rules("filter", "INPUT") {
		if ruleOwnerTap(iptablesRuleFields(line)[2:]) == tp.name && strings.Contains(line, "-j ACCEPT") && !strings.Contains(line, "-s 10.200.114.2/32") {
			t.Fatalf("INPUT ACCEPT without the guest source survived: %s", line)
		}
	}
}

// F15: the keys the code compares are stable across iptables' own rendering
// (live Node B lines) and the specs the code emits.
func TestRuleKeysMatchLiveNodeBFormatting(t *testing.T) {
	tap, guest, comment := "vm-lackej5pmie2", "10.200.114.2", "go-choir-vm-vm-lackej5pmie2"
	for _, tc := range []struct {
		live string
		spec []string
	}{
		{"-A INPUT ! -s 10.200.114.2/32 -i vm-lackej5pmie2 -m comment --comment go-choir-vm-vm-lackej5pmie2 -j DROP", tapInputDropSpec(tap, guest)},
		{"-A PREROUTING -d 10.200.114.1/32 -p tcp -m tcp --dport 8084 -m comment --comment go-choir-vm-vm-lackej5pmie2 -j DNAT --to-destination 127.0.0.1:8084",
			[]string{"-p", "tcp", "--dport", "8084", "-d", "10.200.114.1", "-j", "DNAT", "--to-destination", "127.0.0.1:8084", "-m", "comment", "--comment", comment}},
		{"-A POSTROUTING -s 10.200.114.0/30 ! -d 10.200.114.0/30 -m comment --comment go-choir-vm-vm-lackej5pmie2 -j MASQUERADE",
			[]string{"-s", "10.200.114.2/30", "!", "-d", "10.200.114.2/30", "-j", "MASQUERADE", "-m", "comment", "--comment", comment}},
	} {
		if a, b := ruleKey(iptablesRuleFields(tc.live)[2:]), ruleKey(tc.spec); a != b {
			t.Errorf("key mismatch:\n live %q -> %s\n spec %v -> %s", tc.live, a, tc.spec, b)
		}
	}
	if ruleKey(tapInputDropSpec(tap, guest)) == ruleKey(tapInputDropSpec(tap, "10.200.114.6")) {
		t.Fatal("keys ignore the source address")
	}
}

func TestRuleOwnerTap(t *testing.T) {
	for _, tc := range []struct {
		line, want string
	}{
		{"-A FORWARD -i vm-vm-37e40-tap -j ACCEPT", "vm-vm-37e40-tap"},
		{"-A FORWARD -o vm-vm-37e40-tap -j ACCEPT", "vm-vm-37e40-tap"},
		{"-A INPUT -i vm-vm-eefb4-tap -p tcp -m tcp --dport 8084 -j ACCEPT", "vm-vm-eefb4-tap"},
		{"-A OUTPUT -d 127.0.0.1/32 -p tcp -m tcp --dport 9528 -m comment --comment go-choir-vm-vm-vm-5b0c1-tap -j DNAT --to-destination 10.202.16.2:8085", "vm-vm-5b0c1-tap"},
		{"-A INPUT ! -s 10.200.114.2/32 -i vm-lackej5pmie2 -m comment --comment go-choir-vm-vm-lackej5pmie2 -j DROP", "vm-lackej5pmie2"},
		{"-A CHOIR-GUEST-TAPS -s 10.201.0.2/32 -i vm-abcdefgh2345 -m comment --comment \"go-choir-vm-vm-abcdefgh2345\" -j ACCEPT", "vm-abcdefgh2345"},
		// Not ours.
		{"-A FORWARD -i vm-+ -m comment --comment go-choir-guest -j CHOIR-GUEST-FWD", ""},
		{"-A FORWARD -i vm-abcdefghijkl -m comment --comment operator-debug -j ACCEPT", ""},
		{"-A FORWARD -i vm-custom-iface -j ACCEPT", ""},
		{"-A FORWARD -i vmbr0 -j ACCEPT", ""},
		{"-A FORWARD -i vm-ABCDEFGHIJKL -j ACCEPT", ""},
		{"-A FORWARD -m comment --comment go-choir-vm-eno1 -j ACCEPT", ""},
		{"-A FORWARD ! -i vm-abcdefgh2345 -j ACCEPT", ""},
		{"-A INPUT -j nixos-fw", ""},
	} {
		if got := ruleOwnerTap(iptablesRuleFields(tc.line)[2:]); got != tc.want {
			t.Errorf("ruleOwnerTap(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// F10: a spawn failure after the network was prepared tears it down.
func TestLaunchFailureTearsDownTapNetwork(t *testing.T) {
	ipt := newFakeIPTables(t)
	ip := newFakeIP(nil)
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
	m.hostNet = newFakeHostNet(t, ipt, ip)
	m.firecrackerStarter = func(string, []string) error { return fmt.Errorf("spawn refused") }
	vmID := "vm-launch-failure"
	tap := tapNameForVMID(vmID)
	if err := m.launchFirecracker(VMConfig{VMID: vmID, GuestPort: 8085}, map[string]interface{}{}, 9300); err == nil {
		t.Fatal("launch succeeded with a failing starter")
	}
	if ip.has(tap) {
		t.Fatalf("tap %s survived a failed launch", tap)
	}
	if dump := ipt.dump(); strings.Contains(dump, tap) {
		t.Fatalf("rules for %s survived a failed launch:\n%s", tap, dump)
	}
}

// F10, F16: an exit vmctl did not cause tears the network down, but only
// while the exited process is still the instance's current process.
func TestFirecrackerExitTearsDownOnlyItsOwnNetwork(t *testing.T) {
	setup := func(t *testing.T) (*Manager, *fakeIPTables, *fakeIP, string, *VMInstance, *exec.Cmd) {
		ipt := newFakeIPTables(t)
		ip := newFakeIP(nil)
		m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
		m.hostNet = newFakeHostNet(t, ipt, ip)
		vmID := "vm-exit-teardown"
		m.prepareVMNetwork(vmID, 9301, 8085, -1)
		cmd := &exec.Cmd{}
		inst := &VMInstance{Config: VMConfig{VMID: vmID}, cmd: cmd, State: StateRunning}
		m.vms[vmID] = inst
		return m, ipt, ip, vmID, inst, cmd
	}
	t.Run("current process exits", func(t *testing.T) {
		m, ipt, ip, vmID, inst, cmd := setup(t)
		m.releaseExitedVMNetwork(vmID, inst, cmd)
		tap := tapNameForVMID(vmID)
		if ip.has(tap) || strings.Contains(ipt.dump(), tap) {
			t.Fatalf("crash left tap %s or its rules:\n%s", tap, ipt.dump())
		}
	})
	t.Run("instance relaunched", func(t *testing.T) {
		m, ipt, ip, vmID, inst, cmd := setup(t)
		inst.cmd = &exec.Cmd{}
		m.releaseExitedVMNetwork(vmID, inst, cmd)
		tap := tapNameForVMID(vmID)
		if !ip.has(tap) || !strings.Contains(ipt.dump(), tap) {
			t.Fatal("a stale exit tore down the relaunched VM's network")
		}
	})
	t.Run("instance replaced", func(t *testing.T) {
		m, ipt, ip, vmID, inst, cmd := setup(t)
		m.vms[vmID] = &VMInstance{Config: VMConfig{VMID: vmID}, cmd: cmd}
		m.releaseExitedVMNetwork(vmID, inst, cmd)
		tap := tapNameForVMID(vmID)
		if !ip.has(tap) || !strings.Contains(ipt.dump(), tap) {
			t.Fatal("a stale exit tore down a replacement instance's network")
		}
	})
	t.Run("killed by vmctl", func(t *testing.T) {
		m, ipt, ip, vmID, inst, cmd := setup(t)
		inst.cmd = nil // killFirecrackerProcess clears it and tears down itself
		m.releaseExitedVMNetwork(vmID, inst, cmd)
		if !ip.has(tapNameForVMID(vmID)) || !strings.Contains(ipt.dump(), tapNameForVMID(vmID)) {
			t.Fatal("exit after a vmctl kill tore down the network a second time")
		}
	})
}
