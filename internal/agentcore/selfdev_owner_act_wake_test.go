package agentcore

// Owner acts on a self-development candidate wake the Texture of the
// document that supervises it
// (docs/problems/owner-acts-on-a-candidate-do-not-wake-texture-2026-10-25.md,
// register SD-08, R5-5). Each test names the failure modes it pins (numbers
// are the problem record's "Failure modes of the fix", written before it):
//
//  1. an act does not wake Texture          -> ...ApproveAndApplyEachWakeOnce, ...RejectWakesOnce
//  2. one act mints two records or wakes    -> ...ApproveAndApplyEachWakeOnce (observer race,
//     same-key replay, a second reconcile), ...RejectWakesOnce (replay)
//  3. approve and apply collapse into one   -> ...ApproveAndApplyEachWakeOnce
//  4. no bound document errors or misroutes -> ...NoBoundDocumentMintsNothing
//  5. the record lacks act or operation id  -> ...ApproveAndApplyEachWakeOnce, ...RejectWakesOnce
//  6. the decision path writes the document -> ...ApproveAndApplyEachWakeOnce, ...RejectWakesOnce
//  7. a crash boot wakes Texture            -> ...CrashSettledApplyMintsNothing
//  8. a failed wake fails the decision      -> ...NoBoundDocumentMintsNothing (decision answers 200)
//  9. Texture reads a "closure"             -> ...ApproveAndApplyEachWakeOnce (view kind owner_act)
// 10. the switch is off and something moves -> ...SwitchOffMintsNothing

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// seedOwnerActDocument starts a Texture document lifecycle on the fixture's
// computer, as texture_lifecycle_create does (doc_id in the subject refs),
// and returns its trajectory id and Texture inbox.
func seedOwnerActDocument(t *testing.T, s *store.Store, ownerID, computerID, docID string) (string, string) {
	t.Helper()
	agentID := currentTextureAgentID(docID)
	trajectoryID := "trajectory-owner-act-" + docID
	now := time.Now().UTC()
	req := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "owner-act-start:" + docID,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "owner-act-work:" + docID, Objective: "supervise the candidate", AssignedAgentID: agentID},
		InitialDocument: types.Document{DocID: docID, Title: "Reader request"},
		InitialRevision: types.Revision{
			RevisionID: "owner-act-revision:" + docID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID,
			Content: "The Reader is not verified or accepted.",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start document lifecycle: %v", err)
	}
	return trajectoryID, agentID
}

func ownerActSwitch(t *testing.T, s *store.Store, ownerID, computerID string, on bool) {
	t.Helper()
	if err := s.PutComputerSetting(context.Background(), store.ComputerSetting{
		OwnerID: ownerID, ComputerID: computerID, Key: store.SettingTextureOwnerActWakes, Value: on,
	}); err != nil {
		t.Fatal(err)
	}
}

