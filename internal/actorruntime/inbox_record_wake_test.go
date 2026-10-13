package actorruntime

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cut 1 breakage B4 (docs/problems/reeng-cut1-breakage-2026-10-13.md): a
// record to texture:<doc> arrives after a Texture turn that left the head
// unchanged (a decide turn) and parked. The wake reactivates that parked
// run, so the inbox_record occurrence must execute it, as the coagent path
// does.
//
// Ways this could fail, written first: the reactivated run stays pending
// and nothing runs it (the bug); the wake executes a run it did not
// reactivate (an active one, or a fresh one that its own dispatch runs),
// so a turn runs twice; the run executes but the record stays unread
// because the wake reads the wrong inbox.
func TestTextureRecordsWakeRunsTheParkedRunItReactivates(t *testing.T) {
	env := newAdapterTestEnv(t)
	var dispatched []string
	env.adapter.Runtime.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		dispatched = append(dispatched, kind+"->"+to)
		return nil
	})
	const ownerID, computerID, docID = "owner-records-wake", "autoputer-test", "doc-records-wake"
	textureAgentID := "texture:" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:" + docID, TrajectoryID: "trajectory:" + docID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + docID, Objective: "answer the owner", AssignedAgentID: textureAgentID},
		InitialDocument: types.Document{DocID: docID, Title: "Records wake"},
		InitialRevision: types.Revision{RevisionID: "revision:" + docID, AuthorKind: types.AuthorUser, AuthorLabel: "user", Content: "Owner request"},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := env.store.StartLifecycle(env.ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	// The parked run: its last turn was a decide, so its revision is still
	// the head and its mutation sleeps.
	parked := types.RunRecord{
		RunID: "run-texture-parked", AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, ChannelID: docID,
		TrajectoryID: start.TrajectoryID, State: types.RunPassivated, Prompt: "Texture turn", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"type": "texture_agent_revision", "doc_id": docID, "current_revision_id": "revision:" + docID,
			"agent_profile": "texture", "agent_role": "texture", "trajectory_id": start.TrajectoryID, "lifecycle_work_item_id": "work:" + docID,
			"passivated_reason": "durable_transition_success"},
	}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "project-parked", TrajectoryID: start.TrajectoryID, AgentID: textureAgentID, Run: parked}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := env.store.ReplaceLifecycleActivation(env.ctx, project); err != nil {
		t.Fatalf("project parked run: %v", err)
	}
	if err := env.store.CreateAgentMutation(env.ctx, store.AgentMutation{DocID: docID, RunID: parked.RunID, OwnerID: ownerID, ComputerID: computerID, State: "sleeping", RevisionID: "revision:" + docID, CreatedAt: now}); err != nil {
		t.Fatalf("seed sleeping mutation: %v", err)
	}
	// Research's final Report arrives in Texture's inbox.
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "report-to-texture", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "report-1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "findings"},
		Inbox: textureAgentID, SenderInbox: "research:" + docID, Addressee: textureAgentID, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: "research:" + docID, CommittedAt: now.Format(time.RFC3339Nano)},
	}}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := env.store.CommitInboxRecords(env.ctx, req); err != nil {
		t.Fatalf("commit record: %v", err)
	}
	dispatched = nil

	handler := newActorHandler(env.adapter.Runtime, textureowner.NewHandler(env.adapter.Runtime))
	if _, err := handler.HandleUpdate(env.ctx, textureAgentID, actor.Update{
		UpdateID: "inbox-record-wake", ToAgentID: scopedActorMailboxID(ownerID, computerID, textureAgentID),
		FromAgentID: "research:" + docID, Kind: "inbox_record", TrajectoryID: start.TrajectoryID, CreatedAt: time.Now().UTC().Add(time.Second),
	}, nil); err != nil {
		t.Fatalf("records wake: %v", err)
	}
	after, err := env.store.GetLifecycleRun(env.ctx, ownerID, computerID, parked.RunID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parked run after the wake: state=%s error=%q", after.State, after.Error)
	if after.State == types.RunPending {
		t.Fatalf("the records wake reactivated the parked run and left it pending with nobody to run it: %+v", after)
	}
	if after.State == types.RunPassivated && after.UpdatedAt.Equal(parked.UpdatedAt) {
		t.Fatalf("the records wake did not reactivate the parked run")
	}
	for _, d := range dispatched {
		if d == "initial_dispatch->"+textureAgentID {
			t.Fatalf("the wake also dispatched a fresh Texture run: %v", dispatched)
		}
	}
}
