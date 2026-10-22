package appnet

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/egressgrant"
)

var t0 = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func readEntry() EgressEntry {
	return EgressEntry{
		Verb:         "calendar.sync",
		Destinations: []egressgrant.Destination{{Scheme: "https", Host: "www.googleapis.com", PathPrefix: "/calendar/v3/"}},
		Methods:      []string{"GET"},
		EffectClass:  egressgrant.EffectRead,
		DataCeiling:  "owner-private",
		Budget:       egressgrant.Budget{MaxRequests: 2000, MaxDownloadBytes: 64 << 20, MaxUploadBytes: 1 << 20},
		RenewalDays:  30,
	}
}

func goodPolicy() Policy {
	return Policy{
		Egress:  []EgressEntry{readEntry()},
		Ingress: []IngressEntry{{Source: SourceMail, Address: "calendar"}},
	}
}

func codes(ps []Problem) []Code {
	out := make([]Code, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Code)
	}
	return out
}

func hasCode(ps []Problem, c Code) bool {
	for _, p := range ps {
		if p.Code == c {
			return true
		}
	}
	return false
}

func TestValidPolicyHasNoProblems(t *testing.T) {
	if ps := Validate("calendar", goodPolicy()); len(ps) != 0 {
		t.Fatalf("good policy refused: %v", ps)
	}
	if ps := Validate("calendar", Policy{}); len(ps) != 0 {
		t.Fatalf("empty policy refused: %v", ps)
	}
}

// Failure mode 1: verb grammar and duplicates.
func TestVerbMustBeNamespacedAndUnique(t *testing.T) {
	for _, verb := range []string{"", "sync", "mail.send", "calendar.", "calendar.Sync", "calendar.sync.more", ".sync"} {
		p := goodPolicy()
		p.Egress[0].Verb = verb
		if ps := Validate("calendar", p); !hasCode(ps, CodeVerb) {
			t.Errorf("verb %q: want %s, got %v", verb, CodeVerb, codes(ps))
		}
	}
	p := goodPolicy()
	p.Egress = append(p.Egress, readEntry())
	if ps := Validate("calendar", p); !hasCode(ps, CodeDuplicate) {
		t.Errorf("duplicate verb: want %s, got %v", CodeDuplicate, codes(ps))
	}
}

// Failure mode 2: destinations egressgrant refuses for an app desk.
func TestDestinationsTheGrantRefuses(t *testing.T) {
	bad := []egressgrant.Destination{
		{Scheme: "https", Host: "*"},
		{Scheme: "https", Host: "10.0.0.1"},
		{Scheme: "https", Host: "169.254.169.254"},
		{Scheme: "https", Host: "localhost"},
		{Scheme: "https", Host: "metadata.google.internal"},
		{Scheme: "http", Host: "www.googleapis.com"},
		{Scheme: "tcp", Host: "www.googleapis.com"},
		{Scheme: "https", Host: "www.googleapis.com", PathPrefix: "calendar"},
	}
	for _, d := range bad {
		p := goodPolicy()
		p.Egress[0].Destinations = []egressgrant.Destination{d}
		if ps := Validate("calendar", p); !hasCode(ps, CodeGrant) {
			t.Errorf("destination %+v: want %s, got %v", d, CodeGrant, codes(ps))
		}
	}
	p := goodPolicy()
	p.Egress[0].Destinations = nil
	if ps := Validate("calendar", p); !hasCode(ps, CodeGrant) {
		t.Errorf("no destinations: want %s, got %v", CodeGrant, codes(ps))
	}
}

