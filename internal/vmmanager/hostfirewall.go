package vmmanager

import (
	"bytes"
	"fmt"
	"log"
	"math/bits"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Host firewall for guest taps (S1a boundary).
//
// Guest-originated forwarding is default-deny. FORWARD gets one jump, for
// every guest interface (vm-+), into a chain this code owns:
//
//	FORWARD           -i vm-+ -j CHOIR-GUEST-FWD                  (position 1)
//	CHOIR-GUEST-FWD   -d <each guest pool CIDR> -j DROP           (no guest->guest)
//	                  -j CHOIR-GUEST-TAPS
//	                  -j DROP                                     (fail closed)
//	CHOIR-GUEST-TAPS  -s <guest>/32 -i <tap> -j ACCEPT            (one per live tap)
//
// The pool CIDRs come from the allocator's range (vmSubnetSecondOctetBase,
// vmSubnetSecondOctetCount), so the deny always covers every address
// guestAndHostIP can hand out. A guest interface without its per-tap ACCEPT,
// or a packet with a forged source, reaches the tail DROP. The host's FORWARD
// policy is left alone: other services on the host may depend on it, and
// non-guest forwarding is not this code's to decide. The trade-off is that
// any interface whose name starts with "vm-" is treated as a guest; that
// prefix is this package's tap namespace (tapNameForVMID).
//
// Every rule this code writes for a tap carries the comment
// go-choir-vm-<tap>. Ownership for teardown and the start-up orphan sweep is
// decided by that comment or, for older untagged rules, by an interface name
// matching one of the two tap naming schemes this package has used. Nothing
// else on the host is touched.
//
// IPv6: guests get no routable IPv6 (only kernel link-local, never forwarded,
// and Node B runs with net.ipv6.conf.all.forwarding=0), so there is no
// ip6tables counterpart. If guests ever get v6, mirror this in ip6tables.

const (
	guestForwardChain   = "CHOIR-GUEST-FWD"
	guestTapsChain      = "CHOIR-GUEST-TAPS"
	guestBaseComment    = "go-choir-guest"
	guestRebuildComment = "go-choir-guest-rebuild"
	tapCommentPrefix    = "go-choir-vm-"
	guestIfaceWildcard  = "vm-+"
)

var (
	// tapNameForVMID: "vm-" + 12 lowercase base32 characters.
	currentTapNamePattern = regexp.MustCompile(`^vm-[a-z2-7]{12}$`)
	// The scheme before 1c9ba54b1: "vm-" + up to 8 bytes of the VM ID + "-tap".
	legacyTapNamePattern = regexp.MustCompile(`^vm-[A-Za-z0-9_.-]{1,8}-tap$`)
)

func isOwnedTapName(name string) bool {
	return currentTapNamePattern.MatchString(name) || legacyTapNamePattern.MatchString(name)
}

func tapComment(tap string) string { return tapCommentPrefix + tap }

// guestPoolCIDRs is the allocator's whole guest address range as the fewest
// aligned CIDR blocks.
func guestPoolCIDRs() []string {
	return cidrsForSecondOctetRange(vmSubnetSecondOctetBase, vmSubnetSecondOctetCount)
}

// cidrsForSecondOctetRange covers 10.base.0.0 through 10.(base+count-1).255.255
// exactly with aligned power-of-two blocks.
func cidrsForSecondOctetRange(base, count int) []string {
	var out []string
	start, end := base, base+count
	for start < end && start <= 255 {
		size := 1
		for size < 256 && start%(size*2) == 0 && start+size*2 <= end {
			size *= 2
		}
		prefix := 16 - (bits.Len(uint(size)) - 1)
		out = append(out, fmt.Sprintf("10.%d.0.0/%d", start, prefix))
		start += size
	}
	return out
}

func guestJumpSpec() []string {
	return []string{"-i", guestIfaceWildcard, "-m", "comment", "--comment", guestBaseComment, "-j", guestForwardChain}
}

func guestForwardChainSpecs(pool []string) [][]string {
	specs := make([][]string, 0, len(pool)+2)
	for _, cidr := range pool {
		specs = append(specs, []string{"-d", cidr, "-m", "comment", "--comment", guestBaseComment, "-j", "DROP"})
	}
	specs = append(specs,
		[]string{"-m", "comment", "--comment", guestBaseComment, "-j", guestTapsChain},
		[]string{"-m", "comment", "--comment", guestBaseComment, "-j", "DROP"},
	)
	return specs
}

func guestRebuildGuardSpec() []string {
	return []string{"-m", "comment", "--comment", guestRebuildComment, "-j", "DROP"}
}

// tapInputDropSpec: only the guest's own address may enter its tap (INPUT).
func tapInputDropSpec(tap, guestIP string) []string {
	return []string{"!", "-s", guestIP + "/32", "-i", tap, "-m", "comment", "--comment", tapComment(tap), "-j", "DROP"}
}

// tapHostServiceInputRuleSpec admits the guest, by its own source address,
// to one host service port. The source match makes the ACCEPT safe whatever
// its position relative to the anti-spoof DROP.
func tapHostServiceInputRuleSpec(tap, guestIP, port, comment string) []string {
	return []string{
		"-s", guestIP + "/32",
		"-i", tap,
		"-p", "tcp", "-m", "tcp", "--dport", port,
		"-m", "comment", "--comment", comment,
		"-j", "ACCEPT",
	}
}

// tapForwardAcceptSpec is the guest's egress ACCEPT inside CHOIR-GUEST-TAPS.
func tapForwardAcceptSpec(tap, guestIP string) []string {
	return []string{"-s", guestIP + "/32", "-i", tap, "-m", "comment", "--comment", tapComment(tap), "-j", "ACCEPT"}
}

// tapForwardReturnSpec admits forwarded traffic towards the guest (replies to
// its egress). Cross-guest packets never get here: they are dropped on the
// ingress side by CHOIR-GUEST-FWD.
func tapForwardReturnSpec(tap string) []string {
	return []string{"-o", tap, "-m", "comment", "--comment", tapComment(tap), "-j", "ACCEPT"}
}

type natRuleSpec struct {
	chain string
	spec  []string
}

func tapNATSpecs(tap, hostIP, guestIP string, hostPort, guestPort int) []natRuleSpec {
	comment := tapComment(tap)
	subnet := guestIP + "/30"
	if _, n, err := net.ParseCIDR(subnet); err == nil {
		subnet = n.String()
	}
	specs := []natRuleSpec{
		// Masquerade guest traffic leaving its /30 so replies route back.
		{"POSTROUTING", []string{"-s", subnet, "!", "-d", subnet, "-m", "comment", "--comment", comment, "-j", "MASQUERADE"}},
		// 127.0.0.1:hostPort -> guest:guestPort, so host processes reach the guest.
		{"OUTPUT", []string{"-d", "127.0.0.1/32", "-p", "tcp", "-m", "tcp", "--dport", fmt.Sprintf("%d", hostPort),
			"-m", "comment", "--comment", comment, "-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", guestIP, guestPort)}},
	}
	// hostIP:<port> -> 127.0.0.1:<port>: only the intended host services.
	for _, port := range tapReachableHostServicePorts() {
		specs = append(specs, natRuleSpec{"PREROUTING", []string{"-d", hostIP + "/32", "-p", "tcp", "-m", "tcp", "--dport", port,
			"-m", "comment", "--comment", comment, "-j", "DNAT", "--to-destination", "127.0.0.1:" + port}})
	}
	return specs
}

