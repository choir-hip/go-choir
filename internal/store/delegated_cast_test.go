package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// delegatedCastOpenRequest builds an OpenEngineeringAssignmentRequest under the
// delegated-cast authority (mission R2): the caster's run/work is the parent
// and the cast's commitment record (controlID) is the parent control.
func delegatedCastOpenRequest(f engineeringAssignmentStoreFixture, index int, assignmentID, controlID string) types.OpenEngineeringAssignmentRequest {
	binding := types.EngineeringAssignmentBinding{
		OwnerID: f.ownerID, ComputerID: f.computerID, TrajectoryID: f.trajectoryID,
		ParentAgentID: f.parentAgentID, ParentRunID: f.parentRunID,
		ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("delegated-decision:"+assignmentID)),
		ParentControlID:  controlID,
		ParentWorkItemID: f.parentWorkID, AssignedWorkItemID: f.assignedWorkIDs[index], AssignedAgentID: f.assignedAgentIDs[index],
		Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
		ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
		CapabilityDigest: DigestEngineeringOpaqueCapability("cap-" + assignmentID), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-" + assignmentID)),
		SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
		SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
		Writable:          true, CapsuleID: "capsule-" + assignmentID,
		NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
		FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		CastAuthority:  types.EngineeringCastAuthorityDelegated,
	}
	req := types.OpenEngineeringAssignmentRequest{
		CommandID: "delegated-open-" + assignmentID, AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork: types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID,
			Objective: "delegated cast objective"},
	}
	req.CommandDigest, _ = ComputeOpenEngineeringAssignmentDigest(req)
	return req
}

// TestDelegatedCastOpensAssignmentUnderCasterAuthority proves the R2 delegated
// admission: a desk-authored cast opens an engineering assignment whose parent
// is the caster's own live run + work item and whose parent control is the
// cast's commitment record — no owner revision.
func TestDelegatedCastOpensAssignmentUnderCasterAuthority(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	// The cast mints its commitment record first; its canonical id is the
	// delegated parent control.
	controlID, err := s.AppendCommitmentRecord(ctx, f.ownerID, f.computerID, types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-cell:cast:m0",
		Provenance: types.CommitmentProvenance{AgentID: f.parentAgentID},
	})
	if err != nil {
		t.Fatal(err)
	}
	open := delegatedCastOpenRequest(f, 0, "delegated-assignment-1", controlID)
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatalf("delegated cast assignment rejected: %v", err)
	}
	if opened.Assignment.Binding.CastAuthority != types.EngineeringCastAuthorityDelegated {
		t.Fatalf("cast_authority = %q, want delegated", opened.Assignment.Binding.CastAuthority)
	}
	if opened.Assignment.Disposition != types.EngineeringAssignmentOpen {
		t.Fatalf("disposition = %q, want open", opened.Assignment.Disposition)
	}
}

// TestDelegatedCastRejectsForeignControl proves the delegated control must be
// authored by the caster: a commitment record authored by a different agent is
// refused.
func TestDelegatedCastRejectsForeignControl(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	controlID, err := s.AppendCommitmentRecord(ctx, f.ownerID, f.computerID, types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-cell:cast:foreign",
		Provenance: types.CommitmentProvenance{AgentID: "engineering:not-the-caster"},
	})
	if err != nil {
		t.Fatal(err)
	}
	open := delegatedCastOpenRequest(f, 0, "delegated-assignment-foreign", controlID)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err == nil {
		t.Fatal("delegated cast admitted with a control authored by another agent")
	} else if !strings.Contains(err.Error(), "delegated cast") {
		t.Fatalf("wrong rejection reason: %v", err)
	}
}

// TestDelegatedCastRejectsMissingControl proves a cast whose commitment record
// was never minted cannot open an assignment.
func TestDelegatedCastRejectsMissingControl(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := delegatedCastOpenRequest(f, 0, "delegated-assignment-orphan", fmt.Sprintf("choir.commitment_record:%s", objectgraph.SHA256([]byte("never-minted"))))
	if _, err := s.OpenEngineeringAssignment(ctx, open); err == nil {
		t.Fatal("delegated cast admitted with an absent commitment control")
	}
}

