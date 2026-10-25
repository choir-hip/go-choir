package agentcore

// A round stopped before it froze keeps its work (problem 1 of
// docs/evidence/selfdev-trace-reviews-2026-10-25.md: reader-3's repair round
// was cancelled by the 60 minute activation budget after 159 calls and the
// next round started from the base; owner: "if we get part of the work done,
// but not everything, we can iteratively improve on it rather than having to
// start over"). Just before the stopped round's capsule is destroyed, its
// source change is taken as an inert, digest-bound patch and kept under
// <updater root>/carried/unfinished; the next round on the trajectory is
// offered it, labeled unfinished and unverified, by the carry
// (carried_patch.go). Ways it can fail, written before the code; each has a
// test below:
//
//  A1. With engineering.carry_unfinished off, a snapshot is taken, or a kept
//      one is still offered (off must be the whole rollback).
//  A2. A round that reported (Complete, or a failed Report that minted its
//      candidate) is snapshotted as unfinished.
//  A3. A failed or slow snapshot fails or blocks the revoke: the capsule
//      survives, or the assignment is not cancelled.
//  A4. A verification assignment, or a capsule already gone, is snapshotted.
//  A5. A retried revoke takes a second snapshot or writes a second record.
//  A6. The kept patch becomes a candidate: it is listed with the
//      trajectory's candidates, or reaches approval.
//  A7. The offer derives it from a candidate tree, or the notice says it was
//      frozen, built or tested, or does not say the engineer may ignore it.
//  A8. Kept bytes that no longer match their digest, or that name a path
//      outside the source tree, are offered or mounted.
//  A9. An assignment with both a candidate and a kept patch is carried from
//      the kept patch (the candidate is the better record).
//  A10. A kept patch whose round's operation applied or is pending is
//      carried (the carry's own rule must hold for it too).
//  A11. A kept record that names another trajectory, attempt or base than
//      its assignment is believed.
//  A12. Anything is written outside <updater root>/carried.
//  A13. The snapshot is taken after the capsule is destroyed (nothing left
//      to read) instead of before.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// snapshotCapsule is a capsule runtime whose bound capsules are alive until
// destroyed, and which can take the unfinished snapshot.
type snapshotCapsule struct {
	absentAssignmentCapsule
	mu             sync.Mutex
	patch          capsule.CarriedPatch
	err            error
	destroyed      map[string]bool
	snapshots      []string
	afterDestroyed []string
}

func newSnapshotCapsule(patch capsule.CarriedPatch) *snapshotCapsule {
	return &snapshotCapsule{patch: patch, destroyed: map[string]bool{}}
}

func (c *snapshotCapsule) AssignmentHandle(string, string) (string, error) { return "handle", nil }
func (c *snapshotCapsule) HasCapsule(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.destroyed[id]
}
func (c *snapshotCapsule) ForceDestroy(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.destroyed[id] = true
	return nil
}
func (c *snapshotCapsule) SnapshotUnfinishedSourcePatch(_ context.Context, id string) (capsule.CarriedPatch, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.destroyed[id] {
		c.afterDestroyed = append(c.afterDestroyed, id)
	}
	c.snapshots = append(c.snapshots, id)
	return c.patch, c.err
}

type unfinishedFixture struct {
	*admissionFixture
	caps *snapshotCapsule
	root string
}

func newUnfinishedRoundFixture(t *testing.T, patch capsule.CarriedPatch, on bool) unfinishedFixture {
	t.Helper()
	f := newAdmissionFixture(t, 2)
	caps := newSnapshotCapsule(patch)
	f.rt.assignmentRuntime = caps
	root := filepath.Join(t.TempDir(), "updater")
	f.rt.selfdevUpdaterRoot = root
	setCarryUnfinished(t, f, on)
	return unfinishedFixture{admissionFixture: f, caps: caps, root: root}
}