// Failure mode 3: effect classes.
func TestEffectClassRules(t *testing.T) {
	cases := []struct {
		name string
		edit func(*EgressEntry)
	}{
		{"none", func(e *EgressEntry) { e.EffectClass = "" }},
		{"unknown", func(e *EgressEntry) { e.EffectClass = "write" }},
		{"money", func(e *EgressEntry) { e.EffectClass = egressgrant.EffectMoney }},
		{"irreversible", func(e *EgressEntry) { e.EffectClass = egressgrant.EffectIrreversible }},
		{"visible write without contract", func(e *EgressEntry) {
			e.EffectClass = egressgrant.EffectVisibleWrite
			e.Methods = []string{"POST"}
		}},
		{"read with DELETE", func(e *EgressEntry) { e.Methods = []string{"GET", "DELETE"} }},
		{"read with PUT", func(e *EgressEntry) { e.Methods = []string{"PUT"} }},
		{"read with patch, lower case", func(e *EgressEntry) { e.Methods = []string{"patch"} }},
		// POST is dual-use (a POST to calendar/v3 creates events), so a read
		// is GET and HEAD only; OPTIONS is refused conservatively.
		{"read with POST", func(e *EgressEntry) { e.Methods = []string{"GET", "POST"} }},
		{"read with post, lower case", func(e *EgressEntry) { e.Methods = []string{"post"} }},
		{"read with OPTIONS", func(e *EgressEntry) { e.Methods = []string{"OPTIONS"} }},
		{"read with an unknown method", func(e *EgressEntry) { e.Methods = []string{"QUERY"} }},
	}
	for _, c := range cases {
		p := goodPolicy()
		c.edit(&p.Egress[0])
		if ps := Validate("calendar", p); !hasCode(ps, CodeEffect) {
			t.Errorf("%s: want %s, got %v", c.name, CodeEffect, codes(ps))
		}
	}
	p := goodPolicy()
	p.Egress[0].EffectClass = egressgrant.EffectVisibleWrite
	p.Egress[0].Methods = []string{"POST"}
	p.Egress[0].OperationContract = "sha256:abc"
	if ps := Validate("calendar", p); len(ps) != 0 {
		t.Errorf("visible write with a contract refused: %v", ps)
	}
}

// Failure mode 4: data ceiling required and known.
func TestDataCeilingRequired(t *testing.T) {
	for _, ceiling := range []string{"", "secret"} {
		p := goodPolicy()
		p.Egress[0].DataCeiling = ceiling
		if ps := Validate("calendar", p); !hasCode(ps, CodeCeiling) {
			t.Errorf("ceiling %q: want %s, got %v", ceiling, CodeCeiling, codes(ps))
		}
	}
}

// Failure mode 5: the credential registry is empty, so any credential is
// refused.
func TestAnyCredentialIsRefused(t *testing.T) {
	p := goodPolicy()
	p.Egress[0].Credential = "google-calendar"
	if ps := Validate("calendar", p); !hasCode(ps, CodeCredential) {
		t.Fatalf("credential: want %s, got %v", CodeCredential, codes(ps))
	}
}

// Failure mode 6: budget and renewal.
func TestBudgetAndRenewal(t *testing.T) {
	budgets := []egressgrant.Budget{
		{},
		{MaxRequests: 1, MaxDownloadBytes: 1},
		{MaxRequests: egressgrant.MaxRequestsCap + 1, MaxDownloadBytes: 1, MaxUploadBytes: 1},
		{MaxRequests: 1, MaxDownloadBytes: egressgrant.MaxDownloadBytesCap + 1, MaxUploadBytes: 1},
	}
	for _, b := range budgets {
		p := goodPolicy()
		p.Egress[0].Budget = b
		if ps := Validate("calendar", p); !hasCode(ps, CodeGrant) {
			t.Errorf("budget %+v: want %s, got %v", b, CodeGrant, codes(ps))
		}
	}
	for _, days := range []int{0, -1, 91, 365} {
		p := goodPolicy()
		p.Egress[0].RenewalDays = days
		if ps := Validate("calendar", p); !hasCode(ps, CodeRenewal) {
			t.Errorf("renewal %d: want %s, got %v", days, CodeRenewal, codes(ps))
		}
	}
}

