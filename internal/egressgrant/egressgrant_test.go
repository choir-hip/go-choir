package egressgrant

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
)

var t0 = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

func keys(t *testing.T) (*owneract.Signer, *owneract.Verifier) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := owneract.NewVerifier(public)
	if err != nil {
		t.Fatal(err)
	}
	return signer, verifier
}

func engineeringL3() Declaration {
	return Declaration{
		Holder:       Holder{Kind: HolderEngineeringAssignment, Ref: "assignment-1"},
		ComputerID:   "computer-a",
		Level:        LevelL3,
		Purpose:      "read the paper's API docs",
		Destinations: []Destination{{Scheme: "https", Host: "API.Example.com"}},
		Budget:       Budget{MaxRequests: 100, MaxDownloadBytes: 1 << 20, MaxUploadBytes: 1 << 10},
		NotBefore:    t0,
		ExpiresAt:    t0.Add(2 * time.Hour),
	}
}

func appDeskL3() Declaration {
	return Declaration{
		Holder:            Holder{Kind: HolderAppDesk, Ref: "mail"},
		ComputerID:        "computer-a",
		Level:             LevelL3,
		Destinations:      []Destination{{Scheme: "https", Host: "api.resend.com", PathPrefix: "/emails"}},
		Methods:           []string{"POST"},
		Budget:            Budget{MaxRequests: 1000, MaxDownloadBytes: 1 << 20, MaxUploadBytes: 1 << 20},
		NotBefore:         t0,
		ExpiresAt:         t0.Add(30 * 24 * time.Hour),
		Verb:              "mail.send",
		EffectClass:       EffectVisibleWrite,
		OperationContract: "sha256:abc",
		DataCeiling:       "owner-private",
		Credential:        "resend",
	}
}

