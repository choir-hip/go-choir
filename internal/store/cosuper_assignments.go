package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	ErrCoSuperAssignmentCommandConflict = ErrLifecycleCommandConflict
	ErrCoSuperAssignmentInvalid         = errors.New("co-super assignment invalid transition")
)

const (
	ogKindCoSuperAssignment objectgraph.ObjectKind = "choir.co_super_assignment"
	ogKindCoSuperReport     objectgraph.ObjectKind = "choir.co_super_assignment_report"
	ogKindCoSuperCandidate  objectgraph.ObjectKind = "choir.co_super_subject_candidate"
	ogKindCoSuperCapability objectgraph.ObjectKind = "choir.co_super_capability_claim"
	ogKindCoSuperCapsule    objectgraph.ObjectKind = "choir.co_super_capsule_claim"
	ogKindCoSuperRunClaim   objectgraph.ObjectKind = "choir.co_super_run_claim"

	ogEdgeAssignmentTrajectory objectgraph.EdgeKind = "co_super_assignment_trajectory"
	ogEdgeAssignmentParent     objectgraph.EdgeKind = "co_super_assignment_parent"
	ogEdgeAssignmentParentRun  objectgraph.EdgeKind = "co_super_assignment_parent_run"
	ogEdgeAssignmentParentWork objectgraph.EdgeKind = "co_super_assignment_parent_work"
	ogEdgeAssignmentAgent      objectgraph.EdgeKind = "co_super_assignment_agent"
	ogEdgeAssignmentWork       objectgraph.EdgeKind = "co_super_assignment_work"
	ogEdgeAssignmentRun        objectgraph.EdgeKind = "co_super_assignment_run"
	ogEdgeReportAssignment     objectgraph.EdgeKind = "co_super_report_assignment"
	ogEdgeReportCandidate      objectgraph.EdgeKind = "co_super_report_candidate"
)

type coSuperAuthorityObjects struct {
	trajectory    objectgraph.Object
	trajectoryRec types.TrajectoryRecord
	parentAgent   objectgraph.Object
	parentRun     objectgraph.Object
	parentWork    objectgraph.Object
	assignedAgent objectgraph.Object
	assignedWork  objectgraph.Object
}

type coSuperRunClaim struct {
	RunID        string    `json:"loop_id"`
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	CreatedAt    time.Time `json:"created_at"`
}

type coSuperCapabilityClaim struct {
	CapabilityDigest string    `json:"capability_digest"`
	OwnerID          string    `json:"owner_id"`
	ComputerID       string    `json:"computer_id"`
	AssignmentID     string    `json:"assignment_id"`
	Attempt          uint64    `json:"attempt"`
	RunID            string    `json:"loop_id"`
	CreatedAt        time.Time `json:"created_at"`
}