// Failure mode 7: entry caps.
func TestEntryCaps(t *testing.T) {
	p := Policy{}
	for i := 0; i < MaxEgressEntries+1; i++ {
		e := readEntry()
		e.Verb = "calendar.v" + string(rune('a'+i))
		p.Egress = append(p.Egress, e)
	}
	if ps := Validate("calendar", p); !hasCode(ps, CodeTooMany) {
		t.Errorf("egress cap: want %s, got %v", CodeTooMany, codes(ps))
	}
	p = Policy{}
	for i := 0; i < MaxIngressEntries+1; i++ {
		p.Ingress = append(p.Ingress, IngressEntry{Source: SourceMail, Address: "calendar." + string(rune('a'+i))})
	}
	if ps := Validate("calendar", p); !hasCode(ps, CodeTooMany) {
		t.Errorf("ingress cap: want %s, got %v", CodeTooMany, codes(ps))
	}
}

// Failure mode 8: ingress sources and addresses.
func TestIngressRules(t *testing.T) {
	for _, src := range []string{"", "webhook", "push", "tcp", "MAIL"} {
		p := goodPolicy()
		p.Ingress[0].Source = src
		if ps := Validate("calendar", p); !hasCode(ps, CodeIngressSource) {
			t.Errorf("source %q: want %s, got %v", src, CodeIngressSource, codes(ps))
		}
	}
	for _, addr := range []string{"", "owner", "mail", "calendarx", "calendar-", "calendar-invites", "calendar.", "calendar.-x", "calendar.a.b", "calendar.A", "calendar+x", "Calendar", "calendar.a b", "calendar@evil.test", "calendar." + strings.Repeat("a", 64)} {
		p := goodPolicy()
		p.Ingress[0].Address = addr
		if ps := Validate("calendar", p); !hasCode(ps, CodeIngressAddress) {
			t.Errorf("address %q: want %s, got %v", addr, CodeIngressAddress, codes(ps))
		}
	}
	p := goodPolicy()
	p.Ingress = append(p.Ingress, IngressEntry{Source: SourceMail, Address: "calendar.invites"})
	if ps := Validate("calendar", p); len(ps) != 0 {
		t.Errorf("suffixed address refused: %v", ps)
	}
	p.Ingress = append(p.Ingress, IngressEntry{Source: SourceMail, Address: "calendar"})
	if ps := Validate("calendar", p); !hasCode(ps, CodeDuplicate) {
		t.Errorf("duplicate ingress: want %s, got %v", CodeDuplicate, codes(ps))
	}
}

func TestBadPackageNameRefusesEverything(t *testing.T) {
	for _, pkg := range []string{"", "Calendar", "a", "calendar.x"} {
		if ps := Validate(pkg, goodPolicy()); !hasCode(ps, CodePackage) {
			t.Errorf("package %q: want %s, got %v", pkg, CodePackage, codes(ps))
		}
	}
}

// Failure mode 9: unknown fields are refused at parse, so a port, listener,
// forward or tunnel cannot be declared and ignored.
func TestParseRefusesUnknownFields(t *testing.T) {
	good := `{"egress":[{"verb":"calendar.sync","destinations":[{"scheme":"https","host":"www.googleapis.com"}],"effect_class":"read","data_ceiling":"public","budget":{"max_requests":1,"max_download_bytes":1,"max_upload_bytes":1},"renewal_days":1}],"ingress":[{"source":"mail","address":"calendar"}]}`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatalf("good policy did not parse: %v", err)
	}
	bad := []string{
		`{"ingress":[{"source":"mail","address":"calendar","port":8080}]}`,
		`{"ingress":[{"source":"mail","address":"calendar","listen":":25"}]}`,
		`{"ingress":[{"source":"mail","address":"calendar","host_service":"sshd"}]}`,
		`{"ingress":[{"source":"mail","address":"calendar","taint":"privileged"}]}`,
		`{"forward":{"from":443,"to":8085}}`,
		`{"tunnel":true}`,
		`{"egress":[{"verb":"calendar.sync","level":"L4"}]}`,
		`{"egress":[{"verb":"calendar.sync","holder":{"kind":"engineering_assignment","ref":"x"}}]}`,
		`{"egress":[]} {"egress":[]}`,
		`not json`,
	}
	for _, raw := range bad {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("parsed %s", raw)
		}
	}
}

