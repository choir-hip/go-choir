package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type selfDevelopmentTarget struct {
	ComputerID    string
	RequiredScope string
}

type selfDevelopmentModeProjection struct {
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
	Receipt                          *computerevent.Receipt `json:"receipt,omitempty"`
}

type proxiedSelfDevelopmentDecision struct {
	Decision                         string                 `json:"decision"`
	IdempotencyKey                   string                 `json:"idempotency_key"`
	BundleDigest                     string                 `json:"bundle_digest"`
	VerifierRef                      string                 `json:"verifier_ref"`
	Reason                           string                 `json:"reason,omitempty"`
	ExpectedDesiredEventHead         string                 `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string                 `json:"expected_effective_event_head"`
	ExpectedPendingTransitionRef     *string                `json:"expected_pending_transition_ref"`
	ExpectedDesiredStateCommitment   string                 `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string                 `json:"expected_effective_state_commitment"`
	ModeReceipt                      *computerevent.Receipt `json:"mode_receipt,omitempty"`
}

func isSelfDevelopmentTarget(path, method string) bool {
	_, ok := parseSelfDevelopmentTarget(path, method)
	return ok
}

func parseSelfDevelopmentTarget(path, method string) (selfDevelopmentTarget, bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return selfDevelopmentTarget{}, false
	}
	remainder := strings.TrimPrefix(path, prefix)
	separator := strings.IndexByte(remainder, '/')
	if separator <= 0 {
		return selfDevelopmentTarget{}, false
	}
	computerID, err := url.PathUnescape(remainder[:separator])
	if err != nil || strings.TrimSpace(computerID) == "" {
		return selfDevelopmentTarget{}, false
	}
	suffix := remainder[separator:]
	if suffix == "/self-development/mode" {
		return selfDevelopmentTarget{}, false
	}
	target := selfDevelopmentTarget{ComputerID: strings.TrimSpace(computerID)}
	switch {
	case suffix == "/self-development/kernel-capabilities" && method == http.MethodGet:
		target.RequiredScope = "computer:self_development:read"
	case suffix == "/self-development/operations" && method == http.MethodPost:
		target.RequiredScope = "computer:self_development:propose"
	case suffix == "/self-development/genesis" && method == http.MethodPost:
		target.RequiredScope = "computer:self_development:genesis"
	case suffix == "/self-development/rollbacks" && method == http.MethodPost:
		target.RequiredScope = "computer:self_development:rollback"
	case strings.HasPrefix(suffix, "/self-development/operations/") && strings.HasSuffix(suffix, "/decision") && method == http.MethodPost:
		target.RequiredScope = "computer:self_development:approve"
	case strings.HasPrefix(suffix, "/self-development/operations/") && method == http.MethodGet:
		target.RequiredScope = "computer:self_development:read"
	case strings.HasPrefix(suffix, "/events/") && method == http.MethodGet:
		target.RequiredScope = "computer:self_development:read"
	default:
		return selfDevelopmentTarget{}, false
	}
	return target, true
}

