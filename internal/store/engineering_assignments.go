package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
	// The composed inbox command is identified by its own replay digest,
	// which leaves commit times out.
	inboxDigest, err := ComputeCommitInboxRecordsDigest(req.Records)
	if err != nil {
		return "", err
	}
	req.Records = types.CommitInboxRecordsRequest{CommandID: strings.TrimSpace(req.Records.CommandID), CommandDigest: inboxDigest}
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
// prefixed with it; the receipt is docs/archive/evidence/choir-rlm-settlement-item3-define-2026-09-09.md.
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
	return "engineering-grant:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func executionAttestationRef(att types.EngineeringExecutionAttestation) (string, error) {
	att.AttestationRef = ""
	payload, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	return "engineering-execution:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

func fateStepRef(step types.EngineeringCapsuleFateStep) (string, error) {
	step.StepRef = ""
	payload, err := json.Marshal(step)
	if err != nil {
		return "", err
	}
	return "engineering-fate:sha256:" + strings.TrimPrefix(objectgraph.SHA256(payload), "sha256:"), nil
}

// engineeringDigestRefMatches accepts a stored digest ref equal to want, or
// the same digest under the legacy co-super- spelling minted before refs
// became fixed points of the vocabulary rule
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md).
func engineeringDigestRefMatches(got, want string) bool {
	if got == want {
		return true
	}
	rest, ok := strings.CutPrefix(want, "engineering-")
	return ok && got == "co-super-"+rest
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
	if err != nil || !engineeringDigestRefMatches(att.AttestationRef, want) {
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
		nameErr := func(field string) error {
			return fmt.Errorf("co-super assignment: invalid runtime execution attestation (command %d/%d %s): %w", i, len(atts)-1, field, ErrEngineeringAssignmentInvalid)
		}
		if att.Schema != types.EngineeringExecutionAttestationSchemaV1 {
			return nameErr("schema")
		}
		if att.AssignmentID != assignment.AssignmentID || att.Attempt != assignment.Binding.Attempt {
			return nameErr("assignment/attempt")
		}
		if att.OwnerID != assignment.Binding.OwnerID || att.ComputerID != assignment.Binding.ComputerID || att.TrajectoryID != assignment.Binding.TrajectoryID {
			return nameErr("owner/computer/trajectory")
		}
		if att.RunID != assignment.BoundRunID || att.CapsuleID != assignment.Binding.CapsuleID || att.ReportID != report.ReportID {
			return nameErr("run/capsule/report")
		}
		if att.CommandID != command.CommandID || att.CommandDigest != command.CommandDigest || att.ExitCode != command.ExitCode {
			return nameErr("command binding")
		}
		if att.GrantedReceiptRef != report.ExecutorReceiptRefs[i] || !validTypedDigestRef(att.GrantedReceiptRef, "capsule-granted-exec:") {
			return nameErr("granted receipt ref")
		}
		if !att.Granted || !att.Frozen {
			return nameErr("granted/frozen flags")
		}
		if !types.ValidSHA256Digest(att.StdoutDigest) || !types.ValidSHA256Digest(att.StderrDigest) {
			return nameErr("output digests")
		}
		if att.SourceSubjectDigest != assignment.Binding.SubjectDigest {
			return nameErr("source subject digest")
		}
		if att.FinalSubjectDigest != report.ObservedSubjectDigest {
			return nameErr("final subject digest")
		}
		switch att.SubjectBinding {
		case "":
			if att.WorktreeDigest != att.FinalSubjectDigest {
				return nameErr("final subject digest")
			}
		case types.EngineeringSubjectBindingObserved:
			// Only a report that claims nothing about the subject may carry
			// receipts that do not certify it; the receipt's own observed
			// tree is recorded beside the frozen subject.
			if report.Result != types.EngineeringResultFailed && report.Result != types.EngineeringResultBlocked {
				return nameErr("subject binding")
			}
			if att.WorktreeDigest != "" && !types.ValidSHA256Digest(att.WorktreeDigest) {
				return nameErr("observed worktree digest")
			}
		default:
			return nameErr("subject binding")
		}
		if !validCanonicalTime(att.OccurredAt) || strings.TrimSpace(att.ReportCommandID) == "" {
			return nameErr("occurred_at/report command")
		}
		if att.ReportEventID != att.ReportCommandID+":1" || att.ReducerSeq <= 0 || !validCanonicalTime(att.RecordedAt) {
			return nameErr("report event/seq")
		}
		if seen[att.AttestationRef] {
			return nameErr("duplicate attestation ref")
		}
		want, err := executionAttestationRef(att)
		if err != nil || !engineeringDigestRefMatches(att.AttestationRef, want) {
			return fmt.Errorf("co-super assignment: execution attestation digest mismatch (command %d): %w", i, ErrEngineeringAssignmentInvalid)
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
		if err != nil || !engineeringDigestRefMatches(step.StepRef, want) {
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

// requireEngineeringParentAuthority authenticates the parent scope of a live
// transition (bind, timely report, cancel before revoke). Records bindings
// (C1e D4) rest on the live trajectory the Texture's document names; the
// conversation round is checked at admission, inside the open's lock. A
// binding minted before C1e (work-item parents) has no live authority: it
// may only be cancelled or receive late evidence.
func (s *Store) requireEngineeringParentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding) (engineeringAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return engineeringAuthorityObjects{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if !binding.Records() {
		return engineeringAuthorityObjects{}, fmt.Errorf("co-super assignment: a pre-records binding has no live authority: %w", ErrEngineeringAssignmentInvalid)
	}
	return s.requireEngineeringRecordsAuthority(ctx, binding, false)
}

// requireEngineeringHistoricalParentAuthority authenticates a late report or
// a system cancellation after the live scope ended: the trajectory may be
// settled or cancelled, and a pre-records binding is accepted on its
// trajectory alone so its row can still turn terminal.
func (s *Store) requireEngineeringHistoricalParentAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding) (engineeringAuthorityObjects, error) {
	if err := binding.Validate(); err != nil {
		return engineeringAuthorityObjects{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	return s.requireEngineeringRecordsAuthority(ctx, binding, true)
}

func (s *Store) requireEngineeringRecordsAuthority(ctx context.Context, binding types.EngineeringAssignmentBinding, historical bool) (engineeringAuthorityObjects, error) {
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
	return engineeringAuthorityObjects{trajectory: trajectoryObj, trajectoryRec: trajectory}, nil
}

// requireEngineeringAssignmentAuthority adds the assigned lifecycle
// Engineering agent to the live parent authority (bind).
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
	authority.assignedAgent = assignedAgentObj
	return authority, nil
}

// requireEngineeringRecordsAdmission is D4's admission check, run inside the
// open's lock against the request it will commit: the request carries
// exactly one Cast, from management to the assignment's engineering inbox,
// whose ParentID names a Cast record in management's inbox from a Texture;
// that record's round is the binding's parent round and is still open; the
// Texture's document names the binding's trajectory; and the engineering
// conversation has never opened. It returns the parent conversation head
// object so the open can compare-and-set on it.
func (s *Store) requireEngineeringRecordsAdmission(ctx context.Context, assignmentID string, binding types.EngineeringAssignmentBinding, records types.CommitInboxRecordsRequest) (objectgraph.Object, error) {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("co-super assignment: records admission: "+format+": %w", append(args, ErrEngineeringAssignmentInvalid)...)
	}
	if binding.AssignedAgentID != agentprofile.Engineering+":"+assignmentID {
		return objectgraph.Object{}, refuse("assigned inbox %s is not engineering:%s", binding.AssignedAgentID, assignmentID)
	}
	if records.OwnerID != binding.OwnerID || records.ComputerID != binding.ComputerID {
		return objectgraph.Object{}, refuse("records command is outside the binding's scope")
	}
	var cast *types.CommitmentRecord
	for i := range records.Records {
		rec := records.Records[i]
		if strings.TrimSpace(rec.Inbox) != binding.AssignedAgentID {
			continue
		}
		if cast != nil {
			return objectgraph.Object{}, refuse("more than one record addressed to %s", binding.AssignedAgentID)
		}
		cast = &records.Records[i]
	}
	if cast == nil || !cast.IsCast() || strings.TrimSpace(cast.RecordID) != binding.ParentControlID ||
		strings.TrimSpace(cast.SenderInbox) != binding.ParentAgentID || strings.TrimSpace(cast.Provenance.AgentID) != binding.ParentAgentID {
		return objectgraph.Object{}, refuse("the command carries no Cast %s from %s to %s", binding.ParentControlID, binding.ParentAgentID, binding.AssignedAgentID)
	}
	parent, err := s.GetCommitmentRecord(ctx, binding.OwnerID, binding.ComputerID, strings.TrimSpace(cast.ParentID))
	if err != nil {
		return objectgraph.Object{}, err
	}
	if parent == nil || parent.InboxSeq <= 0 || parent.Inbox != binding.ParentAgentID || !parent.IsCast() ||
		!strings.HasPrefix(parent.SenderInbox, agentprofile.Texture+":") || parent.ConversationID != binding.ParentConversationID || parent.Round != binding.ParentRound {
		return objectgraph.Object{}, refuse("parent %q is not a Texture's Cast into %s round %d of %s", cast.ParentID, binding.ParentAgentID, binding.ParentRound, binding.ParentConversationID)
	}
	headObj, err := s.lifecycleGetObject(ctx, ogKindConversationHead, binding.OwnerID, binding.ComputerID, binding.ParentConversationID)
	if err != nil {
		return objectgraph.Object{}, refuse("parent conversation %s has no head: %v", binding.ParentConversationID, err)
	}
	head, err := decodeLifecycleObject[types.ConversationHead](headObj)
	if err != nil {
		return objectgraph.Object{}, err
	}
	if !head.Open || head.Round != binding.ParentRound || head.CasterInbox != parent.SenderInbox || head.RecipientInbox != binding.ParentAgentID {
		return objectgraph.Object{}, refuse("round %d of %s is not open", binding.ParentRound, binding.ParentConversationID)
	}
	docID := strings.TrimPrefix(parent.SenderInbox, agentprofile.Texture+":")
	document, err := s.GetLifecycleDocument(ctx, binding.OwnerID, binding.ComputerID, docID)
	if err != nil {
		return objectgraph.Object{}, refuse("document %s: %v", docID, err)
	}
	if strings.TrimSpace(document.TrajectoryID) == "" || document.TrajectoryID != binding.TrajectoryID {
		return objectgraph.Object{}, refuse("document %s names trajectory %q, the binding %q", docID, document.TrajectoryID, binding.TrajectoryID)
	}
	if exists, err := s.lifecycleObjectExists(ctx, ogKindConversationHead, binding.OwnerID, binding.ComputerID, binding.AssignedConversationID); err != nil {
		return objectgraph.Object{}, err
	} else if exists {
		return objectgraph.Object{}, refuse("engineering conversation %s already exists; a retry is a new Cast", binding.AssignedConversationID)
	}
	return headObj, nil
}

func (s *Store) lifecycleObjectExists(ctx context.Context, kind objectgraph.ObjectKind, ownerID, computerID, key string) (bool, error) {
	_, err := s.lifecycleGetObject(ctx, kind, ownerID, computerID, key)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func engineeringParentAuthorityConditions(authority engineeringAuthorityObjects) []objectgraph.ObjectCondition {
	return nil
}

func engineeringAuthorityConditions(authority engineeringAuthorityObjects) []objectgraph.ObjectCondition {
	if authority.assignedAgent.CanonicalID == "" {
		return nil
	}
	return []objectgraph.ObjectCondition{engineeringObjectCondition(authority.assignedAgent)}
}

// ListEngineeringAssignmentsByParentRound returns the assignments whose
// parent is one round of management's conversation with a Texture (C1e D5,
// D6): a scan of the owner's assignment objects filtered on the binding's
// parent fields. There is no new table.
func (s *Store) ListEngineeringAssignmentsByParentRound(ctx context.Context, ownerID, computerID, conversationID string, round int) ([]types.EngineeringAssignment, error) {
	ownerID, computerID, conversationID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(conversationID)
	all, err := s.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		return nil, err
	}
	var out []types.EngineeringAssignment
	for _, assignment := range all {
		if assignment.Binding.OwnerID == ownerID && assignment.Binding.Records() &&
			assignment.Binding.ParentConversationID == conversationID && assignment.Binding.ParentRound == round {
			out = append(out, assignment)
		}
	}
	return out, nil
}

// EngineeringAssignmentForInbox returns the records assignment whose
// engineering inbox is engineering:<assignmentID> (C1e D1).
func (s *Store) EngineeringAssignmentForInbox(ctx context.Context, ownerID, computerID, inbox string) (types.EngineeringAssignment, error) {
	assignmentID, ok := strings.CutPrefix(strings.TrimSpace(inbox), agentprofile.Engineering+":")
	if !ok || assignmentID == "" {
		return types.EngineeringAssignment{}, ErrNotFound
	}
	assignment, err := s.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, 1)
	if err != nil {
		return types.EngineeringAssignment{}, err
	}
	if !assignment.Binding.Records() || assignment.Binding.AssignedAgentID != strings.TrimSpace(inbox) {
		return types.EngineeringAssignment{}, ErrNotFound
	}
	return assignment, nil
}

// EngineeringOutcomeRecordID is the id of the final Report record an
// assignment report closes the engineering round with (C1e D3, D6): derived
// from the assignment report's id, so the row's ReportRefs lead to it.
func EngineeringOutcomeRecordID(reportID string) string {
	return "assignment-report:" + strings.TrimSpace(reportID)
}

// EngineeringClosingRecord returns the record that closed an assignment's
// engineering round, found by id from the row's ReportRefs (C1e D6). found
// is false when none exists: the row is not terminal, or a crash boot
// terminalized it after closing the round silently.
func (s *Store) EngineeringClosingRecord(ctx context.Context, assignment types.EngineeringAssignment) (types.CommitmentRecord, bool, error) {
	ownerID, computerID := assignment.Binding.OwnerID, assignment.Binding.ComputerID
	for i := len(assignment.ReportRefs) - 1; i >= 0; i-- {
		obj, err := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(assignment.ReportRefs[i]))
		if err != nil {
			if errors.Is(err, objectgraph.ErrNotFound) {
				continue
			}
			return types.CommitmentRecord{}, false, err
		}
		report, err := decodeLifecycleObject[types.EngineeringAssignmentReport](obj)
		if err != nil {
			continue
		}
		records, err := s.listInboxRecordsBy(ctx, ownerID, computerID, "record_id", EngineeringOutcomeRecordID(report.ReportID))
		if err != nil {
			return types.CommitmentRecord{}, false, err
		}
		for _, rec := range records {
			if rec.Final && rec.ConversationID == assignment.Binding.AssignedConversationID {
				return rec, true, nil
			}
		}
	}
	return types.CommitmentRecord{}, false, nil
}

// EngineeringFreezeOperationID and EngineeringFreezeIdempotencyKey name the
// one self-development operation an assignment's Freeze opens (C1e D2):
// every assignment-scoped caller resolves its operation by the key, never by
// the trajectory.
func EngineeringFreezeOperationID(computerID, assignmentID string) string {
	return "selfdev-" + computerevent.DigestBytes([]byte(strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(assignmentID)))[:32]
}

func EngineeringFreezeIdempotencyKey(assignmentID string) string {
	return "freeze-open:" + strings.TrimSpace(assignmentID)
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

// replaySameTerminalReportCommand answers a terminal report whose command id
// is already committed under another digest. Two writers of one report (the
// worker's Complete and the fate wake's resume of the staged proposal) share
// the command id, because the report id derives from the proposition, but
// carry different evidence. When this command committed this very
// proposition, the request is the same report: it replays the committed
// result. Anything else (a partial, another proposition, another command's
// slot) is not answered here, and the caller keeps the conflict.
func (s *Store) replaySameTerminalReportCommand(ctx context.Context, req types.RecordEngineeringAssignmentReportRequest) (types.EngineeringAssignmentCommandResult, bool, error) {
	if req.Report.Result == types.EngineeringResultPartial {
		return types.EngineeringAssignmentCommandResult{}, false, nil
	}
	assignment, err := s.GetEngineeringAssignment(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, false, err
	}
	digest, err := ComputeTerminalPropositionDigest(assignment.Binding.SubjectDigest, req.Report.Result, req.Report.Verdict,
		req.Report.Commands, req.Report.Outputs, req.Report.EvidenceRefs)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, false, err
	}
	// The fresh path's checks first: a stated proposition digest must be
	// the one the report computes, and its report id the one that digest
	// derives. A request the fresh path refuses is never acknowledged as
	// landed (review round 4b, Sonnet F2).
	if stated := strings.TrimSpace(req.Report.PropositionDigest); stated != "" {
		if stated != digest {
			return types.EngineeringAssignmentCommandResult{}, false, fmt.Errorf("co-super assignment report proposition digest mismatch: %s vs %s: %w", stated, digest, ErrEngineeringAssignmentInvalid)
		}
		if expected := TerminalReportID(req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt, digest); req.Report.ReportID != expected {
			return types.EngineeringAssignmentCommandResult{}, false, fmt.Errorf("co-super assignment report report_id mismatch: %s vs %s: %w", req.Report.ReportID, expected, ErrEngineeringAssignmentInvalid)
		}
	}
	matchID, matchCommandID, _, err := s.SlotTerminalReport(ctx, assignment, digest)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, false, err
	}
	if matchID == "" || matchID != strings.TrimSpace(req.Report.ReportID) || matchCommandID != req.CommandID {
		return types.EngineeringAssignmentCommandResult{}, false, nil
	}
	replay, err := s.ReplayRecordedEngineeringAssignmentReport(ctx, req.OwnerID, req.ComputerID, req.AssignmentID, req.Attempt, matchID, matchCommandID)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, false, err
	}
	// The replay acknowledges the proposition, not this writer's evidence:
	// say so when that evidence differs from the committed report's
	// (review round 4b, Sonnet F7).
	if replay.Report != nil {
		if differs := reportEvidenceDifferences(*replay.Report, req); len(differs) > 0 {
			log.Printf("store: report %s replayed under command %s; this writer's evidence differs from the committed report (%s), which stands",
				matchID, matchCommandID, strings.Join(differs, ", "))
		}
	}
	return replay, true, nil
}

