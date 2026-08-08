package types

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	CoSuperAssignmentSchemaV1                              = "choir.co_super_assignment/v1"
	CoSuperCapsuleNetworkForbidden                         = "forbidden"
	CoSuperCapsuleNetworkNone                              = "none"
	CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay = "assignment_local_writable_overlay"
)

type CoSuperAssignmentKind string

const (
	CoSuperAssignmentImplementation CoSuperAssignmentKind = "implementation"
	CoSuperAssignmentVerification   CoSuperAssignmentKind = "verification"
)

type CoSuperAssignmentDisposition string

const (
	CoSuperAssignmentOpen      CoSuperAssignmentDisposition = "open"
	CoSuperAssignmentBound     CoSuperAssignmentDisposition = "bound"
	CoSuperAssignmentCompleted CoSuperAssignmentDisposition = "completed"
	CoSuperAssignmentFailed    CoSuperAssignmentDisposition = "failed"
	CoSuperAssignmentCancelled CoSuperAssignmentDisposition = "cancelled"
)

func (d CoSuperAssignmentDisposition) Terminal() bool {
	return d == CoSuperAssignmentCompleted || d == CoSuperAssignmentFailed || d == CoSuperAssignmentCancelled
}

type CoSuperCapsuleDisposition string

const (
	CoSuperCapsuleUnbound         CoSuperCapsuleDisposition = "unbound"
	CoSuperCapsuleActive          CoSuperCapsuleDisposition = "active"
	CoSuperCapsuleFreezeRequested CoSuperCapsuleDisposition = "freeze_requested"
	CoSuperCapsuleFrozen          CoSuperCapsuleDisposition = "frozen"
	CoSuperCapsuleRevokeRequested CoSuperCapsuleDisposition = "revoke_requested"
	CoSuperCapsuleRevoked         CoSuperCapsuleDisposition = "revoked"
)

type CoSuperAssignmentBinding struct {
	OwnerID                    string                `json:"owner_id"`
	ComputerID                 string                `json:"computer_id"`
	TrajectoryID               string                `json:"trajectory_id"`
	ParentAgentID              string                `json:"parent_agent_id"`
	ParentRunID                string                `json:"parent_loop_id"`
	ParentDecisionID           string                `json:"parent_decision_id"`
	ParentControlID            string                `json:"parent_control_id"`
	ParentWorkItemID           string                `json:"parent_work_item_id"`
	AssignedWorkItemID         string                `json:"assigned_work_item_id"`
	AssignedAgentID            string                `json:"assigned_agent_id"`
	Kind                       CoSuperAssignmentKind `json:"assignment_kind"`
	Attempt                    uint64                `json:"attempt"`
	ScopeDigest                string                `json:"scope_digest"`
	CapabilityDigest           string                `json:"capability_digest"`
	SubjectDigest              string                `json:"subject_digest"`
	Writable                   bool                  `json:"writable"`
	CapsuleID                  string                `json:"capsule_id,omitempty"`
	NetworkMode                string                `json:"network_mode"`
	FilesystemMode             string                `json:"filesystem_mode"`
	CoordinationContractID     string                `json:"coordination_contract_id,omitempty"`
	CoordinationContractDigest string                `json:"coordination_contract_digest,omitempty"`
}