func setCarryUnfinished(t *testing.T, f *admissionFixture, on bool) {
	t.Helper()
	if err := f.s.PutComputerSetting(context.Background(), store.ComputerSetting{
		OwnerID: f.seed.OwnerID, ComputerID: f.computer, Key: store.SettingEngineeringCarryUnfinished, Value: on,
	}); err != nil {
		t.Fatal(err)
	}
}

// stopByBudget ends the bound round the way the activation budget does:
// HandleActivationBudgetDeadline terminalizes the run, which cancels its
// bound assignment through cancelBoundEngineeringRun.
func (f unfinishedFixture) stopByBudget(t *testing.T, bound types.EngineeringAssignment) {
	t.Helper()
	rec := types.RunRecord{
		RunID: bound.BoundRunID, OwnerID: bound.Binding.OwnerID, ComputerID: bound.Binding.ComputerID, AgentID: bound.Binding.AssignedAgentID,
		Metadata: map[string]any{"assignment_id": bound.AssignmentID, "assignment_attempt": int(bound.Binding.Attempt)},
	}
	if _, err := f.rt.cancelBoundEngineeringRun(context.Background(), rec, "activation budget exceeded: progress deadline reached"); err != nil {
		t.Fatalf("budget cancel: %v", err)
	}
	if got := f.get(0); got.Disposition != types.EngineeringAssignmentCancelled || got.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("the budget cancel left the round %s / %s", got.Disposition, got.CapsuleDisposition)
	}
}

func (f unfinishedFixture) offerTo(t *testing.T, retry types.EngineeringAssignment, deriver *fakeCarriedDeriver) carriedPatchOffer {
	t.Helper()
	return offerCarriedPatch(context.Background(), carriedPatchDeps{History: storeCarriedHistory{rt: f.rt}, Deriver: deriver, Root: f.root},
		carriedPatchInput{OwnerID: retry.Binding.OwnerID, ComputerID: retry.Binding.ComputerID, TrajectoryID: retry.Binding.TrajectoryID,
			AssignmentID: retry.AssignmentID, Attempt: retry.Binding.Attempt, RetryBaseRef: retry.Binding.SourceArtifactRef})
}

// The whole path: a round stopped by its budget keeps its work before its
// capsule is destroyed (A13), and the next round is offered it as
// unfinished, unverified work (A7) without it becoming a candidate (A6).
func TestBudgetStoppedRoundKeepsItsWorkForTheNextRound(t *testing.T) {
	patch := testCarriedPatch("packages/reader/reader.go", "frontend/src/ReaderRoute.svelte")
	f := newUnfinishedRoundFixture(t, patch, true)
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)

	if len(f.caps.snapshots) != 1 || f.caps.snapshots[0] != first.Binding.CapsuleID {
		t.Fatalf("snapshots = %v, want one of %s", f.caps.snapshots, first.Binding.CapsuleID)
	}
	if len(f.caps.afterDestroyed) != 0 || !f.caps.destroyed[first.Binding.CapsuleID] {
		t.Fatalf("snapshot after destroy %v, destroyed %v: the snapshot must come first and the capsule must still go", f.caps.afterDestroyed, f.caps.destroyed)
	}
	rounds := readUnfinishedRounds(f.root, first.Binding.TrajectoryID)
	if len(rounds) != 1 || rounds[0].AssignmentID != first.AssignmentID || rounds[0].PatchSHA256 != patch.SHA256 ||
		!strings.Contains(rounds[0].Reason, "activation budget") {
		t.Fatalf("kept rounds = %+v", rounds)
	}

	// A6: not a candidate.
	candidates, err := f.s.ListEngineeringSubjectCandidates(context.Background(), first.Binding.OwnerID, first.Binding.ComputerID, first.Binding.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("the kept patch became %d candidate(s): %+v", len(candidates), candidates)
	}

	retry := f.open(1)
	deriver := &fakeCarriedDeriver{deriveErr: errors.New("an unfinished round has no candidate tree to derive from")}
	offer := f.offerTo(t, retry, deriver)
	if offer.Status != carriedOffered || !offer.SourceUnfinished || offer.PatchSHA256 != patch.SHA256 || offer.SourceAssignmentID != first.AssignmentID {
		t.Fatalf("offer = %+v, want the kept patch offered as unfinished", offer)
	}
	if len(deriver.derived) != 0 {
		t.Fatalf("the offer derived %v from a candidate tree", deriver.derived)
	}
	if got := carriedPatchDirForSpawn(offer); got != filepath.Join(f.root, "carried", patch.SHA256) {
		t.Fatalf("spawn dir = %q", got)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"unfinished", "unverified", "never frozen", "activation budget", patch.SHA256,
		"packages/reader/reader.go", "/selfdev/carried/source.patch", "git apply", "ignore", "re-run", "nothing has been applied"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
	for _, bad := range []string{"tests passed", "verified by", "is the starting point", "was rejected"} {
		if strings.Contains(notice, bad) {
			t.Fatalf("notice claims %q:\n%s", bad, notice)
		}
	}
}

