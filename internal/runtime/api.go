package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// apiError is a JSON error envelope for API responses.
type apiError struct {
	Error string `json:"error"`
}

// runSubmitRequest is the JSON payload for POST /api/agent/loop.
type runSubmitRequest struct {
	Prompt   string         `json:"prompt"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// internalRunSubmitRequest is the service-to-service payload for starting a
// run inside another sandbox runtime, such as a background worker VM. It is not
// registered under /api/* and must never become browser-public.
type internalRunSubmitRequest struct {
	OwnerID  string         `json:"owner_id"`
	Prompt   string         `json:"prompt"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// promptBarSubmitRequest is the public product payload for POST
// /api/prompt-bar. Browser callers submit user intent only; runtime role,
// model, channel, trajectory, and orchestration metadata are assigned
// server-side.
type promptBarSubmitRequest struct {
	Text string `json:"text"`
}

type promptBarSubmitResponse struct {
	SubmissionID string         `json:"submission_id"`
	State        types.RunState `json:"state"`
	CreatedAt    string         `json:"created_at"`
	StatusURL    string         `json:"status_url"`
}

type promptBarSubmissionStatusResponse struct {
	SubmissionID string             `json:"submission_id"`
	State        types.RunState     `json:"state"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	FinishedAt   *string            `json:"finished_at,omitempty"`
	Decision     *conductorDecision `json:"decision,omitempty"`
	Error        string             `json:"error,omitempty"`
}

type runContinuationListResponse struct {
	Continuations []types.RunContinuationRecord `json:"continuations"`
}

type runContinuationSynthesizeRequest struct {
	SourceRunID string `json:"source_loop_id"`
	Start       bool   `json:"start,omitempty"`
}

type runAcceptanceSynthesizeRequest struct {
	TargetMissionID       string `json:"target_mission_id"`
	SourcePromptObjective string `json:"source_prompt_or_objective,omitempty"`
	TrajectoryID          string `json:"trajectory_id,omitempty"`
	RunID                 string `json:"loop_id,omitempty"`
	CIRunID               string `json:"ci_run_id,omitempty"`
	DeployRunID           string `json:"deploy_run_id,omitempty"`
	StagingURL            string `json:"staging_url,omitempty"`
}

type runAcceptanceListResponse struct {
	Acceptances []types.RunAcceptanceRecord `json:"acceptances"`
}

// spawnRequest is the JSON payload for POST /api/agent/spawn.
// It creates a child run linked to a parent, with an objective and optional
// constraints (VAL-CHOIR-001).
type spawnRequest struct {
	ParentID    string         `json:"parent_id"`
	Objective   string         `json:"objective"`
	Constraints map[string]any `json:"constraints,omitempty"`
}

// spawnResponse is the JSON response for POST /api/agent/spawn.
// It returns the child run handle with the parent linkage.
type spawnResponse struct {
	AgentID   string         `json:"agent_id"`
	RunID     string         `json:"loop_id"`
	ChannelID string         `json:"channel_id,omitempty"`
	ParentID  string         `json:"parent_id"`
	State     types.RunState `json:"state"`
	OwnerID   string         `json:"owner_id"`
	CreatedAt string         `json:"created_at"`
}

// cancelRequest is the JSON payload for POST /api/agent/cancel.
// It cancels a running or pending run (VAL-CHOIR-010).
type cancelRequest struct {
	RunID string `json:"loop_id"`
}

// cancelResponse is the JSON response for POST /api/agent/cancel.
type cancelResponse struct {
	RunID string         `json:"loop_id"`
	State types.RunState `json:"state"`
}

// runSubmitResponse is the JSON response for POST /api/agent/loop.
// It returns the stable run handle and initial lifecycle state
// (VAL-RUNTIME-003).
type runSubmitResponse struct {
	AgentID   string         `json:"agent_id"`
	RunID     string         `json:"loop_id"`
	ChannelID string         `json:"channel_id,omitempty"`
	State     types.RunState `json:"state"`
	OwnerID   string         `json:"owner_id"`
	CreatedAt string         `json:"created_at"`
}

// runStatusResponse is the JSON response for GET /api/agent/status.
// It returns the full run record correlated to the submitted handle
// (VAL-RUNTIME-004).
type runStatusResponse struct {
	AgentID      string         `json:"agent_id"`
	RunID        string         `json:"loop_id"`
	ChannelID    string         `json:"channel_id,omitempty"`
	ParentRunID  string         `json:"parent_loop_id,omitempty"`
	AgentProfile string         `json:"agent_profile,omitempty"`
	AgentRole    string         `json:"agent_role,omitempty"`
	OwnerID      string         `json:"owner_id"`
	SandboxID    string         `json:"sandbox_id"`
	State        types.RunState `json:"state"`
	Prompt       string         `json:"prompt"`
	Result       string         `json:"result,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	FinishedAt   *string        `json:"finished_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// runListResponse is the JSON response for GET /api/agent/loops.
// It returns recent runs owned by the authenticated user so debugging
// surfaces can group live events into runs and child delegations.
type runListResponse struct {
	Runs []runStatusResponse `json:"runs"`
}

// eventListResponse is the JSON response for GET /api/agent/events.
// It returns historical runtime events either for a specific run or for
// the authenticated owner across recent runs.
type eventListResponse struct {
	Events []types.EventRecord `json:"events"`
}

type internalRunEventAppendRequest struct {
	OwnerID string          `json:"owner_id"`
	Kind    types.EventKind `json:"kind"`
	Phase   string          `json:"phase,omitempty"`
	Payload map[string]any  `json:"payload,omitempty"`
}

type internalRunEventAppendResponse struct {
	Status  string          `json:"status"`
	EventID string          `json:"event_id"`
	Kind    types.EventKind `json:"kind"`
}

// channelMessageListResponse is the JSON response for GET /api/agent/channel-messages.
// It returns durable channel message bodies for a specific coordination channel.
type channelMessageListResponse struct {
	Messages []types.ChannelMessage `json:"messages"`
}

// runtimeHealthResponse is the JSON structure returned by GET /health.
// It reports runtime readiness for real run handling, and surfaces
// degraded state rather than hiding it behind a generic healthy response
// (VAL-RUNTIME-001). The active provider name is included so operators
// can distinguish real-provider paths from stub/canned paths.
type runtimeHealthResponse struct {
	Status          string                   `json:"status"`
	Service         string                   `json:"service"`
	SandboxID       string                   `json:"sandbox_id"`
	RuntimeHealth   types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns     int                      `json:"running_runs"`
	ResearcherCount int                      `json:"researcher_count"`
	ActiveProvider  string                   `json:"active_provider"`
	Build           buildinfo.Info           `json:"build"`
}

// runtimeTopologyResponse is the JSON structure returned by GET /api/agent/topology.
// It surfaces the configured orchestration shape so operators and UI surfaces
// can see how many researchers the microVM expects and what the current runtime
// fan-out looks like.
type runtimeTopologyResponse struct {
	SandboxID       string `json:"sandbox_id"`
	ResearcherCount int    `json:"researcher_count"`
	RunningRuns     int    `json:"running_runs"`
	ChannelCount    int    `json:"channel_count"`
	RuntimeHealth   string `json:"runtime_health"`
	ActiveProvider  string `json:"active_provider"`
}

// APIHandler provides HTTP handlers for the runtime API endpoints.
type APIHandler struct {
	rt *Runtime
}

// NewAPIHandler creates an APIHandler for the given runtime.
func NewAPIHandler(rt *Runtime) *APIHandler {
	return &APIHandler{rt: rt}
}

// authenticateUser extracts the authenticated user identity from the
// X-Authenticated-User header injected by the proxy. It returns an error if
// the header is missing, which provides defense-in-depth auth gating at the
// sandbox level (VAL-RUNTIME-002).
func authenticateUser(r *http.Request) (string, error) {
	user := r.Header.Get("X-Authenticated-User")
	if user == "" {
		return "", fmt.Errorf("missing authenticated user identity")
	}
	return user, nil
}

func authenticatedUserEmail(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Authenticated-Email"))
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(addr.Address))
}

func requireInternalRuntimeCaller(r *http.Request) error {
	if r.Header.Get("X-Internal-Caller") != "true" {
		return fmt.Errorf("missing internal caller marker")
	}
	return nil
}

// writeJSON writes a JSON response with the given status code.
func writeAPIJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("runtime api: json encode error: %v", err)
	}
}

