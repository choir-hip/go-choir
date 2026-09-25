package types

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

const (
	EngineeringAssignmentSchemaV1                              = "choir.co_super_assignment/v1"
	EngineeringCapsuleNetworkForbidden                         = "forbidden"
	EngineeringCapsuleNetworkNone                              = "none"
	EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay = "assignment_local_writable_overlay"
	EngineeringGrantPolicyAttestationSchemaV1                  = "choir.co_super_grant_policy_attestation/v1"
	EngineeringExecutionAttestationSchemaV1                    = "choir.co_super_execution_attestation/v1"
	EngineeringCapsuleFateStepSchemaV1                         = "choir.co_super_capsule_fate_step/v1"
)

type EngineeringAssignmentKind string

// EngineeringCastAuthority names which admission authority admitted the
// assignment: an owner-authored document revision (owner cast, the M1 path)
// or a desk-staged choir.Cast whose authority is the caster's own live
// assignment (delegated cast, R2). Empty means owner cast for backward
// compatibility with bindings minted before the field existed.
type EngineeringCastAuthority string

const (
	EngineeringCastAuthorityOwner     EngineeringCastAuthority = "owner"
	EngineeringCastAuthorityDelegated EngineeringCastAuthority = "delegated"
)

const (
	EngineeringAssignmentImplementation EngineeringAssignmentKind = "implementation"
	EngineeringAssignmentVerification   EngineeringAssignmentKind = "verification"
)

type EngineeringAssignmentDisposition string

const (
	EngineeringAssignmentOpen      EngineeringAssignmentDisposition = "open"
	EngineeringAssignmentBound     EngineeringAssignmentDisposition = "bound"
	EngineeringAssignmentCompleted EngineeringAssignmentDisposition = "completed"
	EngineeringAssignmentFailed    EngineeringAssignmentDisposition = "failed"
	EngineeringAssignmentCancelled EngineeringAssignmentDisposition = "cancelled"
)

func (d EngineeringAssignmentDisposition) Terminal() bool {
	return d == EngineeringAssignmentCompleted || d == EngineeringAssignmentFailed || d == EngineeringAssignmentCancelled
}

type EngineeringCapsuleDisposition string

const (
	EngineeringCapsuleUnbound         EngineeringCapsuleDisposition = "unbound"
	EngineeringCapsuleActive          EngineeringCapsuleDisposition = "active"
	EngineeringCapsuleFreezeRequested EngineeringCapsuleDisposition = "freeze_requested"
	EngineeringCapsuleFrozen          EngineeringCapsuleDisposition = "frozen"
	EngineeringCapsuleRevokeRequested EngineeringCapsuleDisposition = "revoke_requested"
	EngineeringCapsuleRevoked         EngineeringCapsuleDisposition = "revoked"
)

type EngineeringAssignmentBinding struct {
	OwnerID                    string                    `json:"owner_id"`
	ComputerID                 string                    `json:"computer_id"`
	TrajectoryID               string                    `json:"trajectory_id"`
	ParentAgentID              string                    `json:"parent_agent_id"`
	ParentRunID                string                    `json:"parent_loop_id"`
	ParentDecisionID           string                    `json:"parent_decision_id"`
	ParentControlID            string                    `json:"parent_control_id"`
	ParentWorkItemID           string                    `json:"parent_work_item_id"`
	AssignedWorkItemID         string                    `json:"assigned_work_item_id"`
	AssignedAgentID            string                    `json:"assigned_agent_id"`
	Kind                       EngineeringAssignmentKind `json:"assignment_kind"`
	Attempt                    uint64                    `json:"attempt"`
	ScopeDigest                string                    `json:"scope_digest"`
	RequestDigest              string                    `json:"request_digest"`
	CapabilityDigest           string                    `json:"capability_digest"`
	ExecutionHandleDigest      string                    `json:"execution_handle_digest"`
	SubjectDigest              string                    `json:"subject_digest"`
	SourceArtifactRef          string                    `json:"source_artifact_ref"`
	SourceCandidateID          string                    `json:"source_candidate_id,omitempty"`
	Writable                   bool                      `json:"writable"`
	CapsuleID                  string                    `json:"capsule_id,omitempty"`
	NetworkMode                string                    `json:"network_mode"`
	FilesystemMode             string                    `json:"filesystem_mode"`
	CoordinationContractID     string                    `json:"coordination_contract_id,omitempty"`
	CoordinationContractDigest string                    `json:"coordination_contract_digest,omitempty"`
	// CastAuthority distinguishes owner-cast admission (a document revision
	// is the parent authority) from delegated-cast admission (the caster's
	// own live assignment/run is the parent authority). Empty or "owner" is
	// the owner-cast path; "delegated" is the R2 desk-authored cast.
	CastAuthority EngineeringCastAuthority `json:"cast_authority,omitempty"`
}

