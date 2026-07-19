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
)

const (
	SelfDevelopmentModeOff         = "off"
	SelfDevelopmentModeAuditOnly   = "audit_only"
	SelfDevelopmentModeProposeOnly = "propose_only"
	SelfDevelopmentModeAcceptOnce  = "accept_once"
)

var ErrSelfDevelopmentModeConflict = errors.New("self-development mode CAS conflict")

type SelfDevelopmentMode struct {
	ComputerID                       string                 `json:"computer_id"`
	Mode                             string                 `json:"mode"`
	Generation                       uint64                 `json:"generation"`
	OperationID                      string                 `json:"operation_id,omitempty"`
	BundleDigest                     string                 `json:"bundle_digest,omitempty"`
	ExpectedDesiredEventHead         string                 `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead       string                 `json:"expected_effective_event_head,omitempty"`
	ExpectedDesiredStateCommitment   string                 `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string                 `json:"expected_effective_state_commitment,omitempty"`
	ExpiresAt                        string                 `json:"expires_at,omitempty"`
	Receipt                          *computerevent.Receipt `json:"receipt,omitempty"`
}

type SetSelfDevelopmentModeRequest struct {
	Mode                             string `json:"mode"`
	ExpectedGeneration               uint64 `json:"expected_generation"`
	OperationID                      string `json:"operation_id,omitempty"`
	BundleDigest                     string `json:"bundle_digest,omitempty"`
	ExpectedDesiredEventHead         string `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead       string `json:"expected_effective_event_head,omitempty"`
	ExpectedDesiredStateCommitment   string `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string `json:"expected_effective_state_commitment,omitempty"`
	ExpiresAt                        string `json:"expires_at,omitempty"`
	IdempotencyKey                   string `json:"idempotency_key"`
}

type SelfDevelopmentModeCAS struct {
	locks      [64]sync.Mutex
	store      *Store
	signingKey computerevent.SigningKey
	now        func() time.Time
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
	if mode.Mode != SelfDevelopmentModeAcceptOnce {
		return mode, nil
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, mode.ExpiresAt)
	if err != nil {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: invalid durable expiry: %w", err)
	}
	if c.now().Before(expiresAt) {
		return mode, nil
	}
	expired, err := c.Set(ctx, computerID, SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeOff, ExpectedGeneration: mode.Generation,
		IdempotencyKey: fmt.Sprintf("selfdev-mode-expiry-v1:%s:%d", computerID, mode.Generation),
	})
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
	if current.Mode == SelfDevelopmentModeAcceptOnce {
		currentExpiry, parseErr := time.Parse(time.RFC3339Nano, current.ExpiresAt)
		if parseErr != nil || !c.now().Before(currentExpiry) {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: expired accept_once reconciliation conflicted")
		}
	}
	return current, nil
}

