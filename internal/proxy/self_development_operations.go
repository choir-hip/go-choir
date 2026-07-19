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

type proxiedSelfDevelopmentStart struct {
	IdempotencyKey string                 `json:"idempotency_key"`
	Prompt         string                 `json:"prompt"`
	ModeReceipt    *computerevent.Receipt `json:"mode_receipt,omitempty"`
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
	var mode selfDevelopmentModeProjection
	modeGuarded := suffix == "/self-development/operations" || suffix == "/self-development/genesis"
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
		case suffix == "/self-development/operations":
			var start proxiedSelfDevelopmentStart
			decoder := json.NewDecoder(bytes.NewReader(requestBody))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&start); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
				return
			}
			start.ModeReceipt = mode.Receipt
			requestBody, err = json.Marshal(start)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
				return
			}
		}
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