func runStatusFromRecord(rec *types.RunRecord) runStatusResponse {
	if rec == nil {
		return runStatusResponse{}
	}
	var finishedAt *string
	if rec.FinishedAt != nil {
		s := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
		finishedAt = &s
	}
	return runStatusResponse{
		AgentID:      rec.AgentID,
		RunID:        rec.RunID,
		ChannelID:    rec.ChannelID,
		ParentRunID:  rec.ParentRunID,
		AgentProfile: rec.AgentProfile,
		AgentRole:    rec.AgentRole,
		OwnerID:      rec.OwnerID,
		SandboxID:    rec.SandboxID,
		State:        rec.State,
		Prompt:       rec.Prompt,
		Result:       rec.Result,
		Error:        rec.Error,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:    rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt:   finishedAt,
		Metadata:     rec.Metadata,
	}
}

// HandlePromptBar handles POST /api/prompt-bar. This is the browser-public
// product entrypoint for user intent. It creates the conductor run internally
// with server-owned role metadata.
func (h *APIHandler) HandlePromptBar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req promptBarSubmitRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid prompt-bar request"})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "text is required"})
		return
	}

	requestedApp := AgentProfileVText
	var contentSourceURL, contentMediaType, contentAppHint string
	if appHint, sourceURL, mediaType, ok := classifyPromptBarContentIntent(text); ok {
		requestedApp = appHint
		contentSourceURL = sourceURL
		contentMediaType = mediaType
		contentAppHint = appHint
	}

	metadata := map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          requestedApp,
		"seed_prompt":            text,
		"initial_document_title": buildInitialVTextTitle(text, ""),
		"submission_surface":     "prompt_bar",
	}
	if ownerEmail := authenticatedUserEmail(r); ownerEmail != "" {
		metadata[runMetadataOwnerEmail] = ownerEmail
	}
	if contentSourceURL != "" {
		metadata["content_source_url"] = contentSourceURL
		metadata["content_media_type"] = contentMediaType
		metadata["content_app_hint"] = contentAppHint
	}

	var rec *types.RunRecord
	if contentSourceURL != "" {
		decision := conductorDecision{
			Action:    "open_app",
			App:       contentAppHint,
			Title:     buildInitialVTextTitle(text, ""),
			SourceURL: contentSourceURL,
			MediaType: contentMediaType,
			AppHint:   contentAppHint,
		}
		rec, err = h.rt.completePromptBarDecisionRun(r.Context(), text, ownerID, metadata, decision)
	} else if requestedApp == AgentProfileVText {
		decision := conductorDecision{
			Action: "open_app",
			App:    AgentProfileVText,
			Title:  buildInitialVTextTitle(text, ""),
		}
		rec, err = h.rt.completePromptBarDecisionRun(r.Context(), text, ownerID, metadata, decision)
		if err == nil {
			if _, routeErr := h.rt.ensureConductorVTextRoute(r.Context(), rec, text, ""); routeErr != nil {
				log.Printf("runtime api: materialize prompt-bar vtext route: %v", routeErr)
				writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to prepare prompt"})
				return
			}
		}
	} else {
		rec, err = h.rt.StartRunWithMetadata(r.Context(), text, ownerID, metadata)
	}
	if err != nil {
		log.Printf("runtime api: submit prompt-bar intent: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit prompt"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, promptBarSubmitResponse{
		SubmissionID: rec.RunID,
		State:        rec.State,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		StatusURL:    "/api/prompt-bar/submissions/" + rec.RunID,
	})
}

