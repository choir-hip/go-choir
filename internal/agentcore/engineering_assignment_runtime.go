package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	engineeringAssignmentMemoryMax = int64(1 << 30)
	engineeringAssignmentCPUQuota  = int64(100000)
	engineeringAssignmentPidsMax   = int64(256)
	// coManagementAssignmentDeadline is the fail-closed scheduling deadline for one
	// live assignment (I26): a bound assignment that has not reached a terminal
	// disposition by this bound is cancelled with an expired reason. The
	// underlying execution request stays pending and retryable — the scheduler
	// may re-admit it later; expiry fails the assignment, never the work.
	engineeringAssignmentDeadline = 6 * time.Hour
	// assignedEngineeringFateWatchdogDelay is the fate-transition watchdog delay:
	// after a committed disposition strands the terminal saga, the watchdog
	// re-drives the continuation without waiting for a worker cell or a
	// restart. Longer than an in-flight commit needs, shorter than the
	// deadline backstop.
	assignedEngineeringFateWatchdogDelay = 5 * time.Minute
)

type assignmentCapsuleRuntime interface {
	Spawn(context.Context, capsule.SpawnSpec) (*capsule.Capsule, error)
	MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration, string) (*capsule.Capability, error)
	RevokeCapability(string, string) error
	ForceDestroy(context.Context, string) error
	ExtractGranted(context.Context, string, string) ([]capsule.FileChange, error)
	ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error)
	ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error)
	AssignmentHandle(string, string) (string, error)
	InspectCapsuleRaw(string) (*capsule.CapsuleDiagnostics, error)
	HasCapsule(string) bool
	CleanupOrphanedCapsule(context.Context, string) error
	PersistRevocationReceipt(string, string, string, string) (capsule.CapsuleRevocationReceipt, error)
}

type AssignedEngineeringStart struct {
	Assignment types.EngineeringAssignment
	Run        types.RunRecord
	Replay     bool
}

// overlayIDNamedInObjective reports the model_policy_overlay_id value assigned
// in prose inside an objective (model_policy_overlay_id=<id>), or "" when the
// objective names none. A bare mention without an assignment is not a naming.
// The id charset mirrors the overlay loader ([A-Za-z0-9_-]); a malformed
// assignment is skipped in favour of a later well-formed one.
func overlayIDNamedInObjective(objective string) string {
	const key = "model_policy_overlay_id"
	named := ""
	rest := objective
	for {
		i := strings.Index(rest, key)
		if i < 0 {
			return named
		}
		after := strings.TrimSpace(rest[i+len(key):])
		rest = after
		if !strings.HasPrefix(after, "=") {
			continue
		}
		after = strings.TrimSpace(strings.TrimPrefix(after, "="))
		after = strings.Trim(after, "\"'")
		end := strings.IndexFunc(after, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		})
		var id string
		if end < 0 {
			id = after
		} else {
			id = after[:end]
		}
		if id != "" && len(id) <= 96 {
			named = id
		}
	}
}

// deterministicDocumentAssignmentIdentity derives the assignment identity for
// one document-driven cast. The revision is the admission: the same revision
// always names the same assignment, so occurrence redelivery and boot scans
// replay rather than mint duplicates.
func deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID string, kind types.EngineeringAssignmentKind, candidateID string) string {
	seed := strings.Join([]string{
		"choir:co-super-assignment:v3", ownerID, computerID, trajectoryID, revisionID, string(kind), candidateID,
	}, "\x00")
	return "assignment-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String()
}

// deterministicDocumentAssignmentCapability derives the opaque capability for a
// document-driven assignment. Determinism is required for resume: an open but
// unbound assignment must re-derive the exact capability its binding digest
// committed, or the bind receipt can never be replayed after a crash between
// open and bind.
func deterministicDocumentAssignmentCapability(assignmentID string, attempt uint64) string {
	return "assignment-cap-" + strings.TrimPrefix(objectgraph.SHA256([]byte(assignmentID+"\x00cap\x00"+fmt.Sprint(attempt))), "sha256:")
}

// OpenDocumentAssignmentRequest is the document-channel cast: one owner-authored
// revision on an engineering-bound lifecycle document opens one assignment.
type OpenDocumentAssignmentRequest struct {
	Objective            string
	Kind                 types.EngineeringAssignmentKind
	CandidateID          string
	RevisionID           string
	ModelPolicyOverlayID string
}

// DelegatedCastRequest is a desk-staged choir.Cast: one desk cell admits one
// engineering assignment under the delegated-cast authority (R2). The parent
// authority is the caster's own live run/work — not an owner revision.
type DelegatedCastRequest struct {
	Objective           string
	Kind                types.EngineeringAssignmentKind
	CandidateID         string // verification casts only
	CommitmentControlID string // canonical ID of the cast's commitment record
	CasterRun           types.RunRecord
	CasterAgentID       string
	TargetDocID         string // engineering-bound document the cast targets
	ScopeDigestSeed     string // deterministic seed so a replayed cell re-derives the same scope digest
}

