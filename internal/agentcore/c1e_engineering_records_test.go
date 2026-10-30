package agentcore

// C1e batch B1: engineering on records
// (docs/archive/reports/c1e-b1-design-note-2026-10-13.md, "Ways this could fail",
// written before the code). Each item names the test that pins it.
//
//  1. A management turn reads an engineering Report before the row is
//     terminal. Prevented by D3: they commit in one transaction.
//     -> TestC1eItem1FinalReportCommitsWithTheTerminalRow (and, at the store,
//     TestEngineeringReportsCommitRecordsWithTheRowAndTerminalProjection)
//  2. A cast with no valid parent (no open round with Texture, as in the
//     paper path's own management turns). Refused by D4's validation.
//     -> TestC1eItems2And11CastWithoutValidParentCommitsNothing
//  3. Two documents' requests, each with an open assignment. D6 is scoped
//     per parent round. Tested with interleaved Reports.
//     -> TestC1eItem3ChildOutcomeGateIsScopedPerParentRound
//  4. The engineering Report arrives while a management turn is mid-call.
//     D6 refuses that turn's final Report to Texture; the turn's re-check
//     wakes one more turn, which reads it.
//     -> TestC1eItems4And9ReportDuringTheTurnHoldsTheRoundAndWakesAReader
//  5. A crash between Complete and management's next turn. The boot closes
//     both rounds interrupted_by_restart, silently. Nothing resumes.
//     -> TestC1eItems5And6CrashBootInterruptsSilently
//  6. A run-end closure at boot cannot happen: the boot writes only the
//     crash closure, which stays silent (D7).
//     -> TestC1eItems5And6CrashBootInterruptsSilently
//  7. A partial Complete, then a terminal one. The first mints a progress
//     Report, the second the final (D3).
//     -> TestC1eItem7ProgressThenFinalReport
//  8. An engineering run dies with its assignment open. The run-end branch
//     cancels it in one transaction; the later asynchronous reconcile does
//     nothing (D7). Tested in both orders.
//     -> TestC1eItem8RunEndSettleAndReconcileMintOneFinalReport
//  9. Engineering finishes while management's last call is in flight. The
//     outcome is above the cursor, so Texture's round survives the turn's
//     end, and the re-check wakes a reader (D5).
//     -> TestC1eItems4And9ReportDuringTheTurnHoldsTheRoundAndWakesAReader,
//     TestC1eItem9HoldPredicate
// 10. A retry on the same trajectory after a failed assignment opens its
//     own operation; Freeze and Verify resolve by assignment and never see
//     the earlier bundle (D2).
//     -> TestC1eItem10RetryOpensItsOwnOperation
// 11. A cast whose admission fails commits nothing: no Cast, no round, no
//     row (D4).
//     -> TestC1eItems2And11CastWithoutValidParentCommitsNothing
// 12. The paper path: Autopaper's terminal-outcome packets do not cross
//     this route, so B1 does not change them (step 7). Pinned here only as
//     far as B1 reaches it: management's answer to a Texture round that
//     opened no assignment is not gated by D6.
//     -> TestC1eItems2And11CastWithoutValidParentCommitsNothing (first act)
//
// The design note's residual "store cancel versus run cancel (D7) is
// checked by a test first" -> TestC1eStoreCancelProjectsTheRunButNotTheLoop.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// c1eWakes captures the inbox wakes a runtime dispatches.
type c1eWakes struct {
	mu    sync.Mutex
	wakes []string
}

func (w *c1eWakes) install(rt *Runtime) {
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.wakes = append(w.wakes, kind+"->"+to)
		return nil
	})
}

func (w *c1eWakes) list() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.wakes...)
}

// c1eRuntime is a runtime whose assigned capsules are absent (revoke acks
// without a capsule) and whose wakes are captured.
func c1eRuntime(t *testing.T) (*Runtime, *store.Store, *c1eWakes) {
	t.Helper()
	rt, s := testRuntime(t)
	rt.assignmentRuntime = absentAssignmentCapsule{}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	wakes := &c1eWakes{}
	wakes.install(rt)
	return rt, s, wakes
}