// managedChains are the only chains the teardown and the sweep read or edit.
var managedChains = []struct{ table, chain string }{
	{"filter", "INPUT"},
	{"filter", "FORWARD"},
	{"filter", guestTapsChain},
	{"nat", "PREROUTING"},
	{"nat", "OUTPUT"},
	{"nat", "POSTROUTING"},
}

// listedRule is one "-A chain ..." line of `iptables -S`.
type listedRule struct {
	chain string
	spec  []string
	line  string
}

func parseListedRules(out []byte) []listedRule {
	var rules []listedRule
	for _, line := range strings.Split(string(out), "\n") {
		fields := iptablesRuleFields(line)
		if len(fields) < 2 || fields[0] != "-A" {
			continue
		}
		rules = append(rules, listedRule{chain: fields[1], spec: fields[2:], line: strings.TrimSpace(line)})
	}
	return rules
}

// ruleOptions reads the options this package writes. -m <module> tokens are
// skipped; any other token is kept so unknown rules never compare equal to
// ours.
func ruleOptions(spec []string) (opts map[string]string, extra []string) {
	opts = map[string]string{}
	neg := false
	for i := 0; i < len(spec); i++ {
		tok := spec[i]
		if tok == "!" {
			neg = true
			continue
		}
		switch tok {
		case "-s", "-d", "-i", "-o", "-p", "--dport", "--comment", "-j", "--to-destination":
			if i+1 >= len(spec) {
				extra = append(extra, tok)
				continue
			}
			val := spec[i+1]
			if tok == "-s" || tok == "-d" {
				val = canonicalCIDR(val)
			}
			key := tok
			if neg {
				key = "!" + tok
			}
			opts[key] = val
			i++
		case "-m":
			i++
		default:
			if neg {
				tok = "!" + tok
			}
			extra = append(extra, tok)
		}
		neg = false
	}
	return opts, extra
}

