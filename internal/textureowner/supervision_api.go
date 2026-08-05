package textureowner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
)

type textureSupervisionImportRequest struct {
	TrajectoryID string `json:"trajectory_id"`
	CommandID    string `json:"command_id,omitempty"`
}

type textureSupervisionRebuildResponse struct {
	Status string `json:"status"`
}

// textureSupervisionCommandRequest deliberately excludes all authority fields:
// owner, computer, and actor identity are bound from the authenticated owner
// and the local Runtime, never accepted from the request. Command-owned
// artifacts are logical plaintext payloads; the appender reserves the command
// before it derives and pins their private content addresses.
type textureSupervisionCommandRequest struct {
	TrajectoryID     string                              `json:"trajectory_id"`
	TransactionID    string                              `json:"transaction_id"`
	TransactionClass string                              `json:"transaction_class"`
	CommandID        string                              `json:"command_id"`
	CommandDigest    string                              `json:"command_digest"`
	Expected         *computerevent.SupervisionExpected  `json:"expected"`
	Mutations        []computerevent.SupervisionMutation `json:"mutations"`
	PrivateArtifacts []textureSupervisionPrivateArtifact `json:"private_artifacts"`
}

type textureSupervisionPrivateArtifact struct {
	BindingID string `json:"binding_id"`
	Plaintext string `json:"plaintext"`
	MediaType string `json:"media_type"`
}

type textureSupervisionCommandResponse struct {
	Receipt          computerevent.Receipt                      `json:"receipt"`
	ArtifactDigest   string                                     `json:"artifact_digest"`
	PrivateArtifacts []computerevent.PrivateSupervisionArtifact `json:"private_artifacts"`
}

// HandleTextureSupervisionImport imports one owner-scoped legacy trajectory
// into the canonical supervision tape. A caller that omits command_id gets a
// stable UUIDv5 identity, so retries name the same import command.
func (h *Handler) HandleTextureSupervisionImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h == nil || h.Core == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision authority unavailable"})
		return
	}
	var request textureSupervisionImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision import request"})
		return
	}
	request.TrajectoryID = strings.TrimSpace(request.TrajectoryID)
	request.CommandID = strings.TrimSpace(request.CommandID)
	if request.TrajectoryID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "trajectory_id is required"})
		return
	}
	if request.CommandID == "" {
		request.CommandID = textureSupervisionImportCommandID(ownerID, request.TrajectoryID)
	}
	result, err := h.Core.ImportLegacySupervisionProjection(r.Context(), ownerID, request.TrajectoryID, request.CommandID)
	if err != nil {
		writeTextureSupervisionAPIError(w, err)
		return
	}
	writeAPIJSON(w, http.StatusOK, result)
}

// HandleTextureSupervisionCommand appends one owner-authorized transaction to
// the canonical per-computer supervision tape. Authority-bearing fields are
// derived locally so a request cannot impersonate another owner or actor.
func (h *Handler) HandleTextureSupervisionCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h == nil || h.Core == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision authority unavailable"})
		return
	}

	var request textureSupervisionCommandRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision command request"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision command request"})
		return
	}
	request.TrajectoryID = strings.TrimSpace(request.TrajectoryID)
	request.TransactionID = strings.TrimSpace(request.TransactionID)
	request.TransactionClass = strings.TrimSpace(request.TransactionClass)
	request.CommandID = strings.TrimSpace(request.CommandID)
	request.CommandDigest = strings.TrimSpace(request.CommandDigest)
	if request.TrajectoryID == "" || request.TransactionID == "" || request.TransactionClass == "" ||
		request.CommandID == "" || request.CommandDigest == "" || request.Expected == nil || len(request.Mutations) == 0 ||
		request.TransactionID != request.CommandID {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "complete supervision command is required"})
		return
	}
	payloads, logicalArtifacts, err := textureSupervisionPrivatePayloads(request.PrivateArtifacts)
	if err != nil || textureSupervisionLogicalArtifactRefs(request.Mutations, logicalArtifacts) != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision private artifacts"})
		return
	}
	transaction := computerevent.SupervisionTransaction{
		Schema:           computerevent.SupervisionSchemaV1,
		Reducer:          computerevent.SupervisionReducerV1,
		DigestRecipe:     computerevent.SupervisionDigestRecipeV1,
		TransactionID:    request.TransactionID,
		TransactionClass: request.TransactionClass,
		OwnerID:          ownerID,
		ComputerID:       h.Core.TextureSandboxID(),
		TrajectoryID:     request.TrajectoryID,
		CommandID:        request.CommandID,
		CommandDigest:    request.CommandDigest,
		Actor:            computerevent.SupervisionActor{ActorID: ownerID, Role: "owner", AuthorityRef: "owner:" + ownerID},
		Expected:         *request.Expected,
		Mutations:        request.Mutations,
	}
	validation := transaction
	validation.ReferencedArtifacts = logicalArtifacts
	validationDigest, validationErr := validation.ComputeCommandDigest()
	if validationErr != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision command request"})
		return
	}
	validation.CommandDigest = validationDigest
	if err := validation.ValidateLogical(); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision command request"})
		return
	}
	logical := transaction
	logical.ReferencedArtifacts = logicalArtifacts
	computedDigest, err := logical.ComputeCommandDigest()
	if err != nil || request.CommandDigest != computedDigest {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "supervision command_digest mismatch"})
		return
	}
	receipt, artifactDigest, artifacts, err := h.Core.AppendSupervisionTransactionWithPrivateArtifacts(r.Context(), transaction, payloads)
	if err != nil {
		writeTextureSupervisionAPIError(w, err)
		return
	}
	writeAPIJSON(w, http.StatusOK, textureSupervisionCommandResponse{
		Receipt: receipt, ArtifactDigest: artifactDigest, PrivateArtifacts: artifacts,
	})
}