// startAssignedEngineeringForDocument opens one assignment whose parent authority is
// the engineering desk agent bound to the document plus the owner-authored
// revision that carried the cast. It replaces the retired Management-mediated
// assign_co_super opener: the revision event IS the admission.
func (rt *Runtime) startAssignedEngineeringForDocument(ctx context.Context, doc types.Document, revision types.Revision, req OpenDocumentAssignmentRequest) (AssignedEngineeringStart, error) {
	req.Objective, req.CandidateID, req.RevisionID = strings.TrimSpace(req.Objective), strings.TrimSpace(req.CandidateID), strings.TrimSpace(req.RevisionID)
	if req.Objective == "" || req.RevisionID == "" ||
		(req.Kind != types.EngineeringAssignmentImplementation && req.Kind != types.EngineeringAssignmentVerification) {
		return AssignedEngineeringStart{}, fmt.Errorf("document assignment requires objective, kind, and the admitting revision")
	}
	if (req.Kind == types.EngineeringAssignmentVerification) != (req.CandidateID != "") {
		return AssignedEngineeringStart{}, fmt.Errorf("verification requires one exact candidate_id and implementation forbids it")
	}
	if strings.TrimSpace(req.ModelPolicyOverlayID) == "" {
		if named := overlayIDNamedInObjective(req.Objective); named != "" {
			return AssignedEngineeringStart{}, fmt.Errorf("document assignment objective names model_policy_overlay_id=%s but the structured field is empty: pass it as model_policy_overlay_id (prose names select nothing; the base policy would silently serve instead)", named)
		}
	}
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("assigned Engineering capsule authority unavailable")
	}
	ownerID, computerID := strings.TrimSpace(doc.OwnerID), strings.TrimSpace(doc.ComputerID)
	trajectoryID, docID := strings.TrimSpace(doc.TrajectoryID), strings.TrimSpace(doc.DocID)
	if ownerID == "" || computerID == "" || trajectoryID == "" || docID == "" ||
		revision.RevisionID != req.RevisionID || revision.DocID != docID || revision.TrajectoryID != trajectoryID ||
		revision.AuthorKind != types.AuthorUser {
		return AssignedEngineeringStart{}, fmt.Errorf("document assignment requires an owner-authored revision on the bound document")
	}
	parentAgentID := agentprofile.Engineering + ":" + docID
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("derive assignment scope: %w", err)
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return AssignedEngineeringStart{}, fmt.Errorf("document assignment requires a live trajectory")
	}
	parentWorkID := ""
	var parentWork *types.WorkItemRecord
	for i := range snapshot.WorkItems {
		work := snapshot.WorkItems[i]
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == parentAgentID && work.AuthorityProfile == agentprofile.Engineering {
			if parentWorkID != "" {
				return AssignedEngineeringStart{}, fmt.Errorf("document trajectory has multiple open engineering desk work items")
			}
			parentWorkID = work.WorkItemID
			copy := work
			parentWork = &copy
		}
	}
	if parentWork == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("document trajectory has no open engineering desk work item")
	}
	attempt := uint64(1)
	assignmentID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, req.RevisionID, req.Kind, req.CandidateID)
	requestDigestParts := []string{
		"choir:co-super-request:v2", req.Objective, string(req.Kind), req.CandidateID, parentWorkID, req.RevisionID,
	}
	if overlay := strings.TrimSpace(req.ModelPolicyOverlayID); overlay != "" {
		requestDigestParts = append(requestDigestParts, overlay)
	}
	requestDigest := objectgraph.SHA256([]byte(strings.Join(requestDigestParts, "\x00")))
	if existing, getErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt); getErr == nil {
		if existing.Binding.ParentAgentID != parentAgentID || existing.Binding.ParentControlID != req.RevisionID ||
			existing.Binding.ParentWorkItemID != parentWorkID || existing.Binding.Kind != req.Kind ||
			existing.Binding.RequestDigest != requestDigest || existing.Binding.SourceCandidateID != req.CandidateID {
			return AssignedEngineeringStart{}, store.ErrEngineeringAssignmentCommandConflict
		}
		if existing.Disposition == types.EngineeringAssignmentBound || existing.Disposition.Terminal() {
			run := types.RunRecord{}
			if existing.BoundRunID != "" {
				run, _ = rt.store.GetLifecycleRun(ctx, ownerID, computerID, existing.BoundRunID)
			}
			return AssignedEngineeringStart{Assignment: existing, Run: run, Replay: true}, nil
		}
		// Open but unbound: the durable open committed and the spawn/bind saga
		// stranded (process crash or transient failure). The deterministic
		// capability re-derives exactly, so resume the saga rather than fail.
		return rt.resumeAssignedEngineeringForDocument(ctx, existing, req)
	} else if !errors.Is(getErr, store.ErrNotFound) {
		return AssignedEngineeringStart{}, getErr
	}
	if err := rt.reclaimSupersededAssignmentCapsules(ctx, types.RunRecord{OwnerID: ownerID, ComputerID: computerID}, assignmentID); err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("reclaim superseded assignment capsules: %w", err)
	}
	parentControlID := req.RevisionID
	if req.Kind == types.EngineeringAssignmentVerification {
		// The verification assignment's parent control is the candidate record:
		// the durable receipt of the completed implementation it verifies.
		parentControlID = req.CandidateID
	}
	parentDecisionID := "decision:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-decision:v3", ownerID, computerID, parentAgentID, trajectoryID, parentWorkID, parentControlID, req.RevisionID,
	}, "\x00")))
	scopeBytes, err := json.Marshal(struct {
		Revision types.Revision       `json:"revision"`
		Work     types.WorkItemRecord `json:"work"`
	}{revision, *parentWork})
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	scopeDigest := objectgraph.SHA256(scopeBytes)
	sourceArtifactRef := ""
	if req.Kind == types.EngineeringAssignmentVerification {
		candidate, candidateErr := rt.store.GetEngineeringSubjectCandidate(ctx, ownerID, computerID, req.CandidateID)
		if candidateErr != nil || candidate.TrajectoryID != trajectoryID || candidate.ArtifactRef == "" {
			return AssignedEngineeringStart{}, fmt.Errorf("verification candidate is unavailable or outside exact trajectory authority")
		}
		implementation, loadErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, candidate.AssignmentID, candidate.Attempt)
		if loadErr != nil || implementation.Binding.Kind != types.EngineeringAssignmentImplementation ||
			implementation.Binding.ParentAgentID != parentAgentID || implementation.Binding.ParentWorkItemID != parentWorkID ||
			implementation.Binding.TrajectoryID != trajectoryID || implementation.Disposition != types.EngineeringAssignmentCompleted {
			return AssignedEngineeringStart{}, fmt.Errorf("verification candidate is not an exact completed implementation artifact")
		}
		sourceArtifactRef = candidate.ArtifactRef
	}
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, sourceArtifactRef)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("preflight immutable assignment subject: %w", err)
	}
	subjectDigest := "sha256:" + strings.TrimPrefix(preflight.SubjectDigest, "sha256:")
	if req.Kind == types.EngineeringAssignmentVerification {
		candidate, _ := rt.store.GetEngineeringSubjectCandidate(ctx, ownerID, computerID, req.CandidateID)
		if candidate.SubjectDigest != subjectDigest || candidate.ArtifactRef != preflight.ArtifactRef {
			return AssignedEngineeringStart{}, fmt.Errorf("verification candidate artifact digest mismatch")
		}
	}
	opaque := deterministicDocumentAssignmentCapability(assignmentID, attempt)
	binding := types.EngineeringAssignmentBinding{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ParentAgentID: parentAgentID, ParentRunID: "", ParentDecisionID: parentDecisionID,
		ParentControlID: parentControlID, ParentWorkItemID: parentWorkID,
		AssignedWorkItemID: "work:" + assignmentID, AssignedAgentID: agentprofile.Engineering + ":" + assignmentID,
		Kind: req.Kind, Attempt: attempt,
		ScopeDigest: scopeDigest, RequestDigest: requestDigest, CapabilityDigest: store.DigestEngineeringOpaqueCapability(opaque),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(opaque)), SubjectDigest: subjectDigest,
		SourceArtifactRef: preflight.ArtifactRef, SourceCandidateID: req.CandidateID,
		Writable: true, CapsuleID: "capsule-" + strings.TrimPrefix(uuid.NewSHA1(uuid.NameSpaceOID, []byte(assignmentID+"\x00"+fmt.Sprint(attempt))).String(), "-"),
		NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
		FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "co-super-open:" + assignmentID + fmt.Sprintf(":%d", attempt), AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID, Objective: req.Objective},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	opened, err := rt.store.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	return rt.spawnBindActivateAssignment(ctx, opened.Assignment, preflight, opaque, req)
}

