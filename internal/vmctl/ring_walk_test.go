package vmctl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Ring-ordered deploys and automatic code rollback
// (docs/design/ring-ordered-deploys-2026-11-03.md). Failure modes, written
// first:
//   - a ring is skipped silently (no history line), or R0 is skipped for want
//     of exposure so a bad release passes the proof ring unseen;
//   - an outer ring refreshes, or cold boots, onto the release before the
//     inner rings are clean (containment leak);
//   - the paper is refreshed before every other ring is clean;
//   - a guard cannot fail (no known-bad fixture flags it), or fires on a
//     known-good reading;
//   - a rollback loops (guards keep running after it, or a return refresh
//     triggers a second rollback);
//   - a rollback leaves a computer on the bad release at its next boot (the
//     pin is not "from" for every computer);
//   - a rollback enrolls computers that never ran the bad release;
//   - a new release while a walk runs leaves pins pointing at a release
//     nothing proved;
//   - a vmctl restart loses the walk.
//
// Zero time floors (owner, 2026-11-10; rings advance on health alone before
// launch). Failure modes, written first:
//   - a ring with zero running members (down, held, divergent) stalls the
//     walk, or passes the proof ring (R0) or the paper's ring (R5) unseen;
//   - a ring below its minimum count (R1 has 1 of 3, R3 has 1 of 2) waits
//     for members that do not exist;
//   - a member whose refresh failed, or that the sweep deferred busy, holds
//     its ring forever;
//   - a member that WAS refreshed is skipped: the ring closes before it was
//     seen healthy on the release (the whole point of the order);
//   - a member refreshed, then crashing or wedging after the next ring
//     opened, is no longer watched, so the release is not rolled back;
//   - a rollback after later rings opened leaves a closed ring's computer on
//     the bad release, or leaves a later ring unpinned;
//   - an expedite stops working or stops guarding with zero floors;
//   - a second release arriving mid-walk leaves pins on an unproven release;
//   - a vmctl restart mid-walk loses the walk or its progress;
//   - the full walk with every member healthy takes longer than a handful of
//     ticks (the estimate in the design doc), or no tick at all (a ring that
//     closes in the tick that opened it with a member still off the release);
//   - a closed ring stays under the zero-tolerance guards for the whole walk,
//     so one unrelated crash hours later (R5 holding on a deferred paper)
//     returns the fleet: the guard window must end 15 minutes after the close,
//     and must not apply to the open ring, and must survive a vmctl restart
//     (ClosedAt is persisted in the ring state, json closed_at).

var (
	rwT0   = time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC)
	rwFrom = ringReleasePair{Single: "fromsingle", SingleCommit: "commit-from", Layered: "fromlayered", LayeredCommit: "commit-from-l"}
	rwTo   = ringReleasePair{Single: "tosingle", SingleCommit: "commit-to", Layered: "tolayered", LayeredCommit: "commit-to-l"}
)

const (
	rwTest1 = "computer-aaaa0000000000000000000000000001"
	rwTest2 = "computer-aaaa0000000000000000000000000002"
	rwTest3 = "computer-aaaa0000000000000000000000000003"
	rwUser1 = "computer-bbbb0000000000000000000000000001"
)

func newWalkingState(t *testing.T) *ringWalkState {
	t.Helper()
	s := &ringWalkState{}
	s.observeRelease(rwFrom, rwT0.Add(-time.Hour))
	if s.Walk != nil || s.Baseline != rwFrom {
		t.Fatalf("first observation must record a baseline and no walk: %+v", s)
	}
	ev := s.observeRelease(rwTo, rwT0)
	if s.Walk == nil || s.Walk.Status != ringWalkWalking || s.Walk.From != rwFrom || s.Walk.To != rwTo {
		t.Fatalf("a new release must start a walk from the baseline: %+v", s.Walk)
	}
	if !hasEvent(ev, ringEventWalkStarted) {
		t.Fatalf("walk start not recorded: %+v", ev)
	}
	return s
}

func hasEvent(ev []ringEvent, kind string) bool {
	for _, e := range ev {
		if e.Event == kind {
			return true
		}
	}
	return false
}

func eventsOf(ev []ringEvent, kind string) []ringEvent {
	var out []ringEvent
	for _, e := range ev {
		if e.Event == kind {
			out = append(out, e)
		}
	}
	return out
}

func healthyObs(id string, ring ringID, release string) ringMemberObs {
	return ringMemberObs{ComputerID: id, UserID: "u-" + id, DesktopID: "primary", VMID: "vm-" + id, Ring: ring, Active: true, Release: release,
		Health: &ringHealth{Answered: true, Status: 200}}
}

func loadHealthFixture(t *testing.T, name string) *ringHealth {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ring_walk", name))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		StatusCode int             `json:"status_code"`
		Body       json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	h := parseRingHealth(f.StatusCode, f.Body)
	return &h
}

func loadCrashFixture(t *testing.T, name string) []CrashRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ring_walk", name))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Crashes []CrashRecord `json:"crashes"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Crashes
}

func TestRingMembershipByID(t *testing.T) {
	cases := []struct {
		id    string
		class WarmnessClass
		want  ringID
	}{
		{ringProofComputer, WarmnessClassTest, ringR0},
		{ringPaperComputer, WarmnessClassCriticalProtected, ringR5},
		{ringPaperComputer, WarmnessClassPrimary, ringR5},
		{"computer-03335285269bdba4f94377e56879f9e6", WarmnessClassPremiumAlwaysOn, ringR2},
		{rwTest1, WarmnessClassTest, ringR1},
		{rwUser1, WarmnessClassPremiumAlwaysOn, ringR3}, // class never makes an owner ring
		{rwUser1, WarmnessClassPublicPlatform, ringR4},
		{rwUser1, WarmnessClassCriticalProtected, ringR3}, // the paper is by id only
		{rwUser1, WarmnessClassPrimary, ringR3},
		{"", "", ringR3},
	}
	for _, c := range cases {
		if got := ringForComputer(c.id, c.class); got != c.want {
			t.Errorf("ringForComputer(%s, %s) = %s, want %s", c.id, c.class, got, c.want)
		}
	}
}

// Containment: before R0 is clean nothing outside R0 boots or refreshes onto
// the new release; the paper waits for every other ring.
func TestRingWalkContainmentPinAndGate(t *testing.T) {
	s := newWalkingState(t)
	ringOf := func(id string) ringID { return ringForComputer(id, WarmnessClassTest) }
	if single, _ := s.pinFor(ringProofComputer, ringR0); single != "" {
		t.Fatalf("R0 is open: d@e must launch the current release, pinned %q", single)
	}
	for _, id := range []string{rwTest1, ringPaperComputer, rwUser1} {
		single, layered := s.pinFor(id, ringOf(id))
		if single != rwFrom.Single || layered != rwFrom.Layered {
			t.Fatalf("%s outside the open ring must be pinned to from, got %q %q", id, single, layered)
		}
	}
	if r := s.gate(ringR1, "commit-to", false); r == "" || !strings.Contains(r, "R1") {
		t.Fatalf("a refresh of an R1 computer to the new release must be refused while R0 soaks, got %q", r)
	}
	if r := s.gate(ringR5, "commit-to", false); r == "" {
		t.Fatalf("the paper must never be refreshed before its ring opens")
	}
	if r := s.gate(ringR1, "commit-from", false); r != "" {
		t.Fatalf("a refresh onto the pinned release (a return) must pass: %q", r)
	}
	if r := s.gate(ringR0, "commit-to", false); r != "" {
		t.Fatalf("R0 refresh refused: %q", r)
	}
}

// Every guard flags its known-bad fixture and passes the known-good one.
func TestRingGuardsKnownBadFixtures(t *testing.T) {
	ok := loadHealthFixture(t, "health-ok.json")
	if !ok.Answered || ok.Status != 200 || ok.Wedged {
		t.Fatalf("known-good fixture parsed as bad: %+v", ok)
	}
	type tc struct {
		name  string
		guard string
		obs   func(i int) ringMemberObs
		ticks int
	}
	cases := []tc{
		{"appender wedge", ringGuardWedge, func(int) ringMemberObs {
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
			o.Health = loadHealthFixture(t, "health-appender-wedged.json")
			return o
		}, 2},
		{"readiness", ringGuardReadiness, func(int) ringMemberObs {
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
			o.Health = loadHealthFixture(t, "health-not-ready.json")
			return o
		}, 9},
		{"readiness, no answer", ringGuardReadiness, func(int) ringMemberObs {
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
			o.Health = &ringHealth{Answered: false, Detail: "connection refused"}
			return o
		}, 9},
		{"boot success", ringGuardBoot, func(int) ringMemberObs {
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
			o.Crashes = loadCrashFixture(t, "crashes-boot-loop.json")
			return o
		}, 1},
		{"crash restart", ringGuardCrash, func(i int) ringMemberObs {
			// Not yet answering /health (inside the readiness delay), so the
			// ring stays open until the crash arrives.
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
			o.Health = &ringHealth{Answered: false}
			if i > 0 {
				o.Crashes = loadCrashFixture(t, "crashes-unhealthy.json")
			}
			return o
		}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newWalkingState(t)
			var all []ringEvent
			for i := 0; i < c.ticks; i++ {
				ev, _ := s.step(rwT0.Add(time.Duration(i+1)*time.Minute), []ringMemberObs{c.obs(i)})
				all = append(all, ev...)
				if s.Walk.Status == ringWalkRolledBack {
					break
				}
			}
			if s.Walk.Status != ringWalkRolledBack {
				t.Fatalf("guard %s did not flag its known-bad fixture: %+v", c.guard, s.Walk.Rings)
			}
			breaches := eventsOf(all, ringEventRingClosed)
			if len(breaches) != 1 || breaches[0].Outcome != ringOutcomeBreached || breaches[0].Breach == nil || breaches[0].Breach.Guard != c.guard {
				t.Fatalf("breach not recorded as guard %s: %+v", c.guard, breaches)
			}
		})
	}
	// Known-good: a healthy d@e for the whole soak breaches nothing.
	s := newWalkingState(t)
	for i := 1; i <= 40; i++ {
		o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
		o.Health = ok
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{o})
	}
	if s.Walk.Status == ringWalkRolledBack {
		t.Fatalf("a known-good reading breached: %+v", s.Walk.Rings[0].Breach)
	}
}