func canonicalCIDR(v string) string {
	if !strings.Contains(v, "/") {
		v += "/32"
	}
	if _, n, err := net.ParseCIDR(v); err == nil {
		return n.String()
	}
	return v
}

// ruleKey compares rules by meaning, not by token order: `iptables -S`
// reorders options, adds /32 and the implicit -m tcp.
func ruleKey(spec []string) string {
	opts, extra := ruleOptions(spec)
	parts := make([]string, 0, len(opts)+len(extra))
	for k, v := range opts {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ") + " | " + strings.Join(extra, " ")
}

// ruleOwnerTap names the tap a rule belongs to, or "" when the rule is not
// this package's. A comment decides when present (go-choir-vm-<tap> only); an
// untagged rule is ours when its -i or -o is an owned tap name.
func ruleOwnerTap(spec []string) string {
	opts, _ := ruleOptions(spec)
	if comment, ok := opts["--comment"]; ok {
		if tap := strings.TrimPrefix(comment, tapCommentPrefix); tap != comment && isOwnedTapName(tap) {
			return tap
		}
		return ""
	}
	for _, flag := range []string{"-i", "-o"} {
		if name := opts[flag]; isOwnedTapName(name) {
			return name
		}
	}
	return ""
}

// hostNet runs the host networking commands for guest taps. Tests replace
// the command runners.
type hostNet struct {
	mu        sync.Mutex
	iptables  func(args ...string) ([]byte, error)
	ip        func(args ...string) ([]byte, error)
	writeProc func(path, value string) error
	pool      []string
}

func newSystemHostNet() *hostNet {
	return &hostNet{
		iptables: func(args ...string) ([]byte, error) {
			return runHostCommand(findBinary("iptables", "/run/current-system/sw/bin/iptables"), args...)
		},
		ip: func(args ...string) ([]byte, error) {
			return runHostCommand(findBinary("ip", "/run/current-system/sw/bin/ip"), args...)
		},
		writeProc: func(path, value string) error { return os.WriteFile(path, []byte(value), 0o644) },
		pool:      guestPoolCIDRs(),
	}
}

func runHostCommand(bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w (%s)", filepath.Base(bin), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (m *Manager) net() *hostNet {
	m.hostNetOnce.Do(func() {
		if m.hostNet == nil {
			m.hostNet = newSystemHostNet()
		}
	})
	return m.hostNet
}

func tableArgs(table string, args ...string) []string {
	if table == "" || table == "filter" {
		return args
	}
	return append([]string{"-t", table}, args...)
}

func (h *hostNet) ipt(table string, args ...string) ([]byte, error) {
	return h.iptables(tableArgs(table, args...)...)
}

func (h *hostNet) listChainLocked(table, chain string) ([]listedRule, error) {
	out, err := h.ipt(table, "-S", chain)
	if err != nil {
		return nil, err
	}
	return parseListedRules(out), nil
}

func (h *hostNet) ensureChainLocked(chain string) error {
	if _, err := h.ipt("filter", "-S", chain); err == nil {
		return nil
	}
	if _, err := h.ipt("filter", "-N", chain); err != nil {
		// Lost a race or the listing failed for another reason; trust -S.
		if _, listErr := h.ipt("filter", "-S", chain); listErr != nil {
			return fmt.Errorf("create chain %s: %w", chain, err)
		}
	}
	return nil
}

func (h *hostNet) deleteListedLocked(table string, r listedRule) error {
	args := append([]string{"-D", r.chain}, r.spec...)
	_, err := h.ipt(table, args...)
	return err
}

// ensureBase installs CHOIR-GUEST-FWD with the pool deny and tail DROP, and
// the FORWARD jump into it. A chain whose content differs (an older pool, a
// partial rebuild) is rebuilt behind a DROP guard, so guest forwarding is
// never open while it changes.
func (h *hostNet) ensureBase() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ensureBaseLocked()
}

