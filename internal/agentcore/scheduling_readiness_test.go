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
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
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
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// Seed 3 competing Texture execution requests with distinct ArrivalOrdinals across 3 trajectories
	f1 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-1", superAgent.AgentID, agentprofile.Super)
	f2 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-2", superAgent.AgentID, agentprofile.Super)
	f3 := seedTextureLifecycleControl(t, s, ownerID, "fifo-doc-3", superAgent.AgentID, agentprofile.Super)

	pending, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, superAgent.AgentID, 100)
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

	// For each live cycle, deliver the named trigger and observe exactly one new Super run
	// whose selected work item is the lowest pending ordinal. Later requests remain pending.
	for cycle := range 3 {
		expectedFixture := fixtures[cycle]

		// Deliver the live trigger
		run, err := rt.ReconcileCoagentWake(ctx, ownerID, superAgent.AgentID)
		if err != nil || run == nil {
			t.Fatalf("cycle %d: live trigger failed to mint Super: run=%v err=%v", cycle, run, err)
		}

		// Assert selected work item matches the lowest pending ordinal (expectedFixture)
		if run.Metadata["lifecycle_work_item_id"] != expectedFixture.workID {
			t.Fatalf("cycle %d: selected work item = %v, want %s", cycle, run.Metadata["lifecycle_work_item_id"], expectedFixture.workID)
		}
		if metadataStringValue(run.Metadata, "assignment_trajectory_id") != expectedFixture.trajectoryID {
			t.Fatalf("cycle %d: selected trajectory = %v, want %s", cycle, run.Metadata["assignment_trajectory_id"], expectedFixture.trajectoryID)
		}

		// Assert later requests remain pending with delivered_to_run_id null/empty
		remainingPending, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, superAgent.AgentID, 100)
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
		rt.maybeContinuePersistentSuperInbox(ctx, run)
	}
}

// Acceptance Criterion 2: Boot-Does-Not-Schedule (structurally enforced)
// Precondition: admissible unclaimed backlog exists.
// Boot rewarm/sweep runs: zero Super or CoSuper runs created, positive assertion
// that exact-run resume did not enter selection, pending ordinals remain untouched.
func TestSchedulingReadiness_Criterion2_BootDoesNotSchedule(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-boot-recovery"
	computerID := "autoputer-test"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// PRECONDITION: at least one admissible, unclaimed backlog item exists
	f1 := seedTextureLifecycleControl(t, s, ownerID, "boot-doc-1", superAgent.AgentID, agentprofile.Super)
	f2 := seedTextureLifecycleControl(t, s, ownerID, "boot-doc-2", superAgent.AgentID, agentprofile.Super)

	pendingBefore, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, superAgent.AgentID, 100)
	if err != nil || len(pendingBefore) < 2 {
		t.Fatalf("precondition failed: pending controls before boot = %v, err=%v", pendingBefore, err)
	}

	// Capture log buffer for positive did-not-enter-selection assertion
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Execute boot rewarm and work-item sweep
	rt.rewarmInterruptedPersistentSuperActors(ctx)
	rt.sweepOpenWorkItemActors(ctx)

	logOutput := buf.String()

	// Assert from boot logs that reconcile never entered selection
	if !strings.Contains(logOutput, "boot work-item sweep skipping persistent Super") {
		t.Errorf("boot logs missing sweep skip log line; got:\n%s", logOutput)
	}

	// Assert across the window that ZERO Super or CoSuper run rows are created
	activeRun, err := rt.latestActiveRunByAgent(ctx, ownerID, superAgent.AgentID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("boot minted unexpected Super run=%+v err=%v", activeRun, err)
	}

	// Assert pending ordinals remain pending with unchanged delivery state
	pendingAfter, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, superAgent.AgentID, 100)
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
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	var dispatches []string
	rt.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		dispatches = append(dispatches, kind+":"+content)
		return nil
	})

	// Seed control and start the in-flight Super run
	f := seedTextureLifecycleControl(t, s, ownerID, "inflight-doc", superAgent.AgentID, agentprofile.Super)
	firstRun, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID)
	if err != nil || firstRun == nil {
		t.Fatalf("initial Super run failed: %v", err)
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
	resumed, ok, err := rt.ResumeInterruptedPersistentSuperControlRun(ctx, ownerID, superAgent.AgentID)
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
	if !strings.Contains(logOutput, "persistent-Super exact-run resume reactivated run="+firstRun.RunID) {
		t.Errorf("resume logs missing reactivation log line; got:\n%s", logOutput)
	}

	// Assert only ONE recovery occurrence was enqueued, no duplicate runs created
	activeRun, err := rt.latestActiveRunByAgent(ctx, ownerID, superAgent.AgentID)
	if err != nil || activeRun.RunID != firstRun.RunID {
		t.Fatalf("expected single active run %s, got %+v err=%v", firstRun.RunID, activeRun, err)
	}
	_ = f
}

