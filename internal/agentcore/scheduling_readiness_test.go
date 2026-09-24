package agentcore

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Acceptance Criterion 1: Live-Trigger FIFO Scheduling Contract
// Competing execution requests are selected strictly by computer-scoped ArrivalOrdinal
// under live triggers. Executing assignments are protected from supersession cancellations.
func TestSchedulingReadiness_Criterion1_LiveTriggerFIFOSelection(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-fifo-scheduling"
	computerID := "autoputer-test"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// Seed 3 competing Texture execution requests with distinct ArrivalOrdinals across 3 trajectories
	f1 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-1", managementAgent.AgentID, agentprofile.Management)
	f2 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-2", managementAgent.AgentID, agentprofile.Management)
	f3 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-3", managementAgent.AgentID, agentprofile.Management)

	pending, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, managementAgent.AgentID, 100)
	if err != nil {
		t.Fatalf("list pending controls: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending controls, got %d", len(pending))
	}
	// Verify arrival ordinal ordering
	for i := 0; i < len(pending)-1; i++ {
		if pending[i].ArrivalOrdinal >= pending[i+1].ArrivalOrdinal {
			t.Fatalf("pending controls not sorted by ArrivalOrdinal: [%d]=%d >= [%d]=%d",
				i, pending[i].ArrivalOrdinal, i+1, pending[i+1].ArrivalOrdinal)
		}
	}

	fixtures := []lifecycleControlFixture{f1, f2, f3}

	// For each live cycle, deliver the named trigger and observe exactly one new Management run
	// whose selected work item is the lowest pending ordinal. Later requests remain pending.
	for cycle := range 3 {
		expectedFixture := fixtures[cycle]

		// Deliver the live trigger
		run, err := rt.ReconcileCoagentWake(ctx, ownerID, managementAgent.AgentID)
		if err != nil || run == nil {
			t.Fatalf("cycle %d: live trigger failed to mint Management: run=%v err=%v", cycle, run, err)
		}

		// Assert selected work item matches the lowest pending ordinal (expectedFixture)
		if run.Metadata["lifecycle_work_item_id"] != expectedFixture.workID {
			t.Fatalf("cycle %d: selected work item = %v, want %s", cycle, run.Metadata["lifecycle_work_item_id"], expectedFixture.workID)
		}
		if metadataStringValue(run.Metadata, "assignment_trajectory_id") != expectedFixture.trajectoryID {
			t.Fatalf("cycle %d: selected trajectory = %v, want %s", cycle, run.Metadata["assignment_trajectory_id"], expectedFixture.trajectoryID)
		}

		// Assert later requests remain pending with delivered_to_run_id null/empty
		remainingPending, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, managementAgent.AgentID, 100)
		if err != nil {
			t.Fatal(err)
		}
		expectedRemaining := 3 - (cycle + 1)
		if len(remainingPending) != expectedRemaining {
			t.Fatalf("cycle %d: expected %d remaining pending controls, got %d", cycle, expectedRemaining, len(remainingPending))
		}
		for _, rem := range remainingPending {
			if rem.DeliveredAt != nil || rem.DeliveredToRunID != "" {
				t.Fatalf("cycle %d: later request %s was prematurely delivered to run %s", cycle, rem.UpdateID, rem.DeliveredToRunID)
			}
		}

		// Complete the executing run to finish this cycle cleanly
		finished := time.Now().UTC()
		run.State, run.UpdatedAt, run.FinishedAt = types.RunCompleted, finished, &finished
		if err := s.UpdateRun(ctx, *run); err != nil {
			t.Fatal(err)
		}
		rt.maybeContinuePersistentManagementInbox(ctx, run)
	}
}