// Failure mode 10: off unless the mode says declare.
func TestModeIsOffUnlessDeclare(t *testing.T) {
	for _, v := range []string{"", "on", "true", "1", "DECLARE", " declare", "enforce"} {
		if m := ModeFromEnv(func(string) string { return v }); m != ModeOff {
			t.Errorf("%q: mode %s, want off", v, m)
		}
	}
	if m := ModeFromEnv(func(k string) string {
		if k == EnvMode {
			return "declare"
		}
		return ""
	}); m != ModeDeclare {
		t.Errorf("declare: mode %s", m)
	}
	if ps := Admit(ModeOff, "calendar", goodPolicy()); !hasCode(ps, CodeOff) {
		t.Errorf("off admitted a policy: %v", ps)
	}
	if ps := Admit(Mode("weird"), "calendar", goodPolicy()); !hasCode(ps, CodeOff) {
		t.Errorf("unknown mode admitted a policy: %v", ps)
	}
	if ps := Admit(ModeOff, "calendar", Policy{}); len(ps) != 0 {
		t.Errorf("off refused an empty policy: %v", ps)
	}
	if ps := Admit(ModeDeclare, "calendar", goodPolicy()); len(ps) != 0 {
		t.Errorf("declare refused a good policy: %v", ps)
	}
	bad := goodPolicy()
	bad.Egress[0].Credential = "x"
	if ps := Admit(ModeDeclare, "calendar", bad); !hasCode(ps, CodeCredential) {
		t.Errorf("declare skipped validation: %v", ps)
	}
}

