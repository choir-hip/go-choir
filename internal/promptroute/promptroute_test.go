package promptroute

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// MA4.8 router decision rule. Ways it could fail (written first):
//
//  R1 An answer adds a destination: a probability for a system desk, a role,
//     an unregistered or a candidate desk is taken.
//  R2 A middling distribution routes away from the default (no threshold).
//  R3 An empty or garbage distribution (NaN, Inf, nothing for a candidate)
//     routes anywhere but the default.
//  R4 A system desk, a role, a candidate desk, a desk with no accepts or a
//     package the computer has not installed is a candidate, or a focus may
//     name one.
//  R5 The logged distribution is not clamped.
//  R6 The accepts shape admits junk (control characters, duplicates, an
//     unknown field, a request kind the router would not send).
//  R7 An abstained route offers the default it already took as a one-tap
//     alternative (review r1 item 8).
//  R8 A package desk's focus is taken without a ref, with an oversized or
//     non-canonical ref, with another owner's object, or with a kind the
//     desk is not the custodian of (review r1 item 1).

const def = "texture-default"

func registerApp(t *testing.T, slug string, uid uint32, accepts ...string) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:" + slug, Verbs: []string{"Cast"},
		Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: 64 << 20, Accepts: accepts})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
}

func TestCandidatesAreTheDefaultAndRoutablePackageDesks(t *testing.T) {
	registerApp(t, "routeme", 62901, "plan a trip")
	registerApp(t, "silent", 62902) // no accepts: never a candidate
	registerApp(t, "routeme-candidate", 62903, "plan a trip")
	got := Candidates(def)
	if len(got) != 2 || !got[0].Default || got[0].Desk != def || got[1].Desk != "app:routeme" || got[1].App != "routeme" {
		t.Fatalf("candidates = %+v", got)
	}
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind == deskmanifest.KindApp {
			continue
		}
		if Addressable(def, name) {
			t.Errorf("a %s desk %s is addressable", d.Kind, name)
		}
	}
	for _, name := range []string{"app:silent", "app:routeme"} {
		if !Addressable(def, name) {
			t.Errorf("%s is not addressable", name)
		}
	}
	for _, name := range []string{"app:routeme-candidate", "app:not-installed", "", "management:owner"} {
		if Addressable(def, name) {
			t.Errorf("%q is addressable", name)
		}
	}
}

func TestDecideTakesOnlyCandidatesAboveTheta(t *testing.T) {
	cands := []Candidate{{Desk: def, App: def, Default: true}, {Desk: "app:mail", App: "mail", Intents: []string{"email"}}}
	// R1: a forbidden name's probability is dropped, whatever its size.
	d := Decide(cands, map[string]float64{"management": 1, "engineering": 0.99, "app:mail": 0.2, def: 0.1}, Theta)
	if d.Destination != def || d.Rule != RuleAbstain {
		t.Fatalf("R1: %+v", d)
	}
	if _, ok := d.Distribution["management"]; ok {
		t.Fatalf("R1: a non-candidate reached the distribution: %v", d.Distribution)
	}
	// R2: below theta abstains to the default and offers the top choices.
	d = Decide(cands, map[string]float64{"app:mail": 0.55, def: 0.45}, Theta)
	if d.Destination != def || d.Rule != RuleAbstain || len(d.Alternatives) != 1 || d.Alternatives[0] != "app:mail" {
		t.Fatalf("R2/R7: %+v", d)
	}
	d = Decide(cands, map[string]float64{"app:mail": 0.8, def: 0.2}, Theta)
	if d.Destination != "app:mail" || d.App != "mail" || d.Rule != RuleJudge || d.Probability != 0.8 {
		t.Fatalf("judge route: %+v", d)
	}
	// R3
	for _, given := range []map[string]float64{nil, {}, {"nobody": 1}} {
		if d := Decide(cands, given, Theta); d.Destination != def || d.Rule != RuleNoAnswer {
			t.Fatalf("R3 %v: %+v", given, d)
		}
	}
	if d := Decide(cands, map[string]float64{"app:mail": math.NaN(), def: math.Inf(1)}, Theta); d.Destination != def || d.Rule != RuleNoAnswer {
		t.Fatalf("R3 NaN/Inf: %+v", d)
	}
	// R5: clamped.
	d = Decide(cands, map[string]float64{"app:mail": 1, def: 0}, Theta)
	if d.Distribution["app:mail"] != ClampHigh || d.Distribution[def] != ClampLow {
		t.Fatalf("R5: %v", d.Distribution)
	}
}

func TestAcceptsShape(t *testing.T) {
	good := `["summarize a thread", {"intent": "reply to an email"}]`
	list, err := ParseAccepts(json.RawMessage(good))
	if err != nil || len(list) != 2 || strings.Join(Intents(list), "|") != "summarize a thread|reply to an email" {
		t.Fatalf("good: %v %+v", err, list)
	}
	for name, raw := range map[string]string{
		"not a list":    `{"intent":"x"}`,
		"null":          `null`,
		"number":        `[1]`,
		"empty intent":  `[""]`,
		"padded":        `[" x"]`,
		"control char":  `["a\u0007b"]`,
		"duplicate":     `["x","x"]`,
		"request kind":  `[{"intent":"x","request_kind":"ask"}]`,
		"unknown field": `[{"intent":"x","grant":"world.read"}]`,
		"too long":      `["` + strings.Repeat("a", MaxIntentRunes+1) + `"]`,
	} {
		if _, err := ParseAccepts(json.RawMessage(raw)); err == nil {
			t.Errorf("R6 %s: accepted", name)
		}
	}
}

// R8
func TestFocusKindNeedsTheOwnersObjectOfAKindTheDeskKeeps(t *testing.T) {
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:focuskind", Verbs: []string{"Cast"},
		Taint: deskmanifest.TaintUntrustedInput, UID: 62904, WorkerMaxBytes: 64 << 20, OwnedKinds: []string{"choir.app.focuskind.note"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:focuskind") })
	registerApp(t, "focusnone", 62905, "notes")
	own, err := objectgraph.BuildCanonicalID("choir.app.focuskind.note", "owner-1", "n1")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := objectgraph.BuildCanonicalID("texture.document", "owner-1", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if kind, err := FocusKind("app:focuskind", "owner-1", own); err != nil || kind != "choir.app.focuskind.note" {
		t.Fatalf("own note: %q %v", kind, err)
	}
	for name, c := range map[string][3]string{
		"empty":         {"app:focuskind", "owner-1", ""},
		"oversized":     {"app:focuskind", "owner-1", own + strings.Repeat("a", MaxObjectRefBytes)},
		"not canonical": {"app:focuskind", "owner-1", "msg-42"},
		"other owner":   {"app:focuskind", "owner-2", own},
		"foreign kind":  {"app:focuskind", "owner-1", foreign},
		"owns no kind":  {"app:focusnone", "owner-1", own},
		"not routable":  {"app:missing", "owner-1", own},
	} {
		if _, err := FocusKind(c[0], c[1], c[2]); err == nil {
			t.Errorf("R8 %s: accepted", name)
		}
	}
}