// c1eOpenBound opens and binds seed assignment index and returns the row
// and its durable run.
func c1eOpenBound(t *testing.T, s *store.Store, seed store.EngineeringAssignmentSeed, index int) (types.EngineeringAssignment, types.RunRecord) {
	t.Helper()
	ctx := context.Background()
	capability := "cap-" + seed.AssignmentIDs[index]
	open := store.SeedEngineeringOpenRequest(seed, index, capability)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("open %s: %v", open.AssignmentID, err)
	}
	bound, err := s.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, seed.AssignedRunIDs[index], capability))
	if err != nil {
		t.Fatalf("bind %s: %v", open.AssignmentID, err)
	}
	run, err := s.GetLifecycleRun(ctx, seed.OwnerID, seed.ComputerID, seed.AssignedRunIDs[index])
	if err != nil {
		t.Fatal(err)
	}
	return bound.Assignment, run
}

// c1eCancel is a system cancellation: revoke, then the cancel that mints
// the final Report unless silent.
func c1eCancel(t *testing.T, rt *Runtime, assignment types.EngineeringAssignment, reason string, silent bool) types.EngineeringAssignment {
	t.Helper()
	ctx := context.Background()
	revoked, err := rt.revokeAssignedCapsule(ctx, assignment, reason)
	if err != nil {
		t.Fatalf("revoke %s: %v", assignment.AssignmentID, err)
	}
	cancelled, err := rt.persistSystemEngineeringCancellation(ctx, revoked, reason, silent)
	if err != nil {
		t.Fatalf("cancel %s: %v", assignment.AssignmentID, err)
	}
	return cancelled.Assignment
}

