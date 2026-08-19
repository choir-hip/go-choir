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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func parseComputerLifecyclePath(path string) (computerID, action string, ok bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 3 || parts[1] != "lifecycle" || (parts[2] != "status" && parts[2] != "start" && parts[2] != "stop" && parts[2] != "restart" && parts[2] != "refresh") {
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
	ComputerID  string
	UserID      string
	DesktopID   string
	VMID        string
	ComputerURL string
	State       string
	Epoch       int64
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
		ComputerID: scoped.ComputerID, UserID: scoped.UserID, DesktopID: scoped.DesktopID, VMID: scoped.VMID,
		ComputerURL: scoped.ComputerURL, State: scoped.State, Epoch: scoped.Epoch,
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
	if authResult.AuthMethod == "api_key" && !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, "computer:lifecycle") {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
		return
	}
	var ownership *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		ownership, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
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
	if authResult.AuthMethod != "api_key" {
		ownership, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || ownership == nil || ownership.ComputerID != computerID {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "computer not found"})
			return
		}
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
	case "refresh":
		_, err = h.vmctlClient.RefreshDesktopContext(r.Context(), ownership.UserID, ownership.DesktopID)
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "lifecycle actuation failed"})
		return
	}
	result, err := h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
	if err != nil || result == nil || (action == "stop" && result.State != "stopped") ||
		(action != "stop" && result.State != "active") || (platform.OwnerVMLifecycleAdvancesEpoch(action) && result.Epoch <= control.PriorEpoch) {
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

func computerLifecycleGuestComputerID(path, action string) (string, bool) {
	const prefix = "/api/computers/"
	suffix := "/lifecycle/" + action
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	computerID, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false
	}
	return strings.TrimSpace(computerID), true
}

func computerWorkspaceReplaceComputerID(path string) (string, bool) {
	return computerLifecycleGuestComputerID(path, "replace-workspace")
}

func computerRematerializeComputerID(path string) (string, bool) {
	return computerLifecycleGuestComputerID(path, "rematerialize-from-tape")
}

func computerRestoreComputerID(path string) (string, bool) {
	return computerLifecycleGuestComputerID(path, "restore")
}

func computerCheckpointComputerID(path string) (string, bool) {
	return computerLifecycleGuestComputerID(path, "checkpoint")
}

func computerImportResidueSnapshotComputerID(path string) (string, bool) {
	return computerLifecycleGuestComputerID(path, "import-residue-snapshot")
}

func isComputerWorkspaceReplacePath(path string) bool {
	_, ok := computerWorkspaceReplaceComputerID(path)
	return ok
}

func isComputerRematerializePath(path string) bool {
	_, ok := computerRematerializeComputerID(path)
	return ok
}

func isComputerRestorePath(path string) bool {
	_, ok := computerRestoreComputerID(path)
	return ok
}

func isComputerCheckpointPath(path string) bool {
	_, ok := computerCheckpointComputerID(path)
	return ok
}

func isComputerImportResidueSnapshotPath(path string) bool {
	_, ok := computerImportResidueSnapshotComputerID(path)
	return ok
}

func (h *Handler) HandleComputerWorkspaceReplace(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerWorkspaceReplaceComputerID(r.URL.Path)
	if !ok {
		computerID, ok = computerRematerializeComputerID(r.URL.Path)
	}
	if !ok {
		computerID, ok = computerRestoreComputerID(r.URL.Path)
	}
	if !ok {
		computerID, ok = computerCheckpointComputerID(r.URL.Path)
	}
	if !ok {
		computerID, ok = computerImportResidueSnapshotComputerID(r.URL.Path)
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:lifecycle"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
			return
		}
	}

	var target *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
	} else {
		target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}

	var autoputerURL string
	if authResult.AuthMethod == "api_key" {
		autoputerURL, err = h.resolveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
	} else if target.State == "active" && strings.TrimSpace(target.ComputerURL) != "" {
		autoputerURL = target.ComputerURL
	} else {
		err = fmt.Errorf("computer realization is not active")
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build workspace replace request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build workspace replace request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	maxBody := int64(64 << 10)
	if isComputerRematerializePath(r.URL.Path) || isComputerRestorePath(r.URL.Path) {
		maxBody = 1 << 20
	}
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, u.String(), http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid workspace replace request"})
		return
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		upstream.Header.Set("Content-Type", contentType)
	}
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		upstream.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		upstream.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.autoputerHTTP
	// Residue import serializes a bounded projection batch (110s budget).
	// Checkpoint, restore, and rematerialize reconstruct/extract full Dolt state (~2m on staging),
	// matching replay completeness. Route them through replayAutoputerHTTP (10m on staging).
	if isComputerImportResidueSnapshotPath(r.URL.Path) && h.residueImportAutoputerHTTP != nil {
		client = h.residueImportAutoputerHTTP
	} else if (isComputerCheckpointPath(r.URL.Path) || isComputerRestorePath(r.URL.Path) || isComputerRematerializePath(r.URL.Path)) && h.replayAutoputerHTTP != nil {
		client = h.replayAutoputerHTTP
	}
	if client == nil {
		client = http.DefaultClient
	}
	if client != nil && client.Timeout > 0 {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(client.Timeout + 30*time.Second))
	}
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "workspace replace authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid workspace replace response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func computerBootstrapChainComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/lifecycle/bootstrap-chain"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	computerID, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false
	}
	return strings.TrimSpace(computerID), true
}

func isComputerBootstrapChainPath(path string) bool {
	_, ok := computerBootstrapChainComputerID(path)
	return ok
}

func (h *Handler) HandleComputerBootstrapChain(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerBootstrapChainComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:lifecycle"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
			return
		}
	}

	var target *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
	} else {
		target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}

	var autoputerURL string
	if authResult.AuthMethod == "api_key" {
		autoputerURL, err = h.resolveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
	} else if target.State == "active" && strings.TrimSpace(target.ComputerURL) != "" {
		autoputerURL = target.ComputerURL
	} else {
		err = fmt.Errorf("computer realization is not active")
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build chain bootstrap request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build chain bootstrap request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, u.String(), http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid chain bootstrap request"})
		return
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		upstream.Header.Set("Content-Type", contentType)
	}
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		upstream.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		upstream.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.autoputerHTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "chain bootstrap authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid chain bootstrap response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