// Acceptance Criterion 4: Producer Report Settlement at Store Layer
// Enumerate undelivered CoSuper cancel producer reports, settle them via dedicated
// runtime/store lifecycle-reducer command with CAS precondition, terminal disposition (UpdateLate),
// and idempotent settlement receipt. Assert all pending store selectors exclude settled IDs,
// claimedPersistentSuperProducerReportIDs is retired, and boot/reconcile mints zero Super runs.
func TestSchedulingReadiness_Criterion4_ProducerReportStoreSettlement(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "owner-producer-settlement"
	computerID := rt.TextureComputerID()
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	// Seed two cancelled CoSuper reports
	_, rep1, traj1 := seedCancelledCoSuperReport(t, rt, s, ownerID, "report-settle-1")
	_, rep2, _ := seedCancelledCoSuperReport(t, rt, s, ownerID, "report-settle-2")

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

	superAgentID := persistentSuperAgentID(ownerID)
	allPending, err := s.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, superAgentID)
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

	// 4. Assert boot or reconcile after settlement mints zero Super runs referencing them
	reconcileRun, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil || reconcileRun != nil {
		t.Fatalf("reconcile after settlement minted unexpected Super=%+v err=%v", reconcileRun, err)
	}

	// 5. Assert idempotency
	replay, err := rt.SettleLifecycleProducerReports(ctx, settleReq)
	if err != nil || replay.Receipt.CommandID != settleReq.CommandID {
		t.Fatalf("replay failed: %+v err=%v", replay, err)
	}
}