type coSuperCapsuleClaim struct {
	CapsuleID    string    `json:"capsule_id"`
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	RunID        string    `json:"loop_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type coSuperLifecycleTransition struct {
	trajectory types.TrajectoryRecord
	seq        int64
	conditions []objectgraph.ObjectCondition
	objects    []objectgraph.Object
}

func coSuperAttemptKey(assignmentID string, attempt uint64) string {
	return strings.TrimSpace(assignmentID) + "\x00" + strconv.FormatUint(attempt, 10)
}

func DigestCoSuperOpaqueCapability(opaque string) string {
	return objectgraph.SHA256([]byte("choir.co-super.opaque-capability/v1\x00" + opaque))
}

func computeCoSuperCommandDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return objectgraph.SHA256(payload), nil
}

func ComputeOpenCoSuperAssignmentDigest(req types.OpenCoSuperAssignmentRequest) (string, error) {
	req.CommandDigest = ""
	return computeCoSuperCommandDigest(req)
}

func normalizeGrantAttestationForCommandDigest(att *types.CoSuperGrantPolicyAttestation) {
	if att == nil {
		return
	}
	att.Schema, att.AttestationRef = "", ""
	att.AssignmentID, att.Attempt, att.OwnerID, att.ComputerID, att.TrajectoryID = "", 0, "", "", ""
	att.RunID, att.CapsuleID, att.TargetCapsule = "", "", ""
	att.NetworkMode, att.FilesystemMode, att.Writable = "", "", false
	att.BindCommandID, att.BindEventID, att.ReducerSeq, att.RecordedAt = "", "", 0, time.Time{}
}

func normalizeExecutionAttestationForCommandDigest(att *types.CoSuperExecutionAttestation) {
	att.Schema, att.AttestationRef = "", ""
	att.AssignmentID, att.Attempt, att.OwnerID, att.ComputerID, att.TrajectoryID = "", 0, "", "", ""
	att.RunID, att.CapsuleID, att.ReportID = "", "", ""
	att.ReportCommandID, att.ReportEventID, att.ReducerSeq, att.RecordedAt = "", "", 0, time.Time{}
}

func normalizeFateStepForCommandDigest(step *types.CoSuperCapsuleFateStep) {
	if step == nil {
		return
	}
	step.Schema, step.StepRef = "", ""
	step.AssignmentID, step.Attempt, step.OwnerID, step.ComputerID, step.TrajectoryID = "", 0, "", "", ""
	step.RunID, step.CapsuleID, step.Disposition = "", "", ""
	step.CommandID, step.EventID, step.ReducerSeq = "", "", 0
	step.IntentRef, step.AckRef, step.AssignmentCapabilityDigest = "", "", ""
	step.RecordedAt = time.Time{}
}

func ComputeBindCoSuperAssignmentDigest(req types.BindCoSuperAssignmentRequest) (string, error) {
	capabilityDigest := DigestCoSuperOpaqueCapability(req.OpaqueCapability)
	req.CommandDigest, req.OpaqueCapability = "", ""
	if req.GrantPolicyAttestation != nil {
		copy := *req.GrantPolicyAttestation
		req.GrantPolicyAttestation = &copy
	}
	normalizeGrantAttestationForCommandDigest(req.GrantPolicyAttestation)
	return computeCoSuperCommandDigest(struct {
		Request          types.BindCoSuperAssignmentRequest `json:"request"`
		CapabilityDigest string                             `json:"capability_digest"`
	}{Request: req, CapabilityDigest: capabilityDigest})
}

func normalizeCoSuperReportForDigest(report types.CoSuperAssignmentReport) types.CoSuperAssignmentReport {
	report.Schema, report.AssignmentID = "", ""
	report.Attempt, report.OwnerID, report.ComputerID, report.TrajectoryID = 0, "", "", ""
	report.RunID, report.AssignedAgentID = "", ""
	report.Late, report.CertifiesOriginalSubject = false, false
	report.CandidateSubjectDigest, report.CandidateID = "", ""
	report.ExecutionAttestations = nil
	report.CreatedAt = time.Time{}
	return report
}

func ComputeRecordCoSuperAssignmentReportDigest(req types.RecordCoSuperAssignmentReportRequest) (string, error) {
	req.CommandDigest = ""
	req.ExpectedLifecycleVersion = 0
	req.Report = normalizeCoSuperReportForDigest(req.Report)
	req.ExecutionAttestations = append([]types.CoSuperExecutionAttestation(nil), req.ExecutionAttestations...)
	for i := range req.ExecutionAttestations {
		normalizeExecutionAttestationForCommandDigest(&req.ExecutionAttestations[i])
	}
	return computeCoSuperCommandDigest(req)
}

func ComputeCancelCoSuperAssignmentDigest(req types.CancelCoSuperAssignmentRequest) (string, error) {
	req.CommandDigest = ""
	// Optimistic lifecycle version is a transition precondition, not command
	// identity; exact retry after the terminal commit must reach its receipt.
	req.ExpectedLifecycleVersion = 0
	return computeCoSuperCommandDigest(req)
}

func ComputeSetCoSuperCapsuleDispositionDigest(req types.SetCoSuperCapsuleDispositionRequest) (string, error) {
	req.CommandDigest = ""
	if req.FateStep != nil {
		copy := *req.FateStep
		req.FateStep = &copy
	}
	normalizeFateStepForCommandDigest(req.FateStep)
	return computeCoSuperCommandDigest(req)
}

func validateCoSuperCommand(commandID, digest, assignmentID string, attempt uint64) error {
	if strings.TrimSpace(commandID) == "" || strings.TrimSpace(digest) == "" || strings.TrimSpace(assignmentID) == "" || attempt == 0 {
		return fmt.Errorf("co-super assignment: command_id, command_digest, assignment_id, and attempt are required: %w", ErrCoSuperAssignmentInvalid)
	}
	return nil
}

func requireCoSuperCommandDigest(got, want string, err error) error {
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != want {
		return fmt.Errorf("co-super assignment: command digest mismatch: %w", ErrCoSuperAssignmentCommandConflict)
	}
	return nil
}

func coSuperCompiledVerbs() []string {
	verbs := make([]string, 0, len(capsule.RoleVerbSets[capsule.RoleCoSuper]))
	for verb, allowed := range capsule.RoleVerbSets[capsule.RoleCoSuper] {
		if allowed {
			verbs = append(verbs, verb)
		}
	}
	slices.Sort(verbs)
	return verbs
}

func coSuperVerbSetDigest(verbs []string) string {
	payload, _ := json.Marshal(struct {
		Schema string   `json:"schema"`
		Role   string   `json:"role"`
		Verbs  []string `json:"verbs"`
	}{"choir.co_super_verb_set/v1", string(capsule.RoleCoSuper), verbs})
	return objectgraph.SHA256(payload)
}

func coSuperPolicyDigest(role string, verbs []string, networkMode, filesystemMode string, writable bool) string {
	payload, _ := json.Marshal(struct {
		Schema         string   `json:"schema"`
		Role           string   `json:"role"`
		Verbs          []string `json:"verbs"`
		NetworkMode    string   `json:"network_mode"`
		FilesystemMode string   `json:"filesystem_mode"`
		Writable       bool     `json:"writable"`
	}{"choir.co_super_grant_policy/v1", role, verbs, networkMode, filesystemMode, writable})
	return objectgraph.SHA256(payload)
}

func ComputeCoSuperGrantVerbSetDigest(verbs []string) string { return coSuperVerbSetDigest(verbs) }
func ComputeCoSuperGrantPolicyDigest(role string, verbs []string, networkMode, filesystemMode string, writable bool) string {
	return coSuperPolicyDigest(role, verbs, networkMode, filesystemMode, writable)
}

func grantAttestationRef(att types.CoSuperGrantPolicyAttestation) (string, error) {
	att.AttestationRef = ""
	payload, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	return "co-super-grant:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func executionAttestationRef(att types.CoSuperExecutionAttestation) (string, error) {
	att.AttestationRef = ""
	payload, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	return "co-super-execution:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func fateStepRef(step types.CoSuperCapsuleFateStep) (string, error) {
	step.StepRef = ""
	payload, err := json.Marshal(step)
	if err != nil {
		return "", err
	}
	return "co-super-fate:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func validCanonicalTime(value time.Time) bool { return !value.IsZero() && value.Location() == time.UTC }

func validateGrantPolicyAttestation(att types.CoSuperGrantPolicyAttestation, assignment types.CoSuperAssignment) error {
	verbs := coSuperCompiledVerbs()
	if att.Schema != types.CoSuperGrantPolicyAttestationSchemaV1 || att.AssignmentID != assignment.AssignmentID || att.Attempt != assignment.Binding.Attempt ||
		att.OwnerID != assignment.Binding.OwnerID || att.ComputerID != assignment.Binding.ComputerID || att.TrajectoryID != assignment.Binding.TrajectoryID ||
		att.RunID != assignment.BoundRunID || att.CapsuleID != assignment.Binding.CapsuleID || att.TargetCapsule != assignment.Binding.CapsuleID ||
		att.Role != string(capsule.RoleCoSuper) || !slices.Equal(att.GrantedVerbs, verbs) || att.VerbSetDigest != coSuperVerbSetDigest(verbs) ||
		att.PolicyDigest != coSuperPolicyDigest(att.Role, verbs, assignment.Binding.NetworkMode, assignment.Binding.FilesystemMode, assignment.Binding.Writable) ||
		!types.ValidSHA256Digest(att.SignedCapabilityDigest) || att.NetworkMode != assignment.Binding.NetworkMode ||
		att.FilesystemMode != assignment.Binding.FilesystemMode || att.Writable != assignment.Binding.Writable ||
		!att.SpawnAcknowledged || !att.ActiveAcknowledged || !att.GrantAcknowledged || !validCanonicalTime(att.SpawnedAt) || !validCanonicalTime(att.GrantedAt) || att.GrantedAt.Before(att.SpawnedAt) ||
		strings.TrimSpace(att.BindCommandID) == "" || att.BindEventID != att.BindCommandID+":1" || att.ReducerSeq <= 0 || !validCanonicalTime(att.RecordedAt) {
		return fmt.Errorf("co-super assignment: invalid runtime grant policy attestation: %w", ErrCoSuperAssignmentInvalid)
	}
	want, err := grantAttestationRef(att)
	if err != nil || att.AttestationRef != want {
		return fmt.Errorf("co-super assignment: grant attestation digest mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	return nil
}

func validTypedDigestRef(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && types.ValidSHA256Digest(strings.TrimPrefix(value, prefix))
}

func validateExecutionAttestations(atts []types.CoSuperExecutionAttestation, report types.CoSuperAssignmentReport, assignment types.CoSuperAssignment) error {
	if len(atts) != len(report.Commands) || len(atts) != len(report.ExecutorReceiptRefs) {
		return fmt.Errorf("co-super assignment: exact execution attestation cardinality required: %w", ErrCoSuperAssignmentInvalid)
	}
	seen := map[string]bool{}
	for i, att := range atts {
		command := report.Commands[i]
		if att.Schema != types.CoSuperExecutionAttestationSchemaV1 || att.AssignmentID != assignment.AssignmentID || att.Attempt != assignment.Binding.Attempt ||
			att.OwnerID != assignment.Binding.OwnerID || att.ComputerID != assignment.Binding.ComputerID || att.TrajectoryID != assignment.Binding.TrajectoryID ||
			att.RunID != assignment.BoundRunID || att.CapsuleID != assignment.Binding.CapsuleID || att.ReportID != report.ReportID ||
			att.CommandID != command.CommandID || att.CommandDigest != command.CommandDigest || att.ExitCode != command.ExitCode ||
			att.GrantedReceiptRef != report.ExecutorReceiptRefs[i] || !validTypedDigestRef(att.GrantedReceiptRef, "capsule-granted-exec:") || !att.Granted || !att.Frozen ||
			!types.ValidSHA256Digest(att.StdoutDigest) || !types.ValidSHA256Digest(att.StderrDigest) ||
			att.SourceSubjectDigest != assignment.Binding.SubjectDigest || att.FinalSubjectDigest != report.ObservedSubjectDigest ||
			att.WorktreeDigest != att.FinalSubjectDigest || !validCanonicalTime(att.OccurredAt) || strings.TrimSpace(att.ReportCommandID) == "" ||
			att.ReportEventID != att.ReportCommandID+":1" || att.ReducerSeq <= 0 || !validCanonicalTime(att.RecordedAt) || seen[att.AttestationRef] {
			return fmt.Errorf("co-super assignment: invalid runtime execution attestation: %w", ErrCoSuperAssignmentInvalid)
		}
		want, err := executionAttestationRef(att)
		if err != nil || att.AttestationRef != want {
			return fmt.Errorf("co-super assignment: execution attestation digest mismatch: %w", ErrCoSuperAssignmentInvalid)
		}
		seen[att.AttestationRef] = true
	}
	return nil
}

func validateFateHistory(history []types.CoSuperCapsuleFateStep, assignment types.CoSuperAssignment) error {
	seen := map[string]bool{}
	lastSeq := int64(0)
	lastDisposition := types.CoSuperCapsuleUnbound
	if assignment.GrantPolicyAttestation != nil {
		lastDisposition = types.CoSuperCapsuleActive
	} else if len(history) > 0 {
		switch history[0].Disposition {
		case types.CoSuperCapsuleFreezeRequested:
			lastDisposition = types.CoSuperCapsuleActive
		case types.CoSuperCapsuleFrozen:
			lastDisposition = types.CoSuperCapsuleFreezeRequested
		case types.CoSuperCapsuleRevokeRequested:
			if assignment.BoundRunID != "" {
				lastDisposition = types.CoSuperCapsuleActive
			}
		case types.CoSuperCapsuleRevoked:
			lastDisposition = types.CoSuperCapsuleRevokeRequested
		}
	}
	var previous *types.CoSuperCapsuleFateStep
	for i := range history {
		step := history[i]
		if step.Schema != types.CoSuperCapsuleFateStepSchemaV1 || step.AssignmentID != assignment.AssignmentID || step.Attempt != assignment.Binding.Attempt ||
			step.OwnerID != assignment.Binding.OwnerID || step.ComputerID != assignment.Binding.ComputerID || step.TrajectoryID != assignment.Binding.TrajectoryID ||
			step.RunID != assignment.BoundRunID || step.CapsuleID != assignment.Binding.CapsuleID || step.ReducerSeq <= lastSeq || step.EventID != step.CommandID+":1" ||
			strings.TrimSpace(step.CommandID) == "" || strings.TrimSpace(step.IntentRef) == "" || step.AssignmentCapabilityDigest != assignment.Binding.CapabilityDigest ||
			!validCanonicalTime(step.OccurredAt) || !validCanonicalTime(step.RecordedAt) || seen[step.StepRef] || !validCoSuperCapsuleTransition(lastDisposition, step.Disposition, step.IntentRef, step.AckRef) {
			return fmt.Errorf("co-super assignment: invalid append-only capsule fate history: %w", ErrCoSuperAssignmentInvalid)
		}
		switch step.Disposition {
		case types.CoSuperCapsuleFreezeRequested, types.CoSuperCapsuleRevokeRequested:
			prefix := "capsule-freeze-intent:"
			if step.Disposition == types.CoSuperCapsuleRevokeRequested {
				prefix = "capsule-revoke-intent:"
			}
			if !validTypedDigestRef(step.IntentRef, prefix) || step.SourceSubjectDigest != "" || step.FinalSubjectDigest != "" || step.CapsuleAbsent {
				return ErrCoSuperAssignmentInvalid
			}
		case types.CoSuperCapsuleFrozen:
			if previous == nil || previous.Disposition != types.CoSuperCapsuleFreezeRequested || previous.IntentRef != step.IntentRef || !validTypedDigestRef(step.IntentRef, "capsule-freeze-intent:") || !validTypedDigestRef(step.AckRef, "capsule-fate:") || !types.ValidSHA256Digest(step.SourceSubjectDigest) || !types.ValidSHA256Digest(step.FinalSubjectDigest) || step.CapsuleAbsent {
				return ErrCoSuperAssignmentInvalid
			}
		case types.CoSuperCapsuleRevoked:
			if previous == nil || previous.Disposition != types.CoSuperCapsuleRevokeRequested || previous.IntentRef != step.IntentRef || !validTypedDigestRef(step.IntentRef, "capsule-revoke-intent:") || !validTypedDigestRef(step.AckRef, "capsule-revoke:") || step.SourceSubjectDigest != "" || step.FinalSubjectDigest != "" || !step.CapsuleAbsent {
				return ErrCoSuperAssignmentInvalid
			}
		}
		want, err := fateStepRef(step)
		if err != nil || step.StepRef != want {
			return fmt.Errorf("co-super assignment: fate step digest mismatch: %w", ErrCoSuperAssignmentInvalid)
		}
		seen[step.StepRef], lastSeq, lastDisposition = true, step.ReducerSeq, step.Disposition
		previous = &history[i]
	}
	if len(history) > 0 {
		last := history[len(history)-1]
		if lastDisposition != assignment.CapsuleDisposition || last.IntentRef != assignment.CapsuleIntentRef || last.AckRef != assignment.CapsuleAckRef {
			return fmt.Errorf("co-super assignment: fate history/current projection mismatch: %w", ErrCoSuperAssignmentInvalid)
		}
	}
	return nil
}

func coSuperAssignmentMetadata(a types.CoSuperAssignment) map[string]any {
	return map[string]any{
		"assignment_id": a.AssignmentID, "attempt": a.Binding.Attempt,
		"computer_id": a.Binding.ComputerID, "trajectory_id": a.Binding.TrajectoryID,
		"parent_agent_id": a.Binding.ParentAgentID, "assigned_agent_id": a.Binding.AssignedAgentID,
		"parent_work_item_id": a.Binding.ParentWorkItemID, "assigned_work_item_id": a.Binding.AssignedWorkItemID,
		"assignment_kind": string(a.Binding.Kind), "disposition": string(a.Disposition),
		"capsule_id": a.Binding.CapsuleID, "network_mode": a.Binding.NetworkMode,
		"filesystem_mode": a.Binding.FilesystemMode, "coordination_contract_id": a.Binding.CoordinationContractID,
		"capsule_disposition": string(a.CapsuleDisposition), "lifecycle_version": a.LifecycleVersion,
	}
}

func coSuperReportMetadata(r types.CoSuperAssignmentReport) map[string]any {
	return map[string]any{
		"report_id": r.ReportID, "assignment_id": r.AssignmentID, "attempt": r.Attempt,
		"computer_id": r.ComputerID, "trajectory_id": r.TrajectoryID, "loop_id": r.RunID,
		"result": string(r.Result), "verdict": string(r.Verdict), "late": r.Late,
	}
}

func coSuperEdge(fromID, toID string, kind objectgraph.EdgeKind, now time.Time) (objectgraph.Edge, error) {
	metadata := json.RawMessage(`{}`)
	id, err := objectgraph.BuildEdgeID(fromID, toID, kind, metadata)
	if err != nil {
		return objectgraph.Edge{}, err
	}
	return objectgraph.Edge{EdgeID: id, FromID: fromID, ToID: toID, Kind: kind, Metadata: metadata, CreatedAt: now}, nil
}

func coSuperObjectCondition(obj objectgraph.Object) objectgraph.ObjectCondition {
	return objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash}
}

func metadataExactString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func persistentSuperControlBinding(metadata map[string]any, trajectoryID, workItemID, updateID string) bool {
	raw := metadata["lifecycle_control_bindings"]
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
		return false
	}
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		entryTrajectory, _ := entry["trajectory_id"].(string)
		entryWork, _ := entry["target_work_item_id"].(string)
		entryUpdate, _ := entry["update_id"].(string)
		if strings.TrimSpace(entryTrajectory) == trajectoryID && strings.TrimSpace(entryWork) == workItemID && strings.TrimSpace(entryUpdate) == updateID {
			return true
		}
	}
	return false
}

func persistentSuperRunStateAllowed(state types.RunState) bool {
	return state == types.RunPending || state == types.RunRunning || state == types.RunPassivated
}

// A late persistent-Super report may authenticate only the exact historical
// run that received its control. Terminal and passivated runs retain evidence
// authority, but never regain live execution authority.
func persistentSuperHistoricalReportRunStateAllowed(state types.RunState) bool {
	return state.Terminal() || state == types.RunPassivated
}

// requireCoSuperAssignmentAuthority proves the join between the lifecycle work
// graph and the exact non-lifecycle persistent Super control run. It never
// promotes that Super run or agent into lifecycle state.
func (s *Store) requireCoSuperParentAuthority(ctx context.Context, binding types.CoSuperAssignmentBinding) (coSuperAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return coSuperAuthorityObjects{}, fmt.Errorf("%w: %v", ErrCoSuperAssignmentInvalid, err)
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if trajectory.OwnerID != binding.OwnerID || trajectory.ComputerID != binding.ComputerID ||
		trajectory.TrajectoryID != binding.TrajectoryID || trajectory.Status != types.TrajectoryLive {
		return coSuperAuthorityObjects{}, ErrCoSuperAssignmentInvalid
	}
	parentAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.ParentAgentID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAgentObj)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentAgent.OwnerID != binding.OwnerID || parentAgent.ComputerID != binding.ComputerID ||
		parentAgent.AgentID != binding.ParentAgentID || parentAgent.Profile != "super" || parentAgent.Role != "super" ||
		parentAgent.ChannelID != binding.ParentAgentID || parentAgent.LifecycleVersion != 0 ||
		(parentAgent.ActiveRunID != "" && parentAgent.ActiveRunID != binding.ParentRunID) {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: exact non-lifecycle persistent Super unavailable: %w", ErrCoSuperAssignmentInvalid)
	}
	parentRunObj, err := s.getRunObjectByOwnerOG(ctx, binding.OwnerID, binding.ParentRunID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	parentRun, err := decodeLifecycleObject[types.RunRecord](parentRunObj)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentRunObj.ComputerID != "" || parentRun.OwnerID != binding.OwnerID || parentRun.ComputerID != binding.ComputerID ||
		parentRun.RunID != binding.ParentRunID || parentRun.AgentID != binding.ParentAgentID || parentRun.TrajectoryID != "" ||
		parentRun.AgentProfile != "super" || parentRun.AgentRole != "super" || !persistentSuperRunStateAllowed(parentRun.State) ||
		metadataExactString(parentRun.Metadata, "assignment_trajectory_id") != binding.TrajectoryID ||
		!persistentSuperControlBinding(parentRun.Metadata, binding.TrajectoryID, binding.ParentWorkItemID, binding.ParentControlID) {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: parent decision/control run binding mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	parentWorkObj, parentWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.ParentWorkItemID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentWork.OwnerID != binding.OwnerID || parentWork.ComputerID != binding.ComputerID ||
		parentWork.TrajectoryID != binding.TrajectoryID || parentWork.AssignedAgentID != binding.ParentAgentID ||
		parentWork.AuthorityProfile != "super" || parentWork.Status != types.WorkItemOpen {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: parent Super target work mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	return coSuperAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory, parentAgent: parentAgentObj,
		parentRun: parentRunObj, parentWork: parentWorkObj}, nil
}

// requireCoSuperHistoricalParentAuthority authenticates a delayed report
// against the immutable authority binding after the live parent scope has
// terminalized. State may advance, but identities and the delivered control
// binding embedded in the exact parent run may not change.
func (s *Store) requireCoSuperHistoricalParentAuthority(ctx context.Context, binding types.CoSuperAssignmentBinding) (coSuperAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return coSuperAuthorityObjects{}, fmt.Errorf("%w: %v", ErrCoSuperAssignmentInvalid, err)
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if trajectory.OwnerID != binding.OwnerID || trajectory.ComputerID != binding.ComputerID || trajectory.TrajectoryID != binding.TrajectoryID ||
		(trajectory.Status != types.TrajectoryLive && trajectory.Status != types.TrajectorySettled && trajectory.Status != types.TrajectoryCancelled) {
		return coSuperAuthorityObjects{}, ErrCoSuperAssignmentInvalid
	}
	parentAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.ParentAgentID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAgentObj)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentAgent.OwnerID != binding.OwnerID || parentAgent.ComputerID != binding.ComputerID || parentAgent.AgentID != binding.ParentAgentID ||
		parentAgent.Profile != "super" || parentAgent.Role != "super" || parentAgent.ChannelID != binding.ParentAgentID || parentAgent.LifecycleVersion != 0 {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: historical persistent Super identity mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	parentRunObj, err := s.getRunObjectByOwnerOG(ctx, binding.OwnerID, binding.ParentRunID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	parentRun, err := decodeLifecycleObject[types.RunRecord](parentRunObj)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentRunObj.ComputerID != "" || parentRun.OwnerID != binding.OwnerID || parentRun.ComputerID != binding.ComputerID ||
		parentRun.RunID != binding.ParentRunID || parentRun.AgentID != binding.ParentAgentID || parentRun.TrajectoryID != "" ||
		parentRun.AgentProfile != "super" || parentRun.AgentRole != "super" || !parentRun.State.Valid() ||
		metadataExactString(parentRun.Metadata, "assignment_trajectory_id") != binding.TrajectoryID ||
		!persistentSuperControlBinding(parentRun.Metadata, binding.TrajectoryID, binding.ParentWorkItemID, binding.ParentControlID) {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: historical parent control binding mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	parentWorkObj, parentWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.ParentWorkItemID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if parentWork.OwnerID != binding.OwnerID || parentWork.ComputerID != binding.ComputerID || parentWork.TrajectoryID != binding.TrajectoryID ||
		parentWork.AssignedAgentID != binding.ParentAgentID || parentWork.AuthorityProfile != "super" ||
		(parentWork.Status != types.WorkItemOpen && parentWork.Status != types.WorkItemCompleted && parentWork.Status != types.WorkItemCancelled && parentWork.Status != types.WorkItemRefused) {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: historical parent work binding mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	return coSuperAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory, parentAgent: parentAgentObj, parentRun: parentRunObj, parentWork: parentWorkObj}, nil
}

func (s *Store) requireCoSuperAssignmentAuthority(ctx context.Context, binding types.CoSuperAssignmentBinding) (coSuperAuthorityObjects, error) {
	authority, err := s.requireCoSuperParentAuthority(ctx, binding)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	assignedAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.AssignedAgentID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	assignedAgent, err := decodeLifecycleObject[types.AgentRecord](assignedAgentObj)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if assignedAgent.OwnerID != binding.OwnerID || assignedAgent.ComputerID != binding.ComputerID ||
		assignedAgent.AgentID != binding.AssignedAgentID || assignedAgent.Profile != "co-super" || assignedAgent.Role != "co-super" || assignedAgent.LifecycleVersion <= 0 {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: assigned lifecycle CoSuper agent mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	assignedWorkObj, assignedWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.AssignedWorkItemID)
	if err != nil {
		return coSuperAuthorityObjects{}, err
	}
	if assignedWork.OwnerID != binding.OwnerID || assignedWork.ComputerID != binding.ComputerID ||
		assignedWork.TrajectoryID != binding.TrajectoryID || assignedWork.AssignedAgentID != binding.AssignedAgentID ||
		assignedWork.AuthorityProfile != "co-super" || assignedWork.Status != types.WorkItemOpen ||
		metadataExactString(assignedWork.Details, "parent_loop_id") != binding.ParentRunID ||
		metadataExactString(assignedWork.Details, "parent_decision_id") != binding.ParentDecisionID ||
		metadataExactString(assignedWork.Details, "parent_control_id") != binding.ParentControlID ||
		metadataExactString(assignedWork.Details, "parent_work_item_id") != binding.ParentWorkItemID {
		return coSuperAuthorityObjects{}, fmt.Errorf("co-super assignment: assigned CoSuper work mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	authority.assignedAgent, authority.assignedWork = assignedAgentObj, assignedWorkObj
	return authority, nil
}

func coSuperParentAuthorityConditions(authority coSuperAuthorityObjects) []objectgraph.ObjectCondition {
	return []objectgraph.ObjectCondition{
		coSuperObjectCondition(authority.parentAgent), coSuperObjectCondition(authority.parentRun),
		coSuperObjectCondition(authority.parentWork),
	}
}

func coSuperAuthorityConditions(authority coSuperAuthorityObjects) []objectgraph.ObjectCondition {
	conditions := coSuperParentAuthorityConditions(authority)
	return append(conditions, coSuperObjectCondition(authority.assignedAgent), coSuperObjectCondition(authority.assignedWork))
}

func (s *Store) prepareCoSuperLifecycleTransition(ctx context.Context, trajectoryObj objectgraph.Object, trajectory types.TrajectoryRecord, now time.Time) (coSuperLifecycleTransition, error) {
	if trajectory.Status == types.TrajectoryLive {
		next := trajectory
		next.ReducerSeq++
		next.LifecycleVersion++
		next.UpdatedAt = now
		updated, err := lifecycleObject(ogKindTrajectory, next.OwnerID, next.ComputerID, next.TrajectoryID, next,
			lifecycleMetadata("trajectory_id", next.TrajectoryID, next.ComputerID, next.TrajectoryID, next.ReducerSeq), trajectoryObj.CreatedAt, now)
		if err != nil {
			return coSuperLifecycleTransition{}, err
		}
		return coSuperLifecycleTransition{
			trajectory: next, seq: next.ReducerSeq,
			conditions: []objectgraph.ObjectCondition{coSuperObjectCondition(trajectoryObj)},
			objects:    []objectgraph.Object{updated},
		}, nil
	}
	seq, sequenceObj, sequenceCondition, err := s.nextPostTerminalSequence(ctx, trajectory.OwnerID, trajectory.ComputerID, trajectory, now)
	if err != nil {
		return coSuperLifecycleTransition{}, err
	}
	return coSuperLifecycleTransition{
		trajectory: trajectory, seq: seq,
		conditions: []objectgraph.ObjectCondition{sequenceCondition}, objects: []objectgraph.Object{sequenceObj},
	}, nil
}

func (s *Store) buildCoSuperLifecycleEvent(now time.Time, assignment types.CoSuperAssignment, commandID, digest string, kind types.LifecycleEventKind, seq int64, artifactRefs, evidenceRefs []string, reason string) (types.LifecycleEvent, objectgraph.Object, error) {
	event := types.LifecycleEvent{
		Schema: types.CoSuperAssignmentSchemaV1, EventID: commandID + ":1",
		OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		TrajectoryID: assignment.Binding.TrajectoryID, WorkItemID: assignment.Binding.AssignedWorkItemID,
		RunID: assignment.BoundRunID, AgentID: assignment.Binding.AssignedAgentID,
		Kind: kind, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq,
		CommandID: commandID, CommandDigest: digest, ArtifactRefs: artifactRefs, EvidenceRefs: evidenceRefs,
		Reason: reason, CreatedAt: now,
	}
	obj, err := lifecycleObject(ogKindLifecycleEvent, event.OwnerID, event.ComputerID, event.EventID, event,
		lifecycleMetadata("event_id", event.EventID, event.ComputerID, event.TrajectoryID, seq), now, now)
	return event, obj, err
}

func (s *Store) replayCoSuperAssignmentCommand(ctx context.Context, ownerID, computerID, commandID, digest, assignmentID string, attempt uint64, reportID string) (types.CoSuperAssignmentCommandResult, bool, error) {
	lifecycleResult, found, err := s.replayLifecycleCommand(ctx, ownerID, computerID, commandID, digest)
	if !found || err != nil {
		return types.CoSuperAssignmentCommandResult{}, found, err
	}
	assignment, err := s.GetCoSuperAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, true, err
	}
	result := types.CoSuperAssignmentCommandResult{Receipt: lifecycleResult.Receipt, Assignment: assignment, Replay: true}
	if strings.TrimSpace(reportID) != "" {
		report, reportErr := s.GetCoSuperAssignmentReport(ctx, ownerID, computerID, reportID)
		if reportErr != nil {
			return types.CoSuperAssignmentCommandResult{}, true, reportErr
		}
		result.Report = &report
		if update, updateErr := s.GetLifecycleUpdate(ctx, ownerID, computerID, assignment.Binding.TrajectoryID,
			assignment.Binding.ParentAgentID, assignment.Binding.AssignedAgentID, report.ReportID); updateErr == nil {
			result.Update = &update
		} else if !errors.Is(updateErr, ErrNotFound) {
			return types.CoSuperAssignmentCommandResult{}, true, updateErr
		}
		if report.CandidateID != "" {
			obj, getErr := s.lifecycleGraph().GetObject(ctx, report.CandidateID)
			if getErr != nil {
				return types.CoSuperAssignmentCommandResult{}, true, getErr
			}
			candidate, decodeErr := decodeLifecycleObject[types.CoSuperSubjectCandidate](obj)
			if decodeErr != nil {
				return types.CoSuperAssignmentCommandResult{}, true, decodeErr
			}
			result.Candidate = &candidate
		}
	}
	return result, true, nil
}

// ReplayRecordedCoSuperAssignmentReport returns the original authenticated
// lifecycle receipt and current assignment projection without synthesizing a
// new report command after fate transitions.
func (s *Store) ReplayRecordedCoSuperAssignmentReport(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64, reportID, commandID string) (types.CoSuperAssignmentCommandResult, error) {
	assignment, err := s.GetCoSuperAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	report, err := s.GetCoSuperAssignmentReport(ctx, ownerID, computerID, reportID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if report.AssignmentID != assignmentID || report.Attempt != attempt {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	commandCanonicalID, err := lifecycleCanonicalID(ogKindLifecycleCmd, ownerID, computerID, commandID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	obj, err := s.lifecycleGraph().GetObject(ctx, commandCanonicalID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](obj)
	if err != nil || receipt.CommandID != commandID || receipt.Kind != types.LifecycleRecordCoSuperAssignment {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	result := types.CoSuperAssignmentCommandResult{Receipt: receipt, Assignment: assignment, Report: &report, Replay: true}
	if update, updateErr := s.GetLifecycleUpdate(ctx, ownerID, computerID, assignment.Binding.TrajectoryID,
		assignment.Binding.ParentAgentID, assignment.Binding.AssignedAgentID, report.ReportID); updateErr == nil {
		result.Update = &update
	} else if !errors.Is(updateErr, ErrNotFound) {
		return types.CoSuperAssignmentCommandResult{}, updateErr
	}
	if report.CandidateID != "" {
		candidateObj, getErr := s.lifecycleGraph().GetObject(ctx, report.CandidateID)
		if getErr != nil {
			return types.CoSuperAssignmentCommandResult{}, getErr
		}
		candidate, decodeErr := decodeLifecycleObject[types.CoSuperSubjectCandidate](candidateObj)
		if decodeErr != nil {
			return types.CoSuperAssignmentCommandResult{}, decodeErr
		}
		result.Candidate = &candidate
	}
	return result, nil
}

func (s *Store) commitCoSuperLifecycleCommand(ctx context.Context, transition coSuperLifecycleTransition, commandKind types.LifecycleCommandKind, eventKind types.LifecycleEventKind, commandID, digest string, assignment types.CoSuperAssignment, report *types.CoSuperAssignmentReport, candidate *types.CoSuperSubjectCandidate, reportID, reason string, artifactObjects []objectgraph.Object, conditions []objectgraph.ObjectCondition, edges []objectgraph.Edge, update *types.CoagentSourcePacket, evidenceRefs []string) (types.CoSuperAssignmentCommandResult, error) {
	now := assignment.UpdatedAt
	artifactRefs := make([]string, 0, len(artifactObjects))
	for _, obj := range artifactObjects {
		artifactRefs = append(artifactRefs, obj.CanonicalID)
	}
	event, eventObj, err := s.buildCoSuperLifecycleEvent(now, assignment, commandID, digest, eventKind, transition.seq, artifactRefs, evidenceRefs, reason)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		assignment.Binding.TrajectoryID, commandID, digest, commandKind, transition.seq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	conditions = append(transition.conditions, conditions...)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects := append(append([]objectgraph.Object{}, transition.objects...), artifactObjects...)
	objects = append(objects, eventObj, receiptObj)
	lifecycleResult, err := s.commitLifecycleTransition(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		commandID, digest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: transition.trajectory, Update: update, Events: []types.LifecycleEvent{event}}, edges...)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if lifecycleResult.Replay {
		replayed, _, replayErr := s.replayCoSuperAssignmentCommand(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
			commandID, digest, assignment.AssignmentID, assignment.Binding.Attempt, reportID)
		return replayed, replayErr
	}
	return types.CoSuperAssignmentCommandResult{Receipt: receipt, Assignment: assignment, Report: report, Candidate: candidate, Update: update}, nil
}

func (s *Store) OpenCoSuperAssignment(ctx context.Context, req types.OpenCoSuperAssignmentRequest) (types.CoSuperAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest, req.AssignmentID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.AssignmentID)
	if err := validateCoSuperCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if err := req.Binding.Validate(); err != nil {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrCoSuperAssignmentInvalid, err)
	}
	if strings.TrimSpace(req.AssignedAgent.AgentID) != req.Binding.AssignedAgentID ||
		strings.TrimSpace(req.AssignedWork.WorkItemID) != req.Binding.AssignedWorkItemID ||
		strings.TrimSpace(req.AssignedWork.AssignedAgentID) != req.Binding.AssignedAgentID ||
		strings.TrimSpace(req.AssignedWork.Objective) == "" {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	computedDigest, digestErr := ComputeOpenCoSuperAssignmentDigest(req)
	if err := requireCoSuperCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayCoSuperAssignmentCommand(ctx, req.Binding.OwnerID, req.Binding.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt, ""); found || err != nil {
		return replay, err
	}
	authority, err := s.requireCoSuperParentAuthority(ctx, req.Binding)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareCoSuperLifecycleTransition(ctx, authority.trajectory, authority.trajectoryRec, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}

	assignedAgent := req.AssignedAgent
	assignedAgent.AgentID, assignedAgent.OwnerID, assignedAgent.ComputerID = req.Binding.AssignedAgentID, req.Binding.OwnerID, req.Binding.ComputerID
	assignedAgent.ComputerID, assignedAgent.Profile, assignedAgent.Role = req.Binding.ComputerID, "co-super", "co-super"
	assignedAgent.ChannelID, assignedAgent.ActiveRunID = req.Binding.AssignedAgentID, ""
	assignedAgent.LifecycleVersion, assignedAgent.LastReducerSeq = 1, transition.seq
	assignedAgent.CreatedAt, assignedAgent.UpdatedAt = now, now
	assignedWork := req.AssignedWork
	assignedWork.WorkItemID, assignedWork.OwnerID, assignedWork.ComputerID = req.Binding.AssignedWorkItemID, req.Binding.OwnerID, req.Binding.ComputerID
	assignedWork.TrajectoryID, assignedWork.AssignedAgentID = req.Binding.TrajectoryID, req.Binding.AssignedAgentID
	assignedWork.Objective = strings.TrimSpace(assignedWork.Objective)
	assignedWork.AuthorityProfile, assignedWork.Status, assignedWork.ResultRef = "co-super", types.WorkItemOpen, ""
	assignedWork.ObjectiveFingerprint = objectgraph.SHA256([]byte(assignedWork.Objective))
	assignedWork.CreatedByRunID = req.Binding.ParentRunID
	assignedWork.Details = map[string]any{
		"assignment_id": req.AssignmentID, "attempt": req.Binding.Attempt, "assignment_kind": string(req.Binding.Kind),
		"parent_loop_id": req.Binding.ParentRunID, "parent_decision_id": req.Binding.ParentDecisionID,
		"parent_control_id": req.Binding.ParentControlID, "parent_work_item_id": req.Binding.ParentWorkItemID,
		"scope_digest": req.Binding.ScopeDigest, "subject_digest": req.Binding.SubjectDigest,
	}
	assignedWork.LifecycleVersion, assignedWork.LastReducerSeq = 1, transition.seq
	assignedWork.CreatedAt, assignedWork.UpdatedAt = now, now

	assignment := types.CoSuperAssignment{
		Schema: types.CoSuperAssignmentSchemaV1, AssignmentID: req.AssignmentID, Binding: req.Binding,
		Disposition: types.CoSuperAssignmentOpen, CapsuleDisposition: types.CoSuperCapsuleUnbound,
		LifecycleVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := assignment.Validate(); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignmentObj, err := lifecycleObject(ogKindCoSuperAssignment, req.Binding.OwnerID, req.Binding.ComputerID,
		coSuperAttemptKey(req.AssignmentID, req.Binding.Attempt), assignment, coSuperAssignmentMetadata(assignment), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	agentObj, err := lifecycleObject(ogKindAgent, req.Binding.OwnerID, req.Binding.ComputerID, assignedAgent.AgentID, assignedAgent,
		lifecycleMetadata("agent_id", assignedAgent.AgentID, req.Binding.ComputerID, req.Binding.TrajectoryID, transition.seq), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	workObj, err := lifecycleObject(ogKindWorkItem, req.Binding.OwnerID, req.Binding.ComputerID, assignedWork.WorkItemID, assignedWork,
		lifecycleMetadata("work_item_id", assignedWork.WorkItemID, req.Binding.ComputerID, req.Binding.TrajectoryID, transition.seq), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	targets := []struct {
		obj  objectgraph.Object
		kind objectgraph.EdgeKind
	}{
		{authority.trajectory, ogEdgeAssignmentTrajectory}, {authority.parentAgent, ogEdgeAssignmentParent},
		{authority.parentRun, ogEdgeAssignmentParentRun}, {authority.parentWork, ogEdgeAssignmentParentWork},
		{agentObj, ogEdgeAssignmentAgent}, {workObj, ogEdgeAssignmentWork},
	}
	edges := make([]objectgraph.Edge, 0, len(targets))
	for _, target := range targets {
		edge, edgeErr := coSuperEdge(assignmentObj.CanonicalID, target.obj.CanonicalID, target.kind, now)
		if edgeErr != nil {
			return types.CoSuperAssignmentCommandResult{}, edgeErr
		}
		edges = append(edges, edge)
	}
	conditions := coSuperParentAuthorityConditions(authority)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: assignmentObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: agentObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID})
	return s.commitCoSuperLifecycleCommand(ctx, transition, types.LifecycleOpenCoSuperAssignment, types.LifecycleCoSuperAssignmentOpened,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", "", []objectgraph.Object{assignmentObj, agentObj, workObj}, conditions, edges, nil, nil)
}

func (s *Store) getCoSuperAssignmentObject(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) (objectgraph.Object, types.CoSuperAssignment, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindCoSuperAssignment, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), coSuperAttemptKey(assignmentID, attempt))
	if err != nil {
		return objectgraph.Object{}, types.CoSuperAssignment{}, err
	}
	assignment, err := decodeLifecycleObject[types.CoSuperAssignment](obj)
	if err != nil {
		return objectgraph.Object{}, types.CoSuperAssignment{}, err
	}
	if assignment.Binding.OwnerID != strings.TrimSpace(ownerID) || assignment.Binding.ComputerID != strings.TrimSpace(computerID) ||
		assignment.AssignmentID != strings.TrimSpace(assignmentID) || assignment.Binding.Attempt != attempt {
		return objectgraph.Object{}, types.CoSuperAssignment{}, ErrNotFound
	}
	if err := assignment.Validate(); err != nil {
		return objectgraph.Object{}, types.CoSuperAssignment{}, err
	}
	if assignment.GrantPolicyAttestation != nil {
		if err := validateGrantPolicyAttestation(*assignment.GrantPolicyAttestation, assignment); err != nil {
			return objectgraph.Object{}, types.CoSuperAssignment{}, err
		}
	}
	if err := validateFateHistory(assignment.CapsuleFateHistory, assignment); err != nil {
		return objectgraph.Object{}, types.CoSuperAssignment{}, err
	}
	return obj, assignment, nil
}

func (s *Store) GetCoSuperAssignment(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) (types.CoSuperAssignment, error) {
	_, assignment, err := s.getCoSuperAssignmentObject(ctx, ownerID, computerID, assignmentID, attempt)
	return assignment, err
}

// ListCoSuperAssignmentsForComputer returns every non-tombstoned CoSuper
// assignment object in one computer, independent of the run-state metadata
// index or the open work-item projection. Boot recovery uses it to reconcile
// capsules for assignments bound before the run-state index existed.
func (s *Store) ListCoSuperAssignmentsForComputer(ctx context.Context, computerID string) ([]types.CoSuperAssignment, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("list CoSuper assignments for computer: computer_id is required")
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindCoSuperAssignment)
	if err != nil {
		return nil, fmt.Errorf("list CoSuper assignments for computer: %w", err)
	}
	out := make([]types.CoSuperAssignment, 0, len(objs))
	for _, obj := range objs {
		if obj.ComputerID != computerID || obj.Tombstone {
			continue
		}
		assignment, decodeErr := decodeLifecycleObject[types.CoSuperAssignment](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if assignment.Binding.ComputerID != computerID {
			continue
		}
		out = append(out, assignment)
	}
	slices.SortFunc(out, func(left, right types.CoSuperAssignment) int {
		if cmp := strings.Compare(left.Binding.TrajectoryID, right.Binding.TrajectoryID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(left.AssignmentID, right.AssignmentID); cmp != 0 {
			return cmp
		}
		return int(left.Binding.Attempt) - int(right.Binding.Attempt)
	})
	return out, nil
}

func (s *Store) ListCoSuperAssignments(ctx context.Context, ownerID, computerID, trajectoryID string) ([]types.CoSuperAssignment, error) {
	objects, err := s.ogListAllByMetadata(ctx, ogKindCoSuperAssignment, "trajectory_id", strings.TrimSpace(trajectoryID))
	if err != nil {
		return nil, err
	}
	out := make([]types.CoSuperAssignment, 0, len(objects))
	for _, obj := range objects {
		if obj.OwnerID != strings.TrimSpace(ownerID) || obj.ComputerID != strings.TrimSpace(computerID) || obj.Tombstone {
			continue
		}
		assignment, decodeErr := decodeLifecycleObject[types.CoSuperAssignment](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if assignment.Binding.TrajectoryID == strings.TrimSpace(trajectoryID) {
			out = append(out, assignment)
		}
	}
	slices.SortFunc(out, func(left, right types.CoSuperAssignment) int {
		if cmp := strings.Compare(left.AssignmentID, right.AssignmentID); cmp != 0 {
			return cmp
		}
		return int(left.Binding.Attempt) - int(right.Binding.Attempt)
	})
	return out, nil
}

func metadataExactUint64(metadata map[string]any, key string) uint64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case uint64:
		return value
	case int:
		if value > 0 {
			return uint64(value)
		}
	case int64:
		if value > 0 {
			return uint64(value)
		}
	case float64:
		if value > 0 && value == float64(uint64(value)) {
			return uint64(value)
		}
	case json.Number:
		parsed, _ := strconv.ParseUint(string(value), 10, 64)
		return parsed
	}
	return 0
}

func validateCoSuperAssignmentRun(assignment types.CoSuperAssignment, assignedAgent types.AgentRecord, run types.RunRecord) error {
	workIDs, bindingErr := lifecycleActivationWorkItemIDs(run.Metadata)
	coordinationID := metadataExactString(run.Metadata, "coordination_contract_id")
	coordinationDigest := metadataExactString(run.Metadata, "coordination_contract_digest")
	if bindingErr != nil || len(workIDs) != 1 || workIDs[0] != assignment.Binding.AssignedWorkItemID ||
		metadataExactString(run.Metadata, "lifecycle_work_item_id") != assignment.Binding.AssignedWorkItemID ||
		strings.TrimSpace(run.RunID) == "" || run.OwnerID != assignment.Binding.OwnerID || run.ComputerID != assignment.Binding.ComputerID ||
		run.TrajectoryID != assignment.Binding.TrajectoryID || run.AgentID != assignment.Binding.AssignedAgentID ||
		run.ChannelID != assignedAgent.ChannelID || run.AgentProfile != "co-super" || run.AgentRole != "co-super" || run.State != types.RunPending ||
		run.RequestedByRunID != assignment.Binding.ParentRunID || !run.CreatedAt.IsZero() || !run.UpdatedAt.IsZero() ||
		run.FinishedAt != nil || run.Result != "" || run.Error != "" ||
		metadataExactString(run.Metadata, "requested_by_agent_id") != assignment.Binding.ParentAgentID ||
		metadataExactString(run.Metadata, "requested_by_profile") != "super" ||
		metadataExactString(run.Metadata, "assignment_id") != assignment.AssignmentID ||
		metadataExactUint64(run.Metadata, "assignment_attempt") != assignment.Binding.Attempt ||
		metadataExactString(run.Metadata, "assignment_kind") != string(assignment.Binding.Kind) ||
		metadataExactString(run.Metadata, "assigned_work_item_id") != assignment.Binding.AssignedWorkItemID ||
		metadataExactString(run.Metadata, "parent_work_item_id") != assignment.Binding.ParentWorkItemID ||
		metadataExactString(run.Metadata, "parent_decision_id") != assignment.Binding.ParentDecisionID ||
		metadataExactString(run.Metadata, "parent_control_id") != assignment.Binding.ParentControlID ||
		metadataExactString(run.Metadata, "capsule_id") != assignment.Binding.CapsuleID ||
		metadataExactString(run.Metadata, "scope_digest") != assignment.Binding.ScopeDigest ||
		metadataExactString(run.Metadata, "request_digest") != assignment.Binding.RequestDigest ||
		metadataExactString(run.Metadata, "capability_digest") != assignment.Binding.CapabilityDigest ||
		(assignment.Binding.ExecutionHandleDigest != "" && metadataExactString(run.Metadata, "execution_handle_digest") != assignment.Binding.ExecutionHandleDigest) ||
		metadataExactString(run.Metadata, "subject_digest") != assignment.Binding.SubjectDigest ||
		metadataExactString(run.Metadata, "source_artifact_ref") != assignment.Binding.SourceArtifactRef ||
		metadataExactString(run.Metadata, "source_candidate_id") != assignment.Binding.SourceCandidateID ||
		coordinationID != assignment.Binding.CoordinationContractID || coordinationDigest != assignment.Binding.CoordinationContractDigest {
		return fmt.Errorf("co-super assignment: exact run/work/parent binding mismatch: %w", ErrCoSuperAssignmentInvalid)
	}
	return nil
}

func coSuperRunMetadata(run types.RunRecord, seq int64) map[string]any {
	meta := lifecycleMetadata("run_id", run.RunID, run.ComputerID, run.TrajectoryID, seq)
	meta["state"] = string(run.State)
	meta["agent_id"] = run.AgentID
	meta["agent_profile"] = run.AgentProfile
	meta["agent_role"] = run.AgentRole
	if !run.CreatedAt.IsZero() {
		meta["created_at"] = run.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !run.UpdatedAt.IsZero() {
		meta["updated_at"] = run.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return meta
}

func (s *Store) BindCoSuperAssignment(ctx context.Context, req types.BindCoSuperAssignmentRequest) (types.CoSuperAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.RunID, req.CapsuleID = strings.TrimSpace(req.RunID), strings.TrimSpace(req.CapsuleID)
	if req.RunID == "" {
		req.RunID = strings.TrimSpace(req.Run.RunID)
	}
	if err := validateCoSuperCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if strings.TrimSpace(req.OpaqueCapability) == "" || req.RunID == "" || req.RunID != strings.TrimSpace(req.Run.RunID) || req.ExpectedLifecycleVersion <= 0 {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	computedDigest, digestErr := ComputeBindCoSuperAssignmentDigest(req)
	if err := requireCoSuperCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayCoSuperAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, ""); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getCoSuperAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.Disposition != types.CoSuperAssignmentOpen ||
		DigestCoSuperOpaqueCapability(req.OpaqueCapability) != assignment.Binding.CapabilityDigest || req.CapsuleID != assignment.Binding.CapsuleID {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	authority, err := s.requireCoSuperAssignmentAuthority(ctx, assignment.Binding)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignedAgent, err := decodeLifecycleObject[types.AgentRecord](authority.assignedAgent)
	if err != nil || assignedAgent.ActiveRunID != "" {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	if err := validateCoSuperAssignmentRun(assignment, assignedAgent, req.Run); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareCoSuperLifecycleTransition(ctx, authority.trajectory, authority.trajectoryRec, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	run := req.Run
	run.RunID, run.CreatedAt, run.UpdatedAt, run.FinishedAt = req.RunID, now, now, nil
	run.Result, run.Error = "", ""
	runObj, err := lifecycleObject(ogKindRun, req.OwnerID, req.ComputerID, run.RunID, run,
		coSuperRunMetadata(run, transition.seq), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignedAgent.ActiveRunID = run.RunID
	assignedAgent.LifecycleVersion++
	assignedAgent.LastReducerSeq, assignedAgent.UpdatedAt = transition.seq, now
	assignedAgentObj, err := lifecycleObject(ogKindAgent, req.OwnerID, req.ComputerID, assignedAgent.AgentID, assignedAgent,
		lifecycleMetadata("agent_id", assignedAgent.AgentID, req.ComputerID, assignment.Binding.TrajectoryID, transition.seq), authority.assignedAgent.CreatedAt, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignment.Disposition, assignment.BoundRunID = types.CoSuperAssignmentBound, req.RunID
	assignment.CapsuleDisposition = types.CoSuperCapsuleActive
	assignment.LifecycleVersion++
	assignment.UpdatedAt = now
	if req.GrantPolicyAttestation != nil {
		att := *req.GrantPolicyAttestation
		att.Schema = types.CoSuperGrantPolicyAttestationSchemaV1
		att.AssignmentID, att.Attempt = assignment.AssignmentID, assignment.Binding.Attempt
		att.OwnerID, att.ComputerID, att.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
		att.RunID, att.CapsuleID, att.TargetCapsule = assignment.BoundRunID, assignment.Binding.CapsuleID, assignment.Binding.CapsuleID
		att.NetworkMode, att.FilesystemMode, att.Writable = assignment.Binding.NetworkMode, assignment.Binding.FilesystemMode, assignment.Binding.Writable
		att.BindCommandID, att.BindEventID, att.ReducerSeq, att.RecordedAt = req.CommandID, req.CommandID+":1", transition.seq, now
		att.AttestationRef = ""
		att.AttestationRef, err = grantAttestationRef(att)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment.GrantPolicyAttestation = &att
		if err := validateGrantPolicyAttestation(att, assignment); err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
	}
	updatedObj, err := lifecycleObject(ogKindCoSuperAssignment, req.OwnerID, req.ComputerID,
		coSuperAttemptKey(req.AssignmentID, req.Attempt), assignment, coSuperAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	runClaim := coSuperRunClaim{RunID: req.RunID, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, CreatedAt: now}
	runClaimObj, err := lifecycleObject(ogKindCoSuperRunClaim, req.OwnerID, req.ComputerID, req.RunID, runClaim,
		map[string]any{"run_id": req.RunID, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	capabilityClaim := coSuperCapabilityClaim{CapabilityDigest: assignment.Binding.CapabilityDigest, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, RunID: req.RunID, CreatedAt: now}
	capabilityObj, err := lifecycleObject(ogKindCoSuperCapability, req.OwnerID, req.ComputerID, assignment.Binding.CapabilityDigest, capabilityClaim,
		map[string]any{"capability_digest": assignment.Binding.CapabilityDigest, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	capsuleClaim := coSuperCapsuleClaim{CapsuleID: req.CapsuleID, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, RunID: req.RunID, CreatedAt: now}
	capsuleObj, err := lifecycleObject(ogKindCoSuperCapsule, req.OwnerID, req.ComputerID, req.CapsuleID, capsuleClaim,
		map[string]any{"capsule_id": req.CapsuleID, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	conditions := coSuperAuthorityConditions(authority)
	conditions = append(conditions, coSuperObjectCondition(assignmentObj), objectgraph.ObjectCondition{CanonicalID: runObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: runClaimObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: capabilityObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: capsuleObj.CanonicalID})
	runEdge, err := coSuperEdge(updatedObj.CanonicalID, runObj.CanonicalID, ogEdgeAssignmentRun, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	return s.commitCoSuperLifecycleCommand(ctx, transition, types.LifecycleBindCoSuperAssignment, types.LifecycleCoSuperAssignmentBound,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", "",
		[]objectgraph.Object{updatedObj, assignedAgentObj, runObj, runClaimObj, capabilityObj, capsuleObj}, conditions, []objectgraph.Edge{runEdge}, nil, nil)
}

func coSuperReportChangedSubject(report types.CoSuperAssignmentReport, originalDigest string) bool {
	if strings.TrimSpace(report.ObservedSubjectDigest) != strings.TrimSpace(originalDigest) {
		return true
	}
	for _, mutation := range report.Mutations {
		if mutation.SubjectBytesChanged {
			return true
		}
	}
	return false
}

func reducerAssignmentOutcome(report types.CoSuperAssignmentReport, assignment types.CoSuperAssignment, changed bool) types.CoSuperAssignmentDisposition {
	if report.Late || assignment.Disposition.Terminal() || report.Result == types.CoSuperResultPartial {
		return assignment.Disposition
	}
	if report.Result == types.CoSuperResultFailed || report.Result == types.CoSuperResultBlocked {
		return types.CoSuperAssignmentFailed
	}
	if assignment.Binding.Kind == types.CoSuperAssignmentVerification &&
		(report.Verdict != types.CoSuperVerdictPass || changed) {
		return types.CoSuperAssignmentFailed
	}
	if report.Result == types.CoSuperResultCompleted {
		return types.CoSuperAssignmentCompleted
	}
	return assignment.Disposition
}

func coSuperReportPacketPayload(report types.CoSuperAssignmentReport, cancellation bool) (types.CoagentSourcePacketPayload, string) {
	kind := "execution_result"
	if report.Result == types.CoSuperResultFailed || report.Result == types.CoSuperResultBlocked || cancellation {
		kind = "blocker"
	}
	notes := append([]string(nil), report.EvidenceRefs...)
	notes = append(notes, fmt.Sprintf("assignment_id=%s attempt=%d report_id=%s late=%t", report.AssignmentID, report.Attempt, report.ReportID, report.Late))
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          kind,
		Summary:       report.Summary,
		Claims:        []types.CoagentPacketClaim{{ClaimID: report.ReportID, Text: report.Summary, Stance: "supports"}},
		Notes:         notes,
	}
	return packet, report.Summary
}

func buildCoSuperReturnPacket(now time.Time, seq int64, assignment types.CoSuperAssignment, report types.CoSuperAssignmentReport, parentRun types.RunRecord, cancellation bool) (types.CoagentSourcePacket, objectgraph.Object, error) {
	packetPayload, content := coSuperReportPacketPayload(report, cancellation)
	payloadDigest, err := ComputeLifecycleUpdatePayloadDigest(packetPayload, content)
	if err != nil {
		return types.CoagentSourcePacket{}, objectgraph.Object{}, err
	}
	updateID := "assignment-report:" + report.ReportID
	deliveredRunID := ""
	var deliveredAt *time.Time
	if !parentRun.State.Terminal() {
		deliveredRunID = assignment.Binding.ParentRunID
		deliveredAt = &now
	}
	update := types.CoagentSourcePacket{
		UpdateID: updateID, ProducerUpdateID: report.ReportID,
		OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AgentID: assignment.Binding.AssignedAgentID, TargetAgentID: assignment.Binding.ParentAgentID,
		ChannelID: strings.TrimSpace(parentRun.ChannelID), MessageSeq: seq, TrajectoryID: assignment.Binding.TrajectoryID,
		Direction: types.LifecyclePacketDirectionProducerReport, ControlBindingID: assignment.Binding.ParentControlID,
		ProducerWorkItemID: assignment.Binding.AssignedWorkItemID, TargetWorkItemID: assignment.Binding.ParentWorkItemID,
		WorkItemID: assignment.Binding.AssignedWorkItemID, Role: "co-super", SourceRunID: assignment.BoundRunID,
		PayloadDigest: payloadDigest, Disposition: types.UpdatePending, LifecycleVersion: 1, ReducerSeq: seq,
		Packet: packetPayload, Content: content, CreatedAt: now,
		DeliveredToRunID: deliveredRunID, DeliveredAt: deliveredAt,
	}
	key := update.TrajectoryID + "\x00" + update.TargetAgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
	meta := lifecycleMetadata("update_id", update.UpdateID, update.ComputerID, update.TrajectoryID, seq)
	meta["producer_update_id"], meta["target_agent_id"] = update.ProducerUpdateID, update.TargetAgentID
	obj, err := lifecycleObject(ogKindWorkerUpdate, update.OwnerID, update.ComputerID, key, update, meta, now, now)
	return update, obj, err
}

func coSuperTerminalRunState(disposition types.CoSuperAssignmentDisposition) types.RunState {
	switch disposition {
	case types.CoSuperAssignmentCompleted:
		return types.RunCompleted
	case types.CoSuperAssignmentCancelled:
		return types.RunCancelled
	default:
		return types.RunFailed
	}
}

func coSuperTerminalWorkState(disposition types.CoSuperAssignmentDisposition) types.WorkItemStatus {
	switch disposition {
	case types.CoSuperAssignmentCompleted:
		return types.WorkItemCompleted
	case types.CoSuperAssignmentCancelled:
		return types.WorkItemCancelled
	default:
		return types.WorkItemRefused
	}
}

// projectCoSuperTerminal atomically closes the exact assigned work/run/agent.
// Capsule fate is deliberately absent: intent/effect/ack remains a separate
// sequenced authority after (or before cancellation) this lifecycle commit.
func (s *Store) projectCoSuperTerminal(ctx context.Context, assignment types.CoSuperAssignment, seq int64, now time.Time, resultRef, reason string) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	if !assignment.Disposition.Terminal() {
		return nil, nil, nil
	}
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.AssignedAgentID)
	if err != nil {
		return nil, nil, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		return nil, nil, err
	}
	workObj, work, err := s.lifecycleWorkObject(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.AssignedWorkItemID)
	if err != nil {
		return nil, nil, err
	}
	if work.AssignedAgentID != assignment.Binding.AssignedAgentID {
		return nil, nil, ErrCoSuperAssignmentInvalid
	}
	objects := []objectgraph.Object{}
	conditions := []objectgraph.ObjectCondition{}
	// Trajectory cancellation may already have projected the exact work and
	// agent terminal state after durable capsule revoke intent/ack. Treat that
	// projection as authenticated system cancellation rather than requiring a
	// now-live parent or attempting to reopen/overwrite it.
	if work.Status == types.WorkItemOpen {
		// The agent may already have released ActiveRunID because the bound run
		// completed without terminalizing the assignment. Refuse only when a
		// different live run occupies the slot.
		if agent.ActiveRunID != "" && agent.ActiveRunID != assignment.BoundRunID {
			return nil, nil, ErrCoSuperAssignmentInvalid
		}
		agent.ActiveRunID = ""
		agent.LifecycleVersion++
		agent.LastReducerSeq, agent.UpdatedAt = seq, now
		work.Status, work.ResultRef, work.Reason = coSuperTerminalWorkState(assignment.Disposition), resultRef, reason
		work.LifecycleVersion++
		work.LastReducerSeq, work.UpdatedAt = seq, now
		agentUpdated, buildErr := lifecycleObject(ogKindAgent, assignment.Binding.OwnerID, assignment.Binding.ComputerID, agent.AgentID, agent,
			lifecycleMetadata("agent_id", agent.AgentID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, seq), agentObj.CreatedAt, now)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		workUpdated, buildErr := lifecycleObject(ogKindWorkItem, assignment.Binding.OwnerID, assignment.Binding.ComputerID, work.WorkItemID, work,
			lifecycleMetadata("work_item_id", work.WorkItemID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, seq), workObj.CreatedAt, now)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		objects = append(objects, agentUpdated, workUpdated)
		conditions = append(conditions, coSuperObjectCondition(agentObj), coSuperObjectCondition(workObj))
	} else {
		if work.Status != types.WorkItemCancelled || assignment.Disposition != types.CoSuperAssignmentCancelled ||
			(agent.ActiveRunID != "" && agent.ActiveRunID != assignment.BoundRunID) {
			return nil, nil, ErrCoSuperAssignmentInvalid
		}
		if agent.ActiveRunID == assignment.BoundRunID && assignment.BoundRunID != "" {
			agent.ActiveRunID = ""
			agent.LifecycleVersion++
			agent.LastReducerSeq, agent.UpdatedAt = seq, now
			agentUpdated, buildErr := lifecycleObject(ogKindAgent, assignment.Binding.OwnerID, assignment.Binding.ComputerID, agent.AgentID, agent,
				lifecycleMetadata("agent_id", agent.AgentID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, seq), agentObj.CreatedAt, now)
			if buildErr != nil {
				return nil, nil, buildErr
			}
			objects = append(objects, agentUpdated)
			conditions = append(conditions, coSuperObjectCondition(agentObj))
		}
	}
	if assignment.BoundRunID == "" {
		return objects, conditions, nil
	}
	runObj, run, err := s.textureTurnRunObject(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.BoundRunID)
	if err != nil {
		return nil, nil, err
	}
	if run.RunID != assignment.BoundRunID || run.AgentID != assignment.Binding.AssignedAgentID || run.TrajectoryID != assignment.Binding.TrajectoryID {
		return nil, nil, ErrCoSuperAssignmentInvalid
	}
	if run.State.Terminal() {
		if run.State == coSuperTerminalRunState(assignment.Disposition) {
			return objects, conditions, nil
		}
		// The run already terminated with a different disposition (e.g. the
		// CoSuper finished without terminalizing the assignment). Re-project the
		// run to match the assignment fate below rather than failing the
		// cancellation.
	}
	run.State = coSuperTerminalRunState(assignment.Disposition)
	run.UpdatedAt, run.FinishedAt = now, &now
	if run.State == types.RunCompleted {
		run.Result, run.Error = resultRef, ""
	} else {
		run.Result, run.Error = "", reason
	}
	runUpdated, err := lifecycleObject(ogKindRun, assignment.Binding.OwnerID, assignment.Binding.ComputerID, run.RunID, run,
		coSuperRunMetadata(run, seq), runObj.CreatedAt, now)
	if err != nil {
		return nil, nil, err
	}
	return append(objects, runUpdated), append(conditions, coSuperObjectCondition(runObj)), nil
}

func (s *Store) RecordCoSuperAssignmentReport(ctx context.Context, req types.RecordCoSuperAssignmentReportRequest) (types.CoSuperAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	if err := validateCoSuperCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	computedDigest, digestErr := ComputeRecordCoSuperAssignmentReportDigest(req)
	if err := requireCoSuperCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayCoSuperAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, req.Report.ReportID); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getCoSuperAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.BoundRunID == "" || assignment.Disposition == types.CoSuperAssignmentOpen {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	_, intentErr := s.GetLifecycleCancellationIntent(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	cancellationIntended := intentErr == nil
	if intentErr != nil && !errors.Is(intentErr, ErrNotFound) {
		return types.CoSuperAssignmentCommandResult{}, intentErr
	}
	lateAuthority := cancellationIntended || assignment.Disposition.Terminal() || assignment.CapsuleDisposition == types.CoSuperCapsuleRevokeRequested || assignment.CapsuleDisposition == types.CoSuperCapsuleRevoked
	var parentAuthority coSuperAuthorityObjects
	if lateAuthority {
		parentAuthority, err = s.requireCoSuperHistoricalParentAuthority(ctx, assignment.Binding)
	} else {
		parentAuthority, err = s.requireCoSuperParentAuthority(ctx, assignment.Binding)
	}
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareCoSuperLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	report := req.Report
	// Attestations embedded in model-authored report JSON are never evidence.
	report.ExecutionAttestations = nil
	report.Schema, report.AssignmentID, report.Attempt = types.CoSuperAssignmentSchemaV1, assignment.AssignmentID, assignment.Binding.Attempt
	report.OwnerID, report.ComputerID, report.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
	report.RunID, report.AssignedAgentID = assignment.BoundRunID, assignment.Binding.AssignedAgentID
	report.Late = lateAuthority
	if report.Late {
		report.CertifiesOriginalSubject, report.CandidateSubjectDigest, report.CandidateID, report.CandidateArtifactRef = false, "", "", ""
		if report.Verdict == types.CoSuperVerdictPass {
			report.Verdict = types.CoSuperVerdictAbstain
		}
	}
	report.Summary = strings.TrimSpace(report.Summary)
	if report.Summary == "" {
		report.Summary = fmt.Sprintf("CoSuper assignment %s attempt %d reported %s", assignment.AssignmentID, assignment.Binding.Attempt, report.Result)
	}
	report.EvidenceRefs = normalizeLifecycleRefs(report.EvidenceRefs)
	report.CreatedAt = now
	changed := coSuperReportChangedSubject(report, assignment.Binding.SubjectDigest)
	var candidate *types.CoSuperSubjectCandidate
	var candidateObj objectgraph.Object
	candidateExists := false
	if changed && !report.Late {
		report.CandidateSubjectDigest = strings.TrimSpace(report.ObservedSubjectDigest)
		candidateKey := strings.Join([]string{
			assignment.Binding.SubjectDigest, report.CandidateSubjectDigest, assignment.AssignmentID,
			strconv.FormatUint(assignment.Binding.Attempt, 10), report.ReportID,
		}, "\x00")
		candidateID, buildErr := lifecycleCanonicalID(ogKindCoSuperCandidate, req.OwnerID, req.ComputerID, candidateKey)
		if buildErr != nil {
			return types.CoSuperAssignmentCommandResult{}, buildErr
		}
		report.CandidateID = candidateID
		created := types.CoSuperSubjectCandidate{Schema: types.CoSuperAssignmentSchemaV1, CandidateID: candidateID,
			OwnerID: req.OwnerID, ComputerID: req.ComputerID, TrajectoryID: assignment.Binding.TrajectoryID,
			AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
			OriginalSubjectDigest: assignment.Binding.SubjectDigest, SubjectDigest: report.CandidateSubjectDigest,
			SourceReportID: report.ReportID, ArtifactRef: report.CandidateArtifactRef, CreatedAt: now}
		existing, getErr := s.lifecycleGraph().GetObject(ctx, candidateID)
		if getErr == nil {
			decoded, decodeErr := decodeLifecycleObject[types.CoSuperSubjectCandidate](existing)
			if decodeErr != nil || decoded.OwnerID != req.OwnerID || decoded.ComputerID != req.ComputerID || decoded.SubjectDigest != report.CandidateSubjectDigest || decoded.ArtifactRef != report.CandidateArtifactRef {
				return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
			}
			candidate, candidateObj, candidateExists = &decoded, existing, true
		} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
			return types.CoSuperAssignmentCommandResult{}, getErr
		} else {
			candidate = &created
			candidateObj, buildErr = lifecycleObject(ogKindCoSuperCandidate, req.OwnerID, req.ComputerID, candidateKey, created,
				map[string]any{"candidate_id": candidateID, "computer_id": req.ComputerID, "trajectory_id": assignment.Binding.TrajectoryID,
					"subject_digest": report.CandidateSubjectDigest}, now, now)
			if buildErr != nil {
				return types.CoSuperAssignmentCommandResult{}, buildErr
			}
		}
	}
	// Certification is reducer-derived. Any authored value is ignored.
	report.CertifiesOriginalSubject = assignment.Binding.Kind == types.CoSuperAssignmentVerification && report.Verdict == types.CoSuperVerdictPass &&
		report.Result == types.CoSuperResultCompleted && !report.Late && !changed
	if len(req.ExecutionAttestations) > 0 && (assignment.GrantPolicyAttestation == nil || report.Late || report.Result == types.CoSuperResultPartial) {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("co-super assignment: execution attestations cannot retro-certify old, late, or partial evidence: %w", ErrCoSuperAssignmentInvalid)
	}
	if assignment.GrantPolicyAttestation != nil && !report.Late && report.Result != types.CoSuperResultPartial {
		commandCount := len(report.Commands)
		if (commandCount == 0 && (len(report.ExecutorReceiptRefs) != 0 || len(req.ExecutionAttestations) != 0)) ||
			(commandCount > 0 && (len(report.ExecutorReceiptRefs) != commandCount || len(req.ExecutionAttestations) != commandCount)) {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("co-super assignment: timely grant-attested command evidence requires exact receipt and attestation cardinality: %w", ErrCoSuperAssignmentInvalid)
		}
	}
	if len(req.ExecutionAttestations) > 0 {
		report.ExecutionAttestations = append([]types.CoSuperExecutionAttestation(nil), req.ExecutionAttestations...)
		for i := range report.ExecutionAttestations {
			att := &report.ExecutionAttestations[i]
			att.Schema = types.CoSuperExecutionAttestationSchemaV1
			att.AssignmentID, att.Attempt = assignment.AssignmentID, assignment.Binding.Attempt
			att.OwnerID, att.ComputerID, att.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
			att.RunID, att.CapsuleID, att.ReportID = assignment.BoundRunID, assignment.Binding.CapsuleID, report.ReportID
			att.ReportCommandID, att.ReportEventID, att.ReducerSeq, att.RecordedAt = req.CommandID, req.CommandID+":1", transition.seq, now
			att.AttestationRef = ""
			att.AttestationRef, err = executionAttestationRef(*att)
			if err != nil {
				return types.CoSuperAssignmentCommandResult{}, err
			}
		}
		if err := validateExecutionAttestations(report.ExecutionAttestations, report, assignment); err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
	}
	if err := report.ValidateAgainst(assignment); err != nil {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrCoSuperAssignmentInvalid, err)
	}
	reportObj, err := lifecycleObject(ogKindCoSuperReport, req.OwnerID, req.ComputerID, report.ReportID, report, coSuperReportMetadata(report), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignment.ReportRefs = append(append([]string(nil), assignment.ReportRefs...), reportObj.CanonicalID)
	previousDisposition := assignment.Disposition
	if !report.Late {
		assignment.Disposition = reducerAssignmentOutcome(report, assignment, changed)
	}
	if assignment.Disposition != previousDisposition && assignment.Disposition.Terminal() {
		assignment.DispositionReason = "reducer-derived from report " + report.ReportID
		assignment.TerminalAt = &now
	}
	assignment.LifecycleVersion++
	assignment.UpdatedAt = now
	if err := assignment.Validate(); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	updatedAssignmentObj, err := lifecycleObject(ogKindCoSuperAssignment, req.OwnerID, req.ComputerID,
		coSuperAttemptKey(req.AssignmentID, req.Attempt), assignment, coSuperAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	conditions := append(coSuperParentAuthorityConditions(parentAuthority), coSuperObjectCondition(assignmentObj),
		objectgraph.ObjectCondition{CanonicalID: reportObj.CanonicalID})
	if report.Late && parentAuthority.trajectoryRec.Status != types.TrajectoryLive {
		// A live transition already CASes the trajectory object. Post-terminal
		// evidence advances the separate terminal sequence, so retain the exact
		// historical trajectory hash as an additional authority condition.
		conditions = append(conditions, coSuperObjectCondition(parentAuthority.trajectory))
	}
	objects := []objectgraph.Object{updatedAssignmentObj, reportObj}
	var update *types.CoagentSourcePacket
	if !report.Late {
		parentRun, decodeErr := decodeLifecycleObject[types.RunRecord](parentAuthority.parentRun)
		if decodeErr != nil {
			return types.CoSuperAssignmentCommandResult{}, decodeErr
		}
		created, updateObj, updateErr := buildCoSuperReturnPacket(now, transition.seq, assignment, report, parentRun, false)
		if updateErr != nil {
			return types.CoSuperAssignmentCommandResult{}, updateErr
		}
		update = &created
		objects = append(objects, updateObj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID})
	}
	if assignment.Disposition.Terminal() && !report.Late {
		projectionObjects, projectionConditions, projectionErr := s.projectCoSuperTerminal(ctx, assignment, transition.seq, now, report.ReportID, assignment.DispositionReason)
		if projectionErr != nil {
			return types.CoSuperAssignmentCommandResult{}, projectionErr
		}
		objects = append(objects, projectionObjects...)
		conditions = append(conditions, projectionConditions...)
	}
	assignmentEdge, err := coSuperEdge(reportObj.CanonicalID, updatedAssignmentObj.CanonicalID, ogEdgeReportAssignment, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	edges := []objectgraph.Edge{assignmentEdge}
	if candidate != nil {
		if candidateExists {
			conditions = append(conditions, coSuperObjectCondition(candidateObj))
		} else {
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: candidateObj.CanonicalID})
			objects = append(objects, candidateObj)
		}
		candidateEdge, edgeErr := coSuperEdge(reportObj.CanonicalID, candidateObj.CanonicalID, ogEdgeReportCandidate, now)
		if edgeErr != nil {
			return types.CoSuperAssignmentCommandResult{}, edgeErr
		}
		edges = append(edges, candidateEdge)
	}
	return s.commitCoSuperLifecycleCommand(ctx, transition, types.LifecycleRecordCoSuperAssignment, types.LifecycleCoSuperAssignmentReported,
		req.CommandID, req.CommandDigest, assignment, &report, candidate, report.ReportID, assignment.DispositionReason,
		objects, conditions, edges, update, report.EvidenceRefs)
}

func (s *Store) CancelCoSuperAssignment(ctx context.Context, req types.CancelCoSuperAssignmentRequest) (types.CoSuperAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.Reason = strings.TrimSpace(req.Reason)
	if err := validateCoSuperCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 || req.Reason == "" {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	computedDigest, digestErr := ComputeCancelCoSuperAssignmentDigest(req)
	if err := requireCoSuperCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	cancelReportID := "cancel-report:" + objectgraph.SHA256([]byte(strings.Join([]string{req.AssignmentID, strconv.FormatUint(req.Attempt, 10), req.CommandID}, "\x00")))
	if replay, found, err := s.replayCoSuperAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, cancelReportID); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getCoSuperAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.Disposition.Terminal() {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	// Once exact revoke acknowledgement is durable, cancellation is system fate
	// completion. It authenticates the immutable assignment/control/run join and
	// never depends on the mutable parent Super activation remaining live.
	var parentAuthority coSuperAuthorityObjects
	if assignment.CapsuleDisposition == types.CoSuperCapsuleRevoked {
		parentAuthority, err = s.requireCoSuperHistoricalParentAuthority(ctx, assignment.Binding)
	} else {
		parentAuthority, err = s.requireCoSuperParentAuthority(ctx, assignment.Binding)
	}
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareCoSuperLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignment.Disposition, assignment.DispositionReason = types.CoSuperAssignmentCancelled, req.Reason
	assignment.LifecycleVersion++
	assignment.UpdatedAt, assignment.TerminalAt = now, &now
	reportID := cancelReportID
	verdict := types.CoSuperVerdictNone
	if assignment.Binding.Kind == types.CoSuperAssignmentVerification {
		verdict = types.CoSuperVerdictAbstain
	}
	report := types.CoSuperAssignmentReport{
		Schema: types.CoSuperAssignmentSchemaV1, ReportID: reportID, AssignmentID: assignment.AssignmentID,
		Attempt: assignment.Binding.Attempt, OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		TrajectoryID: assignment.Binding.TrajectoryID, RunID: assignment.BoundRunID, AssignedAgentID: assignment.Binding.AssignedAgentID,
		Result: types.CoSuperResultFailed, Verdict: verdict, ObservedSubjectDigest: assignment.Binding.SubjectDigest,
		Late: true, Summary: req.Reason + "; assignment cancelled; late results are evidence-only and cannot reopen work", CreatedAt: now,
	}
	if err := report.ValidateAgainst(assignment); err != nil {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrCoSuperAssignmentInvalid, err)
	}
	reportObj, err := lifecycleObject(ogKindCoSuperReport, req.OwnerID, req.ComputerID, report.ReportID, report, coSuperReportMetadata(report), now, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignment.ReportRefs = append(append([]string(nil), assignment.ReportRefs...), reportObj.CanonicalID)
	if err := assignment.Validate(); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	updatedObj, err := lifecycleObject(ogKindCoSuperAssignment, req.OwnerID, req.ComputerID,
		coSuperAttemptKey(req.AssignmentID, req.Attempt), assignment, coSuperAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	parentRun, decodeErr := decodeLifecycleObject[types.RunRecord](parentAuthority.parentRun)
	if decodeErr != nil {
		return types.CoSuperAssignmentCommandResult{}, decodeErr
	}
	update, updateObj, err := buildCoSuperReturnPacket(now, transition.seq, assignment, report, parentRun, true)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	projectionObjects, projectionConditions, err := s.projectCoSuperTerminal(ctx, assignment, transition.seq, now, report.ReportID, req.Reason)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	objects := []objectgraph.Object{updatedObj, reportObj, updateObj}
	objects = append(objects, projectionObjects...)
	conditions := append(coSuperParentAuthorityConditions(parentAuthority), coSuperObjectCondition(assignmentObj),
		objectgraph.ObjectCondition{CanonicalID: reportObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID})
	conditions = append(conditions, projectionConditions...)
	return s.commitCoSuperLifecycleCommand(ctx, transition, types.LifecycleCancelCoSuperAssignment, types.LifecycleCoSuperAssignmentCancelled,
		req.CommandID, req.CommandDigest, assignment, &report, nil, report.ReportID, req.Reason, objects,
		conditions, nil, &update, nil)
}

func validCoSuperCapsuleTransition(current, next types.CoSuperCapsuleDisposition, intentRef, ackRef string) bool {
	switch next {
	case types.CoSuperCapsuleFreezeRequested:
		return current == types.CoSuperCapsuleActive && intentRef != "" && ackRef == ""
	case types.CoSuperCapsuleFrozen:
		return current == types.CoSuperCapsuleFreezeRequested && intentRef != "" && ackRef != ""
	case types.CoSuperCapsuleRevokeRequested:
		return (current == types.CoSuperCapsuleUnbound || current == types.CoSuperCapsuleActive || current == types.CoSuperCapsuleFreezeRequested || current == types.CoSuperCapsuleFrozen) && intentRef != "" && ackRef == ""
	case types.CoSuperCapsuleRevoked:
		return current == types.CoSuperCapsuleRevokeRequested && intentRef != "" && strings.HasPrefix(ackRef, "capsule-revoke:sha256:") &&
			types.ValidSHA256Digest(strings.TrimPrefix(ackRef, "capsule-revoke:"))
	default:
		return false
	}
}

func (s *Store) SetCoSuperCapsuleDisposition(ctx context.Context, req types.SetCoSuperCapsuleDispositionRequest) (types.CoSuperAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.IntentRef, req.AckRef = strings.TrimSpace(req.IntentRef), strings.TrimSpace(req.AckRef)
	if err := validateCoSuperCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	computedDigest, digestErr := ComputeSetCoSuperCapsuleDispositionDigest(req)
	if err := requireCoSuperCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayCoSuperAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, ""); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getCoSuperAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion ||
		(assignment.BoundRunID == "" && assignment.Disposition != types.CoSuperAssignmentOpen) ||
		!validCoSuperCapsuleTransition(assignment.CapsuleDisposition, req.Disposition, req.IntentRef, req.AckRef) {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	if (req.Disposition == types.CoSuperCapsuleFrozen || req.Disposition == types.CoSuperCapsuleRevoked) && req.IntentRef != assignment.CapsuleIntentRef {
		return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
	}
	if (assignment.GrantPolicyAttestation != nil || len(assignment.CapsuleFateHistory) > 0) && req.FateStep == nil {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("co-super assignment: fate-attested assignment requires one fate step per transition: %w", ErrCoSuperAssignmentInvalid)
	}
	if _, intentErr := s.GetLifecycleCancellationIntent(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID); intentErr == nil {
		if req.Disposition != types.CoSuperCapsuleRevokeRequested && req.Disposition != types.CoSuperCapsuleRevoked {
			return types.CoSuperAssignmentCommandResult{}, ErrCoSuperAssignmentInvalid
		}
	} else if !errors.Is(intentErr, ErrNotFound) {
		return types.CoSuperAssignmentCommandResult{}, intentErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareCoSuperLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	assignment.CapsuleDisposition, assignment.CapsuleIntentRef, assignment.CapsuleAckRef = req.Disposition, req.IntentRef, req.AckRef
	assignment.LifecycleVersion++
	assignment.UpdatedAt = now
	if req.FateStep != nil {
		step := *req.FateStep
		step.Schema = types.CoSuperCapsuleFateStepSchemaV1
		step.AssignmentID, step.Attempt = assignment.AssignmentID, assignment.Binding.Attempt
		step.OwnerID, step.ComputerID, step.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
		step.RunID, step.CapsuleID, step.Disposition = assignment.BoundRunID, assignment.Binding.CapsuleID, req.Disposition
		step.CommandID, step.EventID, step.ReducerSeq = req.CommandID, req.CommandID+":1", transition.seq
		step.IntentRef, step.AckRef, step.AssignmentCapabilityDigest = req.IntentRef, req.AckRef, assignment.Binding.CapabilityDigest
		if step.OccurredAt.IsZero() {
			step.OccurredAt = now
		}
		step.RecordedAt = now
		step.StepRef = ""
		step.StepRef, err = fateStepRef(step)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment.CapsuleFateHistory = append(append([]types.CoSuperCapsuleFateStep(nil), assignment.CapsuleFateHistory...), step)
	}
	if err := assignment.Validate(); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if err := validateFateHistory(assignment.CapsuleFateHistory, assignment); err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	updatedObj, err := lifecycleObject(ogKindCoSuperAssignment, req.OwnerID, req.ComputerID,
		coSuperAttemptKey(req.AssignmentID, req.Attempt), assignment, coSuperAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	evidenceRefs := []string{req.IntentRef}
	if req.AckRef != "" {
		evidenceRefs = append(evidenceRefs, req.AckRef)
	}
	return s.commitCoSuperLifecycleCommand(ctx, transition, types.LifecycleSetCoSuperCapsuleDisposition, types.LifecycleCoSuperCapsuleDispositionSet,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", string(req.Disposition), []objectgraph.Object{updatedObj},
		[]objectgraph.ObjectCondition{coSuperObjectCondition(assignmentObj)}, nil, nil, evidenceRefs)
}

func (s *Store) GetCoSuperAssignmentReport(ctx context.Context, ownerID, computerID, reportID string) (types.CoSuperAssignmentReport, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindCoSuperReport, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(reportID))
	if err != nil {
		return types.CoSuperAssignmentReport{}, err
	}
	report, err := decodeLifecycleObject[types.CoSuperAssignmentReport](obj)
	if err != nil {
		return types.CoSuperAssignmentReport{}, err
	}
	if report.OwnerID != strings.TrimSpace(ownerID) || report.ComputerID != strings.TrimSpace(computerID) || report.ReportID != strings.TrimSpace(reportID) {
		return types.CoSuperAssignmentReport{}, ErrNotFound
	}
	return report, nil
}

func (s *Store) GetCoSuperSubjectCandidate(ctx context.Context, ownerID, computerID, candidateID string) (types.CoSuperSubjectCandidate, error) {
	obj, err := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(candidateID))
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return types.CoSuperSubjectCandidate{}, ErrNotFound
		}
		return types.CoSuperSubjectCandidate{}, err
	}
	candidate, err := decodeLifecycleObject[types.CoSuperSubjectCandidate](obj)
	if err != nil {
		return types.CoSuperSubjectCandidate{}, err
	}
	if candidate.CandidateID != strings.TrimSpace(candidateID) || candidate.OwnerID != strings.TrimSpace(ownerID) || candidate.ComputerID != strings.TrimSpace(computerID) {
		return types.CoSuperSubjectCandidate{}, ErrNotFound
	}
	return candidate, nil
}