// A readiness failure inside the initial delay after boot is not a breach
// (the paper is unreachable for minutes after every boot).
func TestRingReadinessInitialDelay(t *testing.T) {
	s := newWalkingState(t)
	for i := 1; i <= 4; i++ {
		o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
		o.Health = &ringHealth{Answered: false}
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{o})
	}
	if s.Walk.Status != ringWalkWalking {
		t.Fatalf("readiness inside the initial delay breached")
	}
}

// The walk: R0 soaks its floor and needs d@e seen healthy; then R1 opens and
// its members are enrolled once; empty rings are skipped with a line; the
// paper is enrolled only after R4; the walk completes and the baseline moves.
func TestRingWalkFullWalkPaperLast(t *testing.T) {
	s := newWalkingState(t)
	now := rwT0
	tick := func(d time.Duration, obs ...ringMemberObs) ([]ringEvent, []ringAction) {
		now = now.Add(d)
		return s.step(now, obs)
	}
	paperFrom := healthyObs(ringPaperComputer, ringR5, rwFrom.Single)
	t1 := healthyObs(rwTest1, ringR1, rwFrom.Single)
	de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
	de.Layered = true
	de.Release = rwFrom.Layered
	// Inside the R0 grace the CI deploy refreshes d@e; the walk does not.
	_, acts := tick(time.Minute, de, t1, paperFrom)
	if len(acts) != 0 {
		t.Fatalf("d@e enrolled inside the deploy's grace: %+v", acts)
	}
	// After the grace d@e is still not on the release; the walk enrolls it once.
	_, acts = tick(ringR0EnrollGrace, de, t1, paperFrom)
	if len(acts) != 1 || acts[0].Kind != ringActionEnroll || acts[0].ComputerID != ringProofComputer || acts[0].Target != rwTo.LayeredCommit {
		t.Fatalf("R0 open must enroll d@e once with its layered target: %+v", acts)
	}
	_, acts = tick(time.Minute, de, t1, paperFrom)
	if len(acts) != 0 {
		t.Fatalf("a member is enrolled once per ring: %+v", acts)
	}
	if s.Walk.Current != 0 {
		t.Fatalf("R0 advanced before d@e ran the release")
	}
	de.Release = rwTo.Layered
	var all []ringEvent
	for i := 0; i < 2; i++ {
		ev, a := tick(time.Minute, de, t1, paperFrom)
		all = append(all, ev...)
		acts = append(acts, a...)
	}
	if s.Walk.Current != 1 {
		t.Fatalf("R0 did not close clean after its floor: %+v", s.Walk.Rings)
	}
	closed := eventsOf(all, ringEventRingClosed)
	if len(closed) != 1 || closed[0].Ring != ringR0 || closed[0].Outcome != ringOutcomeClean || closed[0].Release == nil || closed[0].PredictionIDs == nil {
		t.Fatalf("R0 close line must carry the release, outcome and the prediction slot: %+v", closed)
	}
	if len(closed[0].Computers) != 1 || closed[0].Computers[0].ComputerID != ringProofComputer || closed[0].Computers[0].Healthy == 0 {
		t.Fatalf("R0 close line must carry the soak readings per computer: %+v", closed[0].Computers)
	}
	foundT1 := false
	for _, a := range acts {
		if a.ComputerID == ringPaperComputer {
			t.Fatalf("the paper was enrolled at R1: %+v", a)
		}
		if a.ComputerID == rwTest1 && a.Target == rwTo.SingleCommit {
			foundT1 = true
		}
	}
	if !foundT1 {
		t.Fatalf("R1 open must enroll its active member: %+v", acts)
	}
	// R1 is smaller than its exposure floor (3): it uses all its members.
	t1.Release = rwTo.Single
	all = nil
	var paperEnrolled bool
	for i := 0; i < 24*60 && s.Walk.Status == ringWalkWalking; i++ {
		obs := []ringMemberObs{de, t1, paperFrom}
		ev, a := tick(time.Minute, obs...)
		all = append(all, ev...)
		for _, x := range a {
			if x.ComputerID == ringPaperComputer {
				if s.Walk.Rings[len(s.Walk.Rings)-1].Ring != ringR5 {
					t.Fatalf("paper enrolled outside R5")
				}
				paperEnrolled = true
				paperFrom.Release = rwTo.Single
			}
		}
	}
	if !paperEnrolled {
		t.Fatalf("the paper was never enrolled")
	}
	if s.Walk.Status != ringWalkComplete || s.Baseline != rwTo {
		t.Fatalf("walk did not complete and move the baseline: %+v baseline %+v", s.Walk.Status, s.Baseline)
	}
	skipped := 0
	for _, e := range eventsOf(all, ringEventRingClosed) {
		if e.Outcome == ringOutcomeSkippedEmpty {
			skipped++
		}
	}
	if skipped != 3 { // R2, R3 and R4 have no members here
		t.Fatalf("empty rings must each write a skipped line, got %d: %+v", skipped, eventsOf(all, ringEventRingClosed))
	}
	if single, _ := s.pinFor(rwUser1, ringR3); single != "" {
		t.Fatalf("after a complete walk nothing is pinned")
	}
}

// R0 never skips for want of exposure: with d@e down the walk wakes it once
// after the deploy's grace and, while it stays down, holds.
func TestRingR0HoldsWithoutExposure(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, "")
	de.Active, de.Health = false, nil
	var wakes []ringAction
	for i := 1; i <= 120; i++ {
		_, acts := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
		for _, a := range acts {
			if a.Kind == ringActionWake {
				if i < int(ringR0EnrollGrace/time.Minute) {
					t.Fatalf("d@e woken inside the deploy's grace (minute %d)", i)
				}
				wakes = append(wakes, a)
			}
		}
	}
	// Staging 2026-11-03: a wake can fail; while d@e stays down the walk
	// wakes it again with a doubling gap, bounded.
	if len(wakes) < 2 || len(wakes) > ringWakeMaxAttempts || wakes[0].ComputerID != ringProofComputer {
		t.Fatalf("a down d@e must be woken again with backoff, bounded: %d wakes", len(wakes))
	}
	if s.Walk.Current != 0 || s.Walk.Status != ringWalkWalking {
		t.Fatalf("R0 advanced without d@e seen on the release: %+v", s.Walk.Rings)
	}
	if !strings.Contains(s.Walk.Rings[0].Waiting, "no active member") {
		t.Fatalf("the hold must say why: %q", s.Walk.Rings[0].Waiting)
	}
}

// A ring whose members are all down is skipped after its floor, with a line.
func TestRingOuterRingAllDownSkippedWithLine(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	t1 := healthyObs(rwTest1, ringR1, "")
	t1.Active, t1.Health = false, nil
	var all []ringEvent
	for i := 1; i <= 200; i++ {
		ev, _ := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, t1})
		all = append(all, ev...)
	}
	var r1 *ringEvent
	for i, e := range eventsOf(all, ringEventRingClosed) {
		if e.Ring == ringR1 {
			r1 = &eventsOf(all, ringEventRingClosed)[i]
		}
	}
	if r1 == nil || r1.Outcome != ringOutcomeSkippedNoExposure {
		t.Fatalf("R1 with every member down must close skipped_no_exposure with a line: %+v", r1)
	}
}

