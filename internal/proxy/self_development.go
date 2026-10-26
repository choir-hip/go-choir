package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// errStandingAutonomyTier refuses a standing_consensus arm on a computer
// that is not test-tier (P4; owner ruling: autonomy only on test computers
// until desk-cell confinement lands).
var errStandingAutonomyTier = errors.New("standing autonomy is allowed only on test-tier computers")

func selfDevelopmentGuestRoute(path string) (computerID, action string, ok bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rawID, after, found := strings.Cut(rest, "/self-development/")
	if !found || strings.TrimSpace(rawID) == "" || strings.Contains(rawID, "/") || strings.TrimSpace(after) == "" {
		return "", "", false
	}
	decoded, err := url.PathUnescape(rawID)
	if err != nil || strings.TrimSpace(decoded) == "" {
		return "", "", false
	}
	switch strings.TrimSpace(after) {
	case "mode", "replay-completeness", "genesis":
		return "", "", false
	case "operations":
		// GET lists. C1e B1 deleted the guest's start-operation POST; a
		// change starts with POST /api/prompt-bar on an armed computer, so
		// the collection is read-only here too
		// (docs/problems/surface-broken-calls-2026-10-14.md, item 10).
		return strings.TrimSpace(decoded), "read", true
	case "rollbacks":
		return strings.TrimSpace(decoded), "rollback", true
	case "kernel-capabilities":
		return strings.TrimSpace(decoded), "read", true
	case "adoptions":
		// P8: GET lists (read); POST adopts a published release (the
		// owner's act). The method picks the action.
		return strings.TrimSpace(decoded), "adoptions", true
	case "rebase-retry":
		// U4 slice 8: the owner re-opens a spent rebase pair. POST only, and
		// an owner act like an approval.
		return strings.TrimSpace(decoded), "rebase-retry", true
	}
	parts := strings.Split(strings.Trim(after, "/"), "/")
	if len(parts) == 2 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(decoded), "read", true
	}
	if len(parts) == 3 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" && parts[2] == "decision" {
		return strings.TrimSpace(decoded), "approve", true
	}
	if len(parts) == 3 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" && (parts[2] == "receipts" || parts[2] == "source-patch" || parts[2] == "release-draft" || parts[2] == "visual-evidence" || parts[2] == "release-witness") {
		return strings.TrimSpace(decoded), "read", true
	}
	if len(parts) == 2 && parts[0] == "adoptions" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(decoded), "read", true
	}
	return "", "", false
}

func isSelfDevelopmentGuestPath(path string) bool {
	_, _, ok := selfDevelopmentGuestRoute(path)
	return ok
}

func selfDevelopmentGuestMethodAllowed(method, action string) bool {
	switch action {
	case "approve", "rollback", "rebase-retry":
		return method == http.MethodPost
	case "read":
		return method == http.MethodGet
	case "adoptions":
		return method == http.MethodGet || method == http.MethodPost
	default:
		return false
	}
}

// selfDevelopmentCorpusdRoute names the corpusd-held self-development
// resources: the signed mode, and the event head an owner's accept_once
// approval binds.
func selfDevelopmentCorpusdRoute(path string) (computerID, resource string, ok bool) {
	for _, resource := range []string{"mode", "head"} {
		if computerID, ok := selfDevelopmentCorpusdComputerID(path, "/self-development/"+resource); ok {
			return computerID, resource, true
		}
	}
	return "", "", false
}

func selfDevelopmentModeComputerID(path string) (string, bool) {
	return selfDevelopmentCorpusdComputerID(path, "/self-development/mode")
}

