package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"golang.org/x/crypto/curve25519"
)

type keyEscrowRuntime struct {
	privateKey keyescrow.PrivateKey
	publicKey  keyescrow.PublicKey
	operators  map[string][sha256.Size]byte
	gateOpen   bool
}

type keyEscrowPutRequest struct {
	ComputerID string `json:"computer_id"`
	Protector  string `json:"protector"`
	WrappedKey string `json:"wrapped_key"`
	KeyDigest  string `json:"key_digest"`
}

type keyUnwrapRequestInput struct {
	ComputerID     string `json:"computer_id"`
	RequestedBy    string `json:"requested_by"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

type keyUnwrapApprovalInput struct {
	Approver string `json:"approver"`
}

func (h *Handler) ConfigureKeyEscrow(privateKey keyescrow.PrivateKey, operatorsRaw string) error {
	if h == nil || h.service == nil || h.service.store == nil {
		return fmt.Errorf("corpusd handler: key escrow store is required")
	}
	publicBytes, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("corpusd handler: derive key escrow public key: %w", err)
	}
	var publicKey keyescrow.PublicKey
	copy(publicKey[:], publicBytes)
	operators, gateOpen := parseKeyEscrowOperators(operatorsRaw)
	h.keyEscrow = &keyEscrowRuntime{privateKey: privateKey, publicKey: publicKey, operators: operators, gateOpen: gateOpen}
	return nil
}

func (h *Handler) HandleKeyEscrowPublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !internalKeyEscrowCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.keyEscrow == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key escrow unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": base64.RawStdEncoding.EncodeToString(h.keyEscrow.publicKey[:])})
}

func (h *Handler) HandleKeyEscrow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !internalKeyEscrowCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil || h.keyEscrow == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key escrow unavailable"})
		return
	}
	var input keyEscrowPutRequest
	if !decodeKeyEscrowJSON(r, &input) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if input.Protector != keyescrow.ProtectorCustodian || strings.TrimSpace(input.ComputerID) == "" || strings.TrimSpace(input.KeyDigest) == "" || input.WrappedKey == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid escrow record"})
		return
	}
	record, err := keyescrow.ParseWrappedKey([]byte(input.WrappedKey))
	if err != nil || record.Protector != input.Protector || record.ComputerID != input.ComputerID || record.KeyDigest != input.KeyDigest {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid escrow record"})
		return
	}
	if err := h.service.store.UpsertKeyEscrow(r.Context(), input.ComputerID, input.Protector, []byte(input.WrappedKey), input.KeyDigest); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to store escrow record"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "escrowed"})
}

func (h *Handler) HandleKeyEscrowStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !internalKeyEscrowCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil || h.keyEscrow == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key escrow unavailable"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return
	}
	statuses, err := h.service.store.ListKeyEscrowStatus(r.Context(), computerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list escrow status"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"escrows": statuses})
}

func (h *Handler) HandleKeyUnwrapRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !h.authorizeKeyEscrowOperator(w, r) {
		return
	}
	var input keyUnwrapRequestInput
	if !decodeKeyEscrowJSON(r, &input) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	request, err := h.service.store.CreateKeyUnwrapRequest(r.Context(), input.ComputerID, r.Header.Get("X-Choir-Operator"), input.Reason, input.IdempotencyKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid unwrap request"})
		return
	}
	writeJSON(w, http.StatusCreated, request)
}

func (h *Handler) HandleKeyUnwrapRequestAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !h.authorizeKeyEscrowOperator(w, r) {
		return
	}
	requestID, action := keyUnwrapActionFromPath(r.URL.Path)
	if requestID == "" || (action != "approvals" && action != "reveal") {
		http.NotFound(w, r)
		return
	}
	if action == "approvals" {
		h.handleKeyUnwrapApproval(w, r, requestID)
		return
	}
	h.handleKeyUnwrapReveal(w, r, requestID)
}

func (h *Handler) handleKeyUnwrapApproval(w http.ResponseWriter, r *http.Request, requestID string) {
	var input keyUnwrapApprovalInput
	if !decodeKeyEscrowJSON(r, &input) || strings.TrimSpace(input.Approver) == "" || input.Approver != r.Header.Get("X-Choir-Operator") {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid approval"})
		return
	}
	request, err := h.service.store.ApproveKeyUnwrapRequest(r.Context(), requestID, input.Approver)
	if errors.Is(err, ErrSelfApproval) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "self approval is not allowed"})
		return
	}
	if errors.Is(err, ErrKeyUnwrapNotPending) || errors.Is(err, ErrDuplicateApproval) {
		writeJSON(w, http.StatusConflict, apiError{Error: "unwrap request cannot be approved"})
		return
	}
	if errors.Is(err, ErrKeyEscrowNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to approve unwrap request"})
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (h *Handler) handleKeyUnwrapReveal(w http.ResponseWriter, r *http.Request, requestID string) {
	request, approvers, err := h.service.store.GetKeyUnwrapRequest(r.Context(), requestID)
	if errors.Is(err, ErrKeyEscrowNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get unwrap request"})
		return
	}
	if request.Status != "approved" || !twoIndependentApprovals(approvers, request.RequestedBy) {
		writeJSON(w, http.StatusConflict, apiError{Error: "unwrap request is not approved"})
		return
	}
	wrappedJSON, keyDigest, err := h.service.store.GetKeyEscrow(r.Context(), request.ComputerID, keyescrow.ProtectorCustodian)
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
	dek, err := keyescrow.OpenDEK(h.keyEscrow.privateKey, record, request.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open escrow record"})
		return
	}
	if err := h.service.store.MarkKeyUnwrapRevealed(r.Context(), requestID); err != nil {
		writeJSON(w, http.StatusConflict, apiError{Error: "unwrap request is not approved"})
		return
	}
	revealedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if revealed, _, err := h.service.store.GetKeyUnwrapRequest(r.Context(), requestID); err == nil && revealed.RevealedAt != nil {
		revealedAt = revealed.RevealedAt.Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(map[string]any{
		"type": "dek_reveal", "request_id": requestID, "computer_id": request.ComputerID,
		"key_digest": keyDigest, "approvals": approvers, "revealed_at": revealedAt,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record escrow transparency"})
		return
	}
	if _, _, err := h.service.store.AppendKeyEscrowTransparency(r.Context(), payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record escrow transparency"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"dek": base64.RawStdEncoding.EncodeToString(dek), "key_digest": keyDigest})
}

func (h *Handler) HandleKeyEscrowTransparencyHead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !internalKeyEscrowCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil || h.keyEscrow == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key escrow unavailable"})
		return
	}
	seq, headHash, err := h.service.store.KeyEscrowTransparencyHead(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get transparency head"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "head_hash": headHash})
}

func (h *Handler) authorizeKeyEscrowOperator(w http.ResponseWriter, r *http.Request) bool {
	if h == nil || h.keyEscrow == nil || !h.keyEscrow.gateOpen {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "key escrow operator gate is closed"})
		return false
	}
	name := r.Header.Get("X-Choir-Operator")
	expected, ok := h.keyEscrow.operators[name]
	provided := sha256.Sum256([]byte(r.Header.Get("X-Choir-Operator-Token")))
	if !ok || !hmac.Equal(expected[:], provided[:]) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "operator authorization required"})
		return false
	}
	return true
}

func parseKeyEscrowOperators(raw string) (map[string][sha256.Size]byte, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	operators := make(map[string][sha256.Size]byte)
	for _, part := range strings.Split(raw, ",") {
		name, token, ok := strings.Cut(strings.TrimSpace(part), ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" || token == "" {
			return nil, false
		}
		if _, duplicate := operators[name]; duplicate {
			return nil, false
		}
		operators[name] = sha256.Sum256([]byte(token))
	}
	return operators, len(operators) > 0
}

func internalKeyEscrowCaller(r *http.Request) bool {
	return r.Header.Get("X-Internal-Caller") == "true"
}

func decodeKeyEscrowJSON(r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil && decoder.Decode(&struct{}{}) == io.EOF
}

func keyUnwrapActionFromPath(path string) (requestID, action string) {
	const prefix = "/internal/computers/keys/unwrap-requests/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

func twoIndependentApprovals(approvers []string, requestedBy string) bool {
	seen := make(map[string]struct{}, len(approvers))
	for _, approver := range approvers {
		if approver == requestedBy {
			continue
		}
		seen[approver] = struct{}{}
	}
	return len(seen) >= 2
}