// Rollback: a breach at R0 pins every computer to from, enrolls a return only
// for computers that run the bad release, forgets forward entries, and stops
// the walk; nothing after it evaluates a guard (no loop).
func TestRingRollbackAtR0StopsWalk(t *testing.T) {
	s := newWalkingState(t)
	wedged := loadHealthFixture(t, "health-appender-wedged.json")
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Health = wedged
	t1 := healthyObs(rwTest1, ringR1, rwFrom.Single)
	var all []ringEvent
	var acts []ringAction
	for i := 1; i <= 3; i++ {
		ev, a := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, t1})
		all, acts = append(all, ev...), append(acts, a...)
	}
	if s.Walk.Status != ringWalkRolledBack || s.Walk.Current != 0 {
		t.Fatalf("breach at R0 must roll back and stop the walk at R0: %+v", s.Walk)
	}
	if !hasEvent(all, ringEventRolledBack) {
		t.Fatalf("rollback not in the history: %+v", all)
	}
	var returns []ringAction
	for _, a := range acts {
		if a.Kind == ringActionEnroll && a.Target == rwFrom.SingleCommit {
			returns = append(returns, a)
		}
		if a.ComputerID == rwTest1 && a.Kind == ringActionEnroll {
			t.Fatalf("R1 never ran the release and must not be enrolled: %+v", a)
		}
	}
	if len(returns) != 1 || returns[0].ComputerID != ringProofComputer {
		t.Fatalf("rollback must enroll d@e's return to from: %+v", acts)
	}
	for _, id := range []string{ringProofComputer, rwTest1, ringPaperComputer} {
		if single, _ := s.pinFor(id, ringForComputer(id, WarmnessClassTest)); single != rwFrom.Single {
			t.Fatalf("after rollback %s must be pinned to from, got %q", id, single)
		}
	}
	if r := s.gate(ringR0, "commit-to", false); r == "" {
		t.Fatalf("after rollback a refresh to the bad release must be refused")
	}
	if r := s.gate(ringR0, "commit-from", false); r != "" {
		t.Fatalf("the return refresh must pass the gate: %q", r)
	}
	for _, r := range s.Walk.Rings {
		if r.Ring == ringR1 {
			t.Fatalf("R1 opened after a breach at R0")
		}
	}
	// More bad readings and a crash after the rollback: no second rollback,
	// no new enrolls. The return completes once d@e runs from.
	de.Crashes = loadCrashFixture(t, "crashes-unhealthy.json")
	all, acts = nil, nil
	for i := 4; i <= 30; i++ {
		if i == 10 {
			de.Release, de.Health = rwFrom.Single, &ringHealth{Answered: true, Status: 200}
		}
		ev, a := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, t1})
		all, acts = append(all, ev...), append(acts, a...)
	}
	if len(acts) != 0 || hasEvent(all, ringEventRolledBack) || len(eventsOf(all, ringEventRingClosed)) != 0 {
		t.Fatalf("a rolled-back walk acted again: %+v %+v", acts, all)
	}
	if len(eventsOf(all, ringEventRollbackComplete)) != 1 {
		t.Fatalf("rollback complete must be recorded once: %+v", all)
	}
}

// A crash of a member before it ran the release is not attributed to it.
func TestRingCrashBeforeReleaseNotAttributed(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
	de.Crashes = []CrashRecord{{At: rwT0.Add(30 * time.Second).Format(time.RFC3339Nano), Kind: crashKindUnhealthy}}
	s.step(rwT0.Add(time.Minute), []ringMemberObs{de})
	de.Release = rwTo.Single
	for i := 2; i <= 5; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	if s.Walk.Status == ringWalkRolledBack {
		t.Fatalf("a crash on the old release rolled the new one back")
	}
}

// A new release while a walk runs supersedes it; the new walk starts from the
// last release that finished a walk, so pins never name an unproved release.
// A link that returns to the baseline ends the walk without a new one.
func TestRingWalkSupersedeAndReturn(t *testing.T) {
	s := newWalkingState(t)
	next := ringReleasePair{Single: "nextsingle", SingleCommit: "commit-next"}
	ev := s.observeRelease(next, rwT0.Add(time.Hour))
	if !hasEvent(ev, ringEventWalkSuperseded) || !hasEvent(ev, ringEventWalkStarted) {
		t.Fatalf("supersede must be recorded and a new walk started: %+v", ev)
	}
	if s.Walk.From != rwFrom || s.Walk.To != next {
		t.Fatalf("new walk must start from the baseline: %+v", s.Walk)
	}
	ev = s.observeRelease(rwFrom, rwT0.Add(2*time.Hour))
	if !hasEvent(ev, ringEventWalkSuperseded) || s.Walk != nil {
		t.Fatalf("a return to the baseline must end the walk without a new one: %+v %+v", ev, s.Walk)
	}
	if single, _ := s.pinFor(rwUser1, ringR3); single != "" {
		t.Fatalf("no walk, no pin")
	}
	// An unresolvable current release (legacy) changes nothing.
	before := *s
	s.observeRelease(ringReleasePair{}, rwT0.Add(3*time.Hour))
	if s.Baseline != before.Baseline || s.Walk != nil {
		t.Fatalf("a legacy observation moved the state")
	}
}

// The state survives a vmctl restart.
func TestRingWalkStatePersists(t *testing.T) {
	dir := t.TempDir()
	r := newRingWalkRunner()
	if err := r.load(filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "ring-walk-history.jsonl")); err != nil {
		t.Fatal(err)
	}
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	r2 := newRingWalkRunner()
	if err := r2.load(filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "ring-walk-history.jsonl")); err != nil {
		t.Fatal(err)
	}
	if r2.state.Walk == nil || r2.state.Walk.To != rwTo || r2.state.Baseline != rwFrom {
		t.Fatalf("walk lost across a restart: %+v", r2.state)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ring-walk-history.jsonl"))
	if err != nil || !strings.Contains(string(raw), `"event":"walk_started"`) || !strings.Contains(string(raw), ringHistorySchema) {
		t.Fatalf("history missing the walk start: %s %v", raw, err)
	}
}

// The shipped floors are zero before launch (owner, 2026-11-10).
func TestRingShippedTimeFloorsAreZero(t *testing.T) {
	for _, r := range ringOrder {
		if f := ringSoaks[r].Floor; f != 0 {
			t.Fatalf("ring %s time floor = %s; the floors are zero before launch", r, f)
		}
	}
}

// The time-floor mechanism is kept for launch (the floors are zero before
// it): with a floor set, the floor holds a ring whose members are healthy
// from the first tick.
func TestRingTimeFloorHolds(t *testing.T) {
	saved := ringSoaks[ringR0]
	ringSoaks[ringR0] = ringSoak{Floor: 10 * time.Minute, MinExposure: saved.MinExposure}
	defer func() { ringSoaks[ringR0] = saved }()
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	floor := int(ringSoaks[ringR0].Floor / time.Minute)
	for i := 1; i < floor; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
		if s.Walk.Current != 0 {
			t.Fatalf("R0 closed at minute %d, before its %d minute floor", i, floor)
		}
	}
	s.step(rwT0.Add(time.Duration(floor)*time.Minute), []ringMemberObs{de})
	if s.Walk.Current == 0 {
		t.Fatalf("R0 did not close at its floor")
	}
}

// d@e seen healthy on the release and then idle-hibernated is exposure: R0
// closes clean at its floor instead of holding.
func TestRingR0ExposedThenHibernatedIsClean(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	for i := 1; i <= 12; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	de.Active, de.Health, de.Release = false, nil, ""
	for i := 13; i <= 31; i++ {
		_, acts := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
		for _, a := range acts {
			if a.Kind == ringActionWake {
				t.Fatalf("an exposed d@e was woken again: %+v", a)
			}
		}
	}
	if s.Walk.Current == 0 || s.Walk.Rings[0].Outcome != ringOutcomeClean {
		t.Fatalf("exposed then hibernated d@e must close R0 clean: %+v", s.Walk.Rings[0])
	}
}

// Review 1 F2: R0 with no member observed holds; it never closes skipped.
func TestRingR0EmptyHolds(t *testing.T) {
	s := newWalkingState(t)
	t1 := healthyObs(rwTest1, ringR1, rwTo.Single)
	for i := 1; i <= 90; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{t1})
	}
	if s.Walk.Current != 0 || len(s.Walk.Rings) != 1 || !strings.Contains(s.Walk.Rings[0].Waiting, "no proof computer") {
		t.Fatalf("R0 with no member must hold: %+v", s.Walk.Rings[0])
	}
}

// Review 1 F3: a member whose entry was skipped (held or down at the sweep)
// and forgotten is enrolled again after the gap; one whose refresh to this
// target failed is not.
func TestRingReenrollAfterSkipNotAfterFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		s := newWalkingState(t)
		de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
		now := rwT0.Add(ringR0EnrollGrace)
		_, acts := s.step(now, []ringMemberObs{de})
		if len(acts) != 1 || acts[0].Kind != ringActionEnroll {
			t.Fatalf("first enroll: %+v", acts)
		}
		de.Pending = false
		if failed {
			de.FailedTarget = rwTo.SingleCommit
		}
		var again int
		for i := 1; i <= 15; i++ {
			_, acts = s.step(now.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
			for _, a := range acts {
				if a.Kind == ringActionEnroll {
					again++
					if time.Duration(i)*time.Minute < ringReenrollGap {
						t.Fatalf("re-enrolled before the gap at minute %d", i)
					}
				}
			}
		}
		if failed && again != 0 {
			t.Fatalf("a failed refresh was retried by the walk: %d", again)
		}
		if !failed && again != 1 {
			t.Fatalf("a skipped member must be enrolled again once per gap: %d", again)
		}
	}
}

