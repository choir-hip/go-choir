package agentcore

// Carry selection (residuals SD-03 and SD-04; trace review Round 5, R5-3 and
// R5-4; docs/problems/selfdev-carry-selection-drops-the-newest-candidate-2026-10-26.md).
// The owner's intent: each engineering round is a differential next step on
// the newest candidate the owner has seen. Carrying nothing, or an older
// tree, without saying so is the defect.
//
// The rule, decided before the code:
//
//  (a) Hold. An implementation request whose trajectory's newest work (its
//      newest candidate, or a stopped round newer than it) has a
//      self-development operation still open (selfdev.OpenStates) is not
//      admitted. Admission refuses it retryably (why "candidate_pending",
//      naming the candidate and the operation's state), the request stays
//      open and unbound in its queue position, and it is admitted on the
//      first retry after the operation settles; its offer then carries the
//      settled candidate by the usual rule (rejected and failed carry,
//      applied does not). Chosen over carrying the pending tree as the base:
//      a pending candidate may still apply, and a round built on it would
//      then apply its change twice; and the store allows one open operation
//      per computer (selfdev.ErrOperationOpen), so a round bound in that
//      window cannot Freeze until the operation settles anyway. No deadlock:
//      the hold is a retryable refusal returned under
//      engineeringAssignmentOpenMu, never a wait while holding it, and
//      nothing that settles an operation (management's verify, the owner's
//      decision, the apply) goes through engineering admission. Bounded: a
//      request held past the assignment deadline is expired by the existing
//      rule, with its visible cancel. An offer decided while the newest work
//      is pending (a path around admission) is a refusal that names it and
//      is not recorded, so a replay decides again.
//  (b) Rank. The newest candidate on the tape (frozen or reported, by its
//      creation order) outranks every newer stopped round that does not
//      descend from it. A stopped round descends from it when the round's
//      own recorded offer was decided with that candidate in view (offered;
//      or refused, since the same refusal would meet the next round too),
//      directly or through a chain of stopped rounds. The newest of the
//      candidate and its descendants is carried, and outranked stopped
//      rounds are named in the offer and the notice. When the newest
//      candidate is neither carriable nor pending (applied, or finished and
//      never proposed) it is not a starting point the owner saw, and stopped
//      rounds rank by age as before.
//  (c) A rejected candidate still carries: a rejection with fixes is the
//      normal iteration. No owner act says "start over" today (none in the
//      code), so there is nothing to gate on.
//
// Ways it can fail, each with a test below:
//
//  S1. A request binds while the newest candidate's operation is open (any
//      of selfdev.OpenStates), so it starts from the base.
//  S2. A held request is cancelled or bound instead of left open and
//      unbound.
//  S3. A held request stays held after the operation settles (rejected,
//      failed, applied), or is admitted and then carries nothing.
//  S4. A verification request, or a request on another trajectory, is held
//      by this trajectory's candidate.
//  S5. The hold fires on a settled candidate that will never change again
//      (applied, rolled back, never proposed), stalling the trajectory.
//  S6. An offer decided while the newest candidate is pending carries
//      nothing silently, or records the refusal so a replay keeps it.
//  S7. A newer stopped round that started from the base outranks the newest
//      rejected or failed candidate.
//  S8. A stopped round that started from the newest candidate (directly or
//      through stopped rounds) loses to it, and the work it added on top is
//      dropped.
//  S9. A stopped round whose offer of the newest candidate was refused is
//      outranked by that candidate (the same refusal would meet the next
//      round, and the stopped round's work is lost).
//  S10. An outranked stopped round is not named to the engineer.
//  S11. With no candidate on the trajectory, or the newest one applied or
//      never proposed, a stopped round stops carrying.
//  S12. A history read that fails admits the request (and so carries
//      nothing) instead of holding it.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// stoppedRound is a kept unfinished round, as mergeUnfinishedRounds adds it.
func stoppedRound(id, sha string, age time.Duration) carriedCandidate {
	return carriedCandidate{
		CandidateID: "unfinished:" + id, CandidateRef: carriedUnfinishedRefPrefix + sha, AssignmentID: id, Attempt: 1, BaseRef: baseA,
		Kind: types.EngineeringAssignmentImplementation, Unfinished: true, UnfinishedReason: "activation budget exceeded",
		CreatedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC).Add(-age),
	}
}