// startDelegatedCastAssignment opens one assignment under the delegated-cast
// authority (R2): the caster desk's own live run/work is the parent, and the
// cast's commitment record is the parent control. It is the delegated
// counterpart to startAssignedEngineeringForDocument — same fate saga, a
// different admission authority and identity scheme.
//
// Cell-commit callers must use openDelegatedCastAssignment + armDelegatedCastSpawn:
// the durable open is the commit and the spawn/bind saga resumes from the
// deferred delegated_assignment_spawn_deadline wake (consensus precondition —
// no spawn work inside the cell reducer).
func (rt *Runtime) startDelegatedCastAssignment(ctx context.Context, req DelegatedCastRequest) (AssignedEngineeringStart, error) {
	opened, err := rt.openDelegatedCastAssignment(ctx, req)
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	return rt.resumeDelegatedCastAssignment(ctx, opened.Assignment, req)
}

// openDelegatedCastAssignment commits only the durable open for a delegated
// cast: it resolves the caster's open work item, mints the deterministic
// binding, and persists the assignment as Open. The spawn/bind/activate saga
// is deliberately NOT driven here — the caller schedules the deferred
// delegated_assignment_spawn_deadline wake that resumes it post-commit. A
// replayed call returns the existing assignment (bound/terminal replays;
// open-but-unbound is handed back for resume).
func (rt *Runtime) openDelegatedCastAssignment(ctx context.Context, req DelegatedCastRequest) (AssignedEngineeringStart, error) {
	req.Objective, req.CandidateID = strings.TrimSpace(req.Objective), strings.TrimSpace(req.CandidateID)
	if req.Objective == "" || req.CommitmentControlID == "" ||
		(req.Kind != types.EngineeringAssignmentImplementation && req.Kind != types.EngineeringAssignmentVerification) {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast requires objective, kind, and the commitment control")
	}
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast: capsule authority unavailable")
	}
	run := req.CasterRun
	ownerID, computerID := strings.TrimSpace(run.OwnerID), strings.TrimSpace(run.ComputerID)
	trajectoryID := metadataStringValue(run.Metadata, "assignment_trajectory_id")
	parentAgentID := strings.TrimSpace(req.CasterAgentID)
	if ownerID == "" || computerID == "" || trajectoryID == "" || parentAgentID == "" {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast requires a trajectory-bound caster run and agent")
	}
	// Parent work = the caster's open work item on this trajectory.
	parentWorkID := ""
	var parentWork *types.WorkItemRecord
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast: derive scope: %w", err)
	}
	for i := range snapshot.WorkItems {
		work := snapshot.WorkItems[i]
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == parentAgentID {
			if parentWorkID != "" {
				return AssignedEngineeringStart{}, fmt.Errorf("delegated cast: caster holds multiple open work items")
			}
			parentWorkID = work.WorkItemID
			copy := work
			parentWork = &copy
		}
	}
	if parentWork == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast: caster has no open work item on the trajectory")
	}
	attempt := uint64(1)
	// Delegated assignment identity is deterministic on the caster's cast cell
	// (recorded in the commitment control id), so a replayed cell replays the
	// same assignment rather than minting a second one.
	assignmentID := "delegated-" + objectgraph.SHA256([]byte(strings.Join([]string{
		ownerID, computerID, trajectoryID, parentAgentID, req.CommitmentControlID, string(req.Kind),
	}, "\x00")))[0:24]
	requestDigest := objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:delegated-cast-request:v1", req.Objective, string(req.Kind), req.CandidateID,
		parentWorkID, req.CommitmentControlID,
	}, "\x00")))
	if existing, getErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt); getErr == nil {
		if existing.Binding.CastAuthority != types.EngineeringCastAuthorityDelegated ||
			existing.Binding.ParentAgentID != parentAgentID || existing.Binding.ParentControlID != req.CommitmentControlID ||
			existing.Binding.Kind != req.Kind || existing.Binding.RequestDigest != requestDigest {
			return AssignedEngineeringStart{}, store.ErrEngineeringAssignmentCommandConflict
		}
		if existing.Disposition == types.EngineeringAssignmentBound || existing.Disposition.Terminal() {
			return AssignedEngineeringStart{Assignment: existing, Replay: true}, nil
		}
		return rt.resumeDelegatedCastAssignment(ctx, existing, req)
	} else if !errors.Is(getErr, store.ErrNotFound) {
		return AssignedEngineeringStart{}, getErr
	}
	if err := rt.reclaimSupersededAssignmentCapsules(ctx, types.RunRecord{OwnerID: ownerID, ComputerID: computerID}, assignmentID); err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast reclaim superseded capsules: %w", err)
	}
	parentDecisionID := "decision:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:delegated-decision:v1", ownerID, computerID, parentAgentID, trajectoryID, parentWorkID,
		req.CommitmentControlID, run.RunID,
	}, "\x00")))
	scopeBytes, err := json.Marshal(struct {
		Intent string `json:"intent"`
		Run    string `json:"run"`
		Work   string `json:"work"`
		Seed   string `json:"seed"`
	}{req.Objective, run.RunID, parentWorkID, req.ScopeDigestSeed})
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	scopeDigest := objectgraph.SHA256(scopeBytes)
	sourceArtifactRef := ""
	if req.Kind == types.EngineeringAssignmentVerification {
		candidate, candErr := rt.store.GetEngineeringSubjectCandidate(ctx, ownerID, computerID, req.CandidateID)
		if candErr != nil || candidate.TrajectoryID != trajectoryID || candidate.ArtifactRef == "" {
			return AssignedEngineeringStart{}, fmt.Errorf("delegated verification candidate is unavailable or outside exact trajectory authority")
		}
		sourceArtifactRef = candidate.ArtifactRef
	}
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, sourceArtifactRef)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast preflight subject: %w", err)
	}
	subjectDigest := "sha256:" + strings.TrimPrefix(preflight.SubjectDigest, "sha256:")
	opaque := deterministicDocumentAssignmentCapability(assignmentID, attempt)
	binding := types.EngineeringAssignmentBinding{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ParentAgentID: parentAgentID, ParentRunID: run.RunID, ParentDecisionID: parentDecisionID,
		ParentControlID: req.CommitmentControlID, ParentWorkItemID: parentWorkID,
		AssignedWorkItemID: "work:" + assignmentID, AssignedAgentID: agentprofile.Engineering + ":" + assignmentID,
		Kind: req.Kind, Attempt: attempt,
		ScopeDigest: scopeDigest, RequestDigest: requestDigest, CapabilityDigest: store.DigestEngineeringOpaqueCapability(opaque),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(opaque)), SubjectDigest: subjectDigest,
		SourceArtifactRef: preflight.ArtifactRef, SourceCandidateID: req.CandidateID,
		Writable: true, CapsuleID: "capsule-" + strings.TrimPrefix(uuid.NewSHA1(uuid.NameSpaceOID, []byte(assignmentID+"\x00"+fmt.Sprint(attempt))).String(), "-"),
		NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
		FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		CastAuthority:  types.EngineeringCastAuthorityDelegated,
	}
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "delegated-cast-open:" + assignmentID + fmt.Sprintf(":%d", attempt), AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID, Objective: req.Objective},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	opened, err := rt.store.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		return AssignedEngineeringStart{}, err
	}
	// The durable open is committed. Do NOT run spawnBindActivate here — the
	// cell-commit path arms a deferred delegated_assignment_spawn_deadline wake
	// and the saga resumes post-commit (consensus precondition). The preflight
	// and capability digests recorded on the binding let the wake re-derive and
	// verify the exact spawn inputs.
	return AssignedEngineeringStart{Assignment: opened.Assignment}, nil
}