// Review 1 F8: a crash between the last off-release tick and the first
// on-release tick (the seconds after the refresh boot) belongs to the release.
func TestRingCrashRightAfterRefreshAttributed(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
	s.step(rwT0.Add(time.Minute), []ringMemberObs{de})
	de.Release = rwTo.Single
	de.Crashes = []CrashRecord{{At: rwT0.Add(90 * time.Second).Format(time.RFC3339Nano), Kind: crashKindUnhealthy, Cause: "exited after boot"}}
	s.step(rwT0.Add(2*time.Minute), []ringMemberObs{de})
	if s.Walk.Status != ringWalkRolledBack || s.Walk.Rings[0].Breach.Guard != ringGuardCrash {
		t.Fatalf("a crash just after the refresh boot escaped the guard: %+v", s.Walk.Rings[0])
	}
}

// Review 1 F6: the same release ids with a commit read late do not supersede.
func TestRingLateCommitDoesNotSupersede(t *testing.T) {
	s := &ringWalkState{}
	s.observeRelease(rwFrom, rwT0.Add(-time.Hour))
	noCommit := rwTo
	noCommit.SingleCommit = ""
	s.observeRelease(noCommit, rwT0)
	id := s.Walk.ID
	ev := s.observeRelease(rwTo, rwT0.Add(time.Minute))
	if hasEvent(ev, ringEventWalkSuperseded) || s.Walk.ID != id || s.Walk.To.SingleCommit != rwTo.SingleCommit {
		t.Fatalf("a late commit restarted the walk: %+v %+v", ev, s.Walk)
	}
}

// Review 1 F1: status taken while ticks run is a copy (run with -race).
func TestRingStatusDoesNotAliasLiveWalk(t *testing.T) {
	r := newRingWalkRunner()
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			obs := []ringMemberObs{healthyObs(ringProofComputer, ringR0, rwTo.Single), healthyObs("computer-x"+strings.Repeat("0", i%5), ringR0, rwTo.Single)}
			r.stepWith(rwT0.Add(time.Duration(i)*time.Second), obs)
		}
	}()
	for i := 0; i < 200; i++ {
		st := r.status(10)
		if _, err := json.Marshal(st); err != nil {
			t.Fatal(err)
		}
	}
	<-done
}

// DeepSeek F1: an empty observation set holds every ring; it never cascades
// R0 to R5 and promotes the baseline.
func TestRingEmptyObservationHolds(t *testing.T) {
	s := newWalkingState(t)
	for i := 1; i <= 24*60; i += 30 {
		ev, acts := s.step(rwT0.Add(time.Duration(i)*time.Minute), nil)
		if len(ev) != 0 || len(acts) != 0 {
			t.Fatalf("an empty tick acted: %+v %+v", ev, acts)
		}
	}
	if s.Walk.Current != 0 || s.Walk.Status != ringWalkWalking || s.Baseline != rwFrom {
		t.Fatalf("empty ticks moved the walk: %+v baseline %+v", s.Walk, s.Baseline)
	}
	// d@e absent (a computer with an empty or unknown id observed instead).
	stray := healthyObs("", ringForComputer("", ""), rwTo.Single)
	for i := 1; i <= 120; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{stray})
	}
	if s.Walk.Current != 0 {
		t.Fatalf("R0 passed with d@e absent")
	}
}

// DeepSeek F4: a held member, or one never asked onto the release, cannot
// breach it with a failed boot of the old release.
func TestRingBootGuardScoped(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	held := healthyObs(rwTest1, ringR1, rwFrom.Single)
	held.Active, held.Held, held.Health = false, true, nil
	down := healthyObs(rwTest2, ringR1, "")
	down.Active, down.Health = false, nil
	// t3 is asked onto the release but not there yet, so R1 stays open.
	t3 := healthyObs(rwTest3, ringR1, rwFrom.Single)
	t3.Health = nil
	for i := 1; i <= 3; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, held, down, t3})
	}
	if s.Walk.Current != 1 {
		t.Fatalf("R0 did not close")
	}
	opened := s.Walk.Rings[1].OpenedAt
	held.Crashes = []CrashRecord{{At: opened.Add(time.Minute).Format(time.RFC3339Nano), Kind: crashKindBootFailed, Cause: "boot of from failed"}}
	down.Crashes = held.Crashes
	for i := 0; i < 5; i++ {
		s.step(opened.Add(time.Duration(2+i)*time.Minute), []ringMemberObs{de, held, down, t3})
	}
	if s.Walk.Status == ringWalkRolledBack {
		t.Fatalf("a boot failure of a member never asked onto the release rolled it back: %+v", s.Walk.Rings[1].Breach)
	}
}

// DeepSeek F5: a layered computer running the single image is not on the
// layered target.
func TestRingLayeredOnSingleIsNotOnTarget(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Layered = true
	for i := 1; i <= 60; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	if s.Walk.Current != 0 {
		t.Fatalf("R0 closed on the single image of a layered proof computer")
	}
}

// DeepSeek F3: a ring larger than its exposure floor closes once the floor
// count is met; a busy member that has not refreshed does not hold it.
func TestRingExposureFloorCount(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	var obs []ringMemberObs
	obs = append(obs, de)
	for i := 0; i < 3; i++ {
		obs = append(obs, healthyObs(fmt.Sprintf("computer-cccc00000000000000000000000000%02d", i), ringR1, rwTo.Single))
	}
	busy := healthyObs(rwTest1, ringR1, rwFrom.Single)
	busy.Pending = true
	obs = append(obs, busy)
	for i := 1; i <= 31; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), obs)
	}
	opened := s.Walk.Rings[1].OpenedAt
	s.step(opened.Add(time.Minute), obs)
	s.step(opened.Add(2*time.Minute), obs)
	if s.Walk.Current < 2 || s.Walk.Rings[1].Outcome != ringOutcomeClean {
		t.Fatalf("R1 with 3 of 4 exposed must close clean: %+v", s.Walk.Rings[1])
	}
}

// DeepSeek F8: a computer the walk has not observed is held on from.
func TestRingUnobservedComputerPinnedFrom(t *testing.T) {
	r := newRingWalkRunner()
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	if single, _ := r.pin("computer-new0000000000000000000000000000"); single != rwFrom.Single {
		t.Fatalf("an unobserved computer must be pinned to from, got %q", single)
	}
	if single, _ := r.pin(ringProofComputer); single != "" {
		t.Fatalf("d@e is known by id and R0 is open, pinned %q", single)
	}
}

// Nit: a torn state file loads the last good copy.
func TestRingWalkTornStateLoadsBackup(t *testing.T) {
	dir := t.TempDir()
	path, hist := filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "h.jsonl")
	r := newRingWalkRunner()
	_ = r.load(path, hist)
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	r.stepWith(rwT0.Add(time.Minute), []ringMemberObs{healthyObs(ringProofComputer, ringR0, rwTo.Single)})
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	r2 := newRingWalkRunner()
	if err := r2.load(path, hist); err != nil {
		t.Fatalf("torn file with a backup: %v", err)
	}
	if r2.state.Walk == nil || r2.state.Walk.To != rwTo {
		t.Fatalf("backup did not restore the walk: %+v", r2.state)
	}
}

// Round 2 (DeepSeek 2): a cold boot mid-soak re-arms the readiness delay.
func TestRingReadinessDelayPerBoot(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Epoch = 1
	for i := 1; i <= 10; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	de.Epoch, de.Health = 2, &ringHealth{Answered: false, Detail: "booting"}
	for i := 11; i <= 14; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	if s.Walk.Status == ringWalkRolledBack {
		t.Fatalf("a new boot's first minutes breached readiness")
	}
}

// Round 2 (Devin N2): a computer found on the rolled-back release after the
// breach is enrolled for a return; rollback completes only when none runs it.
func TestRingLateArrivalOnBadReleaseReturned(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Health = &ringHealth{Answered: true, Status: 200, Wedged: true}
	s.step(rwT0.Add(time.Minute), []ringMemberObs{de})
	s.step(rwT0.Add(2*time.Minute), []ringMemberObs{de})
	if s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("no rollback")
	}
	late := healthyObs(rwTest1, ringR1, rwTo.Single) // an in-flight refresh landed
	de.Release = rwFrom.Single
	_, acts := s.step(rwT0.Add(3*time.Minute), []ringMemberObs{de, late})
	if len(acts) != 1 || acts[0].ComputerID != rwTest1 || acts[0].Target != rwFrom.SingleCommit {
		t.Fatalf("a late arrival on the bad release must be returned: %+v", acts)
	}
	ev, _ := s.step(rwT0.Add(4*time.Minute), []ringMemberObs{de, late})
	if hasEvent(ev, ringEventRollbackComplete) {
		t.Fatalf("rollback completed while a computer runs the bad release")
	}
	late.Release = rwFrom.Single
	ev, _ = s.step(rwT0.Add(5*time.Minute), []ringMemberObs{de, late})
	if !hasEvent(ev, ringEventRollbackComplete) {
		t.Fatalf("rollback did not complete once none runs the bad release")
	}
}

// A wake retry is bounded and visible.
func TestRingWakeRetryBoundedAndVisible(t *testing.T) {
	s := newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, "")
	de.Active, de.Health = false, nil
	wakes := 0
	for i := 1; i <= 24*60; i++ {
		_, acts := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
		for _, a := range acts {
			if a.Kind == ringActionWake {
				wakes++
			}
		}
	}
	if wakes != ringWakeMaxAttempts {
		t.Fatalf("wakes = %d, want the bound %d", wakes, ringWakeMaxAttempts)
	}
	if !strings.Contains(s.Walk.Rings[0].Waiting, "no more automatic wakes") {
		t.Fatalf("the exhausted bound must show in status: %q", s.Walk.Rings[0].Waiting)
	}
}