func c1eGet(t *testing.T, s *store.Store, assignment types.EngineeringAssignment) types.EngineeringAssignment {
	t.Helper()
	got, err := s.GetEngineeringAssignment(context.Background(), assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// c1eFromEngineering lists the records management's inbox holds from one
// assignment's engineering inbox.
func c1eFromEngineering(t *testing.T, s *store.Store, assignment types.EngineeringAssignment) []types.CommitmentRecord {
	t.Helper()
	all, err := s.ListInboxRecords(context.Background(), assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentAgentID, 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, rec := range all {
		if rec.SenderInbox == assignment.Binding.AssignedAgentID {
			out = append(out, rec)
		}
	}
	return out
}

func c1eRoundOpen(t *testing.T, s *store.Store, ownerID, computerID, conversation string) bool {
	t.Helper()
	head, err := s.ConversationState(context.Background(), ownerID, computerID, conversation)
	if err != nil {
		t.Fatal(err)
	}
	return head.Open
}

// c1eManagementReduction is management's records turn on the seed's run.
func c1eManagementReduction(rt *Runtime, seed store.EngineeringAssignmentSeed, run types.RunRecord, cell string) (*rlmCallReduction, context.Context) {
	scope := ReductionScope{
		FromAgentID: seed.ParentAgentID, DeskAgentID: seed.ParentAgentID, FromRole: agentprofile.Management,
		ChannelID: seed.ParentAgentID, RunID: run.RunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CellID: cell,
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: seed.OwnerID, ChannelID: seed.ParentAgentID, ComputerID: seed.ComputerID,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &run,
	})
	return &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}, ctx
}

func c1eManagementRun(t *testing.T, s *store.Store, seed store.EngineeringAssignmentSeed) types.RunRecord {
	t.Helper()
	run, err := s.GetRunByOwnerOG(context.Background(), seed.OwnerID, seed.ParentRunID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// c1eAnswer is management's final Report into a Texture's round.
func c1eAnswer(textureInbox string) yaegikernel.StagedIntent {
	return yaegikernel.StagedIntent{LocalID: "answer", Kind: yaegikernel.IntentReport, ToDesk: textureInbox,
		Packet: `{"kind":"blocker","summary":"answered","notes":["answered"],"work_disposition":"completed"}`}
}

func c1eInboxHead(t *testing.T, s *store.Store, ownerID, computerID, inbox string) int64 {
	t.Helper()
	head, err := s.InboxHead(context.Background(), ownerID, computerID, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return head
}

// Item 1: the final Report and the terminal row are one transaction. A
// cancel that fails leaves neither; the one that succeeds leaves both, and
// the Report is the record that closes the engineering round.
func TestC1eItem1FinalReportCommitsWithTheTerminalRow(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-1", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	if !c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("the Cast did not open the engineering round")
	}
	revoked, err := rt.revokeAssignedCapsule(ctx, assignment, "item 1")
	if err != nil {
		t.Fatal(err)
	}
	stale := types.CancelEngineeringAssignmentRequest{
		CommandID: "c1e-stale-cancel", OwnerID: seed.OwnerID, ComputerID: seed.ComputerID,
		AssignmentID: revoked.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: revoked.LifecycleVersion - 1, Reason: "stale",
	}
	stale.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(stale)
	if _, err := s.CancelEngineeringAssignment(ctx, stale); err == nil {
		t.Fatal("a stale cancel committed")
	}
	if got := c1eGet(t, s, revoked); got.Disposition.Terminal() {
		t.Fatalf("a failed cancel turned the row terminal: %+v", got)
	}
	if recs := c1eFromEngineering(t, s, revoked); len(recs) != 0 {
		t.Fatalf("a failed cancel minted a Report: %+v", recs)
	}

	cancelled, err := rt.persistSystemEngineeringCancellation(ctx, revoked, "item 1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled.Assignment.Disposition.Terminal() {
		t.Fatalf("cancel left the row %s", cancelled.Assignment.Disposition)
	}
	recs := c1eFromEngineering(t, s, revoked)
	if len(recs) != 1 || !recs[0].Final || recs[0].ConversationID != assignment.Binding.AssignedConversationID {
		t.Fatalf("final Report = %+v", recs)
	}
	closing, found, err := s.EngineeringClosingRecord(ctx, cancelled.Assignment)
	if err != nil || !found || closing.RecordID != recs[0].RecordID {
		t.Fatalf("closing record = %+v found=%v err=%v; want %s", closing, found, err, recs[0].RecordID)
	}
	hasRef := false
	for _, ref := range recs[0].EvidenceRefs {
		hasRef = hasRef || ref == "assignment:"+assignment.AssignmentID
	}
	if !hasRef {
		t.Fatalf("final Report does not name its assignment: %v", recs[0].EvidenceRefs)
	}
	if c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("the final Report did not close the engineering round")
	}
}

// Items 2, 11 and 12: an engineering Cast with no valid parent, or whose
// admission fails, commits nothing; management's answer to a round that
// opened no assignment is not gated.
func TestC1eItems2And11CastWithoutValidParentCommitsNothing(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-2", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	run := c1eManagementRun(t, s, seed)
	nothingCommitted := func(t *testing.T, mgmtHead, textureHead int64) {
		t.Helper()
		if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.ParentAgentID); got != mgmtHead {
			t.Fatalf("management inbox moved: %d -> %d", mgmtHead, got)
		}
		if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox); got != textureHead {
			t.Fatalf("texture inbox moved: %d -> %d", textureHead, got)
		}
		if rows, err := s.ListEngineeringAssignments(ctx, seed.OwnerID, seed.ComputerID, seed.TrajectoryID); err != nil || len(rows) != 0 {
			t.Fatalf("assignment rows = %+v, %v", rows, err)
		}
	}
	cast := func(spec string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: "cast-eng", Kind: yaegikernel.IntentCast, ToDesk: agentprofile.Engineering, Objective: "change the platform", Statement: spec}
	}
	mgmtHead := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.ParentAgentID)
	textureHead := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox)

	refusals := []struct {
		name, spec, want string
	}{
		{"no parent", ``, "name the Texture request it serves"},
		{"unknown parent", `{"parent":"no-such-record"}`, "is not a Texture's Cast"},
		// Item 11: the parent is valid, but admission (the capsule source
		// preflight) is unavailable: refused before anything commits.
		{"admission unavailable", `{"parent":"` + seed.TextureCastID + `"}`, "capsule authority unavailable"},
	}
	for i, tc := range refusals {
		if tc.name == "admission unavailable" {
			rt.capsuleExecutor = nil
		}
		reduction, cellCtx := c1eManagementReduction(rt, seed, run, "c1e-2-cell-"+string(rune('a'+i)))
		err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{cast(tc.spec)}, 1)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
		nothingCommitted(t, mgmtHead, textureHead)
	}

	// Item 12: the paper path's own management answer: no assignment was
	// opened for this round, so D6 lets the final Report through.
	reduction, cellCtx := c1eManagementReduction(rt, seed, run, "c1e-2-answer")
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{c1eAnswer(seed.TextureInbox)}, 1); err != nil {
		t.Fatalf("management's answer with no child assignment was refused: %v", err)
	}
	if c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, seed.ParentConversationID) {
		t.Fatal("management's final Report did not close the Texture round")
	}

	// Item 2: the round the parent opened is closed now (as in the paper
	// path's own management turns): a Cast naming it is refused.
	mgmtHead = c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.ParentAgentID)
	textureHead = c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox)
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	reduction, cellCtx = c1eManagementReduction(rt, seed, run, "c1e-2-closed")
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{cast(`{"parent":"` + seed.TextureCastID + `"}`)}, 1); err == nil || !strings.Contains(err.Error(), "is not open") {
		t.Fatalf("Cast into a closed round: err = %v", err)
	}
	nothingCommitted(t, mgmtHead, textureHead)
}

