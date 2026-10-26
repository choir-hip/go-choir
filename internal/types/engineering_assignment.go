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
	// EngineeringSubjectBindingObserved marks an execution attestation on a
	// failed or blocked report: the receipt is bound to the run, handle and
	// capsule, and WorktreeDigest is the tree the command itself observed, not
	// a certification of FinalSubjectDigest. Empty means certified.
	EngineeringSubjectBindingObserved = "observed"
	EngineeringCapsuleFateStepSchemaV1                         = "choir.co_super_capsule_fate_step/v1"
)

// EngineeringCapsuleNetworkEcosystemProxy (L2) gives an implementation
// capsule live egress to package, source and paper ecosystems through the
// executor's allowlisting proxy
// (docs/design/engineering-network-grants-2026-10-10.md §4.0b).
const EngineeringCapsuleNetworkEcosystemProxy = "ecosystem_proxy"

// EngineeringCapsuleNetworkClaude is the Claude delegate's mode: an
// implementation capsule on the one allowlisted computer reaches only the
// model API and the Go module proxy, and its broker holds the owner's token
// for headless Claude Code (docs/design/claude-delegate-slice1-2026-10-14.md).
const EngineeringCapsuleNetworkClaude = "claude"

type EngineeringAssignmentKind string

// EngineeringCastAuthority names which admission authority admitted the
// assignment. Records (C1e B1) is the only one minted: management's Cast to
// engineering on its records route, serving the open round a Texture's Cast
// opened with management. Bindings minted before C1e carry "", "owner" or
// "delegated" and work-item parents; they decode for history only.
type EngineeringCastAuthority string

const (
	EngineeringCastAuthorityRecords EngineeringCastAuthority = "records"
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
	// CastAuthority is "records" for every assignment C1e mints; see
	// EngineeringCastAuthority.
	CastAuthority EngineeringCastAuthority `json:"cast_authority,omitempty"`
	// ParentConversationID and ParentRound name the round of management's
	// conversation with a Texture that the engineering Cast serves (C1e D4):
	// the round the Cast's ParentID record opened. They replace
	// ParentWorkItemID on records bindings.
	ParentConversationID string `json:"parent_conversation_id,omitempty"`
	ParentRound          int    `json:"parent_round,omitempty"`
	// AssignedConversationID is the engineering conversation
	// management:<owner>|engineering:<assignmentID>; its round 1 is the
	// assignment (C1e D1). It replaces AssignedWorkItemID on records
	// bindings.
	AssignedConversationID string `json:"assigned_conversation_id,omitempty"`
}

// Records reports whether the binding was admitted on management's records
// route (C1e B1).
func (b EngineeringAssignmentBinding) Records() bool {
	return b.CastAuthority == EngineeringCastAuthorityRecords
}