func (b CoSuperAssignmentBinding) Validate() error {
	for name, value := range map[string]string{
		"owner_id": b.OwnerID, "computer_id": b.ComputerID, "trajectory_id": b.TrajectoryID,
		"parent_agent_id": b.ParentAgentID, "parent_loop_id": b.ParentRunID,
		"parent_decision_id": b.ParentDecisionID, "parent_control_id": b.ParentControlID,
		"parent_work_item_id": b.ParentWorkItemID, "assigned_work_item_id": b.AssignedWorkItemID,
		"assigned_agent_id": b.AssignedAgentID,
	} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("co-super assignment: %s is required and must be canonical", name)
		}
	}
	if b.ParentAgentID != "super:"+b.OwnerID {
		return fmt.Errorf("co-super assignment: parent_agent_id must be exact persistent super:<owner>")
	}
	if b.AssignedAgentID == b.ParentAgentID || b.AssignedWorkItemID == b.ParentWorkItemID {
		return fmt.Errorf("co-super assignment: parent and assigned identities must be distinct")
	}
	if b.Kind != CoSuperAssignmentImplementation && b.Kind != CoSuperAssignmentVerification {
		return fmt.Errorf("co-super assignment: assignment_kind must be implementation or verification")
	}
	if b.Attempt == 0 {
		return fmt.Errorf("co-super assignment: attempt must be positive")
	}
	for name, digest := range map[string]string{
		"scope_digest": b.ScopeDigest, "capability_digest": b.CapabilityDigest, "subject_digest": b.SubjectDigest,
	} {
		if !ValidSHA256Digest(digest) {
			return fmt.Errorf("co-super assignment: %s must be an exact sha256 digest", name)
		}
	}
	if !b.Writable || strings.TrimSpace(b.CapsuleID) == "" {
		return fmt.Errorf("co-super assignment: implementation and verification require a writable isolated capsule_id")
	}
	if b.CapsuleID != strings.TrimSpace(b.CapsuleID) {
		return fmt.Errorf("co-super assignment: capsule_id must be canonical")
	}
	if b.NetworkMode != CoSuperCapsuleNetworkForbidden && b.NetworkMode != CoSuperCapsuleNetworkNone {
		return fmt.Errorf("co-super assignment: capsule network_mode must be forbidden or none")
	}
	if b.FilesystemMode != CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay {
		return fmt.Errorf("co-super assignment: capsule filesystem_mode must be assignment-local writable overlay")
	}
	coordinationID := strings.TrimSpace(b.CoordinationContractID)
	coordinationDigest := strings.TrimSpace(b.CoordinationContractDigest)
	if (coordinationID == "") != (coordinationDigest == "") || b.CoordinationContractID != coordinationID ||
		b.CoordinationContractDigest != coordinationDigest || (coordinationDigest != "" && !ValidSHA256Digest(coordinationDigest)) {
		return fmt.Errorf("co-super assignment: coordination contract id and digest must be supplied together")
	}
	return nil
}

func ValidSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

type CoSuperAssignment struct {
	Schema             string                       `json:"schema"`
	AssignmentID       string                       `json:"assignment_id"`
	Binding            CoSuperAssignmentBinding     `json:"binding"`
	Disposition        CoSuperAssignmentDisposition `json:"disposition"`
	DispositionReason  string                       `json:"disposition_reason,omitempty"`
	CapsuleDisposition CoSuperCapsuleDisposition    `json:"capsule_disposition"`
	CapsuleIntentRef   string                       `json:"capsule_intent_ref,omitempty"`
	CapsuleAckRef      string                       `json:"capsule_ack_ref,omitempty"`
	BoundRunID         string                       `json:"bound_loop_id,omitempty"`
	ReportRefs         []string                     `json:"report_refs,omitempty"`
	LifecycleVersion   int64                        `json:"lifecycle_version"`
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
	TerminalAt         *time.Time                   `json:"terminal_at,omitempty"`
}

