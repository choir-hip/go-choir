package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"os"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// apiError is a JSON error envelope for API responses.
type apiError struct {
	Error string `json:"error"`
}

type internalChannelCastRequest struct {
	OwnerID     string `json:"owner_id"`
	ChannelID   string `json:"channel_id"`
	ToAgentID   string `json:"to_agent_id,omitempty"`
	ToRunID     string `json:"to_loop_id,omitempty"`
	FromAgentID string `json:"from_agent_id,omitempty"`
	FromRunID   string `json:"from_loop_id,omitempty"`
	From        string `json:"from,omitempty"`
	Role        string `json:"role,omitempty"`
	Content     string `json:"content"`
}

type internalChannelCastResponse struct {
	Status string `json:"status"`
	Cursor uint64 `json:"cursor"`
}

// internalRunSubmitRequest is the service-to-service payload for starting a
// run inside another sandbox runtime, such as a background worker VM. It is not
// registered under /api/* and must never become browser-public.
type internalRunSubmitRequest struct {
	OwnerID  string         `json:"owner_id"`
	Prompt   string         `json:"prompt"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

const internalRunSubmissionFingerprintMetadataKey = "internal_run_submission_fingerprint"

// These are the typed sourcecycled processor and reconciler fields used to
// compare a pre-idempotency run that does not yet carry the durable full-request
// fingerprint. New submissions persist a fingerprint of the complete normalized
// request metadata, so future metadata additions automatically participate in
// conflict detection.
var internalIngestionSubmissionLegacyFingerprintKeys = []string{
	runMetadataAgentID,
	runMetadataChannelID,
	runMetadataAgentProfile,
	runMetadataAgentRole,
	"request_source",
	"activation_origin",
	"ingestion_event_ids",
	"source_network_cycle_id",
	"source_network_request_id",
	"source_network_request_kind",
	"ingestion_handoff_request_kind",
	"ingestion_handoff_request_id",
	"ingestion_handoff_cycle_id",
	runMetadataProcessorKey,
	"source_item_ids",
	"source_count",
	"source_types",
	"verticals",
	"regions",
	"continuity_ref",
	runMetadataReconcilerScope,
	"processor_request_ids",
}

func hashInternalRunSubmission(ownerID, prompt string, metadata map[string]any) (string, error) {
	payload := struct {
		OwnerID  string         `json:"owner_id"`
		Prompt   string         `json:"prompt"`
		Metadata map[string]any `json:"metadata"`
	}{
		OwnerID:  strings.TrimSpace(ownerID),
		Prompt:   prompt,
		Metadata: metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum[:]), nil
}

func internalRunSubmissionFingerprint(ownerID, prompt string, metadata map[string]any) (string, error) {
	normalized := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if key == internalRunSubmissionFingerprintMetadataKey {
			continue
		}
		normalized[key] = value
	}
	return hashInternalRunSubmission(ownerID, prompt, normalized)
}

func legacyInternalIngestionSubmissionFingerprint(ownerID, prompt string, metadata map[string]any) (string, error) {
	normalized := make(map[string]any, len(internalIngestionSubmissionLegacyFingerprintKeys))
	for _, key := range internalIngestionSubmissionLegacyFingerprintKeys {
		if value, ok := metadata[key]; ok {
			normalized[key] = value
		}
	}
	return hashInternalRunSubmission(ownerID, prompt, normalized)
}

// promptBarSubmitRequest is the public product payload for POST
// /api/prompt-bar. Browser callers submit user intent only; runtime role,
// model, channel, trajectory, and orchestration metadata are assigned
// server-side.
type promptBarSubmitRequest struct {
	Text string `json:"text"`
}

type modelPolicyResolveResponse struct {
	Role            string `json:"role"`
	OverlayID       string `json:"overlay_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTokens       int    `json:"max_tokens,omitempty"`
	Source          string `json:"source,omitempty"`
	PolicyError     string `json:"policy_error,omitempty"`
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

// runStatusResponse is the JSON response for GET /api/agent/status.
// It returns the full run record correlated to the submitted handle
// (VAL-RUNTIME-004).
type runStatusResponse struct {
	AgentID             string                                `json:"agent_id"`
	RunID               string                                `json:"loop_id"`
	ChannelID           string                                `json:"channel_id,omitempty"`
	RequestedByRunID    string                                `json:"requested_by_run_id,omitempty"`
	AgentProfile        string                                `json:"agent_profile,omitempty"`
	AgentRole           string                                `json:"agent_role,omitempty"`
	OwnerID             string                                `json:"owner_id"`
	SandboxID           string                                `json:"sandbox_id"`
	State               types.RunState                        `json:"state"`
	Prompt              string                                `json:"prompt"`
	Result              string                                `json:"result,omitempty"`
	Error               string                                `json:"error,omitempty"`
	CreatedAt           string                                `json:"created_at"`
	UpdatedAt           string                                `json:"updated_at"`
	FinishedAt          *string                               `json:"finished_at,omitempty"`
	ActiveChildRuns     int                                   `json:"active_child_runs,omitempty"`
	Metadata            map[string]any                        `json:"metadata,omitempty"`
	Trajectory          *runTrajectoryStatusResponse          `json:"trajectory,omitempty"`
	ProcessorResolution *runProcessorResolutionStatusResponse `json:"processor_resolution,omitempty"`
}

type runTrajectoryStatusResponse struct {
	TrajectoryID      string                 `json:"trajectory_id"`
	Status            types.TrajectoryStatus `json:"status"`
	SettlementReady   bool                   `json:"settlement_ready"`
	WaitingOn         []string               `json:"waiting_on,omitempty"`
	OpenWorkItemCount int                    `json:"open_work_item_count"`
}

type runProcessorResolutionStatusResponse struct {
	WorkItemID              string               `json:"work_item_id"`
	Status                  types.WorkItemStatus `json:"status"`
	ResolutionState         string               `json:"resolution_state,omitempty"`
	SourceItemCount         int                  `json:"source_item_count,omitempty"`
	ResolvedSourceItemCount int                  `json:"resolved_source_item_count,omitempty"`
	LastDecision            string               `json:"last_decision,omitempty"`
	StoryDocID              string               `json:"story_doc_id,omitempty"`
	CoveredByDocID          string               `json:"covered_by_doc_id,omitempty"`
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

// runtimeHealthResponse is the JSON structure returned by GET /health.
// It reports runtime readiness for real run handling, and surfaces
// degraded state rather than hiding it behind a generic healthy response
// (VAL-RUNTIME-001). The active provider name is included so operators
// can distinguish real-provider paths from stub/canned paths.
type runtimeHealthResponse struct {
	Status               string                   `json:"status"`
	Service              string                   `json:"service"`
	SandboxID            string                   `json:"sandbox_id"`
	RuntimeHealth        types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns          int                      `json:"running_runs"`
	RunningProcessorRuns int                      `json:"running_processor_runs"`
	ResearcherCount      int                      `json:"researcher_count"`
	ActiveProvider       string                   `json:"active_provider"`
	PersistentDisk       *persistentdisk.Status   `json:"persistent_disk,omitempty"`
	Build                buildinfo.Info           `json:"build"`
}

// APIHandler provides HTTP handlers for the runtime API endpoints.
type APIHandler struct {
	rt *Runtime
}

// NewAPIHandler creates an APIHandler for the given runtime.
//
// Deprecated: use apihandler.NewAPIHandler from internal/apihandler. This
// runtime-level constructor remains only while the actor-runtime defactoring
// migrates the handler methods out of the runtime package.
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
		AgentID:          rec.AgentID,
		RunID:            rec.RunID,
		ChannelID:        rec.ChannelID,
		RequestedByRunID: rec.RequestedByRunID,
		AgentProfile:     rec.AgentProfile,
		AgentRole:        rec.AgentRole,
		OwnerID:          rec.OwnerID,
		SandboxID:        rec.SandboxID,
		State:            rec.State,
		Prompt:           rec.Prompt,
		Result:           rec.Result,
		Error:            rec.Error,
		CreatedAt:        rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:        rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt:       finishedAt,
		ActiveChildRuns:  0,
		Metadata:         rec.Metadata,
	}
}