// ownerActRecords are the owner-act records in one Texture inbox, oldest first.
func ownerActRecords(t *testing.T, s *store.Store, ownerID, computerID, inbox string) []types.CommitmentRecord {
	t.Helper()
	all, err := s.ListInboxRecords(context.Background(), ownerID, computerID, inbox, 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, rec := range all {
		if types.IsSelfDevelopmentOwnerAct(rec) {
			out = append(out, rec)
		}
	}
	return out
}

func ownerActHead(t *testing.T, s *store.Store, ownerID, computerID, docID string) string {
	t.Helper()
	doc, err := s.GetLifecycleDocument(context.Background(), ownerID, computerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	return doc.CurrentRevisionID
}

// checkOwnerActRecord pins one record's shape: a non-desk system Note to
// the Texture inbox, carrying the act and the operation, that wakes its
// reader and shows to Texture as an owner act.
func checkOwnerActRecord(t *testing.T, rec types.CommitmentRecord, inbox, operationID, act string) {
	t.Helper()
	if rec.Inbox != inbox || rec.EmitterKind != types.CommitmentEmitterSystem || rec.Final || rec.Round != 0 {
		t.Fatalf("owner-act record %+v; want a non-final round-0 system record to %s", rec, inbox)
	}
	if rec.SenderInbox != types.OwnerActSenderInbox(operationID) {
		t.Fatalf("owner-act record sent from %q, want %q", rec.SenderInbox, types.OwnerActSenderInbox(operationID))
	}
	if rec.RecordID != types.OwnerActRecordID(operationID, act) {
		t.Fatalf("owner-act record id %q, want %q", rec.RecordID, types.OwnerActRecordID(operationID, act))
	}
	if rec.SubjectRefs["operation_id"] != operationID || rec.SubjectRefs["owner_act"] != act {
		t.Fatalf("owner-act record subject refs %v; want operation %s act %s", rec.SubjectRefs, operationID, act)
	}
	if rec.Directive == nil || rec.Directive.Subtype != types.CommitmentDirectiveNote || !strings.Contains(rec.Directive.Body, operationID) {
		t.Fatalf("owner-act record directive %+v; want a Note naming %s", rec.Directive, operationID)
	}
	if !store.RecordWakesReader(rec) {
		t.Fatalf("owner-act record %s does not wake Texture", rec.RecordID)
	}
	if view := inboxRecordView(rec); view.Kind != "owner_act" || view.Body != rec.Directive.Body {
		t.Fatalf("Texture's view of %s is kind %q; want owner_act", rec.RecordID, view.Kind)
	}
}

// ownerActDecisionRequest is the owner's decision binding at the current head.
func ownerActDecisionRequest(t *testing.T, f *p4Fixture, operation selfdev.Operation, decision, key string) selfDevelopmentDecisionRequest {
	t.Helper()
	head := f.waitForStableHead(t)
	pending := head.PendingTransitionRef
	request := selfDevelopmentDecisionRequest{
		Decision: decision, IdempotencyKey: key,
		BundleDigest: operation.BundleDigest, VerifierRef: operation.VerifierRefs[0],
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
	}
	if decision == "reject" {
		request.Reason = "the page split is still wrong: page 1 shows page 2's text"
		arm := platform.SetSelfDevelopmentModeRequest{
			Mode: platform.SelfDevelopmentModeProposeOnly, ExpectedGeneration: f.currentMode(t).Generation,
			IdempotencyKey: key + "-propose-only",
		}
		if code, _, body := f.ownerWrite(t, arm, platform.ComputerTierTest); code != http.StatusOK {
			t.Fatalf("owner propose_only: %d %s", code, body)
		}
		return request
	}
	arm := platform.SetSelfDevelopmentModeRequest{
		Mode: platform.SelfDevelopmentModeAcceptOnce, ExpectedGeneration: f.currentMode(t).Generation,
		OperationID: operation.OperationID, BundleDigest: operation.BundleDigest,
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		ExpiresAt:      time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano),
		IdempotencyKey: key + "-accept-once",
	}
	if code, _, body := f.ownerWrite(t, arm, platform.ComputerTierTest); code != http.StatusOK {
		t.Fatalf("owner accept_once: %d %s", code, body)
	}
	return request
}

// ownerActReplay is the same-key replay a caller builds after the decision
// moved the head: the binding comes from the release witness, as the CLI
// rebuilds it.
func ownerActReplay(t *testing.T, f *p4Fixture, request selfDevelopmentDecisionRequest, decided selfdev.Operation) selfDevelopmentDecisionRequest {
	t.Helper()
	raw, err := json.Marshal(f.rt.readReleaseWitness(context.Background(), decided))
	if err != nil {
		t.Fatal(err)
	}
	var witness struct {
		Decision map[string]any `json:"decision"`
	}
	if err := json.Unmarshal(raw, &witness); err != nil {
		t.Fatal(err)
	}
	field := func(name string) string { value, _ := witness.Decision[name].(string); return value }
	replay := request
	pending := field("expected_pending_transition_ref")
	replay.ExpectedPendingTransitionRef = &pending
	replay.ExpectedDesiredEventHead, replay.ExpectedEffectiveEventHead = field("expected_desired_event_head"), field("expected_effective_event_head")
	replay.ExpectedDesiredStateCommitment, replay.ExpectedEffectiveStateCommitment = field("expected_desired_state_commitment"), field("expected_effective_state_commitment")
	return replay
}

func TestOwnerActWakeApproveAndApplyEachWakeOnce(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	trajectoryID, inbox := seedOwnerActDocument(t, f.store, "owner", f.computerID, "doc-approve")
	f.trajectoryID = trajectoryID
	ownerActSwitch(t, f.store, "owner", f.computerID, true)
	operation := f.verify(t, f.freeze(t, "owner-act-approve", "owner act approve"))
	if operation.TrajectoryID != trajectoryID {
		t.Fatalf("operation trajectory %q, want the document's %q", operation.TrajectoryID, trajectoryID)
	}
	headBefore := ownerActHead(t, f.store, "owner", f.computerID, "doc-approve")
	if got := ownerActRecords(t, f.store, "owner", f.computerID, inbox); len(got) != 0 {
		t.Fatalf("records before the owner act: %+v", got)
	}

	request := ownerActDecisionRequest(t, f, operation, "approve", "owner-act-approve-key")
	// The post-commit observer wins the race: it finalizes and applies
	// before the handler moves the operation, as on staging.
	f.rt.selfdevDecisionBeforeTransition = func(ctx context.Context, _ string) {
		f.rt.reconcileSelfDevelopmentMaterialization(ctx)
	}
	handler := &APIHandler{rt: f.rt}
	decided, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request)
	f.rt.selfdevDecisionBeforeTransition = nil
	if status != http.StatusOK || err != nil {
		t.Fatalf("approve answered %d: %v", status, err)
	}
	f.waitForState(t, operation.OperationID, selfdev.StateApplied)

	got := ownerActRecords(t, f.store, "owner", f.computerID, inbox)
	if len(got) != 2 {
		t.Fatalf("texture inbox holds %d owner-act records after approve and apply, want 2: %+v", len(got), got)
	}
	acts := map[string]types.CommitmentRecord{}
	for _, rec := range got {
		acts[rec.SubjectRefs["owner_act"]] = rec
	}
	approve, okApprove := acts[types.OwnerActApprove]
	apply, okApply := acts[types.OwnerActApply]
	if !okApprove || !okApply {
		t.Fatalf("owner-act records are %v; want one approve and one apply", acts)
	}
	checkOwnerActRecord(t, approve, inbox, operation.OperationID, types.OwnerActApprove)
	checkOwnerActRecord(t, apply, inbox, operation.OperationID, types.OwnerActApply)
	if approve.Directive.Body == apply.Directive.Body {
		t.Fatal("approve and apply carry the same text; Texture cannot tell accepted from applied")
	}

	// A same-key replay and another reconcile pass mint nothing more.
	replayed, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, ownerActReplay(t, f, request, decided))
	if status != http.StatusOK || err != nil || replayed.State != selfdev.StateApplied {
		t.Fatalf("same-key replay answered %d %v: %+v", status, err, replayed)
	}
	f.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if again := ownerActRecords(t, f.store, "owner", f.computerID, inbox); len(again) != 2 {
		t.Fatalf("replay left %d owner-act records, want 2", len(again))
	}
	head, err := f.store.InboxHead(ctx, "owner", f.computerID, inbox)
	if err != nil || head != 2 {
		t.Fatalf("texture inbox head %d (%v), want 2", head, err)
	}

	// The decision path wrote no document revision: Texture is woken, the
	// document is Texture's to change.
	if after := ownerActHead(t, f.store, "owner", f.computerID, "doc-approve"); after != headBefore {
		t.Fatalf("the document head moved %s -> %s without a Texture turn", headBefore, after)
	}
}

