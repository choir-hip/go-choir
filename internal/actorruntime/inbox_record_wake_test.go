package actorruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// recordsWakeFixture is a lifecycle Texture with one unread research Report
// in texture:<doc>, optionally a parked run whose last turn left the head
// unchanged, and optionally its owner work item already closed.
type recordsWakeFixture struct {
	env                     *adapterTestEnv
	ownerID, computerID, id string
	agentID, trajectoryID   string
	dispatched              []string
}

func newRecordsWakeFixture(t *testing.T, docID string, parked, workClosed bool) *recordsWakeFixture {
	t.Helper()
	f := &recordsWakeFixture{env: newAdapterTestEnv(t), ownerID: "owner-" + docID, computerID: "autoputer-test", id: docID, agentID: "texture:" + docID, trajectoryID: "trajectory:" + docID}
	f.env.adapter.Runtime.SetDispatchActor(func(_ context.Context, _, _, to, kind, content, _, _ string) error {
		f.dispatched = append(f.dispatched, kind+"->"+to+"|"+content)
		return nil
	})
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: f.ownerID, ComputerID: f.computerID, CommandID: "start:" + docID, TrajectoryID: f.trajectoryID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + docID, Objective: "answer the owner", AssignedAgentID: f.agentID},
		InitialDocument: types.Document{DocID: docID, Title: "Records wake"},
		// The head is Texture's own revision, as after a committed turn,
		// so only the records are pressure.
		InitialRevision: types.Revision{RevisionID: "revision:" + docID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture", Content: "Texture's answer", BodyDoc: json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"` + docID + `"},"content":[{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"Texture's answer"}]}]}}`)},
		Agent:           types.AgentRecord{AgentID: f.agentID, OwnerID: f.ownerID, ComputerID: f.computerID, Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := f.env.store.StartLifecycle(f.env.ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	if workClosed {
		refuse := types.RefuseLifecycleWorkRequest{
			OwnerID: f.ownerID, ComputerID: f.computerID, CommandID: "close-owner-work", TrajectoryID: f.trajectoryID,
			WorkItemID: "work:" + docID, ActingAgentID: f.agentID, RefusalRef: "refusal://test/closed", Reason: "the owner's request was answered",
		}
		refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
		if _, err := f.env.store.RefuseLifecycleWork(f.env.ctx, refuse); err != nil {
			t.Fatalf("close owner work: %v", err)
		}
	}
	if parked {
		run := types.RunRecord{
			RunID: "run-texture-parked", AgentID: f.agentID, OwnerID: f.ownerID, ComputerID: f.computerID, ChannelID: docID,
			TrajectoryID: f.trajectoryID, State: types.RunPassivated, Prompt: "Texture turn", AgentProfile: "texture", AgentRole: "texture",
			CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"type": "texture_agent_revision", "doc_id": docID, "current_revision_id": "revision:" + docID,
				"agent_profile": "texture", "agent_role": "texture", "trajectory_id": f.trajectoryID, "lifecycle_work_item_id": "work:" + docID,
				"passivated_reason": "durable_transition_success"},
		}
		project := types.ReplaceLifecycleActivationRequest{OwnerID: f.ownerID, ComputerID: f.computerID, CommandID: "project-parked", TrajectoryID: f.trajectoryID, AgentID: f.agentID, Run: run}
		project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
		if _, err := f.env.store.ReplaceLifecycleActivation(f.env.ctx, project); err != nil {
			t.Fatalf("project parked run: %v", err)
		}
		if err := f.env.store.CreateAgentMutation(f.env.ctx, store.AgentMutation{DocID: docID, RunID: run.RunID, OwnerID: f.ownerID, ComputerID: f.computerID, State: "sleeping", RevisionID: "revision:" + docID, CreatedAt: now}); err != nil {
			t.Fatalf("seed sleeping mutation: %v", err)
		}
	}
	req := types.CommitInboxRecordsRequest{OwnerID: f.ownerID, ComputerID: f.computerID, CommandID: "report-to-texture", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "report-1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "findings"},
		Inbox:     f.agentID, SenderInbox: "research:" + docID, Addressee: f.agentID, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: "research:" + docID, CommittedAt: now.Format(time.RFC3339Nano)},
	}}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := f.env.store.CommitInboxRecords(f.env.ctx, req); err != nil {
		t.Fatalf("commit record: %v", err)
	}
	f.dispatched = nil
	return f
}

func (f *recordsWakeFixture) wake(t *testing.T, handler *actorHandler, kind, content string) error {
	t.Helper()
	_, err := handler.HandleUpdate(f.env.ctx, f.agentID, actor.Update{
		UpdateID: kind + "-" + f.id + "-" + time.Now().Format("150405.000000000"), ToAgentID: scopedActorMailboxID(f.ownerID, f.computerID, f.agentID),
		FromAgentID: "research:" + f.id, Kind: kind, Content: content, TrajectoryID: f.trajectoryID, CreatedAt: time.Now().UTC().Add(time.Second),
	}, nil)
	return err
}

