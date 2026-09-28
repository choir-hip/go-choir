package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Regression for docs/problems/engineering-verification-chain-dead-2026-09-28.md.
//
// A completed implementation whose bound self-development operation is frozen
// must reach the verification opener — through the ref-resolution gate
// (ReportRefs are canonical object IDs, resolved by canonical ID, not by the
// parsed report-id suffix) and past the agent-headed document gate (a
// desk-authored head ends cast admission, not the verification obligation).
// Before the repair both gates failed silently: the candidate never resolved,
// or the reconcile early-returned, and the operation wedged at frozen forever.
//
// The test commits an agent-authored revision as the document head so the
// reconcile takes the verification-only path, mirroring the landed state that
// wedged staging (desk revision head, operation frozen).
func TestReconcileEngineeringDeskOnAgentHeadOpensVerification(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "owner-verification-chain"
	computerID := "computer-verification-chain"
	rt.cfg.ComputerID = computerID

	// Bound lifecycle: trajectory + engineering doc (v0 owner-authored head) +
	// desk work item, exactly as the op-start path builds it.
	operationID := "selfdev-verification-chain-wedge"
	op := selfdev.Operation{
		OperationID:       operationID,
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-verification-chain",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	}
	if err := rt.ensureSelfDevelopmentEngineeringDoc(ctx, op, ownerID, "Author the wedge reproducer"); err != nil {
		t.Fatal(err)
	}
	docID, revisionID, workID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operationID)
	trajectoryID := op.TrajectoryID


	// The desk landed its revision: the document head is agent-authored, the
	// state that previously skipped the verification chain outright.
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	agentRevision := types.Revision{
		RevisionID: "revision-desk-landed-" + operationID,
		DocID:      docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: engineeringDeskAgentID(docID),
		Content: "Landed the requested engine", CreatedAt: time.Now().UTC(),
		BodyDoc: runtimeTestTextureBodyDoc(t, docID, "revision-desk-landed-"+operationID, "Landed the requested engine"),
		ParentRevisionID: snapshot.HeadRevision.RevisionID,
	}
	revCmd := types.CommitLifecycleArtifactHeadRequest{
		CommandID: "revise:verification-chain:" + operationID,
		OwnerID:   ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
		Revision:                 agentRevision,
	}
	revCmd.CommandDigest, _ = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(revCmd, store.TextureSourceGraphWriteSet{})
	if _, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, revCmd, store.TextureSourceGraphWriteSet{}); err != nil {
		t.Fatalf("commit desk revision head: %v", err)
	}

	// A frozen self-development operation bound to the same trajectory.
	now := time.Now().UTC()
	bundleDigest := strings.Repeat("ab", 32)
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
		operationID, computerID, "verification-chain-wedge", strings.Repeat("0", 64), trajectoryID,
		strings.Repeat("a", 64), op.PromptArtifactRef, bundleDigest,
		strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateFrozen, now, now); err != nil {
		t.Fatal(err)
	}

	// A completed implementation assignment admitted by the owner revision.
	assignmentID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentImplementation, "")
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			ParentAgentID:    engineeringDeskAgentID(docID),
			ParentDecisionID: "decision:sha256:" + strings.Repeat("d", 64),
			ParentControlID:  revisionID, ParentWorkItemID: workID,
			AssignedWorkItemID: "work:verification-chain-assigned", AssignedAgentID: "engineering:" + assignmentID,
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-verification-chain"), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-verification-chain")),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-verification-chain",
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: "engineering:" + assignmentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: "work:verification-chain-assigned", AssignedAgentID: "engineering:" + assignmentID, Objective: "implement the wedge"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	runID := "run:" + assignmentID
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		TrajectoryID: trajectoryID, AgentProfile: "engineering", AgentRole: "engineering",
		OwnerID: ownerID, ComputerID: computerID, State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "engineering",
			"assignment_id": assignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
			"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
			"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
		},
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + assignmentID + "-1",
		OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: "cap-verification-chain", CapsuleID: open.Binding.CapsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.BindEngineeringAssignment(ctx, bind)
	if err != nil {
		t.Fatal(err)
	}

	// Terminal report: completed, subject bytes changed -> mints a candidate.
	newDigest := objectgraph.SHA256([]byte("changed subject bytes"))
	report := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "command-report-verification-chain", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: bound.Assignment.LifecycleVersion,
		Report: types.EngineeringAssignmentReport{
			ReportID: "report-verification-chain", Result: types.EngineeringResultCompleted,
			Verdict: types.EngineeringVerdictNone, ObservedSubjectDigest: newDigest, Summary: "landed",
			Commands: []types.EngineeringRecordedCommand{{CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("command")), ExecutionRef: "receipt:execution"}},
			Outputs:  []types.EngineeringRecordedOutput{{OutputID: "output", Kind: "evidence", Digest: objectgraph.SHA256([]byte("output")), Ref: "artifact:output"}},
			Mutations: []types.EngineeringRecordedMutation{{
				MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: open.Binding.SubjectDigest,
				AfterDigest: newDigest, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
			}},
		},
	}
	report.Report.CandidateArtifactRef = "capsule-subject:" + newDigest
	report.CommandDigest, err = store.ComputeRecordEngineeringAssignmentReportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := s.RecordEngineeringAssignmentReport(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Report == nil || recorded.Report.CandidateID == "" || recorded.Assignment.Disposition != types.EngineeringAssignmentCompleted {
		t.Fatalf("report commit: %+v err=%v", recorded.Report, err)
	}

	// The wedge scenario: agent head, frozen op, completed implementation.
	// The verification opener must be reached — with no capsule executor in
	// testRuntime it fails closed at capsule authority, which proves the
	// reconcile passed every upstream gate. Before the repair the reconcile
	// silently returned nil (canonical-ID suffix fed to a ReportID-keyed
	// lookup, or the owner-head gate skipped the chain).
	if _, err := rt.ReconcileEngineeringDesk(ctx, ownerID, docID); err == nil ||
		!strings.Contains(err.Error(), "capsule authority") {
		t.Fatalf("verification reconcile err = %v, want capsule-authority reach", err)
	}
}