func TestOwnerActWakeRejectWakesOnce(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	trajectoryID, inbox := seedOwnerActDocument(t, f.store, "owner", f.computerID, "doc-reject")
	f.trajectoryID = trajectoryID
	ownerActSwitch(t, f.store, "owner", f.computerID, true)
	operation := f.verify(t, f.freeze(t, "owner-act-reject", "owner act reject"))
	headBefore := ownerActHead(t, f.store, "owner", f.computerID, "doc-reject")

	request := ownerActDecisionRequest(t, f, operation, "reject", "owner-act-reject-key")
	handler := &APIHandler{rt: f.rt}
	decided, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request)
	if status != http.StatusOK || err != nil || decided.State != selfdev.StateRejected {
		t.Fatalf("reject answered %d %v: %+v", status, err, decided)
	}
	got := ownerActRecords(t, f.store, "owner", f.computerID, inbox)
	if len(got) != 1 {
		t.Fatalf("texture inbox holds %d owner-act records after reject, want 1: %+v", len(got), got)
	}
	checkOwnerActRecord(t, got[0], inbox, operation.OperationID, types.OwnerActReject)
	if !strings.Contains(got[0].Directive.Body, request.Reason) {
		t.Fatalf("the reject record lost the owner's reason: %q", got[0].Directive.Body)
	}

	replayed, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, ownerActReplay(t, f, request, decided))
	if status != http.StatusOK || err != nil || replayed.State != selfdev.StateRejected {
		t.Fatalf("same-key replay answered %d %v: %+v", status, err, replayed)
	}
	if again := ownerActRecords(t, f.store, "owner", f.computerID, inbox); len(again) != 1 {
		t.Fatalf("replay left %d owner-act records, want 1", len(again))
	}
	if after := ownerActHead(t, f.store, "owner", f.computerID, "doc-reject"); after != headBefore {
		t.Fatalf("the document head moved %s -> %s without a Texture turn", headBefore, after)
	}
}