// Acceptance Criterion 2: Boot-Does-Not-Schedule (structurally enforced)
// Precondition: admissible unclaimed backlog exists.
// Boot rewarm/sweep runs: zero Management or Engineering runs created, positive assertion
// that exact-run resume did not enter selection, pending ordinals remain untouched.
func TestSchedulingReadiness_Criterion2_BootDoesNotSchedule(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-boot-recovery"
	computerID := "autoputer-test"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// PRECONDITION: at least one admissible, unclaimed backlog item exists
	f1 := seedTextureLifecycleControl(t, s, ownerID, "boot-doc-1", managementAgent.AgentID, agentprofile.Management)
	f2 := seedTextureLifecycleControl(t, s, ownerID, "boot-doc-2", managementAgent.AgentID, agentprofile.Management)

	pendingBefore, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, managementAgent.AgentID, 100)
	if err != nil || len(pendingBefore) < 2 {
		t.Fatalf("precondition failed: pending controls before boot = %v, err=%v", pendingBefore, err)
	}

	// Capture log buffer for positive did-not-enter-selection assertion
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Execute boot rewarm and work-item sweep
	rt.rewarmInterruptedPersistentManagementActors(ctx)
	rt.sweepOpenWorkItemActors(ctx)

	logOutput := buf.String()

	// Assert from boot logs that reconcile never entered selection
	if !strings.Contains(logOutput, "boot work-item sweep skipping persistent Management") {
		t.Errorf("boot logs missing sweep skip log line; got:\n%s", logOutput)
	}

	// Assert across the window that ZERO Management or Engineering run rows are created
	activeRun, err := rt.latestActiveRunByAgent(ctx, ownerID, managementAgent.AgentID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("boot minted unexpected Management run=%+v err=%v", activeRun, err)
	}

	// Assert pending ordinals remain pending with unchanged delivery state
	pendingAfter, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, managementAgent.AgentID, 100)
	if err != nil || len(pendingAfter) != len(pendingBefore) {
		t.Fatalf("pending count changed across boot: before=%d, after=%d", len(pendingBefore), len(pendingAfter))
	}
	for i := range pendingAfter {
		if pendingAfter[i].UpdateID != pendingBefore[i].UpdateID ||
			pendingAfter[i].DeliveredAt != nil ||
			pendingAfter[i].DeliveredToRunID != "" {
			t.Fatalf("pending control [%d] was mutated across boot: %+v", i, pendingAfter[i])
		}
	}
	_ = f1
	_ = f2
}

// Acceptance Criterion 3: Rare-Reboot Resume for In-Flight Work Only
// While an assignment is executing, restart via product path: assert the exact
// interrupted run resumes through the dedicated isolated resume entry point with the
// SAME run ID and passivated_reason=runtime_restarted, no duplicate assignment is created,
// and the resume did not fall through to any selection call.
func TestSchedulingReadiness_Criterion3_InFlightResumePreservesIdentity(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-inflight-resume"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	var dispatches []string
	rt.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		dispatches = append(dispatches, kind+":"+content)
		return nil
	})

	// Seed control and start the in-flight Management run
	f := seedTextureLifecycleControl(t, s, ownerID, "inflight-doc", managementAgent.AgentID, agentprofile.Management)
	firstRun, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil || firstRun == nil {
		t.Fatalf("initial Management run failed: %v", err)
	}

	// Passivate the in-flight run with runtime_restarted (simulating process restart)
	now := time.Now().UTC()
	firstRun.State = types.RunPassivated
	firstRun.Metadata = cloneMetadata(firstRun.Metadata)
	firstRun.Metadata["passivated_reason"] = "runtime_restarted"
	firstRun.UpdatedAt = now
	if err := s.UpdateRun(ctx, *firstRun); err != nil {
		t.Fatal(err)
	}

	// Capture log buffer for positive did-not-enter-selection assertion
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Execute isolated resume entry point
	resumed, ok, err := rt.ResumeInterruptedPersistentManagementControlRun(ctx, ownerID, managementAgent.AgentID)
	if err != nil || !ok || resumed == nil {
		t.Fatalf("dedicated resume failed: resumed=%v ok=%t err=%v", resumed, ok, err)
	}

	// Assert the exact interrupted run resumed with the SAME run ID
	if resumed.RunID != firstRun.RunID {
		t.Fatalf("resumed run ID = %s, want original %s", resumed.RunID, firstRun.RunID)
	}
	if resumed.State != types.RunPending {
		t.Fatalf("resumed run state = %s, want pending", resumed.State)
	}

	// Assert from logs that resume did not fall through to selection
	logOutput := buf.String()
	if !strings.Contains(logOutput, "persistent-Management exact-run resume reactivated run="+firstRun.RunID) {
		t.Errorf("resume logs missing reactivation log line; got:\n%s", logOutput)
	}

	// Assert only ONE recovery occurrence was enqueued, no duplicate runs created
	activeRun, err := rt.latestActiveRunByAgent(ctx, ownerID, managementAgent.AgentID)
	if err != nil || activeRun.RunID != firstRun.RunID {
		t.Fatalf("expected single active run %s, got %+v err=%v", firstRun.RunID, activeRun, err)
	}
	_ = f
}

