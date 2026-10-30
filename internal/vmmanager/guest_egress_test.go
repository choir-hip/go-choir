package vmmanager

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Guest egress policy (F6,
// docs/problems/desk-boundary-residuals-2026-10-21.md).
//
// Ways this could fail, written first:
//  1. the host service ports in the egress chain drift from
//     tapReachableHostServicePorts, so a service the guest needs is cut or
//     one it should not reach is opened;
//  2. replies to host-initiated flows (vmctl to guest :8085) are dropped
//     because ESTABLISHED,RELATED is not admitted first;
//  3. a host port outside the service list (sshd 22, Dolt 13306, caddy
//     admin 2019) stays reachable on any host address;
//  4. private, link-local, metadata, CGNAT, multicast or other guests'
//     addresses stay reachable through FORWARD;
//  5. SMTP stays reachable, arbitrary UDP or other protocols pass, or DNS
//     and NTP are cut;
//  6. a second ensure rewrites an equal chain (a fail-open window), or a
//     changed chain is left stale;
//  7. the jump is not first in mangle PREROUTING, or is duplicated;
//  8. IPv6 from guests is not dropped;
//
// SEC-03 (docs/problems/host-egress-floor-fails-open-2026-10-21.md), also
// written before the code:
//  9. the host's public front door (80/443) stays reachable from a guest;
//     it is not a guest service, and Caddy would see a guest as a private
//     source;
//  10. a guest reaches another tap's host address on a service port (the
//      host RETURNs must hold only for the address of the tap the packet
//      came in on);
//  11. DNS to the guest's own host resolver (UDP and TCP 53 on its tap
//      host address) is dropped;
//  12. an install or repair step fails and the guests are left open: a
//      rebuild that flushes first, an error that is logged, not returned;
//  13. a host without ip6tables is tolerated, so IPv6 to host services
//      (sshd on ::) stays open with no error.

// fakeNetfilter is a minimal in-memory iptables: per binary, per table,
// per chain, an ordered list of rule specs.
type fakeNetfilter struct {
	chains  map[string][]string // "bin/table/chain" -> rules
	calls   []string
	missing map[string]bool // binaries that fail every call
	// fail makes a matching call ("bin -t table -X chain spec") fail.
	fail func(call string) bool
	// afterMutation runs after every successful rule change (invariants).
	afterMutation func(call string)
}

func newFakeNetfilter() *fakeNetfilter {
	return &fakeNetfilter{chains: map[string][]string{
		"iptables/mangle/PREROUTING":  {"-i eth0 -j MARK --set-mark 7"},
		"ip6tables/mangle/PREROUTING": {},
	}, missing: map[string]bool{}}
}

func (f *fakeNetfilter) run(bin string, args ...string) ([]byte, error) {
	call := bin + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.missing[bin] {
		return nil, fmt.Errorf("exec: %q: executable file not found", bin)
	}
	if f.fail != nil && f.fail(call) {
		return []byte("injected failure"), fmt.Errorf("exit status 1")
	}
	out, err := f.apply(bin, args)
	if err == nil && f.afterMutation != nil {
		switch {
		case strings.Contains(call, " -A "), strings.Contains(call, " -I "), strings.Contains(call, " -D "), strings.Contains(call, " -F "):
			f.afterMutation(call)
		}
	}
	return out, err
}

func (f *fakeNetfilter) apply(bin string, args []string) ([]byte, error) {
	table := "filter"
	if len(args) >= 2 && args[0] == "-t" {
		table, args = args[1], args[2:]
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("bad args %v", args)
	}
	op, chain, rest := args[0], args[1], args[2:]
	key := bin + "/" + table + "/" + chain
	rules, exists := f.chains[key]
	spec := strings.Join(rest, " ")
	switch op {
	case "-N":
		if exists {
			return []byte("iptables: Chain already exists."), fmt.Errorf("exit status 1")
		}
		f.chains[key] = []string{}
	case "-S":
		if !exists {
			return nil, fmt.Errorf("No chain/target/match by that name")
		}
		out := "-N " + chain + "\n"
		for _, r := range rules {
			out += "-A " + chain + " " + r + "\n"
		}
		return []byte(out), nil
	case "-C":
		if !exists || !slices.Contains(rules, spec) {
			return nil, fmt.Errorf("exit status 1")
		}
	case "-A":
		if !exists {
			return nil, fmt.Errorf("no chain %s", key)
		}
		f.chains[key] = append(rules, spec)
	case "-I":
		if !exists {
			return nil, fmt.Errorf("no chain %s", key)
		}
		pos, err := strconv.Atoi(rest[0])
		if err != nil || pos < 1 || pos > len(rules)+1 {
			return nil, fmt.Errorf("fake -I needs a valid position: %v", rest)
		}
		spec = strings.Join(rest[1:], " ")
		next := append([]string{}, rules[:pos-1]...)
		next = append(next, spec)
		f.chains[key] = append(next, rules[pos-1:]...)
	case "-D":
		if !exists {
			return nil, fmt.Errorf("no chain %s", key)
		}
		idx := slices.Index(rules, spec)
		if len(rest) == 1 {
			if n, err := strconv.Atoi(rest[0]); err == nil {
				idx = n - 1
				if idx < 0 || idx >= len(rules) {
					return nil, fmt.Errorf("index of deletion too big")
				}
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("exit status 1")
		}
		f.chains[key] = append(append([]string{}, rules[:idx]...), rules[idx+1:]...)
	case "-F":
		if !exists {
			return nil, fmt.Errorf("no chain %s", key)
		}
		f.chains[key] = []string{}
	default:
		return nil, fmt.Errorf("fake does not support %s", op)
	}
	return nil, nil
}

func (f *fakeNetfilter) mutations() int {
	n := 0
	for _, c := range f.calls {
		for _, op := range []string{" -A ", " -I ", " -F ", " -D "} {
			if strings.Contains(c, op) {
				n++
			}
		}
	}
	return n
}

func TestGuestEgressChainShape(t *testing.T) {
	rules := guestEgressRuleSpecs()
	joined := make([]string, len(rules))
	for i, r := range rules {
		joined[i] = strings.Join(r, " ")
	}
	// (2) replies first.
	if !strings.Contains(joined[0], "--ctstate ESTABLISHED,RELATED") || !strings.HasSuffix(joined[0], "-j RETURN") {
		t.Fatalf("first rule %q does not admit established flows", joined[0])
	}
	// (1) one source for the host service ports; (10) only on the tap's own
	// address.
	want := strings.Join(tapReachableHostServicePorts(), ",")
	if !slices.ContainsFunc(joined, func(r string) bool {
		return strings.Contains(r, "--dst-type LOCAL --limit-iface-in") && strings.Contains(r, "--dports "+want) && strings.HasSuffix(r, "-j RETURN")
	}) {
		t.Fatalf("no own-tap LOCAL RETURN for the tap service ports %s in %q", want, joined)
	}
	for _, r := range joined {
		if strings.Contains(r, "--dst-type LOCAL") && strings.HasSuffix(r, "-j RETURN") && !strings.Contains(r, "--limit-iface-in") {
			t.Fatalf("(10) host RETURN %q is not limited to the incoming tap", r)
		}
	}
	// (11) DNS to the guest's own host resolver.
	for _, proto := range []string{"udp", "tcp"} {
		rule := "-p " + proto + " -m addrtype --dst-type LOCAL --limit-iface-in -m " + proto + " --dport 53 -m comment --comment " + guestEgressComment + " -j RETURN"
		if !slices.Contains(joined, rule) {
			t.Errorf("(11) no %s DNS RETURN to the tap host: want %q in %q", proto, rule, joined)
		}
	}
	// (3) every other local port drops, after the admitted ones.
	localDrop := slices.IndexFunc(joined, func(r string) bool {
		return strings.Contains(r, "--dst-type LOCAL") && strings.HasSuffix(r, "-j DROP") && !strings.Contains(r, "--dports")
	})
	if localDrop < 0 {
		t.Fatalf("no catch-all DROP for host addresses in %q", joined)
	}
	for i, r := range joined {
		if strings.Contains(r, "--dst-type LOCAL") && strings.HasSuffix(r, "-j RETURN") && i > localDrop {
			t.Fatalf("a host RETURN %q follows the host DROP", r)
		}
		for _, port := range []string{"22", "80", "443", "2019", "13306", "13307", "19090", "6333", "8888"} {
			if strings.Contains(r, "--dst-type LOCAL") && strings.HasSuffix(r, "-j RETURN") && slices.Contains(strings.Split(dportsOf(r), ","), port) {
				t.Fatalf("host port %s admitted by %q", port, r)
			}
		}
	}
	// (4) blocked destination ranges.
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "169.254.0.0/16", "127.0.0.0/8", "0.0.0.0/8", "224.0.0.0/4", "240.0.0.0/4"} {
		if !slices.Contains(joined, "-d "+cidr+" -m comment --comment "+guestEgressComment+" -j DROP") {
			t.Errorf("no DROP for %s", cidr)
		}
	}
	// (5) SMTP drops before TCP returns; UDP only DNS and NTP; ICMP; the rest drops.
	smtp := slices.IndexFunc(joined, func(r string) bool {
		return strings.Contains(r, "-p tcp") && strings.Contains(r, "25,465,587") && strings.HasSuffix(r, "-j DROP")
	})
	tcp := slices.Index(joined, "-p tcp -m comment --comment "+guestEgressComment+" -j RETURN")
	if smtp < 0 || tcp < 0 || smtp > tcp {
		t.Fatalf("SMTP drop (%d) must precede the TCP return (%d): %q", smtp, tcp, joined)
	}
	if !slices.Contains(joined, "-p udp -m multiport --dports 53,123 -m comment --comment "+guestEgressComment+" -j RETURN") {
		t.Errorf("DNS/NTP not admitted: %q", joined)
	}
	if !slices.Contains(joined, "-p icmp -m comment --comment "+guestEgressComment+" -j RETURN") {
		t.Errorf("ICMP not admitted: %q", joined)
	}
	if last := joined[len(joined)-1]; last != "-m comment --comment "+guestEgressComment+" -j DROP" {
		t.Errorf("last rule %q is not a catch-all DROP", last)
	}
	// Every blocking rule precedes the protocol returns.
	for i, r := range joined {
		if strings.HasPrefix(r, "-d ") && i > tcp {
			t.Errorf("range rule %q follows the TCP return", r)
		}
	}
}

func dportsOf(rule string) string {
	f := strings.Fields(rule)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "--dports" || f[i] == "--dport" {
			return f[i+1]
		}
	}
	return ""
}