func (h *hostNet) ensureBaseLocked() error {
	if err := h.ensureChainLocked(guestTapsChain); err != nil {
		return err
	}
	if err := h.ensureChainLocked(guestForwardChain); err != nil {
		return err
	}
	current, err := h.listChainLocked("filter", guestForwardChain)
	if err != nil {
		return err
	}
	want := guestForwardChainSpecs(h.pool)
	if !sameRules(current, want) {
		if err := h.rebuildForwardChainLocked(current, want); err != nil {
			return err
		}
	}
	jump := guestJumpSpec()
	if _, err := h.ipt("filter", append([]string{"-C", "FORWARD"}, jump...)...); err != nil {
		if _, err := h.ipt("filter", append([]string{"-I", "FORWARD", "1"}, jump...)...); err != nil {
			return fmt.Errorf("install guest forward jump: %w", err)
		}
	}
	return nil
}

func sameRules(current []listedRule, want [][]string) bool {
	if len(current) != len(want) {
		return false
	}
	for i := range want {
		if ruleKey(current[i].spec) != ruleKey(want[i]) {
			return false
		}
	}
	return true
}

func (h *hostNet) rebuildForwardChainLocked(current []listedRule, want [][]string) error {
	guard := guestRebuildGuardSpec()
	guarded := len(current) > 0
	if guarded {
		if _, err := h.ipt("filter", append([]string{"-I", guestForwardChain, "1"}, guard...)...); err != nil {
			return fmt.Errorf("guard %s rebuild: %w", guestForwardChain, err)
		}
		// Only this code writes the chain, under h.mu: rule 2 onward is the
		// old content.
		for range current {
			if _, err := h.ipt("filter", "-D", guestForwardChain, "2"); err != nil {
				return fmt.Errorf("clear %s (guard stays, forwarding fails closed): %w", guestForwardChain, err)
			}
		}
	}
	for _, spec := range want {
		if _, err := h.ipt("filter", append([]string{"-A", guestForwardChain}, spec...)...); err != nil {
			return fmt.Errorf("fill %s (guard stays, forwarding fails closed): %w", guestForwardChain, err)
		}
	}
	if guarded {
		if _, err := h.ipt("filter", append([]string{"-D", guestForwardChain}, guard...)...); err != nil {
			return fmt.Errorf("remove %s rebuild guard: %w", guestForwardChain, err)
		}
	}
	log.Printf("vmmanager: installed guest forward chain %s (pool %s)", guestForwardChain, strings.Join(h.pool, ","))
	return nil
}

// desiredRule is a rule a tap must have in a chain; insertAt 0 appends.
type desiredRule struct {
	spec     []string
	insertAt int
}