// resumeDelegatedCastAssignment re-drives the spawn/bind saga for a delegated
// cast whose durable open committed but whose bind never landed.
func (rt *Runtime) resumeDelegatedCastAssignment(ctx context.Context, assignment types.EngineeringAssignment, req DelegatedCastRequest) (AssignedEngineeringStart, error) {
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, assignment.Binding.SourceArtifactRef)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("delegated cast preflight resumed subject: %w", err)
	}
	if preflight.ArtifactRef != assignment.Binding.SourceArtifactRef ||
		"sha256:"+strings.TrimPrefix(preflight.SubjectDigest, "sha256:") != assignment.Binding.SubjectDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed delegated cast subject drifted from the committed binding")
	}
	opaque := deterministicDocumentAssignmentCapability(assignment.AssignmentID, assignment.Binding.Attempt)
	if store.DigestEngineeringOpaqueCapability(opaque) != assignment.Binding.CapabilityDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed delegated cast capability does not match the committed binding")
	}
	return rt.spawnBindActivateAssignment(ctx, assignment, preflight, opaque, OpenDocumentAssignmentRequest{
		Objective: req.Objective, Kind: req.Kind, CandidateID: req.CandidateID,
	})
}

// resumeAssignedEngineeringForDocument re-drives the spawn/bind saga for an
// assignment whose durable open committed but whose bind never landed. The
// deterministic capability re-derives the exact digest the binding committed.
func (rt *Runtime) resumeAssignedEngineeringForDocument(ctx context.Context, assignment types.EngineeringAssignment, req OpenDocumentAssignmentRequest) (AssignedEngineeringStart, error) {
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, assignment.Binding.SourceArtifactRef)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("preflight immutable assignment subject: %w", err)
	}
	if preflight.ArtifactRef != assignment.Binding.SourceArtifactRef ||
		"sha256:"+strings.TrimPrefix(preflight.SubjectDigest, "sha256:") != assignment.Binding.SubjectDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed assignment subject drifted from the committed binding")
	}
	opaque := deterministicDocumentAssignmentCapability(assignment.AssignmentID, assignment.Binding.Attempt)
	if store.DigestEngineeringOpaqueCapability(opaque) != assignment.Binding.CapabilityDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed assignment capability does not match the committed binding")
	}
	return rt.spawnBindActivateAssignment(ctx, assignment, preflight, opaque, req)
}

