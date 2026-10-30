package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/owneract"
)

const (
	SelfDevelopmentModeOff                = "off"
	SelfDevelopmentModeAuditOnly          = "audit_only"
	SelfDevelopmentModeProposeOnly        = "propose_only"
	SelfDevelopmentModeAcceptOnce         = "accept_once"
	SelfDevelopmentModeQualifiedConsensus = "qualified_consensus"
	// SelfDevelopmentModeStandingConsensus is the P4 standing autonomous
	// mode: bound to a computer, one policy and an expiry, never to an
	// operation, and never consumed by the guest. Only an owner act arms
	// it, only on a test-tier computer, for at most
	// StandingConsensusMaxWindow (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md).
	SelfDevelopmentModeStandingConsensus = "standing_consensus"
)

// StandingConsensusMaxWindow bounds a standing arm's expiry.
const StandingConsensusMaxWindow = 24 * time.Hour

// ComputerTierTest is the one tier a standing arm may name: vmctl's
// warmness class for a computer of a test account.
const ComputerTierTest = "test"

// lazyExpiringMode reports a mode corpusd turns off once its expiry passes.
func lazyExpiringMode(mode string) bool {
	return mode == SelfDevelopmentModeAcceptOnce || mode == SelfDevelopmentModeQualifiedConsensus || mode == SelfDevelopmentModeStandingConsensus
}

var ErrSelfDevelopmentModeConflict = errors.New("self-development mode CAS conflict")

// ErrSelfDevelopmentModeOwnerRequired refuses a mode write that is not an
// owner act. Only the owner sets the mode: the proxy reaches corpusd as a
// host-internal caller after it has authenticated the owner's session or an
// owner API key scoped computer:self_development:mode. The guest's
// event:append capability may only consume the grant the owner armed
// (docs/archive/problems/guest-can-arm-own-selfdev-mode-2026-10-15.md).
var ErrSelfDevelopmentModeOwnerRequired = errors.New("self-development mode: owner authority required")

// Mode writers, recorded as mode_writer in every new ModeReceipt. Receipts
// signed before this field existed carry none; they were all written before
// the owner-only rule and are not re-judged.
const (
	ModeWriterHostInternal     = "host-internal"
	ModeWriterGuestConsumption = "guest-consumption"
	ModeWriterCorpusdExpiry    = "corpusd-expiry"
)

// Consumption idempotency keys: agentcore's decision handler writes
// <prefix><operation_id>:<decision binding digest> when it consumes an armed
// grant, and its consumed-receipt verifier expects the same spelling.
const (
	AcceptOnceConsumptionKeyPrefix         = "accept-once-consumed:"
	QualifiedConsensusConsumptionKeyPrefix = "qualified-consensus-consumed:"
)

type SelfDevelopmentMode struct {
	ComputerID                       string                 `json:"computer_id"`
	Mode                             string                 `json:"mode"`
	Generation                       uint64                 `json:"generation"`
	OperationID                      string                 `json:"operation_id,omitempty"`
	BundleDigest                     string                 `json:"bundle_digest,omitempty"`
	ExpectedDesiredEventHead         string                 `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead       string                 `json:"expected_effective_event_head,omitempty"`
	ExpectedPendingTransitionRef     string                 `json:"expected_pending_transition_ref,omitempty"`
	ExpectedDesiredStateCommitment   string                 `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string                 `json:"expected_effective_state_commitment,omitempty"`
	ExpiresAt                        string                 `json:"expires_at,omitempty"`
	PolicyDigest                     string                 `json:"policy_digest,omitempty"`
	ConsensusReceiptDigest           string                 `json:"consensus_receipt_digest,omitempty"`
	Receipt                          *computerevent.Receipt `json:"receipt,omitempty"`
}