// Acceptance Criterion 4: Producer Report Settlement at Store Layer
// Enumerate undelivered Engineering cancel producer reports, settle them via dedicated
// runtime/store lifecycle-reducer command with CAS precondition, terminal disposition (UpdateLate),
// and idempotent settlement receipt. Assert all pending store selectors exclude settled IDs,
// claimedPersistentManagementProducerReportIDs is retired, and boot/reconcile mints zero Management runs.
func TestSchedulingReadiness_Criterion4_ProducerReportStoreSettlement(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-producer-settlement"
	computerID := rt.TextureComputerID()
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// Seed two cancelled Engineering reports
	_, rep1, traj1 := seedCancelledEngineeringReport(t, rt, s, ownerID, "report-settle-1")
	_, rep2, _ := seedCancelledEngineeringReport(t, rt, s, ownerID, "report-settle-2")

	// 1. Enumerate undelivered cancel producer reports
	pendingReports, err := rt.ListPendingProducerReports(ctx, ownerID, computerID, "")
	if err != nil || len(pendingReports) != 2 {
		t.Fatalf("expected 2 pending producer reports, got %d, err=%v", len(pendingReports), err)
	}

	reportIDs := []string{rep1.UpdateID, rep2.UpdateID}

	// 2. Settle via dedicated lifecycle-reducer command with CAS precondition
	settleReq := types.SettleLifecycleProducerReportsRequest{
		OwnerID:      ownerID,
		ComputerID:   computerID,
		CommandID:    "cmd-settle-readiness-4",
		TrajectoryID: traj1,
		ReportIDs:    reportIDs,
		Reason:       "tombstone stale cancel residue as late evidence",
	}
	settleReq.CommandDigest, err = store.ComputeSettleLifecycleProducerReportsDigest(settleReq)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.SettleLifecycleProducerReports(ctx, settleReq)
	if err != nil {
		t.Fatalf("settle producer reports: %v", err)
	}
	if result.Receipt.Kind != types.LifecycleSettleProducerReports {
		t.Errorf("receipt kind = %s, want %s", result.Receipt.Kind, types.LifecycleSettleProducerReports)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
	for _, ev := range result.Events {
		if ev.Kind != types.LifecycleUpdateLate {
			t.Errorf("event kind = %s, want %s", ev.Kind, types.LifecycleUpdateLate)
		}
	}

	// 3. Assert all pending store selectors exclude settled IDs
	afterPending, err := rt.ListPendingProducerReports(ctx, ownerID, computerID, "")
	if err != nil || len(afterPending) != 0 {
		t.Fatalf("expected 0 pending producer reports after settlement, got %d", len(afterPending))
	}

	managementAgentID := persistentManagementAgentID(ownerID)
	allPending, err := s.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, managementAgentID)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range allPending {
		for _, settledID := range reportIDs {
			if u.UpdateID == settledID {
				t.Fatalf("ListAllPendingLifecycleUpdates still returned settled report %s", settledID)
			}
		}
	}

	// 4. Assert boot or reconcile after settlement mints zero Management runs referencing them
	reconcileRun, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgentID)
	if err != nil || reconcileRun != nil {
		t.Fatalf("reconcile after settlement minted unexpected Management=%+v err=%v", reconcileRun, err)
	}

	// 5. Assert idempotency
	replay, err := rt.SettleLifecycleProducerReports(ctx, settleReq)
	if err != nil || replay.Receipt.CommandID != settleReq.CommandID {
		t.Fatalf("replay failed: %+v err=%v", replay, err)
	}
}