func mint(t *testing.T, s *owneract.Signer, computerID, kind string, subject map[string]any, at time.Time) computerevent.Receipt {
	t.Helper()
	receipt, err := s.Mint(owneract.Act{
		Kind: kind, OwnerUserID: "owner-1", AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session-jwt-sha256:x", ComputerID: computerID, Subject: subject,
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func requested(t *testing.T, d Declaration) Grant {
	t.Helper()
	g, err := Request(d, t0.Add(-time.Minute))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return g
}

func granted(t *testing.T, s *owneract.Signer, v *owneract.Verifier, d Declaration) Grant {
	t.Helper()
	g := requested(t, d)
	receipt := mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": VerdictGrant}, t0)
	out, err := Decide(g, VerdictGrant, receipt, v, t0)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	return out
}

// Both holder kinds share one request, validate, decide and check path.
func TestOneModelForEngineeringAndAppDesk(t *testing.T) {
	s, v := keys(t)
	eng := granted(t, s, v, engineeringL3())
	if eng.Disposition != DispositionGranted || eng.Decision == nil || eng.Decision.OwnerUserID != "owner-1" || eng.Decision.OwnerActDigest == "" {
		t.Fatalf("engineering grant not recorded: %+v", eng)
	}
	if !strings.HasPrefix(eng.GrantID, "egress-grant:sha256:") || eng.Schema != Schema {
		t.Fatalf("grant id or schema: %q %q", eng.GrantID, eng.Schema)
	}
	if err := Check(eng, Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET", Path: "/v1"}, t0.Add(time.Hour)); err != nil {
		t.Fatalf("engineering use refused: %v", err)
	}
	desk := granted(t, s, v, appDeskL3())
	if err := Check(desk, Use{Level: LevelL3, Scheme: "https", Host: "api.resend.com", Method: "POST", Path: "/emails/batch"}, t0.Add(time.Hour)); err != nil {
		t.Fatalf("app desk use refused: %v", err)
	}
	if err := desk.Validate(); err != nil {
		t.Fatalf("granted record does not re-validate: %v", err)
	}
}

func TestRequestNormalizes(t *testing.T) {
	d := engineeringL3()
	d.NotBefore = t0.Add(500 * time.Millisecond).In(time.FixedZone("x", 3600))
	g := requested(t, d)
	if g.Destinations[0].Host != "api.example.com" || g.Destinations[0].Port != 443 {
		t.Fatalf("destination not normalized: %+v", g.Destinations[0])
	}
	if strings.Join(g.Methods, ",") != "GET,HEAD" {
		t.Fatalf("default methods: %v", g.Methods)
	}
	if g.NotBefore.Location() != time.UTC || g.NotBefore.Nanosecond() != 0 || g.Disposition != DispositionRequested {
		t.Fatalf("window or disposition: %v %s", g.NotBefore, g.Disposition)
	}
	again := requested(t, d)
	if again.GrantID != g.GrantID {
		t.Fatalf("grant id is not deterministic")
	}
}

func TestRequestRefusesInvalidDeclarations(t *testing.T) {
	cases := map[string]func(*Declaration){
		"no holder":         func(d *Declaration) { d.Holder = Holder{} },
		"unknown holder":    func(d *Declaration) { d.Holder.Kind = "research" },
		"no computer":       func(d *Declaration) { d.ComputerID = " " },
		"level L2":          func(d *Declaration) { d.Level = "L2" },
		"level empty":       func(d *Declaration) { d.Level = "" },
		"no destinations":   func(d *Declaration) { d.Destinations = nil },
		"ipv4 literal":      func(d *Declaration) { d.Destinations[0].Host = "93.184.216.34" },
		"metadata":          func(d *Declaration) { d.Destinations[0].Host = "169.254.169.254" },
		"ipv6 literal":      func(d *Declaration) { d.Destinations[0].Host = "[::1]" },
		"localhost":         func(d *Declaration) { d.Destinations[0].Host = "localhost" },
		"dot internal":      func(d *Declaration) { d.Destinations[0].Host = "goproxy.choir.internal" },
		"dot local":         func(d *Declaration) { d.Destinations[0].Host = "printer.local" },
		"dot localhost":     func(d *Declaration) { d.Destinations[0].Host = "a.localhost" },
		"single label":      func(d *Declaration) { d.Destinations[0].Host = "intranet" },
		"bad host chars":    func(d *Declaration) { d.Destinations[0].Host = "a b.com" },
		"hyphen label":      func(d *Declaration) { d.Destinations[0].Host = "-a.example.com" },
		"partial wildcard":  func(d *Declaration) { d.Destinations[0].Host = "*.example.com" },
		"http at L3":        func(d *Declaration) { d.Destinations[0].Scheme = "http" },
		"tcp at L3":         func(d *Declaration) { d.Destinations[0].Scheme = "tcp" },
		"bad port":          func(d *Declaration) { d.Destinations[0].Port = 70000 },
		"relative path":     func(d *Declaration) { d.Destinations[0].PathPrefix = "emails" },
		"bad method":        func(d *Declaration) { d.Methods = []string{"CONNECT"} },
		"zero requests":     func(d *Declaration) { d.Budget.MaxRequests = 0 },
		"negative download": func(d *Declaration) { d.Budget.MaxDownloadBytes = -1 },
		"zero upload":       func(d *Declaration) { d.Budget.MaxUploadBytes = 0 },
		"requests over cap": func(d *Declaration) { d.Budget.MaxRequests = MaxRequestsCap + 1 },
		"download over cap": func(d *Declaration) { d.Budget.MaxDownloadBytes = MaxDownloadBytesCap + 1 },
		"upload over cap":   func(d *Declaration) { d.Budget.MaxUploadBytes = MaxUploadBytesCap + 1 },
		"no expiry":         func(d *Declaration) { d.ExpiresAt = time.Time{} },
		"no not_before":     func(d *Declaration) { d.NotBefore = time.Time{} },
		"expiry before":     func(d *Declaration) { d.ExpiresAt = d.NotBefore },
		"assignment too long": func(d *Declaration) {
			d.ExpiresAt = d.NotBefore.Add(MaxAssignmentLifetime + time.Second)
		},
		"unknown condition":         func(d *Declaration) { d.Conditions = []string{"trust_me"} },
		"app fields on engineering": func(d *Declaration) { d.Verb = "mail.send" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := engineeringL3()
			d.Destinations = append([]Destination(nil), d.Destinations...)
			mutate(&d)
			if _, err := Request(d, t0); err == nil {
				t.Fatalf("accepted")
			}
		})
	}
}

func TestLevelRules(t *testing.T) {
	// Wildcard needs L3 or L4.
	d := engineeringL3()
	d.Level = LevelL1
	d.Destinations = []Destination{{Scheme: "https", Host: "*"}}
	if _, err := Request(d, t0); err == nil {
		t.Fatal("wildcard at L1 accepted")
	}
	d.Level = LevelL3
	if _, err := Request(d, t0); err != nil {
		t.Fatalf("wildcard at L3 refused: %v", err)
	}
	// L4: tcp/udp only, needs no_private_data_mounted.
	d4 := engineeringL3()
	d4.Level = LevelL4
	d4.Destinations = []Destination{{Scheme: "tcp", Host: "*"}}
	if _, err := Request(d4, t0); err == nil {
		t.Fatal("L4 without no_private_data_mounted accepted")
	}
	d4.Conditions = []string{ConditionNoPrivateData}
	if _, err := Request(d4, t0); err != nil {
		t.Fatalf("L4 with condition refused: %v", err)
	}
	d4.Destinations = []Destination{{Scheme: "https", Host: "*"}}
	if _, err := Request(d4, t0); err == nil {
		t.Fatal("https scheme at L4 accepted")
	}
}

func TestAppDeskRules(t *testing.T) {
	cases := map[string]func(*Declaration){
		"no verb":                        func(d *Declaration) { d.Verb = "" },
		"no effect class":                func(d *Declaration) { d.EffectClass = "" },
		"unknown effect class":           func(d *Declaration) { d.EffectClass = "write" },
		"visible-write without contract": func(d *Declaration) { d.OperationContract = "" },
		"unknown data ceiling":           func(d *Declaration) { d.DataCeiling = "secret" },
		"wildcard":                       func(d *Declaration) { d.Destinations = []Destination{{Scheme: "https", Host: "*"}} },
		"L4": func(d *Declaration) {
			d.Level = LevelL4
			d.Destinations = []Destination{{Scheme: "tcp", Host: "smtp.example.com", Port: 25}}
			d.Conditions = []string{ConditionNoPrivateData}
		},
		"lifetime over cap": func(d *Declaration) { d.ExpiresAt = d.NotBefore.Add(MaxAppDeskLifetime + time.Second) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := appDeskL3()
			mutate(&d)
			if _, err := Request(d, t0); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	d := appDeskL3()
	d.EffectClass = EffectRead
	d.OperationContract = ""
	if _, err := Request(d, t0); err != nil {
		t.Fatalf("read without contract refused: %v", err)
	}
}

func TestDecideRequiresTheExactOwnerAct(t *testing.T) {
	s, v := keys(t)
	other, _ := keys(t)
	g := requested(t, engineeringL3())
	good := map[string]any{"grant_id": g.GrantID, "decision": VerdictGrant}
	cases := map[string]struct {
		receipt  computerevent.Receipt
		verifier *owneract.Verifier
		verdict  string
		now      time.Time
	}{
		"no verifier":       {mint(t, s, g.ComputerID, ActDecide, good, t0), nil, VerdictGrant, t0},
		"empty receipt":     {computerevent.Receipt{}, v, VerdictGrant, t0},
		"other key":         {mint(t, other, g.ComputerID, ActDecide, good, t0), v, VerdictGrant, t0},
		"other act kind":    {mint(t, s, g.ComputerID, owneract.ActSelfDevApprove, good, t0), v, VerdictGrant, t0},
		"other computer":    {mint(t, s, "computer-b", ActDecide, good, t0), v, VerdictGrant, t0},
		"expired receipt":   {mint(t, s, g.ComputerID, ActDecide, good, t0), v, VerdictGrant, t0.Add(owneract.TTL + time.Second)},
		"other grant":       {mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": "egress-grant:sha256:00", "decision": VerdictGrant}, t0), v, VerdictGrant, t0},
		"decision mismatch": {mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": VerdictDeny}, t0), v, VerdictGrant, t0},
		"bad verdict":       {mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": "maybe"}, t0), v, "maybe", t0},
		"extra subject":     {mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": VerdictGrant, "level": "L4"}, t0), v, VerdictGrant, t0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := Decide(g, c.verdict, c.receipt, c.verifier, c.now)
			if err == nil {
				t.Fatal("accepted")
			}
			if out.Disposition != "" && out.Disposition != DispositionRequested {
				t.Fatalf("state changed on refusal: %s", out.Disposition)
			}
		})
	}
	denied, err := Decide(g, VerdictDeny, mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": VerdictDeny}, t0), v, t0)
	if err != nil || denied.Disposition != DispositionDenied {
		t.Fatalf("deny: %v %s", err, denied.Disposition)
	}
	if err := Check(denied, Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET"}, t0); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("denied grant checked: %v", err)
	}
}

func TestDecideOnlyFromRequested(t *testing.T) {
	s, v := keys(t)
	g := granted(t, s, v, engineeringL3())
	receipt := mint(t, s, g.ComputerID, ActDecide, map[string]any{"grant_id": g.GrantID, "decision": VerdictGrant}, t0)
	if _, err := Decide(g, VerdictGrant, receipt, v, t0); err == nil {
		t.Fatal("re-decided a granted grant")
	}
	revoked, err := Revoke(g, "owner revoked", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decide(revoked, VerdictGrant, receipt, v, t0.Add(time.Minute)); err == nil {
		t.Fatal("replayed receipt re-granted a revoked grant")
	}
	if _, err := Revoke(revoked, "again", t0.Add(2*time.Minute)); err == nil {
		t.Fatal("revoked a closed grant")
	}
}

func TestMutatedRecordIsRefused(t *testing.T) {
	s, v := keys(t)
	g := granted(t, s, v, engineeringL3())
	g.Destinations = []Destination{{Scheme: "https", Host: "evil.example.net", Port: 443}}
	if err := g.Validate(); err == nil {
		t.Fatal("mutated scope validated")
	}
	if err := Check(g, Use{Level: LevelL3, Scheme: "https", Host: "evil.example.net", Method: "GET"}, t0.Add(time.Minute)); err == nil {
		t.Fatal("mutated scope allowed a use")
	}
	g2 := granted(t, s, v, engineeringL3())
	g2.ExpiresAt = g2.ExpiresAt.Add(time.Hour)
	if err := Check(g2, Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET"}, t0.Add(2*time.Hour+time.Minute)); err == nil {
		t.Fatal("extended expiry allowed a use")
	}
	g3 := requested(t, engineeringL3())
	g3.Disposition = DispositionGranted
	if err := Check(g3, Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET"}, t0.Add(time.Minute)); err == nil {
		t.Fatal("granted disposition without a decision allowed a use")
	}
}

func TestCheckWindowAndScope(t *testing.T) {
	s, v := keys(t)
	d := engineeringL3()
	d.NotBefore = t0.Add(10 * time.Minute)
	d.ExpiresAt = t0.Add(70 * time.Minute)
	g := granted(t, s, v, d)
	ok := Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET"}
	if err := Check(g, ok, t0.Add(5*time.Minute)); !errors.Is(err, ErrNotYetValid) {
		t.Fatalf("before not_before: %v", err)
	}
	if err := Check(g, ok, t0.Add(10*time.Minute)); err != nil {
		t.Fatalf("at not_before: %v", err)
	}
	if err := Check(g, ok, t0.Add(70*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Fatalf("at expires_at: %v", err)
	}
	now := t0.Add(30 * time.Minute)
	scope := map[string]Use{
		"other host":   {Level: LevelL3, Scheme: "https", Host: "api.example.org", Method: "GET"},
		"subdomain":    {Level: LevelL3, Scheme: "https", Host: "x.api.example.com", Method: "GET"},
		"other port":   {Level: LevelL3, Scheme: "https", Host: "api.example.com", Port: 8443, Method: "GET"},
		"other scheme": {Level: LevelL3, Scheme: "http", Host: "api.example.com", Method: "GET"},
		"method":       {Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "POST"},
		"other level":  {Level: LevelL4, Scheme: "https", Host: "api.example.com", Method: "GET"},
		"metadata":     {Level: LevelL3, Scheme: "https", Host: "169.254.169.254", Method: "GET"},
	}
	for name, use := range scope {
		if err := Check(g, use, now); !errors.Is(err, ErrOutOfScope) {
			t.Errorf("%s: %v", name, err)
		}
	}
	desk := granted(t, s, v, appDeskL3())
	if err := Check(desk, Use{Level: LevelL3, Scheme: "https", Host: "api.resend.com", Method: "POST", Path: "/domains"}, now); !errors.Is(err, ErrOutOfScope) {
		t.Fatalf("path outside prefix: %v", err)
	}
	if err := Check(desk, Use{Level: LevelL3, Scheme: "https", Host: "api.resend.com", Method: "POST", Path: "/emailsx"}, now); !errors.Is(err, ErrOutOfScope) {
		t.Fatalf("path prefix is not segment-bounded: %v", err)
	}
	// Wildcard still refuses names a grant could never name.
	wd := engineeringL3()
	wd.Destinations = []Destination{{Scheme: "https", Host: "*"}}
	w := granted(t, s, v, wd)
	if err := Check(w, Use{Level: LevelL3, Scheme: "https", Host: "docs.python.org", Method: "GET"}, now); err != nil {
		t.Fatalf("wildcard public host: %v", err)
	}
	for _, host := range []string{"localhost", "10.0.0.1", "metadata.google.internal", ""} {
		if err := Check(w, Use{Level: LevelL3, Scheme: "https", Host: host, Method: "GET"}, now); !errors.Is(err, ErrOutOfScope) {
			t.Errorf("wildcard allowed %q: %v", host, err)
		}
	}
}

func TestCloseNarrowsOnly(t *testing.T) {
	s, v := keys(t)
	g := granted(t, s, v, engineeringL3())
	for _, disposition := range []string{DispositionGranted, DispositionRequested, DispositionDenied, "bogus"} {
		if _, err := Close(g, disposition, "x", t0); err == nil {
			t.Errorf("closed to %q", disposition)
		}
	}
	exhausted, err := Close(g, DispositionExhausted, "budget", t0.Add(time.Minute))
	if err != nil || exhausted.Disposition != DispositionExhausted || exhausted.ClosedAt.IsZero() {
		t.Fatalf("exhaust: %v %+v", err, exhausted)
	}
	if err := Check(exhausted, Use{Level: LevelL3, Scheme: "https", Host: "api.example.com", Method: "GET"}, t0.Add(2*time.Minute)); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("exhausted grant checked: %v", err)
	}
	// A crash boot closes an undecided request as interrupted.
	r := requested(t, engineeringL3())
	interrupted, err := Close(r, DispositionInterrupted, ReasonRestart, t0)
	if err != nil || interrupted.Disposition != DispositionInterrupted || interrupted.CloseReason != ReasonRestart {
		t.Fatalf("interrupt: %v %+v", err, interrupted)
	}
}

// A stored record is re-read from JSON; it must still validate and check.
func TestJSONRoundTrip(t *testing.T) {
	s, v := keys(t)
	g := granted(t, s, v, appDeskL3())
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back Grant
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if err := Check(back, Use{Level: LevelL3, Scheme: "https", Host: "api.resend.com", Method: "POST", Path: "/emails"}, t0.Add(time.Hour)); err != nil {
		t.Fatalf("round-tripped grant: %v", err)
	}
}