// Item 11 at the store: an open whose admission fails (a parent round that
// is not the open one) writes no Cast, no round and no row.
func TestC1eItem11StoreAdmissionFailureCommitsNothing(t *testing.T) {
	_, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-11", "autoputer-test", 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "cap-c1e-11")
	open.Binding.ParentRound = seed.ParentRound + 1
	open.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(open)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err == nil {
		t.Fatal("an open naming a round that is not open was admitted")
	}
	if _, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, open.AssignmentID, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("refused open left a row: %v", err)
	}
	if rec, err := s.GetCommitmentRecord(ctx, seed.OwnerID, seed.ComputerID, open.Binding.ParentControlID); err != nil || rec != nil {
		t.Fatalf("refused open left its Cast: %+v, %v", rec, err)
	}
	if c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, open.Binding.AssignedConversationID) {
		t.Fatal("refused open opened the engineering round")
	}
	if head := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, open.Binding.AssignedAgentID); head != 0 {
		t.Fatalf("refused open wrote the engineering inbox: head %d", head)
	}
}

// Item 3: D6 is scoped per parent round. Two documents' requests each hold
// an assignment; their Reports interleave, and each Texture's answer is
// gated only by its own round's assignment.
func TestC1eItem3ChildOutcomeGateIsScopedPerParentRound(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	seedA, err := store.SeedEngineeringAssignmentAuthorityForDocument(s, "owner-c1e-3", rt.TextureComputerID(), "alpha", 1)
	if err != nil {
		t.Fatal(err)
	}
	seedB, err := store.SeedEngineeringAssignmentAuthorityForDocument(s, "owner-c1e-3", rt.TextureComputerID(), "beta", 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c1eOpenBound(t, s, seedA, 0)
	b, _ := c1eOpenBound(t, s, seedB, 0)
	run := c1eManagementRun(t, s, seedA)
	gate := func(seed store.EngineeringAssignmentSeed) error {
		reduction, ctx := c1eManagementReduction(rt, seed, run, "c1e-3-gate")
		return reduction.requireChildOutcomesRead(ctx, seed.ParentConversationID)
	}
	if err := gate(seedA); err == nil || !strings.Contains(err.Error(), a.AssignmentID) {
		t.Fatalf("alpha's answer with its assignment open: %v", err)
	}
	if err := gate(seedB); err == nil || !strings.Contains(err.Error(), b.AssignmentID) {
		t.Fatalf("beta's answer with its assignment open: %v", err)
	}

	// Alpha's engineering ends first; management has not read it.
	a = c1eCancel(t, rt, a, "alpha ended", false)
	if err := gate(seedA); err == nil || !strings.Contains(err.Error(), "not read") {
		t.Fatalf("alpha's answer before reading its outcome: %v", err)
	}
	if err := gate(seedB); err == nil || !strings.Contains(err.Error(), b.AssignmentID) {
		t.Fatalf("beta's gate followed alpha's outcome: %v", err)
	}
	closingA, _, _ := s.EngineeringClosingRecord(context.Background(), a)
	rt.noteRecordsShown(run.RunID, closingA.InboxSeq)
	if err := gate(seedA); err != nil {
		t.Fatalf("alpha's answer after reading its outcome: %v", err)
	}
	if err := gate(seedB); err == nil {
		t.Fatal("reading alpha's outcome opened beta's gate")
	}

	// Beta's ends after: alpha stays open, beta needs its own read.
	b = c1eCancel(t, rt, b, "beta ended", false)
	if err := gate(seedB); err == nil || !strings.Contains(err.Error(), "not read") {
		t.Fatalf("beta's answer before reading its outcome: %v", err)
	}
	if err := gate(seedA); err != nil {
		t.Fatalf("beta's outcome closed alpha's gate: %v", err)
	}
	closingB, _, _ := s.EngineeringClosingRecord(context.Background(), b)
	rt.noteRecordsShown(run.RunID, closingB.InboxSeq)
	if err := gate(seedB); err != nil {
		t.Fatalf("beta's answer after reading its outcome: %v", err)
	}
}

// Items 4 and 9: the engineering Report lands while management's turn is
// in flight. The turn's final Report to Texture is refused and nothing
// commits; when the turn ends the Texture round survives (D5) and the
// re-check wakes management to read the outcome.
func TestC1eItems4And9ReportDuringTheTurnHoldsTheRoundAndWakesAReader(t *testing.T) {
	rt, s, wakes := c1eRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-4", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	run := c1eManagementRun(t, s, seed)
	// The turn read the Texture's Cast only.
	rt.noteRecordsShown(run.RunID, 1)
	c1eCancel(t, rt, assignment, "engineering ended mid-turn", false)

	textureHead := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox)
	reduction, ctx := c1eManagementReduction(rt, seed, run, "c1e-4-answer")
	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{c1eAnswer(seed.TextureInbox)}, 1); err == nil || !strings.Contains(err.Error(), "not read") {
		t.Fatalf("final Report before reading the engineering outcome: %v", err)
	}
	if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.TextureInbox); got != textureHead {
		t.Fatalf("refused answer reached the Texture: %d -> %d", textureHead, got)
	}
	if !c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, seed.ParentConversationID) {
		t.Fatal("refused answer closed the Texture round")
	}

	// The turn ends without answering.
	ended := run
	ended.State = types.RunCompleted
	rt.recheckRecordsInbox(context.Background(), &ended)
	if !c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, seed.ParentConversationID) {
		t.Fatal("the run-end closure closed a round whose assignment outcome is unread (D5)")
	}
	woke := false
	for _, w := range wakes.list() {
		woke = woke || w == "inbox_record->"+seed.ParentAgentID
	}
	if !woke {
		t.Fatalf("no reader was woken for the unread outcome: %v", wakes.list())
	}
}