// Failure mode 11: anything not provably narrower is a widening.
func TestWideningIsConservative(t *testing.T) {
	widen := []struct {
		name string
		edit func(*Policy)
	}{
		{"new verb", func(p *Policy) {
			e := readEntry()
			e.Verb = "calendar.other"
			p.Egress = append(p.Egress, e)
		}},
		{"broader path", func(p *Policy) { p.Egress[0].Destinations[0].PathPrefix = "/" }},
		{"no path", func(p *Policy) { p.Egress[0].Destinations[0].PathPrefix = "" }},
		{"sibling path", func(p *Policy) { p.Egress[0].Destinations[0].PathPrefix = "/drive/v3/" }},
		{"new host", func(p *Policy) {
			p.Egress[0].Destinations = append(p.Egress[0].Destinations, egressgrant.Destination{Scheme: "https", Host: "oauth2.googleapis.com"})
		}},
		{"other port", func(p *Policy) { p.Egress[0].Destinations[0].Port = 8443 }},
		{"new method", func(p *Policy) { p.Egress[0].Methods = []string{"GET", "POST"} }},
		{"default methods add HEAD", func(p *Policy) { p.Egress[0].Methods = nil }},
		{"effect up", func(p *Policy) {
			p.Egress[0].EffectClass = egressgrant.EffectIdempotentWrite
		}},
		{"ceiling up", func(p *Policy) { p.Egress[0].DataCeiling = "third-party-private" }},
		{"credential changed", func(p *Policy) { p.Egress[0].Credential = "x" }},
		{"contract changed", func(p *Policy) { p.Egress[0].OperationContract = "sha256:new" }},
		{"requests up", func(p *Policy) { p.Egress[0].Budget.MaxRequests++ }},
		{"download up", func(p *Policy) { p.Egress[0].Budget.MaxDownloadBytes++ }},
		{"upload up", func(p *Policy) { p.Egress[0].Budget.MaxUploadBytes++ }},
		{"renewal longer", func(p *Policy) { p.Egress[0].RenewalDays = 31 }},
		{"new ingress", func(p *Policy) {
			p.Ingress = append(p.Ingress, IngressEntry{Source: SourceMail, Address: "calendar.x"})
		}},
		{"invalid new entry", func(p *Policy) { p.Egress[0].Destinations[0].Host = "*" }},
	}
	for _, c := range widen {
		next := goodPolicy()
		c.edit(&next)
		if !Widens(goodPolicy(), next) {
			t.Errorf("%s: not reported as widening: %+v", c.name, Widening(goodPolicy(), next))
		}
	}

	narrow := []struct {
		name string
		edit func(*Policy)
	}{
		{"unchanged", func(p *Policy) {}},
		{"verb removed", func(p *Policy) { p.Egress = nil }},
		{"ingress removed", func(p *Policy) { p.Ingress = nil }},
		{"deeper path", func(p *Policy) { p.Egress[0].Destinations[0].PathPrefix = "/calendar/v3/users/" }},
		{"host case and default port", func(p *Policy) {
			p.Egress[0].Destinations[0].Host = "WWW.googleapis.com"
			p.Egress[0].Destinations[0].Port = 443
		}},
		{"budget down", func(p *Policy) { p.Egress[0].Budget.MaxRequests = 10 }},
		{"renewal shorter", func(p *Policy) { p.Egress[0].RenewalDays = 7 }},
		{"ceiling down", func(p *Policy) { p.Egress[0].DataCeiling = "public" }},
	}
	for _, c := range narrow {
		next := goodPolicy()
		c.edit(&next)
		if Widens(goodPolicy(), next) {
			t.Errorf("%s: reported as widening: %+v", c.name, Widening(goodPolicy(), next))
		}
	}

	// Effect down is narrower: a visible write becomes a read.
	old := goodPolicy()
	old.Egress[0].EffectClass = egressgrant.EffectVisibleWrite
	old.Egress[0].OperationContract = "sha256:abc"
	next := goodPolicy()
	next.Egress[0].OperationContract = "sha256:abc"
	if Widens(old, next) {
		t.Errorf("effect down reported as widening: %+v", Widening(old, next))
	}

	// Every change names its key and at least one reason when it widens.
	next = goodPolicy()
	next.Egress[0].Budget.MaxRequests++
	for _, ch := range Widening(goodPolicy(), next) {
		if ch.Widens && (ch.Key != "calendar.sync" || len(ch.Reasons) == 0) {
			t.Errorf("widening change without key or reason: %+v", ch)
		}
	}
}