func (b EngineeringAssignmentBinding) Validate() error {
	for name, value := range map[string]string{
		"owner_id": b.OwnerID, "computer_id": b.ComputerID, "trajectory_id": b.TrajectoryID,
		"parent_agent_id":    b.ParentAgentID,
		"parent_decision_id": b.ParentDecisionID, "parent_control_id": b.ParentControlID,
		"parent_work_item_id": b.ParentWorkItemID, "assigned_work_item_id": b.AssignedWorkItemID,
		"assigned_agent_id": b.AssignedAgentID,
	} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("co-super assignment: %s is required and must be canonical", name)
		}
	}
	if !strings.HasPrefix(b.ParentDecisionID, "decision:sha256:") || !ValidSHA256Digest(strings.TrimPrefix(b.ParentDecisionID, "decision:")) {
		return fmt.Errorf("co-super assignment: parent_decision_id must be runtime-derived")
	}
	if b.ParentRunID == "" {
		// Document-parent binding: the owner-authored revision on the bound
		// document is the parent authority; there is no parent run. The parent
		// agent is the engineering desk agent bound to that document.
		if !strings.HasPrefix(b.ParentAgentID, agentprofile.Engineering+":") {
			return fmt.Errorf("co-super assignment: document-parent binding requires an engineering desk parent agent")
		}
	} else if b.ParentAgentID != agentprofile.Management+":"+b.OwnerID {
		return fmt.Errorf("co-super assignment: parent_agent_id must be exact persistent management:<owner>")
	}
	if b.AssignedAgentID == b.ParentAgentID || b.AssignedWorkItemID == b.ParentWorkItemID {
		return fmt.Errorf("co-super assignment: parent and assigned identities must be distinct")
	}
	if b.Kind != EngineeringAssignmentImplementation && b.Kind != EngineeringAssignmentVerification {
		return fmt.Errorf("co-super assignment: assignment_kind must be implementation or verification")
	}
	if b.Attempt == 0 {
		return fmt.Errorf("co-super assignment: attempt must be positive")
	}
	for name, digest := range map[string]string{
		"scope_digest": b.ScopeDigest, "request_digest": b.RequestDigest, "capability_digest": b.CapabilityDigest,
		"subject_digest": b.SubjectDigest,
	} {
		if !ValidSHA256Digest(digest) {
			return fmt.Errorf("co-super assignment: %s must be an exact sha256 digest", name)
		}
	}
	if b.ExecutionHandleDigest != "" && !ValidSHA256Digest(b.ExecutionHandleDigest) {
		return fmt.Errorf("co-super assignment: execution_handle_digest must be an exact sha256 digest when present")
	}
	if strings.TrimSpace(b.SourceArtifactRef) == "" || b.SourceArtifactRef != strings.TrimSpace(b.SourceArtifactRef) {
		return fmt.Errorf("co-super assignment: exact runtime source artifact ref is required")
	}
	if (b.Kind == EngineeringAssignmentVerification) != (strings.TrimSpace(b.SourceCandidateID) != "") || b.SourceCandidateID != strings.TrimSpace(b.SourceCandidateID) {
		return fmt.Errorf("co-super assignment: verification requires exact runtime-selected source candidate")
	}
	if b.SourceCandidateID != "" {
		if b.SourceArtifactRef != "capsule-subject:"+b.SubjectDigest {
			return fmt.Errorf("co-super assignment: candidate source artifact must bind subject digest")
		}
	} else if !strings.HasPrefix(b.SourceArtifactRef, "capsule-source-git:") || !strings.HasSuffix(b.SourceArtifactRef, ":"+b.SubjectDigest) {
		return fmt.Errorf("co-super assignment: implementation source must bind exact git provenance and subject digest")
	}
	if !b.Writable || strings.TrimSpace(b.CapsuleID) == "" {
		return fmt.Errorf("co-super assignment: implementation and verification require a writable isolated capsule_id")
	}
	if b.CapsuleID != strings.TrimSpace(b.CapsuleID) || b.CapsuleID == "." || b.CapsuleID == ".." || strings.ContainsAny(b.CapsuleID, `/\`) {
		return fmt.Errorf("co-super assignment: capsule_id must be a canonical single path component")
	}
	if b.NetworkMode != EngineeringCapsuleNetworkForbidden && b.NetworkMode != EngineeringCapsuleNetworkNone {
		return fmt.Errorf("co-super assignment: capsule network_mode must be forbidden or none")
	}
	if b.FilesystemMode != EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay {
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

// EngineeringGrantPolicyAttestation is runtime-authored evidence about the exact
// signed capability returned by the capsule executor. It deliberately retains
// digests and policy facts, never capability or handle bytes.
type EngineeringGrantPolicyAttestation struct {
	Schema                 string    `json:"schema"`
	AttestationRef         string    `json:"attestation_ref"`
	AssignmentID           string    `json:"assignment_id"`
	Attempt                uint64    `json:"attempt"`
	OwnerID                string    `json:"owner_id"`
	ComputerID             string    `json:"computer_id"`
	TrajectoryID           string    `json:"trajectory_id"`
	RunID                  string    `json:"loop_id"`
	CapsuleID              string    `json:"capsule_id"`
	TargetCapsule          string    `json:"target_capsule"`
	Role                   string    `json:"role"`
	GrantedVerbs           []string  `json:"granted_verbs"`
	VerbSetDigest          string    `json:"verb_set_digest"`
	PolicyDigest           string    `json:"policy_digest"`
	SignedCapabilityDigest string    `json:"signed_capability_digest"`
	NetworkMode            string    `json:"network_mode"`
	FilesystemMode         string    `json:"filesystem_mode"`
	Writable               bool      `json:"writable"`
	SpawnAcknowledged      bool      `json:"spawn_acknowledged"`
	ActiveAcknowledged     bool      `json:"active_acknowledged"`
	GrantAcknowledged      bool      `json:"grant_acknowledged"`
	SpawnedAt              time.Time `json:"spawned_at"`
	GrantedAt              time.Time `json:"granted_at"`
	BindCommandID          string    `json:"bind_command_id"`
	BindEventID            string    `json:"bind_event_id"`
	ReducerSeq             int64     `json:"reducer_seq"`
	RecordedAt             time.Time `json:"recorded_at"`
}

// EngineeringExecutionAttestation is the sanitized durable copyout constructed
// from one already-validated GrantedExecutionReceipt.
type EngineeringExecutionAttestation struct {
	Schema              string    `json:"schema"`
	AttestationRef      string    `json:"attestation_ref"`
	GrantedReceiptRef   string    `json:"granted_receipt_ref"`
	AssignmentID        string    `json:"assignment_id"`
	Attempt             uint64    `json:"attempt"`
	OwnerID             string    `json:"owner_id"`
	ComputerID          string    `json:"computer_id"`
	TrajectoryID        string    `json:"trajectory_id"`
	RunID               string    `json:"loop_id"`
	CapsuleID           string    `json:"capsule_id"`
	ReportID            string    `json:"report_id"`
	CommandID           string    `json:"command_id"`
	CommandDigest       string    `json:"command_digest"`
	ExitCode            int       `json:"exit_code"`
	StdoutDigest        string    `json:"stdout_digest"`
	StderrDigest        string    `json:"stderr_digest"`
	SourceSubjectDigest string    `json:"source_subject_digest"`
	FinalSubjectDigest  string    `json:"final_subject_digest"`
	WorktreeDigest      string    `json:"worktree_digest"`
	Granted             bool      `json:"granted"`
	Frozen              bool      `json:"frozen"`
	OccurredAt          time.Time `json:"occurred_at"`
	ReportCommandID     string    `json:"report_command_id"`
	ReportEventID       string    `json:"report_event_id"`
	ReducerSeq          int64     `json:"reducer_seq"`
	RecordedAt          time.Time `json:"recorded_at"`
}

// EngineeringCapsuleFateStep is one append-only runtime-authored capsule fate
// transition. Requested and acknowledged transitions remain separate entries.
type EngineeringCapsuleFateStep struct {
	Schema                     string                        `json:"schema"`
	StepRef                    string                        `json:"step_ref"`
	AssignmentID               string                        `json:"assignment_id"`
	Attempt                    uint64                        `json:"attempt"`
	OwnerID                    string                        `json:"owner_id"`
	ComputerID                 string                        `json:"computer_id"`
	TrajectoryID               string                        `json:"trajectory_id"`
	RunID                      string                        `json:"loop_id,omitempty"`
	CapsuleID                  string                        `json:"capsule_id"`
	Disposition                EngineeringCapsuleDisposition `json:"capsule_disposition"`
	CommandID                  string                        `json:"command_id"`
	EventID                    string                        `json:"event_id"`
	ReducerSeq                 int64                         `json:"reducer_seq"`
	IntentRef                  string                        `json:"intent_ref"`
	AckRef                     string                        `json:"ack_ref,omitempty"`
	SourceSubjectDigest        string                        `json:"source_subject_digest,omitempty"`
	FinalSubjectDigest         string                        `json:"final_subject_digest,omitempty"`
	AssignmentCapabilityDigest string                        `json:"assignment_capability_digest"`
	CapsuleAbsent              bool                          `json:"capsule_absent,omitempty"`
	OccurredAt                 time.Time                     `json:"occurred_at"`
	RecordedAt                 time.Time                     `json:"recorded_at"`
}

type EngineeringAssignment struct {
	Schema                 string                             `json:"schema"`
	AssignmentID           string                             `json:"assignment_id"`
	Binding                EngineeringAssignmentBinding       `json:"binding"`
	Disposition            EngineeringAssignmentDisposition   `json:"disposition"`
	DispositionReason      string                             `json:"disposition_reason,omitempty"`
	CapsuleDisposition     EngineeringCapsuleDisposition      `json:"capsule_disposition"`
	CapsuleIntentRef       string                             `json:"capsule_intent_ref,omitempty"`
	CapsuleAckRef          string                             `json:"capsule_ack_ref,omitempty"`
	BoundRunID             string                             `json:"bound_loop_id,omitempty"`
	ReportRefs             []string                           `json:"report_refs,omitempty"`
	GrantPolicyAttestation *EngineeringGrantPolicyAttestation `json:"grant_policy_attestation,omitempty"`
	CapsuleFateHistory     []EngineeringCapsuleFateStep       `json:"capsule_fate_history,omitempty"`
	LifecycleVersion       int64                              `json:"lifecycle_version"`
	CreatedAt              time.Time                          `json:"created_at"`
	UpdatedAt              time.Time                          `json:"updated_at"`
	TerminalAt             *time.Time                         `json:"terminal_at,omitempty"`
	PendingProposal        *EngineeringPendingProposal        `json:"pending_proposal,omitempty"`
}

// EngineeringPendingProposal holds the reducer-owned pending proposal (settlement gate item 5)
// committed durably before issuing physical freeze/revoke actions.
type EngineeringPendingProposal struct {
	PropositionDigest string                      `json:"proposition_digest"`
	Report            EngineeringAssignmentReport `json:"report"`
	FreezeIntentRef   string                      `json:"freeze_intent_ref"`
	RevokeIntentRef   string                      `json:"revoke_intent_ref,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
}