// Expedite (owner, 2026-11-03): every ring opens at once, the paper
// included; every active computer off the release is enrolled in one tick;
// guards still run in every ring and a breach returns every computer that
// took the release.
func TestRingExpediteOpensEveryRingAndGuards(t *testing.T) {
	s := newWalkingState(t)
	if _, err := s.expedite(rwT0.Add(time.Minute), "key fix", "owner", ""); err != nil {
		t.Fatal(err)
	}
	if len(s.Walk.Rings) != len(ringOrder) || !s.Walk.Expedited {
		t.Fatalf("expedite must open every ring: %+v", s.Walk.Rings)
	}
	for _, id := range []string{ringPaperComputer, rwUser1, rwTest1} {
		if single, _ := s.pinFor(id, ringForComputer(id, WarmnessClassTest)); single != "" {
			t.Fatalf("expedited: %s still pinned", id)
		}
	}
	if r := s.gate(ringR5, "commit-to", false); r != "" {
		t.Fatalf("expedited: the paper's refresh is gated: %q", r)
	}
	de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
	paper := healthyObs(ringPaperComputer, ringR5, rwFrom.Single)
	other := healthyObs(rwUser1, ringR3, rwFrom.Single)
	_, acts := s.step(rwT0.Add(2*time.Minute), []ringMemberObs{de, paper, other})
	got := map[string]bool{}
	for _, a := range acts {
		if a.Kind == ringActionEnroll && a.Target == rwTo.SingleCommit {
			got[a.ComputerID] = true
		}
	}
	if !got[ringProofComputer] || !got[ringPaperComputer] || !got[rwUser1] {
		t.Fatalf("expedite must enroll every computer at once, d@e without the grace: %+v", acts)
	}
	// The paper takes it; R3's computer takes it and wedges: rollback returns both.
	paper.Release, other.Release = rwTo.Single, rwTo.Single
	other.Health = &ringHealth{Answered: true, Status: 200, Wedged: true}
	var all []ringEvent
	for i := 3; i <= 4; i++ {
		ev, a := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, paper, other})
		all, acts = append(all, ev...), a
	}
	if s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("a breach in an expedited ring must roll back: %+v", s.Walk.Status)
	}
	returns := map[string]bool{}
	for _, a := range acts {
		if a.Kind == ringActionEnroll && a.Target == rwFrom.SingleCommit {
			returns[a.ComputerID] = true
		}
	}
	if !returns[ringPaperComputer] || !returns[rwUser1] || returns[ringProofComputer] {
		t.Fatalf("rollback must return every computer that took the release, and only those: %+v", acts)
	}
}

func TestRingExpediteRefusedWithoutWalkOrAfterRollback(t *testing.T) {
	s := &ringWalkState{}
	s.observeRelease(rwFrom, rwT0)
	if _, err := s.expedite(rwT0, "x", "owner", ""); err == nil {
		t.Fatalf("expedite with no walk must be refused")
	}
	s = newWalkingState(t)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Health = &ringHealth{Answered: true, Status: 200, Wedged: true}
	s.step(rwT0.Add(time.Minute), []ringMemberObs{de})
	s.step(rwT0.Add(2*time.Minute), []ringMemberObs{de})
	if _, err := s.expedite(rwT0.Add(3*time.Minute), "x", "owner", ""); err == nil || !strings.Contains(err.Error(), "rolled") {
		t.Fatalf("a rolled-back release must never be expedited: %v", err)
	}
}

// An expedited walk completes after the watch once every member that is not
// still pending (busy) was seen healthy; a busy one does not hold it.
func TestRingExpediteCompletesAfterWatch(t *testing.T) {
	s := newWalkingState(t)
	s.expedite(rwT0, "key fix", "owner", "")
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	paper := healthyObs(ringPaperComputer, ringR5, rwTo.Single)
	busy := healthyObs(rwTest1, ringR1, rwFrom.Single)
	busy.Pending = true
	var all []ringEvent
	for i := 1; i <= 12; i++ {
		ev, _ := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, paper, busy})
		all = append(all, ev...)
		if i < 10 && s.Walk.Status != ringWalkWalking {
			t.Fatalf("expedited walk completed inside the watch")
		}
	}
	if s.Walk.Status != ringWalkComplete || s.Baseline != rwTo {
		t.Fatalf("expedited walk did not complete: %+v", s.Walk.Rings)
	}
	for _, e := range eventsOf(all, ringEventRingClosed) {
		if e.Timing == nil || e.Timing.ClosedAfterSeconds <= 0 {
			t.Fatalf("ring close without timing: %+v", e)
		}
	}
}

// Timing: deploy-to-ring-close and deploy-to-last-member-reached.
func TestRingTimingRecorded(t *testing.T) {
	r := newRingWalkRunner()
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	de := healthyObs(ringProofComputer, ringR0, rwFrom.Single)
	t1 := healthyObs(rwTest1, ringR1, rwFrom.Single)
	r.stepWith(rwT0.Add(time.Minute), []ringMemberObs{de, t1})
	de.Release = rwTo.Single
	for i := 2; i <= 12; i++ {
		r.stepWith(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, t1})
	}
	t1.Release = rwTo.Single
	r.stepWith(rwT0.Add(13*time.Minute), []ringMemberObs{de, t1})
	st := r.status(50)
	if len(st.Timing) < 2 {
		t.Fatalf("timing missing: %+v", st.Timing)
	}
	if st.Timing[0].Ring != ringR0 || st.Timing[0].ClosedAfterSeconds != 120 || st.Timing[0].LastReachedAfterSeconds != 120 {
		t.Fatalf("R0 timing = %+v", st.Timing[0])
	}
	if st.Timing[1].Ring != ringR1 || st.Timing[1].LastReachedAfterSeconds != 780 || st.Timing[1].Reached != 1 {
		t.Fatalf("R1 (arms) timing = %+v", st.Timing[1])
	}
}

// Round 3 (DeepSeek 1): an expedited walk never completes with d@e asleep
// or the paper still pending: R0 and R5 hold until each is seen healthy on
// the release, and the walk wakes d@e.
func TestRingExpediteHoldsProofAndPaper(t *testing.T) {
	s := newWalkingState(t)
	s.expedite(rwT0, "key fix", "owner", "")
	de := healthyObs(ringProofComputer, ringR0, "")
	de.Active, de.Health = false, nil
	paper := healthyObs(ringPaperComputer, ringR5, rwFrom.Single)
	paper.Pending = true
	other := healthyObs(rwUser1, ringR3, rwTo.Single)
	woke := false
	for i := 1; i <= 120; i++ {
		_, acts := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, paper, other})
		for _, a := range acts {
			if a.Kind == ringActionWake && a.ComputerID == ringProofComputer {
				woke = true
			}
		}
	}
	if s.Walk.Status != ringWalkWalking || s.Baseline != rwFrom {
		t.Fatalf("an expedited walk completed with d@e asleep and the paper pending: %+v", s.Walk.Status)
	}
	if !woke {
		t.Fatalf("an expedited walk must wake d@e")
	}
	for _, p := range s.Walk.Rings {
		if (p.Ring == ringR0 || p.Ring == ringR5) && !p.ClosedAt.IsZero() {
			t.Fatalf("%s closed unwatched: %+v", p.Ring, p)
		}
	}
	// d@e and the paper take it: the walk completes.
	de.Active, de.Release, de.Health = true, rwTo.Single, &ringHealth{Answered: true, Status: 200}
	paper.Pending, paper.Release = false, rwTo.Single
	for i := 121; i <= 123; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de, paper, other})
	}
	if s.Walk.Status != ringWalkComplete || s.Baseline != rwTo {
		t.Fatalf("expedited walk did not complete once d@e and the paper ran it: %+v", s.Walk.Rings)
	}
}

// Round 3 (DeepSeek 2): an expedite names the release; another walk's
// release is refused. A second expedite is a no-op.
func TestRingExpediteBoundToRelease(t *testing.T) {
	s := newWalkingState(t)
	if _, err := s.expedite(rwT0, "x", "ci", "commit-other"); err == nil {
		t.Fatalf("an expedite for another release must be refused")
	}
	if s.Walk.Expedited {
		t.Fatalf("a refused expedite marked the walk")
	}
	if _, err := s.expedite(rwT0, "x", "ci", rwTo.SingleCommit); err != nil {
		t.Fatal(err)
	}
	ev, err := s.expedite(rwT0.Add(time.Minute), "again", "ci", rwTo.SingleCommit)
	if err != nil || len(ev) != 0 || s.Walk.ExpediteReason != "x" {
		t.Fatalf("a second expedite must be a no-op: %v %+v", err, ev)
	}
}