func selfDevelopmentCorpusdComputerID(path, suffix string) (string, bool) {
	const prefix = "/api/computers/"
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

func isSelfDevelopmentModePath(path string) bool {
	_, _, ok := selfDevelopmentCorpusdRoute(path)
	return ok
}
func selfDevelopmentReplayCompletenessComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/self-development/replay-completeness"
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

func isSelfDevelopmentReplayCompletenessPath(path string) bool {
	_, ok := selfDevelopmentReplayCompletenessComputerID(path)
	return ok
}

func (h *Handler) HandleSelfDevelopmentMode(w http.ResponseWriter, r *http.Request) {
	computerID, resource, ok := selfDevelopmentCorpusdRoute(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet && (r.Method != http.MethodPut || resource != "mode") {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	requiredScope := "computer:self_development:read"
	if r.Method == http.MethodPut {
		requiredScope = "computer:self_development:mode"
	}
	if authResult.AuthMethod == "api_key" {
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
			return
		}
		if _, ok := h.requireAPIKeyComputerTarget(w, r, authResult, computerID, ""); !ok {
			return
		}
		// Arming self-development is the owner's act (selfdev update
		// architecture P3), never an agent key's, `admin` included.
		if r.Method == http.MethodPut && !authorizeOwnerAct(w, authResult, "self-development mode set") {
			return
		}
	} else {
		if h.vmctlClient == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			return
		}
		ownership, ownershipErr := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, computerID)
		if ownershipErr != nil || ownership == nil || ownership.ComputerID != computerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}
	upstreamPath := "/internal/computers/self-development/mode"
	if resource == "head" {
		upstreamPath = "/internal/computers/events/head"
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, upstreamPath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	query := u.Query()
	query.Set("computer_id", computerID)
	u.RawQuery = query.Encode()
	upstreamMethod := r.Method
	var upstreamBody io.Reader
	ownerActHeader := ""
	if upstreamMethod == http.MethodPut {
		upstreamMethod = http.MethodPost
		body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if readErr != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
			return
		}
		// S1: the proxy authenticated the owner above; it now signs that
		// statement over this exact write, for corpusd to verify.
		ownerActHeader, err = h.mintModeOwnerAct(r.Context(), authResult, computerID, body)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errStandingAutonomyTier) {
				status = http.StatusForbidden
			}
			writeJSON(w, status, errorResponse{Error: err.Error()})
			return
		}
		upstreamBody = bytes.NewReader(body)
	}
	upstream, err := http.NewRequestWithContext(r.Context(), upstreamMethod, u.String(), upstreamBody)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
		return
	}
	hostauth.MarkInternal(upstream)
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	if r.Method == http.MethodPut {
		upstream.Header.Set("Content-Type", "application/json")
		if ownerActHeader != "" {
			upstream.Header.Set(owneract.Header, ownerActHeader)
		}
	}
	response, err := h.corpusd.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "self-development authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid self-development response"})
		return
	}
	if response.Header.Get("Content-Type") != "" {
		w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

// mintModeOwnerAct signs the owner-act receipt for a mode write the proxy
// has just authorized: act selfdev.approve for an accept_once arm (the
// owner's approval of one operation and bundle), selfdev.mode.set
// otherwise, over the write's request commitment. With no key loaded it
// returns "" and the write goes without a receipt, as before S1.
//
// P4: arming standing_consensus (act selfdev.autonomy.arm) is allowed only
// on a test-tier computer. The proxy reads the tier from vmctl, the same
// authority that proves ownership, refuses any other tier, and signs
// computer_tier into the act so corpusd can require it.
func (h *Handler) mintModeOwnerAct(ctx context.Context, authResult *AuthResult, computerID string, body []byte) (string, error) {
	var request platform.SetSelfDevelopmentModeRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return "", fmt.Errorf("invalid mode request")
	}
	tier := ""
	if strings.TrimSpace(request.Mode) == platform.SelfDevelopmentModeStandingConsensus {
		if h.vmctlClient == nil {
			return "", fmt.Errorf("%w: computer tier authority unavailable", errStandingAutonomyTier)
		}
		ownership, err := h.vmctlClient.LookupComputerContext(ctx, authResult.UserID, computerID)
		if err != nil || ownership == nil || ownership.ComputerID != computerID {
			return "", fmt.Errorf("%w: computer tier unavailable", errStandingAutonomyTier)
		}
		if ownership.WarmnessClass != string(vmctl.WarmnessClassTest) {
			return "", fmt.Errorf("%w: computer %s is %q", errStandingAutonomyTier, computerID, ownership.WarmnessClass)
		}
		tier = platform.ComputerTierTest
	}
	if h.ownerActSigner == nil {
		return "", nil
	}
	subject, err := platform.OwnerActModeSubjectWithTier(computerID, request, tier)
	if err != nil {
		return "", fmt.Errorf("invalid mode request")
	}
	method := owneract.AuthMethodSession
	if authResult.AuthMethod == "api_key" {
		method = owneract.AuthMethodAPIKey
	}
	receipt, err := h.ownerActSigner.Mint(owneract.Act{
		Kind: platform.OwnerActKindForMode(request.Mode), OwnerUserID: authResult.UserID,
		AuthMethod: method, CredentialRef: authResult.CredentialRef, ComputerID: computerID, Subject: subject,
	}, time.Now())
	if err != nil {
		return "", fmt.Errorf("owner act receipt: %w", err)
	}
	return owneract.Encode(receipt)
}