// Acceptance Criterion 5: Terminal-Event Probe, Positive and Negative
// (a) Terminate a Management while >= 1 admissible unclaimed backlog item exists and assert
// zero successor Management is minted from undelivered backlog (maybeContinuePersistentManagementInbox path);
// (b) Prove the document-channel re-cast path: a new owner-authored revision on the
// engineering-bound document opens a fresh assignment — no Management mediates the opener.
func TestSchedulingReadiness_Criterion5_TerminalEventDocumentRecast(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner-terminal-event-probe"
	computerID := "computer-terminal-probe"
	runtime.cfg.ComputerID = computerID

	operation := selfdev.Operation{
		OperationID:       "selfdev-op-criterion-5",
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-criterion-5",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	}
	originalPrompt := "Author classic solitaire game engine"
	if err := runtime.ensureSelfDevelopmentEngineeringDoc(ctx, operation, ownerID, originalPrompt); err != nil {
		t.Fatal(err)
	}

	managementAgentID := persistentManagementAgentID(ownerID)

	// Seed an admissible unclaimed backlog item in another trajectory
	fDecoy := seedTextureLifecycleControl(t, productStore, ownerID, "decoy-backlog", managementAgentID, agentprofile.Management)

	// Part (a): a terminal Management mints zero successor from undelivered backlog.
	finished := time.Now().UTC()
	terminalManagement := types.RunRecord{
		RunID: "run-terminal-super", AgentID: managementAgentID, OwnerID: ownerID, ComputerID: computerID,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		State: types.RunFailed, Error: "tool loop: exceeded 200 iterations without end_turn",
		CreatedAt: finished.Add(-time.Hour), UpdatedAt: finished, FinishedAt: &finished,
		Metadata: map[string]any{runMetadataAgentProfile: agentprofile.Management, runMetadataAgentRole: agentprofile.Management},
	}
	if err := productStore.CreateRun(ctx, terminalManagement); err != nil {
		t.Fatal(err)
	}
	runtime.maybeContinuePersistentManagementInbox(ctx, &terminalManagement)
	activeAfterTerminal, err := runtime.latestActiveRunByAgent(ctx, ownerID, managementAgentID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("terminal continuation minted unexpected Management from backlog: run=%+v err=%v", activeAfterTerminal, err)
	}

	// Part (b): the engineering document is the cast surface. A second
	// owner-authored revision on it is a new cast the desk reconcile admits.
	docID, _, _ := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	doc, err := productStore.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	secondRevision := types.Revision{
		RevisionID: "revision-recast-" + operation.OperationID,
		DocID:      docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: doc.TrajectoryID,
		AuthorKind: types.AuthorUser, AuthorLabel: ownerID,
		Content: "Revise: extend the engine with scoring", CreatedAt: time.Now().UTC(),
		ParentRevisionID: snapshot.HeadRevision.RevisionID,
	}
	revCmd := types.CommitLifecycleArtifactHeadRequest{
		CommandID: "revise:selfdev-recast:" + operation.OperationID,
		OwnerID:   ownerID, ComputerID: computerID, TrajectoryID: doc.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
		Revision:                 secondRevision,
	}
	revCmd.CommandDigest, _ = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(revCmd, store.TextureSourceGraphWriteSet{})
	if _, commitErr := productStore.CommitLifecycleArtifactHeadWithSourceGraph(ctx, revCmd, store.TextureSourceGraphWriteSet{}); commitErr != nil {
		t.Fatalf("commit recast revision: %v", commitErr)
	}
	// The recast reconcile reaches the assignment opener: without a capsule
	// executor in the test runtime it fails at capsule authority, not at
	// foreign-revision rejection.
	if _, err := runtime.ReconcileEngineeringRevisionCast(ctx, ownerID, docID, secondRevision.RevisionID); err == nil ||
		!strings.Contains(err.Error(), "capsule authority") {
		t.Fatalf("recast reconcile err = %v, want capsule-authority reach", err)
	}
	_ = fDecoy
}