// Round 3 (DeepSeek 3): a rollback of an expedited walk names the breached
// ring and closes every other open ring with a line.
func TestRingExpeditedRollbackClosesOtherRings(t *testing.T) {
	s := newWalkingState(t)
	s.expedite(rwT0, "x", "owner", "")
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	de.Health = &ringHealth{Answered: true, Status: 200, Wedged: true}
	var all []ringEvent
	for i := 1; i <= 2; i++ {
		ev, _ := s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
		all = append(all, ev...)
	}
	if s.Walk.Status != ringWalkRolledBack || s.Walk.open().Ring != ringR0 {
		t.Fatalf("rollback must name R0: %+v", s.Walk.open())
	}
	stopped := 0
	for _, e := range eventsOf(all, ringEventRingClosed) {
		if e.Outcome == ringOutcomeStoppedByRollback {
			stopped++
		}
	}
	if stopped != len(ringOrder)-1 {
		t.Fatalf("every other open ring must close with a line: %d", stopped)
	}
	for _, p := range s.Walk.Rings {
		if p.ClosedAt.IsZero() {
			t.Fatalf("ring %s left open after the rollback", p.Ring)
		}
	}
}

// Round 4 (Devin): the paper absent from the observation set holds R5
// under expedite; it never closes skipped_empty.
func TestRingExpediteHoldsR5WhenPaperUnobserved(t *testing.T) {
	s := newWalkingState(t)
	s.expedite(rwT0, "x", "owner", "")
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	for i := 1; i <= 60; i++ {
		s.step(rwT0.Add(time.Duration(i)*time.Minute), []ringMemberObs{de})
	}
	if s.Walk.Status != ringWalkWalking || s.Baseline != rwFrom {
		t.Fatalf("an expedited walk completed with the paper unobserved")
	}
	for _, p := range s.Walk.Rings {
		if p.Ring == ringR5 && !p.ClosedAt.IsZero() {
			t.Fatalf("R5 closed with the paper unobserved: %+v", p)
		}
	}
}

// ---- zero time floors (owner, 2026-11-10) ----

// rwFleet is a simulated fleet for the zero-floor walk: a tick steps the
// walk, then the retry sweep refreshes every enrolled member at once (the
// real sweep runs right after the tick); the next tick reads /health.
type rwFleet struct {
	t     *testing.T
	s     *ringWalkState
	now   time.Time
	obs   map[string]*ringMemberObs
	order []string
	all   []ringEvent
}

func newRwFleet(t *testing.T) *rwFleet {
	return &rwFleet{t: t, s: newWalkingState(t), now: rwT0, obs: map[string]*ringMemberObs{}}
}

func (f *rwFleet) add(id string, ring ringID, onTarget bool) *ringMemberObs {
	rel := rwFrom.Single
	if onTarget {
		rel = rwTo.Single
	}
	o := healthyObs(id, ring, rel)
	f.obs[id] = &o
	f.order = append(f.order, id)
	return &o
}

func (f *rwFleet) list() []ringMemberObs {
	var out []ringMemberObs
	for _, id := range f.order {
		out = append(out, *f.obs[id])
	}
	return out
}

// tick advances one minute and applies the sweep: enrolled members take the
// target release.
func (f *rwFleet) tick() {
	f.now = f.now.Add(time.Minute)
	ev, acts := f.s.step(f.now, f.list())
	f.all = append(f.all, ev...)
	for _, a := range acts {
		if a.Kind == ringActionEnroll && a.Target == rwTo.SingleCommit {
			if o := f.obs[a.ComputerID]; o != nil && o.Active {
				o.Release = rwTo.Single
				o.Pending = false
			}
		}
	}
}

func (f *rwFleet) closed(r ringID) *ringEvent {
	for _, e := range f.all {
		if e.Event == ringEventRingClosed && e.Ring == r {
			e := e
			return &e
		}
	}
	return nil
}

func (f *rwFleet) opened(r ringID) *ringEvent {
	for _, e := range f.all {
		if e.Event == ringEventRingOpened && e.Ring == r {
			e := e
			return &e
		}
	}
	return nil
}

const rwOwner = "computer-03335285269bdba4f94377e56879f9e6"
const rwOther1 = "computer-dddd0000000000000000000000000001"
const rwOther2 = "computer-dddd0000000000000000000000000002"
const rwPlat = "computer-eeee0000000000000000000000000001"

// The walk with every member healthy: d@e already refreshed by CI, every
// other member needs one sweep refresh. Each ring closes one tick after it
// opened (the refreshed member is read once on the target), so the paper's
// ring opens 5 ticks (5 minutes) after the walk start and the walk completes
// 1 tick later: a handful of minutes, not hours.
func TestRingZeroFloorFullWalkTiming(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	for _, id := range []string{rwTest1, rwTest2, rwTest3} {
		f.add(id, ringR1, false)
	}
	f.add(rwOwner, ringR2, false)
	f.add(rwOther1, ringR3, false)
	f.add(rwOther2, ringR3, false)
	f.add(rwPlat, ringR4, false)
	f.add(ringPaperComputer, ringR5, false)
	for i := 0; i < 12 && f.s.Walk.Status == ringWalkWalking; i++ {
		f.tick()
	}
	if f.s.Walk.Status != ringWalkComplete {
		t.Fatalf("walk did not complete: %+v", f.s.Walk.Status)
	}
	start := f.s.Walk.StartedAt
	if paper := f.opened(ringR5).At.Sub(start); paper != 5*time.Minute {
		t.Fatalf("paper ring opened %s after the walk start, want 5m", paper)
	}
	if done := f.s.Walk.EndedAt.Sub(start); done != 6*time.Minute {
		t.Fatalf("walk completed %s after start, want 6m", done)
	}
	prev := f.closed(ringR0)
	if prev == nil || prev.Outcome != ringOutcomeClean {
		t.Fatalf("R0 did not close clean: %+v", prev)
	}
	for _, r := range ringOrder[1:] {
		op, cl := f.opened(r), f.closed(r)
		if op == nil || cl == nil || op.At.Before(prev.At) {
			t.Fatalf("ring %s opened before the ring before it closed", r)
		}
		// A refreshed member must be seen healthy: no ring closes in the tick
		// that opened it.
		if !cl.At.After(op.At) {
			t.Fatalf("ring %s closed in the tick that opened it with a member still off the release", r)
		}
		prev = cl
	}
	if f.s.Baseline != rwTo {
		t.Fatalf("baseline did not move")
	}
}

// A ring with zero running members (down, held, divergent) is clean at once
// (R1 to R4); the paper is still refreshed and seen healthy.
func TestRingZeroFloorRingWithNoRunningMembers(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	down := f.add(rwTest1, ringR1, false)
	down.Active, down.Health = false, nil
	held := f.add(rwOwner, ringR2, false)
	held.Held, held.Health = true, nil
	div := f.add(rwOther1, ringR3, false)
	div.Divergent, div.Health = true, nil
	plat := f.add(rwPlat, ringR4, false)
	plat.Active, plat.Health = false, nil
	f.add(ringPaperComputer, ringR5, false)
	for i := 0; i < 8 && f.s.Walk.Status == ringWalkWalking; i++ {
		f.tick()
	}
	if f.s.Walk.Status != ringWalkComplete {
		t.Fatalf("walk stalled on rings with no running member: current %d", f.s.Walk.Current)
	}
	for _, r := range []ringID{ringR1, ringR2, ringR3, ringR4} {
		if cl := f.closed(r); cl == nil || cl.Outcome != ringOutcomeSkippedNoExposure {
			t.Fatalf("ring %s with no running member: %+v", r, cl)
		}
	}
	if cl := f.closed(ringR5); cl == nil || cl.Outcome != ringOutcomeClean {
		t.Fatalf("the paper must be refreshed and seen healthy: %+v", cl)
	}
}

// R0 with d@e not running holds (the proof ring is never passed unseen), and
// the paper deferred busy holds R5 (its refresh is unguarded once the ring
// closes).
func TestRingZeroFloorProofAndPaperStillHold(t *testing.T) {
	f := newRwFleet(t)
	de := f.add(ringProofComputer, ringR0, false)
	de.Active, de.Health = false, nil
	for i := 0; i < 5; i++ {
		f.tick()
	}
	if f.s.Walk.Current != 0 {
		t.Fatalf("R0 passed with d@e down")
	}
	g := newRwFleet(t)
	g.add(ringProofComputer, ringR0, true)
	paper := g.add(ringPaperComputer, ringR5, false)
	paper.Pending, paper.Deferred = true, true
	for i := 0; i < 10; i++ {
		g.tick()
	}
	if g.s.Walk.Current != 5 || g.closed(ringR5) != nil {
		t.Fatalf("R5 closed with the paper deferred and unseen: current %d", g.s.Walk.Current)
	}
}

// A ring below its minimum (R1 with 1 of 3, R3 with 1 of 2) does not wait for
// members that do not exist, but the one member is seen healthy.
func TestRingZeroFloorRingBelowMinimum(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	f.add(rwTest1, ringR1, false)
	f.add(rwOther1, ringR3, false)
	f.add(ringPaperComputer, ringR5, false)
	for i := 0; i < 8 && f.s.Walk.Status == ringWalkWalking; i++ {
		f.tick()
	}
	if f.s.Walk.Status != ringWalkComplete {
		t.Fatalf("walk stalled below a ring minimum: current %d", f.s.Walk.Current)
	}
	for _, r := range []ringID{ringR1, ringR3} {
		cl := f.closed(r)
		if cl == nil || cl.Outcome != ringOutcomeClean || cl.Soak.Exposed != 1 {
			t.Fatalf("ring %s must close clean with its one member seen healthy: %+v", r, cl)
		}
	}
}