func (a CoSuperAssignment) Validate() error {
	if a.Schema != CoSuperAssignmentSchemaV1 || strings.TrimSpace(a.AssignmentID) == "" || a.AssignmentID != strings.TrimSpace(a.AssignmentID) {
		return fmt.Errorf("co-super assignment: schema and canonical assignment_id are required")
	}
	if err := a.Binding.Validate(); err != nil {
		return err
	}
	if a.LifecycleVersion <= 0 {
		return fmt.Errorf("co-super assignment: lifecycle_version must be positive")
	}
	switch a.Disposition {
	case CoSuperAssignmentOpen:
		if a.BoundRunID != "" || a.TerminalAt != nil || a.CapsuleDisposition != CoSuperCapsuleUnbound {
			return fmt.Errorf("co-super assignment: open assignment cannot be bound, terminal, or capsule-active")
		}
	case CoSuperAssignmentBound:
		if strings.TrimSpace(a.BoundRunID) == "" || a.TerminalAt != nil || a.CapsuleDisposition == CoSuperCapsuleUnbound {
			return fmt.Errorf("co-super assignment: bound assignment requires run and capsule binding and cannot be terminal")
		}
	case CoSuperAssignmentCompleted, CoSuperAssignmentFailed, CoSuperAssignmentCancelled:
		if a.TerminalAt == nil {
			return fmt.Errorf("co-super assignment: terminal outcome requires terminal_at")
		}
	default:
		return fmt.Errorf("co-super assignment: invalid disposition %q", a.Disposition)
	}
	switch a.CapsuleDisposition {
	case CoSuperCapsuleUnbound, CoSuperCapsuleActive:
		if a.CapsuleIntentRef != "" || a.CapsuleAckRef != "" {
			return fmt.Errorf("co-super assignment: unbound/active capsule cannot carry fate refs")
		}
	case CoSuperCapsuleFreezeRequested, CoSuperCapsuleRevokeRequested:
		if strings.TrimSpace(a.CapsuleIntentRef) == "" || a.CapsuleAckRef != "" {
			return fmt.Errorf("co-super assignment: requested capsule fate requires intent_ref and no ack_ref")
		}
	case CoSuperCapsuleFrozen, CoSuperCapsuleRevoked:
		if strings.TrimSpace(a.CapsuleIntentRef) == "" || strings.TrimSpace(a.CapsuleAckRef) == "" {
			return fmt.Errorf("co-super assignment: acknowledged capsule fate requires intent_ref and ack_ref")
		}
	default:
		return fmt.Errorf("co-super assignment: invalid capsule disposition %q", a.CapsuleDisposition)
	}
	return nil
}

type CoSuperAssignmentResultKind string

const (
	CoSuperResultCompleted CoSuperAssignmentResultKind = "completed"
	CoSuperResultFailed    CoSuperAssignmentResultKind = "failed"
	CoSuperResultBlocked   CoSuperAssignmentResultKind = "blocked"
	CoSuperResultPartial   CoSuperAssignmentResultKind = "partial"
)

type CoSuperAssignmentVerdict string

const (
	CoSuperVerdictNone    CoSuperAssignmentVerdict = "none"
	CoSuperVerdictPass    CoSuperAssignmentVerdict = "pass"
	CoSuperVerdictFail    CoSuperAssignmentVerdict = "fail"
	CoSuperVerdictAbstain CoSuperAssignmentVerdict = "abstain"
)

type CoSuperRecordedCommand struct {
	CommandID     string `json:"command_id"`
	CommandDigest string `json:"command_digest"`
	ExecutionRef  string `json:"execution_ref"`
	ExitCode      int    `json:"exit_code"`
}

type CoSuperRecordedOutput struct {
	OutputID string `json:"output_id"`
	Kind     string `json:"kind"`
	Digest   string `json:"digest"`
	Ref      string `json:"ref"`
}

type CoSuperRecordedMutation struct {
	MutationID          string `json:"mutation_id"`
	Kind                string `json:"kind"`
	BeforeDigest        string `json:"before_digest"`
	AfterDigest         string `json:"after_digest"`
	EvidenceRef         string `json:"evidence_ref"`
	SubjectBytesChanged bool   `json:"subject_bytes_changed,omitempty"`
}

type CoSuperAssignmentReport struct {
	Schema                   string                      `json:"schema"`
	ReportID                 string                      `json:"report_id"`
	AssignmentID             string                      `json:"assignment_id"`
	Attempt                  uint64                      `json:"attempt"`
	OwnerID                  string                      `json:"owner_id"`
	ComputerID               string                      `json:"computer_id"`
	TrajectoryID             string                      `json:"trajectory_id"`
	RunID                    string                      `json:"loop_id"`
	AssignedAgentID          string                      `json:"assigned_agent_id"`
	Result                   CoSuperAssignmentResultKind `json:"result"`
	Verdict                  CoSuperAssignmentVerdict    `json:"verdict"`
	ObservedSubjectDigest    string                      `json:"observed_subject_digest"`
	Commands                 []CoSuperRecordedCommand    `json:"commands,omitempty"`
	Outputs                  []CoSuperRecordedOutput     `json:"outputs,omitempty"`
	Mutations                []CoSuperRecordedMutation   `json:"mutations,omitempty"`
	Late                     bool                        `json:"late"`
	CertifiesOriginalSubject bool                        `json:"certifies_original_subject"`
	CandidateSubjectDigest   string                      `json:"candidate_subject_digest,omitempty"`
	CandidateID              string                      `json:"candidate_id,omitempty"`
	CreatedAt                time.Time                   `json:"created_at"`
}