func (b EngineeringAssignmentBinding) Validate() error {
	required := map[string]string{
		"owner_id": b.OwnerID, "computer_id": b.ComputerID, "trajectory_id": b.TrajectoryID,
		"parent_agent_id":    b.ParentAgentID,
		"parent_decision_id": b.ParentDecisionID, "parent_control_id": b.ParentControlID,
		"assigned_agent_id": b.AssignedAgentID,
	}
	if b.Records() {
		required["parent_loop_id"] = b.ParentRunID
		required["parent_conversation_id"] = b.ParentConversationID
		required["assigned_conversation_id"] = b.AssignedConversationID
	} else {
		required["parent_work_item_id"] = b.ParentWorkItemID
		required["assigned_work_item_id"] = b.AssignedWorkItemID
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("co-super assignment: %s is required and must be canonical", name)
		}
	}
	if !strings.HasPrefix(b.ParentDecisionID, "decision:sha256:") || !ValidSHA256Digest(strings.TrimPrefix(b.ParentDecisionID, "decision:")) {
		return fmt.Errorf("co-super assignment: parent_decision_id must be runtime-derived")
	}
	if b.Records() {
		// C1e D1/D4: one conversation per assignment, serving one open round
		// of management's conversation with a Texture. Attempts above 1 and
		// verification assignments do not exist on records.
		management := agentprofile.Management + ":" + b.OwnerID
		if b.ParentAgentID != management {
			return fmt.Errorf("co-super assignment: records parent_agent_id must be exact management:<owner>")
		}
		if !strings.HasPrefix(b.AssignedAgentID, agentprofile.Engineering+":") {
			return fmt.Errorf("co-super assignment: records assigned agent must be engineering:<assignment>")
		}
		if b.AssignedConversationID != InboxConversationID(b.ParentAgentID, b.AssignedAgentID) {
			return fmt.Errorf("co-super assignment: assigned conversation must be management:<owner>|engineering:<assignment>")
		}
		other, ok := conversationCounterpart(b.ParentConversationID, management)
		if !ok || !strings.HasPrefix(other, agentprofile.Texture+":") || b.ParentRound <= 0 {
			return fmt.Errorf("co-super assignment: records parent must be an open round of management's conversation with a Texture")
		}
		if b.ParentWorkItemID != "" || b.AssignedWorkItemID != "" {
			return fmt.Errorf("co-super assignment: records bindings carry no work items")
		}
		if b.Kind != EngineeringAssignmentImplementation || b.Attempt != 1 {
			return fmt.Errorf("co-super assignment: records assignments are implementation attempt 1")
		}
	} else if b.ParentRunID == "" {
		// Document-parent binding: the owner-authored revision on the bound
		// document is the parent authority; there is no parent run. The parent
		// agent is the engineering desk agent bound to that document.
		if !strings.HasPrefix(b.ParentAgentID, agentprofile.Engineering+":") {
			return fmt.Errorf("co-super assignment: document-parent binding requires an engineering desk parent agent")
		}
	} else if b.ParentAgentID != agentprofile.Management+":"+b.OwnerID {
		return fmt.Errorf("co-super assignment: parent_agent_id must be exact persistent management:<owner>")
	}
	if b.AssignedAgentID == b.ParentAgentID || (!b.Records() && b.AssignedWorkItemID == b.ParentWorkItemID) {
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
	switch b.NetworkMode {
	case EngineeringCapsuleNetworkForbidden, EngineeringCapsuleNetworkNone:
	case EngineeringCapsuleNetworkEcosystemProxy, EngineeringCapsuleNetworkClaude:
		// The verifier rebuilds offline from the frozen bundle; only
		// implementation work reaches the ecosystems or the model API.
		if b.Kind != EngineeringAssignmentImplementation {
			return fmt.Errorf("co-super assignment: %s network is for implementation assignments only", b.NetworkMode)
		}
	default:
		return fmt.Errorf("co-super assignment: capsule network_mode must be forbidden, none, ecosystem_proxy or claude")
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

// conversationCounterpart returns the inbox other than self in a
// conversation id (types.InboxConversationID).
func conversationCounterpart(conversationID, self string) (string, bool) {
	raw, ok := strings.CutPrefix(conversationID, "conv:")
	if !ok {
		return "", false
	}
	a, b, ok := strings.Cut(raw, "|")
	if !ok || InboxConversationID(a, b) != conversationID {
		return "", false
	}
	switch self {
	case a:
		return b, true
	case b:
		return a, true
	}
	return "", false
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
	// SubjectBinding is EngineeringSubjectBindingObserved on a failed or
	// blocked report, empty (certified) otherwise. Omitted when empty so
	// every existing attestation keeps its ref.
	SubjectBinding string `json:"subject_binding,omitempty"`
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
	// Records is the receipt of the inbox command an Open composed (C1e D4):
	// the casting cell's records and cursor commit in the open's batch.
	Records *InboxCommitReceipt `json:"records,omitempty"`
	Replay  bool                `json:"replay"`
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
	// Records is the casting cell's whole inbox command (C1e D4): its records
	// (the engineering Cast among them), cursor advance and verdicts. The
	// store plans it inside the open's lock and commits it in the open's
	// batch, so no round exists without its assignment.
	Records CommitInboxRecordsRequest `json:"records"`
	// Rebase is set by the runtime exactly when it admitted a rebase Cast
	// (U4, docs/design/u4-stack-rebase-build-2026-10-26.md): the Cast's spec
	// names a rebase notice. It carries the clock the store does not read,
	// and is an input to the command digest. The store binds the notice to
	// this assignment, and the pair's claim, in the open's own transaction;
	// a spec that names a notice without it, or it without a notice, is
	// refused.
	Rebase *RebaseAdmission `json:"rebase,omitempty"`
}

// RebaseAdmission is what the runtime knows about this boot that the store
// cannot: when the runtime started and whether the boot followed a planned
// restart (a marker). A notice committed before a markerless boot was closed
// by the crash and is never acted on.
type RebaseAdmission struct {
	BootedAt    time.Time `json:"booted_at"`
	PlannedBoot bool      `json:"planned_boot,omitempty"`
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
	// OutcomeRefs name the candidate the run froze (selfdev operation and
	// bundle digest refs, C1e D2). They ride the Report record the store
	// mints into management's inbox, never the assignment report's
	// proposition.
	OutcomeRefs []string `json:"outcome_refs,omitempty"`
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
	// Silent marks a crash boot's cancellation (C1e D7): it mints no Report
	// even if the engineering round is somehow still open, so a boot never
	// wakes management.
	Silent bool `json:"silent,omitempty"`
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