// HandlePromptBarSubmission handles GET /api/prompt-bar/submissions/{id}.
// It returns product-level submission state without exposing raw run metadata,
// agent identity controls, or runtime mailboxes.
func (h *APIHandler) HandlePromptBarSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	const prefix = "/api/prompt-bar/submissions/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}
	submissionID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if submissionID == "" || strings.Contains(submissionID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}

	rec, err := h.rt.GetRun(r.Context(), submissionID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}
	if agentProfileForRun(rec) != AgentProfileConductor || metadataStringValue(rec.Metadata, "input_source") != "prompt_bar" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}

	var finishedAt *string
	if rec.FinishedAt != nil {
		s := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
		finishedAt = &s
	}

	resp := promptBarSubmissionStatusResponse{
		SubmissionID: rec.RunID,
		State:        rec.State,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:    rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt:   finishedAt,
		Error:        rec.Error,
	}
	if raw := strings.TrimSpace(rec.Result); raw != "" {
		var decision conductorDecision
		if err := json.Unmarshal([]byte(raw), &decision); err == nil && strings.TrimSpace(decision.Action) != "" {
			decision = fillConductorDecisionFromRun(rec, decision)
			resp.Decision = &decision
		}
	}
	if resp.Decision == nil && rec.State == types.RunCompleted {
		var decision conductorDecision
		if err := json.Unmarshal([]byte(normalizeConductorDecision(rec)), &decision); err == nil {
			resp.Decision = &decision
		}
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) HandleRunContinuationsRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		sourceRunID := strings.TrimSpace(r.URL.Query().Get("source_loop_id"))
		if sourceRunID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_loop_id is required"})
			return
		}
		if _, err := h.rt.GetRun(r.Context(), sourceRunID, ownerID); err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "source run not found"})
			return
		}
		continuations, err := h.rt.store.ListRunContinuationsBySource(r.Context(), ownerID, sourceRunID)
		if err != nil {
			log.Printf("runtime api: list continuations: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list continuations"})
			return
		}
		writeAPIJSON(w, http.StatusOK, runContinuationListResponse{Continuations: continuations})
	case http.MethodPost:
		var req runContinuationSynthesizeRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid continuation request"})
			return
		}
		sourceRunID := strings.TrimSpace(req.SourceRunID)
		if sourceRunID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_loop_id is required"})
			return
		}
		rec, err := h.rt.SelectSynthesizedRunContinuation(r.Context(), sourceRunID, ownerID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		if req.Start {
			rec, err = h.rt.StartRunContinuation(r.Context(), ownerID, rec.ContinuationID)
			if err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
		}
		writeAPIJSON(w, http.StatusAccepted, rec)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandleRunContinuationDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/continuations/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "continuation not found"})
		return
	}
	continuationID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		rec, err := h.rt.store.GetRunContinuation(r.Context(), ownerID, continuationID)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "continuation not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
		return
	}
	if len(parts) == 2 && parts[1] == "start" {
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		rec, err := h.rt.StartRunContinuation(r.Context(), ownerID, continuationID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusAccepted, rec)
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "continuation action not found"})
}

func (h *APIHandler) HandleRunAcceptancesRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid limit"})
			return
		}
		limit = parsed
	}
	trajectoryID := strings.TrimSpace(r.URL.Query().Get("trajectory_id"))
	var records []types.RunAcceptanceRecord
	if trajectoryID != "" {
		records, err = h.rt.store.ListRunAcceptancesByTrajectory(r.Context(), ownerID, trajectoryID, limit)
	} else {
		records, err = h.rt.store.ListRunAcceptances(r.Context(), ownerID, limit)
	}
	if err != nil {
		log.Printf("runtime api: list run acceptances: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list run acceptances"})
		return
	}
	writeAPIJSON(w, http.StatusOK, runAcceptanceListResponse{Acceptances: records})
}

func (h *APIHandler) HandleRunAcceptanceSynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req runAcceptanceSynthesizeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid run acceptance request"})
		return
	}
	if strings.TrimSpace(req.StagingURL) == "" && r.Host != "" {
		scheme := "https"
		if r.TLS == nil && strings.Contains(r.Host, "localhost") {
			scheme = "http"
		}
		req.StagingURL = scheme + "://" + r.Host
	}
	rec, err := h.rt.SynthesizeRunAcceptance(r.Context(), ownerID, runAcceptanceSynthesizeInput{
		TargetMissionID:       req.TargetMissionID,
		SourcePromptObjective: req.SourcePromptObjective,
		TrajectoryID:          req.TrajectoryID,
		RunID:                 req.RunID,
		CIRunID:               req.CIRunID,
		DeployRunID:           req.DeployRunID,
		StagingURL:            req.StagingURL,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusAccepted, rec)
}

