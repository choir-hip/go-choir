package vmmanager

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeIPTables is an in-memory iptables good enough to run hostNet against:
// -S/-C/-A/-I/-D/-N/-F over the filter and nat tables, with rule text
// normalized the way `iptables -S` prints it, and a packet evaluator so tests
// assert isolation by verdict instead of by rule text.
type fakeIPTables struct {
	t      *testing.T
	mu     sync.Mutex
	tables map[string]*fakeTable
	calls  [][]string
	// afterMutation runs after every rule-changing command (invariants).
	afterMutation func(step []string)
	// fail makes a matching command return an error.
	fail func(args []string) bool
}

type fakeTable struct {
	order  []string
	chains map[string]*fakeChain
}

type fakeChain struct {
	builtin bool
	policy  string
	rules   []fakeRule
}

// fakeRule is one rule. opaque rules (matches the fake does not model, such
// as nixos-fw's conntrack) are kept verbatim and never match a packet.
type fakeRule struct {
	fields []string
	opaque bool
}

func newFakeIPTables(t *testing.T) *fakeIPTables {
	f := &fakeIPTables{t: t, tables: map[string]*fakeTable{}}
	f.table("filter", "INPUT", "FORWARD", "OUTPUT")
	f.table("nat", "PREROUTING", "INPUT", "OUTPUT", "POSTROUTING")
	return f
}

func (f *fakeIPTables) table(name string, builtins ...string) {
	tb := &fakeTable{chains: map[string]*fakeChain{}}
	for _, c := range builtins {
		tb.order = append(tb.order, c)
		tb.chains[c] = &fakeChain{builtin: true, policy: "ACCEPT"}
	}
	f.tables[name] = tb
}

// loadFixture reads `*table` sections of `iptables -S` output.
func (f *fakeIPTables) loadFixture(path string) {
	f.t.Helper()
	file, err := os.Open(path)
	if err != nil {
		f.t.Fatal(err)
	}
	defer file.Close()
	f.loadText(bufio.NewScanner(file))
}

func (f *fakeIPTables) loadString(text string) {
	f.loadText(bufio.NewScanner(strings.NewReader(text)))
}

func (f *fakeIPTables) loadText(sc *bufio.Scanner) {
	f.t.Helper()
	table := "filter"
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			table = strings.TrimPrefix(line, "*")
			continue
		}
		fields := iptablesRuleFields(line)
		tb := f.tables[table]
		switch fields[0] {
		case "-P":
			tb.chains[fields[1]].policy = fields[2]
		case "-N":
			if _, ok := tb.chains[fields[1]]; !ok {
				tb.order = append(tb.order, fields[1])
				tb.chains[fields[1]] = &fakeChain{}
			}
		case "-A":
			c := tb.chains[fields[1]]
			if c == nil {
				f.t.Fatalf("fixture appends to unknown chain %s", fields[1])
			}
			c.rules = append(c.rules, makeFakeRule(fields[2:]))
		default:
			f.t.Fatalf("fixture line %q", line)
		}
	}
}

func makeFakeRule(spec []string) fakeRule {
	norm, ok := normalizeFakeSpec(spec)
	if !ok {
		return fakeRule{fields: append([]string(nil), spec...), opaque: true}
	}
	return fakeRule{fields: norm}
}

