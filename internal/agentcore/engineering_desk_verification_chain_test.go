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

// Regression for the verified-state wedge the Sep-28 consensus panel found:
// finalizeVerifiedCapsuleBundle commits frozen->verified (final digest +
// verifier ref on the operation), then recordSelfDevelopmentVerification
// commits verified->awaiting_approval. A guest restart inside that pair left
// the op stranded at verified — the reconcile gate never included the state,
// and re-verification was impossible (the bundle draft was already renamed
// to the final digest). The repair resumes the parked transition directly:
// verified -> awaiting_approval is a pure operation transition.
func TestReconcileEngineeringDeskResumesVerifiedOperation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "owner-verified-resume"
	computerID := "computer-verified-resume"
	rt.cfg.ComputerID = computerID

	operationID := "selfdev-verified-resume"
	op := selfdev.Operation{
		OperationID:       operationID,
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-verified-resume",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	}
	if err := rt.ensureSelfDevelopmentEngineeringDoc(ctx, op, ownerID, "Author the wedge reproducer"); err != nil {
		t.Fatal(err)
	}
	docID, revisionID, workID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operationID)
	trajectoryID := op.TrajectoryID

	// Agent-authored head: an owner head would enter cast admission and die
	// at capsule authority before verification-only reconcile runs.
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
		CommandID: "revise:verified-resume:" + operationID,
		OwnerID:   ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
		Revision:                 agentRevision,
	}
	revCmd.CommandDigest, _ = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(revCmd, store.TextureSourceGraphWriteSet{})
	if _, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, revCmd, store.TextureSourceGraphWriteSet{}); err != nil {
		t.Fatalf("commit desk revision head: %v", err)
	}

	// A completed implementation so reconcileVerificationOnly reaches the
	// verification chain where the verified-op resume lives.
	implID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentImplementation, "")
	openImpl := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + implID + "-1", AssignmentID: implID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			ParentAgentID:    engineeringDeskAgentID(docID),
			ParentDecisionID: "decision:sha256:" + strings.Repeat("d", 64),
			ParentControlID:  revisionID, ParentWorkItemID: workID,
			AssignedWorkItemID: "work:verified-resume-assigned", AssignedAgentID: "engineering:" + implID,
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + implID)), RequestDigest: objectgraph.SHA256([]byte("request:" + implID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-verified-resume"), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-verified-resume")),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + implID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+implID)),
			Writable:          true, CapsuleID: "capsule-verified-resume",
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: "engineering:" + implID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: "work:verified-resume-assigned", AssignedAgentID: "engineering:" + implID, Objective: "implement"},
	}
	openImpl.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(openImpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, openImpl); err != nil {
		t.Fatal(err)
	}
	implRun := types.RunRecord{
		RunID: "run:" + implID, AgentID: openImpl.Binding.AssignedAgentID, ChannelID: openImpl.Binding.AssignedAgentID,
		TrajectoryID: trajectoryID, AgentProfile: "engineering", AgentRole: "engineering",
		OwnerID: ownerID, ComputerID: computerID, State: types.RunPending, Prompt: "implement",
		Metadata: map[string]any{
			"work_item_ids": []string{openImpl.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": openImpl.Binding.AssignedWorkItemID,
			"requested_by_agent_id": openImpl.Binding.ParentAgentID, "requested_by_profile": "engineering",
			"assignment_id": implID, "assignment_attempt": 1, "assignment_kind": string(openImpl.Binding.Kind),
			"assigned_work_item_id": openImpl.Binding.AssignedWorkItemID, "parent_work_item_id": openImpl.Binding.ParentWorkItemID,
			"parent_decision_id": openImpl.Binding.ParentDecisionID, "parent_control_id": openImpl.Binding.ParentControlID,
			"capsule_id": openImpl.Binding.CapsuleID, "scope_digest": openImpl.Binding.ScopeDigest, "request_digest": openImpl.Binding.RequestDigest,
			"capability_digest": openImpl.Binding.CapabilityDigest, "execution_handle_digest": openImpl.Binding.ExecutionHandleDigest,
			"subject_digest": openImpl.Binding.SubjectDigest, "source_artifact_ref": openImpl.Binding.SourceArtifactRef,
		},
	}
	implBind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + implID + "-1", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: implID, Attempt: 1, ExpectedLifecycleVersion: 1, RunID: implRun.RunID, Run: implRun,
		OpaqueCapability: "cap-verified-resume", CapsuleID: openImpl.Binding.CapsuleID,
	}
	implBind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(implBind)
	if err != nil {
		t.Fatal(err)
	}
	boundImpl, err := s.BindEngineeringAssignment(ctx, implBind)
	if err != nil {
		t.Fatal(err)
	}
	newDigest := objectgraph.SHA256([]byte("verified resume candidate bytes"))
	implReport := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "command-report-verified-resume", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: implID, Attempt: 1, ExpectedLifecycleVersion: boundImpl.Assignment.LifecycleVersion,
		Report: types.EngineeringAssignmentReport{
			ReportID: "report-verified-resume", Result: types.EngineeringResultCompleted,
			Verdict: types.EngineeringVerdictNone, ObservedSubjectDigest: newDigest, Summary: "landed",
			Commands: []types.EngineeringRecordedCommand{{CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("command")), ExecutionRef: "receipt:execution"}},
			Outputs:  []types.EngineeringRecordedOutput{{OutputID: "output", Kind: "evidence", Digest: objectgraph.SHA256([]byte("output")), Ref: "artifact:output"}},
			Mutations: []types.EngineeringRecordedMutation{{
				MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: openImpl.Binding.SubjectDigest,
				AfterDigest: newDigest, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
			}},
		},
	}
	implReport.Report.CandidateArtifactRef = "capsule-subject:" + newDigest
	implReport.CommandDigest, err = store.ComputeRecordEngineeringAssignmentReportDigest(implReport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordEngineeringAssignmentReport(ctx, implReport); err != nil {
		t.Fatal(err)
	}


	now := time.Now().UTC()
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		operationID, computerID, "verified-resume", strings.Repeat("0", 64), trajectoryID,
		strings.Repeat("a", 64), op.PromptArtifactRef, strings.Repeat("ab", 32),
		`["`+strings.Repeat("ff", 32)+`"]`,
		strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateVerified, now, now); err != nil {
		t.Fatal(err)
	}

	// The verified operation needs no candidate or assignment to resume —
	// bundle + verifier ref are already committed. Reconcile must land the
	// parked verified->awaiting_approval transition.
	if _, err := rt.ReconcileEngineeringDesk(ctx, ownerID, docID); err != nil {
		t.Fatalf("reconcile verified op: %v", err)
	}
	resumed, err := rt.selfdevOperations.GetByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.State != selfdev.StateAwaitingApproval {
		t.Fatalf("verified op did not resume to awaiting_approval; state=%q", resumed.State)
	}
}

