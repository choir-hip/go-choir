package computerevent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SchemaVersionV1  = 1
	ReducerVersionV1 = 1
	ZeroHead         = "0000000000000000000000000000000000000000000000000000000000000000"
)

type EventKind string

const (
	EventGenesisImported        EventKind = "genesis_imported"
	EventTrajectoryStarted      EventKind = "trajectory_started"
	EventModelResolved          EventKind = "model_resolved"
	EventMessageRecorded        EventKind = "message_recorded"
	EventToolInvoked            EventKind = "tool_invoked"
	EventToolReturned           EventKind = "tool_returned"
	EventArtifactProduced       EventKind = "artifact_produced"
	EventEffectProposed         EventKind = "effect_proposed"
	EventVerificationRecorded   EventKind = "verification_recorded"
	EventEffectAccepted         EventKind = "effect_accepted"
	EventEffectRejected         EventKind = "effect_rejected"
	EventMaterializationStarted EventKind = "materialization_started"
	EventMaterializationApplied EventKind = "materialization_applied"
	EventMaterializationFailed  EventKind = "materialization_failed"
	EventRollbackRequested      EventKind = "rollback_requested"
	EventRollbackApplied        EventKind = "rollback_applied"
	EventResearcherUpdate       EventKind = "researcher_update"
	EventCheckpointPublished    EventKind = "checkpoint_published"
	EventRouteProjectionUpdated EventKind = "route_projection_updated"
	EventLifecycleObserved      EventKind = "lifecycle_observed"
	EventKeyRotated             EventKind = "key_rotated"
	EventKeyRevoked             EventKind = "key_revoked"
	EventRecoveryRecorded       EventKind = "recovery_recorded"
)

var validEventKinds = map[EventKind]struct{}{
	EventGenesisImported: {}, EventTrajectoryStarted: {}, EventModelResolved: {},
	EventMessageRecorded: {}, EventToolInvoked: {}, EventToolReturned: {},
	EventArtifactProduced: {}, EventEffectProposed: {}, EventVerificationRecorded: {},
	EventEffectAccepted: {}, EventEffectRejected: {}, EventMaterializationStarted: {},
	EventMaterializationApplied: {}, EventMaterializationFailed: {}, EventRollbackRequested: {},
	EventRollbackApplied: {}, EventResearcherUpdate: {}, EventCheckpointPublished: {},
	EventRouteProjectionUpdated: {}, EventLifecycleObserved: {}, EventKeyRotated: {},
	EventKeyRevoked: {}, EventRecoveryRecorded: {},
}

// Event is the complete V1 semantic event envelope. Event bodies contain no
// self digest; Digest computes the canonical event head externally.
type Event struct {
	SchemaVersion                    int       `json:"schema_version"`
	EventID                          string    `json:"event_id"`
	ComputerID                       string    `json:"computer_id"`
	Sequence                         uint64    `json:"sequence"`
	PreviousHead                     string    `json:"previous_head"`
	EventKind                        EventKind `json:"event_kind"`
	OccurredAt                       string    `json:"occurred_at"`
	IdempotencyKey                   string    `json:"idempotency_key"`
	RequestCommitment                string    `json:"request_commitment"`
	TrajectoryID                     string    `json:"trajectory_id"`
	ParentEventID                    string    `json:"parent_event_id"`
	CapsuleID                        string    `json:"capsule_id"`
	ActorProfile                     string    `json:"actor_profile"`
	AuthorityRef                     string    `json:"authority_ref"`
	ModelPolicyRefs                  []string  `json:"model_policy_refs"`
	InputArtifactRefs                []string  `json:"input_artifact_refs"`
	OutputArtifactRefs               []string  `json:"output_artifact_refs"`
	PayloadCommitment                string    `json:"payload_commitment"`
	PrivacyClass                     string    `json:"privacy_class"`
	ProposedEffectRef                string    `json:"proposed_effect_ref"`
	DecisionRef                      string    `json:"decision_ref"`
	VerifierRefs                     []string  `json:"verifier_refs"`
	ReducerVersion                   int       `json:"reducer_version"`
	ExpectedDesiredEventHead         string    `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string    `json:"expected_effective_event_head"`
	ExpectedPendingTransitionRef     string    `json:"expected_pending_transition_ref"`
	ExpectedDesiredStateCommitment   string    `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string    `json:"expected_effective_state_commitment"`
	RequireExpectedHead              bool      `json:"-"`
	ResultingEffectiveCommitment     string    `json:"resulting_effective_state_commitment"`
}

