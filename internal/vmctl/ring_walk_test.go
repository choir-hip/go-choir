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
			o := healthyObs(ringProofComputer, ringR0, rwTo.Single)
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

// The time floor holds a ring whose members are healthy from the first tick.
func TestRingTimeFloorHolds(t *testing.T) {
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
	t3 := healthyObs(rwTest3, ringR1, rwTo.Single)
	for i := 1; i <= int(ringSoaks[ringR0].Floor/time.Minute)+1; i++ {
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
	s.step(opened.Add(2*time.Hour+time.Minute), obs)
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
	if st.Timing[0].Ring != ringR0 || st.Timing[0].ClosedAfterSeconds != 600 || st.Timing[0].LastReachedAfterSeconds != 120 {
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