type SetSelfDevelopmentModeRequest struct {
	Mode                             string  `json:"mode"`
	ExpectedGeneration               uint64  `json:"expected_generation"`
	OperationID                      string  `json:"operation_id,omitempty"`
	BundleDigest                     string  `json:"bundle_digest,omitempty"`
	ExpectedDesiredEventHead         string  `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead       string  `json:"expected_effective_event_head,omitempty"`
	ExpectedPendingTransitionRef     *string `json:"expected_pending_transition_ref,omitempty"`
	ExpectedDesiredStateCommitment   string  `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string  `json:"expected_effective_state_commitment,omitempty"`
	ExpiresAt                        string  `json:"expires_at,omitempty"`
	PolicyDigest                     string  `json:"policy_digest,omitempty"`
	ConsensusReceiptDigest           string  `json:"consensus_receipt_digest,omitempty"`
	IdempotencyKey                   string  `json:"idempotency_key"`
}

type SelfDevelopmentModeCAS struct {
	locks      [64]sync.Mutex
	store      *Store
	signingKey computerevent.SigningKey
	now        func() time.Time
	// ownerActs is the S1 owner-act policy; nil is the pre-S1 rule (a
	// host-internal caller is the owner).
	ownerActs *OwnerActAuthority
}

// ConfigureOwnerActs installs the S1 owner-act policy for owner writes.
func (c *SelfDevelopmentModeCAS) ConfigureOwnerActs(authority *OwnerActAuthority) {
	c.ownerActs = authority
}

func NewSelfDevelopmentModeCAS(store *Store, signingKey computerevent.SigningKey) (*SelfDevelopmentModeCAS, error) {
	if store == nil || store.db == nil || signingKey.SignerDomain != "platform-control" || signingKey.KeyID == "" || len(signingKey.PrivateKey) == 0 {
		return nil, fmt.Errorf("self-development mode: complete store and platform-control signer are required")
	}
	return &SelfDevelopmentModeCAS{store: store, signingKey: signingKey, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (c *SelfDevelopmentModeCAS) Get(ctx context.Context, computerID string) (SelfDevelopmentMode, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: computer_id is required")
	}
	mode, found, err := readSelfDevelopmentMode(ctx, c.store.db, computerID, false)
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	if !found {
		return SelfDevelopmentMode{ComputerID: computerID, Mode: SelfDevelopmentModeOff}, nil
	}
	hydrateSelfDevelopmentModeBindings(&mode)
	if !lazyExpiringMode(mode.Mode) {
		return mode, nil
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, mode.ExpiresAt)
	if err != nil {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: invalid durable expiry: %w", err)
	}
	if c.now().Before(expiresAt) {
		return mode, nil
	}
	expired, err := c.set(ctx, computerID, SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeOff, ExpectedGeneration: mode.Generation,
		IdempotencyKey: fmt.Sprintf("selfdev-mode-expiry-v1:%s:%d", computerID, mode.Generation),
	}, ModeWriterCorpusdExpiry, nil)
	if err == nil {
		return expired, nil
	}
	if !errors.Is(err, ErrSelfDevelopmentModeConflict) {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: persist expiry: %w", err)
	}
	current, found, readErr := readSelfDevelopmentMode(ctx, c.store.db, computerID, false)
	if readErr != nil {
		return SelfDevelopmentMode{}, readErr
	}
	if !found {
		return SelfDevelopmentMode{ComputerID: computerID, Mode: SelfDevelopmentModeOff}, nil
	}
	if lazyExpiringMode(current.Mode) {
		currentExpiry, parseErr := time.Parse(time.RFC3339Nano, current.ExpiresAt)
		if parseErr != nil || !c.now().Before(currentExpiry) {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: expired one-shot mode reconciliation conflicted")
		}
	}
	return current, nil
}

// Set is the owner's mode write. Its only callers are host-internal: the
// proxy, after it has authenticated the owner.
func (c *SelfDevelopmentModeCAS) Set(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest) (SelfDevelopmentMode, error) {
	return c.set(ctx, computerID, request, ModeWriterHostInternal, nil)
}

// SetAsOwner is the owner's mode write carrying the owner-act receipt the
// handler verified (nil when it carried none and the policy observes).
func (c *SelfDevelopmentModeCAS) SetAsOwner(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest, ownerAct *owneract.Verified) (SelfDevelopmentMode, error) {
	return c.set(ctx, computerID, request, ModeWriterHostInternal, ownerAct)
}

