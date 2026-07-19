package platform

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type computerEventPinRequest struct {
	ComputerID          string `json:"computer_id"`
	Payload             string `json:"payload_base64"`
	MediaType           string `json:"media_type"`
	PrivacyClass        string `json:"privacy_class"`
	PinNamespace        string `json:"pin_namespace"`
	RequestCommitment   string `json:"request_commitment"`
	PinIntentCommitment string `json:"pin_intent_commitment"`
}

type computerEventPinResponse struct {
	ArtifactDigest string                `json:"artifact_digest"`
	Receipt        computerevent.Receipt `json:"receipt"`
}

type computerCredentialIssueRequest struct {
	ComputerID     string `json:"computer_id"`
	RealizationID  string `json:"realization_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type computerCredentialIssueResponse struct {
	Envelope ComputerCredentialEnvelope `json:"envelope"`
	Receipt  computerevent.Receipt      `json:"lifecycle_receipt"`
}

func (h *Handler) HandleComputerCredentialIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" || h == nil || h.service == nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "credential issuance refused"})
		return
	}
	var request computerCredentialIssueRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid credential issuance request"})
		return
	}
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	request.RealizationID = strings.TrimSpace(request.RealizationID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	envelope, receipt, err := h.service.MintComputerCredentialEnvelope(r.Context(), request.ComputerID, request.RealizationID, request.IdempotencyKey, time.Now().UTC().Add(4*time.Minute))
	if err != nil {
		writeJSON(w, http.StatusConflict, apiError{Error: "credential issuance refused"})
		return
	}
	writeJSON(w, http.StatusCreated, computerCredentialIssueResponse{Envelope: envelope, Receipt: receipt})
}

func (h *Handler) HandleComputerCredentialRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var request struct {
		ComputerID string `json:"computer_id"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid credential renewal request"})
		return
	}
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	if h == nil || h.eventAuth == nil || h.service == nil || h.eventAuth.Authorize(r, request.ComputerID, "event:read") != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "credential renewal refused"})
		return
	}
	result, err := h.service.RenewComputerCapability(r.Context(), request.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "credential renewal unavailable"})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) HandleComputerCredentialExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "credential exchange unavailable"})
		return
	}
	encoded, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil || len(encoded) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid credential envelope"})
		return
	}
	result, err := h.service.exchangeComputerCredentialEnvelope(r.Context(), encoded)
	if err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "credential envelope refused"})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) HandleComputerEventHead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if !h.authorizeComputerEvent(w, r, computerID, "event:read") {
		return
	}
	head, err := h.eventCAS.Head(r.Context(), computerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "event head unavailable"})
		return
	}
	if head == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "computer event head not initialized"})
		return
	}
	writeJSON(w, http.StatusOK, head)
}

func (h *Handler) HandleComputerEventPin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var request computerEventPinRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	if !h.authorizeComputerEvent(w, r, request.ComputerID, "event:pin") {
		return
	}
	payload, err := base64.RawStdEncoding.DecodeString(request.Payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid payload encoding"})
		return
	}
	var pin computerevent.PinResult
	switch request.PinNamespace {
	case "computer-event":
		pin, err = h.eventArtifacts.PinEvent(r.Context(), request.ComputerID, payload, request.RequestCommitment)
	case "computer-event-payload":
		pin, err = h.eventArtifacts.pinPayload(r.Context(), request.ComputerID, payload, request.MediaType, request.PrivacyClass, request.PinIntentCommitment)
	default:
		err = errors.New("unsupported pin namespace")
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, computerEventPinResponse{ArtifactDigest: pin.ArtifactDigest, Receipt: pin.Receipt})
}

func (h *Handler) HandleComputerEventAppend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var request computerevent.CASRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if !h.authorizeComputerEvent(w, r, request.Event.ComputerID, "event:append") {
		return
	}
	receipt, err := h.eventCAS.CompareAndSwap(r.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrComputerEventCASConflict) {
			status = http.StatusConflict
		}
		writeJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, receipt)
}

func (h *Handler) HandleComputerEventReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if !h.authorizeComputerEvent(w, r, computerID, "event:read") {
		return
	}
	var after uint64
	if raw := strings.TrimSpace(r.URL.Query().Get("after_sequence")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "after_sequence must be an unsigned integer"})
			return
		}
		after = value
	}
	records, err := h.eventArtifacts.Events(r.Context(), computerID, after)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "computer event replay unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *Handler) authorizeComputerEvent(w http.ResponseWriter, r *http.Request, computerID, scope string) bool {
	if h == nil || h.eventCAS == nil || h.eventArtifacts == nil || h.eventAuth == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer event service disabled"})
		return false
	}
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return false
	}
	if err := h.eventAuth.Authorize(r, computerID, scope); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer event capability refused"})
		return false
	}
	return true
}
