package appdesk

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Failure modes (MA4.3, B3, written first):
//
//	K1 The candidate takes the installed package's kinds, so a candidate run
//	   writes real data. -> a candidate's kinds are choir.app.<pkg>-candidate.*
//	   only; a candidate manifest naming an installed kind is refused.
//	K2 An installed package is named like a candidate. -> reserved suffix.
//	K3 A candidate of nothing (no installed package), or two at once.
//	   -> refused.
//	K4 Acceptance leaves the shadow behind or moves data wrongly.
//	   -> supplant updates the installed desk under the installed names, drops
//	      the candidate desk and retires the shadow kinds.
//	K5 Rejection (drop) leaves the candidate desk registered.
//	   -> the reconcile retires it.
//	K6 The candidate has egress, or an inbound route. -> the manifest's egress
//	   is refused; the candidate desk is an app desk, which has no inbox.
func candidateManifest(base string) Manifest {
	cand := objectgraph.CandidateName(base)
	m := goodManifest(cand)
	m.OwnedKinds = []string{"choir.app." + cand + ".episode_note"}
	return m
}

func candidateEvent(seq uint64, base string, m Manifest) Event {
	return Event{Seq: seq, Action: ActionCandidate, Package: base, ManifestDigest: Digest(m), Manifest: &m}
}

func TestInstalledNamesCannotTakeTheCandidateSuffix(t *testing.T) {
	m := goodManifest("shadowy-candidate")
	m.OwnedKinds = []string{"choir.app.shadowy-candidate.episode_note"}
	if rs := Validate(m); !hasCode(rs, CodeReservedSuffix) {
		t.Fatalf("an installed package named like a candidate: %+v", rs)
	}
	if _, refusals := Derive([]Event{install(1, m)}); !hasCode(refusals, CodeReservedSuffix) {
		t.Fatalf("the install event of a candidate-named package: %+v", refusals)
	}
	long := goodManifest(strings.Repeat("a", 23))
	long.OwnedKinds = []string{"choir.app." + long.Package + ".episode_note"}
	if rs := Validate(long); !hasCode(rs, CodeTooLongForCandidate) {
		t.Fatalf("a name too long to have a candidate: %+v", rs)
	}
}

func TestCandidateRunsBesideTheInstalledDeskAndSupplantsOnAcceptance(t *testing.T) {
	inst := goodManifest("candpkg")
	cm := candidateManifest("candpkg")
	cm.Verbs = []string{"ReadCapture", "Cast"}
	set, refusals := Derive([]Event{install(1, inst), candidateEvent(2, "candpkg", cm)})
	if len(refusals) != 0 {
		t.Fatalf("refusals %+v", refusals)
	}
	if set.Candidates["candpkg"] != "candpkg-candidate" || len(set.Desks) != 2 {
		t.Fatalf("candidates %v desks %d", set.Candidates, len(set.Desks))
	}
	byName := map[string]deskmanifest.Desk{}
	for _, d := range set.Desks {
		byName[d.Name] = d
	}
	cd, id := byName["app:candpkg-candidate"], byName["app:candpkg"]
	if cd.Name == "" || id.Name == "" {
		t.Fatalf("desks %v", byName)
	}
	if cd.Sandbox.UID == id.Sandbox.UID {
		t.Error("the candidate shares the installed desk's worker uid")
	}
	for _, k := range cd.OwnedKinds {
		if !strings.HasPrefix(k, "choir.app.candpkg-candidate.") {
			t.Errorf("candidate kind %s is not a shadow kind", k)
		}
	}
	if w, live, ok := set.Writer("choir.app.candpkg-candidate.episode_note"); !ok || !live || w != "desk:candpkg-candidate" {
		t.Errorf("shadow writer %q %v %v", w, live, ok)
	}
	if w, _, _ := set.Writer("choir.app.candpkg.episode_note"); w != "desk:candpkg" {
		t.Errorf("the installed kind's writer moved to %q", w)
	}
	if cd.Instance != deskmanifest.InstanceNone {
		t.Error("a candidate has no inbox")
	}

	// Acceptance supplants and drops the shadow.
	accept := Event{Seq: 3, Action: ActionSupplant, Package: "candpkg"}
	set, refusals = Derive([]Event{install(1, inst), candidateEvent(2, "candpkg", cm), accept})
	if len(refusals) != 0 {
		t.Fatalf("acceptance refused: %+v", refusals)
	}
	if len(set.Candidates) != 0 || len(set.Desks) != 1 || set.Desks[0].Name != "app:candpkg" {
		t.Fatalf("after acceptance: candidates %v desks %v", set.Candidates, set.Desks)
	}
	if got := set.Desks[0].Verbs; len(got) != 2 {
		t.Errorf("the installed desk did not take the candidate's verbs: %v", got)
	}
	if set.Epochs["app:candpkg"] != 3 {
		t.Errorf("epoch %d, want the supplant's seq 3", set.Epochs["app:candpkg"])
	}
	if w, live, ok := set.Writer("choir.app.candpkg-candidate.episode_note"); !ok || live || w == "" {
		t.Errorf("the shadow kind must be retired, got %q %v %v", w, live, ok)
	}
	if w, live, _ := set.Writer("choir.app.candpkg.episode_note"); !live || w != "desk:candpkg" {
		t.Errorf("the installed kind's writer after acceptance: %q %v", w, live)
	}
	// A rejected candidate leaves the installed desk as it was.
	set, refusals = Derive([]Event{install(1, inst), candidateEvent(2, "candpkg", cm), {Seq: 3, Action: ActionDrop, Package: "candpkg"}})
	if len(refusals) != 0 || len(set.Desks) != 1 || len(set.Desks[0].Verbs) != 4 || set.Epochs["app:candpkg"] != 1 {
		t.Fatalf("after a drop: %+v desks %v epochs %v", refusals, set.Desks, set.Epochs)
	}
	// A new candidate after a drop re-uses the shadow kinds and the uid.
	set2, refusals := Derive([]Event{install(1, inst), candidateEvent(2, "candpkg", cm), {Seq: 3, Action: ActionDrop, Package: "candpkg"}, candidateEvent(4, "candpkg", cm)})
	if len(refusals) != 0 || len(set2.Candidates) != 1 {
		t.Fatalf("second candidate: %+v", refusals)
	}
}