// reconcileTapChainLocked makes the tap's rules in one chain exactly the
// desired set: missing rules are added first, then the tap's other rules
// (legacy shapes, duplicates, a stale guest address) are deleted.
func (h *hostNet) reconcileTapChainLocked(table, chain, tap string, desired []desiredRule) error {
	rules, err := h.listChainLocked(table, chain)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, r := range rules {
		if ruleOwnerTap(r.spec) == tap {
			present[ruleKey(r.spec)] = true
		}
	}
	wanted := map[string]bool{}
	var firstErr error
	for _, d := range desired {
		key := ruleKey(d.spec)
		wanted[key] = true
		if present[key] {
			continue
		}
		args := append([]string{"-A", chain}, d.spec...)
		if d.insertAt > 0 {
			args = append([]string{"-I", chain, fmt.Sprintf("%d", d.insertAt)}, d.spec...)
		}
		if _, err := h.ipt(table, args...); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("add %s/%s rule for %s: %w", table, chain, tap, err)
		}
	}
	if firstErr != nil {
		// Never delete the old rules when the new ones did not all land.
		return firstErr
	}
	kept := map[string]bool{}
	for _, r := range rules {
		if ruleOwnerTap(r.spec) != tap {
			continue
		}
		key := ruleKey(r.spec)
		if wanted[key] && !kept[key] {
			kept[key] = true
			continue
		}
		if err := h.deleteListedLocked(table, r); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("remove stale %s/%s rule %q: %w", table, chain, r.line, err)
		}
	}
	return firstErr
}

// ensureTap installs a live tap's filter rules: its CHOIR-GUEST-TAPS egress
// ACCEPT (before the base, so egress never drops), the base, its INPUT
// anti-spoof DROP and source-bound service ACCEPTs, and the FORWARD return
// ACCEPT. Older rule shapes for the tap are removed only after the new ones
// are in. Idempotent.
func (h *hostNet) ensureTap(tap, guestIP string) error {
	if !isOwnedTapName(tap) {
		return fmt.Errorf("refuse firewall rules for %q: not a guest tap name", tap)
	}
	if ip := net.ParseIP(guestIP); ip == nil || ip.To4() == nil {
		return fmt.Errorf("refuse firewall rules for %s: bad guest IP %q", tap, guestIP)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ensureChainLocked(guestTapsChain); err != nil {
		return err
	}
	if err := h.reconcileTapChainLocked("filter", guestTapsChain, tap, []desiredRule{{spec: tapForwardAcceptSpec(tap, guestIP)}}); err != nil {
		return err
	}
	if err := h.ensureBaseLocked(); err != nil {
		return err
	}
	input := []desiredRule{{spec: tapInputDropSpec(tap, guestIP), insertAt: 1}}
	for _, port := range tapReachableHostServicePorts() {
		input = append(input, desiredRule{spec: tapHostServiceInputRuleSpec(tap, guestIP, port, tapComment(tap)), insertAt: 1})
	}
	if err := h.reconcileTapChainLocked("filter", "INPUT", tap, input); err != nil {
		return err
	}
	return h.reconcileTapChainLocked("filter", "FORWARD", tap, []desiredRule{{spec: tapForwardReturnSpec(tap)}})
}

// ensureNAT installs a tap's nat rules, each only if missing.
func (h *hostNet) ensureNAT(tap, hostIP, guestIP string, hostPort, guestPort int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var firstErr error
	for _, r := range tapNATSpecs(tap, hostIP, guestIP, hostPort, guestPort) {
		if _, err := h.ipt("nat", append([]string{"-C", r.chain}, r.spec...)...); err == nil {
			continue
		}
		if _, err := h.ipt("nat", append([]string{"-A", r.chain}, r.spec...)...); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("add nat/%s rule for %s: %w", r.chain, tap, err)
		}
	}
	return firstErr
}