// viewedFrom answers which candidate each round's recorded offer named.
func viewedFrom(m map[string]string) func(carriedCandidate) string {
	return func(entry carriedCandidate) string { return m[entry.AssignmentID] }
}

func chosenID(sel carrySelection) string {
	if sel.Chosen == nil {
		return ""
	}
	return sel.Chosen.CandidateID
}

// S1, S5: an open operation on the newest candidate holds; a settled one
// never does.
func TestCarrySelectHoldsOnlyWhileTheNewestCandidateIsOpen(t *testing.T) {
	older := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "older", 3*time.Hour)
	for _, state := range selfdev.OpenStates {
		newest := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, state, "", 2*time.Hour)
		sel := selectCarriedCandidate([]carriedCandidate{older, newest}, "retry", viewedFrom(nil))
		if sel.Pending == nil || sel.Pending.AssignmentID != "round-2" || sel.Chosen != nil {
			t.Fatalf("state %s: selection %+v; want held on round-2 and nothing chosen", state, sel)
		}
		// A newer stopped round that did not start from it does not get past
		// the hold either.
		stopped := stoppedRound("round-3", strings.Repeat("ab", 32), time.Hour)
		sel = selectCarriedCandidate([]carriedCandidate{older, newest, stopped}, "retry", viewedFrom(nil))
		if sel.Pending == nil || sel.Pending.AssignmentID != "round-2" || sel.Chosen != nil {
			t.Fatalf("state %s with a newer stopped round: selection %+v; want held on round-2", state, sel)
		}
	}
	for _, state := range []string{selfdev.StateRejected, selfdev.StateFailed} {
		newest := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, state, "", 2*time.Hour)
		sel := selectCarriedCandidate([]carriedCandidate{older, newest}, "retry", viewedFrom(nil))
		if sel.Pending != nil || chosenID(sel) != "candidate-round-2" {
			t.Fatalf("state %s: selection %+v; want round-2 carried", state, sel)
		}
	}
	for _, state := range []string{selfdev.StateApplied, selfdev.StateRolledBack, ""} {
		newest := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, state, "", 2*time.Hour)
		sel := selectCarriedCandidate([]carriedCandidate{older, newest}, "retry", viewedFrom(nil))
		if sel.Pending != nil || sel.Chosen != nil {
			t.Fatalf("state %q: selection %+v; want neither held nor carried", state, sel)
		}
	}
	// The round's own candidate and a verification candidate never hold it.
	own := finishedRound("retry", "capsule-subject:sha256:own", baseA, selfdev.StateAwaitingApproval, "", time.Minute)
	verify := finishedRound("verify-1", "capsule-subject:sha256:v", baseA, selfdev.StateFrozen, "", time.Minute)
	verify.Kind = types.EngineeringAssignmentVerification
	if sel := selectCarriedCandidate([]carriedCandidate{own, verify}, "retry", viewedFrom(nil)); sel.Pending != nil {
		t.Fatalf("held on %+v", sel.Pending)
	}
}