func (h *Handler) HandleSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request) {
	target, ok := parseSelfDevelopmentTarget(r.URL.Path, r.Method)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod != "api_key" {
		if h.vmctlClient == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			return
		}
		ownership, ownershipErr := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, target.ComputerID)
		if ownershipErr != nil || ownership == nil || ownership.ComputerID != target.ComputerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}
	if authResult.AuthMethod == "api_key" {
		if authResult.ComputerID != target.ComputerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
			return
		}
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, target.RequiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + target.RequiredScope})
			return
		}
	}
	var requestBody []byte
	if r.Method == http.MethodPost {
		requestBody, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
			return
		}
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/computers/"+url.PathEscape(target.ComputerID))
	var mode selfDevelopmentModeProjection
	var decision proxiedSelfDevelopmentDecision
	modeGuarded := suffix == "/self-development/operations" || suffix == "/self-development/genesis" || strings.HasSuffix(suffix, "/decision")
	if modeGuarded {
		mode, err = h.readSelfDevelopmentMode(r.Context(), target.ComputerID, authResult.UserID)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "self-development mode authority unavailable"})
			return
		}
		switch {
		case suffix == "/self-development/genesis":
			if mode.Mode != "off" || mode.Generation != 0 ||
				strings.TrimSpace(h.cfg.SelfDevelopmentDisposableComputerID) == "" ||
				target.ComputerID != strings.TrimSpace(h.cfg.SelfDevelopmentDisposableComputerID) {
				writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development genesis requires the configured disposable computer with absent mode state"})
				return
			}
		case suffix == "/self-development/operations" && mode.Mode != "propose_only":
			writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development proposal mode is not enabled"})
			return
		case strings.HasSuffix(suffix, "/decision"):
			decoder := json.NewDecoder(bytes.NewReader(requestBody))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&decision); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development decision"})
				return
			}
			operationID := strings.TrimSuffix(strings.TrimPrefix(suffix, "/self-development/operations/"), "/decision")
			requiresConsumption := mode.Mode == "accept_once"
			if requiresConsumption && decision.Decision != "approve" {
				writeJSON(w, http.StatusConflict, errorResponse{Error: "accept_once authorizes only its exact approval"})
				return
			}
			if mode.Mode == "propose_only" && consumedModeReceiptMatches(mode.Receipt, operationID, decision) {
				decision.ModeReceipt = mode.Receipt
			} else {
				if decision.Decision != "approve" && decision.Decision != "reject" {
					writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development decision mode is not enabled"})
					return
				}
				switch mode.Mode {
				case "accept_once":
					if !modeProjectionMatchesDecision(mode, operationID, decision) {
						writeJSON(w, http.StatusConflict, errorResponse{Error: "accept_once mode does not bind this exact decision"})
						return
					}
				case "propose_only":
					if decision.Decision != "reject" {
						writeJSON(w, http.StatusConflict, errorResponse{Error: "accept_once mode does not bind this exact approval"})
						return
					}
				default:
					writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development decision mode is not enabled"})
					return
				}
				if requiresConsumption {
					receipt, consumeErr := h.consumeSelfDevelopmentMode(r.Context(), target.ComputerID, authResult.UserID, mode, decision)
					if consumeErr != nil {
						writeJSON(w, http.StatusConflict, errorResponse{Error: "accept_once mode consumption failed"})
						return
					}
					decision.ModeReceipt = &receipt
				}
			}
			requestBody, err = json.Marshal(decision)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development decision"})
				return
			}
		}
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "target computer resolver unavailable"})
		return
	}
	ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, target.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve target computer"})
		return
	}
	if ownership == nil || ownership.SandboxURL == "" || ownership.State != "active" {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "target computer is not active"})
		return
	}
	upstreamURL, err := joinBasePath(ownership.SandboxURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build target computer request"})
		return
	}
	var upstreamBody io.Reader
	if requestBody != nil {
		upstreamBody = bytes.NewReader(requestBody)
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, upstreamBody)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
		return
	}
	upstream.Header.Set("X-Internal-Caller", "true")
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", target.ComputerID)
	if suffix == "/self-development/genesis" {
		upstream.Header.Set("X-Self-Development-Disposable", "true")
		upstream.Header.Set("X-Self-Development-Mode-Generation", "0")
	}
	if r.Method == http.MethodPost {
		upstream.Header.Set("Content-Type", "application/json")
	}
	response, err := h.sandboxHTTP.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "target computer request failed"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid target computer response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func (h *Handler) readSelfDevelopmentMode(ctx context.Context, computerID, userID string) (selfDevelopmentModeProjection, error) {
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/self-development/mode")
	if err != nil {
		return selfDevelopmentModeProjection{}, err
	}
	u, err := url.Parse(target)
	if err != nil {
		return selfDevelopmentModeProjection{}, err
	}
	query := u.Query()
	query.Set("computer_id", computerID)
	u.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return selfDevelopmentModeProjection{}, err
	}
	request.Header.Set("X-Internal-Caller", "true")
	request.Header.Set("X-Authenticated-User", userID)
	response, err := h.corpusd.Do(request)
	if err != nil {
		return selfDevelopmentModeProjection{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return selfDevelopmentModeProjection{}, fmt.Errorf("mode authority status %d", response.StatusCode)
	}
	var mode selfDevelopmentModeProjection
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&mode); err != nil {
		return selfDevelopmentModeProjection{}, err
	}
	return mode, nil
}