// A member whose refresh failed, or that the sweep deferred busy, does not
// hold its ring; a member that was refreshed but is not answering yet does.
func TestRingZeroFloorUnrefreshableDoesNotHoldRefreshedDoes(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	failed := f.add(rwTest1, ringR1, false)
	failed.FailedTarget = rwTo.SingleCommit
	busy := f.add(rwTest2, ringR1, false)
	busy.Pending, busy.Deferred = true, true
	slow := f.add(rwTest3, ringR1, false)
	slow.Health = &ringHealth{Answered: false, Detail: "booting"}
	for i := 0; i < 4; i++ {
		f.tick()
	}
	if f.s.Walk.Current != 1 {
		t.Fatalf("R1 closed (or R0 did not) while a refreshed member was not seen healthy: current %d", f.s.Walk.Current)
	}
	// It answers: now R1 closes on it, without the failed or the busy one.
	f.obs[rwTest3].Health = &ringHealth{Answered: true, Status: 200}
	f.tick()
	if cl := f.closed(ringR1); cl == nil || cl.Outcome != ringOutcomeClean || cl.Soak.Exposed != 1 {
		t.Fatalf("R1 must close on the refreshed healthy member: %+v", cl)
	}
}

// A member of a closed ring that crashes after the next ring opened still
// breaches: every opened ring is watched until the walk ends, and the
// rollback returns the computers of all opened rings.
func TestRingZeroFloorCrashAfterNextRingOpenedRollsBack(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	f.add(rwOwner, ringR2, false)
	slowR3 := f.add(rwOther1, ringR3, false)
	slowR3.Health = &ringHealth{Answered: false} // R3 stays open
	f.tick()
	f.tick() // R1 closed, R2 open
	f.tick() // R2 closed, R3 open
	if f.s.Walk.Current != 3 || f.closed(ringR1) == nil {
		t.Fatalf("setup: R3 not open: current %d", f.s.Walk.Current)
	}
	t1.Crashes = []CrashRecord{{At: f.now.Add(30 * time.Second).Format(time.RFC3339Nano), Kind: "unhealthy_restart", Cause: "guest exited"}}
	f.tick()
	if f.s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("a crash in closed R1 while R3 is open did not roll back: %s", f.s.Walk.Status)
	}
	if b := f.s.Walk.Rings[1].Breach; b == nil || b.Guard != ringGuardCrash || b.ComputerID != rwTest1 {
		t.Fatalf("breach not recorded on R1: %+v", f.s.Walk.Rings[1].Breach)
	}
	if f.s.Walk.Rings[3].Outcome != ringOutcomeStoppedByRollback {
		t.Fatalf("the open ring must stop: %+v", f.s.Walk.Rings[3].Outcome)
	}
	// Every computer is pinned to from; the returns cover the closed rings.
	for _, ring := range ringOrder {
		if single, _ := f.s.pinFor("x", ring); single != rwFrom.Single {
			t.Fatalf("ring %s not pinned to from after rollback", ring)
		}
	}
	ret := map[string]bool{}
	for _, id := range f.s.Walk.Returning {
		ret[id] = true
	}
	if !ret[ringProofComputer] || !ret[rwTest1] || !ret[rwOwner] {
		t.Fatalf("returns must cover the computers of closed rings on the target: %v", f.s.Walk.Returning)
	}
}

// A wedge on d@e after R1 opened rolls everything back too (R0 is watched
// for the whole walk).
func TestRingZeroFloorWedgeInProofRingAfterR1Opened(t *testing.T) {
	f := newRwFleet(t)
	de := f.add(ringProofComputer, ringR0, true)
	slow := f.add(rwTest1, ringR1, false)
	slow.Health = &ringHealth{Answered: false}
	f.tick()
	if f.s.Walk.Current != 1 {
		t.Fatalf("setup: R1 not open")
	}
	de.Health = loadHealthFixture(t, "health-appender-wedged.json")
	f.tick()
	f.tick()
	if f.s.Walk.Status != ringWalkRolledBack || f.s.Walk.Rings[0].Breach == nil || f.s.Walk.Rings[0].Breach.Guard != ringGuardWedge {
		t.Fatalf("a wedge on d@e after R1 opened must roll back: %s", f.s.Walk.Status)
	}
}

// An expedite still opens every ring at once and still watches for its 10
// minutes with zero floors.
func TestRingZeroFloorExpediteStillWorks(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	f.add(rwTest1, ringR1, false)
	f.add(ringPaperComputer, ringR5, false)
	if _, err := f.s.expedite(f.now, "key fix", "owner", ""); err != nil {
		t.Fatal(err)
	}
	if len(f.s.Walk.Rings) != len(ringOrder) {
		t.Fatalf("expedite did not open every ring")
	}
	for i := 0; i < 9; i++ {
		f.tick()
	}
	if f.s.Walk.Status != ringWalkWalking {
		t.Fatalf("expedited walk completed inside its %s watch", ringExpediteWatch)
	}
	for i := 0; i < 3; i++ {
		f.tick()
	}
	if f.s.Walk.Status != ringWalkComplete {
		t.Fatalf("expedited walk did not complete after the watch: %s", f.s.Walk.Status)
	}
}

// A second release arriving mid-walk supersedes it and starts again from the
// last release that finished a walk.
func TestRingZeroFloorSecondReleaseMidWalk(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	slow := f.add(rwTest1, ringR1, false)
	slow.Health = &ringHealth{Answered: false}
	f.tick()
	third := ringReleasePair{Single: "thirdsingle", SingleCommit: "commit-third", Layered: "thirdlayered", LayeredCommit: "commit-third-l"}
	ev := f.s.observeRelease(third, f.now.Add(time.Minute))
	if !hasEvent(ev, ringEventWalkSuperseded) || !hasEvent(ev, ringEventWalkStarted) {
		t.Fatalf("second release must supersede and start a walk: %+v", ev)
	}
	if f.s.Walk.From != rwFrom || f.s.Walk.To != third || f.s.Walk.Current != 0 {
		t.Fatalf("new walk must run from the last good release: %+v", f.s.Walk)
	}
	if single, _ := f.s.pinFor(rwTest1, ringR1); single != rwFrom.Single {
		t.Fatalf("R1 must be pinned to the last good release again")
	}
}

// A vmctl restart mid-walk keeps the walk and its progress, and the walk
// finishes on the loaded state.
func TestRingZeroFloorVmctlRestartMidWalk(t *testing.T) {
	dir := t.TempDir()
	statePath, historyPath := filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "ring-walk-history.jsonl")
	r := newRingWalkRunner()
	if err := r.load(statePath, historyPath); err != nil {
		t.Fatal(err)
	}
	r.observe(rwFrom, rwT0.Add(-time.Hour))
	r.observe(rwTo, rwT0)
	de := healthyObs(ringProofComputer, ringR0, rwTo.Single)
	slow := healthyObs(rwTest1, ringR1, rwTo.Single)
	slow.Health = &ringHealth{Answered: false}
	r.stepWith(rwT0.Add(time.Minute), []ringMemberObs{de, slow})
	if r.state.Walk.Current != 1 {
		t.Fatalf("setup: R1 not open")
	}
	r2 := newRingWalkRunner()
	if err := r2.load(statePath, historyPath); err != nil {
		t.Fatal(err)
	}
	if w := r2.state.Walk; w == nil || w.Status != ringWalkWalking || w.Current != 1 || w.Rings[0].Outcome != ringOutcomeClean {
		t.Fatalf("restart lost the walk: %+v", r2.state.Walk)
	}
	slow.Health = &ringHealth{Answered: true, Status: 200}
	r2.stepWith(rwT0.Add(2*time.Minute), []ringMemberObs{de, slow})
	if r2.state.Walk == nil || r2.state.Walk.Status != ringWalkComplete {
		t.Fatalf("walk did not finish after the restart: %+v", r2.state.Walk)
	}
}

// The closed-ring guard window: a breach in a closed ring 14 minutes after it
// closed rolls back; 16 minutes after, it does not; the open ring is not
// bounded; a restart keeps the close time.
func closedRingFleet(t *testing.T) (*rwFleet, *ringMemberObs) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	slow := f.add(rwOther1, ringR3, false)
	slow.Health = &ringHealth{Answered: false} // R3 stays open
	f.tick()
	f.tick() // R1 closed, R2 empty closed, R3 open
	if f.s.Walk.Current != 3 {
		t.Fatalf("setup: R3 not open: %d", f.s.Walk.Current)
	}
	return f, t1
}

func TestRingClosedGuardWindow(t *testing.T) {
	for _, c := range []struct {
		after    time.Duration
		rollback bool
	}{{14 * time.Minute, true}, {16 * time.Minute, false}} {
		f, t1 := closedRingFleet(t)
		closedAt := f.s.Walk.Rings[1].ClosedAt
		f.now = closedAt.Add(c.after - time.Minute)
		t1.Crashes = []CrashRecord{{At: closedAt.Add(c.after - 30*time.Second).Format(time.RFC3339Nano), Kind: "unhealthy_restart", Cause: "x"}}
		f.tick()
		if got := f.s.Walk.Status == ringWalkRolledBack; got != c.rollback {
			t.Fatalf("breach %s after the close: rolled back = %v, want %v", c.after, got, c.rollback)
		}
	}
}