// removeTap deletes every rule owned by the tap in the managed chains.
func (h *hostNet) removeTap(tap string) {
	if !isOwnedTapName(tap) {
		log.Printf("vmmanager: not removing firewall rules for %q: not a guest tap name", tap)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deleteOwnedLocked(func(owner string) bool { return owner == tap })
}

func (h *hostNet) deleteOwnedLocked(match func(owner string) bool) int {
	deleted := 0
	for _, mc := range managedChains {
		rules, err := h.listChainLocked(mc.table, mc.chain)
		if err != nil {
			if mc.chain != guestTapsChain {
				log.Printf("vmmanager: warning: could not list %s/%s: %v", mc.table, mc.chain, err)
			}
			continue
		}
		for _, r := range rules {
			owner := ruleOwnerTap(r.spec)
			if owner == "" || !match(owner) {
				continue
			}
			if err := h.deleteListedLocked(mc.table, r); err != nil {
				log.Printf("vmmanager: warning: could not delete %s rule %q: %v", mc.table, r.line, err)
				continue
			}
			deleted++
		}
	}
	return deleted
}

// sweepOrphans deletes rules owned by taps that no longer exist. An empty
// link set is refused: it means the listing failed, not that no tap exists.
func (h *hostNet) sweepOrphans(live map[string]bool) int {
	if len(live) == 0 {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.deleteOwnedLocked(func(owner string) bool { return !live[owner] })
}

// reconcileAtStart runs once when vmctl starts, before any VM is reattached:
// sweep rules of vanished taps, seed the egress ACCEPT of every live tap from
// its address, then install the base. Seeding first keeps guests that
// outlived the previous vmctl online across the switch to default-deny.
func (h *hostNet) reconcileAtStart() {
	if links, err := h.ip("-o", "link", "show"); err != nil {
		log.Printf("vmmanager: firewall sweep skipped: cannot list links: %v", err)
	} else if live := parseLinkNames(links); len(live) > 0 {
		if n := h.sweepOrphans(live); n > 0 {
			log.Printf("vmmanager: removed %d firewall rule(s) of vanished guest taps", n)
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if addrs, err := h.ip("-o", "-4", "addr", "show"); err != nil {
		log.Printf("vmmanager: warning: cannot list tap addresses to seed guest egress: %v", err)
	} else if err := h.ensureChainLocked(guestTapsChain); err != nil {
		log.Printf("vmmanager: warning: %v", err)
	} else {
		for tap, guestIP := range parseTapGuestIPs(addrs, h.pool) {
			if err := h.reconcileTapChainLocked("filter", guestTapsChain, tap, []desiredRule{{spec: tapForwardAcceptSpec(tap, guestIP)}}); err != nil {
				log.Printf("vmmanager: warning: seed egress for live tap %s: %v", tap, err)
			}
		}
	}
	if err := h.ensureBaseLocked(); err != nil {
		log.Printf("vmmanager: warning: guest forward default-deny not installed: %v", err)
	}
}

// deleteMatching deletes rules of one chain whose listed text contains match.
func (h *hostNet) deleteMatching(table, chain, match string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.TrimSpace(match) == "" {
		return
	}
	out, err := h.ipt(table, "-S", chain)
	if err != nil {
		log.Printf("vmmanager: warning: could not list iptables rules for %s/%s: %v", table, chain, err)
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		deleteArgs, ok := iptablesDeleteArgsForListedRule(table, chain, line, match)
		if !ok {
			continue
		}
		if _, err := h.iptables(deleteArgs...); err != nil {
			log.Printf("vmmanager: warning: could not delete iptables rule %q: %v", line, err)
		}
	}
}

// parseLinkNames reads `ip -o link show`.
func parseLinkNames(out []byte) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[1], ":")
		if i := strings.Index(name, "@"); i >= 0 {
			name = name[:i]
		}
		if name != "" {
			names[name] = true
		}
	}
	return names
}

// parseTapGuestIPs reads `ip -o -4 addr show` and returns the guest address
// of every owned tap laid out as guestAndHostIP does: host .1 and guest .2 of
// a /30 inside the guest pool.
func parseTapGuestIPs(out []byte, pool []string) map[string]string {
	var nets []*net.IPNet
	for _, c := range pool {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	guests := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[2] != "inet" || !isOwnedTapName(fields[1]) {
			continue
		}
		ip, subnet, err := net.ParseCIDR(fields[3])
		if err != nil {
			continue
		}
		ones, _ := subnet.Mask.Size()
		host, network := ip.To4(), subnet.IP.To4()
		if ones != 30 || host == nil || network == nil || host[3] != network[3]+1 {
			continue
		}
		inPool := false
		for _, n := range nets {
			inPool = inPool || n.Contains(host)
		}
		if !inPool {
			continue
		}
		guest := net.IPv4(network[0], network[1], network[2], network[3]+2)
		guests[fields[1]] = guest.String()
	}
	return guests
}