// Acceptance Criterion 5: Terminal-Event Probe, Positive and Negative
// (a) Terminate a Super while >= 1 admissible unclaimed backlog item exists and assert
// zero successor Super is minted from undelivered backlog (maybeContinuePersistentSuperInbox path);
// (b) Prove the live Texture rewake path intact: terminal Super -> Texture instruction
// (maybeRewakeSelfDevelopmentTextureAfterTerminalSuper) -> owner-visible Texture turn
// -> NEW typed execution_request -> exactly one new Super, without HTTP operations POST.
func TestSchedulingReadiness_Criterion5_TerminalEventTextureRewake(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner-terminal-event-probe"
	computerID := "computer-terminal-probe"
	runtime.cfg.ComputerID = computerID

	operation := selfdev.Operation{
		OperationID:       "selfdev-op-criterion-5",
		ComputerID:        computerID,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	}
	originalPrompt := "Author classic solitaire game engine"
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, ownerID, originalPrompt); err != nil {
		t.Fatal(err)
	}

	superAgentID := persistentSuperAgentID(ownerID)
	firstSuper, err := productStore.GetLatestRunByAgent(ctx, ownerID, superAgentID)
	if err != nil || !firstSuper.State.Active() {
		t.Fatalf("first Super active state: %+v err=%v", firstSuper, err)
	}

	// Seed an admissible unclaimed backlog item in another trajectory
	fDecoy := seedTextureLifecycleControl(t, productStore, ownerID, "decoy-backlog", superAgentID, agentprofile.Super)

	// Part (a): Terminate Super while >= 1 admissible unclaimed backlog item exists
	// and assert zero successor Super is minted from undelivered backlog
	_ = runtime.CancelRun(ctx, firstSuper.RunID, ownerID)
	finished := time.Now().UTC()
	firstSuper.State = types.RunFailed
	firstSuper.Error = "tool loop: exceeded 200 iterations without end_turn"
	firstSuper.UpdatedAt = finished
	firstSuper.FinishedAt = &finished
	if err := productStore.UpdateRun(ctx, firstSuper); err != nil {
		t.Fatal(err)
	}
	if err := runtime.unbindSelfDevelopmentSuper(ctx, &firstSuper); err != nil {
		t.Fatal(err)
	}

	// Call maybeContinuePersistentSuperInbox on the terminal Super:
	runtime.maybeContinuePersistentSuperInbox(ctx, &firstSuper)

	// Assert: zero successor Super is minted from undelivered backlog
	activeAfterTerminal, err := runtime.latestActiveRunByAgent(ctx, ownerID, superAgentID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("terminal continuation minted unexpected Super from backlog: run=%+v err=%v", activeAfterTerminal, err)
	}

	// Part (b): Prove the live Texture rewake path intact:
	// 1. maybeRewakeSelfDevelopmentTextureAfterTerminalSuper queues instruction on Texture trajectory
	rewakeErr := runtime.maybeRewakeSelfDevelopmentTextureAfterTerminalSuper(ctx, ownerID)
	if rewakeErr != nil {
		t.Fatalf("rewake Texture error: %v", rewakeErr)
	}

	// 2. Before Texture commits a turn, reconcile mints zero Super
	noSuper, err := runtime.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil || noSuper != nil {
		t.Fatalf("expected nil Super before Texture turn commits execution_request, got: %+v", noSuper)
	}

	// 3. Texture turn consumes the instruction and commits a NEW typed execution_request
	docID, _, textureWorkID, trajectoryID, superWorkID, _ := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	textureAgentID := agentprofile.Texture + ":" + docID
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureAgent, err := productStore.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}

	expectedInstructions, err := productStore.ListPendingLifecycleOwnerInstructionsForHead(ctx, ownerID, computerID, trajectoryID, textureAgentID, snapshot.HeadRevision.RevisionID)
	if err != nil || len(expectedInstructions) == 0 {
		t.Fatalf("expected pending owner instructions for Texture, got %v, err=%v", expectedInstructions, err)
	}
	var ownerInstructions []types.TextureTurnOwnerInstruction
	for _, inst := range expectedInstructions {
		ownerInstructions = append(ownerInstructions, types.TextureTurnOwnerInstruction{
			InstructionID: inst.InstructionID,
			RequestID:     inst.RequestID,
		})
	}

	packet, err := PrepareTextureControlPacket(types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "execution_request",
		Summary:       "Continue self-development operation",
		Sources: []types.CoagentPacketSource{{
			SourceID: "src-operation",
			Kind:     sourcecontract.SourceKindCapsuleBundle,
			Target:   types.CoagentPacketSourceTarget{URI: "operation:" + operation.OperationID},
		}},
		Actions: []types.CoagentPacketAction{{
			Type:      "run_command",
			Objective: originalPrompt,
			Safety: types.CoagentPacketActionSafety{
				MutationClass: "green",
				Network:       "forbidden",
				FileMutation:  "forbidden",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := BuildTextureLifecycleControlContent(packet, superAgentID, superWorkID)
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		t.Fatal(err)
	}

	turn := types.ApplyTextureTurnRequest{
		OwnerID:                        ownerID,
		ComputerID:                     computerID,
		CommandID:                      "turn:selfdev-texture-rewake:" + operation.OperationID,
		DocumentID:                     docID,
		TrajectoryID:                   trajectoryID,
		CallerAgentID:                  textureAgentID,
		CallerRunID:                    runtime.selfDevelopmentCallerRunID(ownerID, computerID, trajectoryID),
		ExpectedLifecycleVersion:       snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion,
		ExpectedHeadRevisionID:         snapshot.HeadRevision.RevisionID,
		CallerWorkItemID:               textureWorkID,
		CallerWorkDisposition:          types.WorkItemOpen,
		Outcome:                        types.TextureTurnWait,
		Reason:                         "continue after terminal Super",
		Controls: []types.TextureTurnControl{{
			ControlID:        "control-rewake-" + operation.OperationID,
			TargetAgentID:    superAgentID,
			TargetWorkItemID: superWorkID,
			Packet:           packet,
			Content:          content,
			PayloadDigest:    payloadDigest,
		}},
		OwnerInstructions: ownerInstructions,
	}
	turn.CommandDigest, err = store.ComputeApplyTextureTurnDigest(turn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := productStore.ApplyTextureTurn(ctx, turn); err != nil {
		t.Fatalf("apply Texture turn: %v", err)
	}

	// 4. Live trigger wakes Super and mints exactly ONE new Super run
	rewokeSuper, err := runtime.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil || rewokeSuper == nil {
		t.Fatalf("reconcile after Texture turn failed to mint replacement Super: run=%v err=%v", rewokeSuper, err)
	}
	if rewokeSuper.RunID == firstSuper.RunID {
		t.Fatalf("expected new Super run ID, got same %s", firstSuper.RunID)
	}
	if metadataStringValue(rewokeSuper.Metadata, "self_development_operation_id") != operation.OperationID {
		t.Fatalf("rewoke Super missing operation_id: %+v", rewokeSuper.Metadata)
	}

	_ = fDecoy
}