// A1: off takes no snapshot, and turning it off stops offering kept ones.
func TestCarryUnfinishedOffTakesAndOffersNothing(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), false)
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)
	if len(f.caps.snapshots) != 0 {
		t.Fatalf("off took snapshots %v", f.caps.snapshots)
	}
	if _, err := os.Stat(filepath.Join(f.root, "carried")); !os.IsNotExist(err) {
		t.Fatalf("off wrote %s: %v", filepath.Join(f.root, "carried"), err)
	}

	// A kept round, then the switch turned off: the next round hears nothing.
	g := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	g.open(0)
	kept := g.bind(0)
	g.stopByBudget(t, kept)
	if len(readUnfinishedRounds(g.root, kept.Binding.TrajectoryID)) != 1 {
		t.Fatal("fixture: nothing was kept with the switch on")
	}
	setCarryUnfinished(t, g.admissionFixture, false)
	if offer := g.offerTo(t, g.open(1), &fakeCarriedDeriver{}); offer.Status != "" {
		t.Fatalf("with the switch off the next round was offered %+v", offer)
	}
}

// A2: the report path's revoke never snapshots.
func TestReportedRoundIsNotKeptAsUnfinished(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	f.open(0)
	bound := f.bind(0)
	if _, err := f.rt.revokeAssignedCapsule(context.Background(), bound, "terminal assignment report recorded"); err != nil {
		t.Fatal(err)
	}
	if len(f.caps.snapshots) != 0 {
		t.Fatalf("the report path took snapshots %v", f.caps.snapshots)
	}
	for _, disposition := range []types.EngineeringAssignmentDisposition{types.EngineeringAssignmentCompleted, types.EngineeringAssignmentFailed} {
		reported := bound
		reported.Disposition = disposition
		if unfinishedSnapshotWanted(reported) {
			t.Fatalf("a %s round is wanted as unfinished", disposition)
		}
	}
}

// A3: a failed snapshot is logged; the round is still cancelled and its
// capsule destroyed, and nothing is kept.
func TestFailedSnapshotNeverBlocksTheCancel(t *testing.T) {
	f := newUnfinishedRoundFixture(t, capsule.CarriedPatch{}, true)
	f.caps.err = errors.New("freeze capsule before its unfinished snapshot: context deadline exceeded")
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)
	if !f.caps.destroyed[first.Binding.CapsuleID] {
		t.Fatal("a failed snapshot left the capsule alive")
	}
	if rounds := readUnfinishedRounds(f.root, first.Binding.TrajectoryID); len(rounds) != 0 {
		t.Fatalf("a failed snapshot kept %+v", rounds)
	}
	// An empty change is the same: nothing kept, nothing said.
	g := newUnfinishedRoundFixture(t, capsule.CarriedPatch{}, true)
	g.caps.err = capsule.ErrCarriedPatchEmpty
	g.open(0)
	second := g.bind(0)
	g.stopByBudget(t, second)
	if rounds := readUnfinishedRounds(g.root, second.Binding.TrajectoryID); len(rounds) != 0 {
		t.Fatalf("an empty change kept %+v", rounds)
	}
}