func (a EngineeringAssignment) Validate() error {
	if a.Schema != EngineeringAssignmentSchemaV1 || strings.TrimSpace(a.AssignmentID) == "" || a.AssignmentID != strings.TrimSpace(a.AssignmentID) {
		return fmt.Errorf("co-super assignment: schema and canonical assignment_id are required")
	}
	if err := a.Binding.Validate(); err != nil {
		return err
	}
	if a.LifecycleVersion <= 0 {
		return fmt.Errorf("co-super assignment: lifecycle_version must be positive")
	}
	switch a.Disposition {
	case EngineeringAssignmentOpen:
		if a.BoundRunID != "" || a.TerminalAt != nil ||
			(a.CapsuleDisposition != EngineeringCapsuleUnbound && a.CapsuleDisposition != EngineeringCapsuleRevokeRequested && a.CapsuleDisposition != EngineeringCapsuleRevoked) {
			return fmt.Errorf("co-super assignment: open assignment cannot be bound, terminal, or capsule-active")
		}
	case EngineeringAssignmentBound:
		if strings.TrimSpace(a.BoundRunID) == "" || a.TerminalAt != nil || a.CapsuleDisposition == EngineeringCapsuleUnbound {
			return fmt.Errorf("co-super assignment: bound assignment requires run and capsule binding and cannot be terminal")
		}
	case EngineeringAssignmentCompleted, EngineeringAssignmentFailed, EngineeringAssignmentCancelled:
		if a.TerminalAt == nil {
			return fmt.Errorf("co-super assignment: terminal outcome requires terminal_at")
		}
	default:
		return fmt.Errorf("co-super assignment: invalid disposition %q", a.Disposition)
	}
	switch a.CapsuleDisposition {
	case EngineeringCapsuleUnbound, EngineeringCapsuleActive:
		if a.CapsuleIntentRef != "" || a.CapsuleAckRef != "" {
			return fmt.Errorf("co-super assignment: unbound/active capsule cannot carry fate refs")
		}
	case EngineeringCapsuleFreezeRequested, EngineeringCapsuleRevokeRequested:
		if strings.TrimSpace(a.CapsuleIntentRef) == "" || a.CapsuleAckRef != "" {
			return fmt.Errorf("co-super assignment: requested capsule fate requires intent_ref and no ack_ref")
		}
	case EngineeringCapsuleFrozen, EngineeringCapsuleRevoked:
		if strings.TrimSpace(a.CapsuleIntentRef) == "" || strings.TrimSpace(a.CapsuleAckRef) == "" {
			return fmt.Errorf("co-super assignment: acknowledged capsule fate requires intent_ref and ack_ref")
		}
	default:
		return fmt.Errorf("co-super assignment: invalid capsule disposition %q", a.CapsuleDisposition)
	}
	return nil
}