// S7, S10: a stopped round that started from the base is outranked by the
// newest rejected or failed candidate, and is named.
func TestCarrySelectNewestCandidateOutranksAStoppedRoundFromTheBase(t *testing.T) {
	rejected := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "fix the page split", 2*time.Hour)
	failedOp := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateFailed, "host build failed", 2*time.Hour)
	failedReport := failedRound("round-1", "capsule-subject:sha256:c1", baseA, 2*time.Hour)
	for _, candidate := range []carriedCandidate{rejected, failedOp, failedReport} {
		stopped := stoppedRound("round-2", strings.Repeat("ab", 32), time.Hour)
		sel := selectCarriedCandidate([]carriedCandidate{stopped, candidate}, "retry", viewedFrom(nil))
		if chosenID(sel) != "candidate-round-1" {
			t.Fatalf("candidate %s/%s: chose %q; want the candidate over the stopped round", candidate.Result, candidate.OperationState, chosenID(sel))
		}
		if strings.Join(sel.Outranked, ",") != "round-2" {
			t.Fatalf("outranked = %v; want the stopped round named", sel.Outranked)
		}
		// A stopped round that started from an older candidate is outranked
		// the same way.
		oldest := finishedRound("round-0", "capsule-subject:sha256:c0", baseA, selfdev.StateRejected, "", 3*time.Hour)
		sel = selectCarriedCandidate([]carriedCandidate{stopped, candidate, oldest}, "retry", viewedFrom(map[string]string{"round-2": "candidate-round-0"}))
		if chosenID(sel) != "candidate-round-1" || strings.Join(sel.Outranked, ",") != "round-2" {
			t.Fatalf("stopped round from an older candidate: chose %q, outranked %v", chosenID(sel), sel.Outranked)
		}
		if strings.Join(sel.Older, ",") != "candidate-round-0" {
			t.Fatalf("older = %v; want the older candidate named", sel.Older)
		}
	}
}

// S8, S9: a stopped round that descends from the newest candidate carries,
// through a chain of stopped rounds, and through a refused offer of it.
func TestCarrySelectStoppedRoundThatStartedFromTheNewestCandidateCarries(t *testing.T) {
	candidate := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "", 3*time.Hour)
	first := stoppedRound("round-2", strings.Repeat("ab", 32), 2*time.Hour)
	second := stoppedRound("round-3", strings.Repeat("cd", 32), time.Hour)

	direct := selectCarriedCandidate([]carriedCandidate{candidate, first}, "retry", viewedFrom(map[string]string{"round-2": "candidate-round-1"}))
	if chosenID(direct) != "unfinished:round-2" || len(direct.Outranked) != 0 {
		t.Fatalf("direct: chose %q outranked %v; want the stopped round", chosenID(direct), direct.Outranked)
	}
	chain := selectCarriedCandidate([]carriedCandidate{candidate, first, second}, "retry",
		viewedFrom(map[string]string{"round-2": "candidate-round-1", "round-3": "unfinished:round-2"}))
	if chosenID(chain) != "unfinished:round-3" || len(chain.Outranked) != 0 {
		t.Fatalf("chain: chose %q outranked %v; want the newest descendant", chosenID(chain), chain.Outranked)
	}
	// A broken chain: round-3 started from the base, round-2 from the
	// candidate. round-2 is the newest descendant; round-3 is named.
	broken := selectCarriedCandidate([]carriedCandidate{candidate, first, second}, "retry", viewedFrom(map[string]string{"round-2": "candidate-round-1"}))
	if chosenID(broken) != "unfinished:round-2" || strings.Join(broken.Outranked, ",") != "round-3" {
		t.Fatalf("broken chain: chose %q outranked %v", chosenID(broken), broken.Outranked)
	}
}

// S11: no candidate, or a newest candidate that is no starting point, leaves
// stopped rounds ranked by age.
func TestCarrySelectStoppedRoundStillCarriesWithNoStartingCandidate(t *testing.T) {
	stopped := stoppedRound("round-2", strings.Repeat("ab", 32), time.Hour)
	if sel := selectCarriedCandidate([]carriedCandidate{stopped}, "retry", viewedFrom(nil)); chosenID(sel) != "unfinished:round-2" {
		t.Fatalf("alone: chose %q", chosenID(sel))
	}
	applied := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateApplied, "", 2*time.Hour)
	unproposed := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, "", "", 2*time.Hour)
	for _, older := range []carriedCandidate{applied, unproposed} {
		sel := selectCarriedCandidate([]carriedCandidate{older, stopped}, "retry", viewedFrom(nil))
		if chosenID(sel) != "unfinished:round-2" || sel.Pending != nil || len(sel.Outranked) != 0 {
			t.Fatalf("newest candidate %q: selection %+v; want the stopped round", older.OperationState, sel)
		}
	}
}

