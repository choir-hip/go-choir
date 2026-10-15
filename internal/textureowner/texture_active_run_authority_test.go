package textureowner

// Regression for the second path in
// docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md
// ("Second path, from the trace and code"): TextureActiveRunByAgent read the
// latest-updated run of the desk (og updated_at DESC) and answered "none" when
// that run was inactive, although agent.ActiveRunID named a pending run the
// activation gate protects. A later write to an older run of the desk hid the
// active run; the locked wake then staled the active run's pending mutation,
// lost its own activation, and recovered the run with a stale_activation row.
//
// Ways the fix (read agent.ActiveRunID first) could fail:
//
//  G1. The wake still trusts the latest-updated run and stales the active
//      run's mutation (the bug). Covered: an older run is written after the
//      active run, then the wake runs.
//  G2. The authority read returns a run the gate does not protect (passivated,
//      terminal, blocked), so the wake stands down with nothing running.
//      Covered for passivated: the older run is passivated and must not be
//      returned; terminal and blocked follow State.Active() minus blocked, as
//      before.
//  G3. The fallback scan is dropped, and agents without ActiveRunID (pre-
//      lifecycle rows) lose their only answer. Kept; not exercised here.
//  G4. The active run passes the authority read but the wake mints a second
//      run anyway. Covered: the wake must return nil, and the active run must
//      still pass ValidateActivationAuthority.
//  G5. DATETIME second precision orders same-second writes arbitrarily, so a
//      test that writes both runs in one second proves nothing. The test
//      spaces the older run's write by more than a second.

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureWakeSeesActiveRunWhenAnOlderRunIsWrittenLater(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	ctx := context.Background()
	ownerID, computerID := "user-alice", "autoputer-test"
	docID, trajID, workID := "doc-hidden-active-1", "traj-hidden-active-1", "work-hidden-active-1"
	agentID := currentTextureAgentID(docID)

	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "cmd-hidden-active-1", TrajectoryID: trajID,
		Kind:        types.TrajectoryKindTask,
		SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID},
		SettlementRule: types.SettlementRule{
			Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"},
		},
		InitialWork: types.WorkItemRecord{
			WorkItemID: workID, Objective: "answer the owner", AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture,
		},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajID, Title: "Hidden active run"},
		InitialRevision: types.Revision{RevisionID: "rev-hidden-active-v0", AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "owner prompt"},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	started, err := handler.Store.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("StartLifecycle: %v", err)
	}
	doc := *started.Document

	project := func(run types.RunRecord, commandID string) {
		t.Helper()
		req := types.ReplaceLifecycleActivationRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: commandID,
			TrajectoryID: trajID, AgentID: agentID, Run: run,
		}
		req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
		if _, err := handler.Store.ReplaceLifecycleActivation(ctx, req); err != nil {
			t.Fatalf("project run %s %s: %v", run.RunID, run.State, err)
		}
	}

	// The previous desk run (985d8cd7 in the trace): activated, then
	// passivated, so it no longer holds the activation.
	now := time.Now().UTC()
	older := types.RunRecord{
		RunID: "run-older-1", AgentID: agentID, ChannelID: docID, TrajectoryID: trajID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPending, Prompt: "previous turn", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"lifecycle_work_item_id": workID, "type": textureAgentRevisionTaskType, "doc_id": docID},
	}
	project(older, "activation:run-older-1")
	older.State, older.UpdatedAt = types.RunPassivated, time.Now().UTC()
	older.Metadata = map[string]any{"lifecycle_work_item_id": workID, "type": textureAgentRevisionTaskType, "doc_id": docID, "passivated_reason": "idle"}
	project(older, "passivate:run-older-1")

	// The successor (f80d052d): a real submit, pending mutation, holds the
	// activation.
	active, err := handler.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{Intent: "initial_owner_work"}, 0)
	if err != nil {
		t.Fatalf("submit active run: %v", err)
	}
	if err := handler.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, active.RunID); err != nil {
		t.Fatalf("active run authority before the older run's write: %v", err)
	}

	// G5: the older run is written after the active run, in a later second.
	time.Sleep(1100 * time.Millisecond)
	older.UpdatedAt = time.Now().UTC()
	older.Metadata = map[string]any{"lifecycle_work_item_id": workID, "type": textureAgentRevisionTaskType, "doc_id": docID, "passivated_reason": "idle", "late_bookkeeping": true}
	project(older, "passivate-late:run-older-1")

	// G1/G2: the desk's active run is the one the gate protects.
	got, found, err := core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentID)
	if err != nil || !found || got.RunID != active.RunID {
		t.Errorf("TextureActiveRunByAgent = %q found=%v err=%v, want active run %s", got.RunID, found, err, active.RunID)
	}

	// G4: the wake stands down, and the active run stays dispatchable.
	wakeRun, err := handler.ReconcileAgentWake(ctx, ownerID, docID)
	if err != nil {
		t.Fatalf("wake: %v", err)
	}
	if wakeRun != nil && wakeRun.RunID != active.RunID {
		t.Fatalf("wake minted run %s beside active run %s", wakeRun.RunID, active.RunID)
	}
	if err := handler.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, active.RunID); err != nil {
		t.Fatalf("active run authority after the wake: %v", err)
	}
}
