package agentcore

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
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

// apiError is a JSON error envelope for API responses.
type apiError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
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

// internalRunSubmitRequest is the private service payload for typed ingestion
// processor and reconciler handoffs. It is never a delegated-agent transport
// and must not become browser-public.
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

type cancelResponse struct {
	RunID string         `json:"run_id"`
	State types.RunState `json:"state"`
}

// runStatusResponse is the public /api/runs projection.
type runStatusResponse struct {
	AgentID             string                                `json:"agent_id"`
	RunID               string                                `json:"run_id"`
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
	Status                string                   `json:"status"`
	Service               string                   `json:"service"`
	SandboxID             string                   `json:"sandbox_id"`
	RuntimeHealth         types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns           int                      `json:"running_runs"`
	RunningProcessorRuns  int                      `json:"running_processor_runs"`
	ResearcherCount       int                      `json:"researcher_count"`
	ActiveProvider        string                   `json:"active_provider"`
	PersistentDisk        *persistentdisk.Status   `json:"persistent_disk,omitempty"`
	Build                 buildinfo.Info           `json:"build"`
	SelfDevelopmentMarker string                   `json:"self_development_marker,omitempty"`
	EventSchemaVersion    uint64                   `json:"event_schema_version,omitempty"`
	ReducerVersion        uint64                   `json:"reducer_version,omitempty"`
	ReleaseDigest         string                   `json:"release_digest,omitempty"`
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
	if agentprofile.Canonical(agentProfileForRun(rec)) == agentprofile.Processor && ownerID != "" {
		item, found, err := h.rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.ProcessorDecisionFingerprint(trajectoryID))
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
	role := modelpolicy.NormalizeRole(r.URL.Query().Get("role"))
	if role == "" {
		role = agentprofile.Conductor
	}
	metadata := map[string]any{
		runMetadataAgentProfile: role,
		runMetadataAgentRole:    role,
	}
	overlayID := strings.TrimSpace(r.URL.Query().Get("overlay_id"))
	if overlayID != "" {
		metadata[modelpolicy.MetadataPolicyOverlayID] = overlayID
	}
	metadata = h.rt.modelPolicy.EnrichMetadata(r.Context(), ownerID, role, metadata)
	writeAPIJSON(w, http.StatusOK, modelPolicyResolveResponse{
		Role:            role,
		OverlayID:       overlayID,
		Provider:        metadataStringValue(metadata, modelpolicy.MetadataProvider),
		Model:           metadataStringValue(metadata, modelpolicy.MetadataModel),
		ReasoningEffort: metadataStringValue(metadata, modelpolicy.MetadataReasoningEffort),
		MaxTokens:       metadataIntValue(metadata, modelpolicy.MetadataMaxTokens),
		Source:          metadataStringValue(metadata, modelpolicy.MetadataPolicySource),
		PolicyError:     metadataStringValue(metadata, modelpolicy.MetadataPolicyError),
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

// HandleInternalRunSubmission handles private typed ingestion handoffs. Durable
// agent delegation stays inside the guest runtime and never crosses this route.
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
	profile := agentprofile.Canonical(metadataStringValue(req.Metadata, runMetadataAgentProfile))
	if profile == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "agent_profile is required"})
		return
	}
	switch profile {
	case agentprofile.Processor, agentprofile.Reconciler:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "internal runs may only start processor or reconciler profiles"})
		return
	}
	req.Metadata[runMetadataAgentProfile] = profile
	if metadataStringValue(req.Metadata, runMetadataAgentRole) == "" {
		req.Metadata[runMetadataAgentRole] = profile
	}
	if metadataStringValue(req.Metadata, "request_source") == "" {
		req.Metadata["request_source"] = "internal_ingestion_handoff"
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
	if typedIngestionSubmission && profile != agentprofile.Processor && profile != agentprofile.Reconciler {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "ingestion handoff identity is only valid for processor or reconciler profiles"})
		return
	}
	if profile == agentprofile.Processor || typedIngestionSubmission {
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

	if profile == agentprofile.Processor {
		// Reject genuinely new processor submissions when too many runs are
		// active. Duplicate typed identities were returned above.
		maxProc := 1
		if v := os.Getenv("RUNTIME_MAX_PROCESSOR_RUNS"); v != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && parsed > 0 {
				maxProc = parsed
			}
		}
		if h.rt.RunningCountByProfile(r.Context(), agentprofile.Processor) >= maxProc {
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
	runCtx := toolregistry.WithExecutionContext(r.Context(), toolExecutionContextForRun(&types.RunRecord{
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
	}))
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

// HandleRunList handles GET /api/runs.
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
		runs, err = h.rt.ListRunsByChannel(r.Context(), ownerID, channelID, limit)
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
func (h *APIHandler) HandleRunResource(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	parts := strings.Split(suffix, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" || len(parts) > 2 {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	runID := strings.TrimSpace(parts[0])
	if len(parts) == 1 && r.Method == http.MethodGet {
		run, getErr := h.rt.GetRun(r.Context(), runID, ownerID)
		if getErr != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, run)
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		if cancelErr := h.rt.CancelRun(r.Context(), runID, ownerID); cancelErr != nil {
			if strings.Contains(cancelErr.Error(), "not found") {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
				return
			}
			writeAPIJSON(w, http.StatusConflict, apiError{Error: cancelErr.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, cancelResponse{RunID: runID, State: types.RunCancelled})
		return
	}
	writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
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
	runningProcessorRuns := h.rt.RunningCountByProfile(r.Context(), agentprofile.Processor)
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
	resp.SelfDevelopmentMarker = h.rt.selfdevStartupMarker
	resp.EventSchemaVersion = h.rt.selfdevStartupEventSchema
	resp.ReducerVersion = h.rt.selfdevStartupReducer
	resp.ReleaseDigest = h.rt.selfdevStartupReleaseDigest
	if usage, err := persistentdisk.Statfs(filepath.Dir(h.rt.cfg.StorePath)); err == nil {
		status := persistentdisk.StatusFromGuestUsage(usage)
		resp.PersistentDisk = &status
	}
	_ = json.NewEncoder(w).Encode(resp)
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
