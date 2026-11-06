package pkgrelease

import (
	"strings"
	"testing"
	"time"
)

// Ways the update decision could fail (F1), written before the code:
//
//  1. an absent, unreadable or partial policy reads as automatic;
//  2. a policy with an unknown mode or field is stored and then misread;
//  3. another track's release (another publisher) is offered as an update;
//  4. a withdrawn, suspended or yanked release is offered as an update;
//  5. an automatic installer takes a release listed less than 24h ago;
//  6. an automatic installer takes an update that adds a grant or an SDK use;
//  7. an install recorded before uses were kept is updated automatically
//     although its uses cannot be compared;
//  8. a forced_release is written by anyone but the platform, or on a
//     listing that is not yanked, or names the release itself;
//  9. a listing without forced_release hashes differently than before.

func TestUpdatePolicyReadsManualUnlessSaidOtherwise(t *testing.T) {
	var zero UpdatePolicy
	if zero.For("calendar") != UpdateManual {
		t.Fatalf("zero policy is %q", zero.For("calendar"))
	}
	p, err := ParseUpdatePolicy([]byte(`{"default":"manual","apps":{"calendar":"automatic"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.For("calendar") != UpdateAutomatic || p.For("pulse") != UpdateManual {
		t.Fatalf("per-app override: %+v", p)
	}
	for _, bad := range []string{
		`{"default":"always"}`,
		`{"default":"manual","apps":{"Calendar!":"automatic"}}`,
		`{"default":"manual","apps":{"calendar":"later"}}`,
		`{"default":"manual","extra":1}`,
		`{}`,
		`{"default":"manual"}{"default":"manual"}`,
	} {
		if _, err := ParseUpdatePolicy([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestNewerReleasesAreTheSameTrackAndListedOnly(t *testing.T) {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	installed := TrackRelease{ReleaseID: sha("1"), Package: "calendar", Track: "owner-a", Seq: 1, State: StateListed}
	track := []TrackRelease{
		installed,
		{ReleaseID: sha("2"), Package: "calendar", Track: "owner-a", Seq: 2, State: StateListed},
		{ReleaseID: sha("3"), Package: "calendar", Track: "owner-a", Seq: 3, State: StateWithdrawn},
		{ReleaseID: sha("4"), Package: "calendar", Track: "owner-a", Seq: 4, State: StateYanked},
		{ReleaseID: sha("5"), Package: "calendar", Track: "owner-b", Seq: 9, State: StateListed},
		{ReleaseID: sha("6"), Package: "pulse", Track: "owner-a", Seq: 9, State: StateListed},
		{ReleaseID: sha("7"), Package: "calendar", Track: "owner-a", Seq: 5, State: StateSuspended},
	}
	newer := NewerReleases(installed, track)
	if len(newer) != 1 || newer[0].ReleaseID != sha("2") {
		t.Fatalf("newer = %+v", newer)
	}
}

func TestAutomaticWaitsForTheSoakAndNeverWidens(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	fresh := TrackRelease{State: StateListed, ListedAt: now.Add(-23 * time.Hour)}
	soaked := TrackRelease{State: StateListed, ListedAt: now.Add(-UpdateSoak)}
	cur := UpdateSurface{Uses: []string{"appTitle"}, UsesKnown: true, Grants: []string{"files.read"}, BehaviourSurface: "a"}
	same := CompareSurface(cur, UpdateSurface{Uses: []string{"appTitle"}, UsesKnown: true, Grants: []string{"files.read"}, BehaviourSurface: "b"})
	if same.Widening() || !same.SurfaceChanged {
		t.Fatalf("narrow change: %+v", same)
	}
	if ok, why := AutomaticCandidate(UpdateAutomatic, fresh, same, now); ok || !strings.Contains(why, "soak") {
		t.Fatalf("fresh release taken: %v %s", ok, why)
	}
	if ok, _ := AutomaticCandidate(UpdateAutomatic, soaked, same, now); !ok {
		t.Fatal("soaked narrow update refused")
	}
	if ok, _ := AutomaticCandidate(UpdateManual, soaked, same, now); ok {
		t.Fatal("manual installer updated automatically")
	}
	moreUses := CompareSurface(cur, UpdateSurface{Uses: []string{"appTitle", "openFile"}, UsesKnown: true, Grants: []string{"files.read"}})
	moreGrants := CompareSurface(cur, UpdateSurface{Uses: []string{"appTitle"}, UsesKnown: true, Grants: []string{"files.read", "net.fetch"}})
	unknown := CompareSurface(UpdateSurface{Grants: []string{"files.read"}}, UpdateSurface{Uses: []string{}, UsesKnown: true, Grants: []string{"files.read"}})
	for name, c := range map[string]UpdateChange{"uses": moreUses, "grants": moreGrants, "unknown": unknown} {
		if !c.Widening() {
			t.Errorf("%s: not widening: %+v", name, c)
		}
		if ok, why := AutomaticCandidate(UpdateAutomatic, soaked, c, now); ok || !strings.Contains(why, "approval") {
			t.Errorf("%s: widening update taken automatically: %s", name, why)
		}
	}
	if moreUses.AddedUses[0] != "openFile" || moreGrants.AddedGrants[0] != "net.fetch" {
		t.Fatalf("added: %+v %+v", moreUses, moreGrants)
	}
}

func TestForcedReleaseIsThePlatformsOnAYankedListing(t *testing.T) {
	base := Listing{
		RecordKind: RecordKindPackageRelease, RecordID: strings.Repeat("a", 64), Package: "calendar", Track: "owner-a",
		Seq: 1, Scopes: []string{ScopeOwner, ScopePublic}, State: StateListed, StateActor: "owner_act:x", Version: 1,
	}
	plain, err := base.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "forced_release") {
		t.Fatal("a listing without forced_release names it")
	}
	cases := map[string]func(l *Listing){
		"listed state": func(l *Listing) { l.ForcedRelease = strings.Repeat("b", 64) },
		"owner actor":  func(l *Listing) { l.State = StateYanked; l.ForcedRelease = strings.Repeat("b", 64) },
		"itself":       func(l *Listing) { l.State, l.StateActor, l.ForcedRelease = StateYanked, "platform:op", l.RecordID },
		"not a sha256": func(l *Listing) { l.State, l.StateActor, l.ForcedRelease = StateYanked, "platform:op", "release-2" },
	}
	for name, mutate := range cases {
		l := base
		mutate(&l)
		if err := l.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := base
	ok.State, ok.StateActor, ok.ForcedRelease = StateYanked, "platform:op", strings.Repeat("b", 64)
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}
