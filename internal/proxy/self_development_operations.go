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
	"time"
)

type selfDevelopmentTarget struct {
	ComputerID    string
	RequiredScope string
}

type selfDevelopmentModeProjection struct {
	ComputerID                       string `json:"computer_id"`
	Mode                             string `json:"mode"`
	Generation                       uint64 `json:"generation"`
	OperationID                      string `json:"operation_id,omitempty"`
	BundleDigest                     string `json:"bundle_digest,omitempty"`
	ExpectedDesiredEventHead         string `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead       string `json:"expected_effective_event_head,omitempty"`
	ExpectedDesiredStateCommitment   string `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string `json:"expected_effective_state_commitment,omitempty"`
}

type proxiedSelfDevelopmentDecision struct {
	Decision                         string `json:"decision"`
	IdempotencyKey                   string `json:"idempotency_key"`
	BundleDigest                     string `json:"bundle_digest"`
	VerifierRef                      string `json:"verifier_ref"`
	Reason                           string `json:"reason,omitempty"`
	ExpectedDesiredEventHead         string `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string `json:"expected_effective_event_head"`
	ExpectedDesiredStateCommitment   string `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string `json:"expected_effective_state_commitment"`
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
	modeGuarded := suffix == "/self-development/operations" || strings.HasSuffix(suffix, "/decision")
	if modeGuarded {
		mode, err = h.readSelfDevelopmentMode(r.Context(), target.ComputerID, authResult.UserID)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "self-development mode authority unavailable"})
			return
		}
		switch {
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
			if decision.Decision == "approve" {
				if mode.Mode != "accept_once" || mode.OperationID != operationID || mode.BundleDigest != decision.BundleDigest ||
					mode.ExpectedDesiredEventHead != decision.ExpectedDesiredEventHead || mode.ExpectedEffectiveEventHead != decision.ExpectedEffectiveEventHead ||
					mode.ExpectedDesiredStateCommitment != decision.ExpectedDesiredStateCommitment ||
					mode.ExpectedEffectiveStateCommitment != decision.ExpectedEffectiveStateCommitment {
					writeJSON(w, http.StatusConflict, errorResponse{Error: "accept_once mode does not bind this exact approval"})
					return
				}
			} else if decision.Decision != "reject" || (mode.Mode != "propose_only" && mode.Mode != "accept_once") {
				writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development decision mode is not enabled"})
				return
			}
		}
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer routing unavailable"})
		return
	}
	ownership, err := h.vmctlClient.LookupDesktopContext(r.Context(), authResult.UserID, target.ComputerID)
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
	if strings.HasSuffix(suffix, "/decision") && response.StatusCode >= 200 && response.StatusCode < 300 && mode.Mode == "accept_once" {
		if err := h.disableConsumedSelfDevelopmentMode(r.Context(), target.ComputerID, authResult.UserID, mode, decision); err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "decision committed but accept_once mode reversion is pending"})
			return
		}
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

func (h *Handler) disableConsumedSelfDevelopmentMode(ctx context.Context, computerID, userID string, current selfDevelopmentModeProjection, decision proxiedSelfDevelopmentDecision) error {
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/self-development/mode")
	if err != nil {
		return err
	}
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Set("computer_id", computerID)
	u.RawQuery = query.Encode()
	body, err := json.Marshal(map[string]any{
		"mode": "off", "expected_generation": current.Generation,
		"idempotency_key": fmt.Sprintf("accept-once-consumed:%s:%d:%s", current.OperationID, current.Generation, decision.IdempotencyKey),
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("X-Internal-Caller", "true")
	request.Header.Set("X-Authenticated-User", userID)
	request.Header.Set("Content-Type", "application/json")
	response, err := h.corpusd.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("mode reversion status %d at %s", response.StatusCode, time.Now().UTC().Format(time.RFC3339Nano))
	}
	return nil
}