func TestGuestEgressEnsureInstallsOnceAndRepairs(t *testing.T) {
	f := newFakeNetfilter()
	g := &guestEgressInstaller{run: f.run, ipt: "iptables", ip6t: "ip6tables"}
	if err := g.ensure(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	pre := f.chains["iptables/mangle/PREROUTING"]
	jump := strings.Join(guestEgressJumpSpec(), " ")
	if len(pre) != 2 || pre[0] != jump {
		t.Fatalf("(7) mangle PREROUTING = %q, want the jump first and the existing rule kept", pre)
	}
	chain := f.chains["iptables/mangle/"+guestEgressChain]
	if len(chain) != len(guestEgressRuleSpecs()) {
		t.Fatalf("chain has %d rules, want %d", len(chain), len(guestEgressRuleSpecs()))
	}
	if v6 := f.chains["ip6tables/mangle/PREROUTING"]; len(v6) != 1 || v6[0] != strings.Join(guestIPv6DropSpec(), " ") {
		t.Fatalf("(8) ip6 mangle PREROUTING = %q, want the guest drop", v6)
	}

	// (6) an equal policy is not rewritten.
	f.calls = nil
	if err := g.ensure(); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if n := f.mutations(); n != 0 {
		t.Fatalf("second ensure mutated netfilter %d times: %q", n, f.calls)
	}
	if got := f.chains["iptables/mangle/PREROUTING"]; len(got) != 2 {
		t.Fatalf("(7) jump duplicated: %q", got)
	}

	// (6) a stale chain (an older policy, a hand edit) is rebuilt in order.
	f.chains["iptables/mangle/"+guestEgressChain] = append([]string{"-j RETURN"}, chain[1:]...)
	if err := g.ensure(); err != nil {
		t.Fatalf("repair ensure: %v", err)
	}
	if got := f.chains["iptables/mangle/"+guestEgressChain]; !slices.Equal(got, chain) {
		t.Fatalf("stale chain not rebuilt: %q", got)
	}
}

// (14) The policy is in place from vmctl start, before any guest is
// reattached, and every tap setup re-ensures it. A hostNet without an
// installer cannot set a tap up (12): it never runs a guest without the
// policy.
func TestGuestEgressEnsuredAtStartAndOnTapSetup(t *testing.T) {
	ipt := newFakeIPTables(t)
	h := newFakeHostNet(t, ipt, newFakeIP(map[string]string{}))
	h.egress = nil
	h.reconcileAtStart() // no installer: logs, must not panic
	noPolicy := &Manager{hostNet: h}
	noPolicy.hostNetOnce.Do(func() {})
	if err := noPolicy.setupHostNetworking("vm-abcdefghijkl", "10.200.0.1", 12000, "10.200.0.2", 8085); err == nil {
		t.Fatal("(12) tap set up with no egress policy installer")
	}
	f := newFakeNetfilter()
	h.egress = &guestEgressInstaller{run: f.run, ipt: "iptables", ip6t: "ip6tables"}
	h.reconcileAtStart()
	if len(f.chains["iptables/mangle/"+guestEgressChain]) != len(guestEgressRuleSpecs()) {
		t.Fatalf("egress chain not installed at start: %q", f.chains["iptables/mangle/"+guestEgressChain])
	}
	f.chains["iptables/mangle/PREROUTING"] = nil // lost (a firewall reload)
	m := &Manager{hostNet: h}
	m.hostNetOnce.Do(func() {})
	if err := m.setupHostNetworking("vm-abcdefghijkl", "10.200.0.1", 12000, "10.200.0.2", 8085); err != nil {
		t.Fatalf("setupHostNetworking: %v", err)
	}
	if pre := f.chains["iptables/mangle/PREROUTING"]; len(pre) != 1 || pre[0] != strings.Join(guestEgressJumpSpec(), " ") {
		t.Fatalf("tap setup did not restore the egress jump: %q", pre)
	}
}

// (13) No ip6tables is an error: the IPv6 drop is part of the floor.
func TestGuestEgressEnsureWithoutIP6TablesFails(t *testing.T) {
	f := newFakeNetfilter()
	f.missing["ip6tables"] = true
	g := &guestEgressInstaller{run: f.run, ipt: "iptables", ip6t: "ip6tables"}
	if err := g.ensure(); err == nil {
		t.Fatal("(13) ensure succeeded with no ip6tables: guest IPv6 to host services stays open")
	}
}

// (12) A stale chain is rebuilt behind a DROP guard: at no step does the
// chain admit more than the old or the new policy, and a failed step
// leaves the guard (guests fail closed) and returns the error.
func TestGuestEgressRebuildIsGuarded(t *testing.T) {
	want := make([]string, 0, len(guestEgressRuleSpecs()))
	for _, spec := range guestEgressRuleSpecs() {
		want = append(want, strings.Join(spec, " "))
	}
	guard := strings.Join(guestEgressGuardSpec(), " ")
	key := "iptables/mangle/" + guestEgressChain
	for _, tc := range []struct {
		name  string
		stale func([]string) []string
	}{
		{"older policy", func(w []string) []string { return append([]string{"-j RETURN"}, w[1:]...) }},
		{"flushed under a live jump", func([]string) []string { return []string{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeNetfilter()
			g := &guestEgressInstaller{run: f.run, ipt: "iptables", ip6t: "ip6tables"}
			if err := g.ensure(); err != nil {
				t.Fatal(err)
			}
			stale := tc.stale(want)
			f.chains[key] = append([]string{}, stale...)
			f.afterMutation = func(call string) {
				got := f.chains[key]
				if !strings.Contains(call, guestEgressChain) {
					return
				}
				if slices.Equal(got, want) || (len(got) > 0 && got[0] == guard) {
					return
				}
				t.Fatalf("after %q the chain is neither guarded nor the policy: %q", call, got)
			}
			if err := g.ensure(); err != nil {
				t.Fatalf("repair: %v", err)
			}
			if got := f.chains[key]; !slices.Equal(got, want) {
				t.Fatalf("not rebuilt: %q", got)
			}

			// A failed append leaves the guard in place and is returned.
			f.afterMutation = nil
			f.chains[key] = append([]string{}, stale...)
			appends := 0
			f.fail = func(call string) bool {
				if strings.Contains(call, " -A "+guestEgressChain+" ") {
					appends++
					return appends == 3
				}
				return false
			}
			if err := g.ensure(); err == nil {
				t.Fatal("(12) a failed append was not returned")
			}
			if got := f.chains[key]; len(got) == 0 || got[0] != guard {
				t.Fatalf("(12) failed rebuild left the chain unguarded: %q", got)
			}
		})
	}
}