func (c *SelfDevelopmentModeCAS) Set(ctx context.Context, computerID string, request SetSelfDevelopmentModeRequest) (SelfDevelopmentMode, error) {
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
		"expected_pending_transition_ref":     "",
		"expected_desired_state_commitment":   next.ExpectedDesiredStateCommitment,
		"expected_effective_state_commitment": next.ExpectedEffectiveStateCommitment,
		"bundle_digest":                       next.BundleDigest, "expires_at": next.ExpiresAt,
		"consumed_operation_id":               current.OperationID,
		"consumed_bundle_digest":              current.BundleDigest,
		"consumed_desired_event_head":         current.ExpectedDesiredEventHead,
		"consumed_effective_event_head":       current.ExpectedEffectiveEventHead,
		"consumed_desired_state_commitment":   current.ExpectedDesiredStateCommitment,
		"consumed_effective_state_commitment": current.ExpectedEffectiveStateCommitment,
		"idempotency_key":                     request.IdempotencyKey, "request_commitment": requestCommitment,
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
	values := []any{next.Mode, next.Generation, nullableString(next.OperationID), nullableString(next.BundleDigest), nullableString(next.ExpectedDesiredEventHead), nullableString(next.ExpectedEffectiveEventHead), nullableString(next.ExpectedDesiredStateCommitment), nullableString(next.ExpectedEffectiveStateCommitment), expiry, request.IdempotencyKey, requestCommitment, string(receiptJSON), computerevent.DigestBytes(receiptJSON), now}
	if found {
		result, execErr := tx.ExecContext(ctx, `UPDATE computer_self_development_modes SET mode=?, generation=?, operation_id=?, bundle_digest=?, expected_desired_event_head=?, expected_effective_event_head=?, expected_desired_state_commitment=?, expected_effective_state_commitment=?, expires_at=?, last_idempotency_key=?, last_request_commitment=?, mode_receipt_json=?, mode_receipt_digest=?, updated_at=? WHERE computer_id=? AND generation=?`, append(values, computerID, current.Generation)...)
		if execErr != nil {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: update: %w", execErr)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return SelfDevelopmentMode{}, ErrSelfDevelopmentModeConflict
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_self_development_modes (computer_id, mode, generation, operation_id, bundle_digest, expected_desired_event_head, expected_effective_event_head, expected_desired_state_commitment, expected_effective_state_commitment, expires_at, last_idempotency_key, last_request_commitment, mode_receipt_json, mode_receipt_digest, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, append([]any{computerID}, values...)...)
		if err != nil {
			return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return SelfDevelopmentMode{}, fmt.Errorf("self-development mode: commit: %w", err)
	}
	if err := c.store.commitDolt(ctx, "set self-development mode for "+computerID); err != nil {
		return SelfDevelopmentMode{}, err
	}
	return next, nil
}

func validateSelfDevelopmentModeTransition(current SelfDevelopmentMode, request SetSelfDevelopmentModeRequest, now time.Time) (SelfDevelopmentMode, any, error) {
	next := SelfDevelopmentMode{ComputerID: current.ComputerID, Mode: request.Mode}
	ordinaryBindings := []string{request.OperationID, request.BundleDigest, request.ExpectedDesiredEventHead, request.ExpectedEffectiveEventHead, request.ExpectedDesiredStateCommitment, request.ExpectedEffectiveStateCommitment, request.ExpiresAt}
	switch request.Mode {
	case SelfDevelopmentModeOff, SelfDevelopmentModeAuditOnly, SelfDevelopmentModeProposeOnly:
		for _, value := range ordinaryBindings {
			if strings.TrimSpace(value) != "" {
				return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: bindings are forbidden for %s", request.Mode)
			}
		}
		return next, nil, nil
	case SelfDevelopmentModeAcceptOnce:
		if strings.TrimSpace(request.OperationID) == "" || !computerevent.IsSHA256(request.BundleDigest) || !computerevent.IsSHA256(request.ExpectedDesiredEventHead) || !computerevent.IsSHA256(request.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(request.ExpectedDesiredStateCommitment) || !computerevent.IsSHA256(request.ExpectedEffectiveStateCommitment) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: accept_once requires exact operation, bundle, heads, and commitments")
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
		if err != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != request.ExpiresAt || !expiresAt.After(now.UTC()) {
			return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: accept_once requires a future canonical UTC expiry")
		}
		next.OperationID = request.OperationID
		next.BundleDigest = request.BundleDigest
		next.ExpectedDesiredEventHead = request.ExpectedDesiredEventHead
		next.ExpectedEffectiveEventHead = request.ExpectedEffectiveEventHead
		next.ExpectedDesiredStateCommitment = request.ExpectedDesiredStateCommitment
		next.ExpectedEffectiveStateCommitment = request.ExpectedEffectiveStateCommitment
		next.ExpiresAt = request.ExpiresAt
		return next, expiresAt.UTC(), nil
	default:
		return SelfDevelopmentMode{}, nil, fmt.Errorf("self-development mode: unknown mode %q", request.Mode)
	}
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
	query := `SELECT mode, generation, COALESCE(operation_id, ''), COALESCE(bundle_digest, ''), COALESCE(expected_desired_event_head, ''), COALESCE(expected_effective_event_head, ''), COALESCE(expected_desired_state_commitment, ''), COALESCE(expected_effective_state_commitment, ''), expires_at, mode_receipt_json FROM computer_self_development_modes WHERE computer_id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var mode SelfDevelopmentMode
	var expiresAt sql.NullTime
	var receiptJSON string
	err := db.QueryRowContext(ctx, query, computerID).Scan(&mode.Mode, &mode.Generation, &mode.OperationID, &mode.BundleDigest, &mode.ExpectedDesiredEventHead, &mode.ExpectedEffectiveEventHead, &mode.ExpectedDesiredStateCommitment, &mode.ExpectedEffectiveStateCommitment, &expiresAt, &receiptJSON)
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
	return mode, true, nil
}

func (h *Handler) HandleSelfDevelopmentMode(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.selfDevelopmentModes == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development mode authority unavailable"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		mode, err := h.selfDevelopmentModes.Get(r.Context(), computerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read self-development mode"})
			return
		}
		writeJSON(w, http.StatusOK, mode)
	case http.MethodPost:
		var request SetSelfDevelopmentModeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid mode request"})
			return
		}
		mode, err := h.selfDevelopmentModes.Set(r.Context(), computerID, request)
		if err != nil {
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