func NewEventID() (string, error) {
	id, err := uuid.NewV7FromReader(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("computer event: uuidv7: %w", err)
	}
	return id.String(), nil
}

func (e Event) CanonicalBytes() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	normalized := e
	normalized.ModelPolicyRefs = nonNilStrings(e.ModelPolicyRefs)
	normalized.InputArtifactRefs = nonNilStrings(e.InputArtifactRefs)
	normalized.OutputArtifactRefs = nonNilStrings(e.OutputArtifactRefs)
	normalized.VerifierRefs = nonNilStrings(e.VerifierRefs)
	return CanonicalJSON(normalized)
}

func (e Event) Digest() (string, error) {
	body, err := e.CanonicalBytes()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func (e Event) Validate() error {
	if e.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("computer event: unsupported schema version %d", e.SchemaVersion)
	}
	parsedID, err := uuid.Parse(e.EventID)
	if err != nil || parsedID.Version() != 7 {
		return fmt.Errorf("computer event: event_id must be UUIDv7")
	}
	if strings.TrimSpace(e.ComputerID) == "" {
		return fmt.Errorf("computer event: computer_id is required")
	}
	if e.Sequence == 0 {
		return fmt.Errorf("computer event: sequence must be positive")
	}
	if !isSHA256(e.PreviousHead) {
		return fmt.Errorf("computer event: previous_head must be lowercase SHA-256")
	}
	if _, ok := validEventKinds[e.EventKind]; !ok {
		return fmt.Errorf("computer event: unknown event kind %q", e.EventKind)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, e.OccurredAt)
	if err != nil || occurredAt.Location() != time.UTC || occurredAt.Format(time.RFC3339Nano) != e.OccurredAt {
		return fmt.Errorf("computer event: occurred_at must be canonical UTC RFC3339")
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" || !isSHA256(e.RequestCommitment) {
		return fmt.Errorf("computer event: idempotency_key and request_commitment are required")
	}
	if strings.TrimSpace(e.ActorProfile) == "" || strings.TrimSpace(e.AuthorityRef) == "" || strings.TrimSpace(e.PrivacyClass) == "" {
		return fmt.Errorf("computer event: actor_profile, authority_ref, and privacy_class are required")
	}
	if e.ReducerVersion != ReducerVersionV1 {
		return fmt.Errorf("computer event: unsupported reducer version %d", e.ReducerVersion)
	}
	for name, value := range map[string]string{
		"payload_commitment":                  e.PayloadCommitment,
		"expected_desired_event_head":         e.ExpectedDesiredEventHead,
		"expected_effective_event_head":       e.ExpectedEffectiveEventHead,
		"expected_desired_state_commitment":   e.ExpectedDesiredStateCommitment,
		"expected_effective_state_commitment": e.ExpectedEffectiveStateCommitment,
	} {
		if !isSHA256(value) {
			return fmt.Errorf("computer event: %s must be lowercase SHA-256", name)
		}
	}
	if e.ExpectedPendingTransitionRef != "" && !isSHA256(e.ExpectedPendingTransitionRef) {
		return fmt.Errorf("computer event: expected_pending_transition_ref must be lowercase SHA-256 or empty")
	}
	if e.ResultingEffectiveCommitment != "" && !isSHA256(e.ResultingEffectiveCommitment) {
		return fmt.Errorf("computer event: resulting_effective_state_commitment must be lowercase SHA-256 or empty")
	}
	return nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func IsSHA256(value string) bool {
	return isSHA256(value)
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