// S7, S10 through the offer: the candidate is carried, and the notice names
// the stopped round it outranked.
func TestCarriedOfferNamesTheStoppedRoundTheNewestCandidateOutranked(t *testing.T) {
	patch := testCarriedPatch("frontend/src/apps/reader/Reader.svelte")
	rejected := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "page 1 shows page 2 text", 2*time.Hour)
	stopped := stoppedRound("round-2", strings.Repeat("ab", 32), time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{"capsule-subject:sha256:c1": patch}}
	deps := carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{stopped, rejected}}, deriver)
	deps.CarryUnfinished = true
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != carriedOffered || offer.SourceUnfinished || offer.CandidateID != "candidate-round-1" || offer.PatchSHA256 != patch.SHA256 {
		t.Fatalf("offer = %+v; want the rejected candidate carried", offer)
	}
	if strings.Join(offer.OutrankedStopped, ",") != "round-2" {
		t.Fatalf("outranked = %v", offer.OutrankedStopped)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"page 1 shows page 2 text", "starting point", "round-2", "stopped before it froze", "did not start from this candidate"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
}

// S8, S9 through the offer: the stopped round's recorded offer decides its
// lineage, offered or refused.
func TestCarriedOfferCarriesAStoppedRoundThatStartedFromTheNewestCandidate(t *testing.T) {
	for _, status := range []string{carriedOffered, carriedRefused} {
		root := filepath.Join(t.TempDir(), "updater")
		patch := testCarriedPatch("packages/reader/reader.go")
		stopped := types.EngineeringAssignment{AssignmentID: "round-2", Disposition: types.EngineeringAssignmentCancelled,
			Binding: types.EngineeringAssignmentBinding{TrajectoryID: "traj", Attempt: 1, Kind: types.EngineeringAssignmentImplementation, CapsuleID: "c2", SourceArtifactRef: baseA}}
		if err := keepUnfinishedRound(context.Background(), newSnapshotCapsule(patch), root, stopped, "activation budget exceeded", time.Now()); err != nil {
			t.Fatal(err)
		}
		writeCarriedOffer(root, carriedPatchOffer{Schema: carriedOfferSchema, AssignmentID: "round-2", Attempt: 1, Status: status,
			CandidateID: "candidate-round-1", PatchSHA256: strings.Repeat("ef", 32)})
		rejected := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "", 2*time.Hour)
		kept := stoppedRound("round-2", patch.SHA256, time.Hour)
		deps := carriedPatchDeps{History: &fakeCarriedHistory{entries: []carriedCandidate{rejected, kept}}, Deriver: &fakeCarriedDeriver{}, Root: root, CarryUnfinished: true}
		offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
		if offer.Status != carriedOffered || !offer.SourceUnfinished || offer.SourceAssignmentID != "round-2" || len(offer.OutrankedStopped) != 0 {
			t.Fatalf("lineage %s: offer = %+v; want the stopped round carried", status, offer)
		}
		// Offered is "started from it"; refused is not, and the notice says so.
		if offer.OlderFrozenStartedFrom != (status == carriedOffered) {
			t.Fatalf("lineage %s: started from = %v", status, offer.OlderFrozenStartedFrom)
		}
	}
}

// S6: an offer decided while the newest candidate is pending is a refusal
// that names it, is never recorded, and derives nothing.
func TestCarriedOfferWhileTheNewestCandidateIsPendingRefusesAndIsNotRecorded(t *testing.T) {
	pending := finishedRound("round-2", "capsule-subject:sha256:c2", baseA, selfdev.StateVerified, "", time.Hour)
	older := finishedRound("round-1", "capsule-subject:sha256:c1", baseA, selfdev.StateRejected, "older", 2*time.Hour)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{
		"capsule-subject:sha256:c1": testCarriedPatch("a.go"), "capsule-subject:sha256:c2": testCarriedPatch("b.go"),
	}}
	deps := carriedDeps(t, &fakeCarriedHistory{entries: []carriedCandidate{pending, older}}, deriver)
	offer := offerCarriedPatch(context.Background(), deps, carriedInput(baseA))
	if offer.Status != carriedRefused || !offer.SourcePending || offer.CandidateID != "candidate-round-2" ||
		offer.SourceOperationState != selfdev.StateVerified || carriedPatchDirForSpawn(offer) != "" || len(deriver.derived) != 0 {
		t.Fatalf("offer = %+v derived %v; want a pending refusal naming round-2", offer, deriver.derived)
	}
	if recorded := readCarriedOffer(deps.Root, "assignment-retry", 1); recorded.Status != "" {
		t.Fatalf("the pending refusal was recorded: %+v", recorded)
	}
	notice := carriedPatchNotice(offer)
	for _, want := range []string{"candidate-round-2", "selfdev-round-2", selfdev.StateVerified, "still", "clean base"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice does not say %q:\n%s", want, notice)
		}
	}
	if strings.Contains(notice, "was rejected") || strings.Contains(notice, "git apply") {
		t.Fatalf("notice:\n%s", notice)
	}
}