// ConsumeAsGuest is the only mode write a guest capability may make: the
// decision handler's consumption of the grant the owner armed, accept_once
// or qualified_consensus to propose_only, under the consumption key that
// names the armed operation. Everything else is ErrSelfDevelopmentModeOwnerRequired.
func (c *SelfDevelopmentModeCAS) ConsumeAsGuest(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest) (SelfDevelopmentMode, error) {
	return c.set(ctx, computerID, request, ModeWriterGuestConsumption, nil)
}

// guestConsumptionAllowed admits exactly the two consumption writes in
// agentcore's decision handler (api_self_development.go): the current mode
// is an armed grant, the next mode is propose_only, and the idempotency key
// is that grant's consumption key for the armed operation.
func guestConsumptionAllowed(current SelfDevelopmentMode, request SetSelfDevelopmentModeRequest) bool {
	prefix := ""
	switch current.Mode {
	case SelfDevelopmentModeAcceptOnce:
		prefix = AcceptOnceConsumptionKeyPrefix
	case SelfDevelopmentModeQualifiedConsensus:
		prefix = QualifiedConsensusConsumptionKeyPrefix
	default:
		return false
	}
	operationID := strings.TrimSpace(current.OperationID)
	if request.Mode != SelfDevelopmentModeProposeOnly || operationID == "" {
		return false
	}
	digest, ok := strings.CutPrefix(request.IdempotencyKey, prefix+operationID+":")
	return ok && computerevent.IsSHA256(digest)
}