// normalizeFakeSpec renders a rule spec in iptables -S order: addresses get
// a prefix length and their network address, --dport implies -m tcp, and
// options print as -s -d -i -o -p, matches, then -j.
func normalizeFakeSpec(spec []string) ([]string, bool) {
	type opt struct {
		neg bool
		val string
		set bool
	}
	var s, d, in, out, proto, dport, uidOwner, comment, target, toDest opt
	neg := false
	for i := 0; i < len(spec); i++ {
		tok := spec[i]
		if tok == "!" {
			neg = true
			continue
		}
		if i+1 >= len(spec) {
			return nil, false
		}
		val := spec[i+1]
		var dst *opt
		switch tok {
		case "-s":
			dst, val = &s, fakeCIDR(val)
		case "-d":
			dst, val = &d, fakeCIDR(val)
		case "-i":
			dst = &in
		case "-o":
			dst = &out
		case "-p":
			dst = &proto
		case "--dport":
			dst = &dport
		case "--comment":
			dst = &comment
		case "--uid-owner":
			dst = &uidOwner
		case "-j":
			dst = &target
		case "--to-destination":
			dst = &toDest
		case "-m":
			if val != "tcp" && val != "comment" && val != "owner" {
				return nil, false
			}
			i++
			neg = false
			continue
		default:
			return nil, false
		}
		if val == "" {
			return nil, false
		}
		*dst = opt{neg: neg, val: val, set: true}
		neg = false
		i++
	}
	var outFields []string
	add := func(o opt, flag string) {
		if !o.set {
			return
		}
		if o.neg {
			outFields = append(outFields, "!")
		}
		outFields = append(outFields, flag, o.val)
	}
	add(s, "-s")
	add(d, "-d")
	add(in, "-i")
	add(out, "-o")
	add(proto, "-p")
	if dport.set {
		outFields = append(outFields, "-m", "tcp")
		add(dport, "--dport")
	}
	if uidOwner.set {
		outFields = append(outFields, "-m", "owner")
		add(uidOwner, "--uid-owner")
	}
	if comment.set {
		outFields = append(outFields, "-m", "comment")
		add(comment, "--comment")
	}
	add(target, "-j")
	add(toDest, "--to-destination")
	return outFields, true
}

func fakeCIDR(v string) string {
	if !strings.Contains(v, "/") {
		v += "/32"
	}
	_, n, err := net.ParseCIDR(v)
	if err != nil {
		return ""
	}
	return n.String()
}

func (f *fakeIPTables) run(args ...string) ([]byte, error) {
	out, err := f.apply(args...)
	if err == nil && out == nil && f.afterMutation != nil && isFakeMutation(args) {
		// Outside the lock: the hook evaluates packets and may t.Fatal.
		f.afterMutation(args)
	}
	return out, err
}

func isFakeMutation(args []string) bool {
	if len(args) >= 2 && args[0] == "-t" {
		args = args[2:]
	}
	switch args[0] {
	case "-A", "-I", "-D", "-F":
		return true
	}
	return false
}

func (f *fakeIPTables) apply(args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), args...))
	if f.fail != nil && f.fail(args) {
		return nil, fmt.Errorf("fake iptables: injected failure for %v", args)
	}
	table := "filter"
	if len(args) >= 2 && args[0] == "-t" {
		table, args = args[1], args[2:]
	}
	tb := f.tables[table]
	if tb == nil || len(args) == 0 {
		return nil, fmt.Errorf("fake iptables: bad table/args")
	}
	cmd := args[0]
	chainName := ""
	if len(args) > 1 {
		chainName = args[1]
	}
	chain := tb.chains[chainName]
	noChain := fmt.Errorf("iptables: No chain/target/match by that name")
	switch cmd {
	case "-S":
		if chainName == "" {
			return []byte(f.listTable(tb, "")), nil
		}
		if chain == nil {
			return nil, noChain
		}
		return []byte(f.listTable(tb, chainName)), nil
	case "-P", "-X":
		f.t.Errorf("hostNet must never run iptables %v (host policy/chains are not ours to change)", args)
		return nil, fmt.Errorf("forbidden")
	}
	if chain == nil {
		return nil, noChain
	}
	switch cmd {
	case "-N":
		return nil, fmt.Errorf("iptables: Chain already exists")
	case "-C":
		if f.indexOf(chain, args[2:]) < 0 {
			return nil, fmt.Errorf("iptables: Bad rule (does a matching rule exist in that chain?)")
		}
		return nil, nil
	case "-A", "-I":
		spec := args[2:]
		pos := len(chain.rules)
		if cmd == "-I" {
			pos = 0
			if len(spec) > 0 {
				if n, err := strconv.Atoi(spec[0]); err == nil {
					pos, spec = n-1, spec[1:]
				}
			}
			if pos < 0 || pos > len(chain.rules) {
				return nil, fmt.Errorf("iptables: Index of insertion too big")
			}
		}
		rule := makeFakeRule(spec)
		if rule.opaque {
			f.t.Errorf("hostNet emitted a rule the fake cannot model: %v", spec)
		}
		if tgt := fakeTarget(rule.fields); tgt != "" && !fakeTerminal(tgt) && tb.chains[tgt] == nil {
			return nil, noChain
		}
		chain.rules = append(chain.rules, fakeRule{})
		copy(chain.rules[pos+1:], chain.rules[pos:])
		chain.rules[pos] = rule
	case "-D":
		spec := args[2:]
		idx := -1
		if len(spec) == 1 {
			if n, err := strconv.Atoi(spec[0]); err == nil {
				idx = n - 1
				if idx < 0 || idx >= len(chain.rules) {
					return nil, fmt.Errorf("iptables: Index of deletion too big")
				}
			}
		}
		if idx < 0 {
			idx = f.indexOf(chain, spec)
		}
		if idx < 0 {
			return nil, fmt.Errorf("iptables: Bad rule (does a matching rule exist in that chain?)")
		}
		chain.rules = append(chain.rules[:idx], chain.rules[idx+1:]...)
	case "-F":
		chain.rules = nil
	default:
		return nil, fmt.Errorf("fake iptables: unsupported %v", args)
	}
	return nil, nil
}