func (h *APIHandler) HandleRunAcceptanceDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	const prefix = "/api/run-acceptances/"
	acceptanceID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if acceptanceID == "" || strings.Contains(acceptanceID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run acceptance not found"})
		return
	}
	rec, err := h.rt.store.GetRunAcceptance(r.Context(), ownerID, acceptanceID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run acceptance not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

// HandleRunSubmission handles POST /api/agent/loop.
// It accepts work only through the authenticated same-origin proxy path and
// denies missing or invalid auth before runtime work starts
// (VAL-RUNTIME-002). Returns a stable run handle (VAL-RUNTIME-003).
func (h *APIHandler) HandleRunSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req runSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "prompt is required"})
		return
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]any)
	}
	req.Metadata[runMetadataDesktopID] = requestDesktopID(r)

	rec, err := h.rt.StartRunWithMetadata(r.Context(), req.Prompt, ownerID, req.Metadata)
	if err != nil {
		log.Printf("runtime api: submit run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit run"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, runSubmitResponse{
		AgentID:   rec.AgentID,
		RunID:     rec.RunID,
		ChannelID: rec.ChannelID,
		State:     rec.State,
		OwnerID:   rec.OwnerID,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleSpawn handles POST /api/agent/spawn.
// It creates a child run linked to the given parent. The child run inherits
// the owner from the authenticated user context and begins in pending state.
func (h *APIHandler) HandleSpawn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req spawnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(req.ParentID) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "parent_id is required"})
		return
	}

	if strings.TrimSpace(req.Objective) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "objective is required"})
		return
	}

	rec, err := h.rt.StartChildRun(r.Context(), req.ParentID, req.Objective, ownerID, req.Constraints)
	if err != nil {
		// Check if the parent run was not found.
		if strings.Contains(err.Error(), "parent run not found") {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "parent run not found"})
			return
		}
		log.Printf("runtime api: start child run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to start child run"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, spawnResponse{
		AgentID:   rec.AgentID,
		RunID:     rec.RunID,
		ChannelID: rec.ChannelID,
		ParentID:  req.ParentID,
		State:     rec.State,
		OwnerID:   rec.OwnerID,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleInternalRunSubmission handles POST /internal/runtime/runs.
// This is a service-to-service worker VM bridge: platform/runtime components
// can start a constrained run inside a sandbox without exposing raw run control
// to the browser route table.
func (h *APIHandler) HandleInternalRunSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}

	var req internalRunSubmitRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "prompt is required"})
		return
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]any)
	}
	profile := canonicalAgentProfile(metadataStringValue(req.Metadata, runMetadataAgentProfile))
	if profile == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "agent_profile is required"})
		return
	}
	switch profile {
	case AgentProfileCoSuper, AgentProfileResearcher, AgentProfileVSuper:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "internal worker runs may only start co-super, researcher, or vsuper profiles"})
		return
	}
	req.Metadata[runMetadataAgentProfile] = profile
	if metadataStringValue(req.Metadata, runMetadataAgentRole) == "" {
		req.Metadata[runMetadataAgentRole] = profile
	}
	if metadataStringValue(req.Metadata, "request_source") == "" {
		req.Metadata["request_source"] = "internal_worker_vm"
	}

	rec, err := h.rt.StartRunWithMetadata(r.Context(), req.Prompt, ownerID, req.Metadata)
	if err != nil {
		log.Printf("runtime api: submit internal run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit internal run"})
		return
	}
	writeAPIJSON(w, http.StatusAccepted, runStatusFromRecord(rec))
}

// HandleInternalRunStatus handles GET /internal/runtime/runs/{id}.
func (h *APIHandler) HandleInternalRunStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required"})
		return
	}
	runID := internalRuntimeRunIDFromPath(r.URL.Path)
	if runID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	rec, err := h.rt.GetRun(r.Context(), runID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, runStatusFromRecord(rec))
}

// HandleInternalRunCancel handles POST /internal/runtime/runs/{id}/cancel.
// This is the service-to-service cancellation path used by async worker
// delegation. It is not browser-public and requires the internal caller header.
func (h *APIHandler) HandleInternalRunCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required"})
		return
	}
	runID := internalRuntimeRunIDFromPath(strings.TrimSuffix(r.URL.Path, "/cancel"))
	if runID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	if err := h.rt.CancelRun(r.Context(), runID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, runStatusResponse{RunID: runID, State: types.RunCancelled})
}

// HandleInternalChannelCast handles POST /internal/runtime/channel-casts.
// It lets a supervising runtime redirect a worker-vsuper through the worker
// runtime's durable inbox without exposing browser-public mutation routes.
func (h *APIHandler) HandleInternalChannelCast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	var req internalChannelCastRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.Content = strings.TrimSpace(req.Content)
	if req.OwnerID == "" || req.ChannelID == "" || req.Content == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id, channel_id, and content are required"})
		return
	}
	runCtx := WithToolExecutionContext(r.Context(), &types.RunRecord{
		RunID:        strings.TrimSpace(req.FromRunID),
		AgentID:      firstNonEmpty(strings.TrimSpace(req.FromAgentID), strings.TrimSpace(req.From)),
		ChannelID:    req.ChannelID,
		AgentProfile: strings.TrimSpace(req.Role),
		AgentRole:    strings.TrimSpace(req.Role),
		OwnerID:      req.OwnerID,
		Metadata: map[string]any{
			runMetadataAgentProfile: strings.TrimSpace(req.Role),
			runMetadataAgentRole:    strings.TrimSpace(req.Role),
		},
	})
	cursor, err := h.rt.ChannelCast(runCtx, req.ChannelID, strings.TrimSpace(req.ToAgentID), strings.TrimSpace(req.ToRunID), strings.TrimSpace(req.From), strings.TrimSpace(req.Role), req.Content)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, internalChannelCastResponse{Status: "cast", Cursor: cursor})
}

// HandleInternalRunEvents handles internal service-to-service run events.
func (h *APIHandler) HandleInternalRunEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required"})
		return
	}
	runID := internalRuntimeRunIDFromPath(strings.TrimSuffix(r.URL.Path, "/events"))
	if runID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	if _, err := h.rt.GetRun(r.Context(), runID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	if r.Method == http.MethodPost {
		h.handleInternalRunEventAppend(w, r, ownerID, runID)
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	events, err := h.rt.Store().ListEvents(r.Context(), runID, limit)
	if err != nil {
		log.Printf("runtime api: list internal run events: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list run events"})
		return
	}
	writeAPIJSON(w, http.StatusOK, eventListResponse{Events: events})
}

func (h *APIHandler) handleInternalRunEventAppend(w http.ResponseWriter, r *http.Request, ownerID, runID string) {
	var in internalRunEventAppendRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(in.OwnerID) != "" && strings.TrimSpace(in.OwnerID) != ownerID {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id mismatch"})
		return
	}
	kind := types.EventKind(strings.TrimSpace(string(in.Kind)))
	switch kind {
	case types.EventEmailDraftApprovalRecorded, types.EventEmailDraftSent:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsupported internal event kind"})
		return
	}
	rec, err := h.rt.GetRun(r.Context(), runID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	raw, err := json.Marshal(in.Payload)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid event payload"})
		return
	}
	event := types.EventRecord{
		EventID:      uuid.NewString(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Phase:        firstNonEmpty(strings.TrimSpace(in.Phase), "email_appagent_evidence"),
		Payload:      raw,
	}
	if err := h.rt.Store().AppendEvent(r.Context(), &event); err != nil {
		log.Printf("runtime api: append internal run event: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to append event"})
		return
	}
	if h.rt.bus != nil {
		h.rt.bus.Publish(events.RuntimeEvent{
			Record: event,
			Actor:  events.ActorRuntime,
			Cause:  events.CauseHostAction,
		})
	}
	writeAPIJSON(w, http.StatusAccepted, internalRunEventAppendResponse{Status: "appended", EventID: event.EventID, Kind: kind})
}