func (h *APIHandler) runStatusWithTrajectory(ctx context.Context, rec *types.RunRecord) runStatusResponse {
	resp := runStatusFromRecord(rec)
	if h == nil || h.rt == nil || rec == nil {
		return resp
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if trajectoryID == "" {
		return resp
	}
	// Errors are intentionally swallowed: this is a best-effort status
	// projection, not an authority surface.
	obligations, err := h.rt.TrajectoryObligations(ctx, ownerID, trajectoryID)
	if err != nil {
		return resp
	}
	resp.Trajectory = &runTrajectoryStatusResponse{
		TrajectoryID:      obligations.Trajectory.TrajectoryID,
		Status:            obligations.Trajectory.Status,
		SettlementReady:   obligations.SettlementReady,
		WaitingOn:         append([]string(nil), obligations.WaitingOn...),
		OpenWorkItemCount: len(obligations.OpenWorkItems),
	}
	if canonicalAgentProfile(agentProfileForRun(rec)) == AgentProfileProcessor && ownerID != "" {
		item, found, err := h.rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, wireProcessorDecisionWorkItemFingerprint(trajectoryID))
		if err == nil && found {
			resp.ProcessorResolution = &runProcessorResolutionStatusResponse{
				WorkItemID:              item.WorkItemID,
				Status:                  item.Status,
				ResolutionState:         metadataStringValue(item.Details, wireDetailKeyResolutionState),
				SourceItemCount:         metadataIntValue(item.Details, "source_item_count"),
				ResolvedSourceItemCount: metadataIntValue(item.Details, "resolved_source_item_count"),
				LastDecision:            metadataStringValue(item.Details, wireDetailKeyLastDecision),
				StoryDocID:              metadataStringValue(item.Details, wireDetailKeyStoryDocID),
				CoveredByDocID:          metadataStringValue(item.Details, wireDetailKeyCoveredByDocID),
			}
		}
	}
	return resp
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

	requestedApp := AgentProfileTexture
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
		"initial_document_title": buildInitialTextureTitle(text, ""),
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
			Title:     buildInitialTextureTitle(text, ""),
			SourceURL: contentSourceURL,
			MediaType: contentMediaType,
			AppHint:   contentAppHint,
		}
		rec, err = h.rt.completePromptBarDecisionRun(r.Context(), text, ownerID, metadata, decision)
	} else if isTextureDecisionApp(requestedApp) {
		decision := conductorDecision{
			Action: "open_app",
			App:    AgentProfileTexture,
			Title:  buildInitialTextureTitle(text, ""),
		}
		rec, err = h.rt.completePromptBarDecisionRun(r.Context(), text, ownerID, metadata, decision)
		if err == nil {
			if _, routeErr := h.rt.ensureConductorTextureRoute(r.Context(), rec, text, ""); routeErr != nil {
				log.Printf("runtime api: materialize prompt-bar texture route: %v", routeErr)
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

func (h *APIHandler) HandleModelPolicyResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	role := normalizeModelPolicyRole(r.URL.Query().Get("role"))
	if role == "" {
		role = AgentProfileConductor
	}
	metadata := map[string]any{
		runMetadataAgentProfile: role,
		runMetadataAgentRole:    role,
	}
	overlayID := strings.TrimSpace(r.URL.Query().Get("overlay_id"))
	if overlayID != "" {
		metadata[runMetadataLLMPolicyOverlayID] = overlayID
	}
	metadata = h.rt.ensureResolvedLLMMetadata(r.Context(), ownerID, metadata)
	writeAPIJSON(w, http.StatusOK, modelPolicyResolveResponse{
		Role:            role,
		OverlayID:       overlayID,
		Provider:        metadataStringValue(metadata, runMetadataLLMProvider),
		Model:           metadataStringValue(metadata, runMetadataLLMModel),
		ReasoningEffort: metadataStringValue(metadata, runMetadataLLMReasoningEffort),
		MaxTokens:       metadataIntValue(metadata, runMetadataLLMMaxTokens),
		Source:          metadataStringValue(metadata, runMetadataLLMPolicySource),
		PolicyError:     metadataStringValue(metadata, runMetadataLLMPolicyError),
	})
}

func (h *APIHandler) HandleModelPolicyRouter(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/model-policy/resolve":
		h.HandleModelPolicyResolve(w, r)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "model policy route not found"})
	}
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
	case AgentProfileCoSuper, AgentProfileResearcher, AgentProfileVSuper, AgentProfileProcessor, AgentProfileReconciler:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "internal worker runs may only start co-super, researcher, vsuper, processor, or reconciler profiles"})
		return
	}
	req.Metadata[runMetadataAgentProfile] = profile
	if metadataStringValue(req.Metadata, runMetadataAgentRole) == "" {
		req.Metadata[runMetadataAgentRole] = profile
	}
	if metadataStringValue(req.Metadata, "request_source") == "" {
		req.Metadata["request_source"] = "internal_worker_vm"
	}

	rawRequestID, hasRequestID := req.Metadata["ingestion_handoff_request_id"]
	rawRequestKind, hasRequestKind := req.Metadata["ingestion_handoff_request_kind"]
	requestID, requestIDIsString := rawRequestID.(string)
	requestKind, requestKindIsString := rawRequestKind.(string)
	requestID = strings.TrimSpace(requestID)
	requestKind = strings.TrimSpace(requestKind)
	if hasRequestID != hasRequestKind ||
		(hasRequestID && (!requestIDIsString || !requestKindIsString || requestID == "" || requestKind == "")) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "ingestion_handoff_request_id and ingestion_handoff_request_kind must be provided together as non-empty strings"})
		return
	}
	typedIngestionSubmission := hasRequestID
	if typedIngestionSubmission && profile != AgentProfileProcessor && profile != AgentProfileReconciler {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "ingestion handoff identity is only valid for processor or reconciler profiles"})
		return
	}
	if profile == AgentProfileProcessor || typedIngestionSubmission {
		// One critical section owns typed identity lookup and persistence across
		// ingestion profiles. It also serializes processor overload admission.
		// Concurrent lost-receipt retries therefore cannot both observe absence.
		h.rt.internalIngestionSubmissionMu.Lock()
		defer h.rt.internalIngestionSubmissionMu.Unlock()
	}

	if typedIngestionSubmission {
		req.Metadata["ingestion_handoff_request_id"] = requestID
		req.Metadata["ingestion_handoff_request_kind"] = requestKind
		fingerprint, err := internalRunSubmissionFingerprint(ownerID, req.Prompt, req.Metadata)
		if err != nil {
			log.Printf("runtime api: fingerprint internal ingestion submission: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to fingerprint internal ingestion submission"})
			return
		}
		req.Metadata[internalRunSubmissionFingerprintMetadataKey] = fingerprint

		existing, err := h.rt.Store().ListRunsByIngestionHandoff(r.Context(), ownerID, profile, requestID, requestKind, 2)
		if err != nil {
			log.Printf("runtime api: resolve ingestion handoff: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve ingestion handoff"})
			return
		}
		if len(existing) > 1 {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "ingestion handoff identity resolves to multiple runs"})
			return
		}
		if len(existing) == 1 {
			existingFingerprint := strings.TrimSpace(metadataStringValue(existing[0].Metadata, internalRunSubmissionFingerprintMetadataKey))
			if existingFingerprint == "" {
				// Runs created before the durable fingerprint was introduced can
				// still be retried safely by comparing the complete typed
				// sourcecycled ingestion contract for their profile.
				existingFingerprint, err = legacyInternalIngestionSubmissionFingerprint(existing[0].OwnerID, existing[0].Prompt, existing[0].Metadata)
				if err == nil {
					fingerprint, err = legacyInternalIngestionSubmissionFingerprint(ownerID, req.Prompt, req.Metadata)
				}
				if err != nil {
					log.Printf("runtime api: compare legacy ingestion handoff: %v", err)
					writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to compare ingestion handoff"})
					return
				}
			}
			if existingFingerprint != fingerprint {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "ingestion handoff identity already exists with a different payload"})
				return
			}
			// Resolve idempotency before overload: the admitted run is the
			// durable receipt even while it occupies the last processor slot.
			writeAPIJSON(w, http.StatusAccepted, runStatusFromRecord(&existing[0]))
			return
		}
	}

	if profile == AgentProfileProcessor {
		// Reject genuinely new processor submissions when too many runs are
		// active. Duplicate typed identities were returned above.
		maxProc := 1
		if v := os.Getenv("RUNTIME_MAX_PROCESSOR_RUNS"); v != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && parsed > 0 {
				maxProc = parsed
			}
		}
		if h.rt.RunningCountByProfile(r.Context(), AgentProfileProcessor) >= maxProc {
			writeAPIJSON(w, http.StatusTooManyRequests, apiError{Error: "too many active processor runs; try again later"})
			return
		}
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
	writeAPIJSON(w, http.StatusOK, h.runStatusWithTrajectory(r.Context(), rec))
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
// It accepts unaddressed audit casts from trusted runtime callers. Addressed
// agent-to-agent wakes must use update_coagent so authority and delivery shape
// checks stay on the single durable wake primitive.
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
	targetAgentID := strings.TrimSpace(req.ToAgentID)
	if targetAgentID != "" || strings.TrimSpace(req.ToRunID) != "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "addressed internal channel casts are disabled; use update_coagent for agent-to-agent wake delivery"})
		return
	}
	cursor, err := h.rt.ChannelCast(runCtx, req.ChannelID, "", "", strings.TrimSpace(req.From), strings.TrimSpace(req.Role), req.Content)
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
			AgentID:          rec.AgentID,
			RunID:            rec.RunID,
			ChannelID:        rec.ChannelID,
			RequestedByRunID: rec.RequestedByRunID,
			AgentProfile:     rec.AgentProfile,
			AgentRole:        rec.AgentRole,
			OwnerID:          rec.OwnerID,
			SandboxID:        rec.SandboxID,
			State:            rec.State,
			Prompt:           rec.Prompt,
			Result:           rec.Result,
			Error:            rec.Error,
			CreatedAt:        rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt:        rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
			FinishedAt:       finishedAt,
			Metadata:         rec.Metadata,
		})
	}

	writeAPIJSON(w, http.StatusOK, resp)
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
	runningProcessorRuns := h.rt.RunningCountByProfile(r.Context(), AgentProfileProcessor)
	resp := runtimeHealthResponse{
		Status:               string(health),
		Service:              "sandbox",
		SandboxID:            h.rt.cfg.SandboxID,
		RuntimeHealth:        health,
		RunningRuns:          h.rt.RunningCount(),
		RunningProcessorRuns: runningProcessorRuns,
		ResearcherCount:      h.rt.cfg.ResearcherCount,
		ActiveProvider:       h.rt.provider.ProviderName(),
		Build:                buildinfo.Snapshot("sandbox"),
	}
	if usage, err := persistentdisk.Statfs(filepath.Dir(h.rt.cfg.StorePath)); err == nil {
		status := persistentdisk.StatusFromGuestUsage(usage)
		resp.PersistentDisk = &status
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// RegisterRoutes registers runtime API routes on the given server.
// The health handler overrides the default server health handler to
// report runtime readiness.
//
// Deprecated: use apihandler.RegisterRoutes from internal/apihandler. This
// runtime-level registration function remains only while the actor-runtime
// defactoring migrates the handler methods out of the runtime package.
func RegisterRoutes(s *server.Server, h *APIHandler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/api/prompt-bar", h.HandlePromptBar)
	s.HandleFunc("/api/prompt-bar/submissions/", h.HandlePromptBarSubmission)
	s.HandleFunc("/api/agent/loops", h.HandleRunList)
	s.HandleFunc("/api/agent/cancel", h.HandleCancel)
	s.HandleFunc("/api/model-policy/", h.HandleModelPolicyRouter)
	s.HandleFunc("/api/costs", h.HandleCosts)
	s.HandleFunc("/api/podcast/subscriptions/refresh", h.HandlePodcastSubscriptionsRefresh)
	s.HandleFunc("/api/podcast/subscriptions", h.HandlePodcastSubscriptions)
	s.HandleFunc("/api/podcast/search", h.HandlePodcastSearch)
	s.HandleFunc("/api/content/items", h.HandleContentItemsRoot)
	s.HandleFunc("/api/content/", h.HandleContentRouter)
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
	RegisterCandidatePackageReviewSurfaceRoutes(s, h)
	s.HandleFunc("/api/trajectories", h.HandleTrajectoriesRoot)
	s.HandleFunc("/api/trajectories/", h.HandleTrajectoryDetail)
	s.HandleFunc("/api/run-acceptances", h.HandleRunAcceptancesRoot)
	s.HandleFunc("/api/run-acceptances/synthesize", h.HandleRunAcceptanceSynthesize)
	s.HandleFunc("/api/run-acceptances/", h.HandleRunAcceptanceDetail)
	s.HandleFunc("/api/evals/texture-prompt", h.HandleTexturePromptEval)
	s.HandleFunc("/internal/runtime/app-change-packages", h.HandleInternalAppChangePackagesRoot)
	s.HandleFunc("/internal/runtime/app-change-packages/", h.HandleInternalAppChangePackageDetail)
	s.HandleFunc("/internal/runtime/channel-casts", h.HandleInternalChannelCast)
	s.HandleFunc("/internal/runtime/refresh", h.HandleInternalRuntimeRefresh)
	s.HandleFunc("/internal/runtime/runs", h.HandleInternalRunSubmission)
	s.HandleFunc("/internal/runtime/runs/", h.HandleInternalRuntimeRunRouter)
	s.HandleFunc("/internal/texture/documents/", h.HandleInternalTextureDocument)
	s.HandleFunc("/internal/texture/revisions/", h.HandleInternalTextureRevision)
	s.HandleFunc("/internal/texture/proposals", h.HandleInternalTextureProposalDelivery)
	if h.rt.cfg.EnableTestAPIs {
		s.HandleFunc("/api/prompts", h.HandlePromptList)
		s.HandleFunc("/api/prompts/", h.HandlePromptRole)
		s.HandleFunc("/api/test/texture/worker-update", h.HandleTestTextureWorkerUpdate)
	}

	// Texture document/revision/history/diff/blame APIs.
	// All routes are dispatched from a single prefix handler that inspects
	// the URL path and method to route to the correct handler. This avoids
	// ambiguity with Go's ServeMux prefix matching.
	RegisterTextureRoutes(s, h)
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

// RegisterTextureRoutes registers the Texture API routes on the given server.
// These routes expose document CRUD, revision, history, snapshot, diff,
// and blame APIs through the authenticated same-origin proxy path.
func RegisterTextureRoutes(s *server.Server, h *APIHandler) {
	// Exact match for document collection (create/list).
	s.HandleFunc("/api/texture/documents", h.HandleTextureDocumentsRoot)

	// Prefix match for all other Texture routes.
	s.HandleFunc("/api/texture/", h.HandleTextureRouter)
}

const (
	textureAPIPathPrefix       = "/api/texture/"
	textureDocumentsPathPrefix = "/api/texture/documents/"
	textureRevisionsPathPrefix = "/api/texture/revisions/"
)

// HandleTextureRouter dispatches Texture API requests based on URL path and
// method. It handles all paths under /api/texture/ that are not matched by
// the exact /api/texture/documents route.
//
// Route mapping:
//
//	POST   /api/texture/files/open               → resolve/create aliased file document
//	POST   /api/texture/markdown-lineage/import  → migrate ordered Markdown snapshots into Texture revisions
//	POST   /api/texture/documents/{id}/manifest  → ensure a filesystem manifestation
//	GET    /api/texture/documents/{id}           → get document
//	PUT    /api/texture/documents/{id}           → update document
//	DELETE /api/texture/documents/{id}           → delete document
//	POST   /api/texture/documents/{id}/revisions → create revision
//	GET    /api/texture/documents/{id}/revisions → list revisions
//	GET    /api/texture/documents/{id}/stream    → document-scoped stream
//	POST   /api/texture/documents/{id}/revise    → submit a document revise request
//	GET    /api/texture/documents/{id}/compare   → semantic compare
//	POST   /api/texture/documents/{id}/merge-preview → preview concept merge
//	POST   /api/texture/documents/{id}/accept-merge → accept merge preview
//	POST   /api/texture/documents/{id}/restore   → restore historical revision as latest
//	GET    /api/texture/documents/{id}/diagnosis → owner-scoped diagnosis bundle
//	GET    /api/texture/documents/{id}/export    → export current Texture revision
//	GET    /api/texture/documents/{id}/history   → revision history
//	GET    /api/texture/revisions/{id}           → get revision (snapshot)
//	GET    /api/texture/revisions/{id}/blame     → blame revision
//	GET    /api/texture/diff                     → diff two revisions
func (h *APIHandler) HandleTextureRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Diff endpoint: /api/texture/diff
	if path == "/api/texture/diff" {
		h.HandleTextureDiff(w, r)
		return
	}
	if path == "/api/texture/files/open" {
		h.HandleTextureOpenFile(w, r)
		return
	}
	if path == "/api/texture/markdown-lineage/import" {
		h.HandleTextureImportMarkdownLineage(w, r)
		return
	}

	// Revision item: /api/texture/revisions/{id}
	if strings.HasPrefix(path, textureRevisionsPathPrefix) {
		// Check for blame suffix: /api/texture/revisions/{id}/blame
		if strings.HasSuffix(path, "/blame") {
			h.HandleTextureBlame(w, r)
			return
		}
		h.HandleTextureRevision(w, r)
		return
	}

	// Document sub-paths: /api/texture/documents/{id}/...
	if strings.HasPrefix(path, textureDocumentsPathPrefix) {
		// Extract the part after /api/texture/documents/
		rest := strings.TrimPrefix(path, textureDocumentsPathPrefix)

		// Check for sub-resource suffixes.
		if strings.HasSuffix(rest, "/revisions") {
			// /api/texture/documents/{id}/revisions
			h.HandleTextureRevisions(w, r)
			return
		}
		if strings.HasSuffix(rest, "/manifest") {
			// /api/texture/documents/{id}/manifest
			h.HandleTextureEnsureManifest(w, r)
			return
		}
		if strings.HasSuffix(rest, "/stream") {
			// /api/texture/documents/{id}/stream
			h.HandleTextureDocumentStream(w, r)
			return
		}
		if strings.HasSuffix(rest, "/revise") {
			// /api/texture/documents/{id}/revise
			h.HandleTextureAgentRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/cancel") {
			// /api/texture/documents/{id}/cancel
			h.HandleTextureCancelAgentRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/compare") {
			h.HandleTextureSemanticCompare(w, r)
			return
		}
		if strings.HasSuffix(rest, "/merge-preview") {
			h.HandleTextureMergePreview(w, r)
			return
		}
		if strings.HasSuffix(rest, "/accept-merge") {
			h.HandleTextureAcceptMerge(w, r)
			return
		}
		if strings.HasSuffix(rest, "/restore") {
			h.HandleTextureRestoreRevision(w, r)
			return
		}
		if strings.HasSuffix(rest, "/diagnosis") {
			h.HandleTextureDiagnosis(w, r)
			return
		}
		if strings.HasSuffix(rest, "/export") {
			h.HandleTextureExportDocument(w, r)
			return
		}
		if strings.HasSuffix(rest, "/agent-revision") {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
			return
		}
		if strings.HasSuffix(rest, "/history") {
			// /api/texture/documents/{id}/history
			h.HandleTextureHistory(w, r)
			return
		}

		// Otherwise, it's a document item: /api/texture/documents/{id}
		h.HandleTextureDocument(w, r)
		return
	}

	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
}

// HandleTextureDocumentsRoot routes POST to create and GET to list at
// /api/texture/documents (exact match, no trailing slash).
func (h *APIHandler) HandleTextureDocumentsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.HandleTextureCreateDocument(w, r)
	case http.MethodGet:
		h.HandleTextureListDocuments(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
