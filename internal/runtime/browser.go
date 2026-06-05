package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type browserCapabilitiesResponse struct {
	Provider              string          `json:"provider"`
	Mode                  string          `json:"mode"`
	Substrate             string          `json:"substrate"`
	Available             bool            `json:"available"`
	Configured            bool            `json:"configured"`
	Status                string          `json:"status"`
	Binary                string          `json:"binary,omitempty"`
	Supports              map[string]bool `json:"supports"`
	LegacyIframeAvailable bool            `json:"legacy_iframe_available"`
}

type browserSessionCreateRequest struct {
	InitialURL string `json:"initial_url,omitempty"`
}

type browserNavigateRequest struct {
	URL string `json:"url"`
}

type browserControlRequest struct {
	Action   string `json:"action"`
	Selector string `json:"selector"`
	Value    string `json:"value,omitempty"`
}

type browserControlResponse struct {
	Session types.BrowserSessionRecord `json:"session"`
	Control browserControlResult       `json:"control"`
}

type browserControlResult struct {
	OK           bool   `json:"ok"`
	Action       string `json:"action"`
	Selector     string `json:"selector"`
	Value        string `json:"value,omitempty"`
	Text         string `json:"text,omitempty"`
	DocumentText string `json:"document_text,omitempty"`
	Error        string `json:"error,omitempty"`
}

type browserSnapshotResult struct {
	Text             string
	HTML             string
	Links            []types.BrowserLink
	ScreenshotPNG    string
	Warnings         []string
	ExecutionScope   string
	BackendSessionID string
}

const maxBrowserSnapshotLinks = 100
const maxBrowserScreenshotBase64 = 2 * 1024 * 1024
const maxBrowserControlText = 500
const browserTextSnapshotTimeout = 30 * time.Second
const browserOptionalSnapshotTimeout = 5 * time.Second

func (h *APIHandler) HandleBrowserCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	writeAPIJSON(w, http.StatusOK, h.rt.BrowserCapabilities())
}

func (h *APIHandler) HandleBrowserSessionsRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		sessions, err := h.rt.Store().ListBrowserSessions(r.Context(), ownerID, 50)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list browser sessions"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	case http.MethodPost:
		var req browserSessionCreateRequest
		if r.Body != nil {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil && err.Error() != "EOF" {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid browser session request"})
				return
			}
		}
		currentURL := ""
		if strings.TrimSpace(req.InitialURL) != "" {
			currentURL, err = normalizeHTTPURL(req.InitialURL)
			if err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
		}
		caps := h.rt.BrowserCapabilities()
		state := types.BrowserSessionIdle
		sessionErr := ""
		if !caps.Available {
			state = types.BrowserSessionUnavailable
			sessionErr = caps.Status
		}
		rec := types.BrowserSessionRecord{
			OwnerID:        ownerID,
			Provider:       caps.Provider,
			Mode:           caps.Mode,
			ExecutionScope: browserExecutionScope(caps),
			WorldKind:      "foreground",
			State:          state,
			CurrentURL:     currentURL,
			Error:          sessionErr,
		}
		rec, err := h.rt.Store().CreateBrowserSession(r.Context(), rec)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create browser session"})
			return
		}
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserSessionCreated, "session", map[string]any{
			"configured": caps.Configured,
			"available":  caps.Available,
			"status":     caps.Status,
		})
		writeAPIJSON(w, http.StatusCreated, rec)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandleBrowserSessionRouter(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/browser/sessions/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if rest == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
		return
	}
	parts := strings.Split(rest, "/")
	sessionID := strings.TrimSpace(parts[0])
	if sessionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
		return
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		rec, err := h.rt.Store().GetBrowserSession(r.Context(), ownerID, sessionID)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load browser session"})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
		return
	}
	if len(parts) == 2 && parts[1] == "navigate" {
		h.HandleBrowserSessionNavigate(w, r, ownerID, sessionID)
		return
	}
	if len(parts) == 2 && parts[1] == "control" {
		h.HandleBrowserSessionControl(w, r, ownerID, sessionID)
		return
	}
	if len(parts) == 2 && parts[1] == "close" {
		h.HandleBrowserSessionClose(w, r, ownerID, sessionID)
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session route not found"})
}