// TestDelegatedCastReportDoesNotPoisonConsumerDeliveredListing is the
// regression for sa-delegated-report-poisons-management-listing-2026-10-06:
// a delegated cast's producer report carries the cast commitment record id as
// its ControlBindingID — not a lifecycle control update id — so the
// consuming Management run's persistentManagementControlBinding check can
// never match. Before the fix the delivered-page listing returned
// ErrLifecycleInvalidTransition on the whole page, killing the run ~30s into
// every subsequent activation. The delegated lineage is authenticated via
// the producing work item's recorded parent join instead.
func TestDelegatedCastReportDoesNotPoisonConsumerDeliveredListing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	controlID, err := s.AppendCommitmentRecord(ctx, f.ownerID, f.computerID, types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-cell:cast:poison-test",
		Provenance: types.CommitmentProvenance{AgentID: f.parentAgentID},
	})
	if err != nil {
		t.Fatal(err)
	}
	open := delegatedCastOpenRequest(f, 0, "delegated-assignment-poison", controlID)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("delegated open: %v", err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-delegated-assignment-poison")); err != nil {
		t.Fatalf("delegated bind: %v", err)
	}
	report := assignmentReportRequest(open, 2, "report-poison", open.Binding.SubjectDigest, types.EngineeringResultPartial, types.EngineeringVerdictNone)
	result, err := s.RecordEngineeringAssignmentReport(ctx, report)
	if err != nil || result.Update == nil || result.Update.Direction != types.LifecyclePacketDirectionProducerReport ||
		result.Update.DeliveredToRunID != f.parentRunID || result.Update.ControlBindingID != controlID {
		t.Fatalf("delegated report = %+v err=%v", result, err)
	}
	// The consuming Management run's delivered-page listing must not throw on
	// the delegated report — before the fix this returned
	// ErrLifecycleInvalidTransition (the poisoned-listing defect).
	packets, err := s.ListLifecycleControlsDeliveredToRun(ctx, f.ownerID, f.computerID, f.trajectoryID, f.parentAgentID, f.parentRunID, 10)
	if err != nil {
		t.Fatalf("delivered listing threw on delegated report: %v", err)
	}
	if len(packets) != 1 || packets[0].UpdateID != result.Update.UpdateID || packets[0].Direction != types.LifecyclePacketDirectionProducerReport {
		t.Fatalf("delivered packets = %+v", packets)
	}
}

// docs/problems/delegated-cast-authority-dies-with-the-caster-turn-2026-10-10.md:
// management casts, reports and ends its turn within seconds; the deferred
// spawn binds after. Failure modes pinned:
//   - live transitions after the casting turn completes are refused ("caster
//     run is not live"), so no delegated cast ever reaches a capsule;
//   - a newer management turn (the agent's ActiveRunID moved on) revokes the
//     assignment;
//   - admission stops requiring the live casting turn;
//   - the parent work item closing no longer revokes live authority.
func TestDelegatedCastAuthorityOutlivesTheCastingTurn(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	controlID, err := s.AppendCommitmentRecord(ctx, f.ownerID, f.computerID, types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cast-cell:cast:turn",
		Provenance: types.CommitmentProvenance{AgentID: f.parentAgentID},
	})
	if err != nil {
		t.Fatal(err)
	}
	open := delegatedCastOpenRequest(f, 0, "delegated-assignment-turn", controlID)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("admission during the casting turn: %v", err)
	}

	// The casting turn ends (UpdateRun releases the agent's ActiveRunID), then
	// a newer management turn becomes the agent's active run.
	run, err := s.GetRun(ctx, f.parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	finished := time.Now().UTC()
	run.State, run.UpdatedAt, run.FinishedAt = types.RunCompleted, finished, &finished
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := s.requireEngineeringParentAuthority(ctx, open.Binding); err != nil {
		t.Fatalf("live authority died with the casting turn: %v", err)
	}
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, f.ownerID, f.computerID, f.parentAgentID)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		t.Fatal(err)
	}
	agent.ActiveRunID = "run-management-next-turn"
	nextTurn, err := lifecycleObject(ogKindAgent, f.ownerID, f.computerID, f.parentAgentID, agent,
		map[string]any{"agent_id": f.parentAgentID, "computer_id": f.computerID}, agent.CreatedAt, finished)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{nextTurn}}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.requireEngineeringParentAuthority(ctx, open.Binding); err != nil {
		t.Fatalf("live authority died when a newer management turn started: %v", err)
	}
	if _, err := s.requireEngineeringDelegatedParentAuthority(ctx, open.Binding, delegatedAuthorityAdmission); err == nil {
		t.Fatal("admission accepted a cast whose turn has ended")
	}

	// Closing the parent work item is a revocation.
	_, work, err := s.lifecycleWorkObject(ctx, f.ownerID, f.computerID, f.parentWorkID)
	if err != nil {
		t.Fatal(err)
	}
	work.Status, work.LifecycleVersion = types.WorkItemCompleted, work.LifecycleVersion+1
	closed, err := lifecycleObject(ogKindWorkItem, f.ownerID, f.computerID, f.parentWorkID, work,
		lifecycleMetadata("work_item_id", f.parentWorkID, f.computerID, f.trajectoryID, work.LifecycleVersion), work.CreatedAt, finished)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{closed}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.requireEngineeringParentAuthority(ctx, open.Binding); err == nil {
		t.Fatal("live authority survived the parent work item closing")
	}
	if _, err := s.requireEngineeringHistoricalParentAuthority(ctx, open.Binding); err != nil {
		t.Fatalf("historical authority lost after the parent work closed: %v", err)
	}
}