// Item 9's predicate: a round is held while an assignment it opened is
// open, or ended with a record above the cursor committed after the ending
// turn began; a record the turn could have seen does not hold it.
func TestC1eItem9HoldPredicate(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-9", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	run := c1eManagementRun(t, s, seed)
	round, err := s.ConversationState(context.Background(), seed.OwnerID, seed.ComputerID, seed.ParentConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.childAssignmentsHoldRound(context.Background(), &run, round) {
		t.Fatal("an open assignment did not hold its round")
	}
	c1eCancel(t, rt, assignment, "ended", false)
	if !rt.childAssignmentsHoldRound(context.Background(), &run, round) {
		t.Fatal("an unread outcome committed during the turn did not hold its round")
	}
	later := run
	later.CreatedAt = time.Now().UTC().Add(time.Second)
	if rt.childAssignmentsHoldRound(context.Background(), &later, round) {
		t.Fatal("an outcome committed before the turn began held its round")
	}
}

// Items 5 and 6: a crash boot closes both rounds interrupted_by_restart and
// turns the open assignment terminal silently: no engineering Report, no
// wake, no resume, and a run-end closure of the crash-closed run writes
// nothing. Both orders of closure and reconcile converge.
func TestC1eItems5And6CrashBootInterruptsSilently(t *testing.T) {
	for _, order := range []string{"closure-then-reconcile", "reconcile-then-closure"} {
		t.Run(order, func(t *testing.T) {
			rt, s, wakes := c1eRuntime(t)
			ctx := context.Background()
			owner := "owner-c1e-5-" + order
			t.Setenv("CHOIR_OWNER_ID", owner)
			seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
			if err != nil {
				t.Fatal(err)
			}
			assignment, engRun := c1eOpenBound(t, s, seed, 0)
			rt.bootRestartPlanned = false
			if order == "closure-then-reconcile" {
				rt.closeOpenRoundsAfterCrash(ctx)
				rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
			} else {
				rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
				rt.closeOpenRoundsAfterCrash(ctx)
			}
			got := c1eGet(t, s, assignment)
			if !got.Disposition.Terminal() || got.DispositionReason != engineeringInterruptedByRestart {
				t.Fatalf("assignment after crash boot = %s %q", got.Disposition, got.DispositionReason)
			}
			recs := c1eFromEngineering(t, s, assignment)
			if len(recs) != 1 || recs[0].EmitterKind != types.CommitmentEmitterSystem || recs[0].Report == nil ||
				recs[0].Report.StopReason != types.ClosureReasonInterruptedByRestart {
				t.Fatalf("engineering round closure = %+v; want only the silent crash closure", recs)
			}
			if _, found, err := s.EngineeringClosingRecord(ctx, got); err != nil || found {
				t.Fatalf("crash boot minted an engineering Report: found=%v err=%v", found, err)
			}
			for _, conversation := range []string{seed.ParentConversationID, assignment.Binding.AssignedConversationID} {
				if c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, conversation) {
					t.Fatalf("round of %s survived the crash boot", conversation)
				}
			}
			if w := wakes.list(); len(w) != 0 {
				t.Fatalf("crash boot woke: %v", w)
			}

			// Item 6: the crash-closed engineering run's end closes nothing.
			before := len(c1eFromEngineering(t, s, assignment))
			crashed := engRun
			crashed.State, crashed.Error = types.RunCancelled, runFateInterruptedByRestart
			rt.closeRoundsOfEndedRun(ctx, &crashed)
			if after := len(c1eFromEngineering(t, s, assignment)); after != before {
				t.Fatalf("run-end closure at boot wrote %d records", after-before)
			}
			if w := wakes.list(); len(w) != 0 {
				t.Fatalf("run-end closure at boot woke: %v", w)
			}
		})
	}
}