const replayCompletenessWriteGrace = time.Second

// extendReplayCompletenessWriteDeadline gives the owner-only replay probe a
// route-local response budget. The proxy server's default write deadline must
// remain short for ordinary interactive routes; ResponseController lets this
// one long-running route extend its connection deadline without weakening that
// global fail-fast boundary. Test response writers may not support deadlines,
// so an unsupported writer is left at its existing deadline.
func (h *Handler) extendReplayCompletenessWriteDeadline(w http.ResponseWriter) {
	timeout := DefaultReplayCompletenessTimeout
	if h != nil && h.replayAutoputerHTTP != nil && h.replayAutoputerHTTP.Timeout > 0 {
		timeout = h.replayAutoputerHTTP.Timeout
	} else if h != nil && h.cfg != nil && h.cfg.ReplayCompletenessTimeout > 0 {
		timeout = h.cfg.ReplayCompletenessTimeout
	}
	if timeout <= 0 {
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeout + replayCompletenessWriteGrace))
}

func (h *Handler) HandleSelfDevelopmentReplayCompleteness(w http.ResponseWriter, r *http.Request) {
	computerID, ok := selfDevelopmentReplayCompletenessComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:self_development:read"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
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
		logComputerAuthorityFailure(r, computerID, target, err)
		writeAutoputerResolveFailure(w, http.StatusBadGateway, "computer authority unavailable", err)
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build replay completeness request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build replay completeness request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid replay completeness request"})
		return
	}
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		upstream.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		upstream.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.replayAutoputerHTTP
	if client == nil {
		client = &http.Client{Timeout: DefaultReplayCompletenessTimeout}
	}
	h.extendReplayCompletenessWriteDeadline(w)
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "replay completeness authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid replay completeness response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func (h *Handler) HandleSelfDevelopmentGuest(w http.ResponseWriter, r *http.Request) {
	computerID, action, ok := selfDevelopmentGuestRoute(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if !selfDevelopmentGuestMethodAllowed(r.Method, action) {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if action == "adoptions" {
		action = "read"
		if r.Method == http.MethodPost {
			action = "adopt"
		}
	}
	requiredScope := "computer:self_development:" + action
	if action == "rebase-retry" {
		// An approval-class act: the approve scope plus the owner-act gate below.
		requiredScope = "computer:self_development:approve"
	}
	if action == "adopt" {
		// Adopting a release is an approval-class act (P8): the approve
		// scope plus the owner-act gate below.
		requiredScope = releasePublishScope
	}
	if authResult.AuthMethod == "api_key" {
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
			return
		}
	}
	switch action {
	case "approve":
		// A rejection withdraws a candidate and is fail-safe, so an agent
		// key may send it; every other decision is the owner's act.
		reject, err := selfDevelopmentDecisionIsReject(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development decision"})
			return
		}
		if !reject && !authorizeOwnerAct(w, authResult, "self-development approval") {
			return
		}
	case "rollback":
		if !authorizeOwnerAct(w, authResult, "self-development rollback") {
			return
		}
	case "adopt":
		if !authorizeOwnerAct(w, authResult, "release adoption") {
			return
		}
	case "rebase-retry":
		if !authorizeOwnerAct(w, authResult, "rebase retry") {
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
		// Reads wake the joined computer; approve, rollback, adopt and
		// rebase-retry are owner acts and never do.
		if action == "read" {
			autoputerURL, err = h.resolveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
		} else {
			autoputerURL, err = h.resolveActiveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
		}
	} else if target.State == "active" && strings.TrimSpace(target.ComputerURL) != "" {
		autoputerURL = target.ComputerURL
	} else {
		err = fmt.Errorf("computer realization is not active")
	}
	if err != nil {
		logComputerAuthorityFailure(r, computerID, target, err)
		writeAutoputerResolveFailure(w, http.StatusBadGateway, "computer authority unavailable", err)
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	maxBody := int64(64 << 10)
	if action == "approve" || action == "adopt" {
		maxBody = 1 << 20
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
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
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "self-development authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid self-development response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