// Failure mode 12: requests are app-desk grants that egressgrant accepts.
func TestRequestsAreRequestedAppDeskGrants(t *testing.T) {
	gs, err := Requests(ModeDeclare, "calendar", "computer-a", goodPolicy(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 1 {
		t.Fatalf("%d grants, want 1", len(gs))
	}
	g := gs[0]
	if err := g.Validate(); err != nil {
		t.Fatalf("grant does not validate: %v", err)
	}
	if g.Disposition != egressgrant.DispositionRequested || g.Decision != nil {
		t.Errorf("grant is %s with decision %v, want requested and undecided", g.Disposition, g.Decision)
	}
	if g.Holder != (egressgrant.Holder{Kind: egressgrant.HolderAppDesk, Ref: "app:calendar"}) {
		t.Errorf("holder %+v", g.Holder)
	}
	if g.Level != egressgrant.LevelL1 || g.ComputerID != "computer-a" || g.Verb != "calendar.sync" {
		t.Errorf("level %s computer %s verb %s", g.Level, g.ComputerID, g.Verb)
	}
	if !g.NotBefore.Equal(t0) || !g.ExpiresAt.Equal(t0.Add(30*24*time.Hour)) {
		t.Errorf("window %s to %s", g.NotBefore, g.ExpiresAt)
	}
	// A requested grant allows nothing.
	if err := egressgrant.Check(g, egressgrant.Use{Level: egressgrant.LevelL1, Scheme: "https", Host: "www.googleapis.com", Method: "GET", Path: "/calendar/v3/x"}, t0.Add(time.Minute)); err == nil {
		t.Error("a requested grant allowed a use")
	}

	bad := goodPolicy()
	bad.Egress[0].Credential = "x"
	if _, err := Requests(ModeDeclare, "calendar", "computer-a", bad, t0); err == nil {
		t.Error("requests built from an invalid policy")
	}
	if _, err := Requests(ModeDeclare, "calendar", "", goodPolicy(), t0); err == nil {
		t.Error("requests built with no computer")
	}
	if gs, err := Requests(ModeDeclare, "calendar", "computer-a", Policy{Ingress: goodPolicy().Ingress}, t0); err != nil || len(gs) != 0 {
		t.Errorf("ingress-only policy: %d grants, %v", len(gs), err)
	}
}

// Ingress namespaces are disjoint: package names may contain "-" but never
// ".", so an address names exactly one package (the text before its first
// "."). Without that, package mail would claim mail-calendar-inbox, which
// belongs to package mail-calendar.
func TestIngressNamespacesAreDisjoint(t *testing.T) {
	mailPolicy := func(addr string) Policy {
		return Policy{Ingress: []IngressEntry{{Source: SourceMail, Address: addr}}}
	}
	if ps := Validate("mail", mailPolicy("mail-calendar-inbox")); !hasCode(ps, CodeIngressAddress) {
		t.Errorf("package mail claimed mail-calendar-inbox: %v", codes(ps))
	}
	if ps := Validate("mail", mailPolicy("mail-calendar.inbox")); !hasCode(ps, CodeIngressAddress) {
		t.Errorf("package mail claimed mail-calendar.inbox: %v", codes(ps))
	}
	if ps := Validate("mail-calendar", mailPolicy("mail-calendar.inbox")); len(ps) != 0 {
		t.Errorf("package mail-calendar refused its own address: %v", ps)
	}
	if ps := Validate("mail", mailPolicy("mail.calendar-inbox")); len(ps) != 0 {
		t.Errorf("package mail refused its own address: %v", ps)
	}

	pkgs := []string{"mail", "mail-calendar", "mail-calendar-inbox", "ma", "mail-c", "calendar", "calendar-x", "x1", "a-b", "a-b-c"}
	var addrs []string
	for _, p := range pkgs {
		addrs = append(addrs, p)
		for _, q := range pkgs {
			addrs = append(addrs, p+"."+q, p+"-"+q, p+"."+q+"."+q)
		}
	}
	for _, addr := range addrs {
		var owners []string
		for _, p := range pkgs {
			if len(Validate(p, mailPolicy(addr))) == 0 {
				owners = append(owners, p)
			}
		}
		if len(owners) > 1 {
			t.Errorf("address %q is in %d namespaces: %v", addr, len(owners), owners)
		}
	}
}

// Admit is the only path: with the mode off, no grant request is minted,
// even for a valid policy.
func TestRequestsRefusedWithModeOff(t *testing.T) {
	for _, m := range []Mode{ModeOff, "", "on", "weird"} {
		if gs, err := Requests(m, "calendar", "computer-a", goodPolicy(), t0); err == nil || len(gs) != 0 {
			t.Errorf("mode %q: %d grants, err %v; want refusal", m, len(gs), err)
		}
	}
	if gs, err := Requests(ModeOff, "calendar", "computer-a", Policy{}, t0); err != nil || len(gs) != 0 {
		t.Errorf("mode off, empty policy: %d grants, err %v; want none and no error", len(gs), err)
	}
}