// reportEvidenceDifferences names the evidence fields in which a second
// writer's request differs from the committed report.
func reportEvidenceDifferences(committed types.EngineeringAssignmentReport, req types.RecordEngineeringAssignmentReportRequest) []string {
	var differs []string
	if !slices.Equal(committed.ExecutorReceiptRefs, req.Report.ExecutorReceiptRefs) {
		differs = append(differs, "executor receipts")
	}
	if committed.CandidateArtifactRef != req.Report.CandidateArtifactRef {
		differs = append(differs, "candidate")
	}
	if !reflect.DeepEqual(committed.Mutations, req.Report.Mutations) {
		differs = append(differs, "mutations")
	}
	if len(committed.ExecutionAttestations) != len(req.ExecutionAttestations) {
		differs = append(differs, "attestations")
	}
	return differs
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
	// The command that recorded the report is authoritative; the caller's id
	// is a derivation whose spelling changed (co-super- to engineering-).
	if recorded := strings.TrimSpace(report.RecordCommandID); recorded != "" {
		commandID = recorded
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

// commitEngineeringLifecycleCommand commits one assignment transition.
// artifactObjects are the transition's own objects, named on its event;
// inboxObjects are the records (and their heads, wakes and cursor) the
// transition composes into the same batch (C1e D3, D4), committed but not
// listed as artifacts.
func (s *Store) commitEngineeringLifecycleCommand(ctx context.Context, transition engineeringLifecycleTransition, commandKind types.LifecycleCommandKind, eventKind types.LifecycleEventKind, commandID, digest string, assignment types.EngineeringAssignment, report *types.EngineeringAssignmentReport, candidate *types.EngineeringSubjectCandidate, reportID, reason string, artifactObjects []objectgraph.Object, conditions []objectgraph.ObjectCondition, edges []objectgraph.Edge, evidenceRefs []string, inboxObjects ...objectgraph.Object) (types.EngineeringAssignmentCommandResult, error) {
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
	objects = append(objects, inboxObjects...)
	objects = append(objects, eventObj, receiptObj)
	lifecycleResult, err := s.commitLifecycleTransition(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		commandID, digest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: transition.trajectory, Events: []types.LifecycleEvent{event}}, edges...)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if lifecycleResult.Replay {
		replayed, _, replayErr := s.replayEngineeringAssignmentCommand(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
			commandID, digest, assignment.AssignmentID, assignment.Binding.Attempt, reportID)
		return replayed, replayErr
	}
	return types.EngineeringAssignmentCommandResult{Receipt: receipt, Assignment: assignment, Report: report, Candidate: candidate}, nil
}

// OpenEngineeringAssignment admits one records assignment (C1e D4): the
// casting management cell's whole inbox command (its records, the
// engineering Cast among them, its cursor advance and verdicts) is planned
// inside this lock and commits in the same batch as the assignment row, the
// assigned agent and the spawn obligation. A failed admission writes
// nothing: no Cast, no round, no row.
func (s *Store) OpenEngineeringAssignment(ctx context.Context, req types.OpenEngineeringAssignmentRequest) (types.EngineeringAssignmentCommandResult, error) {
	req.CommandID, req.CommandDigest, req.AssignmentID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.AssignmentID)
	if err := validateEngineeringCommand(req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if err := req.Binding.Validate(); err != nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("%w: %v", ErrEngineeringAssignmentInvalid, err)
	}
	if !req.Binding.Records() || strings.TrimSpace(req.AssignedAgent.AgentID) != req.Binding.AssignedAgentID {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: only a records cast opens an assignment: %w", ErrEngineeringAssignmentInvalid)
	}
	computedDigest, digestErr := ComputeOpenEngineeringAssignmentDigest(req)
	if err := requireEngineeringCommandDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	records, err := normalizeInboxCommand(req.Records)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if s.ogStore == nil {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("open engineering assignment: object graph not initialized")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayEngineeringAssignmentCommand(ctx, req.Binding.OwnerID, req.Binding.ComputerID, req.CommandID, req.CommandDigest, req.AssignmentID, req.Binding.Attempt, ""); found || err != nil {
		if err == nil {
			// The cell's records committed in the open's batch: their receipt
			// sequence numbers come from the stored inbox command.
			inbox, inboxFound, inboxErr := s.replayInboxCommand(ctx, records.OwnerID, records.ComputerID, records.CommandID, records.CommandDigest)
			if inboxErr != nil {
				return types.EngineeringAssignmentCommandResult{}, inboxErr
			}
			if inboxFound {
				replay.Records = &inbox
			}
		}
		return replay, err
	}
	if _, found, err := s.replayInboxCommand(ctx, records.OwnerID, records.ComputerID, records.CommandID, records.CommandDigest); found || err != nil {
		if err == nil {
			err = fmt.Errorf("co-super assignment: inbox command %s committed without its assignment: %w", records.CommandID, ErrEngineeringAssignmentInvalid)
		}
		return types.EngineeringAssignmentCommandResult{}, err
	}
	parentHead, err := s.requireEngineeringRecordsAdmission(ctx, req.AssignmentID, req.Binding, records)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	authority, err := s.requireEngineeringParentAuthority(ctx, req.Binding)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	now := time.Now().UTC()
	// U4: a Cast whose spec names a rebase notice is bound to it inside this
	// lock, and the pair's claim joins the batch under an absent-object
	// condition, so two racing opens cannot both commit.
	rebaseClaim, rebaseConditions, err := s.requireRebaseNoticeAdmission(ctx, req, records, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	transition, err := s.prepareEngineeringLifecycleTransition(ctx, authority.trajectory, authority.trajectoryRec, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	inboxObjects, inboxConditions, inboxEdges, inboxReceipt, err := s.planInboxCommand(ctx, records, now)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	openedOne := false
	for _, row := range inboxReceipt.Records {
		if row.RecordID == req.Binding.ParentControlID {
			openedOne = row.OpenedRound && row.Round == 1 && row.ConversationID == req.Binding.AssignedConversationID
		}
	}
	if !openedOne {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("co-super assignment: the Cast did not open round 1 of %s: %w", req.Binding.AssignedConversationID, ErrEngineeringAssignmentInvalid)
	}

	assignedAgent := req.AssignedAgent
	assignedAgent.AgentID, assignedAgent.OwnerID, assignedAgent.ComputerID = req.Binding.AssignedAgentID, req.Binding.OwnerID, req.Binding.ComputerID
	assignedAgent.Profile, assignedAgent.Role = agentprofile.Engineering, agentprofile.Engineering
	assignedAgent.ChannelID, assignedAgent.ActiveRunID = req.Binding.AssignedAgentID, ""
	assignedAgent.LifecycleVersion, assignedAgent.LastReducerSeq = 1, transition.seq
	assignedAgent.CreatedAt, assignedAgent.UpdatedAt = now, now

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
	edges := append([]objectgraph.Edge(nil), inboxEdges...)
	for _, target := range []struct {
		obj  objectgraph.Object
		kind objectgraph.EdgeKind
	}{{authority.trajectory, ogEdgeAssignmentTrajectory}, {agentObj, ogEdgeAssignmentAgent}} {
		edge, edgeErr := engineeringEdge(assignmentObj.CanonicalID, target.obj.CanonicalID, target.kind, now)
		if edgeErr != nil {
			return types.EngineeringAssignmentCommandResult{}, edgeErr
		}
		edges = append(edges, edge)
	}
	conditions := append([]objectgraph.ObjectCondition(nil), inboxConditions...)
	if !slices.ContainsFunc(conditions, func(c objectgraph.ObjectCondition) bool { return c.CanonicalID == parentHead.CanonicalID }) {
		// The parent round must still be open when the batch lands.
		conditions = append(conditions, engineeringObjectCondition(parentHead))
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: assignmentObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: agentObj.CanonicalID})
	if rebaseClaim != nil {
		inboxObjects = append(inboxObjects, *rebaseClaim)
		conditions = append(conditions, rebaseConditions...)
	}
	result, err := s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleOpenEngineeringAssignment, types.LifecycleEngineeringAssignmentOpened,
		req.CommandID, req.CommandDigest, assignment, nil, nil, "", "", []objectgraph.Object{assignmentObj, agentObj}, conditions, edges, nil, inboxObjects...)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if result.Replay {
		inbox, found, inboxErr := s.replayInboxCommand(ctx, records.OwnerID, records.ComputerID, records.CommandID, records.CommandDigest)
		if inboxErr != nil {
			return types.EngineeringAssignmentCommandResult{}, inboxErr
		}
		if found {
			result.Records = &inbox
		}
		return result, nil
	}
	result.Records = &inboxReceipt
	return result, nil
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

// EngineeringParentProfileForBinding is the requested_by_profile an assigned
// run must carry: the profile prefix of the binding's parent agent.
func EngineeringParentProfileForBinding(binding types.EngineeringAssignmentBinding) string {
	return engineeringParentProfileForBinding(binding)
}

// ValidateEngineeringAssignmentRun is the exact bind check a run must pass to
// bind an assignment; the runtime's run construction is tested against it.
func ValidateEngineeringAssignmentRun(assignment types.EngineeringAssignment, assignedAgent types.AgentRecord, run types.RunRecord) error {
	return validateEngineeringAssignmentRun(assignment, assignedAgent, run)
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
	if !assignment.Binding.Records() {
		return fmt.Errorf("co-super assignment: a pre-records binding binds no new run: %w", ErrEngineeringAssignmentInvalid)
	}
	// C1e: a records run carries no work items; it names its conversation
	// and the parent round it serves instead.
	workIDs, bindingErr := lifecycleActivationWorkItemIDs(run.Metadata)
	coordinationID := metadataExactString(run.Metadata, "coordination_contract_id")
	coordinationDigest := metadataExactString(run.Metadata, "coordination_contract_digest")
	if bindingErr != nil || len(workIDs) != 0 ||
		metadataExactString(run.Metadata, "lifecycle_work_item_id") != "" ||
		metadataExactString(run.Metadata, "assigned_conversation_id") != assignment.Binding.AssignedConversationID ||
		metadataExactString(run.Metadata, "parent_conversation_id") != assignment.Binding.ParentConversationID ||
		metadataExactUint64(run.Metadata, "parent_round") != uint64(assignment.Binding.ParentRound) ||
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
		[]objectgraph.Object{updatedObj, assignedAgentObj, runObj, runClaimObj, capabilityObj, capsuleObj}, conditions, []objectgraph.Edge{runEdge}, nil)
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
	objects := []objectgraph.Object{}
	conditions := []objectgraph.ObjectCondition{}
	if assignment.Binding.Records() {
		// C1e: a records assignment has no work item; the row is the
		// outcome authority, and only the agent and the run project.
		if agent.ActiveRunID != "" && agent.ActiveRunID != assignment.BoundRunID {
			return nil, nil, ErrEngineeringAssignmentInvalid
		}
		if agent.ActiveRunID != "" {
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
		return s.projectEngineeringTerminalRun(ctx, assignment, seq, now, resultRef, reason, objects, conditions)
	}
	workObj, work, err := s.lifecycleWorkObject(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.AssignedWorkItemID)
	if err != nil {
		return nil, nil, err
	}
	if work.AssignedAgentID != assignment.Binding.AssignedAgentID {
		return nil, nil, ErrEngineeringAssignmentInvalid
	}
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
	return s.projectEngineeringTerminalRun(ctx, assignment, seq, now, resultRef, reason, objects, conditions)
}

// projectEngineeringTerminalRun projects the assignment's terminal fate onto
// its bound run, after the agent (and, before C1e, the work item).
func (s *Store) projectEngineeringTerminalRun(ctx context.Context, assignment types.EngineeringAssignment, seq int64, now time.Time, resultRef, reason string, objects []objectgraph.Object, conditions []objectgraph.ObjectCondition) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
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
				// Legacy reports predate RecordCommandID and the engineering- spelling.
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
		if errors.Is(err, ErrLifecycleCommandConflict) {
			if same, ok, sameErr := s.replaySameTerminalReportCommand(ctx, req); sameErr != nil {
				return types.EngineeringAssignmentCommandResult{}, sameErr
			} else if ok {
				return same, nil
			}
		}
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
	var inboxObjects []objectgraph.Object
	if !report.Late && assignment.Binding.Records() {
		// C1e D3: engineering's Report to management commits with the row.
		// It is final only when the row turns terminal in this batch.
		final := assignment.Disposition != previousDisposition && assignment.Disposition.Terminal()
		recordObjects, recordConditions, recordErr := s.planEngineeringReportRecord(ctx, assignment, report, final, req.OutcomeRefs, now)
		if recordErr != nil {
			return types.EngineeringAssignmentCommandResult{}, recordErr
		}
		inboxObjects = recordObjects
		conditions = append(conditions, recordConditions...)
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
		objects, conditions, edges, report.EvidenceRefs, inboxObjects...)
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
	projectionObjects, projectionConditions, err := s.projectEngineeringTerminal(ctx, assignment, transition.seq, now, report.ReportID, req.Reason)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	objects := []objectgraph.Object{updatedObj, reportObj}
	objects = append(objects, projectionObjects...)
	conditions := append(engineeringParentAuthorityConditions(parentAuthority), engineeringObjectCondition(assignmentObj),
		objectgraph.ObjectCondition{CanonicalID: reportObj.CanonicalID})
	conditions = append(conditions, projectionConditions...)
	var inboxObjects []objectgraph.Object
	if assignment.Binding.Records() && !req.Silent {
		// C1e D3/D7: the cancellation's final Report closes the engineering
		// round and wakes management, atomic with the row, but only while
		// the round is open; a crash boot's cancellation is silent.
		recordObjects, recordConditions, recordErr := s.planEngineeringReportRecord(ctx, assignment, report, true, nil, now)
		if recordErr != nil {
			return types.EngineeringAssignmentCommandResult{}, recordErr
		}
		inboxObjects = recordObjects
		conditions = append(conditions, recordConditions...)
	}
	return s.commitEngineeringLifecycleCommand(ctx, transition, types.LifecycleCancelEngineeringAssignment, types.LifecycleEngineeringAssignmentCancelled,
		req.CommandID, req.CommandDigest, assignment, &report, nil, report.ReportID, req.Reason, objects,
		conditions, nil, nil, inboxObjects...)
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
		[]objectgraph.ObjectCondition{engineeringObjectCondition(assignmentObj)}, nil, evidenceRefs)
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

// GetEngineeringAssignmentReportByCanonicalID resolves a stored report from
// its ReportRefs canonical object ID (the refs a terminal write appends are
// canonical IDs, not ReportIDs). Used by the restart-recast supersede tuple.
func (s *Store) GetEngineeringAssignmentReportByCanonicalID(ctx context.Context, canonicalID string) (types.EngineeringAssignmentReport, error) {
	obj, err := s.lifecycleGraph().GetObject(ctx, strings.TrimSpace(canonicalID))
	if err != nil {
		return types.EngineeringAssignmentReport{}, err
	}
	if obj.ObjectKind != ogKindEngineeringReport {
		return types.EngineeringAssignmentReport{}, ErrNotFound
	}
	report, err := decodeLifecycleObject[types.EngineeringAssignmentReport](obj)
	if err != nil {
		return types.EngineeringAssignmentReport{}, err
	}
	if report.ReportID == "" {
		return types.EngineeringAssignmentReport{}, ErrNotFound
	}
	return report, nil
}

// ListEngineeringSubjectCandidates returns the subject candidates the
// trajectory's assignments minted, newest first. A candidate is the durable
// record of a round's changed source tree (its artifact ref names the
// content-addressed tree), whatever result the round reported. A retry reads
// them to carry the newest unfinished work forward as an inert patch (failure
// 15, docs/problems/engineering-completion-path-cluster-2026-10-20.md).
func (s *Store) ListEngineeringSubjectCandidates(ctx context.Context, ownerID, computerID, trajectoryID string) ([]types.EngineeringSubjectCandidate, error) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return nil, nil
	}
	objects, err := s.ogListAllByMetadata(ctx, ogKindEngineeringCandidate, "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	var out []types.EngineeringSubjectCandidate
	for _, obj := range objects {
		if obj.OwnerID != strings.TrimSpace(ownerID) || obj.ComputerID != strings.TrimSpace(computerID) || obj.Tombstone {
			continue
		}
		candidate, decodeErr := decodeLifecycleObject[types.EngineeringSubjectCandidate](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if candidate.TrajectoryID == trajectoryID && candidate.OwnerID == strings.TrimSpace(ownerID) && candidate.ComputerID == strings.TrimSpace(computerID) {
			out = append(out, candidate)
		}
	}
	slices.SortFunc(out, func(left, right types.EngineeringSubjectCandidate) int {
		if cmp := right.CreatedAt.Compare(left.CreatedAt); cmp != 0 {
			return cmp
		}
		return strings.Compare(right.CandidateID, left.CandidateID)
	})
	return out, nil
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
		commandID := fmt.Sprintf("engineering-orphan:%s:%d:%s", obs.AssignmentID, obs.Attempt, obs.RunID)
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
		CommandID:                fmt.Sprintf("engineering-orphan:%s:%d:%s", obs.AssignmentID, obs.Attempt, obs.RunID),
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

// planEngineeringReportRecord composes engineering's Report to management
// into an assignment transition's batch (C1e D3). It is sent by
// engineering:<assignment> to management:<owner>; final (stop reason = the
// disposition) only when the row turns terminal in the batch, a progress
// Report otherwise. Nothing is minted when the engineering round is not
// open: a crash boot closed it, and a late outcome mints no record. Its
// evidence refs name the assignment, the assignment report and the
// candidate the run froze (D2), which management's Verify names.
func (s *Store) planEngineeringReportRecord(ctx context.Context, assignment types.EngineeringAssignment, report types.EngineeringAssignmentReport, final bool, outcomeRefs []string, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	binding := assignment.Binding
	head, err := loadInboxCached[types.ConversationHead](ctx, s, ogKindConversationHead, binding.OwnerID, binding.ComputerID, binding.AssignedConversationID)
	if err != nil {
		return nil, nil, err
	}
	if !head.exists || !head.value.Open || head.value.RecipientInbox != binding.AssignedAgentID {
		return nil, nil, nil
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	body := &types.CommitmentReport{Claims: []types.ReportClaim{{Text: report.Summary}}}
	if final {
		body.StopReason = string(assignment.Disposition)
	}
	refs := []string{"assignment:" + assignment.AssignmentID, "assignment-report:" + report.ReportID}
	for _, ref := range outcomeRefs {
		if ref = strings.TrimSpace(ref); ref != "" && !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: EngineeringOutcomeRecordID(report.ReportID),
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{Hypothesis: report.Summary, CommittedAt: stamp},
		Provenance: types.CommitmentProvenance{AgentID: binding.AssignedAgentID, CommittedAt: stamp},
		Inbox:      binding.ParentAgentID, SenderInbox: binding.AssignedAgentID, Addressee: binding.ParentAgentID,
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: binding.ComputerID,
		Report: body, Final: final, EvidenceRefs: refs,
	}
	objects, conditions, _, err := s.planInboxRecords(ctx, binding.OwnerID, binding.ComputerID, []types.CommitmentRecord{rec}, nil, now)
	return objects, conditions, err
}