func (h *Handler) consumeSelfDevelopmentMode(ctx context.Context, computerID, userID string, current selfDevelopmentModeProjection, decision proxiedSelfDevelopmentDecision) (computerevent.Receipt, error) {
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/self-development/mode")
	if err != nil {
		return computerevent.Receipt{}, err
	}
	u, err := url.Parse(target)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	query := u.Query()
	query.Set("computer_id", computerID)
	u.RawQuery = query.Encode()
	consumptionKey, err := consumedModeIdempotency(decisionOperationID(current), decision)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	body, err := json.Marshal(map[string]any{
		"mode": "propose_only", "expected_generation": current.Generation,
		"idempotency_key": consumptionKey,
	})
	if err != nil {
		return computerevent.Receipt{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return computerevent.Receipt{}, err
	}
	request.Header.Set("X-Internal-Caller", "true")
	request.Header.Set("X-Authenticated-User", userID)
	request.Header.Set("Content-Type", "application/json")
	response, err := h.corpusd.Do(request)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	defer response.Body.Close()
	var consumed selfDevelopmentModeProjection
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&consumed) != nil || consumed.Receipt == nil {
		return computerevent.Receipt{}, fmt.Errorf("mode consumption status %d", response.StatusCode)
	}
	return *consumed.Receipt, nil
}

func decisionOperationID(mode selfDevelopmentModeProjection) string {
	return strings.TrimSpace(mode.OperationID)
}

func modeProjectionMatchesDecision(mode selfDevelopmentModeProjection, operationID string, decision proxiedSelfDevelopmentDecision) bool {
	return decision.ExpectedPendingTransitionRef != nil &&
		mode.OperationID == operationID && mode.BundleDigest == decision.BundleDigest &&
		mode.ExpectedDesiredEventHead == decision.ExpectedDesiredEventHead && mode.ExpectedEffectiveEventHead == decision.ExpectedEffectiveEventHead &&
		mode.ExpectedPendingTransitionRef == strings.TrimSpace(*decision.ExpectedPendingTransitionRef) &&
		mode.ExpectedDesiredStateCommitment == decision.ExpectedDesiredStateCommitment &&
		mode.ExpectedEffectiveStateCommitment == decision.ExpectedEffectiveStateCommitment
}

func consumedModeIdempotency(operationID string, decision proxiedSelfDevelopmentDecision) (string, error) {
	digest, err := selfdevprotocol.DecisionBindingDigest(proxiedDecisionBinding(operationID, decision))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("accept-once-consumed:%s:%s", strings.TrimSpace(operationID), digest), nil
}

func consumedModeReceiptMatches(receipt *computerevent.Receipt, operationID string, decision proxiedSelfDevelopmentDecision) bool {
	if receipt == nil || receipt.ReceiptKind != "ModeReceipt" || decision.ExpectedPendingTransitionRef == nil || decision.Decision != "approve" {
		return false
	}
	expectedIdempotency, err := consumedModeIdempotency(operationID, decision)
	if err != nil {
		return false
	}
	field := func(name string) string {
		value, _ := receipt.KindFields[name].(string)
		return value
	}
	return field("old_mode") == "accept_once" && field("new_mode") == "propose_only" &&
		field("consumed_operation_id") == operationID && field("consumed_bundle_digest") == decision.BundleDigest &&
		field("consumed_desired_event_head") == decision.ExpectedDesiredEventHead &&
		field("consumed_effective_event_head") == decision.ExpectedEffectiveEventHead &&
		field("consumed_pending_transition_ref") == strings.TrimSpace(*decision.ExpectedPendingTransitionRef) &&
		field("consumed_desired_state_commitment") == decision.ExpectedDesiredStateCommitment &&
		field("consumed_effective_state_commitment") == decision.ExpectedEffectiveStateCommitment &&
		field("idempotency_key") == expectedIdempotency
}

func proxiedDecisionBinding(operationID string, decision proxiedSelfDevelopmentDecision) selfdevprotocol.DecisionBinding {
	return selfdevprotocol.DecisionBinding{
		OperationID: operationID, Decision: decision.Decision, IdempotencyKey: decision.IdempotencyKey,
		BundleDigest: decision.BundleDigest, VerifierRef: decision.VerifierRef, Reason: decision.Reason,
		ExpectedDesiredEventHead: decision.ExpectedDesiredEventHead, ExpectedEffectiveEventHead: decision.ExpectedEffectiveEventHead,
		ExpectedPendingTransitionRef:     decision.ExpectedPendingTransitionRef,
		ExpectedDesiredStateCommitment:   decision.ExpectedDesiredStateCommitment,
		ExpectedEffectiveStateCommitment: decision.ExpectedEffectiveStateCommitment,
	}
}