// newChain handles -N separately: it is the one command valid on a missing chain.
func (f *fakeIPTables) runWithNew(args ...string) ([]byte, error) {
	a := args
	table := "filter"
	if len(a) >= 2 && a[0] == "-t" {
		table, a = a[1], a[2:]
	}
	if len(a) == 2 && a[0] == "-N" {
		f.mu.Lock()
		tb := f.tables[table]
		f.calls = append(f.calls, append([]string(nil), args...))
		if _, ok := tb.chains[a[1]]; ok {
			f.mu.Unlock()
			return nil, fmt.Errorf("iptables: Chain already exists")
		}
		tb.order = append(tb.order, a[1])
		tb.chains[a[1]] = &fakeChain{}
		f.mu.Unlock()
		return nil, nil
	}
	return f.run(args...)
}

func (f *fakeIPTables) indexOf(chain *fakeChain, spec []string) int {
	norm, ok := normalizeFakeSpec(spec)
	if !ok {
		return -1
	}
	want := strings.Join(norm, " ")
	for i, r := range chain.rules {
		if !r.opaque && strings.Join(r.fields, " ") == want {
			return i
		}
	}
	return -1
}

func (f *fakeIPTables) listTable(tb *fakeTable, only string) string {
	var b strings.Builder
	for _, name := range tb.order {
		if only != "" && name != only {
			continue
		}
		c := tb.chains[name]
		if c.builtin {
			fmt.Fprintf(&b, "-P %s %s\n", name, c.policy)
		} else {
			fmt.Fprintf(&b, "-N %s\n", name)
		}
	}
	for _, name := range tb.order {
		if only != "" && name != only {
			continue
		}
		for _, r := range tb.chains[name].rules {
			fmt.Fprintf(&b, "-A %s %s\n", name, strings.Join(r.fields, " "))
		}
	}
	return b.String()
}

// rules returns "-A chain spec" lines of one chain (empty when missing).
func (f *fakeIPTables) rules(table, chain string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.tables[table].chains[chain]
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.rules))
	for _, r := range c.rules {
		out = append(out, "-A "+chain+" "+strings.Join(r.fields, " "))
	}
	return out
}

func (f *fakeIPTables) dump() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "*filter\n" + f.listTable(f.tables["filter"], "") + "*nat\n" + f.listTable(f.tables["nat"], "")
}

func (f *fakeIPTables) mutations() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out [][]string
	for _, c := range f.calls {
		a := c
		if len(a) >= 2 && a[0] == "-t" {
			a = a[2:]
		}
		switch a[0] {
		case "-A", "-I", "-D", "-N", "-F":
			out = append(out, c)
		}
	}
	return out
}

func fakeTarget(fields []string) string {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "-j" {
			return fields[i+1]
		}
	}
	return ""
}

func fakeTerminal(target string) bool {
	switch target {
	case "ACCEPT", "DROP", "RETURN", "REJECT", "DNAT", "SNAT", "MASQUERADE":
		return true
	}
	return false
}

type fakePacket struct {
	in, out  string
	src, dst string
	proto    string
	dport    string
}

// verdict walks a packet through filter/<chain> and returns ACCEPT, DROP,
// "policy ACCEPT"/"policy DROP", or "opaque" when it reaches a rule the fake
// does not model (a foreign chain's conntrack match, say).
func (f *fakeIPTables) verdict(chain string, p fakePacket) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, done := f.walk(f.tables["filter"], chain, p, 0)
	if done {
		return v
	}
	return "policy " + f.tables["filter"].chains[chain].policy
}