func (r CoSuperAssignmentReport) ValidateAgainst(a CoSuperAssignment) error {
	if r.Schema != CoSuperAssignmentSchemaV1 || strings.TrimSpace(r.ReportID) == "" || r.ReportID != strings.TrimSpace(r.ReportID) {
		return fmt.Errorf("co-super assignment report: schema and canonical report_id are required")
	}
	if r.AssignmentID != a.AssignmentID || r.Attempt != a.Binding.Attempt || r.OwnerID != a.Binding.OwnerID ||
		r.ComputerID != a.Binding.ComputerID || r.TrajectoryID != a.Binding.TrajectoryID ||
		r.RunID != a.BoundRunID || r.AssignedAgentID != a.Binding.AssignedAgentID {
		return fmt.Errorf("co-super assignment report: exact assignment/run scope binding is required")
	}
	switch r.Result {
	case CoSuperResultCompleted, CoSuperResultFailed, CoSuperResultBlocked, CoSuperResultPartial:
	default:
		return fmt.Errorf("co-super assignment report: invalid result %q", r.Result)
	}
	if !ValidSHA256Digest(r.ObservedSubjectDigest) {
		return fmt.Errorf("co-super assignment report: observed_subject_digest is required")
	}
	if a.Binding.Kind == CoSuperAssignmentImplementation {
		if r.Verdict != CoSuperVerdictNone {
			return fmt.Errorf("co-super assignment report: implementation cannot issue verification verdict")
		}
	} else {
		if r.Verdict != CoSuperVerdictPass && r.Verdict != CoSuperVerdictFail && r.Verdict != CoSuperVerdictAbstain {
			return fmt.Errorf("co-super assignment report: verification requires typed verdict")
		}
		if r.Verdict == CoSuperVerdictPass && r.Result != CoSuperResultCompleted {
			return fmt.Errorf("co-super assignment report: pass requires completed result")
		}
	}
	seen := map[string]struct{}{}
	for _, command := range r.Commands {
		if strings.TrimSpace(command.CommandID) == "" || !ValidSHA256Digest(command.CommandDigest) || strings.TrimSpace(command.ExecutionRef) == "" {
			return fmt.Errorf("co-super assignment report: commands require id, digest, and execution_ref")
		}
		if _, duplicate := seen["command:"+command.CommandID]; duplicate {
			return fmt.Errorf("co-super assignment report: duplicate command_id")
		}
		seen["command:"+command.CommandID] = struct{}{}
	}
	for _, output := range r.Outputs {
		if strings.TrimSpace(output.OutputID) == "" || strings.TrimSpace(output.Kind) == "" || !ValidSHA256Digest(output.Digest) || strings.TrimSpace(output.Ref) == "" {
			return fmt.Errorf("co-super assignment report: outputs require id, kind, digest, and ref")
		}
		if _, duplicate := seen["output:"+output.OutputID]; duplicate {
			return fmt.Errorf("co-super assignment report: duplicate output_id")
		}
		seen["output:"+output.OutputID] = struct{}{}
	}
	changed := r.ObservedSubjectDigest != a.Binding.SubjectDigest
	for _, mutation := range r.Mutations {
		if strings.TrimSpace(mutation.MutationID) == "" || strings.TrimSpace(mutation.Kind) == "" ||
			!ValidSHA256Digest(mutation.BeforeDigest) || !ValidSHA256Digest(mutation.AfterDigest) || strings.TrimSpace(mutation.EvidenceRef) == "" {
			return fmt.Errorf("co-super assignment report: mutations require id, kind, before/after digests, and evidence_ref")
		}
		if _, duplicate := seen["mutation:"+mutation.MutationID]; duplicate {
			return fmt.Errorf("co-super assignment report: duplicate mutation_id")
		}
		seen["mutation:"+mutation.MutationID] = struct{}{}
		if mutation.SubjectBytesChanged {
			if mutation.BeforeDigest != a.Binding.SubjectDigest || mutation.AfterDigest == a.Binding.SubjectDigest || mutation.AfterDigest != r.ObservedSubjectDigest {
				return fmt.Errorf("co-super assignment report: subject-byte mutation must bind original and new subject digests")
			}
			changed = true
		}
	}
	if changed {
		if r.CandidateSubjectDigest != r.ObservedSubjectDigest || strings.TrimSpace(r.CandidateID) == "" || r.CertifiesOriginalSubject {
			return fmt.Errorf("co-super assignment report: changed subject requires a distinct non-certifying candidate identity")
		}
	} else if r.CandidateSubjectDigest != "" || r.CandidateID != "" {
		return fmt.Errorf("co-super assignment report: unchanged subject cannot create candidate identity")
	}
	if r.CertifiesOriginalSubject && (a.Binding.Kind != CoSuperAssignmentVerification || r.Verdict != CoSuperVerdictPass || r.Late || changed) {
		return fmt.Errorf("co-super assignment report: original subject certification requires timely immutable verification pass")
	}
	return nil
}