// Regression for the terminal-verification wedge: a verification assignment
// that reached a terminal disposition while the bound operation stayed frozen
// left the reconcile returning the dead verification forever — the op could
// never advance and no further verification could be admitted. The repair
// fails the operation instead of retrying a chain that already terminated.
func TestReconcileEngineeringDeskFailsFrozenOpOnTerminalVerification(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "owner-terminal-verification"
	computerID := "computer-terminal-verification"
	rt.cfg.ComputerID = computerID

	operationID := "selfdev-terminal-verification"
	op := selfdev.Operation{
		OperationID:       operationID,
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-terminal-verification",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("e", 64),
	}
	if err := rt.ensureSelfDevelopmentEngineeringDoc(ctx, op, ownerID, "Author the wedge reproducer"); err != nil {
		t.Fatal(err)
	}
	docID, revisionID, workID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operationID)
	trajectoryID := op.TrajectoryID

	// Agent head: verification-only reconcile path.
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
		CommandID: "revise:terminal-verification:" + operationID,
		OwnerID:   ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
		Revision:                 agentRevision,
	}
	revCmd.CommandDigest, _ = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(revCmd, store.TextureSourceGraphWriteSet{})
	if _, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, revCmd, store.TextureSourceGraphWriteSet{}); err != nil {
		t.Fatalf("commit desk revision head: %v", err)
	}

	now := time.Now().UTC()
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
		operationID, computerID, "terminal-verification", strings.Repeat("0", 64), trajectoryID,
		strings.Repeat("a", 64), op.PromptArtifactRef, strings.Repeat("ab", 32),
		strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateFrozen, now, now); err != nil {
		t.Fatal(err)
	}

	// Completed implementation with a candidate, exactly as the chain test.
	implID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentImplementation, "")
	openImpl := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + implID + "-1", AssignmentID: implID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			ParentAgentID:    engineeringDeskAgentID(docID),
			ParentDecisionID: "decision:sha256:" + strings.Repeat("d", 64),
			ParentControlID:  revisionID, ParentWorkItemID: workID,
			AssignedWorkItemID: "work:terminal-verification-assigned", AssignedAgentID: "engineering:" + implID,
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + implID)), RequestDigest: objectgraph.SHA256([]byte("request:" + implID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-terminal-verification"), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-terminal-verification")),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + implID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+implID)),
			Writable:          true, CapsuleID: "capsule-terminal-verification",
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: "engineering:" + implID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: "work:terminal-verification-assigned", AssignedAgentID: "engineering:" + implID, Objective: "implement"},
	}
	openImpl.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(openImpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, openImpl); err != nil {
		t.Fatal(err)
	}
	implRun := types.RunRecord{
		RunID: "run:" + implID, AgentID: openImpl.Binding.AssignedAgentID, ChannelID: openImpl.Binding.AssignedAgentID,
		TrajectoryID: trajectoryID, AgentProfile: "engineering", AgentRole: "engineering",
		OwnerID: ownerID, ComputerID: computerID, State: types.RunPending, Prompt: "implement",
		Metadata: map[string]any{
			"work_item_ids": []string{openImpl.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": openImpl.Binding.AssignedWorkItemID,
			"requested_by_agent_id": openImpl.Binding.ParentAgentID, "requested_by_profile": "engineering",
			"assignment_id": implID, "assignment_attempt": 1, "assignment_kind": string(openImpl.Binding.Kind),
			"assigned_work_item_id": openImpl.Binding.AssignedWorkItemID, "parent_work_item_id": openImpl.Binding.ParentWorkItemID,
			"parent_decision_id": openImpl.Binding.ParentDecisionID, "parent_control_id": openImpl.Binding.ParentControlID,
			"capsule_id": openImpl.Binding.CapsuleID, "scope_digest": openImpl.Binding.ScopeDigest, "request_digest": openImpl.Binding.RequestDigest,
			"capability_digest": openImpl.Binding.CapabilityDigest, "execution_handle_digest": openImpl.Binding.ExecutionHandleDigest,
			"subject_digest": openImpl.Binding.SubjectDigest, "source_artifact_ref": openImpl.Binding.SourceArtifactRef,
		},
	}
	implBind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + implID + "-1", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: implID, Attempt: 1, ExpectedLifecycleVersion: 1, RunID: implRun.RunID, Run: implRun,
		OpaqueCapability: "cap-terminal-verification", CapsuleID: openImpl.Binding.CapsuleID,
	}
	implBind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(implBind)
	if err != nil {
		t.Fatal(err)
	}
	boundImpl, err := s.BindEngineeringAssignment(ctx, implBind)
	if err != nil {
		t.Fatal(err)
	}
	newDigest := objectgraph.SHA256([]byte("terminal verification candidate bytes"))
	implReport := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "command-report-terminal-verification", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: implID, Attempt: 1, ExpectedLifecycleVersion: boundImpl.Assignment.LifecycleVersion,
		Report: types.EngineeringAssignmentReport{
			ReportID: "report-terminal-verification", Result: types.EngineeringResultCompleted,
			Verdict: types.EngineeringVerdictNone, ObservedSubjectDigest: newDigest, Summary: "landed",
			Commands: []types.EngineeringRecordedCommand{{CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("command")), ExecutionRef: "receipt:execution"}},
			Outputs:  []types.EngineeringRecordedOutput{{OutputID: "output", Kind: "evidence", Digest: objectgraph.SHA256([]byte("output")), Ref: "artifact:output"}},
			Mutations: []types.EngineeringRecordedMutation{{
				MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: openImpl.Binding.SubjectDigest,
				AfterDigest: newDigest, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
			}},
		},
	}
	implReport.Report.CandidateArtifactRef = "capsule-subject:" + newDigest
	implReport.CommandDigest, err = store.ComputeRecordEngineeringAssignmentReportDigest(implReport)
	if err != nil {
		t.Fatal(err)
	}
	recordedImpl, err := s.RecordEngineeringAssignmentReport(ctx, implReport)
	if err != nil {
		t.Fatal(err)
	}
	candidateID := recordedImpl.Report.CandidateID
	if candidateID == "" {
		t.Fatal("implementation report minted no candidate")
	}

	// A verification assignment that already terminated while the operation
	// stayed frozen — the wedge the reconcile previously returned forever.
	verificationID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentVerification, candidateID)
	openVer := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + verificationID + "-1", AssignmentID: verificationID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			ParentAgentID:    engineeringDeskAgentID(docID),
			ParentDecisionID: "decision:sha256:" + strings.Repeat("e", 64),
			ParentControlID:  candidateID, ParentWorkItemID: workID,
			AssignedWorkItemID: "work:terminal-verification-v", AssignedAgentID: "engineering:" + verificationID,
			Kind: types.EngineeringAssignmentVerification, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + verificationID)), RequestDigest: objectgraph.SHA256([]byte("request:" + verificationID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-terminal-verification-v"), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-terminal-verification-v")),
			SubjectDigest:       newDigest,
			SourceArtifactRef:   "capsule-subject:" + newDigest,
			SourceCandidateID:   candidateID,
			Writable:            true, CapsuleID: "capsule-terminal-verification-v",
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: "engineering:" + verificationID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: "work:terminal-verification-v", AssignedAgentID: "engineering:" + verificationID, Objective: "verify"},
	}
	openVer.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(openVer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, openVer); err != nil {
		t.Fatal(err)
	}
	verRun := types.RunRecord{
		RunID: "run:" + verificationID, AgentID: openVer.Binding.AssignedAgentID, ChannelID: openVer.Binding.AssignedAgentID,
		TrajectoryID: trajectoryID, AgentProfile: "engineering", AgentRole: "engineering",
		OwnerID: ownerID, ComputerID: computerID, State: types.RunPending, Prompt: "verify",
		Metadata: map[string]any{
			"work_item_ids": []string{openVer.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": openVer.Binding.AssignedWorkItemID,
			"requested_by_agent_id": openVer.Binding.ParentAgentID, "requested_by_profile": "engineering",
			"assignment_id": verificationID, "assignment_attempt": 1, "assignment_kind": string(openVer.Binding.Kind),
			"assigned_work_item_id": openVer.Binding.AssignedWorkItemID, "parent_work_item_id": openVer.Binding.ParentWorkItemID,
			"parent_decision_id": openVer.Binding.ParentDecisionID, "parent_control_id": openVer.Binding.ParentControlID,
			"capsule_id": openVer.Binding.CapsuleID, "scope_digest": openVer.Binding.ScopeDigest, "request_digest": openVer.Binding.RequestDigest,
			"capability_digest": openVer.Binding.CapabilityDigest, "execution_handle_digest": openVer.Binding.ExecutionHandleDigest,
			"subject_digest": openVer.Binding.SubjectDigest, "source_artifact_ref": openVer.Binding.SourceArtifactRef,
			"source_candidate_id": candidateID,
		},
	}
	verBind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + verificationID + "-1", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: verificationID, Attempt: 1, ExpectedLifecycleVersion: 1, RunID: verRun.RunID, Run: verRun,
		OpaqueCapability: "cap-terminal-verification-v", CapsuleID: openVer.Binding.CapsuleID,
	}
	verBind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(verBind)
	if err != nil {
		t.Fatal(err)
	}
	boundVer, err := s.BindEngineeringAssignment(ctx, verBind)
	if err != nil {
		t.Fatal(err)
	}
	verReport := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "command-report-terminal-v", OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: verificationID, Attempt: 1, ExpectedLifecycleVersion: boundVer.Assignment.LifecycleVersion,
		Report: types.EngineeringAssignmentReport{
			ReportID: "report-terminal-v", Result: types.EngineeringResultFailed,
			Verdict: types.EngineeringVerdictFail, ObservedSubjectDigest: newDigest, Summary: "verifier cell died",
			Commands: []types.EngineeringRecordedCommand{{CommandID: "observed-command-v", CommandDigest: objectgraph.SHA256([]byte("command-v")), ExecutionRef: "receipt:execution-v"}},
		},
	}
	verReport.CommandDigest, err = store.ComputeRecordEngineeringAssignmentReportDigest(verReport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordEngineeringAssignmentReport(ctx, verReport); err != nil {
		t.Fatal(err)
	}

	// Reconcile must terminalize the frozen operation, not retry forever.
	_, err = rt.ReconcileEngineeringDesk(ctx, ownerID, docID)
	if err == nil || !strings.Contains(err.Error(), "stayed frozen") {
		t.Fatalf("reconcile terminal verification err = %v, want stayed-frozen wedge error", err)
	}
	failed, err := rt.selfdevOperations.GetByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.State != selfdev.StateFailed {
		t.Fatalf("op state = %q, want failed after terminal verification wedge", failed.State)
	}
}