func (h *APIHandler) HandleBrowserSessionNavigate(w http.ResponseWriter, r *http.Request, ownerID, sessionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req browserNavigateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid browser navigate request"})
		return
	}
	targetURL, err := normalizeHTTPURL(req.URL)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	rec, err := h.rt.Store().GetBrowserSession(r.Context(), ownerID, sessionID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load browser session"})
		return
	}
	if rec.State == types.BrowserSessionClosed {
		rec.Error = "browser session is closed"
		writeAPIJSON(w, http.StatusConflict, rec)
		return
	}
	unlock := h.rt.lockBrowserOperation(sessionID)
	defer unlock()

	caps := h.rt.BrowserCapabilities()
	rec.Provider = caps.Provider
	rec.Mode = caps.Mode
	rec.ExecutionScope = browserExecutionScope(caps)
	rec.CurrentURL = targetURL
	if !caps.Available {
		rec.State = types.BrowserSessionUnavailable
		rec.Error = caps.Status
		rec.UpdatedAt = time.Now().UTC()
		updated, updateErr := h.rt.Store().UpdateBrowserSession(r.Context(), rec)
		if updateErr == nil {
			rec = updated
		}
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserNavigationFailed, "navigate", map[string]any{
			"url":    targetURL,
			"error":  rec.Error,
			"status": caps.Status,
		})
		writeAPIJSON(w, http.StatusServiceUnavailable, rec)
		return
	}
	snapshot, err := h.rt.fetchBrowserSnapshots(r.Context(), rec.SessionID, targetURL)
	if err != nil {
		rec.State = types.BrowserSessionError
		rec.Error = err.Error()
		rec.TextSnapshot = ""
		rec.HTMLSnapshot = ""
		rec.Links = nil
		rec.ScreenshotPNG = ""
		rec.UpdatedAt = time.Now().UTC()
		updated, updateErr := h.rt.Store().UpdateBrowserSession(r.Context(), rec)
		if updateErr == nil {
			rec = updated
		}
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserNavigationFailed, "navigate", map[string]any{
			"url":   targetURL,
			"error": rec.Error,
		})
		writeAPIJSON(w, http.StatusBadGateway, rec)
		return
	}
	rec.State = types.BrowserSessionReady
	rec.Title = browserSnapshotTitle(snapshot.Text, targetURL)
	rec.TextSnapshot = snapshot.Text
	rec.HTMLSnapshot = snapshot.HTML
	rec.Links = snapshot.Links
	rec.ScreenshotPNG = snapshot.ScreenshotPNG
	rec.SnapshotWarnings = snapshot.Warnings
	rec.ExecutionScope = snapshot.ExecutionScope
	rec.BackendSessionID = snapshot.BackendSessionID
	rec.Error = ""
	rec.UpdatedAt = time.Now().UTC()
	rec, err = h.rt.Store().UpdateBrowserSession(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update browser session"})
		return
	}
	h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserNavigationCompleted, "snapshot", map[string]any{
		"url":                    targetURL,
		"title":                  rec.Title,
		"text_snapshot_bytes":    len(rec.TextSnapshot),
		"html_snapshot_bytes":    len(rec.HTMLSnapshot),
		"text_snapshot_excerpt":  browserSnapshotExcerpt(rec.TextSnapshot),
		"links_count":            len(rec.Links),
		"screenshot_png_bytes":   browserScreenshotBytes(rec.ScreenshotPNG),
		"snapshot_warning_count": len(rec.SnapshotWarnings),
		"snapshot_warnings":      rec.SnapshotWarnings,
		"execution_scope":        rec.ExecutionScope,
		"backend_session_id":     rec.BackendSessionID,
	})
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) HandleBrowserSessionControl(w http.ResponseWriter, r *http.Request, ownerID, sessionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req browserControlRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid browser control request"})
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.Selector = strings.TrimSpace(req.Selector)
	if req.Action != "fill" && req.Action != "click" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "browser control action must be fill or click"})
		return
	}
	if req.Selector == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "browser control selector is required"})
		return
	}
	rec, err := h.rt.Store().GetBrowserSession(r.Context(), ownerID, sessionID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load browser session"})
		return
	}
	if rec.State == types.BrowserSessionClosed {
		rec.Error = "browser session is closed"
		writeAPIJSON(w, http.StatusConflict, rec)
		return
	}
	caps := h.rt.BrowserCapabilities()
	if !caps.Supports["bounded_input"] {
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserControlFailed, "control", map[string]any{
			"action":   req.Action,
			"selector": req.Selector,
			"error":    "bounded input/control is unavailable",
		})
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "bounded input/control is unavailable; enable CHOIR_OBSCURA_CDP_SCREENSHOTS"})
		return
	}
	if rec.BackendSessionID == "" {
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserControlFailed, "control", map[string]any{
			"action":   req.Action,
			"selector": req.Selector,
			"error":    "backend CDP session is not active",
		})
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "backend CDP session is not active; navigate first"})
		return
	}
	unlock := h.rt.lockBrowserOperation(sessionID)
	defer unlock()

	control, screenshotPNG, backendSessionID, err := h.rt.controlBrowserCDPSession(r.Context(), sessionID, req.Action, req.Selector, req.Value)
	if err != nil {
		rec.State = types.BrowserSessionError
		rec.Error = err.Error()
		rec.UpdatedAt = time.Now().UTC()
		updated, updateErr := h.rt.Store().UpdateBrowserSession(r.Context(), rec)
		if updateErr == nil {
			rec = updated
		}
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserControlFailed, "control", map[string]any{
			"action":   req.Action,
			"selector": req.Selector,
			"error":    rec.Error,
		})
		writeAPIJSON(w, http.StatusBadGateway, browserControlResponse{Session: rec, Control: browserControlResult{
			OK:       false,
			Action:   req.Action,
			Selector: req.Selector,
			Error:    rec.Error,
		}})
		return
	}
	if !control.OK {
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserControlFailed, "control", map[string]any{
			"action":   req.Action,
			"selector": req.Selector,
			"error":    control.Error,
		})
		writeAPIJSON(w, http.StatusUnprocessableEntity, browserControlResponse{Session: rec, Control: control})
		return
	}
	rec.State = types.BrowserSessionReady
	rec.ScreenshotPNG = screenshotPNG
	rec.BackendSessionID = backendSessionID
	rec.Error = ""
	rec.UpdatedAt = time.Now().UTC()
	rec, err = h.rt.Store().UpdateBrowserSession(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update browser session"})
		return
	}
	h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserControlCompleted, "control", map[string]any{
		"action":               control.Action,
		"selector":             control.Selector,
		"value":                control.Value,
		"text":                 control.Text,
		"document_text":        control.DocumentText,
		"screenshot_png_bytes": browserScreenshotBytes(rec.ScreenshotPNG),
		"execution_scope":      rec.ExecutionScope,
		"backend_session_id":   rec.BackendSessionID,
		"bounded_control":      true,
	})
	writeAPIJSON(w, http.StatusOK, browserControlResponse{Session: rec, Control: control})
}