// Item 7: a partial Complete mints a progress Report (not final, the round
// stays open); the terminal transition mints the final one.
func TestC1eItem7ProgressThenFinalReport(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-7", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	partial := types.EngineeringAssignmentReport{Result: types.EngineeringResultPartial, Verdict: types.EngineeringVerdictNone, Summary: "halfway there"}
	if _, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:partial-1", partial); err != nil {
		t.Fatalf("partial Complete: %v", err)
	}
	recs := c1eFromEngineering(t, s, assignment)
	if len(recs) != 1 || recs[0].Final {
		t.Fatalf("after partial Complete: %+v; want one progress Report", recs)
	}
	if got := c1eGet(t, s, assignment); got.Disposition.Terminal() {
		t.Fatalf("partial Complete turned the row %s", got.Disposition)
	}
	if !c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("a progress Report closed the engineering round")
	}
	terminal := c1eCancel(t, rt, c1eGet(t, s, assignment), "terminal", false)
	recs = c1eFromEngineering(t, s, assignment)
	if len(recs) != 2 || recs[0].Final || !recs[1].Final {
		t.Fatalf("after terminal transition: %+v; want progress then final", recs)
	}
	if !terminal.Disposition.Terminal() || c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("the final Report did not end the round with the row")
	}
}

// Item 8: an engineering run that ends with its assignment open is settled
// by the run-end branch (one final Report, reason run_<state>); the
// asynchronous reconcile then does nothing. In the other order the
// reconcile settles and the run-end branch writes nothing.
func TestC1eItem8RunEndSettleAndReconcileMintOneFinalReport(t *testing.T) {
	for _, order := range []string{"run-end-first", "reconcile-first"} {
		t.Run(order, func(t *testing.T) {
			rt, s, _ := c1eRuntime(t)
			ctx := context.Background()
			seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-8-"+order, rt.TextureComputerID(), 1)
			if err != nil {
				t.Fatal(err)
			}
			assignment, run := c1eOpenBound(t, s, seed, 0)
			ended := run
			ended.State = types.RunFailed
			reconcile := func() {
				if err := rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, seed.OwnerID, seed.ComputerID, seed.TrajectoryID); err != nil {
					t.Fatalf("reconcile: %v", err)
				}
			}
			if order == "run-end-first" {
				rt.closeRoundsOfEndedRun(ctx, &ended)
				if got := c1eGet(t, s, assignment); got.DispositionReason != "run_failed" {
					t.Fatalf("run-end settle reason = %q", got.DispositionReason)
				}
				reconcile()
			} else {
				reconcile()
				rt.closeRoundsOfEndedRun(ctx, &ended)
			}
			got := c1eGet(t, s, assignment)
			if !got.Disposition.Terminal() {
				t.Fatalf("assignment = %s", got.Disposition)
			}
			finals := 0
			for _, rec := range c1eFromEngineering(t, s, assignment) {
				if rec.Final {
					finals++
				}
			}
			if finals != 1 {
				t.Fatalf("final records = %d; want exactly one", finals)
			}
			// A second pass of both writes nothing.
			reconcile()
			rt.closeRoundsOfEndedRun(ctx, &ended)
			if again := len(c1eFromEngineering(t, s, assignment)); again != 1 {
				t.Fatalf("records after a second pass = %d", again)
			}
		})
	}
}