// spawnBindActivateAssignment is the shared post-open saga: spawn the capsule,
// mint the capability, bind the run, wake the actor. Every failure path cancels
// the durable open so a stranded assignment never blocks a later cast.
func (rt *Runtime) spawnBindActivateAssignment(ctx context.Context, assignment types.EngineeringAssignment, preflight capsule.SourcePreflight, opaque string, req OpenDocumentAssignmentRequest) (AssignedEngineeringStart, error) {
	binding := assignment.Binding
	ownerID, computerID, trajectoryID := binding.OwnerID, binding.ComputerID, binding.TrajectoryID
	assignmentID, attempt := assignment.AssignmentID, binding.Attempt
	agentID, workID, runID, capsuleID := binding.AssignedAgentID, binding.AssignedWorkItemID, "run:"+assignmentID, binding.CapsuleID
	cancelOpen := func(cause error) error {
		current, loadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr == nil && !current.Disposition.Terminal() {
			cancel := types.CancelEngineeringAssignmentRequest{CommandID: "co-super-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
				AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: current.LifecycleVersion, Reason: cause.Error()}
			cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
			_, _ = rt.store.CancelEngineeringAssignment(context.Background(), cancel)
		}
		return cause
	}
	spawnCtx, cancelSpawn := context.WithTimeout(ctx, 90*time.Second)
	defer cancelSpawn()
	spec := capsule.SpawnSpec{CapsuleID: capsuleID, OwnerRunID: runID,
		MemoryMax: engineeringAssignmentMemoryMax, CpuQuota: engineeringAssignmentCPUQuota, CpuPeriod: 100000, PidsMax: engineeringAssignmentPidsMax,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight.ArtifactRef, ExpectedSubjectDigest: preflight.SubjectDigest}
	if req.Kind == types.EngineeringAssignmentVerification {
		// The verifier's exact-binding mount is mandatory: the host installs
		// the operation's frozen bundle read-only at /selfdev/bundle with a
		// binding.json the in-cell inspect reads. A verification assignment
		// without a mountable frozen bundle has nothing to verify; fail
		// closed rather than spawn a verifier whose inspection capability is
		// unreachable.
		if rt.selfdevOperations == nil || rt.selfdevUpdaterRoot == "" {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("verification assignment requires the self-development operation store and updater root"))
		}
		operation, opErr := rt.selfdevOperations.GetByTrajectory(spawnCtx, computerID, trajectoryID)
		if opErr != nil {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("verification assignment cannot resolve its self-development operation: %w", opErr))
		}
		if operation.BundleDigest == "" ||
			(operation.State != selfdev.StateFrozen && operation.State != selfdev.StateVerified && operation.State != selfdev.StateAwaitingApproval) {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("verification assignment requires a frozen self-development bundle (state %s)", operation.State))
		}
		bundleDir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
		info, statErr := os.Stat(bundleDir)
		if statErr != nil || !info.IsDir() {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("verification assignment bundle directory unavailable: %v", statErr))
		}
		bindingJSON, _ := json.Marshal(map[string]string{"operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest})
		spec.VerifierBundleDir = bundleDir
		spec.VerifierBinding = string(bindingJSON)
	}
	var created *capsule.Capsule
	if rt.capsuleExecutor.HasCapsule(capsuleID) {
		// Resume after a crash between spawn and bind: the capsule already
		// exists; verify it is still active rather than respawning.
		diagnostics, diagErr := rt.capsuleExecutor.InspectCapsuleRaw(capsuleID)
		if diagErr != nil || diagnostics == nil {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("resumed assignment capsule is not inspectable: %v", diagErr))
		}
		created = &capsule.Capsule{ID: capsuleID, State: capsule.StateActive}
	} else {
		var spawnErr error
		created, spawnErr = rt.capsuleExecutor.Spawn(spawnCtx, spec)
		if spawnErr != nil {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("spawn assigned capsule after durable open: %w", spawnErr))
		}
	}
	cleanupCapsule := func(cause error) error {
		current, loadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr != nil {
			return fmt.Errorf("%w (load opened assignment for capsule cleanup: %v)", cause, loadErr)
		}
		intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(current.AssignmentID+"\x00pre-bind\x00"+cause.Error()))
		requested, fateErr := rt.store.SetEngineeringCapsuleDisposition(context.Background(), engineeringFateRequest(current, types.EngineeringCapsuleRevokeRequested, intent, ""))
		if fateErr != nil {
			return fmt.Errorf("%w (persist pre-bind capsule revoke intent: %v)", cause, fateErr)
		}
		_ = rt.capsuleExecutor.RevokeCapability(runID, opaque)
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			if destroyErr := rt.capsuleExecutor.ForceDestroy(context.Background(), capsuleID); destroyErr != nil {
				return fmt.Errorf("%w (destroy after pre-bind revoke intent: %v)", cause, destroyErr)
			}
		}
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			return fmt.Errorf("%w (pre-bind capsule continued after executor acknowledgement)", cause)
		}
		receipt, receiptErr := rt.capsuleExecutor.PersistRevocationReceipt(runID, requested.Assignment.Binding.CapabilityDigest, capsuleID, intent)
		if receiptErr != nil {
			return fmt.Errorf("%w (persist structured pre-bind revoke acknowledgement: %v)", cause, receiptErr)
		}
		fateAck, fateAckErr := engineeringFateAckRequest(requested.Assignment, types.EngineeringCapsuleRevoked, intent, receipt.ReceiptRef, "", "", receipt.OccurredAt, receipt.CapsuleAbsent)
		if fateAckErr != nil {
			return fmt.Errorf("%w (invalid revoke receipt occurred_at: %v)", cause, fateAckErr)
		}
		acked, fateErr := rt.store.SetEngineeringCapsuleDisposition(context.Background(), fateAck)
		if fateErr != nil {
			return fmt.Errorf("%w (persist pre-bind capsule revoke acknowledgement: %v)", cause, fateErr)
		}
		cancel := types.CancelEngineeringAssignmentRequest{CommandID: "co-super-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
			AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: acked.Assignment.LifecycleVersion, Reason: cause.Error()}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		if _, cancelErr := rt.store.CancelEngineeringAssignment(context.Background(), cancel); cancelErr != nil {
			return fmt.Errorf("%w (cancel pre-bind assignment after revoke ack: %v)", cause, cancelErr)
		}
		return cause
	}
	if created.ID != capsuleID || created.State != capsule.StateActive {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("assigned capsule acknowledgement mismatch"))
	}
	spawnedAt := time.Now().UTC()
	slot := "implementation"
	if req.Kind == types.EngineeringAssignmentVerification {
		slot = "verifier"
	}
	capability, err := rt.capsuleExecutor.MintCapabilityHandle(runID, capsule.RoleEngineering, capsuleID, opaque, 24*time.Hour, slot)
	grantedAt := time.Now().UTC()
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("mint exact assignment capability: %w", err))
	}
	compiledVerbs := make([]string, 0, len(capsule.RoleVerbSets[capsule.RoleEngineering]))
	for verb, allowed := range capsule.RoleVerbSets[capsule.RoleEngineering] {
		if allowed {
			compiledVerbs = append(compiledVerbs, verb)
		}
	}
	slices.Sort(compiledVerbs)
	actualVerbs := make([]string, 0, len(capability.Verbs))
	for verb, allowed := range capability.Verbs {
		if !allowed {
			return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability contains a disabled verb"))
		}
		actualVerbs = append(actualVerbs, verb)
	}
	slices.Sort(actualVerbs)
	if capability.AgentRole != capsule.RoleEngineering || capability.AgentRunID != runID || capability.CapsuleID != capsuleID || capability.TargetCapsule != capsuleID ||
		capability.Handle != opaque || capability.Slot != slot || !slices.Equal(actualVerbs, compiledVerbs) || len(capability.ExternalAccess) != 0 || strings.TrimSpace(capability.KeyID) == "" ||
		len(capability.Signature) == 0 || !capability.ExpiresAt.After(grantedAt) || capability.ExpiresAt.After(grantedAt.Add(24*time.Hour+time.Second)) {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability acknowledgement mismatch"))
	}
	capabilityBytes, err := json.Marshal(capability)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("digest minted assignment capability: %w", err))
	}
	grantAttestation := &types.EngineeringGrantPolicyAttestation{
		Role: string(capability.AgentRole), GrantedVerbs: actualVerbs,
		VerbSetDigest:          store.ComputeEngineeringGrantVerbSetDigest(actualVerbs),
		PolicyDigest:           store.ComputeEngineeringGrantPolicyDigest(string(capability.AgentRole), actualVerbs, binding.NetworkMode, binding.FilesystemMode, binding.Writable),
		SignedCapabilityDigest: objectgraph.SHA256(capabilityBytes), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true,
		SpawnedAt: spawnedAt, GrantedAt: grantedAt,
	}
	run := types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: agentID, RequestedByRunID: "", TrajectoryID: trajectoryID,
		AgentProfile: agentprofile.Engineering, AgentRole: agentprofile.Engineering, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPending, Prompt: req.Objective,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Engineering, runMetadataAgentRole: agentprofile.Engineering, runMetadataAgentID: agentID,
			runMetadataTrajectoryID: trajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID,
			"requested_by_agent_id": binding.ParentAgentID, "requested_by_profile": agentprofile.Engineering,
			"assignment_id": assignmentID, "assignment_attempt": attempt, "assignment_kind": string(req.Kind),
			runMetadataEngineeringSlot: slot,
			"assigned_work_item_id":    workID, "capsule_id": capsuleID,
			"parent_decision_id": binding.ParentDecisionID, "parent_control_id": binding.ParentControlID,
			"parent_work_item_id": binding.ParentWorkItemID, "scope_digest": binding.ScopeDigest, "request_digest": binding.RequestDigest,
			"capability_digest": binding.CapabilityDigest, "execution_handle_digest": binding.ExecutionHandleDigest, "subject_digest": binding.SubjectDigest,
			"source_artifact_ref": preflight.ArtifactRef, "source_candidate_id": req.CandidateID,
		},
	}
	if overlay := strings.TrimSpace(req.ModelPolicyOverlayID); overlay != "" {
		run.Metadata[modelpolicy.MetadataPolicyOverlayID] = overlay
	}
	run.Metadata = rt.modelPolicy.EnrichMetadata(ctx, ownerID, agentprofile.Engineering, run.Metadata)
	// Fail closed on policy errors like the Texture eval path: an unknown or
	// unresolvable overlay must not silently fall back to the default model.
	if policyErr := metadataStringValue(run.Metadata, modelpolicy.MetadataPolicyError); policyErr != "" {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("model policy overlay did not resolve: %s", policyErr))
	}
	if model := metadataStringValue(run.Metadata, modelpolicy.MetadataModel); model != "" {
		run.Metadata[runMetadataModel] = model
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "co-super-bind:" + assignmentID + fmt.Sprintf(":%d", attempt), OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: assignment.LifecycleVersion,
		RunID: runID, Run: run, OpaqueCapability: opaque, CapsuleID: capsuleID, GrantPolicyAttestation: grantAttestation,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(err)
	}
	bound, err := rt.store.BindEngineeringAssignment(ctx, bind)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("bind assigned Engineering activation: %w", err))
	}
	// The lifecycle Bind receipt is durable before this actor wake. No generic
	// lifecycle-Management refusal is removed and no pre-cutover mailbox is written.
	rt.activate(&run)
	return AssignedEngineeringStart{Assignment: bound.Assignment, Run: run}, nil
}

