package platform

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Verifier-key preflight and the owner's re-pin
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
//
// The preflight answers, read only, whether the checkpoint authority would
// accept a verifier certificate signed by the presented key: the same pin
// lookup Publish makes (checkpoints.go, the verifier key block). A guest
// asks it before an apply spends an attempt and reports it on /health.
//
// The re-pin replaces the active pin with the key the guest presents. Trust
// argument: the first-use pin (checkpoints.go) already pins whatever key the
// guest's verifier presents, because the verifier key lives on the guest
// data volume, which the host cannot observe; a wrong key there is at most
// as bad as the verifier socket itself being compromised. A re-pin is the
// same binding with more authority behind it: it runs only for a
// host-internal caller carrying an owner-act receipt the proxy signed with
// its host-owner-act key, after it authenticated the computer's owner
// (owner:acts), binding this computer, the new key, the key it replaces
// and the owner's reason. A guest reaches corpusd directly, but it is never
// a trusted internal caller and cannot mint that receipt, so it cannot
// re-pin itself. The receipt is required in every rollout mode: observe
// does not apply to a key substitution.

const (
	verifierKeyPreflightRoute = "/internal/computers/verifier-key/preflight"
	verifierKeyRepinRoute     = "/internal/computers/verifier-key/repin"
)

var (
	errVerifierRepinConflict = errors.New("verifier re-pin conflict")
	errVerifierRepinInvalid  = errors.New("verifier re-pin invalid")
)