func internalRuntimeRunIDFromPath(path string) string {
	const prefix = "/internal/runtime/runs/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	runID := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if runID == "" || strings.Contains(runID, "/") {
		return ""
	}
	return runID
}

// HandleCancel handles POST /api/agent/cancel.
// It cancels a running or pending run, transitioning it to cancelled state.
// The cancel endpoint is owner-scoped — a request for a run owned by a
// different user returns 404 to prevent IDOR probing (VAL-CHOIR-010).
func (h *APIHandler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req cancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(req.RunID) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "loop_id is required"})
		return
	}

	err = h.rt.CancelRun(r.Context(), req.RunID, ownerID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
			return
		}
		if strings.Contains(err.Error(), "cannot cancel") {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
		log.Printf("runtime api: cancel run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to cancel run"})
		return
	}

	writeAPIJSON(w, http.StatusOK, cancelResponse{
		RunID: req.RunID,
		State: types.RunCancelled,
	})
}

// HandleRunStatus handles GET /api/agent/status.
// It is exposed through the authenticated same-origin proxy path, accepts or
// returns a stable correlation to the run handle from submission, and exposes
// machine-readable lifecycle state including non-happy-path outcomes
// (VAL-RUNTIME-004, VAL-RUNTIME-006).
func (h *APIHandler) HandleRunStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	runID := r.URL.Query().Get("loop_id")
	if runID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "loop_id query parameter is required"})
		return
	}

	rec, err := h.rt.GetRun(r.Context(), runID, ownerID)
	if err != nil {
		// ErrNotFound covers both "run doesn't exist" and "run belongs to
		// another user" so callers cannot probe for other users' runs.
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}

	var finishedAt *string
	if rec.FinishedAt != nil {
		s := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
		finishedAt = &s
	}

	writeAPIJSON(w, http.StatusOK, runStatusResponse{
		AgentID:      rec.AgentID,
		RunID:        rec.RunID,
		ChannelID:    rec.ChannelID,
		ParentRunID:  rec.ParentRunID,
		AgentProfile: rec.AgentProfile,
		AgentRole:    rec.AgentRole,
		OwnerID:      rec.OwnerID,
		SandboxID:    rec.SandboxID,
		State:        rec.State,
		Prompt:       rec.Prompt,
		Result:       rec.Result,
		Error:        rec.Error,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:    rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt:   finishedAt,
		Metadata:     rec.Metadata,
	})
}

// HandleRunList handles GET /api/agent/loops.
// It returns recent owner-scoped runs in reverse chronological order so
// debugging and orchestration surfaces can inspect current work and run
// families without polling individual IDs one by one.
func (h *APIHandler) HandleRunList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	var runs []types.RunRecord
	if channelID != "" {
		runs, err = h.rt.Store().ListRunsByChannel(r.Context(), ownerID, channelID, limit)
	} else {
		runs, err = h.rt.ListRunsByOwner(r.Context(), ownerID, limit)
	}
	if err != nil {
		log.Printf("runtime api: list runs: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list runs"})
		return
	}

	resp := runListResponse{Runs: make([]runStatusResponse, 0, len(runs))}
	for _, rec := range runs {
		var finishedAt *string
		if rec.FinishedAt != nil {
			s := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
			finishedAt = &s
		}
		resp.Runs = append(resp.Runs, runStatusResponse{
			AgentID:      rec.AgentID,
			RunID:        rec.RunID,
			ChannelID:    rec.ChannelID,
			ParentRunID:  rec.ParentRunID,
			AgentProfile: rec.AgentProfile,
			AgentRole:    rec.AgentRole,
			OwnerID:      rec.OwnerID,
			SandboxID:    rec.SandboxID,
			State:        rec.State,
			Prompt:       rec.Prompt,
			Result:       rec.Result,
			Error:        rec.Error,
			CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt:    rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
			FinishedAt:   finishedAt,
			Metadata:     rec.Metadata,
		})
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleEventList handles GET /api/agent/events.
// When loop_id is present, it returns historical events for that specific
// loop after verifying owner access. Otherwise it returns recent owner-scoped
// events across loops. This legacy handler is not browser-public; Trace uses
// /api/trace/* projections.
func (h *APIHandler) HandleEventList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	runID := strings.TrimSpace(r.URL.Query().Get("loop_id"))
	if runID != "" {
		if _, err := h.rt.GetRun(r.Context(), runID, ownerID); err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
			return
		}
		events, err := h.rt.Store().ListEvents(r.Context(), runID, limit)
		if err != nil {
			log.Printf("runtime api: list run events: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list run events"})
			return
		}
		writeAPIJSON(w, http.StatusOK, eventListResponse{Events: events})
		return
	}
	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	if channelID != "" {
		events, err := h.rt.Store().ListEventsByChannel(r.Context(), ownerID, channelID, limit)
		if err != nil {
			log.Printf("runtime api: list channel events: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list channel events"})
			return
		}
		writeAPIJSON(w, http.StatusOK, eventListResponse{Events: events})
		return
	}

	events, err := h.rt.Store().ListEventsByOwner(r.Context(), ownerID, limit)
	if err != nil {
		log.Printf("runtime api: list owner events: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list events"})
		return
	}
	writeAPIJSON(w, http.StatusOK, eventListResponse{Events: events})
}

// HandleChannelMessageList handles GET /api/agent/channel-messages.
// It returns persisted message bodies for a specific owner-scoped coordination channel.
func (h *APIHandler) HandleChannelMessageList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
	if channelID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "channel_id is required"})
		return
	}

	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	afterSeq := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after_seq")); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n >= 0 {
			afterSeq = n
		}
	}

	messages, err := h.rt.Store().ListChannelMessages(r.Context(), ownerID, channelID, afterSeq, limit)
	if err != nil {
		log.Printf("runtime api: list channel messages: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list channel messages"})
		return
	}

	writeAPIJSON(w, http.StatusOK, channelMessageListResponse{Messages: messages})
}

