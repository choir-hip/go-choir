package computerevent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrProjectionMismatch = errors.New("computer event projection mismatch")
	ErrPendingTransition  = errors.New("computer event transition already pending")
	ErrInvalidTransition  = errors.New("invalid computer event transition")
)

type Head struct {
	ComputerID                string `json:"computer_id"`
	Sequence                  uint64 `json:"sequence"`
	CanonicalEventHead        string `json:"canonical_event_head"`
	DesiredEventHead          string `json:"desired_event_head"`
	EffectiveEventHead        string `json:"effective_event_head"`
	PendingTransitionRef      string `json:"pending_transition_ref"`
	DesiredStateCommitment    string `json:"desired_state_commitment"`
	EffectiveStateCommitment  string `json:"effective_state_commitment"`
	ReducerVersion            int    `json:"reducer_version"`
	CredentialRevocationEpoch uint64 `json:"credential_revocation_epoch"`
}

type TransitionInput struct {
	// TargetStateCommitment is resolved from the immutable proposed effect,
	// rollback decision, or typed Researcher mutation referenced by the event.
	TargetStateCommitment string `json:"target_state_commitment"`
	// RestoredPriorEffective is set only after a verified MaterializationReceipt
	// or UpdaterRecoveryReceipt proves the previous effective release is restored.
	RestoredPriorEffective bool `json:"restored_prior_effective"`
}

type EffectiveStateRefs struct {
	ReducerVersion     int      `json:"reducer_version"`
	CodeRef            string   `json:"code_ref"`
	ArtifactProgramRef string   `json:"artifact_program_ref"`
	EmbeddedDoltRefs   []string `json:"embedded_dolt_refs"`
}