func (h *APIHandler) HandleBrowserSessionClose(w http.ResponseWriter, r *http.Request, ownerID, sessionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	rec, err := h.rt.Store().GetBrowserSession(r.Context(), ownerID, sessionID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "browser session not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load browser session"})
		return
	}
	unlock := h.rt.lockBrowserOperation(sessionID)
	defer unlock()

	wasClosed := rec.State == types.BrowserSessionClosed
	cdpClosed := h.rt.closeBrowserCDPSession(sessionID)
	rec.State = types.BrowserSessionClosed
	rec.Error = ""
	rec.UpdatedAt = time.Now().UTC()
	rec, err = h.rt.Store().UpdateBrowserSession(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to close browser session"})
		return
	}
	if !wasClosed {
		h.rt.emitBrowserSessionEvent(r.Context(), rec, types.EventBrowserSessionClosed, "session", map[string]any{
			"closed":             true,
			"cdp_session_closed": cdpClosed,
		})
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (rt *Runtime) emitBrowserSessionEvent(ctx context.Context, rec types.BrowserSessionRecord, kind types.EventKind, phase string, extra map[string]any) {
	payload := map[string]any{
		"session_id":         rec.SessionID,
		"provider":           rec.Provider,
		"mode":               rec.Mode,
		"state":              rec.State,
		"current_url":        rec.CurrentURL,
		"title":              rec.Title,
		"world_kind":         rec.WorldKind,
		"vm_id":              rec.VMID,
		"snapshot_id":        rec.SnapshotID,
		"source_loop_id":     rec.SourceRunID,
		"candidate_trace_id": rec.CandidateTraceID,
	}
	for key, value := range extra {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = json.RawMessage(`{}`)
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		AgentID:      "browser",
		ChannelID:    browserSessionTraceID(rec.SessionID),
		OwnerID:      rec.OwnerID,
		TrajectoryID: browserSessionTraceID(rec.SessionID),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Phase:        phase,
		Payload:      raw,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorHost,
		Cause:  events.CauseHostAction,
	})
}