// VerifierKeyPreflight reports the pin state of a presented verifier key.
func (a *CheckpointAuthority) VerifierKeyPreflight(ctx context.Context, request selfdevprotocol.VerifierKeyPreflightRequest) (selfdevprotocol.VerifierKeyPin, error) {
	computerID := strings.TrimSpace(request.ComputerID)
	pin := selfdevprotocol.VerifierKeyPin{ComputerID: computerID, PresentedKeyID: strings.TrimSpace(request.KeyID)}
	if a == nil || a.cas == nil || a.cas.store == nil || computerID == "" {
		return pin, fmt.Errorf("verifier key preflight: computer_id is required")
	}
	presented, keyErr := selfdevprotocol.DecodeVerifierKey(request.KeyID, request.PublicKey)
	var pinnedKeyID string
	var pinnedKey []byte
	var pinnedAt sql.NullTime
	err := a.cas.store.db.QueryRowContext(ctx, `SELECT key_id,public_key,activation_time FROM control_key_history WHERE signer_domain='verifier-control' AND computer_id=? AND status='active'`, computerID).Scan(&pinnedKeyID, &pinnedKey, &pinnedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		var genesisSeq uint64
		genErr := a.cas.store.db.QueryRowContext(ctx, `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_kind=? ORDER BY sequence LIMIT 1`, computerID, string(computerevent.EventGenesisImported)).Scan(&genesisSeq)
		switch {
		case errors.Is(genErr, sql.ErrNoRows):
			pin.State = selfdevprotocol.VerifierKeyUnpinnedNoGenesis
		case genErr != nil:
			return pin, genErr
		default:
			pin.State = selfdevprotocol.VerifierKeyUnpinned
		}
	case err != nil:
		return pin, err
	default:
		pin.PinnedKeyID = pinnedKeyID
		if pinnedAt.Valid {
			pin.PinnedAt = pinnedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if keyErr == nil && pinnedKeyID == pin.PresentedKeyID && ed25519.PublicKey(pinnedKey).Equal(presented) {
			pin.State = selfdevprotocol.VerifierKeyMatch
		} else {
			pin.State = selfdevprotocol.VerifierKeyMismatch
		}
	}
	if keyErr != nil {
		pin.State = selfdevprotocol.VerifierKeyInvalid
	}
	return pin, nil
}

// RepinVerifierKey replaces a computer's active verifier pin with the
// presented key under a verified owner act. The caller verified the receipt;
// the act is recorded here, single use, in the same transaction.
func (a *CheckpointAuthority) RepinVerifierKey(ctx context.Context, request selfdevprotocol.VerifierKeyRepinRequest, verified *owneract.Verified) (selfdevprotocol.VerifierKeyRepinResponse, error) {
	var response selfdevprotocol.VerifierKeyRepinResponse
	computerID := strings.TrimSpace(request.ComputerID)
	reason := strings.TrimSpace(request.Reason)
	keyID := strings.TrimSpace(request.KeyID)
	if a == nil || a.cas == nil || verified == nil || computerID == "" || reason == "" || strings.TrimSpace(request.ReplacesKeyID) == "" {
		return response, fmt.Errorf("%w: computer_id, key_id, replaces_key_id and reason are required", errVerifierRepinInvalid)
	}
	presented, err := selfdevprotocol.DecodeVerifierKey(keyID, request.PublicKey)
	if err != nil {
		return response, fmt.Errorf("%w: %v", errVerifierRepinInvalid, err)
	}
	now := a.now()
	tx, err := a.cas.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return response, err
	}
	defer tx.Rollback()
	if replayed, replayErr := ownerActReplayed(ctx, tx, verified); replayErr != nil {
		return response, replayErr
	} else if replayed {
		return response, fmt.Errorf("%w: owner act receipt already used", ErrOwnerActRefused)
	}
	var activeKeyID string
	var activeKey []byte
	var activeReceiptJSON string
	err = tx.QueryRowContext(ctx, `SELECT key_id,public_key,authorizing_receipt_json FROM control_key_history WHERE signer_domain='verifier-control' AND computer_id=? AND status='active'`, computerID).Scan(&activeKeyID, &activeKey, &activeReceiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return response, fmt.Errorf("%w: no verifier key is pinned for %s; the next verifier checkpoint pins the presented key on first use", errVerifierRepinConflict, computerID)
	}
	if err != nil {
		return response, err
	}
	if activeKeyID == keyID && ed25519.PublicKey(activeKey).Equal(presented) {
		var receipt computerevent.Receipt
		_ = json.Unmarshal([]byte(activeReceiptJSON), &receipt)
		if err := recordOwnerAct(ctx, tx, verified, computerevent.ZeroHead, now); err != nil {
			return response, err
		}
		if err := tx.Commit(); err != nil {
			return response, err
		}
		response.Status = selfdevprotocol.VerifierKeyAlreadyPinned
		response.Pin = selfdevprotocol.VerifierKeyPin{ComputerID: computerID, State: selfdevprotocol.VerifierKeyMatch, PresentedKeyID: keyID, PinnedKeyID: keyID}
		response.PinReceipt = receipt
		return response, nil
	}
	if activeKeyID != strings.TrimSpace(request.ReplacesKeyID) {
		return response, fmt.Errorf("%w: the active pin is %s, not %s", errVerifierRepinConflict, activeKeyID, strings.TrimSpace(request.ReplacesKeyID))
	}
	var prior int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM control_key_history WHERE signer_domain='verifier-control' AND computer_id=? AND key_id=?`, computerID, keyID).Scan(&prior); err != nil {
		return response, err
	}
	if prior > 0 {
		return response, fmt.Errorf("%w: %s was pinned for %s before and revoked; a revoked key is never pinned again", errVerifierRepinConflict, keyID, computerID)
	}
	var head sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(sequence) FROM computer_event_append_receipts WHERE computer_id=?`, computerID).Scan(&head); err != nil {
		return response, err
	}
	cutover := uint64(1)
	if head.Valid && head.Int64 > 0 {
		cutover = uint64(head.Int64) + 1
	}
	pinReceipt, err := computerevent.NewSignedReceipt(selfdevprotocol.VerifierKeyPinnedReceiptKind, "corpusd", map[string]any{
		"computer_id": computerID, "key_id": keyID, "public_key": strings.TrimSpace(request.PublicKey),
		"repair": selfdevprotocol.VerifierKeyRepinRepair, "reason": reason, "replaces_key_id": activeKeyID,
		"activation_sequence": fmt.Sprint(cutover), "owner_act_receipt_digest": verified.Digest, "owner_user_id": verified.OwnerUserID,
	}, []computerevent.SigningKey{a.cas.signingKey}, now)
	if err != nil {
		return response, err
	}
	pinReceiptJSON, err := pinReceipt.CanonicalBytes()
	if err != nil {
		return response, err
	}
	pinReceiptDigest := computerevent.DigestBytes(pinReceiptJSON)
	result, err := tx.ExecContext(ctx, `UPDATE control_key_history SET status='revoked', first_invalid_sequence=?, first_invalid_time=?, replacement_key_id=? WHERE signer_domain='verifier-control' AND computer_id=? AND key_id=? AND status='active'`,
		cutover, now, keyID, computerID, activeKeyID)
	if err != nil {
		return response, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return response, fmt.Errorf("%w: the active pin changed", errVerifierRepinConflict)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO control_key_history (signer_domain,computer_id,key_id,public_key,status,activation_sequence,activation_time,first_invalid_sequence,first_invalid_time,replacement_key_id,authorizing_receipt_json,authorizing_receipt_digest,inserted_at) VALUES ('verifier-control',?,?,?,'active',?,?,NULL,NULL,NULL,?,?,?)`,
		computerID, keyID, []byte(presented), cutover, now, string(pinReceiptJSON), pinReceiptDigest, now); err != nil {
		return response, err
	}
	if err := recordOwnerAct(ctx, tx, verified, pinReceiptDigest, now); err != nil {
		return response, err
	}
	if err := tx.Commit(); err != nil {
		return response, err
	}
	log.Printf("verifier key: owner %s re-pinned %s: %s replaces %s from sequence %d (reason: %s)", verified.OwnerUserID, computerID, keyID, activeKeyID, cutover, reason)
	response.Status = selfdevprotocol.VerifierKeyRepinned
	response.Pin = selfdevprotocol.VerifierKeyPin{ComputerID: computerID, State: selfdevprotocol.VerifierKeyMatch, PresentedKeyID: keyID, PinnedKeyID: keyID, PinnedAt: now.UTC().Format(time.RFC3339Nano)}
	response.PinReceipt = pinReceipt
	return response, nil
}

// HandleVerifierKeyPreflight answers POST verifier-key/preflight: a
// host-internal caller, or the computer's own guest with its event
// capability. Read only.
func (h *Handler) HandleVerifierKeyPreflight(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.checkpointAuthority == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint authority unavailable"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var request selfdevprotocol.VerifierKeyPreflightRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.ComputerID) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid verifier key preflight"})
		return
	}
	if !trustedInternalCaller(r) && !h.authorizeComputerEvent(w, r, strings.TrimSpace(request.ComputerID), "event:read") {
		return
	}
	pin, err := h.checkpointAuthority.VerifierKeyPreflight(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pin)
}

// HandleVerifierKeyRepin is the owner's re-pin: host-internal only, with an
// owner-act receipt (owneract.ActVerifierRepin) binding the request.
func (h *Handler) HandleVerifierKeyRepin(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.checkpointAuthority == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint authority unavailable"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var request selfdevprotocol.VerifierKeyRepinRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.ComputerID) == "" || strings.TrimSpace(request.OwnerID) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid verifier re-pin"})
		return
	}
	verified, err := h.verifyVerifierRepinOwnerAct(r, request)
	if err != nil {
		log.Printf("verifier key: refused re-pin of %s: %v", request.ComputerID, err)
		writeJSON(w, http.StatusForbidden, apiError{Error: fmt.Sprintf("%v: %v", ErrOwnerActRefused, err)})
		return
	}
	response, err := h.checkpointAuthority.RepinVerifierKey(r.Context(), request, verified)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, response)
	case errors.Is(err, ErrOwnerActRefused):
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
	case errors.Is(err, errVerifierRepinConflict):
		writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
	case errors.Is(err, errVerifierRepinInvalid):
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
	}
}

// verifyVerifierRepinOwnerAct verifies the receipt in every mode: a key
// substitution is never admitted on observe.
func (h *Handler) verifyVerifierRepinOwnerAct(r *http.Request, request selfdevprotocol.VerifierKeyRepinRequest) (*owneract.Verified, error) {
	authority := h.packageOwnerActs
	if authority == nil || authority.verifier == nil {
		return nil, fmt.Errorf("no owner-act authority is configured")
	}
	receipt, err := owneract.Decode(r.Header.Get(owneract.Header))
	if err != nil {
		return nil, err
	}
	verified, err := authority.verifier.Verify(receipt, strings.TrimSpace(request.ComputerID), owneract.ActVerifierRepin, authority.now())
	if err != nil {
		return nil, err
	}
	if verified.OwnerUserID != strings.TrimSpace(request.OwnerID) {
		return nil, fmt.Errorf("owner act: the receipt does not bind this owner")
	}
	for key, want := range selfdevprotocol.VerifierKeyRepinSubject(request) {
		if verified.SubjectString(key) != want {
			return nil, fmt.Errorf("owner act: the receipt does not bind this %s", key)
		}
	}
	return &verified, nil
}