func (c *SelfDevelopmentModeCAS) set(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest, writer string, ownerAct *owneract.Verified) (SelfDevelopmentMode, error) {
	computerID = strings.TrimSpace(computerID)
	request.Mode = strings.TrimSpace(request.Mode)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if computerID == "" || request.IdempotencyKey == "" {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: computer_id and idempotency_key are required")
	}
	requestCommitment, err := selfDevelopmentModeRequestCommitment(computerID, request)
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	lock := &c.locks[computerEventLockIndex(computerID)]
	lock.Lock()
	defer lock.Unlock()

	tx, err := c.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: begin: %w", err)
	}
	defer tx.Rollback()
	current, found, err := readSelfDevelopmentMode(ctx, tx, computerID, true)
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	if found && current.Receipt != nil && current.Receipt.KindFields["idempotency_key"] == request.IdempotencyKey {
		if current.Receipt.KindFields["request_commitment"] != requestCommitment {
			return SelfDevelopmentMode{}, fmt.Errorf("%w: idempotency commitment changed", ErrSelfDevelopmentModeConflict)
		}
		return current, nil
	}
	if !found {
		current = SelfDevelopmentMode{ComputerID: computerID, Mode: SelfDevelopmentModeOff}
	}
	switch writer {
	case ModeWriterHostInternal:
		if c.ownerActs != nil {
			if ownerAct != nil {
				replayed, replayErr := ownerActReplayed(ctx, tx, ownerAct)
				if replayErr != nil {
					return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: owner act lookup: %w", replayErr)
				}
				if replayed {
					if refuseErr := c.ownerActs.refuse(ErrOwnerActRefused, "mode write", computerID, "owner-act receipt "+ownerAct.ReceiptID+" was already used"); refuseErr != nil {
						return SelfDevelopmentMode{}, refuseErr
					}
					ownerAct = nil
				}
			} else if c.ownerActs.Enforcing() {
				return SelfDevelopmentMode{}, fmt.Errorf("%w: no verified owner-act receipt", ErrOwnerActRefused)
			}
		} else {
			ownerAct = nil
		}
	case ModeWriterCorpusdExpiry:
		ownerAct = nil
	case ModeWriterGuestConsumption:
		ownerAct = nil
		if !guestConsumptionAllowed(current, request) {
			return SelfDevelopmentMode{}, ErrSelfDevelopmentModeOwnerRequired
		}
	default:
		return SelfDevelopmentMode{}, ErrSelfDevelopmentModeOwnerRequired
	}
	if request.Mode == SelfDevelopmentModeStandingConsensus {
		// P4: a new authority starts fail-closed. Only the owner arms it,
		// with a verified owner act whatever CHOIR_OWNER_ACT_MODE says,
		// and only on a computer the proxy attested is test-tier.
		if err := standingArmRefusal(writer, computerID, ownerAct); err != nil {
			return SelfDevelopmentMode{}, err
		}
	}
	if current.Generation != request.ExpectedGeneration {
		return SelfDevelopmentMode{}, fmt.Errorf("%w: generation changed", ErrSelfDevelopmentModeConflict)
	}
	next, expiry, err := validateSelfDevelopmentModeTransition(current, request, c.now())
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	next.Generation = current.Generation + 1
	now := c.now().UTC().Truncate(time.Microsecond)
	fields := map[string]any{
		"computer_id": computerID, "old_mode": current.Mode, "new_mode": next.Mode,
		"old_generation": current.Generation, "committed_generation": next.Generation,
		"operation_id": next.OperationID, "base_event_head": next.ExpectedDesiredEventHead,
		"expected_desired_event_head":         next.ExpectedDesiredEventHead,
		"expected_effective_event_head":       next.ExpectedEffectiveEventHead,
		"expected_pending_transition_ref":     next.ExpectedPendingTransitionRef,
		"expected_desired_state_commitment":   next.ExpectedDesiredStateCommitment,
		"expected_effective_state_commitment": next.ExpectedEffectiveStateCommitment,
		"bundle_digest":                       next.BundleDigest, "expires_at": next.ExpiresAt,
		"policy_digest":                       next.PolicyDigest,
		"consensus_receipt_digest":            next.ConsensusReceiptDigest,
		"consumed_operation_id":               current.OperationID,
		"consumed_bundle_digest":              current.BundleDigest,
		"consumed_desired_event_head":         current.ExpectedDesiredEventHead,
		"consumed_effective_event_head":       current.ExpectedEffectiveEventHead,
		"consumed_pending_transition_ref":     current.ExpectedPendingTransitionRef,
		"consumed_expires_at":                 current.ExpiresAt,
		"consumed_desired_state_commitment":   current.ExpectedDesiredStateCommitment,
		"consumed_effective_state_commitment": current.ExpectedEffectiveStateCommitment,
		"consumed_policy_digest":              current.PolicyDigest,
		"consumed_consensus_receipt_digest":   current.ConsensusReceiptDigest,
		"idempotency_key":                     request.IdempotencyKey, "request_commitment": requestCommitment,
		"mode_writer": writer,
	}
	if ownerAct != nil {
		fields["owner_act_receipt_digest"] = ownerAct.Digest
		fields["owner_user_id"] = ownerAct.OwnerUserID
		fields["owner_auth_method"] = ownerAct.AuthMethod
	}
	if next.Mode == SelfDevelopmentModeStandingConsensus {
		fields["computer_tier"] = ComputerTierTest
	}
	receipt, err := computerevent.NewSignedReceipt("ModeReceipt", "corpusd", fields, []computerevent.SigningKey{c.signingKey}, now)
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		return SelfDevelopmentMode{}, err
	}
	next.Receipt = &receipt
	values := []any{next.Mode, next.Generation, nullableString(next.OperationID), nullableString(next.BundleDigest), nullableString(next.ExpectedDesiredEventHead), nullableString(next.ExpectedEffectiveEventHead), nullableString(next.ExpectedPendingTransitionRef), nullableString(next.ExpectedDesiredStateCommitment), nullableString(next.ExpectedEffectiveStateCommitment), expiry, request.IdempotencyKey, requestCommitment, string(receiptJSON), computerevent.DigestBytes(receiptJSON), now}
	if found {
		result, execErr := tx.ExecContext(ctx, `UPDATE computer_self_development_modes SET mode=?, generation=?, operation_id=?, bundle_digest=?, expected_desired_event_head=?, expected_effective_event_head=?, expected_pending_transition_ref=?, expected_desired_state_commitment=?, expected_effective_state_commitment=?, expires_at=?, last_idempotency_key=?, last_request_commitment=?, mode_receipt_json=?, mode_receipt_digest=?, updated_at=? WHERE computer_id=? AND generation=?`, append(values, computerID, current.Generation)...)
		if execErr != nil {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: update: %w", execErr)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return SelfDevelopmentMode{}, ErrSelfDevelopmentModeConflict
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_self_development_modes (computer_id, mode, generation, operation_id, bundle_digest, expected_desired_event_head, expected_effective_event_head, expected_pending_transition_ref, expected_desired_state_commitment, expected_effective_state_commitment, expires_at, last_idempotency_key, last_request_commitment, mode_receipt_json, mode_receipt_digest, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, append([]any{computerID}, values...)...)
		if err != nil {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: insert: %w", err)
		}
	}
	ownerActDigest := ""
	if ownerAct != nil {
		ownerActDigest = ownerAct.Digest
		if err := recordOwnerAct(ctx, tx, ownerAct, computerevent.DigestBytes(receiptJSON), now); err != nil {
			return SelfDevelopmentMode{}, err
		}
	}
	if err := recordModeReceiptIssued(ctx, tx, computerID, current, next, writer, ownerActDigest, receiptJSON, now); err != nil {
		return SelfDevelopmentMode{}, err
	}
	if err := tx.Commit(); err != nil {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: commit: %w", err)
	}
	return next, nil
}