func (rt *Runtime) BrowserCapabilities() browserCapabilitiesResponse {
	provider := "obscura"
	path := strings.TrimSpace(rt.cfg.ObscuraPath)
	resp := browserCapabilitiesResponse{
		Provider:              provider,
		Mode:                  "legacy_iframe",
		Substrate:             "frontend_iframe",
		Available:             false,
		Configured:            path != "",
		Status:                "not_configured",
		Supports:              browserSupports(false),
		LegacyIframeAvailable: true,
	}
	if path == "" {
		return resp
	}
	resp.Binary = filepath.Base(path)
	resolved, err := resolveExecutable(path)
	if err != nil {
		resp.Status = "missing"
		return resp
	}
	resp.Binary = filepath.Base(resolved)
	resp.Mode = "backend"
	resp.Substrate = "obscura_cli_fetch"
	resp.Available = true
	resp.Status = "ready"
	resp.Supports = browserSupports(true)
	if rt.cfg.ObscuraCDPScreenshots {
		resp.Substrate = "obscura_cli_fetch+obscura_cdp_screenshot"
		resp.Supports["screenshot"] = true
		resp.Supports["cdp_screenshot"] = true
		resp.Supports["bounded_input"] = true
		resp.Supports["fill"] = true
		resp.Supports["click"] = true
	}
	return resp
}

func (rt *Runtime) lockBrowserOperation(browserSessionID string) func() {
	browserSessionID = strings.TrimSpace(browserSessionID)
	if browserSessionID == "" {
		return func() {}
	}
	rt.browserOpMu.Lock()
	if rt.browserOps == nil {
		rt.browserOps = make(map[string]*sync.Mutex)
	}
	lock := rt.browserOps[browserSessionID]
	if lock == nil {
		lock = &sync.Mutex{}
		rt.browserOps[browserSessionID] = lock
	}
	rt.browserOpMu.Unlock()

	lock.Lock()
	return lock.Unlock
}

func browserSupports(snapshot bool) map[string]bool {
	return map[string]bool{
		"navigate":       snapshot,
		"text":           snapshot,
		"html":           snapshot,
		"links":          snapshot,
		"markdown":       false,
		"screenshot":     false,
		"cdp_screenshot": false,
		"bounded_input":  false,
		"fill":           false,
		"click":          false,
		"input":          false,
		"cdp":            false,
	}
}

func browserExecutionScope(caps browserCapabilitiesResponse) string {
	if caps.Available && caps.Mode == "backend" {
		return "host_process"
	}
	if caps.LegacyIframeAvailable {
		return "frontend"
	}
	return ""
}

func resolveExecutable(path string) (string, error) {
	if strings.Contains(path, string(os.PathSeparator)) {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", os.ErrPermission
		}
		if info.Mode()&0o111 == 0 {
			return "", os.ErrPermission
		}
		return path, nil
	}
	return exec.LookPath(path)
}