// Item 10: after a failed assignment, a retry on the same trajectory opens
// its own operation. Its Freeze resolves by its own assignment and never
// sees the earlier bundle; the earlier operation is untouched.
func TestC1eItem10RetryOpensItsOwnOperation(t *testing.T) {
	ctx := context.Background()
	productStore, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	trajectory := "trajectory-retry"
	first, err := openFreezeCandidate(ctx, operations, authorized, computerID, trajectory, "assignment-first", "first try")
	if err != nil {
		t.Fatal(err)
	}
	earlierBundle := strings.Repeat("e", 64)
	if _, err := operations.Transition(ctx, computerID, first.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = earlierBundle
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevOperations: operations}
	rt.failBoundSelfdevOperation(ctx, computerID, "assignment-first", "assignment failed")
	failed, err := operations.Get(ctx, computerID, first.OperationID)
	if err != nil || failed.State != selfdev.StateFailed {
		t.Fatalf("first operation after its assignment failed = %+v, %v", failed, err)
	}

	toolCtx := &CapsuleToolCtx{
		ComputerID: computerID, AgentRunID: "run-second", UpdaterRoot: t.TempDir(),
		Executor:           capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0),
		OperationStore:     operations,
		EventProjection:    freezeTestProjection{store: productStore},
		EventAppender:      new(computerevent.ComputerEventAppender),
		TransactionBuilder: transaction.NewTransactionBuilder(transaction.NewClassifier()),
		OpenFreezeCandidate: func(ctx context.Context, trajectoryID string) (selfdev.Operation, error) {
			return openFreezeCandidate(ctx, operations, authorized, computerID, trajectoryID, "assignment-second", "second try")
		},
	}
	rec := &types.RunRecord{RunID: "run-second", TrajectoryID: trajectory, Metadata: map[string]any{"assignment_id": "assignment-second"}}
	out, freezeErr := freezeCapsuleEffectBundle(ctx, toolCtx, rec, "handle-second",
		"recipe:sha256:"+strings.Repeat("1", 64), []string{"test:sha256:" + strings.Repeat("2", 64)}, []string{"toolchain:sha256:" + strings.Repeat("3", 64)})
	if digest, _ := out["bundle_digest"].(string); digest == earlierBundle {
		t.Fatalf("the retry's freeze returned the earlier bundle: %+v", out)
	}
	if freezeErr != nil && strings.Contains(freezeErr.Error(), "resolve self-development operation") {
		t.Fatalf("the retry could not open its own operation: %v", freezeErr)
	}
	second, found, err := operations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey("assignment-second"))
	if err != nil || !found {
		t.Fatalf("the retry's operation = found %v, %v", found, err)
	}
	if second.OperationID == first.OperationID || second.BundleDigest == earlierBundle || second.TrajectoryID != trajectory {
		t.Fatalf("the retry resolved the earlier operation: %+v", second)
	}
	// Verify resolves the same way: by the assignment's key.
	if byFirst, _, _ := operations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey("assignment-first")); byFirst.OperationID != first.OperationID || byFirst.State != selfdev.StateFailed || byFirst.BundleDigest != earlierBundle {
		t.Fatalf("the earlier operation changed: %+v", byFirst)
	}
}

