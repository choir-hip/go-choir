package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// Realization key delivery (station SH slice 3, operational invariant O21):
// a realization of an existing computer receives the computer's privacy key
// from custodian escrow, sealed to a recipient key the realization generated.
// Design and review: docs/definitions/choir-appdev-sh-state-homes-2026-10-08.md
// (slice 3 revision) and docs/evidence/sh-panel-review-2026-10-08.md.
//
// Binding: the caller presents the capability minted by exchanging its own
// credential envelope; that capability's nonce is the envelope nonce, which
// only corpusd can derive from the issuance's request commitment. A
// capability from any other issuance (or a renewal) cannot claim delivery.
// The issuance must be the computer's current one. Re-delivery with the same
// proof is allowed (a guest that crashed before persisting the key). Every
// delivery is recorded in the key-escrow transparency log before unwrap.

type realizationKeyDeliveryRequest struct {
	ComputerID         string `json:"computer_id"`
	RequestCommitment  string `json:"request_commitment"`
	RecipientPublicKey string `json:"recipient_public_key"`
}

type realizationKeyDeliveryResponse struct {
	WrappedKey string `json:"wrapped_key"`
	KeyDigest  string `json:"key_digest"`
}

func (h *Handler) HandleRealizationKeyDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil || h.service.signingKey == nil || h.keyEscrow == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key delivery unavailable"})
		return
	}
	var input realizationKeyDeliveryRequest
	if !decodeKeyEscrowJSON(r, &input) || strings.TrimSpace(input.ComputerID) == "" || strings.TrimSpace(input.RequestCommitment) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid delivery request"})
		return
	}
	var recipient keyescrow.PublicKey
	decoded, err := base64.RawStdEncoding.DecodeString(input.RecipientPublicKey)
	if err != nil || len(decoded) != len(recipient) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid recipient key"})
		return
	}
	copy(recipient[:], decoded)

	if h.authorizeKeyEscrowGuest(r, input.ComputerID, "event:append") != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	capability, err := presentedCapability(r)
	expectedNonce := credentialPRF(h.service.signingKey.Private.Seed(), "nonce", input.RequestCommitment)
	if err != nil || !hmac.Equal([]byte(capability.Nonce), []byte(expectedNonce)) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "capability is not bound to this issuance"})
		return
	}

	ctx := r.Context()
	current, err := h.service.store.currentCredentialIssuance(ctx, input.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read credential issuance"})
		return
	}
	if current != input.RequestCommitment {
		writeJSON(w, http.StatusConflict, apiError{Error: "credential issuance superseded"})
		return
	}
	head, err := readComputerEventHead(ctx, h.service.store.db, input.ComputerID, false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read canonical head"})
		return
	}
	if head == nil {
		// Before genesis the realization creates its own key.
		writeJSON(w, http.StatusConflict, apiError{Error: "computer has no canonical chain"})
		return
	}
	wrappedJSON, keyDigest, err := h.service.store.GetKeyEscrow(ctx, input.ComputerID, keyescrow.ProtectorCustodian)
	if errors.Is(err, ErrKeyEscrowNotFound) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "escrow record not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get escrow record"})
		return
	}
	record, err := keyescrow.ParseWrappedKey(wrappedJSON)
	if err != nil || record.KeyDigest != keyDigest {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "invalid escrow record"})
		return
	}
	recipientDigest := sha256.Sum256(recipient[:])
	payload, err := json.Marshal(map[string]any{
		"type": "realization_delivery", "computer_id": input.ComputerID,
		"request_commitment": input.RequestCommitment, "key_digest": keyDigest,
		"recipient_key_digest": fmt.Sprintf("%x", recipientDigest),
		"head_sequence":        head.Sequence, "delivered_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record escrow transparency"})
		return
	}
	if _, _, err := h.service.store.AppendKeyEscrowTransparency(ctx, payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record escrow transparency"})
		return
	}
	dek, err := keyescrow.OpenDEK(h.keyEscrow.privateKey, record, input.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open escrow record"})
		return
	}
	defer clear(dek)
	sealed, err := keyescrow.SealDEK(recipient, input.ComputerID, dek)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to seal key"})
		return
	}
	sealedJSON, err := json.Marshal(sealed)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to seal key"})
		return
	}
	writeJSON(w, http.StatusOK, realizationKeyDeliveryResponse{WrappedKey: string(sealedJSON), KeyDigest: keyDigest})
}

// presentedCapability decodes the bearer capability payload. Call only after
// the request was authorized: the signature is not re-verified here.
func presentedCapability(r *http.Request) (ComputerCapability, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Split(strings.TrimPrefix(header, "Bearer "), ".")
	if !strings.HasPrefix(header, "Bearer ") || len(parts) != 2 {
		return ComputerCapability{}, fmt.Errorf("computer capability: malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ComputerCapability{}, fmt.Errorf("computer capability: malformed payload")
	}
	var capability ComputerCapability
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&capability); err != nil {
		return ComputerCapability{}, fmt.Errorf("computer capability: invalid payload")
	}
	return capability, nil
}

// currentCredentialIssuance returns the request commitment of the computer's
// most recent credential envelope issuance, or "" when none exists.
func (s *Store) currentCredentialIssuance(ctx context.Context, computerID string) (string, error) {
	var commitment string
	err := s.db.QueryRowContext(ctx, `SELECT request_commitment FROM computer_lifecycle_receipts WHERE computer_id=? AND action='credential_envelope_issued' ORDER BY completed_at DESC, receipt_id DESC LIMIT 1`, computerID).Scan(&commitment)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return commitment, err
}