func (rt *Runtime) fetchBrowserSnapshots(ctx context.Context, browserSessionID, targetURL string) (browserSnapshotResult, error) {
	resolved, err := resolveExecutable(strings.TrimSpace(rt.cfg.ObscuraPath))
	if err != nil {
		return browserSnapshotResult{}, fmt.Errorf("backend browser unavailable: %w", err)
	}
	result := browserSnapshotResult{ExecutionScope: "host_process"}
	text, err := runObscuraFetchDump(ctx, resolved, targetURL, "text", browserTextSnapshotTimeout)
	if err != nil {
		return browserSnapshotResult{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return browserSnapshotResult{}, fmt.Errorf("backend browser text snapshot was empty")
	}
	linksRaw, err := runObscuraFetchDump(ctx, resolved, targetURL, "links", browserOptionalSnapshotTimeout)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}
	html, err := runObscuraFetchDump(ctx, resolved, targetURL, "html", browserOptionalSnapshotTimeout)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}
	screenshotPNG := ""
	backendSessionID := ""
	if rt.cfg.ObscuraCDPScreenshots {
		screenshotPNG, backendSessionID, err = rt.captureBrowserCDPScreenshot(ctx, browserSessionID, resolved, targetURL)
		if err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		}
	}
	if len(text) > maxStoredExtractedText {
		text = text[:maxStoredExtractedText]
	}
	html = strings.TrimSpace(html)
	if len(html) > maxStoredExtractedText {
		html = html[:maxStoredExtractedText]
	}
	result.Text = text
	result.HTML = html
	if strings.TrimSpace(linksRaw) != "" {
		result.Links = parseBrowserLinks(linksRaw)
	}
	result.ScreenshotPNG = screenshotPNG
	result.BackendSessionID = backendSessionID
	return result, nil
}

type obscuraCDPVersionEndpoint struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpMessage struct {
	ID        int             `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Message string `json:"message"`
}

type browserCDPSession struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	conn      *websocket.Conn
	sessionID string
	nextID    int
	logs      *bytes.Buffer
	closed    bool
}

func captureObscuraCDPScreenshot(ctx context.Context, resolved, targetURL string) (string, error) {
	session, err := startObscuraCDPSession(ctx, resolved)
	if err != nil {
		return "", err
	}
	defer session.close()
	return session.navigateAndScreenshot(ctx, targetURL)
}

func startObscuraCDPSession(ctx context.Context, resolved string) (*browserCDPSession, error) {
	port, err := pickLocalBrowserPort()
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())

	logs := &bytes.Buffer{}
	cmd := exec.CommandContext(runCtx, resolved, "serve", "--port", strconv.Itoa(port), "--workers", "1")
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("backend browser cdp start failed: %w", err)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			stopObscuraCDPProcess(cmd, cancel)
		}
	}()

	versionEndpoint, err := waitForObscuraCDPVersionEndpoint(ctx, port, logs)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.DialContext(ctx, versionEndpoint.WebSocketDebuggerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("backend browser cdp dial failed: %w", err)
	}
	defer func() {
		if closeOnError {
			_ = conn.Close()
		}
	}()

	seen := map[string]bool{}
	id := 1
	createTargetResult, err := cdpCall(ctx, conn, id, "", "Target.createTarget", map[string]any{"url": "about:blank"}, seen)
	if err != nil {
		return nil, err
	}
	id++
	var createTargetPayload struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(createTargetResult, &createTargetPayload); err != nil {
		return nil, fmt.Errorf("backend browser cdp target decode failed: %w", err)
	}
	if strings.TrimSpace(createTargetPayload.TargetID) == "" {
		return nil, fmt.Errorf("backend browser cdp target id was empty")
	}
	attachResult, err := cdpCall(ctx, conn, id, "", "Target.attachToTarget", map[string]any{
		"targetId": createTargetPayload.TargetID,
		"flatten":  true,
	}, seen)
	if err != nil {
		return nil, err
	}
	id++
	var attachPayload struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(attachResult, &attachPayload); err != nil {
		return nil, fmt.Errorf("backend browser cdp attach decode failed: %w", err)
	}
	sessionID := strings.TrimSpace(attachPayload.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("backend browser cdp session id was empty")
	}
	session := &browserCDPSession{
		cmd:       cmd,
		cancel:    cancel,
		conn:      conn,
		sessionID: sessionID,
		nextID:    id,
		logs:      logs,
	}
	for _, call := range []struct {
		method string
		params map[string]any
	}{
		{method: "Page.enable"},
		{method: "Runtime.enable"},
	} {
		if _, err := session.callLocked(ctx, call.method, call.params, seen); err != nil {
			return nil, err
		}
	}
	closeOnError = false
	return session, nil
}

func (session *browserCDPSession) navigateAndScreenshot(ctx context.Context, targetURL string) (string, error) {
	if session == nil {
		return "", fmt.Errorf("backend browser cdp session unavailable")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.conn == nil {
		return "", fmt.Errorf("backend browser cdp session is closed")
	}

	seen := map[string]bool{}
	if _, err := session.callLocked(ctx, "Page.navigate", map[string]any{"url": targetURL}, seen); err != nil {
		return "", err
	}
	if !seen["Page.loadEventFired"] && !seen["Page.frameStoppedLoading"] {
		_ = waitForCDPEvent(ctx, session.conn, seen, 2*time.Second, "Page.loadEventFired", "Page.frameStoppedLoading")
	}
	return session.captureScreenshotLocked(ctx, seen)
}

func (session *browserCDPSession) controlAndScreenshot(ctx context.Context, action, selector, value string) (browserControlResult, string, error) {
	if session == nil {
		return browserControlResult{}, "", fmt.Errorf("backend browser cdp session unavailable")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.conn == nil {
		return browserControlResult{}, "", fmt.Errorf("backend browser cdp session is closed")
	}
	control, seen, err := session.applyControlLocked(ctx, action, selector, value)
	if err != nil {
		return browserControlResult{}, "", err
	}
	if !control.OK {
		return control, "", nil
	}
	screenshot, err := session.captureScreenshotLocked(ctx, seen)
	if err != nil {
		return browserControlResult{}, "", err
	}
	return control, screenshot, nil
}

func (session *browserCDPSession) applyControlLocked(ctx context.Context, action, selector, value string) (browserControlResult, map[string]bool, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	selector = strings.TrimSpace(selector)
	seen := map[string]bool{}
	expression, err := browserControlExpression(action, selector, value)
	if err != nil {
		return browserControlResult{}, seen, err
	}
	result, err := session.callLocked(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	}, seen)
	if err != nil {
		return browserControlResult{}, seen, err
	}
	var payload struct {
		Result struct {
			Value browserControlResult `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return browserControlResult{}, seen, fmt.Errorf("backend browser control result decode failed: %w", err)
	}
	payload.Result.Value.Action = action
	payload.Result.Value.Selector = selector
	payload.Result.Value.Text = traceExcerpt(payload.Result.Value.Text, maxBrowserControlText)
	payload.Result.Value.DocumentText = traceExcerpt(payload.Result.Value.DocumentText, maxBrowserControlText)
	return payload.Result.Value, seen, nil
}