// A4: a verification round is not wanted; a gone capsule is not read.
func TestUnfinishedSnapshotWantsOnlyALiveImplementationRound(t *testing.T) {
	assignment := types.EngineeringAssignment{AssignmentID: "a", Disposition: types.EngineeringAssignmentBound,
		Binding: types.EngineeringAssignmentBinding{Kind: types.EngineeringAssignmentImplementation, CapsuleID: "c"}}
	if !unfinishedSnapshotWanted(assignment) {
		t.Fatal("a bound implementation round is not wanted")
	}
	cancelled := assignment
	cancelled.Disposition = types.EngineeringAssignmentCancelled
	if !unfinishedSnapshotWanted(cancelled) {
		t.Fatal("a cancelled implementation round (a management cancel, the deadline) is not wanted")
	}
	verification := assignment
	verification.Binding.Kind = types.EngineeringAssignmentVerification
	if unfinishedSnapshotWanted(verification) {
		t.Fatal("a verification round is wanted")
	}

	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	f.open(0)
	bound := f.bind(0)
	f.caps.destroyed[bound.Binding.CapsuleID] = true
	f.rt.keepUnfinishedRoundBeforeDestroy(context.Background(), f.caps, bound, "run_failed")
	if len(f.caps.snapshots) != 0 {
		t.Fatalf("a gone capsule was read: %v", f.caps.snapshots)
	}
}

// A5: a retried revoke keeps one record and takes one snapshot.
func TestUnfinishedSnapshotIsTakenOnce(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	f.open(0)
	bound := f.bind(0)
	for i := 0; i < 2; i++ {
		f.rt.keepUnfinishedRoundBeforeDestroy(context.Background(), f.caps, bound, "run_failed")
	}
	if len(f.caps.snapshots) != 1 {
		t.Fatalf("snapshots = %v, want one", f.caps.snapshots)
	}
	if rounds := readUnfinishedRounds(f.root, bound.Binding.TrajectoryID); len(rounds) != 1 {
		t.Fatalf("kept = %+v, want one record", rounds)
	}
}

