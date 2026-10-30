package vmmanager

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

// Guest egress policy (F6,
// docs/problems/desk-boundary-residuals-2026-10-21.md).
//
// Every packet a guest sends enters the host on its tap (vm-*). Before nat
// and filter see it, mangle PREROUTING sends it through CHOIR-GUEST-EGRESS:
//
//	ESTABLISHED,RELATED                      RETURN  replies to host->guest flows
//	to its own tap's host address,
//	  TCP service ports                      RETURN  tapReachableHostServicePorts
//	  UDP and TCP 53                         RETURN  the host resolver (SEC-10)
//	to a host address, anything else         DROP    sshd, Caddy 80/443, Dolt,
//	                                                 another tap's host address
//	to private/link-local/metadata/CGNAT/
//	  loopback/multicast/reserved ranges     DROP    includes every other guest
//	TCP 25/465/587                           DROP    no mail from Node B's address
//	TCP                                      RETURN  research, sources, packages
//	UDP 53, 123                              RETURN  DNS (resolved), NTP
//	ICMP                                     RETURN
//	anything else                            DROP
//
// RETURN hands the packet on unchanged to the existing per-tap rules (DNAT,
// INPUT, FORWARD, MASQUERADE), which keep doing what they do. The chain only
// ever narrows. IPv6 from guests is dropped outright: guests have no IPv6
// route, and sshd listens on :: (reachable over link-local).
//
// Why the guest side is not the place: a compromised guest root, the case
// this exists for, flushes guest-side rules. Why mangle: the filter-table
// guest rules (hostfirewall.go: CHOIR-GUEST-FWD/TAPS, the per-tap INPUT
// rules) insert at position 1 of INPUT and FORWARD; a separate table cannot
// be reordered by them, and mangle PREROUTING sees the guest's original
// destination before nat rewrites the service ports to 127.0.0.1. The two
// compose: this chain only drops, and what it returns the filter chains
// still decide (guest-to-guest is dropped by both, on purpose).
//
// What it does not do: the internet is still any public TCP destination,
// because research reads any public URL from the guest. A host allowlist
// needs the recording proxy (S4) and NET-01's host fetch service.
//
// SEC-03 (docs/problems/host-egress-floor-fails-open-2026-10-21.md): the
// public front door (80/443) is no longer a guest destination on any host
// address; host RETURNs hold only for the address of the tap the packet
// came in on (addrtype --limit-iface-in); and every failure is returned,
// so a VM whose policy did not install does not start. A rebuild runs
// behind a DROP guard: guests lose packets for its duration, never gain
// reach.

const (
	guestEgressChain        = "CHOIR-GUEST-EGRESS"
	guestEgressComment      = "go-choir-guest-egress"
	guestEgressGuardComment = "go-choir-guest-egress-rebuild"
)

// guestBlockedDestinations are never reachable from a guest. They are a
// superset of sourcefetch.AddressBlocked's IPv4 ranges.
var guestBlockedDestinations = []string{
	"0.0.0.0/8",       // this network
	"10.0.0.0/8",      // private, and every guest's /30
	"100.64.0.0/10",   // CGNAT
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local, provider metadata (169.254.169.254)
	"172.16.0.0/12",   // private
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"192.168.0.0/16",  // private
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reserved, broadcast
}

func withEgressComment(spec ...string) []string {
	return append(spec, "-m", "comment", "--comment", guestEgressComment)
}

// guestEgressRuleSpecs is the chain, in order, as `iptables -A` specs.
func guestEgressRuleSpecs() [][]string {
	services := strings.Join(tapReachableHostServicePorts(), ",")
	rules := [][]string{
		append(withEgressComment("-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED"), "-j", "RETURN"),
		append(withEgressComment("-p", "tcp", "-m", "addrtype", "--dst-type", "LOCAL", "--limit-iface-in", "-m", "multiport", "--dports", services), "-j", "RETURN"),
		append(withEgressComment("-p", "udp", "-m", "addrtype", "--dst-type", "LOCAL", "--limit-iface-in", "-m", "udp", "--dport", "53"), "-j", "RETURN"),
		append(withEgressComment("-p", "tcp", "-m", "addrtype", "--dst-type", "LOCAL", "--limit-iface-in", "-m", "tcp", "--dport", "53"), "-j", "RETURN"),
		append(withEgressComment("-m", "addrtype", "--dst-type", "LOCAL"), "-j", "DROP"),
	}
	for _, cidr := range guestBlockedDestinations {
		rules = append(rules, append(withEgressComment("-d", cidr), "-j", "DROP"))
	}
	rules = append(rules,
		append(withEgressComment("-p", "tcp", "-m", "multiport", "--dports", "25,465,587"), "-j", "DROP"),
		append(withEgressComment("-p", "tcp"), "-j", "RETURN"),
		append(withEgressComment("-p", "udp", "-m", "multiport", "--dports", "53,123"), "-j", "RETURN"),
		append(withEgressComment("-p", "icmp"), "-j", "RETURN"),
		append(withEgressComment(), "-j", "DROP"),
	)
	return rules
}

func guestEgressJumpSpec() []string {
	return append(withEgressComment("-i", guestIfaceWildcardEgress), "-j", guestEgressChain)
}

func guestIPv6DropSpec() []string {
	return append(withEgressComment("-i", guestIfaceWildcardEgress), "-j", "DROP")
}

func guestEgressGuardSpec() []string {
	return []string{"-m", "comment", "--comment", guestEgressGuardComment, "-j", "DROP"}
}

// guestIfaceWildcardEgress matches every tap this package creates
// (tapNameForVMID: "vm-" + base32).
const guestIfaceWildcardEgress = "vm-+"

type netfilterRunner func(bin string, args ...string) ([]byte, error)