type EngineeringAssignmentResultKind string

const (
	EngineeringResultCompleted EngineeringAssignmentResultKind = "completed"
	EngineeringResultFailed    EngineeringAssignmentResultKind = "failed"
	EngineeringResultBlocked   EngineeringAssignmentResultKind = "blocked"
	EngineeringResultPartial   EngineeringAssignmentResultKind = "partial"
)

type EngineeringAssignmentVerdict string

const (
	EngineeringVerdictNone    EngineeringAssignmentVerdict = "none"
	EngineeringVerdictPass    EngineeringAssignmentVerdict = "pass"
	EngineeringVerdictFail    EngineeringAssignmentVerdict = "fail"
	EngineeringVerdictAbstain EngineeringAssignmentVerdict = "abstain"
)

type EngineeringRecordedCommand struct {
	CommandID     string `json:"command_id"`
	CommandDigest string `json:"command_digest"`
	ExecutionRef  string `json:"execution_ref"`
	ExitCode      int    `json:"exit_code"`
}

type EngineeringRecordedOutput struct {
	OutputID string `json:"output_id"`
	Kind     string `json:"kind"`
	Digest   string `json:"digest"`
	Ref      string `json:"ref"`
}

type EngineeringRecordedMutation struct {
	MutationID          string `json:"mutation_id"`
	Kind                string `json:"kind"`
	BeforeDigest        string `json:"before_digest"`
	AfterDigest         string `json:"after_digest"`
	EvidenceRef         string `json:"evidence_ref"`
	SubjectBytesChanged bool   `json:"subject_bytes_changed,omitempty"`
}

