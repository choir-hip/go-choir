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
	"github.com/yusefmosiah/go-choir/internal/owneract"
)

// The two writable self-development modes (docs/design/selfdev-apply-and-revert-2026-11-06.md
// 3.1). off: this computer does no self-development. apply: propose, verify,
// host build, decide by consensus, apply. A computer with no row is apply.
const (
	SelfDevelopmentModeOff   = "off"
	SelfDevelopmentModeApply = "apply"
)

// Legacy stored mode values. They are never written again; rows that carry
// them stay as they are and read through CanonicalSelfDevelopmentMode.
const (
	legacyModeAuditOnly          = "audit_only"
	legacyModeProposeOnly        = "propose_only"
	legacyModeAcceptOnce         = "accept_once"
	legacyModeQualifiedConsensus = "qualified_consensus"
	legacyModeStandingConsensus  = "standing_consensus"
)

// CanonicalSelfDevelopmentMode is the one normaliser every reader of a mode
// goes through: off and audit_only read off; propose_only, accept_once,
// qualified_consensus, standing_consensus and no row (the empty string) read
// apply. An unknown stored value fails closed to off.
func CanonicalSelfDevelopmentMode(stored string) string {
	switch strings.TrimSpace(stored) {
	case SelfDevelopmentModeApply, legacyModeProposeOnly, legacyModeAcceptOnce, legacyModeQualifiedConsensus, legacyModeStandingConsensus, "":
		return SelfDevelopmentModeApply
	case SelfDevelopmentModeOff, legacyModeAuditOnly:
		return SelfDevelopmentModeOff
	default:
		return SelfDevelopmentModeOff
	}
}

// DefaultApplyModeMarker is the content whose digest stands in as the mode
// receipt for a computer with no mode row: the default is apply, generation 0.
const DefaultApplyModeMarker = "selfdev-mode:default-apply:0"

// DefaultApplyModeDigest is sha256(DefaultApplyModeMarker), 64 hex: the
// digest of input artifact 0 and the decisions table's standing_digest for a
// computer with no mode row.
func DefaultApplyModeDigest() string {
	return computerevent.DigestBytes([]byte(DefaultApplyModeMarker))
}

var ErrSelfDevelopmentModeConflict = errors.New("self-development mode CAS conflict")

// ErrSelfDevelopmentModeOwnerRequired refuses a mode write that is not an
// owner act. Only the owner sets the mode: the proxy reaches corpusd as a
// host-internal caller after it has authenticated the owner's session or an
// owner API key scoped computer:self_development:mode. The guest can write no
// mode at all (docs/archive/problems/guest-can-arm-own-selfdev-mode-2026-10-15.md).
var ErrSelfDevelopmentModeOwnerRequired = errors.New("self-development mode: owner authority required")

// Mode writers, recorded as mode_writer in every new ModeReceipt. Receipts
// signed before this field existed carry none; they were all written before
// the owner-only rule and are not re-judged. The retired writers
// "guest-consumption" and "corpusd-expiry" still name historic log rows.
const (
	ModeWriterHostInternal = "host-internal"
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

// Get reads the computer's mode through the canonical normaliser: a computer
// with no row is apply at generation 0 with no receipt; a legacy row reads its
// canonical mode (the row is never rewritten).
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
		return SelfDevelopmentMode{ComputerID: computerID, Mode: SelfDevelopmentModeApply}, nil
	}
	return mode, nil
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
		current = SelfDevelopmentMode{ComputerID: computerID, Mode: SelfDevelopmentModeApply}
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
	default:
		return SelfDevelopmentMode{}, ErrSelfDevelopmentModeOwnerRequired
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

// validateSelfDevelopmentModeTransition admits the two writable modes, off
// and apply, with no bindings. Every retired value is refused.
func validateSelfDevelopmentModeTransition(current SelfDevelopmentMode, request SetSelfDevelopmentModeRequest, _ time.Time) (SelfDevelopmentMode, any, error) {
	next := SelfDevelopmentMode{ComputerID: current.ComputerID, Mode: request.Mode}
	switch request.Mode {
	case SelfDevelopmentModeOff, SelfDevelopmentModeApply:
		if request.ExpectedPendingTransitionRef != nil {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
		}
		for _, value := range []string{request.OperationID, request.BundleDigest, request.ExpectedDesiredEventHead, request.ExpectedEffectiveEventHead, request.ExpectedDesiredStateCommitment, request.ExpectedEffectiveStateCommitment, request.ExpiresAt, request.PolicyDigest, request.ConsensusReceiptDigest} {
			if strings.TrimSpace(value) != "" {
				return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
			}
		}
		return next, nil, nil
	default:
		return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: %q is not a writable mode (off or apply)", request.Mode)
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
	// Every reader sees the canonical mode; the stored column is never rewritten.
	mode.Mode = CanonicalSelfDevelopmentMode(mode.Mode)
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
		// owner act. A guest capability writes no mode.
		if !internalCaller {
			writeJSON(w, http.StatusForbidden, apiError{Error: ErrSelfDevelopmentModeOwnerRequired.Error()})
			return
		}
		var request SetSelfDevelopmentModeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid mode request"})
			return
		}
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
		mode, err := h.selfDevelopmentModes.SetAsOwner(r.Context(), computerID, request, ownerAct)
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