// HandleRunStatusByID handles GET /api/agent/{id}/status.
// It returns the full run record for the run identified by the URL path
// parameter {id}. The response includes state, result (if complete), error
// (if failed), and timestamps (VAL-CHOIR-002, VAL-CHOIR-005).
// Access is scoped to the authenticated owner — a request for a run owned
// by a different user returns 404 to prevent IDOR probing. State updates
// are visible immediately after change.
func (h *APIHandler) HandleRunStatusByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	// Extract run ID from URL path: /api/agent/{id}/status
	// Expected prefix: /api/agent/  and suffix: /status
	path := r.URL.Path
	prefix := "/api/agent/"
	suffix := "/status"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid status path"})
		return
	}
	runID := strings.TrimPrefix(path, prefix)
	runID = strings.TrimSuffix(runID, suffix)
	if runID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "run ID is required"})
		return
	}

	rec, err := h.rt.GetRun(r.Context(), runID, ownerID)
	if err != nil {
		// ErrNotFound covers both "run doesn't exist" and "run belongs to
		// another user" so callers cannot probe for other users' runs.
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}

	var finishedAt *string
	if rec.FinishedAt != nil {
		s := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
		finishedAt = &s
	}

	writeAPIJSON(w, http.StatusOK, runStatusResponse{
		AgentID:      rec.AgentID,
		RunID:        rec.RunID,
		ChannelID:    rec.ChannelID,
		ParentRunID:  rec.ParentRunID,
		AgentProfile: rec.AgentProfile,
		AgentRole:    rec.AgentRole,
		OwnerID:      rec.OwnerID,
		SandboxID:    rec.SandboxID,
		State:        rec.State,
		Prompt:       rec.Prompt,
		Result:       rec.Result,
		Error:        rec.Error,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:    rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt:   finishedAt,
		Metadata:     rec.Metadata,
	})
}

// HandleTopology handles GET /api/agent/topology.
// It exposes the runtime's orchestration shape for operator/UI inspection:
// researcher count, current running run count, and the number of active
// coordination channels.
func (h *APIHandler) HandleTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(runtimeTopologyResponse{
		SandboxID:       h.rt.cfg.SandboxID,
		ResearcherCount: h.rt.cfg.ResearcherCount,
		RunningRuns:     h.rt.RunningCount(),
		ChannelCount:    len(h.rt.ChannelManager().ListChannels()),
		RuntimeHealth:   string(h.rt.HealthState()),
		ActiveProvider:  h.rt.provider.ProviderName(),
	})
}