func TestCandidateRefusals(t *testing.T) {
	inst := goodManifest("candref")
	cm := candidateManifest("candref")
	if _, rs := Derive([]Event{candidateEvent(2, "candref", cm)}); !hasCode(rs, CodeNotInstalled) {
		t.Errorf("a candidate of nothing: %+v", rs)
	}
	if _, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", cm), candidateEvent(3, "candref", cm)}); !hasCode(rs, CodeCandidateLive) {
		t.Errorf("two candidates: %+v", rs)
	}
	takes := cm
	takes.OwnedKinds = []string{"choir.app.candref.episode_note"}
	if _, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", takes)}); !hasCode(rs, CodeKindNotNamespaced) {
		t.Errorf("a candidate that names the installed kind: %+v", rs)
	}
	egress := cm
	egress.Egress = []string{"example.com"}
	if _, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", egress)}); !hasCode(rs, CodeEgressNotGranted) {
		t.Errorf("a candidate with egress: %+v", rs)
	}
	wrongName := goodManifest("candref-other")
	if _, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", wrongName)}); !hasCode(rs, CodeDigestMismatch) {
		t.Errorf("a candidate manifest for another name: %+v", rs)
	}
	priv := cm
	priv.Verbs = []string{"Exec"}
	if _, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", priv)}); !hasCode(rs, CodeVerbNotApp) {
		t.Errorf("a candidate with a privileged verb: %+v", rs)
	}
	if _, rs := Derive([]Event{install(1, inst), {Seq: 2, Action: ActionSupplant, Package: "candref"}}); !hasCode(rs, CodeNoCandidate) {
		t.Errorf("acceptance with no candidate: %+v", rs)
	}
	if _, rs := Derive([]Event{install(1, inst), {Seq: 2, Action: ActionDrop, Package: "candref"}}); !hasCode(rs, CodeNoCandidate) {
		t.Errorf("drop with no candidate: %+v", rs)
	}
	// Disabling the installed package takes its candidate with it.
	set, rs := Derive([]Event{install(1, inst), candidateEvent(2, "candref", cm), disable(3, "candref")})
	if len(rs) != 0 || len(set.Desks) != 0 || len(set.Candidates) != 0 {
		t.Errorf("disable with a candidate: %+v %v", rs, set.Desks)
	}
}

func TestReconcileRunsTheCandidateBesideAndRetiresIt(t *testing.T) {
	inst := goodManifest("candrec")
	cm := candidateManifest("candrec")
	t.Cleanup(func() { cleanupAll("app:candrec", "app:candrec-candidate") })
	events := []Event{install(1, inst), candidateEvent(2, "candrec", cm)}
	set, _ := Derive(events)
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"app:candrec", "app:candrec-candidate"} {
		if _, ok := deskmanifest.Lookup(n); !ok {
			t.Fatalf("%s is not registered", n)
		}
	}
	cd, _ := deskmanifest.Lookup("app:candrec-candidate")
	if len(cd.OwnedKinds) != 1 || cd.OwnedKinds[0] != "choir.app.candrec-candidate.episode_note" {
		t.Errorf("candidate kinds %v", cd.OwnedKinds)
	}
	set, _ = Derive(append(events, Event{Seq: 3, Action: ActionDrop, Package: "candrec"}))
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	if _, ok := deskmanifest.Lookup("app:candrec-candidate"); ok {
		t.Fatal("a dropped candidate's desk is still registered")
	}
	if _, ok := deskmanifest.Lookup("app:candrec"); !ok {
		t.Fatal("the installed desk went with the candidate")
	}
}