func StateCommitment(refs EffectiveStateRefs) (string, error) {
	if refs.ReducerVersion != ReducerVersionV1 || refs.CodeRef == "" || refs.ArtifactProgramRef == "" {
		return "", fmt.Errorf("state commitment: reducer, code, and artifact program refs are required")
	}
	refs.EmbeddedDoltRefs = nonNilStrings(refs.EmbeddedDoltRefs)
	body, err := CanonicalJSON(refs)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

// Reduce validates the event against the current projections and returns the
// deterministic next head. A nil current head is valid only for GenesisImported.
func Reduce(current *Head, event Event, input TransitionInput) (Head, error) {
	if err := event.Validate(); err != nil {
		return Head{}, err
	}
	digest, err := event.Digest()
	if err != nil {
		return Head{}, err
	}
	if current == nil {
		return reduceGenesis(event, digest, input)
	}
	if current.ComputerID != event.ComputerID || event.Sequence != current.Sequence+1 || event.PreviousHead != current.CanonicalEventHead {
		return Head{}, fmt.Errorf("%w: canonical head or sequence", ErrProjectionMismatch)
	}
	if current.ReducerVersion != event.ReducerVersion || event.ExpectedDesiredEventHead != current.DesiredEventHead || event.ExpectedEffectiveEventHead != current.EffectiveEventHead || event.ExpectedPendingTransitionRef != current.PendingTransitionRef || event.ExpectedDesiredStateCommitment != current.DesiredStateCommitment || event.ExpectedEffectiveStateCommitment != current.EffectiveStateCommitment {
		return Head{}, fmt.Errorf("%w: expected state projection", ErrProjectionMismatch)
	}
	next := *current
	next.Sequence = event.Sequence
	next.CanonicalEventHead = digest

	switch event.EventKind {
	case EventEffectAccepted:
		if current.PendingTransitionRef != "" {
			return Head{}, ErrPendingTransition
		}
		if !isSHA256(input.TargetStateCommitment) || event.ProposedEffectRef == "" || event.DecisionRef == "" || len(event.VerifierRefs) == 0 {
			return Head{}, fmt.Errorf("%w: accepted effect is not fully bound", ErrInvalidTransition)
		}
		next.DesiredEventHead = digest
		next.DesiredStateCommitment = input.TargetStateCommitment
		next.PendingTransitionRef = digest
	case EventMaterializationStarted:
		if current.PendingTransitionRef == "" || event.DecisionRef != current.PendingTransitionRef {
			return Head{}, fmt.Errorf("%w: materialization does not bind pending transition", ErrInvalidTransition)
		}
	case EventMaterializationApplied:
		if current.PendingTransitionRef == "" || event.DecisionRef != current.PendingTransitionRef || event.ResultingEffectiveCommitment != current.DesiredStateCommitment {
			return Head{}, fmt.Errorf("%w: applied materialization does not match desired state", ErrInvalidTransition)
		}
		next.EffectiveEventHead = digest
		next.EffectiveStateCommitment = current.DesiredStateCommitment
		next.PendingTransitionRef = ""
	case EventMaterializationFailed:
		if current.PendingTransitionRef == "" || event.DecisionRef != current.PendingTransitionRef || !input.RestoredPriorEffective {
			return Head{}, fmt.Errorf("%w: failed materialization lacks verified recovery", ErrInvalidTransition)
		}
		next.DesiredEventHead = current.EffectiveEventHead
		next.DesiredStateCommitment = current.EffectiveStateCommitment
		next.PendingTransitionRef = ""
	case EventRollbackRequested:
		if current.PendingTransitionRef != "" {
			return Head{}, ErrPendingTransition
		}
		if !isSHA256(input.TargetStateCommitment) || event.DecisionRef == "" {
			return Head{}, fmt.Errorf("%w: rollback target is not fully bound", ErrInvalidTransition)
		}
		next.DesiredEventHead = digest
		next.DesiredStateCommitment = input.TargetStateCommitment
		next.PendingTransitionRef = digest
	case EventRollbackApplied:
		if current.PendingTransitionRef == "" || event.DecisionRef != current.PendingTransitionRef || event.ResultingEffectiveCommitment != current.DesiredStateCommitment {
			return Head{}, fmt.Errorf("%w: applied rollback does not match desired state", ErrInvalidTransition)
		}
		next.EffectiveEventHead = digest
		next.EffectiveStateCommitment = current.DesiredStateCommitment
		next.PendingTransitionRef = ""
	case EventResearcherUpdate:
		if current.PendingTransitionRef != "" {
			return Head{}, ErrPendingTransition
		}
		if !isSHA256(input.TargetStateCommitment) || event.ResultingEffectiveCommitment != input.TargetStateCommitment {
			return Head{}, fmt.Errorf("%w: researcher update commitment mismatch", ErrInvalidTransition)
		}
		next.DesiredEventHead = digest
		next.EffectiveEventHead = digest
		next.DesiredStateCommitment = input.TargetStateCommitment
		next.EffectiveStateCommitment = input.TargetStateCommitment
	case EventKeyRevoked:
		next.CredentialRevocationEpoch++
	case EventGenesisImported:
		return Head{}, fmt.Errorf("%w: duplicate genesis", ErrInvalidTransition)
	default:
		if !isCausalEvent(event.EventKind) {
			return Head{}, fmt.Errorf("%w: unsupported reducer event %q", ErrInvalidTransition, event.EventKind)
		}
	}
	return next, nil
}

func reduceGenesis(event Event, digest string, input TransitionInput) (Head, error) {
	if event.EventKind != EventGenesisImported || event.Sequence != 1 || event.PreviousHead != ZeroHead || event.ExpectedDesiredEventHead != ZeroHead || event.ExpectedEffectiveEventHead != ZeroHead || event.ExpectedPendingTransitionRef != "" || !isSHA256(input.TargetStateCommitment) || event.ResultingEffectiveCommitment != input.TargetStateCommitment {
		return Head{}, fmt.Errorf("%w: invalid genesis", ErrInvalidTransition)
	}
	return Head{
		ComputerID:               event.ComputerID,
		Sequence:                 1,
		CanonicalEventHead:       digest,
		DesiredEventHead:         digest,
		EffectiveEventHead:       digest,
		DesiredStateCommitment:   input.TargetStateCommitment,
		EffectiveStateCommitment: input.TargetStateCommitment,
		ReducerVersion:           event.ReducerVersion,
	}, nil
}

func isCausalEvent(kind EventKind) bool {
	switch kind {
	case EventTrajectoryStarted, EventModelResolved, EventMessageRecorded,
		EventToolInvoked, EventToolReturned, EventArtifactProduced,
		EventEffectProposed, EventVerificationRecorded, EventEffectRejected,
		EventCheckpointPublished, EventRouteProjectionUpdated,
		EventLifecycleObserved, EventKeyRotated, EventKeyRevoked,
		EventRecoveryRecorded:
		return true
	default:
		return false
	}
}