// HandleEvents is the legacy raw owner event stream handler. It is intentionally
// not registered on the browser-public route table; Trace uses trajectory-scoped
// /api/trace/* projections instead.
func (h *APIHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	// Flush headers.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Parse optional after_seq for catch-up.
	afterSeq := int64(0)
	if v := r.URL.Query().Get("after_seq"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			afterSeq = n
		}
	}

	// Send historical events for catch-up if requested.
	if afterSeq > 0 {
		h.sendHistoricalEvents(r.Context(), w, ownerID, afterSeq)
	}

	// Subscribe to live events.
	bus := h.rt.EventBus()
	ch := bus.SubscribeWithBuffer(128)
	defer bus.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			// Filter by owner (caller scoping, VAL-RUNTIME-006).
			if ev.Record.OwnerID != ownerID && ev.Record.OwnerID != "" {
				continue
			}
			// Write SSE event.
			data, err := json.Marshal(ev.Record)
			if err != nil {
				log.Printf("runtime api: marshal event: %v", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// sendHistoricalEvents fetches and writes historical events from the store
// for the given owner with sequence > afterSeq. This supports SSE catch-up
// after reconnection.
func (h *APIHandler) sendHistoricalEvents(ctx context.Context, w http.ResponseWriter, ownerID string, afterSeq int64) {
	events, err := h.rt.Store().ListEventsByOwnerAfter(ctx, ownerID, afterSeq, 200)
	if err != nil {
		log.Printf("runtime api: fetch historical events: %v", err)
		return
	}
	for _, ev := range events {
		data, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	if len(events) > 0 {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// HandleHealth handles GET /health for the runtime service.
// It reports runtime readiness for real run handling, and surfaces degraded
// state rather than hiding it behind a generic healthy response
// (VAL-RUNTIME-001).
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := h.rt.HealthState()

	// Map runtime health to HTTP status code.
	httpStatus := http.StatusOK
	switch health {
	case types.HealthFailed:
		httpStatus = http.StatusServiceUnavailable
	case types.HealthDegraded:
		httpStatus = http.StatusOK // degraded is still serving, just observable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(runtimeHealthResponse{
		Status:          string(health),
		Service:         "sandbox",
		SandboxID:       h.rt.cfg.SandboxID,
		RuntimeHealth:   health,
		RunningRuns:     h.rt.RunningCount(),
		ResearcherCount: h.rt.cfg.ResearcherCount,
		ActiveProvider:  h.rt.provider.ProviderName(),
		Build:           buildinfo.Snapshot("sandbox"),
	})
}

// RegisterRoutes registers runtime API routes on the given server.
// The health handler overrides the default server health handler to
// report runtime readiness.
func RegisterRoutes(s *server.Server, h *APIHandler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/api/prompt-bar", h.HandlePromptBar)
	s.HandleFunc("/api/prompt-bar/submissions/", h.HandlePromptBarSubmission)
	s.HandleFunc("/api/trace/trajectories", h.HandleTraceTrajectories)
	s.HandleFunc("/api/trace/trajectories/", h.HandleTraceTrajectories)
	s.HandleFunc("/api/podcast/subscriptions/refresh", h.HandlePodcastSubscriptionsRefresh)
	s.HandleFunc("/api/podcast/subscriptions", h.HandlePodcastSubscriptions)
	s.HandleFunc("/api/podcast/search", h.HandlePodcastSearch)
	s.HandleFunc("/api/content/items", h.HandleContentItemsRoot)
	s.HandleFunc("/api/content/", h.HandleContentRouter)
	s.HandleFunc("/api/global-wire/stories", h.HandleGlobalWireStories)
	s.HandleFunc("/api/global-wire/sourcemaxx-status", h.HandleGlobalWireSourceMaxxStatus)
	s.HandleFunc("/api/global-wire/source-search", h.HandleGlobalWireSourceSearch)
	s.HandleFunc("/api/global-wire/source-refresh", h.HandleGlobalWireSourceRefresh)
	s.HandleFunc("/api/global-wire/contributions", h.HandleGlobalWireContributions)
	s.HandleFunc("/api/global-wire/reconciliation", h.HandleGlobalWireReconciliation)
	s.HandleFunc("/api/global-wire/source-dossiers", h.HandleGlobalWireSourceDossiers)
	s.HandleFunc("/api/global-wire/research-tasks", h.HandleGlobalWireResearchTasks)
	s.HandleFunc("/api/global-wire/research-evidence", h.HandleGlobalWireResearchEvidence)
	s.HandleFunc("/api/global-wire/publication-updates", h.HandleGlobalWirePublicationUpdates)
	s.HandleFunc("/api/global-wire/publication-artifacts", h.HandleGlobalWirePublicationArtifacts)
	s.HandleFunc("/api/global-wire/publication-feed", h.HandleGlobalWirePublicationFeed)
	s.HandleFunc("/api/global-wire/publication-artifact-reviews", h.HandleGlobalWirePublicationArtifactReviews)
	s.HandleFunc("/api/global-wire/publication-deliveries", h.HandleGlobalWirePublicationDeliveries)
	s.HandleFunc("/api/global-wire/publication-deliveries/", h.HandleGlobalWirePublicationDeliveryDetail)
	s.HandleFunc("/api/global-wire/autoradio-scripts", h.HandleGlobalWireAutoradioScripts)
	s.HandleFunc("/api/global-wire/autoradio-episodes", h.HandleGlobalWireAutoradioEpisodes)
	s.HandleFunc("/api/global-wire/publication-delivery-exports", h.HandleGlobalWirePublicationDeliveryExports)
	s.HandleFunc("/api/global-wire/publication-public-links", h.HandleGlobalWirePublicationPublicLinks)
	s.HandleFunc("/api/global-wire/publication-public-links/", h.HandleGlobalWirePublicationPublicLinkDetail)
	s.HandleFunc("/api/global-wire/newsletter-subscribers", h.HandleGlobalWireNewsletterSubscribers)
	s.HandleFunc("/api/global-wire/newsletter-issues", h.HandleGlobalWireNewsletterIssues)
	s.HandleFunc("/api/global-wire/graph-candidates", h.HandleGlobalWireGraphCandidates)
	s.HandleFunc("/api/global-wire/fetch-cycles", h.HandleGlobalWireFetchCycles)
	s.HandleFunc("/api/global-wire/style-sources", h.HandleGlobalWireStyleSources)
	s.HandleFunc("/api/global-wire/projection-reviews", h.HandleGlobalWireProjectionReviews)
	s.HandleFunc("/api/ws", h.HandleLiveWS)
	s.HandleFunc("/api/browser/capabilities", h.HandleBrowserCapabilities)
	s.HandleFunc("/api/browser/sessions", h.HandleBrowserSessionsRoot)
	s.HandleFunc("/api/browser/sessions/", h.HandleBrowserSessionRouter)
	s.HandleFunc("/api/desktop/state", h.HandleDesktopState)
	s.HandleFunc("/api/media/progress", h.HandleMediaProgress)
	s.HandleFunc("/api/media/recents", h.HandleMediaRecents)
	s.HandleFunc("/api/preferences/theme", h.HandleThemePreference)
	s.HandleFunc("/api/computers/", h.HandleComputersRouter)
	s.HandleFunc("/api/app-change-packages", h.HandleAppChangePackagesRoot)
	s.HandleFunc("/api/app-change-packages/", h.HandleAppChangePackageDetail)
	s.HandleFunc("/api/adoptions", h.HandleAppAdoptionsRoot)
	s.HandleFunc("/api/adoptions/", h.HandleAppAdoptionDetail)
	s.HandleFunc("/api/continuations", h.HandleRunContinuationsRoot)
	s.HandleFunc("/api/continuations/", h.HandleRunContinuationDetail)
	s.HandleFunc("/api/run-acceptances", h.HandleRunAcceptancesRoot)
	s.HandleFunc("/api/run-acceptances/synthesize", h.HandleRunAcceptanceSynthesize)
	s.HandleFunc("/api/run-acceptances/", h.HandleRunAcceptanceDetail)
	s.HandleFunc("/internal/runtime/app-change-packages", h.HandleInternalAppChangePackagesRoot)
	s.HandleFunc("/internal/runtime/app-change-packages/", h.HandleInternalAppChangePackageDetail)
	s.HandleFunc("/internal/runtime/channel-casts", h.HandleInternalChannelCast)
	s.HandleFunc("/internal/runtime/refresh", h.HandleInternalRuntimeRefresh)
	s.HandleFunc("/internal/runtime/runs", h.HandleInternalRunSubmission)
	s.HandleFunc("/internal/runtime/runs/", h.HandleInternalRuntimeRunRouter)
	s.HandleFunc("/internal/vtext/proposals", h.HandleInternalVTextProposalDelivery)
	if h.rt.cfg.EnableTestAPIs {
		s.HandleFunc("/api/prompts", h.HandlePromptList)
		s.HandleFunc("/api/prompts/", h.HandlePromptRole)
		s.HandleFunc("/api/test/vtext/research-findings", h.HandleTestVTextResearchFindings)
		s.HandleFunc("/api/test/vtext/worker-update", h.HandleTestVTextWorkerUpdate)
	}

	// VText document/revision/history/diff/blame APIs.
	// All routes are dispatched from a single prefix handler that inspects
	// the URL path and method to route to the correct handler. This avoids
	// ambiguity with Go's ServeMux prefix matching.
	RegisterVTextRoutes(s, h)
}

// HandleInternalRuntimeRunRouter dispatches internal service-to-service run
// status and event requests. It is intentionally separate from /api/*.
func (h *APIHandler) HandleInternalRuntimeRunRouter(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/events") {
		h.HandleInternalRunEvents(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/cancel") {
		h.HandleInternalRunCancel(w, r)
		return
	}
	h.HandleInternalRunStatus(w, r)
}

// RegisterVTextRoutes registers the vtext API routes on the given server.
// These routes expose document CRUD, revision, history, snapshot, diff,
// and blame APIs through the authenticated same-origin proxy path.
func RegisterVTextRoutes(s *server.Server, h *APIHandler) {
	// Exact match for document collection (create/list).
	s.HandleFunc("/api/vtext/documents", h.HandleVTextDocumentsRoot)

	// Prefix match for all other vtext routes.
	s.HandleFunc("/api/vtext/", h.HandleVTextRouter)
}

// HandleVTextRouter dispatches vtext API requests based on URL path and
// method. It handles all paths under /api/vtext/ that are not matched by
// the exact /api/vtext/documents route.
//
// Route mapping:
//
//	POST   /api/vtext/files/open               → resolve/create aliased file document
//	POST   /api/vtext/markdown-lineage/import  → migrate ordered Markdown snapshots into VText revisions
//	POST   /api/vtext/documents/{id}/manifest  → ensure a filesystem manifestation
//	GET    /api/vtext/documents/{id}           → get document
//	PUT    /api/vtext/documents/{id}           → update document
//	DELETE /api/vtext/documents/{id}           → delete document
//	POST   /api/vtext/documents/{id}/revisions → create revision
//	GET    /api/vtext/documents/{id}/revisions → list revisions
//	GET    /api/vtext/documents/{id}/stream    → document-scoped stream
//	POST   /api/vtext/documents/{id}/revise   → submit a document revise request
//	GET    /api/vtext/documents/{id}/compare  → semantic compare
//	POST   /api/vtext/documents/{id}/merge-preview → preview concept merge
//	POST   /api/vtext/documents/{id}/accept-merge → accept merge preview
//	POST   /api/vtext/documents/{id}/source-repairs → repair unresolved source gaps
//	POST   /api/vtext/documents/{id}/source-attachments → attach source artifacts to existing source entities
//	POST   /api/vtext/documents/{id}/restore  → restore historical revision as latest
//	GET    /api/vtext/documents/{id}/diagnosis → owner-scoped diagnosis bundle
//	GET    /api/vtext/documents/{id}/export    → export current VText revision
//	GET    /api/vtext/documents/{id}/history   → revision history
//	GET    /api/vtext/revisions/{id}          → get revision (snapshot)
//	GET    /api/vtext/revisions/{id}/blame     → blame revision
//	GET    /api/vtext/diff                     → diff two revisions
func (h *APIHandler) HandleVTextRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Diff endpoint: /api/vtext/diff
	if path == "/api/vtext/diff" {
		h.HandleVTextDiff(w, r)
		return
	}
	if path == "/api/vtext/files/open" {
		h.HandleVTextOpenFile(w, r)
		return
	}
	if path == "/api/vtext/markdown-lineage/import" {
		h.HandleVTextImportMarkdownLineage(w, r)
		return
	}

	// Revision item: /api/vtext/revisions/{id}
	if strings.HasPrefix(path, "/api/vtext/revisions/") {
		// Check for blame suffix: /api/vtext/revisions/{id}/blame
		if strings.HasSuffix(path, "/blame") {
			h.HandleVTextBlame(w, r)
			return
		}
		h.HandleVTextRevision(w, r)
		return
	}

	// Document sub-paths: /api/vtext/documents/{id}/...
	if strings.HasPrefix(path, "/api/vtext/documents/") {
		// Extract the part after /api/vtext/documents/
		rest := strings.TrimPrefix(path, "/api/vtext/documents/")

		// Check for sub-resource suffixes.
		if strings.HasSuffix(rest, "/revisions") {
			// /api/vtext/documents/{id}/revisions
			h.HandleVTextRevisions(w, r)
			return
		}
		if strings.HasSuffix(rest, "/manifest") {
			// /api/vtext/documents/{id}/manifest
			h.HandleVTextEnsureManifest(w, r)
			return
		}
		if strings.HasSuffix(rest, "/stream") {
			// /api/vtext/documents/{id}/stream
			h.HandleVTextDocumentStream(w, r)
			return
		}
		if strings.HasSuffix(rest, "/revise") {
			// /api/vtext/documents/{id}/revise
			h.HandleVTextAgentRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/cancel") {
			// /api/vtext/documents/{id}/cancel
			h.HandleVTextCancelAgentRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/compare") {
			h.HandleVTextSemanticCompare(w, r)
			return
		}
		if strings.HasSuffix(rest, "/merge-preview") {
			h.HandleVTextMergePreview(w, r)
			return
		}
		if strings.HasSuffix(rest, "/accept-merge") {
			h.HandleVTextAcceptMerge(w, r)
			return
		}
		if strings.HasSuffix(rest, "/source-repairs") {
			h.HandleVTextSourceGapRepair(w, r)
			return
		}
		if strings.HasSuffix(rest, "/source-attachments") {
			h.HandleVTextSourceArtifactAttachment(w, r)
			return
		}
		if strings.HasSuffix(rest, "/restore") {
			h.HandleVTextRestoreRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/diagnosis") {
			h.HandleVTextDiagnosis(w, r)
			return
		}
		if strings.HasSuffix(rest, "/export") {
			h.HandleVTextExportDocument(w, r)
			return
		}
		if strings.HasSuffix(rest, "/agent-revision") {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "vtext endpoint not found"})
			return
		}
		if strings.HasSuffix(rest, "/history") {
			// /api/vtext/documents/{id}/history
			h.HandleVTextHistory(w, r)
			return
		}

		// Otherwise, it's a document item: /api/vtext/documents/{id}
		h.HandleVTextDocument(w, r)
		return
	}

	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "vtext endpoint not found"})
}

// HandleVTextDocumentsRoot routes POST to create and GET to list at
// /api/vtext/documents (exact match, no trailing slash).
func (h *APIHandler) HandleVTextDocumentsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.HandleVTextCreateDocument(w, r)
	case http.MethodGet:
		h.HandleVTextListDocuments(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
