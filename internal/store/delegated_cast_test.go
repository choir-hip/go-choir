package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