func (session *browserCDPSession) captureScreenshotLocked(ctx context.Context, seen map[string]bool) (string, error) {
	if seen == nil {
		seen = map[string]bool{}
	}
	result, err := session.callLocked(ctx, "Page.captureScreenshot", map[string]any{
		"format":                "png",
		"captureBeyondViewport": true,
	}, seen)
	if err != nil {
		return "", err
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return "", fmt.Errorf("backend browser cdp screenshot decode failed: %w", err)
	}
	payload.Data = strings.TrimSpace(payload.Data)
	if payload.Data == "" {
		return "", fmt.Errorf("backend browser cdp screenshot was empty")
	}
	if len(payload.Data) > maxBrowserScreenshotBase64 {
		return "", fmt.Errorf("backend browser cdp screenshot too large")
	}
	if _, err := base64.StdEncoding.DecodeString(payload.Data); err != nil {
		return "", fmt.Errorf("backend browser cdp screenshot was not valid base64: %w", err)
	}
	return payload.Data, nil
}

func browserControlExpression(action, selector, value string) (string, error) {
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return "", err
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	switch action {
	case "fill":
		return fmt.Sprintf(`(function() {
  var selector = %s;
  var value = %s;
  var el = document.querySelector(selector);
  if (!el) return { ok: false, action: "fill", selector: selector, error: "selector not found" };
  if (!("value" in el)) return { ok: false, action: "fill", selector: selector, error: "element is not fillable", text: String(el.textContent || "") };
  el.focus && el.focus();
  el.value = value;
  el.setAttribute && el.setAttribute("value", value);
  el.dispatchEvent && el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent && el.dispatchEvent(new Event("change", { bubbles: true }));
  return { ok: true, action: "fill", selector: selector, value: String(el.value || ""), text: String(el.textContent || ""), document_text: String((document.body && document.body.textContent) || "") };
})()`, string(selectorJSON), string(valueJSON)), nil
	case "click":
		return fmt.Sprintf(`(function() {
  var selector = %s;
  var el = document.querySelector(selector);
  if (!el) return { ok: false, action: "click", selector: selector, error: "selector not found" };
  el.scrollIntoView && el.scrollIntoView();
  el.focus && el.focus();
  if (typeof el.click === "function") {
    el.click();
  } else if (el.dispatchEvent) {
    el.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  }
  return { ok: true, action: "click", selector: selector, text: String(el.textContent || el.value || ""), document_text: String((document.body && document.body.textContent) || "") };
})()`, string(selectorJSON)), nil
	default:
		return "", fmt.Errorf("unsupported browser control action %q", action)
	}
}