func validateSelfDevelopmentModeTransition(current SelfDevelopmentMode, request SetSelfDevelopmentModeRequest, now time.Time) (SelfDevelopmentMode, any, error) {
	next := SelfDevelopmentMode{ComputerID: current.ComputerID, Mode: request.Mode}
	ordinaryBindings := []string{request.OperationID, request.BundleDigest, request.ExpectedDesiredEventHead, request.ExpectedEffectiveEventHead, request.ExpectedDesiredStateCommitment, request.ExpectedEffectiveStateCommitment, request.ExpiresAt}
	switch request.Mode {
	case SelfDevelopmentModeOff, SelfDevelopmentModeAuditOnly, SelfDevelopmentModeProposeOnly:
		if request.ExpectedPendingTransitionRef != nil {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
		}
		for _, value := range ordinaryBindings {
			if strings.TrimSpace(value) != "" {
				return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
			}
		}
		if strings.TrimSpace(request.PolicyDigest) != "" || strings.TrimSpace(request.ConsensusReceiptDigest) != "" {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
		}
		return next, nil, nil
	// accept_once is the legacy fail-closed owner-gated mode. The active effects
	// Definition replaces it atomically with policy-bound consensus modes; these
	// exact bindings remain required until that replacement is complete.
	case SelfDevelopmentModeAcceptOnce:
		if strings.TrimSpace(request.PolicyDigest) != "" || strings.TrimSpace(request.ConsensusReceiptDigest) != "" {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: accept_once does not bind consensus receipts")
		}
		if request.ExpectedPendingTransitionRef == nil || strings.TrimSpace(request.OperationID) == "" || !computerevent.IsSHA256(request.BundleDigest) || !computerevent.IsSHA256(request.ExpectedDesiredEventHead) || !computerevent.IsSHA256(request.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(request.ExpectedDesiredStateCommitment) || !computerevent.IsSHA256(request.ExpectedEffectiveStateCommitment) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: accept_once requires exact operation, bundle, heads, pending transition, and commitments")
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
		if err != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != request.ExpiresAt || !expiresAt.After(now.UTC()) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: accept_once requires a future canonical UTC expiry")
		}
		next.OperationID = request.OperationID
		next.BundleDigest = request.BundleDigest
		next.ExpectedDesiredEventHead = request.ExpectedDesiredEventHead
		next.ExpectedEffectiveEventHead = request.ExpectedEffectiveEventHead
		next.ExpectedPendingTransitionRef = strings.TrimSpace(*request.ExpectedPendingTransitionRef)
		next.ExpectedDesiredStateCommitment = request.ExpectedDesiredStateCommitment
		next.ExpectedEffectiveStateCommitment = request.ExpectedEffectiveStateCommitment
		next.ExpiresAt = request.ExpiresAt
		return next, expiresAt.UTC(), nil
	case SelfDevelopmentModeQualifiedConsensus:
		if request.ExpectedPendingTransitionRef == nil || strings.TrimSpace(request.OperationID) == "" || !computerevent.IsSHA256(request.BundleDigest) || !computerevent.IsSHA256(request.ExpectedDesiredEventHead) || !computerevent.IsSHA256(request.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(request.ExpectedDesiredStateCommitment) || !computerevent.IsSHA256(request.ExpectedEffectiveStateCommitment) || !computerevent.IsSHA256(request.PolicyDigest) || !computerevent.IsSHA256(request.ConsensusReceiptDigest) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: qualified_consensus requires exact operation, bundle, heads, pending transition, commitments, policy_digest, and consensus_receipt_digest")
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
		if err != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != request.ExpiresAt || !expiresAt.After(now.UTC()) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: qualified_consensus requires a future canonical UTC expiry")
		}
		next.OperationID = request.OperationID
		next.BundleDigest = request.BundleDigest
		next.ExpectedDesiredEventHead = request.ExpectedDesiredEventHead
		next.ExpectedEffectiveEventHead = request.ExpectedEffectiveEventHead
		next.ExpectedPendingTransitionRef = strings.TrimSpace(*request.ExpectedPendingTransitionRef)
		next.ExpectedDesiredStateCommitment = request.ExpectedDesiredStateCommitment
		next.ExpectedEffectiveStateCommitment = request.ExpectedEffectiveStateCommitment
		next.ExpiresAt = request.ExpiresAt
		next.PolicyDigest = request.PolicyDigest
		next.ConsensusReceiptDigest = request.ConsensusReceiptDigest
		return next, expiresAt.UTC(), nil
	case SelfDevelopmentModeStandingConsensus:
		// Bound to a policy and an expiry; the one-operation subject
		// binding lives in each ballot and decision, never in the mode.
		if request.ExpectedPendingTransitionRef != nil || strings.TrimSpace(request.ConsensusReceiptDigest) != "" {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: standing_consensus binds no operation or consensus receipt")
		}
		for _, value := range ordinaryBindings[:6] {
			if strings.TrimSpace(value) != "" {
				return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: standing_consensus binds no operation or consensus receipt")
			}
		}
		if !decisionpolicy.StandingPolicyAllowed(request.PolicyDigest) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: standing_consensus arms only %s", decisionpolicy.PolicyIDReversibleSelfDevV1)
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
		if err != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != request.ExpiresAt || !expiresAt.After(now.UTC()) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: standing_consensus requires a future canonical UTC expiry")
		}
		if expiresAt.Sub(now.UTC()) > StandingConsensusMaxWindow {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: standing_consensus expiry is more than %s ahead", StandingConsensusMaxWindow)
		}
		next.PolicyDigest = request.PolicyDigest
		next.ExpiresAt = request.ExpiresAt
		return next, expiresAt.UTC(), nil
	default:
		return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: unknown mode %q", request.Mode)
	}
}