// An operation whose trajectory binds no document (a generated trajectory,
// as rollback and genesis operations have) wakes nobody, and the decision
// still lands.
func TestOwnerActWakeNoBoundDocumentMintsNothing(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	_, inbox := seedOwnerActDocument(t, f.store, "owner", f.computerID, "doc-unbound")
	ownerActSwitch(t, f.store, "owner", f.computerID, true)
	operation := f.verify(t, f.freeze(t, "owner-act-unbound", "owner act unbound"))
	if strings.HasPrefix(operation.TrajectoryID, "trajectory-owner-act-") {
		t.Fatalf("operation is bound to a seeded document: %s", operation.TrajectoryID)
	}
	request := ownerActDecisionRequest(t, f, operation, "reject", "owner-act-unbound-key")
	handler := &APIHandler{rt: f.rt}
	if decided, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request); status != http.StatusOK || err != nil || decided.State != selfdev.StateRejected {
		t.Fatalf("reject of an unbound candidate answered %d %v: %+v", status, err, decided)
	}
	if got := ownerActRecords(t, f.store, "owner", f.computerID, inbox); len(got) != 0 {
		t.Fatalf("an unbound candidate woke another document's Texture: %+v", got)
	}
	current, err := f.operations.Get(ctx, f.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	minted, err := f.rt.wakeTextureForOwnerAct(ctx, "owner", current, types.OwnerActReject, request.Reason)
	if err != nil || minted {
		t.Fatalf("direct wake of an unbound candidate: minted=%v err=%v; want nothing and no error", minted, err)
	}
}

func TestOwnerActWakeSwitchOffMintsNothing(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	trajectoryID, inbox := seedOwnerActDocument(t, f.store, "owner", f.computerID, "doc-off")
	f.trajectoryID = trajectoryID
	// No setting stored: the default.
	operation := f.verify(t, f.freeze(t, "owner-act-off", "owner act off"))
	request := ownerActDecisionRequest(t, f, operation, "approve", "owner-act-off-key")
	handler := &APIHandler{rt: f.rt}
	if _, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request); status != http.StatusOK || err != nil {
		t.Fatalf("approve answered %d: %v", status, err)
	}
	f.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if head, err := f.store.InboxHead(ctx, "owner", f.computerID, inbox); err != nil || head != 0 {
		t.Fatalf("texture inbox head %d (%v) with the switch off, want 0", head, err)
	}
}

// A crash boot settles an interrupted apply from the updater journal. That
// settlement is not new work: it mints no owner-act record, so no Texture
// turn starts because the computer restarted.
func TestOwnerActWakeCrashSettledApplyMintsNothing(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-owner-act-crash")
	ctx := context.Background()
	trajectoryID, inbox := seedOwnerActDocument(t, fx.store, "owner", fx.computerID, "doc-crash")
	fx.trajectoryID = trajectoryID
	ownerActSwitch(t, fx.store, "owner", fx.computerID, true)
	fx.controlFail.Store(true)
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "owner-act-crash")
	fx.appendDecision(t, operation, bundleDigest, "owner-act-crash-decision")
	fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)

	// The crash boot: the operation is marked interrupted while control is
	// still failing (so no live pass can finish it first), and the next
	// reconcile settles it from the journal only.
	fx.rt.markSelfDevelopmentOperationsInterruptedAfterCrash(ctx)
	fx.rt.selfdevMaterializeMu.Lock()
	interrupted := fx.rt.selfdevCrashInterrupted[operation.OperationID]
	fx.rt.selfdevMaterializeMu.Unlock()
	if !interrupted {
		t.Fatalf("crash boot did not mark %s interrupted", operation.OperationID)
	}
	fx.controlFail.Store(false)
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if got := ownerActRecords(t, fx.store, "owner", fx.computerID, inbox); len(got) != 0 {
		t.Fatalf("a crash-boot settlement woke Texture: %+v", got)
	}
}