func TestRingClosedGuardWindowOpenRingUnbounded(t *testing.T) {
	f := newRwFleet(t)
	de := f.add(ringProofComputer, ringR0, true)
	slow := f.add(rwTest1, ringR1, false)
	slow.Health = &ringHealth{Answered: false}
	f.tick() // R1 open, R0 closed
	f.now = f.now.Add(3 * time.Hour)
	slow.Crashes = []CrashRecord{{At: f.now.Format(time.RFC3339Nano), Kind: "unhealthy_restart", Cause: "x"}}
	f.tick()
	_ = de
	if f.s.Walk.Status != ringWalkRolledBack || f.s.Walk.Rings[1].Breach == nil {
		t.Fatalf("the open ring must be guarded however long it waits: %s", f.s.Walk.Status)
	}
}

func TestRingClosedGuardWindowSurvivesRestart(t *testing.T) {
	f, _ := closedRingFleet(t)
	raw, err := json.Marshal(f.s)
	if err != nil {
		t.Fatal(err)
	}
	var back ringWalkState
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if got, want := back.Walk.Rings[1].ClosedAt, f.s.Walk.Rings[1].ClosedAt; got.IsZero() || !got.Equal(want) {
		t.Fatalf("closed_at not persisted: %v vs %v", got, want)
	}
	if !ringClosedGuarded(back.Walk.Rings[1], back.Walk.Rings[1].ClosedAt.Add(14*time.Minute)) || ringClosedGuarded(back.Walk.Rings[1], back.Walk.Rings[1].ClosedAt.Add(16*time.Minute)) {
		t.Fatalf("window wrong after a reload")
	}
}

// ---- review round 1 (Gemini), each scenario written before its fix ----
//
// Failure modes:
//   - F1: a refreshed member that is rebooting (not Active) on the next tick
//     leaves its ring with required=0, exposed=0, and with a zero floor the
//     ring closes skipped_no_exposure before any refreshed member was seen
//     healthy;
//   - F2: R5 closes skipped, and the walk completes with the paper never
//     refreshed, when the paper is inactive, held, divergent, or its refresh
//     failed;
//   - F3: a member that crashes on the target while inactive has its crash
//     suppressed forever (LastOffTarget moves to now on the inactive tick);
//   - F4: a probe that never ran (no slot, tick deadline) counts as a
//     readiness strike and rolls the fleet back;
//   - F5: a breach in a closed ring writes a second ring_closed line for it,
//     overwrites its ClosedAt, and leaves later closed rings reading clean.

func TestRingReviewF1RefreshedMemberRebootingHoldsRing(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	f.tick() // R0 closes, R1 opens, t1 enrolled and refreshed by the sweep
	if f.s.Walk.Current != 1 || f.obs[rwTest1].Release != rwTo.Single {
		t.Fatalf("setup: current %d release %s", f.s.Walk.Current, f.obs[rwTest1].Release)
	}
	t1.Active, t1.Health = false, nil // rebooting
	f.tick()
	if f.s.Walk.Current != 1 || f.closed(ringR1) != nil {
		t.Fatalf("R1 closed %v with its refreshed member rebooting and never seen healthy", f.closed(ringR1))
	}
	// It comes back healthy: now the ring closes clean.
	t1.Active, t1.Health = true, &ringHealth{Answered: true, Status: 200}
	f.tick()
	if cl := f.closed(ringR1); cl == nil || cl.Outcome != ringOutcomeClean {
		t.Fatalf("R1 must close clean once the member is seen healthy: %+v", cl)
	}
}

// The hold is bounded: a refreshed member that stays inactive past
// ringInactiveHold with no boot failure stops holding its ring (treated as
// down); a boot failure breaches at once.
func TestRingReviewF1InactiveHoldIsBounded(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	f.tick()
	t1.Active, t1.Health = false, nil
	for i := 0; i < int(ringInactiveHold/time.Minute)-1; i++ {
		f.tick()
	}
	if f.s.Walk.Current != 1 {
		t.Fatalf("hold ended early: current %d", f.s.Walk.Current)
	}
	f.tick()
	f.tick()
	if cl := f.closed(ringR1); cl == nil || cl.Outcome != ringOutcomeSkippedNoExposure {
		t.Fatalf("the hold must end after %s: %+v", ringInactiveHold, cl)
	}
	g := newRwFleet(t)
	g.add(ringProofComputer, ringR0, true)
	u := g.add(rwTest1, ringR1, false)
	g.tick()
	u.Active, u.Health = false, nil
	u.Crashes = []CrashRecord{{At: g.now.Add(10 * time.Second).Format(time.RFC3339Nano), Kind: crashKindBootFailed, Cause: "boot of to failed"}}
	g.tick()
	if g.s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("a boot failure of a refreshed inactive member must breach: %s", g.s.Walk.Status)
	}
}

func TestRingReviewF2PaperNeverSkippedForTransientState(t *testing.T) {
	cases := map[string]func(o *ringMemberObs){
		"inactive":  func(o *ringMemberObs) { o.Active, o.Health = false, nil },
		"held":      func(o *ringMemberObs) { o.Held, o.Health = true, nil },
		"divergent": func(o *ringMemberObs) { o.Divergent, o.Health = true, nil },
		"failed":    func(o *ringMemberObs) { o.FailedTarget = rwTo.SingleCommit },
		"deferred":  func(o *ringMemberObs) { o.Pending, o.Deferred = true, true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRwFleet(t)
			f.add(ringProofComputer, ringR0, true)
			paper := f.add(ringPaperComputer, ringR5, false)
			mutate(paper)
			for i := 0; i < 10; i++ {
				f.tick()
			}
			if f.s.Walk.Status != ringWalkWalking || f.closed(ringR5) != nil {
				t.Fatalf("R5 closed (%+v) or the walk ended (%s) with the paper %s and never refreshed", f.closed(ringR5), f.s.Walk.Status, name)
			}
		})
	}
	// A paper that is legitimately absent from the fleet closes R5 as empty.
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	for i := 0; i < 8; i++ {
		f.tick()
	}
	if cl := f.closed(ringR5); cl == nil || cl.Outcome != ringOutcomeSkippedEmpty {
		t.Fatalf("an absent paper closes R5 as empty: %+v", cl)
	}
}

func TestRingReviewF3CrashWhileInactiveIsNotForgotten(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	t1.Health = &ringHealth{Answered: false}
	f.tick() // R1 opens, t1 refreshed
	f.tick() // t1 seen on the target
	t1.Active, t1.Health = false, nil
	t1.Crashes = []CrashRecord{{At: f.now.Add(30 * time.Second).Format(time.RFC3339Nano), Kind: "unhealthy_restart", Cause: "guest exited"}}
	f.tick() // inactive tick
	t1.Active, t1.Health = true, &ringHealth{Answered: false}
	f.tick() // back on the target, the crash still in its record
	if f.s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("a crash on the target while inactive was suppressed: %s", f.s.Walk.Status)
	}
}

func TestRingReviewF4UnprobedIsNeitherHealthyNorStrike(t *testing.T) {
	f := newRwFleet(t)
	f.add(ringProofComputer, ringR0, true)
	t1 := f.add(rwTest1, ringR1, false)
	t1.Health = &ringHealth{Answered: false}
	f.tick()
	for i := 0; i < 15; i++ {
		t1.Health = &ringHealth{Answered: false, Unprobed: true, Detail: "no probe slot before the tick deadline"}
		f.tick()
	}
	if f.s.Walk.Status != ringWalkWalking {
		t.Fatalf("unprobed samples rolled the walk back: %+v", f.s.Walk.Rings[1].Breach)
	}
	if m := f.s.Walk.Rings[1].Members[rwTest1]; m.Healthy != 0 || m.ReadyStrikes != 0 || !m.ExposedAt.IsZero() {
		t.Fatalf("unprobed must be neither healthy nor a strike: %+v", m)
	}
}

func TestRingReviewF5ClosedRingBreachRecordsOnce(t *testing.T) {
	f, t1 := closedRingFleet(t)
	closedAt := f.s.Walk.Rings[1].ClosedAt
	t1.Crashes = []CrashRecord{{At: f.now.Add(30 * time.Second).Format(time.RFC3339Nano), Kind: "unhealthy_restart", Cause: "x"}}
	f.tick()
	if f.s.Walk.Status != ringWalkRolledBack {
		t.Fatalf("setup: no rollback")
	}
	n := 0
	for _, e := range f.all {
		if e.Event == ringEventRingClosed && e.Ring == ringR1 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("R1 wrote %d ring_closed lines", n)
	}
	if !f.s.Walk.Rings[1].ClosedAt.Equal(closedAt) || f.s.Walk.Rings[1].Outcome != ringOutcomeBreached || f.s.Walk.Rings[1].Breach == nil {
		t.Fatalf("R1 after the breach: %+v", f.s.Walk.Rings[1])
	}
	if o := f.s.Walk.Rings[2].Outcome; o != ringOutcomeStoppedByRollback {
		t.Fatalf("a ring closed after the breached one must read stopped_by_rollback, got %s", o)
	}
}
