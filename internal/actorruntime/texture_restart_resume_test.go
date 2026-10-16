package actorruntime

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A planned restart parks the Texture turn in flight and marks its mutation
// stale_activation. The turn's initial_dispatch, still queued, is the one
// resume a planned restart allows. The generic reactivation set the run
// running without re-arming the mutation, so Texture's activation check
// refused it eight times, the dispatch was dead-lettered and the run was
// left "running" with nobody running it
// (docs/problems/planned-restart-drops-texture-dispatch-mutation-mismatch-2026-10-16.md).
//
// Ways this could fail, written first:
//  1. the dispatch is refused for mutation authority (the bug);
//  2. the run resumes, but the mutation stays stale, so the turn's commit
//     has no authority;
//  3. when the document moved on while the run was parked, the run is set
//     running anyway and left so after the refusal;
//  4. that refusal is an error, so the dispatcher retries it to the poison
//     cap instead of consuming it once;
//  5. a fresh turn the controller starts instead is run here and again by
//     its own dispatch.

const restartRunID = "run-texture-restart"

// parkTextureRunAtRestart seeds the run a planned restart passivated, with
// the stale mutation boot passivation leaves.
func parkTextureRunAtRestart(t *testing.T, f *recordsWakeFixture, mutationRevision string) {
	t.Helper()
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: restartRunID, AgentID: f.agentID, OwnerID: f.ownerID, ComputerID: f.computerID, ChannelID: f.id,
		TrajectoryID: f.trajectoryID, State: types.RunPassivated, Prompt: "Texture turn", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"type": "texture_agent_revision", "doc_id": f.id, "current_revision_id": mutationRevision,
			"agent_profile": "texture", "agent_role": "texture", "trajectory_id": f.trajectoryID, "lifecycle_work_item_id": "work:" + f.id,
			"passivated_reason": "runtime_restarted"},
	}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: f.ownerID, ComputerID: f.computerID, CommandID: "project-restart-parked", TrajectoryID: f.trajectoryID, AgentID: f.agentID, Run: run}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := f.env.store.ReplaceLifecycleActivation(f.env.ctx, project); err != nil {
		t.Fatalf("project restart-parked run: %v", err)
	}
	if err := f.env.store.CreateAgentMutation(f.env.ctx, store.AgentMutation{DocID: f.id, RunID: restartRunID, OwnerID: f.ownerID, ComputerID: f.computerID, State: "stale_activation", RevisionID: mutationRevision, CreatedAt: now}); err != nil {
		t.Fatalf("seed stale mutation: %v", err)
	}
}

// Failure modes 1 and 2.
func TestPlannedRestartResumesTheParkedTextureTurn(t *testing.T) {
	f := newRecordsWakeFixture(t, "doc-restart-resume", false, false)
	parkTextureRunAtRestart(t, f, "revision:"+f.id)
	handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
	handler.plannedBoot = true
	if err := f.wake(t, handler, "initial_dispatch", restartRunID); err != nil {
		t.Fatalf("the queued initial_dispatch of a restart-parked Texture turn: %v", err)
	}
	after, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, restartRunID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State == types.RunPassivated || after.State == types.RunPending || after.State == types.RunRunning {
		t.Fatalf("the restart-parked turn was not resumed: %s %+v", after.State, after)
	}
	if reactivated, _ := after.Metadata["actor_reactivated_from_passivated"].(bool); !reactivated {
		t.Fatalf("the turn ran without being resumed from its parked state: %+v", after.Metadata)
	}
}

// Failure modes 3, 4 and 5.
func TestPlannedRestartConsumesAParkedTextureTurnTheDocumentOutgrew(t *testing.T) {
	f := newRecordsWakeFixture(t, "doc-restart-outgrown", false, false)
	parkTextureRunAtRestart(t, f, "revision:older")
	handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
	handler.plannedBoot = true
	if err := f.wake(t, handler, "initial_dispatch", restartRunID); err != nil {
		t.Fatalf("a parked turn the document outgrew is retried instead of consumed: %v", err)
	}
	after, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, restartRunID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State == types.RunRunning || after.State == types.RunPending {
		t.Fatalf("a parked turn the document outgrew was left %s: %+v", after.State, after)
	}
	runs, err := f.env.store.ListLifecycleRunsByOwner(f.env.ctx, f.ownerID, f.computerID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		// The fixture's dispatcher has no Texture owner, so a fresh run's
		// own dispatch defers and leaves it pending.
		if r.AgentID == f.agentID && r.RunID != restartRunID && r.State != types.RunPending {
			t.Fatalf("the restart dispatch ran a fresh turn its own dispatch runs: %+v", r)
		}
	}
}
