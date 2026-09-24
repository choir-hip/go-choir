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

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	ErrEngineeringAssignmentCommandConflict = ErrLifecycleCommandConflict
	ErrEngineeringAssignmentInvalid         = errors.New("co-super assignment invalid transition")
)

const (
	ogKindEngineeringAssignment objectgraph.ObjectKind = "choir.co_super_assignment"
	ogKindEngineeringReport     objectgraph.ObjectKind = "choir.co_super_assignment_report"
	ogKindEngineeringCandidate  objectgraph.ObjectKind = "choir.co_super_subject_candidate"
	ogKindEngineeringCapability objectgraph.ObjectKind = "choir.co_super_capability_claim"
	ogKindEngineeringCapsule    objectgraph.ObjectKind = "choir.co_super_capsule_claim"
	ogKindEngineeringRunClaim   objectgraph.ObjectKind = "choir.co_super_run_claim"

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

type engineeringAuthorityObjects struct {
	trajectory    objectgraph.Object
	trajectoryRec types.TrajectoryRecord
	parentAgent   objectgraph.Object
	parentRun     objectgraph.Object
	parentWork    objectgraph.Object
	// parentRevision carries the owner-authored revision that is the parent
	// authority for document-driven bindings (ParentRunID == "").
	parentRevision objectgraph.Object
	assignedAgent  objectgraph.Object
	assignedWork   objectgraph.Object
}

type engineeringRunClaim struct {
	RunID        string    `json:"loop_id"`
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	CreatedAt    time.Time `json:"created_at"`
}

type engineeringCapabilityClaim struct {
	CapabilityDigest string    `json:"capability_digest"`
	OwnerID          string    `json:"owner_id"`
	ComputerID       string    `json:"computer_id"`
	AssignmentID     string    `json:"assignment_id"`
	Attempt          uint64    `json:"attempt"`
	RunID            string    `json:"loop_id"`
	CreatedAt        time.Time `json:"created_at"`
}

type engineeringCapsuleClaim struct {
	CapsuleID    string    `json:"capsule_id"`
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	RunID        string    `json:"loop_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type engineeringLifecycleTransition struct {
	trajectory types.TrajectoryRecord
	seq        int64
	conditions []objectgraph.ObjectCondition
	objects    []objectgraph.Object
}

func engineeringAttemptKey(assignmentID string, attempt uint64) string {
	return strings.TrimSpace(assignmentID) + "\x00" + strconv.FormatUint(attempt, 10)
}

func DigestEngineeringOpaqueCapability(opaque string) string {
	return objectgraph.SHA256([]byte("choir.co-super.opaque-capability/v1\x00" + opaque))
}

func computeEngineeringCommandDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return objectgraph.SHA256(payload), nil
}

func ComputeOpenEngineeringAssignmentDigest(req types.OpenEngineeringAssignmentRequest) (string, error) {
	req.CommandDigest = ""
	return computeEngineeringCommandDigest(req)
}

func normalizeGrantAttestationForCommandDigest(att *types.EngineeringGrantPolicyAttestation) {
	if att == nil {
		return
	}
	att.Schema, att.AttestationRef = "", ""
	att.AssignmentID, att.Attempt, att.OwnerID, att.ComputerID, att.TrajectoryID = "", 0, "", "", ""
	att.RunID, att.CapsuleID, att.TargetCapsule = "", "", ""
	att.NetworkMode, att.FilesystemMode, att.Writable = "", "", false
	att.BindCommandID, att.BindEventID, att.ReducerSeq, att.RecordedAt = "", "", 0, time.Time{}
}

func normalizeExecutionAttestationForCommandDigest(att *types.EngineeringExecutionAttestation) {
	att.Schema, att.AttestationRef = "", ""
	att.AssignmentID, att.Attempt, att.OwnerID, att.ComputerID, att.TrajectoryID = "", 0, "", "", ""
	att.RunID, att.CapsuleID, att.ReportID = "", "", ""
	att.ReportCommandID, att.ReportEventID, att.ReducerSeq, att.RecordedAt = "", "", 0, time.Time{}
}

func normalizeFateStepForCommandDigest(step *types.EngineeringCapsuleFateStep) {
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

func ComputeBindEngineeringAssignmentDigest(req types.BindEngineeringAssignmentRequest) (string, error) {
	capabilityDigest := DigestEngineeringOpaqueCapability(req.OpaqueCapability)
	req.CommandDigest, req.OpaqueCapability = "", ""
	if req.GrantPolicyAttestation != nil {
		copy := *req.GrantPolicyAttestation
		req.GrantPolicyAttestation = &copy
	}
	normalizeGrantAttestationForCommandDigest(req.GrantPolicyAttestation)
	return computeEngineeringCommandDigest(struct {
		Request          types.BindEngineeringAssignmentRequest `json:"request"`
		CapabilityDigest string                                 `json:"capability_digest"`
	}{Request: req, CapabilityDigest: capabilityDigest})
}

func normalizeEngineeringReportForDigest(report types.EngineeringAssignmentReport) types.EngineeringAssignmentReport {
	report.Schema, report.ReportID, report.AssignmentID = "", "", ""
	report.Attempt, report.OwnerID, report.ComputerID, report.TrajectoryID = 0, "", "", ""
	report.RunID, report.AssignedAgentID = "", ""
	report.Late, report.CertifiesOriginalSubject = false, false
	report.CandidateSubjectDigest, report.CandidateID = "", ""
	report.ExecutionAttestations = nil
	report.CreatedAt = time.Time{}
	// v1 settlement identity: excluded metadata and reducer derivations never
	// enter command identity, so a reworded identical proposition replays the
	// original command receipt instead of conflicting on prose. The terminal
	// proposition digest (slot gate) decides semantic conflicts separately.
	report.Summary = ""
	report.CandidateArtifactRef = ""
	report.PropositionDigest, report.RecordCommandID = "", ""
	return report
}

func ComputeRecordEngineeringAssignmentReportDigest(req types.RecordEngineeringAssignmentReportRequest) (string, error) {
	req.CommandDigest = ""
	req.ExpectedLifecycleVersion = 0
	req.Report = normalizeEngineeringReportForDigest(req.Report)
	req.ExecutionAttestations = append([]types.EngineeringExecutionAttestation(nil), req.ExecutionAttestations...)
	for i := range req.ExecutionAttestations {
		normalizeExecutionAttestationForCommandDigest(&req.ExecutionAttestations[i])
	}
	return computeEngineeringCommandDigest(req)
}

// TerminalPropositionV1 is the frozen version domain for terminal settlement
// identity (settlement gate item 3). Every proposition digest input is
// prefixed with it; the receipt is docs/evidence/choir-rlm-settlement-item3-define-2026-09-09.md.
const TerminalPropositionV1 = "choir:terminal-proposition:v1"

// terminalPropositionCommandV1 is one ordered command claim: digest, declared
// execution reference, and exit code. CommandID is derivation, never input.
type terminalPropositionCommandV1 struct {
	CommandDigest string `json:"command_digest"`
	ExecutionRef  string `json:"execution_ref"`
	ExitCode      int    `json:"exit_code"`
}

// terminalPropositionOutputV1 is one ordered output claim: kind and content
// digest. OutputID and Ref are deterministic derivations of execution_ref+kind,
// never digest inputs.
type terminalPropositionOutputV1 struct {
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
}

// terminalPropositionV1 is the frozen canonical proposition: exactly the
// submitted-claim content that settles terminal truth, plus the pinned
// pre-execution subject belief. Ordered sequences keep submission order;
// evidence is a key-sorted set. Excluded: provider/transport/retry/batch/model
// metadata, summary and prose, timestamps, late flag, schema tags, report/command
// identifiers, mutations/attestations/candidate-discovery overlays (all bound
// post-reservation and validated by field rules, never digested), and the
// proposition digest itself.
type terminalPropositionV1 struct {
	Version               string                         `json:"v"`
	Result                string                         `json:"result"`
	Verdict               string                         `json:"verdict"`
	ObservedSubjectDigest string                         `json:"observed_subject_digest"`
	Commands              []terminalPropositionCommandV1 `json:"commands"`
	Outputs               []terminalPropositionOutputV1  `json:"outputs"`
	EvidenceRefs          []string                       `json:"evidence_refs"`
}

// ComputeTerminalPropositionDigest reduces a submitted terminal report to its
// v1 proposition digest. pinnedSubjectDigest is the attempt's binding subject
// digest (the pre-execution belief); the submitted observed value is validated
// elsewhere and never enters identity, so post-reservation ack overlays cannot
// move the digest. Attestations, mutations, and candidate discovery are
// post-reservation overlays: validated by exact field rules, never digested.
// Nil slices normalize to empty (a missing list and an empty list are the same
// proposition). Any error fails closed.
func ComputeTerminalPropositionDigest(pinnedSubjectDigest string, result types.EngineeringAssignmentResultKind, verdict types.EngineeringAssignmentVerdict, commands []types.EngineeringRecordedCommand, outputs []types.EngineeringRecordedOutput, evidenceRefs []string) (string, error) {
	pinnedSubjectDigest = strings.ToLower(strings.TrimSpace(pinnedSubjectDigest))
	if !types.ValidSHA256Digest(pinnedSubjectDigest) {
		return "", fmt.Errorf("terminal proposition: pinned subject digest is required: %w", ErrEngineeringAssignmentInvalid)
	}
	prop := terminalPropositionV1{
		Version:               TerminalPropositionV1,
		Result:                string(result),
		Verdict:               string(verdict),
		ObservedSubjectDigest: pinnedSubjectDigest,
		Commands:              make([]terminalPropositionCommandV1, 0, len(commands)),
		Outputs:               make([]terminalPropositionOutputV1, 0, len(outputs)),
		EvidenceRefs:          make([]string, 0, len(evidenceRefs)),
	}
	for _, command := range commands {
		digest := strings.ToLower(strings.TrimSpace(command.CommandDigest))
		if !types.ValidSHA256Digest(digest) {
			return "", fmt.Errorf("terminal proposition: command digest is required: %w", ErrEngineeringAssignmentInvalid)
		}
		prop.Commands = append(prop.Commands, terminalPropositionCommandV1{
			CommandDigest: digest,
			ExecutionRef:  strings.TrimSpace(command.ExecutionRef),
			ExitCode:      command.ExitCode,
		})
	}
	for _, output := range outputs {
		digest := strings.ToLower(strings.TrimSpace(output.Digest))
		if !types.ValidSHA256Digest(digest) {
			return "", fmt.Errorf("terminal proposition: output digest is required: %w", ErrEngineeringAssignmentInvalid)
		}
		prop.Outputs = append(prop.Outputs, terminalPropositionOutputV1{
			Kind:   strings.TrimSpace(output.Kind),
			Digest: digest,
		})
	}
	seenEvidence := make(map[string]struct{}, len(evidenceRefs))
	for _, ref := range evidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, exists := seenEvidence[ref]; !exists {
			seenEvidence[ref] = struct{}{}
			prop.EvidenceRefs = append(prop.EvidenceRefs, ref)
		}
	}
	slices.Sort(prop.EvidenceRefs)
	payload, err := json.Marshal(prop)
	if err != nil {
		return "", err
	}
	return objectgraph.SHA256([]byte(TerminalPropositionV1 + "\x00" + string(payload))), nil
}

// TerminalReportID derives the terminal report identifier from its reservation
// slot and proposition digest. It is derivation, never digest input: a
// provider-fresh resubmission of the same proposition replays the same ID.
func TerminalReportID(ownerID, computerID, assignmentID string, attempt uint64, propositionDigest string) string {
	slot := strings.Join([]string{
		strings.TrimSpace(ownerID), strings.TrimSpace(computerID),
		strings.TrimSpace(assignmentID), strconv.FormatUint(attempt, 10),
		propositionDigest,
	}, "\x00")
	return "report:" + objectgraph.SHA256([]byte(TerminalPropositionV1+"\x00"+slot))
}

func ComputeCancelEngineeringAssignmentDigest(req types.CancelEngineeringAssignmentRequest) (string, error) {
	req.CommandDigest = ""
	// Optimistic lifecycle version is a transition precondition, not command
	// identity; exact retry after the terminal commit must reach its receipt.
	req.ExpectedLifecycleVersion = 0
	return computeEngineeringCommandDigest(req)
}

func ComputeSetEngineeringCapsuleDispositionDigest(req types.SetEngineeringCapsuleDispositionRequest) (string, error) {
	req.CommandDigest = ""
	if req.FateStep != nil {
		copy := *req.FateStep
		req.FateStep = &copy
	}
	normalizeFateStepForCommandDigest(req.FateStep)
	return computeEngineeringCommandDigest(req)
}

func validateEngineeringCommand(commandID, digest, assignmentID string, attempt uint64) error {
	if strings.TrimSpace(commandID) == "" || strings.TrimSpace(digest) == "" || strings.TrimSpace(assignmentID) == "" || attempt == 0 {
		return fmt.Errorf("co-super assignment: command_id, command_digest, assignment_id, and attempt are required: %w", ErrEngineeringAssignmentInvalid)
	}
	return nil
}

func requireEngineeringCommandDigest(got, want string, err error) error {
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != want {
		return fmt.Errorf("co-super assignment: command digest mismatch: %w", ErrEngineeringAssignmentCommandConflict)
	}
	return nil
}

func engineeringCompiledVerbs() []string {
	verbs := make([]string, 0, len(capsule.RoleVerbSets[capsule.RoleEngineering]))
	for verb, allowed := range capsule.RoleVerbSets[capsule.RoleEngineering] {
		if allowed {
			verbs = append(verbs, verb)
		}
	}
	slices.Sort(verbs)
	return verbs
}

func engineeringVerbSetDigest(verbs []string) string {
	payload, _ := json.Marshal(struct {
		Schema string   `json:"schema"`
		Role   string   `json:"role"`
		Verbs  []string `json:"verbs"`
	}{"choir.co_super_verb_set/v1", string(capsule.RoleEngineering), verbs})
	return objectgraph.SHA256(payload)
}

func engineeringPolicyDigest(role string, verbs []string, networkMode, filesystemMode string, writable bool) string {
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

func ComputeEngineeringGrantVerbSetDigest(verbs []string) string {
	return engineeringVerbSetDigest(verbs)
}
func ComputeEngineeringGrantPolicyDigest(role string, verbs []string, networkMode, filesystemMode string, writable bool) string {
	return engineeringPolicyDigest(role, verbs, networkMode, filesystemMode, writable)
}

func grantAttestationRef(att types.EngineeringGrantPolicyAttestation) (string, error) {
	att.AttestationRef = ""
	payload, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	return "co-super-grant:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func executionAttestationRef(att types.EngineeringExecutionAttestation) (string, error) {
	att.AttestationRef = ""
	payload, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	return "co-super-execution:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func fateStepRef(step types.EngineeringCapsuleFateStep) (string, error) {
	step.StepRef = ""
	payload, err := json.Marshal(step)
	if err != nil {
		return "", err
	}
	return "co-super-fate:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func validCanonicalTime(value time.Time) bool { return !value.IsZero() && value.Location() == time.UTC }

func validateGrantPolicyAttestation(att types.EngineeringGrantPolicyAttestation, assignment types.EngineeringAssignment) error {
	verbs := engineeringCompiledVerbs()
	if att.Schema != types.EngineeringGrantPolicyAttestationSchemaV1 || att.AssignmentID != assignment.AssignmentID || att.Attempt != assignment.Binding.Attempt ||
		att.OwnerID != assignment.Binding.OwnerID || att.ComputerID != assignment.Binding.ComputerID || att.TrajectoryID != assignment.Binding.TrajectoryID ||
		att.RunID != assignment.BoundRunID || att.CapsuleID != assignment.Binding.CapsuleID || att.TargetCapsule != assignment.Binding.CapsuleID ||
		att.Role != string(capsule.RoleEngineering) || !slices.Equal(att.GrantedVerbs, verbs) || att.VerbSetDigest != engineeringVerbSetDigest(verbs) ||
		att.PolicyDigest != engineeringPolicyDigest(att.Role, verbs, assignment.Binding.NetworkMode, assignment.Binding.FilesystemMode, assignment.Binding.Writable) ||
		!types.ValidSHA256Digest(att.SignedCapabilityDigest) || att.NetworkMode != assignment.Binding.NetworkMode ||
		att.FilesystemMode != assignment.Binding.FilesystemMode || att.Writable != assignment.Binding.Writable ||
		!att.SpawnAcknowledged || !att.ActiveAcknowledged || !att.GrantAcknowledged || !validCanonicalTime(att.SpawnedAt) || !validCanonicalTime(att.GrantedAt) || att.GrantedAt.Before(att.SpawnedAt) ||
		strings.TrimSpace(att.BindCommandID) == "" || att.BindEventID != att.BindCommandID+":1" || att.ReducerSeq <= 0 || !validCanonicalTime(att.RecordedAt) {
		return fmt.Errorf("co-super assignment: invalid runtime grant policy attestation: %w", ErrEngineeringAssignmentInvalid)
	}
	want, err := grantAttestationRef(att)
	if err != nil || att.AttestationRef != want {
		return fmt.Errorf("co-super assignment: grant attestation digest mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	return nil
}

func validTypedDigestRef(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && types.ValidSHA256Digest(strings.TrimPrefix(value, prefix))
}

func validateExecutionAttestations(atts []types.EngineeringExecutionAttestation, report types.EngineeringAssignmentReport, assignment types.EngineeringAssignment) error {
	if len(atts) != len(report.Commands) || len(atts) != len(report.ExecutorReceiptRefs) {
		return fmt.Errorf("co-super assignment: exact execution attestation cardinality required: %w", ErrEngineeringAssignmentInvalid)
	}
	seen := map[string]bool{}
	for i, att := range atts {
		command := report.Commands[i]
		if att.Schema != types.EngineeringExecutionAttestationSchemaV1 || att.AssignmentID != assignment.AssignmentID || att.Attempt != assignment.Binding.Attempt ||
			att.OwnerID != assignment.Binding.OwnerID || att.ComputerID != assignment.Binding.ComputerID || att.TrajectoryID != assignment.Binding.TrajectoryID ||
			att.RunID != assignment.BoundRunID || att.CapsuleID != assignment.Binding.CapsuleID || att.ReportID != report.ReportID ||
			att.CommandID != command.CommandID || att.CommandDigest != command.CommandDigest || att.ExitCode != command.ExitCode ||
			att.GrantedReceiptRef != report.ExecutorReceiptRefs[i] || !validTypedDigestRef(att.GrantedReceiptRef, "capsule-granted-exec:") || !att.Granted || !att.Frozen ||
			!types.ValidSHA256Digest(att.StdoutDigest) || !types.ValidSHA256Digest(att.StderrDigest) ||
			att.SourceSubjectDigest != assignment.Binding.SubjectDigest || att.FinalSubjectDigest != report.ObservedSubjectDigest ||
			att.WorktreeDigest != att.FinalSubjectDigest || !validCanonicalTime(att.OccurredAt) || strings.TrimSpace(att.ReportCommandID) == "" ||
			att.ReportEventID != att.ReportCommandID+":1" || att.ReducerSeq <= 0 || !validCanonicalTime(att.RecordedAt) || seen[att.AttestationRef] {
			return fmt.Errorf("co-super assignment: invalid runtime execution attestation: %w", ErrEngineeringAssignmentInvalid)
		}
		want, err := executionAttestationRef(att)
		if err != nil || att.AttestationRef != want {
			return fmt.Errorf("co-super assignment: execution attestation digest mismatch: %w", ErrEngineeringAssignmentInvalid)
		}
		seen[att.AttestationRef] = true
	}
	return nil
}

func validateFateHistory(history []types.EngineeringCapsuleFateStep, assignment types.EngineeringAssignment) error {
	seen := map[string]bool{}
	lastSeq := int64(0)
	lastDisposition := types.EngineeringCapsuleUnbound
	if assignment.GrantPolicyAttestation != nil {
		lastDisposition = types.EngineeringCapsuleActive
	} else if len(history) > 0 {
		switch history[0].Disposition {
		case types.EngineeringCapsuleFreezeRequested:
			lastDisposition = types.EngineeringCapsuleActive
		case types.EngineeringCapsuleFrozen:
			lastDisposition = types.EngineeringCapsuleFreezeRequested
		case types.EngineeringCapsuleRevokeRequested:
			if assignment.BoundRunID != "" {
				lastDisposition = types.EngineeringCapsuleActive
			}
		case types.EngineeringCapsuleRevoked:
			lastDisposition = types.EngineeringCapsuleRevokeRequested
		}
	}
	var previous *types.EngineeringCapsuleFateStep
	for i := range history {
		step := history[i]
		if step.Schema != types.EngineeringCapsuleFateStepSchemaV1 || step.AssignmentID != assignment.AssignmentID || step.Attempt != assignment.Binding.Attempt ||
			step.OwnerID != assignment.Binding.OwnerID || step.ComputerID != assignment.Binding.ComputerID || step.TrajectoryID != assignment.Binding.TrajectoryID ||
			step.RunID != assignment.BoundRunID || step.CapsuleID != assignment.Binding.CapsuleID || step.ReducerSeq <= lastSeq || step.EventID != step.CommandID+":1" ||
			strings.TrimSpace(step.CommandID) == "" || strings.TrimSpace(step.IntentRef) == "" || step.AssignmentCapabilityDigest != assignment.Binding.CapabilityDigest ||
			!validCanonicalTime(step.OccurredAt) || !validCanonicalTime(step.RecordedAt) || seen[step.StepRef] || !validEngineeringCapsuleTransition(lastDisposition, step.Disposition, step.IntentRef, step.AckRef) {
			return fmt.Errorf("co-super assignment: invalid append-only capsule fate history: %w", ErrEngineeringAssignmentInvalid)
		}
		switch step.Disposition {
		case types.EngineeringCapsuleFreezeRequested, types.EngineeringCapsuleRevokeRequested:
			prefix := "capsule-freeze-intent:"
			if step.Disposition == types.EngineeringCapsuleRevokeRequested {
				prefix = "capsule-revoke-intent:"
			}
			if !validTypedDigestRef(step.IntentRef, prefix) || step.SourceSubjectDigest != "" || step.FinalSubjectDigest != "" || step.CapsuleAbsent {
				return ErrEngineeringAssignmentInvalid
			}
		case types.EngineeringCapsuleFrozen:
			if previous == nil || previous.Disposition != types.EngineeringCapsuleFreezeRequested || previous.IntentRef != step.IntentRef || !validTypedDigestRef(step.IntentRef, "capsule-freeze-intent:") || !validTypedDigestRef(step.AckRef, "capsule-fate:") || !types.ValidSHA256Digest(step.SourceSubjectDigest) || !types.ValidSHA256Digest(step.FinalSubjectDigest) || step.CapsuleAbsent {
				return ErrEngineeringAssignmentInvalid
			}
		case types.EngineeringCapsuleRevoked:
			if previous == nil || previous.Disposition != types.EngineeringCapsuleRevokeRequested || previous.IntentRef != step.IntentRef || !validTypedDigestRef(step.IntentRef, "capsule-revoke-intent:") || !validTypedDigestRef(step.AckRef, "capsule-revoke:") || step.SourceSubjectDigest != "" || step.FinalSubjectDigest != "" || !step.CapsuleAbsent {
				return ErrEngineeringAssignmentInvalid
			}
		}
		want, err := fateStepRef(step)
		if err != nil || step.StepRef != want {
			return fmt.Errorf("co-super assignment: fate step digest mismatch: %w", ErrEngineeringAssignmentInvalid)
		}
		seen[step.StepRef], lastSeq, lastDisposition = true, step.ReducerSeq, step.Disposition
		previous = &history[i]
	}
	if len(history) > 0 {
		last := history[len(history)-1]
		if lastDisposition != assignment.CapsuleDisposition || last.IntentRef != assignment.CapsuleIntentRef || last.AckRef != assignment.CapsuleAckRef {
			return fmt.Errorf("co-super assignment: fate history/current projection mismatch: %w", ErrEngineeringAssignmentInvalid)
		}
	}
	return nil
}

func engineeringAssignmentMetadata(a types.EngineeringAssignment) map[string]any {
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

func engineeringReportMetadata(r types.EngineeringAssignmentReport) map[string]any {
	return map[string]any{
		"report_id": r.ReportID, "assignment_id": r.AssignmentID, "attempt": r.Attempt,
		"computer_id": r.ComputerID, "trajectory_id": r.TrajectoryID, "loop_id": r.RunID,
		"result": string(r.Result), "verdict": string(r.Verdict), "late": r.Late,
	}
}

func engineeringEdge(fromID, toID string, kind objectgraph.EdgeKind, now time.Time) (objectgraph.Edge, error) {
	metadata := json.RawMessage(`{}`)
	id, err := objectgraph.BuildEdgeID(fromID, toID, kind, metadata)
	if err != nil {
		return objectgraph.Edge{}, err
	}
	return objectgraph.Edge{EdgeID: id, FromID: fromID, ToID: toID, Kind: kind, Metadata: metadata, CreatedAt: now}, nil
}

func engineeringObjectCondition(obj objectgraph.Object) objectgraph.ObjectCondition {
	return objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash}
}

func metadataExactString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func persistentManagementControlBinding(metadata map[string]any, trajectoryID, workItemID, updateID string) bool {
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

func persistentManagementRunStateAllowed(state types.RunState) bool {
	return state == types.RunPending || state == types.RunRunning || state == types.RunPassivated
}

// A late persistent-Management report may authenticate only the exact historical
// run that received its control. Terminal and passivated runs retain evidence
// authority, but never regain live execution authority.
func persistentManagementHistoricalReportRunStateAllowed(state types.RunState) bool {
	return state.Terminal() || state == types.RunPassivated
}

// requireEngineeringAssignmentAuthority proves the join between the lifecycle work
// graph and the exact non-lifecycle persistent Management control run. It never
// promotes that Management run or agent into lifecycle state.
func (s *Store) requireEngineeringParentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding) (engineeringAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return engineeringAuthorityObjects{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if binding.ParentRunID == "" {
		return s.requireEngineeringDocumentParentAuthority(ctx, binding, false)
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if trajectory.OwnerID != binding.OwnerID || trajectory.ComputerID != binding.ComputerID ||
		trajectory.TrajectoryID != binding.TrajectoryID || trajectory.Status != types.TrajectoryLive {
		return engineeringAuthorityObjects{}, ErrEngineeringAssignmentInvalid
	}
	parentAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.ParentAgentID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAgentObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentAgent.OwnerID != binding.OwnerID || parentAgent.ComputerID != binding.ComputerID ||
		parentAgent.AgentID != binding.ParentAgentID || parentAgent.Profile != agentprofile.Management || parentAgent.Role != agentprofile.Management ||
		parentAgent.ChannelID != binding.ParentAgentID || parentAgent.LifecycleVersion != 0 ||
		(parentAgent.ActiveRunID != "" && parentAgent.ActiveRunID != binding.ParentRunID) {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: exact non-lifecycle persistent Management unavailable: %w", ErrEngineeringAssignmentInvalid)
	}
	parentRunObj, err := s.getRunObjectByOwnerOG(ctx, binding.OwnerID, binding.ParentRunID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	parentRun, err := decodeLifecycleObject[types.RunRecord](parentRunObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentRunObj.ComputerID != "" || parentRun.OwnerID != binding.OwnerID || parentRun.ComputerID != binding.ComputerID ||
		parentRun.RunID != binding.ParentRunID || parentRun.AgentID != binding.ParentAgentID || parentRun.TrajectoryID != "" ||
		parentRun.AgentProfile != agentprofile.Management || parentRun.AgentRole != agentprofile.Management || !persistentManagementRunStateAllowed(parentRun.State) ||
		metadataExactString(parentRun.Metadata, "assignment_trajectory_id") != binding.TrajectoryID ||
		!persistentManagementControlBinding(parentRun.Metadata, binding.TrajectoryID, binding.ParentWorkItemID, binding.ParentControlID) {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent decision/control run binding mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	parentWorkObj, parentWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.ParentWorkItemID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentWork.OwnerID != binding.OwnerID || parentWork.ComputerID != binding.ComputerID ||
		parentWork.TrajectoryID != binding.TrajectoryID || parentWork.AssignedAgentID != binding.ParentAgentID ||
		parentWork.AuthorityProfile != agentprofile.Management || parentWork.Status != types.WorkItemOpen {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent Management target work mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	return engineeringAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory, parentAgent: parentAgentObj,
		parentRun: parentRunObj, parentWork: parentWorkObj}, nil
}

// requireEngineeringHistoricalParentAuthority authenticates a delayed report
// against the immutable authority binding after the live parent scope has
// terminalized. State may advance, but identities and the delivered control
// binding embedded in the exact parent run may not change.
func (s *Store) requireEngineeringHistoricalParentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding) (engineeringAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return engineeringAuthorityObjects{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if binding.ParentRunID == "" {
		return s.requireEngineeringDocumentParentAuthority(ctx, binding, true)
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if trajectory.OwnerID != binding.OwnerID || trajectory.ComputerID != binding.ComputerID || trajectory.TrajectoryID != binding.TrajectoryID ||
		(trajectory.Status != types.TrajectoryLive && trajectory.Status != types.TrajectorySettled && trajectory.Status != types.TrajectoryCancelled) {
		return engineeringAuthorityObjects{}, ErrEngineeringAssignmentInvalid
	}
	parentAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.ParentAgentID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAgentObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentAgent.OwnerID != binding.OwnerID || parentAgent.ComputerID != binding.ComputerID || parentAgent.AgentID != binding.ParentAgentID ||
		parentAgent.Profile != agentprofile.Management || parentAgent.Role != agentprofile.Management || parentAgent.ChannelID != binding.ParentAgentID || parentAgent.LifecycleVersion != 0 {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: historical persistent Management identity mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	parentRunObj, err := s.getRunObjectByOwnerOG(ctx, binding.OwnerID, binding.ParentRunID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	parentRun, err := decodeLifecycleObject[types.RunRecord](parentRunObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentRunObj.ComputerID != "" || parentRun.OwnerID != binding.OwnerID || parentRun.ComputerID != binding.ComputerID ||
		parentRun.RunID != binding.ParentRunID || parentRun.AgentID != binding.ParentAgentID || parentRun.TrajectoryID != "" ||
		parentRun.AgentProfile != agentprofile.Management || parentRun.AgentRole != agentprofile.Management || !parentRun.State.Valid() ||
		metadataExactString(parentRun.Metadata, "assignment_trajectory_id") != binding.TrajectoryID ||
		!persistentManagementControlBinding(parentRun.Metadata, binding.TrajectoryID, binding.ParentWorkItemID, binding.ParentControlID) {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: historical parent control binding mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	parentWorkObj, parentWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.ParentWorkItemID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentWork.OwnerID != binding.OwnerID || parentWork.ComputerID != binding.ComputerID || parentWork.TrajectoryID != binding.TrajectoryID ||
		parentWork.AssignedAgentID != binding.ParentAgentID || parentWork.AuthorityProfile != agentprofile.Management ||
		(parentWork.Status != types.WorkItemOpen && parentWork.Status != types.WorkItemCompleted && parentWork.Status != types.WorkItemCancelled && parentWork.Status != types.WorkItemRefused) {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: historical parent work binding mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	return engineeringAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory, parentAgent: parentAgentObj, parentRun: parentRunObj, parentWork: parentWorkObj}, nil
}

// requireEngineeringDocumentParentAuthority authenticates a document-driven
// assignment open: the parent authority is the engineering desk agent bound to
// the lifecycle document plus the owner-authored revision that carried the
// cast. There is no parent run; binding.ParentControlID names the revision.
// historical=true relaxes trajectory/work state for late reports.
func (s *Store) requireEngineeringDocumentParentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding, historical bool) (engineeringAuthorityObjects, error) {
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if trajectory.OwnerID != binding.OwnerID || trajectory.ComputerID != binding.ComputerID || trajectory.TrajectoryID != binding.TrajectoryID {
		return engineeringAuthorityObjects{}, ErrEngineeringAssignmentInvalid
	}
	if !historical && trajectory.Status != types.TrajectoryLive {
		return engineeringAuthorityObjects{}, ErrEngineeringAssignmentInvalid
	}
	if historical && trajectory.Status != types.TrajectoryLive && trajectory.Status != types.TrajectorySettled && trajectory.Status != types.TrajectoryCancelled {
		return engineeringAuthorityObjects{}, ErrEngineeringAssignmentInvalid
	}
	parentAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.ParentAgentID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAgentObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if parentAgent.OwnerID != binding.OwnerID || parentAgent.ComputerID != binding.ComputerID ||
		parentAgent.AgentID != binding.ParentAgentID || parentAgent.Profile != agentprofile.Engineering ||
		parentAgent.Role != agentprofile.Engineering || parentAgent.LifecycleVersion <= 0 ||
		docID == "" || parentAgent.AgentID != agentprofile.Engineering+":"+docID || parentAgent.ChannelID != docID {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: exact engineering desk parent unavailable: %w", ErrEngineeringAssignmentInvalid)
	}
	var parentControlObj objectgraph.Object
	if binding.SourceCandidateID != "" {
		// Verification assignment: the parent control is the candidate record —
		// the durable receipt of the completed implementation it verifies.
		candidateObj, candErr := s.lifecycleGraph().GetObject(ctx, binding.ParentControlID)
		if candErr != nil {
			return engineeringAuthorityObjects{}, candErr
		}
		candidate, decodeErr := decodeLifecycleObject[types.EngineeringSubjectCandidate](candidateObj)
		if decodeErr != nil {
			return engineeringAuthorityObjects{}, decodeErr
		}
		if candidate.CandidateID != binding.ParentControlID || candidate.CandidateID != binding.SourceCandidateID ||
			candidate.OwnerID != binding.OwnerID || candidate.ComputerID != binding.ComputerID ||
			candidate.TrajectoryID != binding.TrajectoryID {
			return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent candidate is outside exact trajectory authority: %w", ErrEngineeringAssignmentInvalid)
		}
		parentControlObj = candidateObj
	} else {
		revisionObj, revErr := s.lifecycleGetObject(ctx, ogKindTexRev, binding.OwnerID, binding.ComputerID, binding.ParentControlID)
		if revErr != nil {
			return engineeringAuthorityObjects{}, revErr
		}
		revision, decodeErr := decodeLifecycleObject[types.Revision](revisionObj)
		if decodeErr != nil {
			return engineeringAuthorityObjects{}, decodeErr
		}
		if revision.RevisionID != binding.ParentControlID || revision.OwnerID != binding.OwnerID ||
			revision.ComputerID != binding.ComputerID || revision.TrajectoryID != binding.TrajectoryID ||
			revision.DocID != docID || revision.AuthorKind != types.AuthorUser {
			return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent revision is not an owner-authored revision on the bound document: %w", ErrEngineeringAssignmentInvalid)
		}
		parentControlObj = revisionObj
	}
	parentWorkObj, parentWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.ParentWorkItemID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if parentWork.OwnerID != binding.OwnerID || parentWork.ComputerID != binding.ComputerID ||
		parentWork.TrajectoryID != binding.TrajectoryID || parentWork.AssignedAgentID != binding.ParentAgentID ||
		parentWork.AuthorityProfile != agentprofile.Engineering {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent engineering desk work mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	if !historical && parentWork.Status != types.WorkItemOpen {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: parent engineering desk work mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	if historical && parentWork.Status != types.WorkItemOpen && parentWork.Status != types.WorkItemCompleted &&
		parentWork.Status != types.WorkItemCancelled && parentWork.Status != types.WorkItemRefused {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: historical parent work binding mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	return engineeringAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory, parentAgent: parentAgentObj,
		parentRevision: parentControlObj, parentWork: parentWorkObj}, nil
}

func (s *Store) requireEngineeringAssignmentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding) (engineeringAuthorityObjects, error) {
	authority, err := s.requireEngineeringParentAuthority(ctx, binding)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	assignedAgentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, binding.OwnerID, binding.ComputerID, binding.AssignedAgentID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	assignedAgent, err := decodeLifecycleObject[types.AgentRecord](assignedAgentObj)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if assignedAgent.OwnerID != binding.OwnerID || assignedAgent.ComputerID != binding.ComputerID ||
		assignedAgent.AgentID != binding.AssignedAgentID || assignedAgent.Profile != agentprofile.Engineering || assignedAgent.Role != agentprofile.Engineering || assignedAgent.LifecycleVersion <= 0 {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: assigned lifecycle Engineering agent mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	assignedWorkObj, assignedWork, err := s.lifecycleWorkObject(ctx, binding.OwnerID, binding.ComputerID, binding.AssignedWorkItemID)
	if err != nil {
		return engineeringAuthorityObjects{}, err
	}
	if assignedWork.OwnerID != binding.OwnerID || assignedWork.ComputerID != binding.ComputerID ||
		assignedWork.TrajectoryID != binding.TrajectoryID || assignedWork.AssignedAgentID != binding.AssignedAgentID ||
		assignedWork.AuthorityProfile != agentprofile.Engineering || assignedWork.Status != types.WorkItemOpen ||
		metadataExactString(assignedWork.Details, "parent_loop_id") != binding.ParentRunID ||
		metadataExactString(assignedWork.Details, "parent_decision_id") != binding.ParentDecisionID ||
		metadataExactString(assignedWork.Details, "parent_control_id") != binding.ParentControlID ||
		metadataExactString(assignedWork.Details, "parent_work_item_id") != binding.ParentWorkItemID {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: assigned Engineering work mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	authority.assignedAgent, authority.assignedWork = assignedAgentObj, assignedWorkObj
	return authority, nil
}

func engineeringParentAuthorityConditions(authority engineeringAuthorityObjects) []objectgraph.ObjectCondition {
	conditions := []objectgraph.ObjectCondition{
		engineeringObjectCondition(authority.parentAgent),
		engineeringObjectCondition(authority.parentWork),
	}
	if authority.parentRun.CanonicalID != "" {
		conditions = append(conditions, engineeringObjectCondition(authority.parentRun))
	}
	if authority.parentRevision.CanonicalID != "" {
		conditions = append(conditions, engineeringObjectCondition(authority.parentRevision))
	}
	return conditions
}

func engineeringAuthorityConditions(authority engineeringAuthorityObjects) []objectgraph.ObjectCondition {
	conditions := engineeringParentAuthorityConditions(authority)
	return append(conditions, engineeringObjectCondition(authority.assignedAgent), engineeringObjectCondition(authority.assignedWork))
}

func (s *Store) prepareEngineeringLifecycleTransition(ctx context.Context, trajectoryObj objectgraph.Object, trajectory types.TrajectoryRecord, now time.Time) (engineeringLifecycleTransition, error) {
	if trajectory.Status == types.TrajectoryLive {
		next := trajectory
		next.ReducerSeq++
		next.LifecycleVersion++
		next.UpdatedAt = now
		updated, err := lifecycleObject(ogKindTrajectory, next.OwnerID, next.ComputerID, next.TrajectoryID, next,
			lifecycleMetadata("trajectory_id", next.TrajectoryID, next.ComputerID, next.TrajectoryID, next.ReducerSeq), trajectoryObj.CreatedAt, now)
		if err != nil {
			return engineeringLifecycleTransition{}, err
		}
		return engineeringLifecycleTransition{
			trajectory: next, seq: next.ReducerSeq,
			conditions: []objectgraph.ObjectCondition{engineeringObjectCondition(trajectoryObj)},
			objects:    []objectgraph.Object{updated},
		}, nil
	}
	seq, sequenceObj, sequenceCondition, err := s.nextPostTerminalSequence(ctx, trajectory.OwnerID, trajectory.ComputerID, trajectory, now)
	if err != nil {
		return engineeringLifecycleTransition{}, err
	}
	return engineeringLifecycleTransition{
		trajectory: trajectory, seq: seq,
		conditions: []objectgraph.ObjectCondition{sequenceCondition}, objects: []objectgraph.Object{sequenceObj},
	}, nil
}

func (s *Store) buildEngineeringLifecycleEvent(now time.Time, assignment types.EngineeringAssignment, commandID, digest string, kind types.LifecycleEventKind, seq int64, artifactRefs, evidenceRefs []string, reason string) (types.LifecycleEvent, objectgraph.Object, error) {
	event := types.LifecycleEvent{
		Schema: types.EngineeringAssignmentSchemaV1, EventID: commandID + ":1",
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

func (s *Store) replayEngineeringAssignmentCommand(ctx context.Context, ownerID, computerID, commandID, digest, assignmentID string, attempt uint64, reportID string) (types.EngineeringAssignmentCommandResult, bool, error) {
	lifecycleResult, found, err := s.replayLifecycleCommand(ctx, ownerID, computerID, commandID, digest)
	if !found || err != nil {
		return types.EngineeringAssignmentCommandResult{}, found, err
	}
	assignment, err := s.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, true, err
	}
	result := types.EngineeringAssignmentCommandResult{Receipt: lifecycleResult.Receipt, Assignment: assignment, Replay: true}
	if strings.TrimSpace(reportID) != "" {
		report, reportErr := s.GetEngineeringAssignmentReport(ctx, ownerID, computerID, reportID)
		if reportErr != nil {
			return types.EngineeringAssignmentCommandResult{}, true, reportErr
		}
		result.Report = &report
		if update, updateErr := s.GetLifecycleUpdate(ctx, ownerID, computerID, assignment.Binding.TrajectoryID,
			assignment.Binding.ParentAgentID, assignment.Binding.AssignedAgentID, report.ReportID); updateErr == nil {
			result.Update = &update
		} else if !errors.Is(updateErr, ErrNotFound) {
			return types.EngineeringAssignmentCommandResult{}, true, updateErr
		}
		if report.CandidateID != "" {
			obj, getErr := s.lifecycleGraph().GetObject(ctx, report.CandidateID)
			if getErr != nil {
				return types.EngineeringAssignmentCommandResult{}, true, getErr
			}
			candidate, decodeErr := decodeLifecycleObject[types.EngineeringSubjectCandidate](obj)
			if decodeErr != nil {
				return types.EngineeringAssignmentCommandResult{}, true, decodeErr
			}
			result.Candidate = &candidate
		}
	}
	return result, true, nil
}

// ReplayRecordedEngineeringAssignmentReport returns the original authenticated
// lifecycle receipt and current assignment projection without synthesizing a
// new report command after fate transitions.
func (s *Store) ReplayRecordedEngineeringAssignmentReport(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64, reportID, commandID string) (types.EngineeringAssignmentCommandResult, error) {
	assignment, err := s.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	report, err := s.GetEngineeringAssignmentReport(ctx, ownerID, computerID, reportID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if report.AssignmentID != assignmentID || report.Attempt != attempt {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	commandCanonicalID, err := lifecycleCanonicalID(ogKindLifecycleCmd, ownerID, computerID, commandID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	obj, err := s.lifecycleGraph().GetObject(ctx, commandCanonicalID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](obj)
	if err != nil || receipt.CommandID != commandID || receipt.Kind != types.LifecycleRecordEngineeringAssignment {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	result := types.EngineeringAssignmentCommandResult{Receipt: receipt, Assignment: assignment, Report: &report, Replay: true}
	if update, updateErr := s.GetLifecycleUpdate(ctx, ownerID, computerID, assignment.Binding.TrajectoryID,
		assignment.Binding.ParentAgentID, assignment.Binding.AssignedAgentID, report.ReportID); updateErr == nil {
		result.Update = &update
	} else if !errors.Is(updateErr, ErrNotFound) {
		return types.EngineeringAssignmentCommandResult{}, updateErr
	}
	if report.CandidateID != "" {
		candidateObj, getErr := s.lifecycleGraph().GetObject(ctx, report.CandidateID)
		if getErr != nil {
			return types.EngineeringAssignmentCommandResult{}, getErr
		}
		candidate, decodeErr := decodeLifecycleObject[types.EngineeringSubjectCandidate](candidateObj)
		if decodeErr != nil {
			return types.EngineeringAssignmentCommandResult{}, decodeErr
		}
		result.Candidate = &candidate
	}
	return result, nil
}

func (s *Store) commitEngineeringLifecycleCommand(ctx context.Context, transition engineeringLifecycleTransition, commandKind types.LifecycleCommandKind, eventKind types.LifecycleEventKind, commandID, digest string, assignment types.EngineeringAssignment, report *types.EngineeringAssignmentReport, candidate *types.EngineeringSubjectCandidate, reportID, reason string, artifactObjects []objectgraph.Object, conditions []objectgraph.ObjectCondition, edges []objectgraph.Edge, update *types.CoagentSourcePacket, evidenceRefs []string) (types.EngineeringAssignmentCommandResult, error) {
	now := assignment.UpdatedAt
	artifactRefs := make([]string, 0, len(artifactObjects))
	for _, obj := range artifactObjects {
		artifactRefs = append(artifactRefs, obj.CanonicalID)
	}
	event, eventObj, err := s.buildEngineeringLifecycleEvent(now, assignment, commandID, digest, eventKind, transition.seq, artifactRefs, evidenceRefs, reason)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		assignment.Binding.TrajectoryID, commandID, digest, commandKind, transition.seq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	conditions = append(transition.conditions, conditions...)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects := append(append([]objectgraph.Object{}, transition.objects...), artifactObjects...)
	objects = append(objects, eventObj, receiptObj)
	lifecycleResult, err := s.commitLifecycleTransition(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		commandID, digest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: transition.trajectory, Update: update, Events: []types.LifecycleEvent{event}}, edges...)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if lifecycleResult.Replay {
		replayed, _, replayErr := s.replayEngineeringAssignmentCommand(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
			commandID, digest, assignment.AssignmentID, assignment.Binding.Attempt, reportID)
		return replayed, replayErr
	}
	return types.EngineeringAssignmentCommandResult{Receipt: receipt, Assignment: assignment, Report: report, Candidate: candidate, Update: update}, nil
}

func (s *Store) OpenEngineeringAssignment(ctx context.Context, req types.OpenEngineeringAssignmentRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest, req.AssignmentID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.AssignmentID)
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if err := req.Binding.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if strings.TrimSpace(req.AssignedAgent.AgentID) != req.Binding.AssignedAgentID ||
		strings.TrimSpace(req.AssignedWork.WorkItemID) != req.Binding.AssignedWorkItemID ||
		strings.TrimSpace(req.AssignedWork.AssignedAgentID) != req.Binding.AssignedAgentID ||
		strings.TrimSpace(req.AssignedWork.Objective) == "" {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	computedDigest, digestErr := ComputeOpenEngineeringAssignmentDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.Binding.OwnerID, req.Binding.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt, ""); found || err != nil {
		return replay, err
	}
	if err := s.validateEngineeringSupersedeTuple(ctx, req); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	authority, err := s.requireEngineeringParentAuthority(ctx, req.Binding)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, authority.trajectory, authority.trajectoryRec, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}

	assignedAgent := req.AssignedAgent
	assignedAgent.AgentID, assignedAgent.OwnerID, assignedAgent.ComputerID = req.Binding.AssignedAgentID, req.Binding.OwnerID, req.Binding.ComputerID
	assignedAgent.ComputerID, assignedAgent.Profile, assignedAgent.Role = req.Binding.ComputerID, agentprofile.Engineering, agentprofile.Engineering
	assignedAgent.ChannelID, assignedAgent.ActiveRunID = req.Binding.AssignedAgentID, ""
	assignedAgent.LifecycleVersion, assignedAgent.LastReducerSeq = 1, transition.seq
	assignedAgent.CreatedAt, assignedAgent.UpdatedAt = now, now
	assignedWork := req.AssignedWork
	assignedWork.WorkItemID, assignedWork.OwnerID, assignedWork.ComputerID = req.Binding.AssignedWorkItemID, req.Binding.OwnerID, req.Binding.ComputerID
	assignedWork.TrajectoryID, assignedWork.AssignedAgentID = req.Binding.TrajectoryID, req.Binding.AssignedAgentID
	assignedWork.Objective = strings.TrimSpace(assignedWork.Objective)
	assignedWork.AuthorityProfile, assignedWork.Status, assignedWork.ResultRef = agentprofile.Engineering, types.WorkItemOpen, ""
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

	assignment := types.EngineeringAssignment{
		Schema: types.EngineeringAssignmentSchemaV1, AssignmentID: req.AssignmentID, Binding: req.Binding,
		Disposition: types.EngineeringAssignmentOpen, CapsuleDisposition: types.EngineeringCapsuleUnbound,
		LifecycleVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := assignment.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignmentObj, err := lifecycleObject(ogKindEngineeringAssignment, req.Binding.OwnerID, req.Binding.ComputerID,
		engineeringAttemptKey(req.AssignmentID, req.Binding.Attempt), assignment, engineeringAssignmentMetadata(assignment), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	agentObj, err := lifecycleObject(ogKindAgent, req.Binding.OwnerID, req.Binding.ComputerID, assignedAgent.AgentID, assignedAgent,
		lifecycleMetadata("agent_id", assignedAgent.AgentID, req.Binding.ComputerID, req.Binding.TrajectoryID, transition.seq), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	workObj, err := lifecycleObject(ogKindWorkItem, req.Binding.OwnerID, req.Binding.ComputerID, assignedWork.WorkItemID, assignedWork,
		lifecycleMetadata("work_item_id", assignedWork.WorkItemID, req.Binding.ComputerID, req.Binding.TrajectoryID, transition.seq), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	targets := []struct {
		obj  objectgraph.Object
		kind objectgraph.EdgeKind
	}{
		{authority.trajectory, ogEdgeAssignmentTrajectory}, {authority.parentAgent, ogEdgeAssignmentParent},
		{authority.parentWork, ogEdgeAssignmentParentWork},
		{agentObj, ogEdgeAssignmentAgent}, {workObj, ogEdgeAssignmentWork},
	}
	if authority.parentRun.CanonicalID != "" {
		targets = append(targets, struct {
			obj  objectgraph.Object
			kind objectgraph.EdgeKind
		}{authority.parentRun, ogEdgeAssignmentParentRun})
	}
	if authority.parentRevision.CanonicalID != "" {
		targets = append(targets, struct {
			obj  objectgraph.Object
			kind objectgraph.EdgeKind
		}{authority.parentRevision, ogEdgeAssignmentParentRun})
	}
	edges := make([]objectgraph.Edge, 0, len(targets))
	for _, target := range targets {
		edge, edgeErr := engineeringEdge(assignmentObj.CanonicalID, target.obj.CanonicalID, target.kind, now)
		if edgeErr != nil {
			return types.EngineeringAssignmentCommandResult{}, edgeErr
		}
		edges = append(edges, edge)
	}
	conditions := engineeringParentAuthorityConditions(authority)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: assignmentObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: agentObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID})
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleOpenEngineeringAssignment, types.LifecycleEngineeringAssignmentOpened,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", "", []objectgraph.Object{assignmentObj, agentObj, workObj}, conditions, edges, nil, nil)
}

// validateEngineeringSupersedeTuple enforces the frozen correction rule
// (settlement gate item 3, v1): attempt 1 never carries a tuple; attempt > 1
// always carries one naming a recorded prior report of the same assignment's
// earlier attempt with a valid kind, a non-empty reason, and a well-formed
// structured-fields delta digest. The tuple rides the open command digest, so
// a correction cannot be re-targeted without invalidating the command.
func (s *Store) validateEngineeringSupersedeTuple(ctx context.Context, req types.OpenEngineeringAssignmentRequest) error {
	tuple := req.Supersedes
	if tuple == nil {
		if req.Binding.Attempt > 1 {
			return fmt.Errorf("co-super assignment: attempt %d requires the superseding tuple: %w", req.Binding.Attempt, ErrEngineeringAssignmentInvalid)
		}
		return nil
	}
	if req.Binding.Attempt <= 1 {
		return fmt.Errorf("co-super assignment: attempt 1 cannot carry a superseding tuple: %w", ErrEngineeringAssignmentInvalid)
	}
	if strings.TrimSpace(tuple.SupersedesAssignmentID) != strings.TrimSpace(req.AssignmentID) {
		return fmt.Errorf("co-super assignment: supersede targets its own assignment: %w", ErrEngineeringAssignmentInvalid)
	}
	if tuple.SupersedesAttempt == 0 || tuple.SupersedesAttempt >= req.Binding.Attempt {
		return fmt.Errorf("co-super assignment: supersede names a strictly earlier attempt: %w", ErrEngineeringAssignmentInvalid)
	}
	switch tuple.SupersedeKind {
	case types.EngineeringSupersedeCorrection, types.EngineeringSupersedeRetryAfterBlock, types.EngineeringSupersedeOwnerReopen:
	default:
		return fmt.Errorf("co-super assignment: invalid supersede kind %q: %w", tuple.SupersedeKind, ErrEngineeringAssignmentInvalid)
	}
	if strings.TrimSpace(tuple.ReasonEnum) == "" || strings.TrimSpace(tuple.PriorReceiptRef) == "" {
		return fmt.Errorf("co-super assignment: supersede requires reason and prior receipt: %w", ErrEngineeringAssignmentInvalid)
	}
	if !types.ValidSHA256Digest(strings.TrimSpace(tuple.DeltaDigest)) {
		return fmt.Errorf("co-super assignment: supersede delta digest must be exact sha256: %w", ErrEngineeringAssignmentInvalid)
	}
	_, prior, err := s.getEngineeringAssignmentObject(ctx, req.Binding.OwnerID, req.Binding.ComputerID, req.AssignmentID, tuple.SupersedesAttempt)
	if err != nil {
		return fmt.Errorf("co-super assignment: superseded attempt unavailable: %w", ErrEngineeringAssignmentInvalid)
	}
	for _, ref := range prior.ReportRefs {
		obj, getErr := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(ref))
		if getErr != nil {
			continue
		}
		stored, decodeErr := decodeLifecycleObject[types.EngineeringAssignmentReport](obj)
		if decodeErr != nil {
			continue
		}
		if strings.TrimSpace(stored.ReportID) == strings.TrimSpace(tuple.PriorReceiptRef) {
			return nil
		}
	}
	return fmt.Errorf("co-super assignment: prior receipt is not a recorded report of the superseded attempt: %w", ErrEngineeringAssignmentInvalid)
}

func (s *Store) getEngineeringAssignmentObject(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) (objectgraph.Object, types.EngineeringAssignment, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindEngineeringAssignment, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), engineeringAttemptKey(assignmentID, attempt))
	if err != nil {
		return objectgraph.Object{}, types.EngineeringAssignment{}, err
	}
	assignment, err := decodeLifecycleObject[types.EngineeringAssignment](obj)
	if err != nil {
		return objectgraph.Object{}, types.EngineeringAssignment{}, err
	}
	if assignment.Binding.OwnerID != strings.TrimSpace(ownerID) || assignment.Binding.ComputerID != strings.TrimSpace(computerID) ||
		assignment.AssignmentID != strings.TrimSpace(assignmentID) || assignment.Binding.Attempt != attempt {
		return objectgraph.Object{}, types.EngineeringAssignment{}, ErrNotFound
	}
	if err := assignment.Validate(); err != nil {
		return objectgraph.Object{}, types.EngineeringAssignment{}, err
	}
	if assignment.GrantPolicyAttestation != nil {
		if err := validateGrantPolicyAttestation(*assignment.GrantPolicyAttestation, assignment); err != nil {
			return objectgraph.Object{}, types.EngineeringAssignment{}, err
		}
	}
	if err := validateFateHistory(assignment.CapsuleFateHistory, assignment); err != nil {
		return objectgraph.Object{}, types.EngineeringAssignment{}, err
	}
	return obj, assignment, nil
}

func (s *Store) GetEngineeringAssignment(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) (types.EngineeringAssignment, error) {
	_, assignment, err := s.getEngineeringAssignmentObject(ctx, ownerID, computerID, assignmentID, attempt)
	return assignment, err
}

func engineeringParentProfileForBinding(binding types.EngineeringAssignmentBinding) string {
	if profile, _, ok := strings.Cut(binding.ParentAgentID, ":"); ok {
		return profile
	}
	return ""
}

// ListEngineeringAssignmentsForComputer returns every non-tombstoned Engineering
// assignment object in one computer, independent of the run-state metadata
// index or the open work-item projection. Boot recovery uses it to reconcile
// capsules for assignments bound before the run-state index existed.
func (s *Store) ListEngineeringAssignmentsForComputer(ctx context.Context, computerID string) ([]types.EngineeringAssignment, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("list Engineering assignments for computer: computer_id is required")
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindEngineeringAssignment)
	if err != nil {
		return nil, fmt.Errorf("list Engineering assignments for computer: %w", err)
	}
	out := make([]types.EngineeringAssignment, 0, len(objs))
	for _, obj := range objs {
		if obj.ComputerID != computerID || obj.Tombstone {
			continue
		}
		assignment, decodeErr := decodeLifecycleObject[types.EngineeringAssignment](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if assignment.Binding.ComputerID != computerID {
			continue
		}
		out = append(out, assignment)
	}
	slices.SortFunc(out, func(left, right types.EngineeringAssignment) int {
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

func (s *Store) ListEngineeringAssignments(ctx context.Context, ownerID, computerID, trajectoryID string) ([]types.EngineeringAssignment, error) {
	objects, err := s.ogListAllByMetadata(ctx, ogKindEngineeringAssignment, "trajectory_id", strings.TrimSpace(trajectoryID))
	if err != nil {
		return nil, err
	}
	out := make([]types.EngineeringAssignment, 0, len(objects))
	for _, obj := range objects {
		if obj.OwnerID != strings.TrimSpace(ownerID) || obj.ComputerID != strings.TrimSpace(computerID) || obj.Tombstone {
			continue
		}
		assignment, decodeErr := decodeLifecycleObject[types.EngineeringAssignment](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if assignment.Binding.TrajectoryID == strings.TrimSpace(trajectoryID) {
			out = append(out, assignment)
		}
	}
	slices.SortFunc(out, func(left, right types.EngineeringAssignment) int {
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

func validateEngineeringAssignmentRun(assignment types.EngineeringAssignment, assignedAgent types.AgentRecord, run types.RunRecord) error {
	workIDs, bindingErr := lifecycleActivationWorkItemIDs(run.Metadata)
	coordinationID := metadataExactString(run.Metadata, "coordination_contract_id")
	coordinationDigest := metadataExactString(run.Metadata, "coordination_contract_digest")
	if bindingErr != nil || len(workIDs) != 1 || workIDs[0] != assignment.Binding.AssignedWorkItemID ||
		metadataExactString(run.Metadata, "lifecycle_work_item_id") != assignment.Binding.AssignedWorkItemID ||
		strings.TrimSpace(run.RunID) == "" || run.OwnerID != assignment.Binding.OwnerID || run.ComputerID != assignment.Binding.ComputerID ||
		run.TrajectoryID != assignment.Binding.TrajectoryID || run.AgentID != assignment.Binding.AssignedAgentID ||
		run.ChannelID != assignedAgent.ChannelID || run.AgentProfile != agentprofile.Engineering || run.AgentRole != agentprofile.Engineering || run.State != types.RunPending ||
		run.RequestedByRunID != assignment.Binding.ParentRunID || !run.CreatedAt.IsZero() || !run.UpdatedAt.IsZero() ||
		run.FinishedAt != nil || run.Result != "" || run.Error != "" ||
		metadataExactString(run.Metadata, "requested_by_agent_id") != assignment.Binding.ParentAgentID ||
		metadataExactString(run.Metadata, "requested_by_profile") != engineeringParentProfileForBinding(assignment.Binding) ||
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
		return fmt.Errorf("co-super assignment: exact run/work/parent binding mismatch: %w", ErrEngineeringAssignmentInvalid)
	}
	return nil
}

func engineeringRunMetadata(run types.RunRecord, seq int64) map[string]any {
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

func (s *Store) BindEngineeringAssignment(ctx context.Context, req types.BindEngineeringAssignmentRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.RunID, req.CapsuleID = strings.TrimSpace(req.RunID), strings.TrimSpace(req.CapsuleID)
	if req.RunID == "" {
		req.RunID = strings.TrimSpace(req.Run.RunID)
	}
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if strings.TrimSpace(req.OpaqueCapability) == "" || req.RunID == "" || req.RunID != strings.TrimSpace(req.Run.RunID) || req.ExpectedLifecycleVersion <= 0 {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	computedDigest, digestErr := ComputeBindEngineeringAssignmentDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, ""); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getEngineeringAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.Disposition != types.EngineeringAssignmentOpen ||
		DigestEngineeringOpaqueCapability(req.OpaqueCapability) != assignment.Binding.CapabilityDigest || req.CapsuleID != assignment.Binding.CapsuleID {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	authority, err := s.requireEngineeringAssignmentAuthority(ctx, assignment.Binding)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignedAgent, err := decodeLifecycleObject[types.AgentRecord](authority.assignedAgent)
	if err != nil || assignedAgent.ActiveRunID != "" {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	if err := validateEngineeringAssignmentRun(assignment, assignedAgent, req.Run); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, authority.trajectory, authority.trajectoryRec, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	run := req.Run
	run.RunID, run.CreatedAt, run.UpdatedAt, run.FinishedAt = req.RunID, now, now, nil
	run.Result, run.Error = "", ""
	runObj, err := lifecycleObject(ogKindRun, req.OwnerID, req.ComputerID, run.RunID, run,
		engineeringRunMetadata(run, transition.seq), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignedAgent.ActiveRunID = run.RunID
	assignedAgent.LifecycleVersion++
	assignedAgent.LastReducerSeq, assignedAgent.UpdatedAt = transition.seq, now
	assignedAgentObj, err := lifecycleObject(ogKindAgent, req.OwnerID, req.ComputerID, assignedAgent.AgentID, assignedAgent,
		lifecycleMetadata("agent_id", assignedAgent.AgentID, req.ComputerID, assignment.Binding.TrajectoryID, transition.seq), authority.assignedAgent.CreatedAt, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignment.Disposition, assignment.BoundRunID = types.EngineeringAssignmentBound, req.RunID
	assignment.CapsuleDisposition = types.EngineeringCapsuleActive
	assignment.LifecycleVersion++
	assignment.UpdatedAt = now
	if req.GrantPolicyAttestation != nil {
		att := *req.GrantPolicyAttestation
		att.Schema = types.EngineeringGrantPolicyAttestationSchemaV1
		att.AssignmentID, att.Attempt = assignment.AssignmentID, assignment.Binding.Attempt
		att.OwnerID, att.ComputerID, att.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
		att.RunID, att.CapsuleID, att.TargetCapsule = assignment.BoundRunID, assignment.Binding.CapsuleID, assignment.Binding.CapsuleID
		att.NetworkMode, att.FilesystemMode, att.Writable = assignment.Binding.NetworkMode, assignment.Binding.FilesystemMode, assignment.Binding.Writable
		att.BindCommandID, att.BindEventID, att.ReducerSeq, att.RecordedAt = req.CommandID, req.CommandID+":1", transition.seq, now
		att.AttestationRef = ""
		att.AttestationRef, err = grantAttestationRef(att)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
		assignment.GrantPolicyAttestation = &att
		if err := validateGrantPolicyAttestation(att, assignment); err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
	}
	updatedObj, err := lifecycleObject(ogKindEngineeringAssignment, req.OwnerID, req.ComputerID,
		engineeringAttemptKey(req.AssignmentID, req.Attempt), assignment, engineeringAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	runClaim := engineeringRunClaim{RunID: req.RunID, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, CreatedAt: now}
	runClaimObj, err := lifecycleObject(ogKindEngineeringRunClaim, req.OwnerID, req.ComputerID, req.RunID, runClaim,
		map[string]any{"run_id": req.RunID, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	capabilityClaim := engineeringCapabilityClaim{CapabilityDigest: assignment.Binding.CapabilityDigest, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, RunID: req.RunID, CreatedAt: now}
	capabilityObj, err := lifecycleObject(ogKindEngineeringCapability, req.OwnerID, req.ComputerID, assignment.Binding.CapabilityDigest, capabilityClaim,
		map[string]any{"capability_digest": assignment.Binding.CapabilityDigest, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	capsuleClaim := engineeringCapsuleClaim{CapsuleID: req.CapsuleID, OwnerID: req.OwnerID, ComputerID: req.ComputerID,
		AssignmentID: req.AssignmentID, Attempt: req.Attempt, RunID: req.RunID, CreatedAt: now}
	capsuleObj, err := lifecycleObject(ogKindEngineeringCapsule, req.OwnerID, req.ComputerID, req.CapsuleID, capsuleClaim,
		map[string]any{"capsule_id": req.CapsuleID, "computer_id": req.ComputerID, "assignment_id": req.AssignmentID, "attempt": req.Attempt}, now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	conditions := engineeringAuthorityConditions(authority)
	conditions = append(conditions, engineeringObjectCondition(assignmentObj), objectgraph.ObjectCondition{CanonicalID: runObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: runClaimObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: capabilityObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: capsuleObj.CanonicalID})
	runEdge, err := engineeringEdge(updatedObj.CanonicalID, runObj.CanonicalID, ogEdgeAssignmentRun, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleBindEngineeringAssignment, types.LifecycleEngineeringAssignmentBound,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", "",
		[]objectgraph.Object{updatedObj, assignedAgentObj, runObj, runClaimObj, capabilityObj, capsuleObj}, conditions, []objectgraph.Edge{runEdge}, nil, nil)
}

func engineeringReportChangedSubject(report types.EngineeringAssignmentReport, originalDigest string) bool {
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

func reducerAssignmentOutcome(report types.EngineeringAssignmentReport, assignment types.EngineeringAssignment, changed bool) types.EngineeringAssignmentDisposition {
	if report.Late || assignment.Disposition.Terminal() || report.Result == types.EngineeringResultPartial {
		return assignment.Disposition
	}
	if report.Result == types.EngineeringResultFailed || report.Result == types.EngineeringResultBlocked {
		return types.EngineeringAssignmentFailed
	}
	if assignment.Binding.Kind == types.EngineeringAssignmentVerification &&
		(report.Verdict != types.EngineeringVerdictPass || changed) {
		return types.EngineeringAssignmentFailed
	}
	if report.Result == types.EngineeringResultCompleted {
		return types.EngineeringAssignmentCompleted
	}
	return assignment.Disposition
}

func engineeringReportPacketPayload(report types.EngineeringAssignmentReport, cancellation bool) (types.CoagentSourcePacketPayload, string) {
	kind := "execution_result"
	if report.Result == types.EngineeringResultFailed || report.Result == types.EngineeringResultBlocked || cancellation {
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

func buildEngineeringReturnPacket(now time.Time, seq int64, assignment types.EngineeringAssignment, report types.EngineeringAssignmentReport, parentRun types.RunRecord, parentChannelID string, cancellation bool) (types.CoagentSourcePacket, objectgraph.Object, error) {
	packetPayload, content := engineeringReportPacketPayload(report, cancellation)
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
		ChannelID: strings.TrimSpace(parentChannelID), MessageSeq: seq, TrajectoryID: assignment.Binding.TrajectoryID,
		Direction: types.LifecyclePacketDirectionProducerReport, ControlBindingID: assignment.Binding.ParentControlID,
		ProducerWorkItemID: assignment.Binding.AssignedWorkItemID, TargetWorkItemID: assignment.Binding.ParentWorkItemID,
		WorkItemID: assignment.Binding.AssignedWorkItemID, Role: agentprofile.Engineering, SourceRunID: assignment.BoundRunID,
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

// coManagementParentReturnTarget resolves the return-packet parent run and channel
// for one assignment binding. Run-parented bindings decode the bound parent
// run; document-parented bindings (ParentRunID == "") have no parent run —
// the engineering desk agent's channel (the bound document) is the return
// target, and the packet stays undelivered until a desk activation consumes
// it.
func engineeringParentReturnTarget(parentAuthority engineeringAuthorityObjects, binding types.EngineeringAssignmentBinding) (types.RunRecord, string, error) {
	if strings.TrimSpace(binding.ParentRunID) != "" {
		parentRun, err := decodeLifecycleObject[types.RunRecord](parentAuthority.parentRun)
		if err != nil {
			return types.RunRecord{}, "", err
		}
		return parentRun, parentRun.ChannelID, nil
	}
	parentAgent, err := decodeLifecycleObject[types.AgentRecord](parentAuthority.parentAgent)
	if err != nil {
		return types.RunRecord{}, "", err
	}
	return types.RunRecord{}, parentAgent.ChannelID, nil
}

func engineeringTerminalRunState(disposition types.EngineeringAssignmentDisposition) types.RunState {
	switch disposition {
	case types.EngineeringAssignmentCompleted:
		return types.RunCompleted
	case types.EngineeringAssignmentCancelled:
		return types.RunCancelled
	default:
		return types.RunFailed
	}
}

func engineeringTerminalWorkState(disposition types.EngineeringAssignmentDisposition) types.WorkItemStatus {
	switch disposition {
	case types.EngineeringAssignmentCompleted:
		return types.WorkItemCompleted
	case types.EngineeringAssignmentCancelled:
		return types.WorkItemCancelled
	default:
		return types.WorkItemRefused
	}
}

// projectEngineeringTerminal atomically closes the exact assigned work/run/agent.
// Capsule fate is deliberately absent: intent/effect/ack remains a separate
// sequenced authority after (or before cancellation) this lifecycle commit.
func (s *Store) projectEngineeringTerminal(ctx context.Context, assignment types.EngineeringAssignment, seq int64, now time.Time, resultRef, reason string) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
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
		return nil, nil, ErrEngineeringAssignmentInvalid
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
			return nil, nil, ErrEngineeringAssignmentInvalid
		}
		agent.ActiveRunID = ""
		agent.LifecycleVersion++
		agent.LastReducerSeq, agent.UpdatedAt = seq, now
		work.Status, work.ResultRef, work.Reason = engineeringTerminalWorkState(assignment.Disposition), resultRef, reason
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
		conditions = append(conditions, engineeringObjectCondition(agentObj), engineeringObjectCondition(workObj))
	} else {
		if work.Status != types.WorkItemCancelled || assignment.Disposition != types.EngineeringAssignmentCancelled ||
			(agent.ActiveRunID != "" && agent.ActiveRunID != assignment.BoundRunID) {
			return nil, nil, ErrEngineeringAssignmentInvalid
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
			conditions = append(conditions, engineeringObjectCondition(agentObj))
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
		return nil, nil, ErrEngineeringAssignmentInvalid
	}
	if run.State.Terminal() {
		if run.State == engineeringTerminalRunState(assignment.Disposition) {
			return objects, conditions, nil
		}
		// The run already terminated with a different disposition (e.g. the
		// Engineering finished without terminalizing the assignment). Re-project the
		// run to match the assignment fate below rather than failing the
		// cancellation.
	}
	run.State = engineeringTerminalRunState(assignment.Disposition)
	run.UpdatedAt, run.FinishedAt = now, &now
	if run.State == types.RunCompleted {
		run.Result, run.Error = resultRef, ""
	} else {
		run.Result, run.Error = "", reason
	}
	runUpdated, err := lifecycleObject(ogKindRun, assignment.Binding.OwnerID, assignment.Binding.ComputerID, run.RunID, run,
		engineeringRunMetadata(run, seq), runObj.CreatedAt, now)
	if err != nil {
		return nil, nil, err
	}
	return append(objects, runUpdated), append(conditions, engineeringObjectCondition(runObj)), nil
}

// SlotTerminalReport scans an assignment's recorded reports for settlement
// slot relevance (settlement gate item 3, v1). Partial reports use the
// separate nonterminal sequence and never compete. It returns the occupying
// report ID for a same-digest match (late or terminal: a match replays its
// receipt with no new effects) and, separately, a non-late terminal occupant
// with a different digest (only a non-late submission may conflict with it;
// late evidence never competes for terminal truth). Legacy rows without a
// stored digest recompute best-effort from the pinned belief; a stale overlay
// can only fail closed, never accept twice.
func (s *Store) SlotTerminalReport(ctx context.Context, assignment types.EngineeringAssignment, propositionDigest string) (matchID, matchCommandID, conflictID string, err error) {
	if assignment.PendingProposal != nil {
		if assignment.PendingProposal.PropositionDigest != propositionDigest {
			if conflictID == "" {
				conflictID = assignment.PendingProposal.Report.ReportID
			}
		}
	}
	for _, ref := range assignment.ReportRefs {
		obj, getErr := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(ref))
		if getErr != nil {
			if errors.Is(getErr, objectgraph.ErrNotFound) {
				continue
			}
			return "", "", "", getErr
		}
		stored, decodeErr := decodeLifecycleObject[types.EngineeringAssignmentReport](obj)
		if decodeErr != nil {
			continue
		}
		if stored.Result == types.EngineeringResultPartial {
			continue
		}
		digest := strings.TrimSpace(stored.PropositionDigest)
		if digest == "" {
			digest, decodeErr = ComputeTerminalPropositionDigest(
				assignment.Binding.SubjectDigest,
				stored.Result, stored.Verdict,
				stored.Commands, stored.Outputs, stored.EvidenceRefs)
			if decodeErr != nil {
				return "", "", stored.ReportID, fmt.Errorf("legacy terminal report proposition recompute failed: %w", decodeErr)
			}
		}
		if digest == propositionDigest {
			replayCommandID := strings.TrimSpace(stored.RecordCommandID)
			if replayCommandID == "" {
				replayCommandID = "co-super-report:" + assignment.AssignmentID + ":" + stored.ReportID
			}
			return stored.ReportID, replayCommandID, "", nil
		}
		if !stored.Late && conflictID == "" {
			conflictID = stored.ReportID
		}
	}
	return "", "", conflictID, nil
}

func (s *Store) RecordEngineeringAssignmentReport(ctx context.Context, req types.RecordEngineeringAssignmentReportRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	computedDigest, digestErr := ComputeRecordEngineeringAssignmentReportDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, req.Report.ReportID); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getEngineeringAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.BoundRunID == "" || assignment.Disposition == types.EngineeringAssignmentOpen {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	// v1 terminal settlement gate: the authoritative proposition digest is
	// computed over the submitted claims with the pinned pre-execution belief
	// before any derivation (late/verdict/candidate overlays below). Partial
	// reports never compete and skip the gate. A same-digest occupant (late or
	// terminal) replays its receipt with no new effects; a non-late terminal
	// occupant with a different digest conflicts a non-late submission before
	// any report, fate, outbox, wake, or physical effect. Late evidence never
	// competes for terminal truth.
	_, intentErr := s.GetLifecycleCancellationIntent(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	cancellationIntended := intentErr == nil
	if intentErr != nil && !errors.Is(intentErr, ErrNotFound) {
		return types.EngineeringAssignmentCommandResult{}, intentErr
	}
	propositionDigest := ""
	if req.Report.Result != types.EngineeringResultPartial {
		var propErr error
		propositionDigest, propErr = ComputeTerminalPropositionDigest(
			assignment.Binding.SubjectDigest,
			req.Report.Result, req.Report.Verdict,
			req.Report.Commands, req.Report.Outputs, req.Report.EvidenceRefs)
		if propErr != nil {
			return types.EngineeringAssignmentCommandResult{}, propErr
		}
		if req.Report.PropositionDigest != "" && req.Report.PropositionDigest != propositionDigest {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment report proposition digest mismatch: %s vs %s: %w", req.Report.PropositionDigest, propositionDigest, ErrEngineeringAssignmentInvalid)
		}
		expectedReportID := TerminalReportID(req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt, propositionDigest)
		if req.Report.PropositionDigest != "" && req.Report.ReportID != expectedReportID {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment report report_id mismatch: %s vs %s: %w", req.Report.ReportID, expectedReportID, ErrEngineeringAssignmentInvalid)
		}
	}
	pendingMatches := assignment.PendingProposal != nil && assignment.PendingProposal.PropositionDigest == propositionDigest && !cancellationIntended && !assignment.Disposition.Terminal()
	lateAuthority := (cancellationIntended || assignment.Disposition.Terminal() || assignment.CapsuleDisposition == types.EngineeringCapsuleRevokeRequested || assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked) && !pendingMatches
	if req.Report.Result != types.EngineeringResultPartial {
		matchID, matchCommandID, conflictID, scanErr := s.SlotTerminalReport(ctx, assignment, propositionDigest)
		if scanErr != nil {
			return types.EngineeringAssignmentCommandResult{}, scanErr
		}
		if matchID != "" {
			return s.ReplayRecordedEngineeringAssignmentReport(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt, matchID, matchCommandID)
		}
		if conflictID != "" {
			return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentCommandConflict
		}
	}
	var parentAuthority engineeringAuthorityObjects
	if lateAuthority {
		parentAuthority, err = s.requireEngineeringHistoricalParentAuthority(ctx, assignment.Binding)
	} else {
		parentAuthority, err = s.requireEngineeringParentAuthority(ctx, assignment.Binding)
	}
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	report := req.Report
	// Attestations embedded in model-authored report JSON are never evidence.
	report.ExecutionAttestations = nil
	report.Schema, report.AssignmentID, report.Attempt = types.EngineeringAssignmentSchemaV1, assignment.AssignmentID, assignment.Binding.Attempt
	report.OwnerID, report.ComputerID, report.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
	report.RunID, report.AssignedAgentID = assignment.BoundRunID, assignment.Binding.AssignedAgentID
	report.Late = lateAuthority
	report.PropositionDigest = propositionDigest
	report.RecordCommandID = req.CommandID
	if report.Late {
		report.CertifiesOriginalSubject, report.CandidateSubjectDigest, report.CandidateID, report.CandidateArtifactRef = false, "", "", ""
		if report.Verdict == types.EngineeringVerdictPass {
			report.Verdict = types.EngineeringVerdictAbstain
		}
	}
	report.Summary = strings.TrimSpace(report.Summary)
	if report.Summary == "" {
		report.Summary = fmt.Sprintf("Engineering assignment %s attempt %d reported %s", assignment.AssignmentID, assignment.Binding.Attempt, report.Result)
	}
	report.EvidenceRefs = normalizeLifecycleRefs(report.EvidenceRefs)
	report.CreatedAt = now
	changed := engineeringReportChangedSubject(report, assignment.Binding.SubjectDigest)
	var candidate *types.EngineeringSubjectCandidate
	var candidateObj objectgraph.Object
	candidateExists := false
	if changed && !report.Late {
		report.CandidateSubjectDigest = strings.TrimSpace(report.ObservedSubjectDigest)
		candidateKey := strings.Join([]string{
			assignment.Binding.SubjectDigest, report.CandidateSubjectDigest, assignment.AssignmentID,
			strconv.FormatUint(assignment.Binding.Attempt, 10), report.ReportID,
		}, "\x00")
		candidateID, buildErr := lifecycleCanonicalID(ogKindEngineeringCandidate, req.OwnerID, req.ComputerID, candidateKey)
		if buildErr != nil {
			return types.EngineeringAssignmentCommandResult{}, buildErr
		}
		report.CandidateID = candidateID
		created := types.EngineeringSubjectCandidate{Schema: types.EngineeringAssignmentSchemaV1, CandidateID: candidateID,
			OwnerID: req.OwnerID, ComputerID: req.ComputerID, TrajectoryID: assignment.Binding.TrajectoryID,
			AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
			OriginalSubjectDigest: assignment.Binding.SubjectDigest, SubjectDigest: report.CandidateSubjectDigest,
			SourceReportID: report.ReportID, ArtifactRef: report.CandidateArtifactRef, CreatedAt: now}
		existing, getErr := s.lifecycleGraph().GetObject(ctx, candidateID)
		if getErr == nil {
			decoded, decodeErr := decodeLifecycleObject[types.EngineeringSubjectCandidate](existing)
			if decodeErr != nil || decoded.OwnerID != req.OwnerID || decoded.ComputerID != req.ComputerID || decoded.SubjectDigest != report.CandidateSubjectDigest || decoded.ArtifactRef != report.CandidateArtifactRef {
				return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
			}
			candidate, candidateObj, candidateExists = &decoded, existing, true
		} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
			return types.EngineeringAssignmentCommandResult{}, getErr
		} else {
			candidate = &created
			candidateObj, buildErr = lifecycleObject(ogKindEngineeringCandidate, req.OwnerID, req.ComputerID, candidateKey, created,
				map[string]any{"candidate_id": candidateID, "computer_id": req.ComputerID, "trajectory_id": assignment.Binding.TrajectoryID,
					"subject_digest": report.CandidateSubjectDigest}, now, now)
			if buildErr != nil {
				return types.EngineeringAssignmentCommandResult{}, buildErr
			}
		}
	}
	// Certification is reducer-derived. Any authored value is ignored.
	report.CertifiesOriginalSubject = assignment.Binding.Kind == types.EngineeringAssignmentVerification && report.Verdict == types.EngineeringVerdictPass &&
		report.Result == types.EngineeringResultCompleted && !report.Late && !changed
	if len(req.ExecutionAttestations) > 0 && (assignment.GrantPolicyAttestation == nil || report.Late || report.Result == types.EngineeringResultPartial) {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: execution attestations cannot retro-certify old, late, or partial evidence: %w", ErrEngineeringAssignmentInvalid)
	}
	if assignment.GrantPolicyAttestation != nil && !report.Late && report.Result != types.EngineeringResultPartial {
		commandCount := len(report.Commands)
		if (commandCount == 0 && (len(report.ExecutorReceiptRefs) != 0 || len(req.ExecutionAttestations) != 0)) ||
			(commandCount > 0 && (len(report.ExecutorReceiptRefs) != commandCount || len(req.ExecutionAttestations) != commandCount)) {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: timely grant-attested command evidence requires exact receipt and attestation cardinality: %w", ErrEngineeringAssignmentInvalid)
		}
	}
	if len(req.ExecutionAttestations) > 0 {
		report.ExecutionAttestations = append([]types.EngineeringExecutionAttestation(nil), req.ExecutionAttestations...)
		for i := range report.ExecutionAttestations {
			att := &report.ExecutionAttestations[i]
			att.Schema = types.EngineeringExecutionAttestationSchemaV1
			att.AssignmentID, att.Attempt = assignment.AssignmentID, assignment.Binding.Attempt
			att.OwnerID, att.ComputerID, att.TrajectoryID = assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID
			att.RunID, att.CapsuleID, att.ReportID = assignment.BoundRunID, assignment.Binding.CapsuleID, report.ReportID
			att.ReportCommandID, att.ReportEventID, att.ReducerSeq, att.RecordedAt = req.CommandID, req.CommandID+":1", transition.seq, now
			att.AttestationRef = ""
			att.AttestationRef, err = executionAttestationRef(*att)
			if err != nil {
				return types.EngineeringAssignmentCommandResult{}, err
			}
		}
		if err := validateExecutionAttestations(report.ExecutionAttestations, report, assignment); err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
	}
	if err := report.ValidateAgainst(assignment); err != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	reportObj, err := lifecycleObject(ogKindEngineeringReport, req.OwnerID, req.ComputerID, report.ReportID, report, engineeringReportMetadata(report), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignment.ReportRefs = append(append([]string(nil), assignment.ReportRefs...), reportObj.CanonicalID)
	previousDisposition := assignment.Disposition
	if !report.Late {
		assignment.Disposition = reducerAssignmentOutcome(report, assignment, changed)
	}
	if assignment.Disposition != previousDisposition && assignment.Disposition.Terminal() {
		assignment.DispositionReason = "reducer-derived from report " + report.ReportID
		assignment.PendingProposal = nil
		assignment.TerminalAt = &now
	}
	assignment.LifecycleVersion++
	assignment.UpdatedAt = now
	if err := assignment.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	updatedAssignmentObj, err := lifecycleObject(ogKindEngineeringAssignment, req.OwnerID, req.ComputerID,
		engineeringAttemptKey(req.AssignmentID, req.Attempt), assignment, engineeringAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	conditions := append(engineeringParentAuthorityConditions(parentAuthority), engineeringObjectCondition(assignmentObj),
		objectgraph.ObjectCondition{CanonicalID: reportObj.CanonicalID})
	if report.Late && parentAuthority.trajectoryRec.Status != types.TrajectoryLive {
		// A live transition already CASes the trajectory object. Post-terminal
		// evidence advances the separate terminal sequence, so retain the exact
		// historical trajectory hash as an additional authority condition.
		conditions = append(conditions, engineeringObjectCondition(parentAuthority.trajectory))
	}
	objects := []objectgraph.Object{updatedAssignmentObj, reportObj}
	var update *types.CoagentSourcePacket
	if !report.Late {
		parentRun, parentChannelID, decodeErr := engineeringParentReturnTarget(parentAuthority, assignment.Binding)
		if decodeErr != nil {
			return types.EngineeringAssignmentCommandResult{}, decodeErr
		}
		created, updateObj, updateErr := buildEngineeringReturnPacket(now, transition.seq, assignment, report, parentRun, parentChannelID, false)
		if updateErr != nil {
			return types.EngineeringAssignmentCommandResult{}, updateErr
		}
		update = &created
		objects = append(objects, updateObj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID})
	}
	if assignment.Disposition.Terminal() && !report.Late {
		projectionObjects, projectionConditions, projectionErr := s.projectEngineeringTerminal(ctx, assignment, transition.seq, now, report.ReportID, assignment.DispositionReason)
		if projectionErr != nil {
			return types.EngineeringAssignmentCommandResult{}, projectionErr
		}
		objects = append(objects, projectionObjects...)
		conditions = append(conditions, projectionConditions...)
	}
	assignmentEdge, err := engineeringEdge(reportObj.CanonicalID, updatedAssignmentObj.CanonicalID, ogEdgeReportAssignment, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	edges := []objectgraph.Edge{assignmentEdge}
	if candidate != nil {
		if candidateExists {
			conditions = append(conditions, engineeringObjectCondition(candidateObj))
		} else {
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: candidateObj.CanonicalID})
			objects = append(objects, candidateObj)
		}
		candidateEdge, edgeErr := engineeringEdge(reportObj.CanonicalID, candidateObj.CanonicalID, ogEdgeReportCandidate, now)
		if edgeErr != nil {
			return types.EngineeringAssignmentCommandResult{}, edgeErr
		}
		edges = append(edges, candidateEdge)
	}
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleRecordEngineeringAssignment, types.LifecycleEngineeringAssignmentReported,
		req.CommandID, req.CommandDigest, assignment, &report, candidate, report.ReportID, assignment.DispositionReason,
		objects, conditions, edges, update, report.EvidenceRefs)
}

func (s *Store) CancelEngineeringAssignment(ctx context.Context, req types.CancelEngineeringAssignmentRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.Reason = strings.TrimSpace(req.Reason)
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 || req.Reason == "" {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	computedDigest, digestErr := ComputeCancelEngineeringAssignmentDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	cancelReportID := "cancel-report:" + objectgraph.SHA256([]byte(strings.Join([]string{req.AssignmentID, strconv.FormatUint(req.Attempt, 10), req.CommandID}, "\x00")))
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, cancelReportID); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getEngineeringAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion || assignment.Disposition.Terminal() {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	// Once exact revoke acknowledgement is durable, cancellation is system fate
	// completion. It authenticates the immutable assignment/control/run join and
	// never depends on the mutable parent Management activation remaining live.
	var parentAuthority engineeringAuthorityObjects
	if assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked {
		parentAuthority, err = s.requireEngineeringHistoricalParentAuthority(ctx, assignment.Binding)
	} else {
		parentAuthority, err = s.requireEngineeringParentAuthority(ctx, assignment.Binding)
	}
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignment.Disposition, assignment.DispositionReason = types.EngineeringAssignmentCancelled, req.Reason
	assignment.LifecycleVersion++
	assignment.UpdatedAt, assignment.TerminalAt = now, &now
	reportID := cancelReportID
	verdict := types.EngineeringVerdictNone
	if assignment.Binding.Kind == types.EngineeringAssignmentVerification {
		verdict = types.EngineeringVerdictAbstain
	}
	report := types.EngineeringAssignmentReport{
		Schema: types.EngineeringAssignmentSchemaV1, ReportID: reportID, AssignmentID: assignment.AssignmentID,
		Attempt: assignment.Binding.Attempt, OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		TrajectoryID: assignment.Binding.TrajectoryID, RunID: assignment.BoundRunID, AssignedAgentID: assignment.Binding.AssignedAgentID,
		Result: types.EngineeringResultFailed, Verdict: verdict, ObservedSubjectDigest: assignment.Binding.SubjectDigest,
		Late: true, Summary: req.Reason + "; assignment cancelled; late results are evidence-only and cannot reopen work", CreatedAt: now,
	}
	if err := report.ValidateAgainst(assignment); err != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	reportObj, err := lifecycleObject(ogKindEngineeringReport, req.OwnerID, req.ComputerID, report.ReportID, report, engineeringReportMetadata(report), now, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignment.ReportRefs = append(append([]string(nil), assignment.ReportRefs...), reportObj.CanonicalID)
	if err := assignment.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	updatedObj, err := lifecycleObject(ogKindEngineeringAssignment, req.OwnerID, req.ComputerID,
		engineeringAttemptKey(req.AssignmentID, req.Attempt), assignment, engineeringAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	parentRun, parentChannelID, decodeErr := engineeringParentReturnTarget(parentAuthority, assignment.Binding)
	if decodeErr != nil {
		return types.EngineeringAssignmentCommandResult{}, decodeErr
	}
	update, updateObj, err := buildEngineeringReturnPacket(now, transition.seq, assignment, report, parentRun, parentChannelID, true)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	projectionObjects, projectionConditions, err := s.projectEngineeringTerminal(ctx, assignment, transition.seq, now, report.ReportID, req.Reason)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	objects := []objectgraph.Object{updatedObj, reportObj, updateObj}
	objects = append(objects, projectionObjects...)
	conditions := append(engineeringParentAuthorityConditions(parentAuthority), engineeringObjectCondition(assignmentObj),
		objectgraph.ObjectCondition{CanonicalID: reportObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID})
	conditions = append(conditions, projectionConditions...)
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleCancelEngineeringAssignment, types.LifecycleEngineeringAssignmentCancelled,
		req.CommandID, req.CommandDigest, assignment, &report, nil, report.ReportID, req.Reason, objects,
		conditions, nil, &update, nil)
}

func validEngineeringCapsuleTransition(current, next types.EngineeringCapsuleDisposition, intentRef, ackRef string) bool {
	switch next {
	case types.EngineeringCapsuleFreezeRequested:
		return current == types.EngineeringCapsuleActive && intentRef != "" && ackRef == ""
	case types.EngineeringCapsuleFrozen:
		return current == types.EngineeringCapsuleFreezeRequested && intentRef != "" && ackRef != ""
	case types.EngineeringCapsuleRevokeRequested:
		return (current == types.EngineeringCapsuleUnbound || current == types.EngineeringCapsuleActive || current == types.EngineeringCapsuleFreezeRequested || current == types.EngineeringCapsuleFrozen) && intentRef != "" && ackRef == ""
	case types.EngineeringCapsuleRevoked:
		return current == types.EngineeringCapsuleRevokeRequested && intentRef != "" && strings.HasPrefix(ackRef, "capsule-revoke:sha256:") &&
			types.ValidSHA256Digest(strings.TrimPrefix(ackRef, "capsule-revoke:"))
	default:
		return false
	}
}

func (s *Store) SetEngineeringCapsuleDisposition(ctx context.Context, req types.SetEngineeringCapsuleDispositionRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.OwnerID, req.ComputerID, req.AssignmentID = strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID), strings.TrimSpace(req.AssignmentID)
	req.IntentRef, req.AckRef = strings.TrimSpace(req.IntentRef), strings.TrimSpace(req.AckRef)
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	computedDigest, digestErr := ComputeSetEngineeringCapsuleDispositionDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Attempt, ""); found || err != nil {
		return replay, err
	}
	assignmentObj, assignment, err := s.getEngineeringAssignmentObject(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.LifecycleVersion != req.ExpectedLifecycleVersion ||
		(assignment.BoundRunID == "" && assignment.Disposition != types.EngineeringAssignmentOpen) ||
		!validEngineeringCapsuleTransition(assignment.CapsuleDisposition, req.Disposition, req.IntentRef, req.AckRef) {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	if (req.Disposition == types.EngineeringCapsuleFrozen || req.Disposition == types.EngineeringCapsuleRevoked) && req.IntentRef != assignment.CapsuleIntentRef {
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
	}
	if (assignment.GrantPolicyAttestation != nil || len(assignment.CapsuleFateHistory) > 0) && req.FateStep == nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: fate-attested assignment requires one fate step per transition: %w", ErrEngineeringAssignmentInvalid)
	}
	if _, intentErr := s.GetLifecycleCancellationIntent(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID); intentErr == nil {
		if req.Disposition != types.EngineeringCapsuleRevokeRequested && req.Disposition != types.EngineeringCapsuleRevoked {
			return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentInvalid
		}
	} else if !errors.Is(intentErr, ErrNotFound) {
		return types.EngineeringAssignmentCommandResult{}, intentErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, req.OwnerID, req.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, trajectoryObj, trajectory, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	assignment.CapsuleDisposition, assignment.CapsuleIntentRef, assignment.CapsuleAckRef = req.Disposition, req.IntentRef, req.AckRef
	assignment.LifecycleVersion++
	if req.PendingProposal != nil {
		copy := *req.PendingProposal
		assignment.PendingProposal = &copy
	}
	assignment.UpdatedAt = now
	if req.FateStep != nil {
		step := *req.FateStep
		step.Schema = types.EngineeringCapsuleFateStepSchemaV1
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
			return types.EngineeringAssignmentCommandResult{}, err
		}
		assignment.CapsuleFateHistory = append(append([]types.EngineeringCapsuleFateStep(nil), assignment.CapsuleFateHistory...), step)
	}
	if err := assignment.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if err := validateFateHistory(assignment.CapsuleFateHistory, assignment); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	updatedObj, err := lifecycleObject(ogKindEngineeringAssignment, req.OwnerID, req.ComputerID,
		engineeringAttemptKey(req.AssignmentID, req.Attempt), assignment, engineeringAssignmentMetadata(assignment), assignmentObj.CreatedAt, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	evidenceRefs := []string{req.IntentRef}
	if req.AckRef != "" {
		evidenceRefs = append(evidenceRefs, req.AckRef)
	}
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleSetEngineeringCapsuleDisposition, types.LifecycleEngineeringCapsuleDispositionSet,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", string(req.Disposition), []objectgraph.Object{updatedObj},
		[]objectgraph.ObjectCondition{engineeringObjectCondition(assignmentObj)}, nil, nil, evidenceRefs)
}

func (s *Store) GetEngineeringAssignmentReport(ctx context.Context, ownerID, computerID, reportID string) (types.EngineeringAssignmentReport, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindEngineeringReport, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(reportID))
	if err != nil {
		return types.EngineeringAssignmentReport{}, err
	}
	report, err := decodeLifecycleObject[types.EngineeringAssignmentReport](obj)
	if err != nil {
		return types.EngineeringAssignmentReport{}, err
	}
	if report.OwnerID != strings.TrimSpace(ownerID) || report.ComputerID != strings.TrimSpace(computerID) || report.ReportID != strings.TrimSpace(reportID) {
		return types.EngineeringAssignmentReport{}, ErrNotFound
	}
	return report, nil
}

func (s *Store) GetEngineeringSubjectCandidate(ctx context.Context, ownerID, computerID, candidateID string) (types.EngineeringSubjectCandidate, error) {
	obj, err := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(candidateID))
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return types.EngineeringSubjectCandidate{}, ErrNotFound
		}
		return types.EngineeringSubjectCandidate{}, err
	}
	candidate, err := decodeLifecycleObject[types.EngineeringSubjectCandidate](obj)
	if err != nil {
		return types.EngineeringSubjectCandidate{}, err
	}
	if candidate.CandidateID != strings.TrimSpace(candidateID) || candidate.OwnerID != strings.TrimSpace(ownerID) || candidate.ComputerID != strings.TrimSpace(computerID) {
		return types.EngineeringSubjectCandidate{}, ErrNotFound
	}
	return candidate, nil
}

// RecordEngineeringOrphanObservation processes an authenticated immutable orphan observation (settlement gate item 6).
// If the child run has an assignment obligation, the reducer validates the slot family:
// - An in-flight pending proposal must reconcile through the fate saga, never through the orphan path.
// - An already terminal assignment replays or conflicts.
// - An unreserved bound assignment is closed by the reducer deriving a terminal failed proposition.
func (s *Store) RecordEngineeringOrphanObservation(ctx context.Context, obs types.EngineeringOrphanObservation) (types.EngineeringAssignmentCommandResult, error) {
	if err := obs.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if strings.TrimSpace(obs.AssignmentID) == "" {
		return types.EngineeringAssignmentCommandResult{}, nil
	}
	assignmentObj, assignment, err := s.getEngineeringAssignmentObject(ctx, obs.OwnerID, obs.ComputerID, obs.AssignmentID, obs.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	_ = assignmentObj
	// Obligation check: if a pending proposal is in-flight, route exclusively through fate saga.
	if assignment.PendingProposal != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: pending proposal in-flight; must reconcile through fate saga, not orphan path: %w", ErrEngineeringAssignmentCommandConflict)
	}
	if assignment.BoundRunID != "" && assignment.BoundRunID != obs.RunID {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: orphan run %s does not match bound run %s: %w", obs.RunID, assignment.BoundRunID, ErrEngineeringAssignmentInvalid)
	}
	// Terminal check: if already terminal, replay the recorded orphan close when this
	// observation's derived report is the occupant; otherwise conflict. The report ID
	// is derived identically to the close path below, so a retry replays instead of
	// conflicting with itself.
	if assignment.Disposition.Terminal() {
		commandID := fmt.Sprintf("co-super-orphan:%s:%d:%s", obs.AssignmentID, obs.Attempt, obs.RunID)
		replayVerdict := types.EngineeringVerdictNone
		if assignment.Binding.Kind == types.EngineeringAssignmentVerification {
			replayVerdict = types.EngineeringVerdictAbstain
		}
		replayPropDigest, _ := ComputeTerminalPropositionDigest(assignment.Binding.SubjectDigest, types.EngineeringResultFailed, replayVerdict, nil, nil, nil)
		replayReportID := TerminalReportID(obs.OwnerID, obs.ComputerID, obs.AssignmentID, obs.Attempt, replayPropDigest)
		if replay, replayErr := s.ReplayRecordedEngineeringAssignmentReport(ctx, obs.OwnerID, obs.ComputerID, obs.AssignmentID, obs.Attempt, replayReportID, commandID); replayErr == nil {
			return replay, nil
		}
		return types.EngineeringAssignmentCommandResult{}, ErrEngineeringAssignmentCommandConflict
	}
	verdict := types.EngineeringVerdictNone
	if assignment.Binding.Kind == types.EngineeringAssignmentVerification {
		verdict = types.EngineeringVerdictAbstain
	}
	orphanPropDigest, _ := ComputeTerminalPropositionDigest(assignment.Binding.SubjectDigest, types.EngineeringResultFailed, verdict, nil, nil, nil)
	reportID := TerminalReportID(obs.OwnerID, obs.ComputerID, obs.AssignmentID, obs.Attempt, orphanPropDigest)
	report := types.EngineeringAssignmentReport{
		Schema:                types.EngineeringAssignmentSchemaV1,
		ReportID:              reportID,
		Result:                types.EngineeringResultFailed,
		Verdict:               verdict,
		ObservedSubjectDigest: assignment.Binding.SubjectDigest,
		Summary:               fmt.Sprintf("orphan close: child run %s terminated without packet (%s)", obs.RunID, obs.Reason),
		EvidenceRefs:          nil,
		CreatedAt:             obs.ObservedAt,
	}
	req := types.RecordEngineeringAssignmentReportRequest{
		CommandID:                fmt.Sprintf("co-super-orphan:%s:%d:%s", obs.AssignmentID, obs.Attempt, obs.RunID),
		OwnerID:                  obs.OwnerID,
		ComputerID:               obs.ComputerID,
		AssignmentID:             obs.AssignmentID,
		Attempt:                  obs.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion,
		Report:                   report,
	}
	req.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(req)
	return s.RecordEngineeringAssignmentReport(ctx, req)
}