func (session *browserCDPSession) callLocked(ctx context.Context, method string, params map[string]any, seen map[string]bool) (json.RawMessage, error) {
	id := session.nextID
	session.nextID++
	return cdpCall(ctx, session.conn, id, session.sessionID, method, params, seen)
}

func (session *browserCDPSession) close() bool {
	if session == nil {
		return false
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return false
	}
	session.closed = true
	conn := session.conn
	cmd := session.cmd
	cancel := session.cancel
	session.conn = nil
	session.cmd = nil
	session.cancel = nil
	session.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if cmd != nil && cancel != nil {
		stopObscuraCDPProcess(cmd, cancel)
	}
	return true
}

func (session *browserCDPSession) id() string {
	if session == nil {
		return ""
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.sessionID
}

func (rt *Runtime) captureBrowserCDPScreenshot(ctx context.Context, browserSessionID, resolved, targetURL string) (string, string, error) {
	browserSessionID = strings.TrimSpace(browserSessionID)
	if browserSessionID == "" {
		screenshot, err := captureObscuraCDPScreenshot(ctx, resolved, targetURL)
		return screenshot, "", err
	}
	session, err := rt.ensureBrowserCDPSession(ctx, browserSessionID, resolved)
	if err != nil {
		return "", "", err
	}
	screenshot, err := session.navigateAndScreenshot(ctx, targetURL)
	if err != nil {
		rt.closeBrowserCDPSession(browserSessionID)
		return "", "", err
	}
	return screenshot, session.id(), nil
}

func (rt *Runtime) controlBrowserCDPSession(ctx context.Context, browserSessionID, action, selector, value string) (browserControlResult, string, string, error) {
	browserSessionID = strings.TrimSpace(browserSessionID)
	if browserSessionID == "" {
		return browserControlResult{}, "", "", fmt.Errorf("browser session id is required")
	}
	session := rt.getBrowserCDPSession(browserSessionID)
	if session == nil {
		return browserControlResult{}, "", "", fmt.Errorf("backend CDP session is not active")
	}
	control, screenshot, err := session.controlAndScreenshot(ctx, action, selector, value)
	if err != nil {
		rt.closeBrowserCDPSession(browserSessionID)
		return browserControlResult{}, "", "", err
	}
	if !control.OK {
		return control, "", session.id(), nil
	}
	return control, screenshot, session.id(), nil
}

func (rt *Runtime) getBrowserCDPSession(browserSessionID string) *browserCDPSession {
	rt.browserCDPMu.Lock()
	defer rt.browserCDPMu.Unlock()
	return rt.browserCDP[strings.TrimSpace(browserSessionID)]
}

func (rt *Runtime) ensureBrowserCDPSession(ctx context.Context, browserSessionID, resolved string) (*browserCDPSession, error) {
	rt.browserCDPMu.Lock()
	defer rt.browserCDPMu.Unlock()
	if rt.browserCDP == nil {
		rt.browserCDP = make(map[string]*browserCDPSession)
	}
	if session := rt.browserCDP[browserSessionID]; session != nil {
		return session, nil
	}
	session, err := startObscuraCDPSession(ctx, resolved)
	if err != nil {
		return nil, err
	}
	rt.browserCDP[browserSessionID] = session
	return session, nil
}

func (rt *Runtime) closeBrowserCDPSession(browserSessionID string) bool {
	rt.browserCDPMu.Lock()
	session := rt.browserCDP[strings.TrimSpace(browserSessionID)]
	delete(rt.browserCDP, strings.TrimSpace(browserSessionID))
	rt.browserCDPMu.Unlock()
	return session.close()
}

func (rt *Runtime) closeAllBrowserCDPSessions() {
	rt.browserCDPMu.Lock()
	sessions := make([]*browserCDPSession, 0, len(rt.browserCDP))
	for key, session := range rt.browserCDP {
		sessions = append(sessions, session)
		delete(rt.browserCDP, key)
	}
	rt.browserCDPMu.Unlock()
	for _, session := range sessions {
		session.close()
	}
}

func pickLocalBrowserPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("pick backend browser cdp port: %w", err)
	}
	defer func() { _ = ln.Close() }()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("pick backend browser cdp port: unexpected addr %T", ln.Addr())
	}
	return addr.Port, nil
}