func runtimePersistentManagementControlID(metadata map[string]any, trajectoryID, workItemID string) string {
	raw, ok := metadata["lifecycle_control_bindings"]
	if !ok {
		return ""
	}
	var entries []any
	switch value := raw.(type) {
	case []any:
		entries = value
	case []map[string]any:
		entries = make([]any, len(value))
		for i := range value {
			entries[i] = value[i]
		}
	default:
		return ""
	}
	matched := ""
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		entryTrajectory, _ := entry["trajectory_id"].(string)
		entryWork, _ := entry["target_work_item_id"].(string)
		entryUpdate, _ := entry["update_id"].(string)
		if strings.TrimSpace(entryTrajectory) == trajectoryID && strings.TrimSpace(entryWork) == workItemID && strings.TrimSpace(entryUpdate) != "" {
			if matched != "" {
				return ""
			}
			matched = strings.TrimSpace(entryUpdate)
		}
	}
	return matched
}

func persistentManagementRunStateAllowedRuntime(state types.RunState) bool {
	return state == types.RunPending || state == types.RunRunning || state == types.RunPassivated
}

// reclaimSupersededAssignmentCapsules enforces one live assignment capsule at
// a time. Every prior assignment on this computer whose capsule disposition is
// not already revoked (and that is not the assignment being opened) is durably
// cancelled through the existing fate path, which revokes its capsule and
// releases admission budget before the new capsule spawns. Fail closed: a
// reclaim failure must not open a new assignment on a leaked budget.
func (rt *Runtime) reclaimSupersededAssignmentCapsules(ctx context.Context, parent types.RunRecord, currentAssignmentID string) error {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, parent.ComputerID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		// Only bound/live capsules hold admission budget. Unbound capsules were
		// never spawned and revoked capsules already released theirs.
		if assignment.AssignmentID == currentAssignmentID ||
			assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked ||
			assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound {
			continue
		}
		// Prior assignments may have been opened by a different persistent-Management
		// run identity (e.g. before a restart reactivated the actor). Reclaim
		// through the exact parent recorded on each assignment binding, not the
		// current caller, so the fate path's parent-identity check passes.
		reclaimParent := parent
		reclaimParent.RunID = assignment.Binding.ParentRunID
		reclaimParent.AgentID = assignment.Binding.ParentAgentID
		if assignment.Disposition.Terminal() && assignment.BoundRunID != "" {
			// A terminal assignment whose capsule was never revoked still holds
			// admission budget. cancelAssignedEngineering replays without revoking
			// for terminal dispositions, so revoke the capsule directly through
			// the fate path.
			reclaimed, err := rt.revokeAssignedCapsule(ctx, assignment,
				"terminal assignment capsule budget reclaimed for fresh assignment")
			if err != nil {
				return fmt.Errorf("reclaim terminal %s (capsule %s): %w",
					assignment.AssignmentID, assignment.Binding.CapsuleID, err)
			}
			log.Printf("runtime: reclaimed terminal assignment capsule %s (capsule %s) disposition=%s",
				assignment.AssignmentID, assignment.Binding.CapsuleID, reclaimed.CapsuleDisposition)
			continue
		}
		result, err := rt.cancelAssignedEngineering(ctx, reclaimParent, assignment.AssignmentID, assignment.Binding.Attempt,
			"superseded by a fresh implementation assignment; capsule budget reclaimed")
		if err != nil {
			return fmt.Errorf("reclaim %s (capsule %s): %w", assignment.AssignmentID, assignment.Binding.CapsuleID, err)
		}
		log.Printf("runtime: reclaimed superseded assignment capsule %s (capsule %s) disposition=%s",
			assignment.AssignmentID, assignment.Binding.CapsuleID, result.Assignment.CapsuleDisposition)
	}
	return nil
}