// carryHoldFixture is a real store with assignment 0 reported (its
// candidate minted on the tape) and its operation in the given state, and
// assignment 1 opened on the same trajectory.
type carryHoldFixture struct {
	rt          *Runtime
	s           *store.Store
	operations  *selfdev.Store
	first       types.EngineeringAssignment
	retry       types.EngineeringAssignment
	operationID string
	candidateID string
}

func newCarryHoldFixture(t *testing.T, state string) carryHoldFixture {
	t.Helper()
	ctx := context.Background()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-carry-hold", rt.TextureComputerID(), 2)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := c1eOpenBound(t, s, seed, 0)
	report := terminalAssignmentReport(first, types.EngineeringResultCompleted)
	req := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "engineering-report:" + first.AssignmentID + ":" + report.ReportID,
		OwnerID:   first.Binding.OwnerID, ComputerID: first.Binding.ComputerID,
		AssignmentID: first.AssignmentID, Attempt: first.Binding.Attempt,
		ExpectedLifecycleVersion: first.LifecycleVersion, Report: report,
	}
	req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
	committed, err := s.RecordEngineeringAssignmentReport(ctx, req)
	if err != nil || !committed.Assignment.Disposition.Terminal() {
		t.Fatalf("report = %+v, %v", committed.Assignment.Disposition, err)
	}
	candidates, err := s.ListEngineeringSubjectCandidates(ctx, first.Binding.OwnerID, first.Binding.ComputerID, first.Binding.TrajectoryID)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates = %+v, %v; want the report's one", candidates, err)
	}
	operationID := seedAssignmentOperation(t, rt, first, state, "")
	open := store.SeedEngineeringOpenRequest(seed, 1, "cap-"+seed.AssignmentIDs[1])
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Assignment.Binding.TrajectoryID != first.Binding.TrajectoryID {
		t.Fatal("fixture: the retry is on another trajectory")
	}
	return carryHoldFixture{rt: rt, s: s, operations: operations, first: first, retry: opened.Assignment, operationID: operationID, candidateID: candidates[0].CandidateID}
}

func (f carryHoldFixture) admit(t *testing.T) error {
	t.Helper()
	return f.rt.admitEngineeringAssignment(context.Background(), c1eGet(t, f.s, f.retry))
}