// Cut 1 breakage B4 (docs/archive/problems/reeng-cut1-breakage-2026-10-13.md): a
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
	f := newRecordsWakeFixture(t, "doc-records-wake", true, false)
	handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
	if err := f.wake(t, handler, "inbox_record", ""); err != nil {
		t.Fatalf("records wake: %v", err)
	}
	after, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, "run-texture-parked")
	if err != nil {
		t.Fatal(err)
	}
	if after.State == types.RunPending || after.State == types.RunPassivated {
		t.Fatalf("the records wake left the parked run %s with nobody to run it: %+v", after.State, after)
	}
	for _, d := range f.dispatched {
		if strings.HasPrefix(d, "initial_dispatch->"+f.agentID) {
			t.Fatalf("the wake also dispatched a fresh Texture run: %v", f.dispatched)
		}
	}
}

// Cut 1 breakage B5: once the owner's work item has closed, a records wake
// still runs Texture, whether it reactivates the parked run or starts a
// fresh one; the records are the work.
//
// Ways this could fail, written first: the parked run carries the closed
// work item and fails activation authority; a fresh run has no work item
// and its initial dispatch fails authority ("record not found"); the
// mutation of a workless run is scoped to no computer, so the parked run is
// never found to reactivate and a second run is minted beside it.
func TestTextureRecordsWakeRunsAfterTheOwnerWorkCloses(t *testing.T) {
	t.Run("parked", func(t *testing.T) {
		f := newRecordsWakeFixture(t, "doc-records-closed-parked", true, true)
		handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
		if err := f.wake(t, handler, "inbox_record", ""); err != nil {
			t.Fatalf("records wake: %v", err)
		}
		after, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, "run-texture-parked")
		if err != nil {
			t.Fatal(err)
		}
		if after.State == types.RunPending || after.State == types.RunPassivated {
			t.Fatalf("parked run after the wake: %s, want it run: %+v", after.State, after)
		}
		if _, ok := after.Metadata["lifecycle_work_item_id"]; ok {
			t.Fatalf("the reactivated run still carries the closed work item: %+v", after.Metadata)
		}
	})
	t.Run("fresh", func(t *testing.T) {
		f := newRecordsWakeFixture(t, "doc-records-closed-fresh", false, true)
		handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
		if err := f.wake(t, handler, "inbox_record", ""); err != nil {
			t.Fatalf("records wake: %v", err)
		}
		var runID string
		runs, err := f.env.store.ListLifecycleRunsByOwner(f.env.ctx, f.ownerID, f.computerID, 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			if r.AgentID == f.agentID {
				runID = r.RunID
			}
		}
		if runID == "" {
			t.Fatalf("the records wake started no Texture run (dispatched %v)", f.dispatched)
		}
		if err := handler.textureOwner.ValidateActivationAuthority(f.env.ctx, f.ownerID, f.computerID, f.agentID, runID); err != nil {
			t.Fatalf("a records-started run on a Texture with no open work fails activation authority: %v", err)
		}
	})
}

// Cut 1 breakage B8: a wake reactivated the parked run to pending, then its
// execution deferred, so the occurrence is retried. The retry finds the run
// pending and reactivated with nobody running it, and runs it.
//
// Ways this could fail, written first: the retry sees an active run with
// valid authority and acknowledges without running it (B4 again); the
// retry runs a fresh pending run that its own initial dispatch also runs.
func TestTextureRecordsWakeRetryRunsAStrandedReactivatedRun(t *testing.T) {
	f := newRecordsWakeFixture(t, "doc-records-stranded", true, false)
	run, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, "run-texture-parked")
	if err != nil {
		t.Fatal(err)
	}
	// The first wake's reactivation, with its execution never reached.
	if err := f.env.store.ReactivateAgentMutation(f.env.ctx, f.ownerID, f.computerID, run.RunID, 0); err != nil {
		t.Fatalf("reactivate mutation: %v", err)
	}
	run.State = types.RunPending
	run.Metadata["actor_reactivated_from_passivated"] = true
	run.Metadata["actor_reactivate_existing_memory"] = true
	run.UpdatedAt = time.Now().UTC()
	if err := f.env.store.UpdateRun(f.env.ctx, run); err != nil {
		t.Fatalf("project the stranded run: %v", err)
	}
	handler := newActorHandler(f.env.adapter.Runtime, textureowner.NewHandler(f.env.adapter.Runtime))
	if err := f.wake(t, handler, "inbox_record", ""); err != nil {
		t.Fatalf("retried records wake: %v", err)
	}
	after, err := f.env.store.GetLifecycleRun(f.env.ctx, f.ownerID, f.computerID, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State == types.RunPending {
		t.Fatalf("the retried wake left the stranded run pending: %+v", after)
	}
}