type EngineeringAssignmentReport struct {
	Schema                   string                            `json:"schema"`
	ReportID                 string                            `json:"report_id"`
	AssignmentID             string                            `json:"assignment_id"`
	Attempt                  uint64                            `json:"attempt"`
	OwnerID                  string                            `json:"owner_id"`
	ComputerID               string                            `json:"computer_id"`
	TrajectoryID             string                            `json:"trajectory_id"`
	RunID                    string                            `json:"loop_id"`
	AssignedAgentID          string                            `json:"assigned_agent_id"`
	Result                   EngineeringAssignmentResultKind   `json:"result"`
	Verdict                  EngineeringAssignmentVerdict      `json:"verdict"`
	ObservedSubjectDigest    string                            `json:"observed_subject_digest"`
	Commands                 []EngineeringRecordedCommand      `json:"commands,omitempty"`
	Outputs                  []EngineeringRecordedOutput       `json:"outputs,omitempty"`
	Mutations                []EngineeringRecordedMutation     `json:"mutations,omitempty"`
	Late                     bool                              `json:"late"`
	CertifiesOriginalSubject bool                              `json:"certifies_original_subject"`
	CandidateSubjectDigest   string                            `json:"candidate_subject_digest,omitempty"`
	CandidateID              string                            `json:"candidate_id,omitempty"`
	CandidateArtifactRef     string                            `json:"candidate_artifact_ref,omitempty"`
	ExecutorReceiptRefs      []string                          `json:"executor_receipt_refs,omitempty"`
	ExecutionAttestations    []EngineeringExecutionAttestation `json:"execution_attestations,omitempty"`
	Summary                  string                            `json:"summary,omitempty"`
	EvidenceRefs             []string                          `json:"evidence_refs,omitempty"`
	CreatedAt                time.Time                         `json:"created_at"`
	// PropositionDigest is the reducer-derived v1 terminal-proposition digest
	// (settlement gate item 3): sha over the canonical submitted-proposition
	// content plus the pinned pre-execution subject belief. It is derivation,
	// never digest input; the store computes it authoritatively at record.
	PropositionDigest string `json:"proposition_digest,omitempty"`
	// RecordCommandID is the lifecycle command that recorded this report
	// (derivation, never identity input): same-digest replays resolve the
	// original receipt through it.
	RecordCommandID string `json:"record_command_id,omitempty"`
}