// S1, S2, S3 on the real store: a request whose trajectory's candidate is
// awaiting its decision waits, open and unbound; once the decision rejects
// it, the request is admitted and its offer carries the candidate.
func TestAdmissionHoldsARequestUntilTheNewestCandidateSettles(t *testing.T) {
	ctx := context.Background()
	f := newCarryHoldFixture(t, selfdev.StateAwaitingApproval)
	err := f.admit(t)
	var busy *engineeringAdmissionBusy
	if !errors.As(err, &busy) || !errors.Is(err, ErrEngineeringAdmissionBusy) || busy.Why != "candidate_pending" {
		t.Fatalf("admission = %v; want a retryable candidate_pending refusal", err)
	}
	for _, want := range []string{f.candidateID, f.operationID, selfdev.StateAwaitingApproval} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal does not name %q: %v", want, err)
		}
	}
	if held := c1eGet(t, f.s, f.retry); held.Disposition != types.EngineeringAssignmentOpen || held.CapsuleDisposition != types.EngineeringCapsuleUnbound || held.BoundRunID != "" {
		t.Fatalf("the held request is %s/%s bound %q; want open and unbound", held.Disposition, held.CapsuleDisposition, held.BoundRunID)
	}

	if _, err := f.operations.Transition(ctx, f.first.Binding.ComputerID, f.operationID, selfdev.StateAwaitingApproval, selfdev.StateRejected, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.admit(t); err != nil {
		t.Fatalf("after the rejection the request is still refused: %v", err)
	}
	report := terminalAssignmentReport(f.first, types.EngineeringResultCompleted)
	deriver := &fakeCarriedDeriver{patches: map[string]capsule.CarriedPatch{report.CandidateArtifactRef: testCarriedPatch("frontend/src/apps/reader/Reader.svelte")}}
	root := filepath.Join(t.TempDir(), "updater")
	offer := offerCarriedPatch(ctx, carriedPatchDeps{History: storeCarriedHistory{rt: f.rt}, Deriver: deriver, Root: root},
		carriedPatchInput{OwnerID: f.retry.Binding.OwnerID, ComputerID: f.retry.Binding.ComputerID, TrajectoryID: f.retry.Binding.TrajectoryID,
			AssignmentID: f.retry.AssignmentID, Attempt: f.retry.Binding.Attempt, RetryBaseRef: f.first.Binding.SourceArtifactRef})
	if offer.Status != carriedOffered || offer.CandidateID != f.candidateID || offer.SourceOperationState != selfdev.StateRejected {
		t.Fatalf("offer after the rejection = %+v; want the candidate carried", offer)
	}
}

// S3, S5: every settled state admits; every open state holds.
func TestAdmissionHoldFollowsTheOperationState(t *testing.T) {
	for _, state := range selfdev.AllStates {
		f := newCarryHoldFixture(t, state)
		err := f.admit(t)
		open := false
		for _, s := range selfdev.OpenStates {
			open = open || s == state
		}
		if open != (err != nil) {
			t.Fatalf("state %s: admission = %v; want held %v", state, err, open)
		}
	}
}

// S4: a verification request is not held by the trajectory's candidate.
func TestAdmissionHoldIsForImplementationRequestsOnly(t *testing.T) {
	f := newCarryHoldFixture(t, selfdev.StateFrozen)
	verification := c1eGet(t, f.s, f.retry)
	verification.Binding.Kind = types.EngineeringAssignmentVerification
	if held, err := f.rt.carryHoldFor(context.Background(), verification); err != nil || held != nil {
		t.Fatalf("verification request held by %+v, %v", held, err)
	}
	other := c1eGet(t, f.s, f.retry)
	other.Binding.TrajectoryID = "trajectory-elsewhere"
	if held, err := f.rt.carryHoldFor(context.Background(), other); err != nil || held != nil {
		t.Fatalf("another trajectory's request held by %+v, %v", held, err)
	}
}

// S12: a history read that fails holds the request, retryably; it is not
// admitted to start from the base.
func TestAdmissionHoldsWhenTheHistoryCannotBeRead(t *testing.T) {
	if held, err := carryHoldFrom(context.Background(), &fakeCarriedHistory{err: errors.New("store down")}, "", types.EngineeringAssignment{
		AssignmentID: "retry", Binding: types.EngineeringAssignmentBinding{TrajectoryID: "traj", Attempt: 1, Kind: types.EngineeringAssignmentImplementation},
	}); err == nil || held != nil {
		t.Fatalf("a failed history read = %+v, %v; want the error", held, err)
	}
	f := newCarryHoldFixture(t, selfdev.StateRejected)
	if err := f.admit(t); err != nil {
		t.Fatalf("fixture: a rejected candidate holds: %v", err)
	}
	// The operations read fails; the assignment rows still read.
	if _, err := f.s.DB().Exec(`DROP TABLE self_development_operations`); err != nil {
		t.Fatal(err)
	}
	err := f.admit(t)
	var busy *engineeringAdmissionBusy
	if !errors.As(err, &busy) || busy.Why != "carry_unreadable" {
		t.Fatalf("admission with an unreadable history = %v; want a retryable carry_unreadable refusal", err)
	}
}