// SelfDevelopmentModeRequestCommitment is the commitment a mode write is
// idempotent on and the subject an owner-act receipt binds: the proxy and
// corpusd compute it from this one function.
func SelfDevelopmentModeRequestCommitment(computerID string, request SetSelfDevelopmentModeRequest) (string, error) {
	request.Mode = strings.TrimSpace(request.Mode)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	return selfDevelopmentModeRequestCommitment(strings.TrimSpace(computerID), request)
}

func selfDevelopmentModeRequestCommitment(computerID string, request SetSelfDevelopmentModeRequest) (string, error) {
	body := struct {
		ComputerID string `json:"computer_id"`
		SetSelfDevelopmentModeRequest
	}{ComputerID: computerID, SetSelfDevelopmentModeRequest: request}
	canonical, err := computerevent.CanonicalJSON(body)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func readSelfDevelopmentMode(ctx context.Context, db queryRower, computerID string, forUpdate bool) (SelfDevelopmentMode, bool, error) {
	query := `SELECT mode, generation, COALESCE(operation_id, ''), COALESCE(bundle_digest, ''), COALESCE(expected_desired_event_head, ''), COALESCE(expected_effective_event_head, ''), COALESCE(expected_pending_transition_ref, ''), COALESCE(expected_desired_state_commitment, ''), COALESCE(expected_effective_state_commitment, ''), expires_at, mode_receipt_json FROM computer_self_development_modes WHERE computer_id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var mode SelfDevelopmentMode
	var expiresAt sql.NullTime
	var receiptJSON string
	err := db.QueryRowContext(ctx, query, computerID).Scan(&mode.Mode, &mode.Generation, &mode.OperationID, &mode.BundleDigest, &mode.ExpectedDesiredEventHead, &mode.ExpectedEffectiveEventHead, &mode.ExpectedPendingTransitionRef, &mode.ExpectedDesiredStateCommitment, &mode.ExpectedEffectiveStateCommitment, &expiresAt, &receiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return SelfDevelopmentMode{}, false, nil
	}
	if err != nil {
		return SelfDevelopmentMode{}, false, fmt.Errorf("self-development mode: read: %w", err)
	}
	mode.ComputerID = computerID
	if expiresAt.Valid {
		mode.ExpiresAt = expiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal([]byte(receiptJSON), &receipt); err != nil {
		return SelfDevelopmentMode{}, false, fmt.Errorf("self-development mode: decode receipt: %w", err)
	}
	mode.Receipt = &receipt
	hydrateSelfDevelopmentModeBindings(&mode)
	return mode, true, nil
}

func hydrateSelfDevelopmentModeBindings(mode *SelfDevelopmentMode) {
	if mode == nil || mode.Receipt == nil {
		return
	}
	field := func(name string) string {
		value, _ := mode.Receipt.KindFields[name].(string)
		return value
	}
	if mode.PolicyDigest == "" {
		mode.PolicyDigest = field("policy_digest")
	}
	if mode.ConsensusReceiptDigest == "" {
		mode.ConsensusReceiptDigest = field("consensus_receipt_digest")
	}
}

func (h *Handler) HandleSelfDevelopmentMode(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.selfDevelopmentModes == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development mode authority unavailable"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return
	}
	internalCaller := trustedInternalCaller(r)
	switch r.Method {
	case http.MethodGet:
		if !internalCaller && (h.eventAuth == nil || h.eventAuth.Authorize(r, computerID, "event:read") != nil) {
			writeJSON(w, http.StatusForbidden, apiError{Error: "self-development mode capability refused"})
			return
		}
		mode, err := h.selfDevelopmentModes.Get(r.Context(), computerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read self-development mode"})
			return
		}
		writeJSON(w, http.StatusOK, mode)
	case http.MethodPost:
		// A host-internal caller is the proxy carrying an authenticated
		// owner act. A guest capability may only consume the armed grant.
		var write func(context.Context, string, SetSelfDevelopmentModeRequest) (SelfDevelopmentMode, error)
		if !internalCaller {
			if h.eventAuth == nil || h.eventAuth.Authorize(r, computerID, "event:append") != nil {
				writeJSON(w, http.StatusForbidden, apiError{Error: "self-development mode mutation capability refused"})
				return
			}
			write = h.selfDevelopmentModes.ConsumeAsGuest
		}
		var request SetSelfDevelopmentModeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid mode request"})
			return
		}
		if write == nil {
			// S1: the owner's write must carry the proxy's owner-act
			// receipt. In observe a missing or invalid one is logged.
			var ownerAct *owneract.Verified
			if authority := h.selfDevelopmentModes.ownerActs; authority != nil {
				verified, verifyErr := authority.verifyModeWrite(r, computerID, request)
				if verifyErr != nil {
					if refuseErr := authority.refuse(ErrOwnerActRefused, "mode write", computerID, verifyErr.Error()); refuseErr != nil {
						writeJSON(w, http.StatusForbidden, apiError{Error: refuseErr.Error()})
						return
					}
				} else {
					ownerAct = verified
				}
			}
			write = func(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest) (SelfDevelopmentMode, error) {
				return h.selfDevelopmentModes.SetAsOwner(ctx, computerID, request, ownerAct)
			}
		}
		mode, err := write(r.Context(), computerID, request)
		if err != nil {
			if errors.Is(err, ErrSelfDevelopmentModeOwnerRequired) || errors.Is(err, ErrOwnerActRefused) {
				writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
				return
			}
			if errors.Is(err, ErrSelfDevelopmentModeConflict) {
				writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
				return
			}
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, mode)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