// A8: kept bytes that changed, or that name a path outside the source, are
// refused and never mounted.
func TestTamperedUnfinishedPatchIsRefused(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)
	rounds := readUnfinishedRounds(f.root, first.Binding.TrajectoryID)
	if len(rounds) != 1 {
		t.Fatal("fixture: nothing kept")
	}
	blob := unfinishedPatchPath(f.root, rounds[0].PatchSHA256)
	if err := os.Chmod(blob, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("diff --git a/../../etc/passwd b/../../etc/passwd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	offer := f.offerTo(t, f.open(1), &fakeCarriedDeriver{})
	if offer.Status != carriedRefused || carriedPatchDirForSpawn(offer) != "" || !offer.SourceUnfinished {
		t.Fatalf("a tampered kept patch gave %+v", offer)
	}
	if notice := carriedPatchNotice(offer); !strings.Contains(notice, "not carried") {
		t.Fatalf("notice = %q", notice)
	}
	if _, err := loadUnfinishedPatch(f.root, rounds[0].PatchSHA256); err == nil {
		t.Fatal("the tampered blob loads")
	}
}

// A9, A11: a candidate beats a kept patch of the same assignment, and a
// record that disagrees with its assignment is not believed.
func TestMergeUnfinishedRoundsTrustsTheAssignment(t *testing.T) {
	binding := types.EngineeringAssignmentBinding{TrajectoryID: "traj", Attempt: 1, Kind: types.EngineeringAssignmentImplementation, SourceArtifactRef: baseA}
	assignments := map[string]types.EngineeringAssignment{
		"stopped":   {AssignmentID: "stopped", Binding: binding},
		"finished":  {AssignmentID: "finished", Binding: binding},
		"elsewhere": {AssignmentID: "elsewhere", Binding: types.EngineeringAssignmentBinding{TrajectoryID: "other", Attempt: 1, Kind: types.EngineeringAssignmentImplementation, SourceArtifactRef: baseA}},
		"rebased":   {AssignmentID: "rebased", Binding: binding},
	}
	lookup := func(id string, _ uint64) (types.EngineeringAssignment, error) {
		a, ok := assignments[id]
		if !ok {
			return types.EngineeringAssignment{}, store.ErrNotFound
		}
		return a, nil
	}
	now := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC)
	round := func(id string, attempt uint64, base string) unfinishedRound {
		return unfinishedRound{Schema: carriedUnfinishedSchema, PatchSHA256: strings.Repeat("ab", 32), TrajectoryID: "traj", AssignmentID: id, Attempt: attempt, BaseRef: base, TakenAt: now}
	}
	entries := []carriedCandidate{failedRound("finished", "capsule-subject:sha256:c1", baseA, time.Hour)}
	got := mergeUnfinishedRounds(entries, []unfinishedRound{
		round("stopped", 1, baseA), round("finished", 1, baseA), round("elsewhere", 1, baseA),
		round("stopped", 2, baseA), round("rebased", 1, baseB), round("unknown", 1, baseA),
	}, "traj", lookup)
	var unfinished []string
	for _, entry := range got {
		if entry.Unfinished {
			unfinished = append(unfinished, entry.AssignmentID)
			if entry.BaseRef != baseA || entry.Kind != types.EngineeringAssignmentImplementation || entry.CandidateRef != carriedUnfinishedRefPrefix+strings.Repeat("ab", 32) {
				t.Fatalf("entry = %+v", entry)
			}
		}
	}
	if strings.Join(unfinished, ",") != "stopped" || len(got) != 2 {
		t.Fatalf("merged = %+v; want the candidate plus only 'stopped'", got)
	}
}

// A10: the carry's rule holds for a kept patch: an applied or pending
// operation never carries; a failed one (a cancel fails it) does.
func TestUnfinishedRoundFollowsTheCarryRule(t *testing.T) {
	kept := carriedCandidate{CandidateID: "unfinished:stopped", CandidateRef: carriedUnfinishedRefPrefix + "ab", AssignmentID: "stopped", Attempt: 1,
		Kind: types.EngineeringAssignmentImplementation, Unfinished: true, CreatedAt: time.Now()}
	for _, state := range []string{selfdev.StateApplied, selfdev.StateAwaitingApproval} {
		entry := kept
		entry.OperationState = state
		if chosen, _ := pickCarriedCandidate([]carriedCandidate{entry}, "retry"); chosen != nil {
			t.Fatalf("a kept round with operation %s is carried", state)
		}
	}
	for _, state := range []string{"", selfdev.StateFailed} {
		entry := kept
		entry.OperationState = state
		if chosen, _ := pickCarriedCandidate([]carriedCandidate{entry}, "retry"); chosen == nil {
			t.Fatalf("a kept round with operation %q is not carried", state)
		}
	}
	if chosen, _ := pickCarriedCandidate([]carriedCandidate{kept}, "stopped"); chosen != nil {
		t.Fatal("a round was offered its own kept patch")
	}
}

// A12: everything is under <updater root>/carried.
func TestUnfinishedSnapshotWritesOnlyUnderCarried(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)
	f.offerTo(t, f.open(1), &fakeCarriedDeriver{})
	var outside []string
	_ = filepath.Walk(f.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if rel, _ := filepath.Rel(filepath.Join(f.root, "carried"), path); strings.HasPrefix(rel, "..") {
			outside = append(outside, path)
		}
		return nil
	})
	if len(outside) != 0 {
		t.Fatalf("written outside carried/: %v", outside)
	}
	for _, path := range []string{unfinishedPatchPath(f.root, readUnfinishedRounds(f.root, first.Binding.TrajectoryID)[0].PatchSHA256)} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("%s is writable or missing: %v %v", path, info, err)
		}
	}
}
