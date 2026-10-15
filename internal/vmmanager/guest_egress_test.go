package vmmanager

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Guest egress policy (F6,
// docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md).
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
//  8. IPv6 from guests is not dropped, or a host without ip6tables fails
//     the whole ensure.

// fakeNetfilter is a minimal in-memory iptables: per binary, per table,
// per chain, an ordered list of rule specs.
type fakeNetfilter struct {
	chains  map[string][]string // "bin/table/chain" -> rules
	calls   []string
	missing map[string]bool // binaries that fail every call
}

func newFakeNetfilter() *fakeNetfilter {
	return &fakeNetfilter{chains: map[string][]string{
		"iptables/mangle/PREROUTING":  {"-i eth0 -j MARK --set-mark 7"},
		"ip6tables/mangle/PREROUTING": {},
	}, missing: map[string]bool{}}
}

func (f *fakeNetfilter) run(bin string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, bin+" "+strings.Join(args, " "))
	if f.missing[bin] {
		return nil, fmt.Errorf("exec: %q: executable file not found", bin)
	}
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
		pos := rest[0]
		spec = strings.Join(rest[1:], " ")
		if pos != "1" {
			return nil, fmt.Errorf("fake supports -I 1 only")
		}
		f.chains[key] = append([]string{spec}, rules...)
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
	// (1) one source for the host service ports.
	want := strings.Join(tapReachableHostServicePorts(), ",")
	if !slices.ContainsFunc(joined, func(r string) bool {
		return strings.Contains(r, "--dst-type LOCAL") && strings.Contains(r, "--dports "+want) && strings.HasSuffix(r, "-j RETURN")
	}) {
		t.Fatalf("no LOCAL RETURN for the tap service ports %s in %q", want, joined)
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
		for _, port := range []string{"22", "2019", "13306", "13307", "19090", "6333", "8888"} {
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
// reattached, and every tap setup re-ensures it; a hostNet without an
// installer (the fakes) runs nothing against the host.
func TestGuestEgressEnsuredAtStartAndOnTapSetup(t *testing.T) {
	ipt := newFakeIPTables(t)
	h := newFakeHostNet(t, ipt, newFakeIP(map[string]string{}))
	h.reconcileAtStart() // no installer: must not panic or run anything
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

func TestGuestEgressEnsureWithoutIP6Tables(t *testing.T) {
	f := newFakeNetfilter()
	f.missing["ip6tables"] = true
	g := &guestEgressInstaller{run: f.run, ipt: "iptables", ip6t: "ip6tables"}
	if err := g.ensure(); err != nil {
		t.Fatalf("(8) a host without ip6tables failed the IPv4 policy: %v", err)
	}
	if len(f.chains["iptables/mangle/"+guestEgressChain]) == 0 {
		t.Fatal("IPv4 policy not installed")
	}
}

// TestGuestEgressAdmitsEveryGuestToHostRoute: every host service the guest
// is told about must stay reachable through the egress chain. The routes are
// read from the guest's own kernel command line (choir.*_url, the source the
// guest image turns into RUNTIME_GATEWAY_URL, RUNTIME_VMCTL_URL and so on),
// plus the capsule Go module proxy on the gateway host
// (capsule.GoProxyHostPort, 8088). vmctl (8083) carries self-development
// routes today and the guest memory route on feat/memory-hotplug.
// Ways this could fail: a new choir.*_url is added without its port in
// tapReachableHostServicePorts, so the policy silently cuts the guest off.
func TestGuestEgressAdmitsEveryGuestToHostRoute(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"
	mgr := NewManager(cfg)
	fc := mgr.buildFirecrackerConfig(VMConfig{VMID: "vm-egress-routes", GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 512, Epoch: 1}, 9001)
	boot, _ := fc["boot-source"].(map[string]interface{})
	args, _ := boot["boot_args"].(string)
	ports := map[string]string{"goproxy (capsule.GoProxyHostPort)": "8088"}
	for _, arg := range strings.Fields(args) {
		key, value, ok := strings.Cut(arg, "=")
		if !ok || !strings.HasPrefix(key, "choir.") || !strings.HasSuffix(key, "_url") {
			continue
		}
		hostport := strings.TrimPrefix(strings.TrimPrefix(value, "http://"), "https://")
		hostport, _, _ = strings.Cut(hostport, "/")
		if i := strings.LastIndex(hostport, ":"); i >= 0 {
			ports[key] = hostport[i+1:]
		}
	}
	for _, want := range []string{"choir.gateway_url", "choir.vmctl_url", "choir.platform_url", "choir.proxy_url"} {
		if _, ok := ports[want]; !ok {
			t.Fatalf("kernel command line has no %s: %q", want, args)
		}
	}
	admitted := ""
	for _, r := range guestEgressRuleSpecs() {
		joined := strings.Join(r, " ")
		if strings.Contains(joined, "--dst-type LOCAL") && strings.HasSuffix(joined, "-j RETURN") && !strings.Contains(joined, "80,443") {
			admitted = dportsOf(joined)
		}
	}
	for route, port := range ports {
		if !slices.Contains(strings.Split(admitted, ","), port) {
			t.Errorf("guest route %s (port %s) is not admitted by the egress chain (admitted: %s)", route, port, admitted)
		}
	}
}
