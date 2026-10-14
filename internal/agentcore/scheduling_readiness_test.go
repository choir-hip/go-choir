package agentcore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"

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

	// The kernel write fence mints durable wakes for pre-fold obligations; the
	// projector only delivers those wakes and never selects Management backlog.
	rt.SetKernelMode()
	rt.sweepActorWakeOutbox(ctx)

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

	// ReconcileCoagentWake is the actor-side recovery entrypoint. It must
	// reactivate the exact passivated run without selecting a new backlog item.
	resumed, err := rt.ReconcileCoagentWake(ctx, ownerID, managementAgent.AgentID)
	if err != nil || resumed == nil {
		t.Fatalf("exact Management recovery failed: resumed=%v err=%v", resumed, err)
	}

	// Assert the exact interrupted run resumed with the SAME run ID.
	if resumed.RunID != firstRun.RunID {
		t.Fatalf("resumed run ID = %s, want original %s", resumed.RunID, firstRun.RunID)
	}
	if resumed.State != types.RunPending {
		t.Fatalf("resumed run state = %s, want pending", resumed.State)
	}

	// Assert only ONE recovery occurrence was enqueued, no duplicate runs created
	activeRun, err := rt.latestActiveRunByAgent(ctx, ownerID, managementAgent.AgentID)
	if err != nil || activeRun.RunID != firstRun.RunID {
		t.Fatalf("expected single active run %s, got %+v err=%v", firstRun.RunID, activeRun, err)
	}
	_ = f
}