type CoSuperSubjectCandidate struct {
	Schema                string    `json:"schema"`
	CandidateID           string    `json:"candidate_id"`
	OwnerID               string    `json:"owner_id"`
	ComputerID            string    `json:"computer_id"`
	TrajectoryID          string    `json:"trajectory_id"`
	AssignmentID          string    `json:"assignment_id"`
	Attempt               uint64    `json:"attempt"`
	OriginalSubjectDigest string    `json:"original_subject_digest"`
	SubjectDigest         string    `json:"subject_digest"`
	SourceReportID        string    `json:"source_report_id"`
	CreatedAt             time.Time `json:"created_at"`
}

type CoSuperAssignmentCommandResult struct {
	Receipt    LifecycleCommandReceipt  `json:"receipt"`
	Assignment CoSuperAssignment        `json:"assignment"`
	Report     *CoSuperAssignmentReport `json:"report,omitempty"`
	Candidate  *CoSuperSubjectCandidate `json:"candidate,omitempty"`
	Replay     bool                     `json:"replay"`
}

type OpenCoSuperAssignmentRequest struct {
	CommandID     string                   `json:"command_id"`
	CommandDigest string                   `json:"command_digest"`
	AssignmentID  string                   `json:"assignment_id"`
	Binding       CoSuperAssignmentBinding `json:"binding"`
}

type BindCoSuperAssignmentRequest struct {
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	AssignmentID             string `json:"assignment_id"`
	Attempt                  uint64 `json:"attempt"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	RunID                    string `json:"loop_id"`
	OpaqueCapability         string `json:"-"`
	CapsuleID                string `json:"capsule_id,omitempty"`
}

type RecordCoSuperAssignmentReportRequest struct {
	CommandID                string                  `json:"command_id"`
	CommandDigest            string                  `json:"command_digest"`
	OwnerID                  string                  `json:"owner_id"`
	ComputerID               string                  `json:"computer_id"`
	AssignmentID             string                  `json:"assignment_id"`
	Attempt                  uint64                  `json:"attempt"`
	ExpectedLifecycleVersion int64                   `json:"expected_lifecycle_version"`
	Report                   CoSuperAssignmentReport `json:"report"`
}

type CancelCoSuperAssignmentRequest struct {
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	AssignmentID             string `json:"assignment_id"`
	Attempt                  uint64 `json:"attempt"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	Reason                   string `json:"reason"`
}

type SetCoSuperCapsuleDispositionRequest struct {
	CommandID                string                    `json:"command_id"`
	CommandDigest            string                    `json:"command_digest"`
	OwnerID                  string                    `json:"owner_id"`
	ComputerID               string                    `json:"computer_id"`
	AssignmentID             string                    `json:"assignment_id"`
	Attempt                  uint64                    `json:"attempt"`
	ExpectedLifecycleVersion int64                     `json:"expected_lifecycle_version"`
	Disposition              CoSuperCapsuleDisposition `json:"capsule_disposition"`
	IntentRef                string                    `json:"intent_ref"`
	AckRef                   string                    `json:"ack_ref,omitempty"`
}