type guestEgressInstaller struct {
	mu   sync.Mutex
	run  netfilterRunner
	ipt  string
	ip6t string
}

// ensureGuestEgressPolicy re-ensures the policy for guests already running
// (reattach). A failure is logged and the guest keeps running: a computer
// that is on stays on; the next VM start fails closed on the same error
// (setupHostNetworking).
func (m *Manager) ensureGuestEgressPolicy() {
	if err := m.net().ensureGuestEgress(); err != nil {
		log.Printf("vmmanager: WARNING: guest egress policy not repaired for running guests: %v", err)
	}
}

// ensureGuestEgress installs or repairs the policy. It runs at vmctl start
// (reconcileAtStart), before every tap is set up and for every guest
// reattached, so a lost chain or an older policy is corrected while guests
// run. A hostNet without an installer has no policy: that is an error, so
// no tap is set up without one.
func (h *hostNet) ensureGuestEgress() error {
	if h == nil || h.egress == nil {
		return fmt.Errorf("guest egress policy installer missing")
	}
	return h.egress.ensure()
}

func (g *guestEgressInstaller) ensure() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if out, err := g.run(g.ipt, "-t", "mangle", "-N", guestEgressChain); err != nil && !strings.Contains(string(out), "exists") {
		return fmt.Errorf("create %s: %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
	}
	if !g.chainMatches() {
		if err := g.rebuildLocked(); err != nil {
			return err
		}
	}
	jump := guestEgressJumpSpec()
	if _, err := g.run(g.ipt, append([]string{"-t", "mangle", "-C", "PREROUTING"}, jump...)...); err != nil {
		if out, err := g.run(g.ipt, append([]string{"-t", "mangle", "-I", "PREROUTING", "1"}, jump...)...); err != nil {
			return fmt.Errorf("jump to %s: %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
		}
	}
	// IPv6 is part of the floor: sshd listens on :: and a guest root can
	// bring up link-local on its NIC.
	drop6 := guestIPv6DropSpec()
	if _, err := g.run(g.ip6t, append([]string{"-t", "mangle", "-C", "PREROUTING"}, drop6...)...); err != nil {
		if out, err := g.run(g.ip6t, append([]string{"-t", "mangle", "-I", "PREROUTING", "1"}, drop6...)...); err != nil {
			return fmt.Errorf("guest IPv6 drop: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// rebuildLocked replaces the chain's content behind a DROP guard: guard at
// position 1, old rules deleted from position 2, the policy appended, the
// guard removed. A failed step returns with the guard still in place, so
// guests lose packets, never gain reach.
func (g *guestEgressInstaller) rebuildLocked() error {
	out, err := g.run(g.ipt, "-t", "mangle", "-S", guestEgressChain)
	if err != nil {
		return fmt.Errorf("list %s: %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
	}
	old := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-A ") {
			old++
		}
	}
	guard := guestEgressGuardSpec()
	if out, err := g.run(g.ipt, append([]string{"-t", "mangle", "-I", guestEgressChain, "1"}, guard...)...); err != nil {
		return fmt.Errorf("guard %s: %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
	}
	for i := 0; i < old; i++ {
		if out, err := g.run(g.ipt, "-t", "mangle", "-D", guestEgressChain, "2"); err != nil {
			return fmt.Errorf("clear %s (guard stays): %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
		}
	}
	for _, spec := range guestEgressRuleSpecs() {
		args := append([]string{"-t", "mangle", "-A", guestEgressChain}, spec...)
		if out, err := g.run(g.ipt, args...); err != nil {
			return fmt.Errorf("append to %s (guard stays): %v (%s) [%s]", guestEgressChain, err, strings.TrimSpace(string(out)), strings.Join(spec, " "))
		}
	}
	if out, err := g.run(g.ipt, append([]string{"-t", "mangle", "-D", guestEgressChain}, guard...)...); err != nil {
		return fmt.Errorf("remove %s guard: %v (%s)", guestEgressChain, err, strings.TrimSpace(string(out)))
	}
	log.Printf("vmmanager: installed guest egress policy %s", guestEgressChain)
	return nil
}

// chainMatches: the chain holds exactly the policy's rules. iptables -C
// compares semantically (it does not care how -S prints a rule); the count
// rules out extra rules. Order is not re-checked: only a hand edit could
// reorder an equal set.
func (g *guestEgressInstaller) chainMatches() bool {
	out, err := g.run(g.ipt, "-t", "mangle", "-S", guestEgressChain)
	if err != nil {
		return false
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-A ") {
			n++
		}
	}
	want := guestEgressRuleSpecs()
	if n != len(want) {
		return false
	}
	for _, spec := range want {
		if _, err := g.run(g.ipt, append([]string{"-t", "mangle", "-C", guestEgressChain}, spec...)...); err != nil {
			return false
		}
	}
	return true
}

// remove takes the policy out (rollback and tests): the jump, the chain,
// the IPv6 drop.
func (g *guestEgressInstaller) remove() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := 0; i < 8; i++ {
		if _, err := g.run(g.ipt, append([]string{"-t", "mangle", "-D", "PREROUTING"}, guestEgressJumpSpec()...)...); err != nil {
			break
		}
	}
	_, _ = g.run(g.ipt, "-t", "mangle", "-F", guestEgressChain)
	_, _ = g.run(g.ipt, "-t", "mangle", "-X", guestEgressChain)
	for i := 0; i < 8; i++ {
		if _, err := g.run(g.ip6t, append([]string{"-t", "mangle", "-D", "PREROUTING"}, guestIPv6DropSpec()...)...); err != nil {
			break
		}
	}
}

func runCombinedOutput(bin string, args ...string) ([]byte, error) {
	return exec.Command(bin, args...).CombinedOutput()
}