func (f *fakeIPTables) walk(tb *fakeTable, chain string, p fakePacket, depth int) (string, bool) {
	if depth > 16 {
		return "loop", true
	}
	c := tb.chains[chain]
	if c == nil {
		return "missing-chain " + chain, true
	}
	for _, r := range c.rules {
		if r.opaque {
			return "opaque", true
		}
		if !fakeMatches(r.fields, p) {
			continue
		}
		switch tgt := fakeTarget(r.fields); tgt {
		case "ACCEPT", "DROP", "REJECT":
			return tgt, true
		case "RETURN":
			return "", false
		case "":
			continue
		default:
			if v, done := f.walk(tb, tgt, p, depth+1); done {
				return v, true
			}
		}
	}
	return "", false
}

func fakeMatches(fields []string, p fakePacket) bool {
	neg := false
	for i := 0; i < len(fields); i++ {
		tok := fields[i]
		if tok == "!" {
			neg = true
			continue
		}
		if i+1 >= len(fields) {
			return false
		}
		val := fields[i+1]
		ok := true
		switch tok {
		case "-s":
			ok = fakeInCIDR(p.src, val)
		case "-d":
			ok = fakeInCIDR(p.dst, val)
		case "-i":
			ok = fakeIface(p.in, val)
		case "-o":
			ok = fakeIface(p.out, val)
		case "-p":
			ok = p.proto == val
		case "--dport":
			ok = p.dport == val
		default:
			i++
			neg = false
			continue
		}
		if ok == neg {
			return false
		}
		neg = false
		i++
	}
	return true
}

func fakeInCIDR(ip, cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return n.Contains(net.ParseIP(ip))
}

func fakeIface(name, pattern string) bool {
	if strings.HasSuffix(pattern, "+") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "+"))
	}
	return name == pattern
}

// fakeIP answers the `ip` invocations hostNet makes from a link table.
type fakeIP struct {
	mu      sync.Mutex
	links   map[string]string // name -> "a.b.c.d/30" ("" = no IPv4)
	calls   [][]string
	failAll bool
}

func newFakeIP(links map[string]string) *fakeIP {
	if links == nil {
		links = map[string]string{}
	}
	if _, ok := links["lo"]; !ok {
		links["lo"] = "127.0.0.1/8"
	}
	return &fakeIP{links: links}
}

func (f *fakeIP) run(args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), args...))
	if f.failAll {
		return nil, fmt.Errorf("fake ip: unavailable")
	}
	joined := strings.Join(args, " ")
	var b strings.Builder
	i := 1
	switch {
	case joined == "-o link show":
		for name := range f.links {
			fmt.Fprintf(&b, "%d: %s: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP mode DEFAULT group default qlen 1000\\    link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff\n", i, name)
			i++
		}
		return []byte(b.String()), nil
	case joined == "-o -4 addr show":
		for name, addr := range f.links {
			if addr != "" {
				fmt.Fprintf(&b, "%d: %s    inet %s scope global %s\\       valid_lft forever preferred_lft forever\n", i, name, addr, name)
			}
			i++
		}
		return []byte(b.String()), nil
	case len(args) == 3 && args[0] == "link" && args[1] == "del":
		if _, ok := f.links[args[2]]; !ok {
			return nil, fmt.Errorf("Cannot find device %q", args[2])
		}
		delete(f.links, args[2])
		return nil, nil
	case len(args) == 3 && args[0] == "link" && args[1] == "show":
		if _, ok := f.links[args[2]]; !ok {
			return nil, fmt.Errorf("Device %q does not exist", args[2])
		}
		return nil, nil
	case len(args) >= 4 && args[0] == "tuntap" && args[1] == "add":
		f.links[args[3]] = ""
		return nil, nil
	case len(args) == 4 && args[0] == "link" && args[1] == "set":
		return nil, nil
	case len(args) == 5 && args[0] == "addr" && args[1] == "add":
		f.links[args[4]] = args[2]
		return nil, nil
	}
	return nil, fmt.Errorf("fake ip: unsupported %v", args)
}

func (f *fakeIPTables) policy(table, chain string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tables[table].chains[chain].policy
}

func (f *fakeIP) has(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.links[name]
	return ok
}

func newFakeHostNet(t *testing.T, ipt *fakeIPTables, ip *fakeIP) *hostNet {
	return &hostNet{
		iptables:  ipt.runWithNew,
		ip:        ip.run,
		writeProc: func(string, string) error { return nil },
		pool:      guestPoolCIDRs(),
	}
}
