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
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func parseComputerLifecyclePath(path string) (computerID, action string, ok bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 3 || parts[1] != "lifecycle" || (parts[2] != "status" && parts[2] != "start" && parts[2] != "stop" && parts[2] != "restart") {
		return "", "", false
	}
	computerID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", "", false
	}
	return strings.TrimSpace(computerID), parts[2], true
}

func isComputerLifecyclePath(path string) bool {
	_, _, ok := parseComputerLifecyclePath(path)
	return ok
}

type resolvedComputerTarget struct {
	ComputerID string
	UserID     string
	DesktopID  string
	SandboxURL string
	State      string
	Epoch      int64
}

func (h *Handler) resolveAuthorizedComputer(ctx context.Context, authResult *AuthResult, computerID string) (*resolvedComputerTarget, error) {
	if h.vmctlClient == nil {
		return nil, fmt.Errorf("computer ownership authority unavailable")
	}
	scoped, err := h.vmctlClient.LookupComputerContext(ctx, authResult.UserID, computerID)
	if err != nil || scoped == nil {
		return nil, err
	}
	return &resolvedComputerTarget{
		ComputerID: scoped.ComputerID, UserID: scoped.UserID, DesktopID: scoped.DesktopID,
		SandboxURL: scoped.SandboxURL, State: scoped.State, Epoch: scoped.Epoch,
	}, nil
}

func (h *Handler) HandleComputerLifecycle(w http.ResponseWriter, r *http.Request) {
	computerID, action, ok := parseComputerLifecyclePath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if (action == "status" && r.Method != http.MethodGet) || (action != "status" && r.Method != http.MethodPost) {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" && (authResult.ComputerID != computerID || (!hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, "computer:lifecycle"))) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
		return
	}
	var request struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if action != "status" {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || strings.TrimSpace(request.IdempotencyKey) == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "idempotency_key is required"})
			return
		}
	}
	ownership, err := h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
	if err != nil || ownership == nil || ownership.ComputerID != computerID {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "computer not found"})
		return
	}
	if action == "status" {
		writeJSON(w, http.StatusOK, map[string]any{
			"computer_id": ownership.ComputerID, "desktop_id": ownership.DesktopID,
			"state": ownership.State, "realization_epoch": ownership.Epoch,
		})
		return
	}
	commitmentBytes, _ := computerevent.CanonicalJSON(map[string]string{"computer_id": computerID, "action": action, "idempotency_key": strings.TrimSpace(request.IdempotencyKey)})
	control := platform.LifecycleControlRequest{
		Phase: "prepare", ComputerID: computerID, IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		RequestCommitment: computerevent.DigestBytes(commitmentBytes), Action: action,
		PriorState: ownership.State, PriorEpoch: ownership.Epoch,
	}
	prepared, err := h.lifecycleControl(r, authResult.UserID, control)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "lifecycle durable intent unavailable"})
		return
	}
	if prepared.Status == "completed" && prepared.Receipt != nil {
		writeJSON(w, http.StatusOK, prepared.Receipt)
		return
	}
	control.PriorState, control.PriorEpoch = prepared.PriorState, prepared.PriorEpoch
	switch action {
	case "stop":
		if ownership.State != "stopped" {
			err = h.vmctlClient.StopDesktop(ownership.UserID, ownership.DesktopID)
		}
	case "start":
		if ownership.State != "active" {
			_, err = h.vmctlClient.ResolveDesktopContext(r.Context(), ownership.UserID, ownership.DesktopID)
		}
	case "restart":
		if ownership.State == "stopped" {
			_, err = h.vmctlClient.ResolveDesktopContext(r.Context(), ownership.UserID, ownership.DesktopID)
		} else if ownership.Epoch <= control.PriorEpoch {
			if err = h.vmctlClient.StopDesktop(ownership.UserID, ownership.DesktopID); err == nil {
				_, err = h.vmctlClient.ResolveDesktopContext(r.Context(), ownership.UserID, ownership.DesktopID)
			}
		}
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "lifecycle actuation failed"})
		return
	}
	result, err := h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
	if err != nil || result == nil || (action == "stop" && result.State != "stopped") ||
		(action != "stop" && result.State != "active") || (action == "restart" && result.Epoch <= control.PriorEpoch) {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "lifecycle resulting state was not observed"})
		return
	}
	control.Phase, control.ResultingState, control.ResultingEpoch = "complete", result.State, result.Epoch
	completed, err := h.lifecycleControl(r, authResult.UserID, control)
	if err != nil || completed.Receipt == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "lifecycle receipt authority unavailable after observed actuation"})
		return
	}
	writeJSON(w, http.StatusCreated, completed.Receipt)
}

func (h *Handler) lifecycleControl(r *http.Request, userID string, control platform.LifecycleControlRequest) (platform.LifecycleControlResult, error) {
	body, err := computerevent.CanonicalJSON(control)
	if err != nil {
		return platform.LifecycleControlResult{}, err
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/lifecycle/control")
	if err != nil {
		return platform.LifecycleControlResult{}, err
	}
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return platform.LifecycleControlResult{}, err
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("X-Internal-Caller", "true")
	upstream.Header.Set("X-Authenticated-User", userID)
	response, err := h.corpusd.Do(upstream)
	if err != nil {
		return platform.LifecycleControlResult{}, err
	}
	defer response.Body.Close()
	var result platform.LifecycleControlResult
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return result, fmt.Errorf("lifecycle control status %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}