// D7 residual: the store cancel projects the bound run terminal in its own
// transaction but cannot reach the live loop; the runtime cancel stops the
// loop too. Only the casting management desk may cancel.
func TestC1eStoreCancelProjectsTheRunButNotTheLoop(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-d7", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	stopped := false
	rt.runningMu.Lock()
	rt.running[run.RunID] = func() { stopped = true }
	rt.runningMu.Unlock()

	other := types.RunRecord{RunID: "run-other", AgentID: "management:someone-else", OwnerID: seed.OwnerID, ComputerID: seed.ComputerID}
	if _, err := rt.cancelAssignedEngineering(ctx, other, assignment.AssignmentID, 1, "not mine"); err == nil {
		t.Fatal("a desk that did not cast the assignment cancelled it")
	}
	caster := types.RunRecord{RunID: "run-fresh-turn", AgentID: persistentManagementAgentID(seed.OwnerID), OwnerID: seed.OwnerID, ComputerID: seed.ComputerID}
	cancelled, err := rt.cancelAssignedEngineering(ctx, caster, assignment.AssignmentID, 1, "owner changed course")
	if err != nil {
		t.Fatalf("the casting desk's fresh turn could not cancel: %v", err)
	}
	if !cancelled.Assignment.Disposition.Terminal() {
		t.Fatalf("cancel left the row %s", cancelled.Assignment.Disposition)
	}
	projected, err := s.GetLifecycleRun(ctx, seed.OwnerID, seed.ComputerID, run.RunID)
	if err != nil || !projected.State.Terminal() {
		t.Fatalf("store cancel did not project the run terminal: %+v, %v", projected, err)
	}
	if !stopped {
		t.Fatal("the cancel did not stop the live loop")
	}
	if recs := c1eFromEngineering(t, s, assignment); len(recs) != 1 || !recs[0].Final {
		t.Fatalf("cancellation Report = %+v; want one final Report", recs)
	}
}