func waitForObscuraCDPVersionEndpoint(ctx context.Context, port int, logs *bytes.Buffer) (obscuraCDPVersionEndpoint, error) {
	client := http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return obscuraCDPVersionEndpoint{}, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
		if err != nil {
			return obscuraCDPVersionEndpoint{}, err
		}
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			var version obscuraCDPVersionEndpoint
			decodeErr := json.NewDecoder(resp.Body).Decode(&version)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decodeErr == nil && strings.TrimSpace(version.WebSocketDebuggerURL) != "" {
				return version, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return obscuraCDPVersionEndpoint{}, fmt.Errorf("backend browser cdp endpoint unavailable: %s", traceExcerpt(logs.String(), 500))
}

func cdpCall(ctx context.Context, conn *websocket.Conn, id int, sessionID, method string, params map[string]any, seen map[string]bool) (json.RawMessage, error) {
	req := map[string]any{"id": id, "method": method}
	if sessionID != "" {
		req["sessionId"] = sessionID
	}
	if len(params) > 0 {
		req["params"] = params
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("backend browser cdp %s write failed: %w", method, err)
	}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("backend browser cdp %s read failed: %w", method, err)
		}
		var msg cdpMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Method != "" {
			seen[msg.Method] = true
			continue
		}
		if msg.ID != id {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("backend browser cdp %s failed: %s", method, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func waitForCDPEvent(ctx context.Context, conn *websocket.Conn, seen map[string]bool, timeout time.Duration, methods ...string) error {
	deadline := time.Now().Add(timeout)
	wanted := map[string]bool{}
	for _, method := range methods {
		wanted[method] = true
		if seen[method] {
			return nil
		}
	}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			continue
		}
		var msg cdpMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Method != "" {
			seen[msg.Method] = true
			if wanted[msg.Method] {
				return nil
			}
		}
	}
	return fmt.Errorf("backend browser cdp load event timed out")
}

func stopObscuraCDPProcess(cmd *exec.Cmd, cancel context.CancelFunc) {
	if cmd == nil {
		return
	}
	cancel()
	if cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func runObscuraFetchDump(ctx context.Context, resolved, targetURL, dump string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = browserTextSnapshotTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, resolved, "fetch", targetURL, "--dump", dump, "--timeout", "15", "--quiet")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if runCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("backend browser %s fetch timed out", dump)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("backend browser %s fetch failed: %s", dump, msg)
	}
	return string(out), nil
}

func parseBrowserLinks(raw string) []types.BrowserLink {
	links := []types.BrowserLink{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		url, err := normalizeHTTPURL(parts[0])
		if err != nil {
			continue
		}
		text := ""
		if len(parts) == 2 {
			text = strings.Join(strings.Fields(strings.TrimSpace(parts[1])), " ")
		}
		links = append(links, types.BrowserLink{URL: url, Text: text})
		if len(links) >= maxBrowserSnapshotLinks {
			break
		}
	}
	return links
}

func browserSnapshotTitle(text, fallbackURL string) string {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 120 {
				return line[:120]
			}
			return line
		}
	}
	return fallbackURL
}

func browserSnapshotExcerpt(text string) string {
	excerpt := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(excerpt) > 240 {
		return excerpt[:240]
	}
	return excerpt
}

func browserSessionTraceID(sessionID string) string {
	return "browser:" + strings.TrimSpace(sessionID)
}

func browserScreenshotBytes(encoded string) int {
	if strings.TrimSpace(encoded) == "" {
		return 0
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return 0
	}
	return len(raw)
}