func textureSupervisionPrivatePayloads(requests []textureSupervisionPrivateArtifact) ([]computerevent.PrivateSupervisionArtifactPayload, []computerevent.ReferencedArtifact, error) {
	payloads := make([]computerevent.PrivateSupervisionArtifactPayload, 0, len(requests))
	logical := make([]computerevent.ReferencedArtifact, 0, len(requests))
	seen := make(map[string]struct{}, len(requests))
	for _, request := range requests {
		bindingID := strings.TrimSpace(request.BindingID)
		mediaType := strings.TrimSpace(request.MediaType)
		if bindingID == "" || mediaType == "" || request.Plaintext == "" {
			return nil, nil, fmt.Errorf("complete private artifact is required")
		}
		if _, exists := seen[bindingID]; exists {
			return nil, nil, fmt.Errorf("duplicate private artifact binding")
		}
		seen[bindingID] = struct{}{}
		plaintext := []byte(request.Plaintext)
		plaintextDigest := computerevent.DigestBytes(plaintext)
		payloads = append(payloads, computerevent.PrivateSupervisionArtifactPayload{
			BindingID: bindingID, Plaintext: plaintext, MediaType: mediaType,
		})
		logical = append(logical, computerevent.ReferencedArtifact{
			Ref: computerevent.SupervisionArtifactPlaceholder(bindingID), ArtifactDigest: computerevent.ZeroHead,
			PlaintextDigest: plaintextDigest, LogicalPlaintextDigest: plaintextDigest,
			MediaType: mediaType, BindingID: bindingID,
		})
	}
	return payloads, logical, nil
}

func textureSupervisionLogicalArtifactRefs(mutations []computerevent.SupervisionMutation, artifacts []computerevent.ReferencedArtifact) error {
	placeholders := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		placeholders[artifact.Ref] = struct{}{}
	}
	for _, mutation := range mutations {
		var body any
		decoder := json.NewDecoder(bytes.NewReader(mutation.Body))
		decoder.UseNumber()
		if err := decoder.Decode(&body); err != nil {
			return fmt.Errorf("decode mutation private artifacts: %w", err)
		}
		if err := textureSupervisionLogicalArtifactRefValue(body, placeholders); err != nil {
			return err
		}
	}
	return nil
}

func textureSupervisionLogicalArtifactRefValue(value any, placeholders map[string]struct{}) error {
	switch value := value.(type) {
	case string:
		if computerevent.IsSupervisionArtifactPlaceholder(value) {
			if _, exists := placeholders[value]; !exists {
				return fmt.Errorf("unknown supervision artifact placeholder")
			}
			return nil
		}
		if strings.HasPrefix(value, "artifact:sha256:") {
			return fmt.Errorf("pre-pinned artifact refs are not accepted")
		}
	case []any:
		for _, item := range value {
			if err := textureSupervisionLogicalArtifactRefValue(item, placeholders); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range value {
			if err := textureSupervisionLogicalArtifactRefValue(item, placeholders); err != nil {
				return err
			}
		}
	}
	return nil
}

// HandleTextureSupervisionRebuild replays the authenticated computer's
// retained private supervision tape. Runtime reconstruction verifies the
// external signed chain before atomically replacing the local projection.
func (h *Handler) HandleTextureSupervisionRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "trusted internal caller required"})
		return
	}
	if h == nil || h.Core == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision authority unavailable"})
		return
	}
	rebuild := h.Core.RebuildSupervisionProjection
	if h.rebuildSupervisionProjection != nil {
		rebuild = h.rebuildSupervisionProjection
	}
	if err := rebuild(r.Context()); err != nil {
		writeTextureSupervisionAPIError(w, err)
		return
	}
	writeAPIJSON(w, http.StatusOK, textureSupervisionRebuildResponse{Status: "rebuilt"})
}