func (r EngineeringAssignmentReport) ValidateAgainst(a EngineeringAssignment) error {
	if r.Schema != EngineeringAssignmentSchemaV1 || strings.TrimSpace(r.ReportID) == "" || r.ReportID != strings.TrimSpace(r.ReportID) {
		return fmt.Errorf("co-super assignment report: schema and canonical report_id are required")
	}
	if r.AssignmentID != a.AssignmentID || r.Attempt != a.Binding.Attempt || r.OwnerID != a.Binding.OwnerID ||
		r.ComputerID != a.Binding.ComputerID || r.TrajectoryID != a.Binding.TrajectoryID ||
		r.RunID != a.BoundRunID || r.AssignedAgentID != a.Binding.AssignedAgentID {
		return fmt.Errorf("co-super assignment report: exact assignment/run scope binding is required")
	}
	switch r.Result {
	case EngineeringResultCompleted, EngineeringResultFailed, EngineeringResultBlocked, EngineeringResultPartial:
	default:
		return fmt.Errorf("co-super assignment report: invalid result %q", r.Result)
	}
	if !ValidSHA256Digest(r.ObservedSubjectDigest) {
		return fmt.Errorf("co-super assignment report: observed_subject_digest is required")
	}
	if strings.TrimSpace(r.Summary) == "" || r.Summary != strings.TrimSpace(r.Summary) {
		return fmt.Errorf("co-super assignment report: canonical summary is required")
	}
	seenEvidence := map[string]struct{}{}
	for _, ref := range r.EvidenceRefs {
		if strings.TrimSpace(ref) == "" || ref != strings.TrimSpace(ref) {
			return fmt.Errorf("co-super assignment report: evidence refs must be canonical")
		}
		if _, duplicate := seenEvidence[ref]; duplicate {
			return fmt.Errorf("co-super assignment report: duplicate evidence ref")
		}
		seenEvidence[ref] = struct{}{}
	}
	if a.Binding.Kind == EngineeringAssignmentImplementation {
		if r.Verdict != EngineeringVerdictNone {
			return fmt.Errorf("co-super assignment report: implementation cannot issue verification verdict")
		}
	} else {
		if r.Verdict != EngineeringVerdictPass && r.Verdict != EngineeringVerdictFail && r.Verdict != EngineeringVerdictAbstain {
			return fmt.Errorf("co-super assignment report: verification requires typed verdict")
		}
		if r.Verdict == EngineeringVerdictPass && r.Result != EngineeringResultCompleted {
			return fmt.Errorf("co-super assignment report: pass requires completed result")
		}
	}
	if a.Binding.Kind == EngineeringAssignmentVerification && r.Result == EngineeringResultCompleted && r.Verdict == EngineeringVerdictPass && (len(r.Commands) == 0 || len(r.ExecutorReceiptRefs) == 0) {
		return fmt.Errorf("co-super assignment report: verification pass requires exact successful frozen-subject executor receipt evidence")
	}
	if a.Binding.Kind == EngineeringAssignmentImplementation && r.Result == EngineeringResultCompleted && len(r.Commands) == 0 && len(r.Mutations) == 0 {
		return fmt.Errorf("co-super assignment report: implementation completion requires command or runtime-derived mutation evidence")
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
	for _, ref := range r.ExecutorReceiptRefs {
		if strings.TrimSpace(ref) == "" || ref != strings.TrimSpace(ref) {
			return fmt.Errorf("co-super assignment report: executor receipt refs must be canonical")
		}
		if _, duplicate := seen["executor:"+ref]; duplicate {
			return fmt.Errorf("co-super assignment report: duplicate executor receipt ref")
		}
		seen["executor:"+ref] = struct{}{}
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
	if changed && !r.Late {
		if r.CandidateSubjectDigest != r.ObservedSubjectDigest || strings.TrimSpace(r.CandidateID) == "" || r.CandidateArtifactRef != "capsule-subject:"+r.CandidateSubjectDigest || r.CertifiesOriginalSubject {
			return fmt.Errorf("co-super assignment report: changed subject requires a distinct non-certifying candidate identity")
		}
	} else if r.CandidateSubjectDigest != "" || r.CandidateID != "" || r.CandidateArtifactRef != "" {
		return fmt.Errorf("co-super assignment report: unchanged subject cannot create candidate identity")
	}
	if r.CertifiesOriginalSubject && (a.Binding.Kind != EngineeringAssignmentVerification || r.Verdict != EngineeringVerdictPass || r.Late || changed) {
		return fmt.Errorf("co-super assignment report: original subject certification requires timely immutable verification pass")
	}
	return nil
}

type EngineeringSubjectCandidate struct {
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
	ArtifactRef           string    `json:"artifact_ref"`
	CreatedAt             time.Time `json:"created_at"`
}

type EngineeringAssignmentCommandResult struct {
	Receipt    LifecycleCommandReceipt      `json:"receipt"`
	Assignment EngineeringAssignment        `json:"assignment"`
	Report     *EngineeringAssignmentReport `json:"report,omitempty"`
	Candidate  *EngineeringSubjectCandidate `json:"candidate,omitempty"`
	Update     *CoagentSourcePacket         `json:"update,omitempty"`
	Replay     bool                         `json:"replay"`
}

// EngineeringOrphanReason names why a child run is observed as orphaned (settlement gate item 6).
type EngineeringOrphanReason string

const (
	OrphanReasonProcessExitedWithoutPacket EngineeringOrphanReason = "process_exited_without_packet"
	OrphanReasonCancelled                  EngineeringOrphanReason = "cancelled"
	OrphanReasonConfirmedDead              EngineeringOrphanReason = "confirmed_dead"
)

// EngineeringOrphanObservation is an authenticated immutable observation submitted
// to the reducer when a child run terminates without a terminal packet (settlement gate item 6).
type EngineeringOrphanObservation struct {
	OwnerID      string                  `json:"owner_id"`
	ComputerID   string                  `json:"computer_id"`
	RunID        string                  `json:"run_id"`
	AssignmentID string                  `json:"assignment_id,omitempty"`
	Attempt      uint64                  `json:"attempt,omitempty"`
	Reason       EngineeringOrphanReason `json:"reason"`
	ObservedAt   time.Time               `json:"observed_at"`
	EvidenceRef  string                  `json:"evidence_ref,omitempty"`
}

func (o EngineeringOrphanObservation) Validate() error {
	if strings.TrimSpace(o.OwnerID) == "" || strings.TrimSpace(o.ComputerID) == "" || strings.TrimSpace(o.RunID) == "" {
		return fmt.Errorf("orphan observation: owner_id, computer_id, and run_id are required")
	}
	switch o.Reason {
	case OrphanReasonProcessExitedWithoutPacket, OrphanReasonCancelled, OrphanReasonConfirmedDead:
	default:
		return fmt.Errorf("orphan observation: invalid reason %q", o.Reason)
	}
	return nil
}

type OpenEngineeringAssignmentRequest struct {
	CommandID     string                       `json:"command_id"`
	CommandDigest string                       `json:"command_digest"`
	AssignmentID  string                       `json:"assignment_id"`
	Binding       EngineeringAssignmentBinding `json:"binding"`
	AssignedAgent AgentRecord                  `json:"assigned_agent"`
	AssignedWork  WorkItemRecord               `json:"assigned_work"`
	// Supersedes carries the frozen correction tuple (settlement gate item 3):
	// a correction lands only as a new attempt carrying it. Attempt 1 never
	// carries it; attempt > 1 always does.
	Supersedes *EngineeringSupersedeTuple `json:"supersedes,omitempty"`
}

// EngineeringSupersedeKind names why a new attempt supersedes a prior one.
type EngineeringSupersedeKind string

const (
	EngineeringSupersedeCorrection      EngineeringSupersedeKind = "correction"
	EngineeringSupersedeRetryAfterBlock EngineeringSupersedeKind = "retry_after_block"
	EngineeringSupersedeOwnerReopen     EngineeringSupersedeKind = "owner_reopen"
)

// EngineeringSupersedeTuple is the frozen correction tuple: who is superseded,
// which receipt it corrects, why, and a digest over structured fields only
// (never summary or prose).
type EngineeringSupersedeTuple struct {
	SupersedesAssignmentID string                   `json:"supersedes_assignment_id"`
	SupersedesAttempt      uint64                   `json:"supersedes_attempt"`
	PriorReceiptRef        string                   `json:"prior_receipt_ref"`
	SupersedeKind          EngineeringSupersedeKind `json:"supersede_kind"`
	ReasonEnum             string                   `json:"reason_enum"`
	DeltaDigest            string                   `json:"delta_digest"`
}
type BindEngineeringAssignmentRequest struct {
	CommandID                string                             `json:"command_id"`
	CommandDigest            string                             `json:"command_digest"`
	OwnerID                  string                             `json:"owner_id"`
	ComputerID               string                             `json:"computer_id"`
	AssignmentID             string                             `json:"assignment_id"`
	Attempt                  uint64                             `json:"attempt"`
	ExpectedLifecycleVersion int64                              `json:"expected_lifecycle_version"`
	RunID                    string                             `json:"loop_id"`
	Run                      RunRecord                          `json:"run"`
	OpaqueCapability         string                             `json:"-"`
	CapsuleID                string                             `json:"capsule_id,omitempty"`
	GrantPolicyAttestation   *EngineeringGrantPolicyAttestation `json:"grant_policy_attestation,omitempty"`
}

type RecordEngineeringAssignmentReportRequest struct {
	CommandID                string                            `json:"command_id"`
	CommandDigest            string                            `json:"command_digest"`
	OwnerID                  string                            `json:"owner_id"`
	ComputerID               string                            `json:"computer_id"`
	AssignmentID             string                            `json:"assignment_id"`
	Attempt                  uint64                            `json:"attempt"`
	ExpectedLifecycleVersion int64                             `json:"expected_lifecycle_version"`
	Report                   EngineeringAssignmentReport       `json:"report"`
	ExecutionAttestations    []EngineeringExecutionAttestation `json:"execution_attestations,omitempty"`
}

type CancelEngineeringAssignmentRequest struct {
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	AssignmentID             string `json:"assignment_id"`
	Attempt                  uint64 `json:"attempt"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	Reason                   string `json:"reason"`
}

type SetEngineeringCapsuleDispositionRequest struct {
	CommandID                string                        `json:"command_id"`
	CommandDigest            string                        `json:"command_digest"`
	OwnerID                  string                        `json:"owner_id"`
	ComputerID               string                        `json:"computer_id"`
	AssignmentID             string                        `json:"assignment_id"`
	Attempt                  uint64                        `json:"attempt"`
	ExpectedLifecycleVersion int64                         `json:"expected_lifecycle_version"`
	Disposition              EngineeringCapsuleDisposition `json:"capsule_disposition"`
	IntentRef                string                        `json:"intent_ref"`
	AckRef                   string                        `json:"ack_ref,omitempty"`
	FateStep                 *EngineeringCapsuleFateStep   `json:"fate_step,omitempty"`
	PendingProposal          *EngineeringPendingProposal   `json:"pending_proposal,omitempty"`
}