func textureSupervisionImportCommandID(ownerID, trajectoryID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:texture:supervision:import:"+ownerID+":"+trajectoryID)).String()
}

func writeTextureSupervisionAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "supervision trajectory not found", Code: "trajectory_not_found"})
	case errors.Is(err, computerevent.ErrSupervisionWritesDisabled):
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision authority unavailable", Code: "writes_disabled"})
	case errors.Is(err, computerevent.ErrNeedsProjectionRepair):
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision authority unavailable", Code: "projection_repair_required"})
	case errors.Is(err, computerevent.ErrSupervisionIdempotencyConflict), errors.Is(err, computerevent.ErrCASConflict), errors.Is(err, store.ErrLifecycleCommandConflict), errors.Is(err, store.ErrConcurrentStateChange),
		textureSupervisionProjectionConflict(err):
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "supervision transition rejected", Code: "state_conflict"})
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid supervision request", Code: textureSupervisionFailureCode(err)})
	}
}

func writeTextureSupervisionCreateAppendAPIError(w http.ResponseWriter, err error) {
	writeAPIJSON(w, http.StatusInternalServerError, apiError{
		Error: "failed to create document",
		Code:  textureSupervisionFailureCode(err),
	})
}

func textureSupervisionFailureCode(err error) string {
	switch {
	case errors.Is(err, computerevent.ErrSupervisionWritesDisabled):
		return "writes_disabled"
	case errors.Is(err, computerevent.ErrNeedsProjectionRepair):
		return "projection_repair_required"
	case errors.Is(err, computerevent.ErrSupervisionIdempotencyConflict), errors.Is(err, computerevent.ErrCASConflict), errors.Is(err, store.ErrLifecycleCommandConflict), errors.Is(err, store.ErrConcurrentStateChange):
		return "state_conflict"
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	for _, stage := range []struct {
		marker string
		code   string
	}{
		{"supervision transaction authority unavailable", "event_appender_unavailable"},
		{"supervision reservation unavailable", "reservation_store_unavailable"},
		{"frozen supervision plan storage unavailable", "frozen_plan_store_unavailable"},
		{"private supervision payload authority unavailable", "private_payload_authority_unavailable"},
		{"artifact pin receipt verifier unavailable", "pin_receipt_verifier_unavailable"},
		{"supervision command lookup unavailable", "command_store_unavailable"},
		{"supervision transaction targets wrong computer", "wrong_computer"},
		{"computer event appender: wrong computer", "wrong_computer"},
		{"pre-reservation transaction_id", "transaction_identity_invalid"},
		{"supervision event entropy", "event_identity_invalid"},
		{"compute supervision command digest", "command_digest_failed"},
		{"create reserved supervision event identity", "event_identity_failed"},
		{"canonical supervision transaction", "transaction_encoding_failed"},
		{"create supervision artifact ref", "artifact_ref_failed"},
		{"compute supervision pin intent", "pin_intent_failed"},
		{"pinned supervision artifact digest mismatch", "pin_digest_mismatch"},
		{"canonical supervision payload receipt", "pin_receipt_encoding_failed"},
		{"compute supervision request commitment", "request_commitment_failed"},
		{"reserve supervision command", "command_reservation_failed"},
		{"record supervision pin receipt", "frozen_plan_write_failed"},
		{"load frozen supervision plan", "frozen_plan_load_failed"},
		{"freeze supervision plan", "frozen_plan_write_failed"},
		{"encrypt supervision transaction", "transaction_encryption_failed"},
		{"pin private supervision transaction", "transaction_pin_failed"},
		{"verify supervision pin receipt", "transaction_pin_verification_failed"},
		{"verify frozen supervision pin receipt", "transaction_pin_verification_failed"},
		{"resolve head for new event", "canonical_head_unavailable"},
		{"resolve canonical head", "canonical_head_unavailable"},
		{"resolve embedded head", "projection_head_unavailable"},
		{"pin event", "event_pin_failed"},
		{"prepare embedded projection", "projection_prepare_failed"},
		{"head CAS", "event_append_failed"},
		{"verify head receipt", "event_receipt_verification_failed"},
		{"supervision transaction:", "transaction_invalid"},
	} {
		if strings.Contains(message, stage.marker) {
			return stage.code
		}
	}
	return "authority_failed"
}

func textureSupervisionProjectionConflict(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "supervision projection: stale") ||
		strings.Contains(message, "supervision projection: trajectory is settled") ||
		strings.Contains(message, "supervision projection: trajectory does not exist")
}
