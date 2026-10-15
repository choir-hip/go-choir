package agentcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"os"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// apiError is a JSON error envelope for API responses.
type apiError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
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
	AgentID          string                       `json:"agent_id"`
	RunID            string                       `json:"run_id"`
	ChannelID        string                       `json:"channel_id,omitempty"`
	RequestedByRunID string                       `json:"requested_by_run_id,omitempty"`
	AgentProfile     string                       `json:"agent_profile,omitempty"`
	AgentRole        string                       `json:"agent_role,omitempty"`
	OwnerID          string                       `json:"owner_id"`
	ComputerID       string                       `json:"computer_id"`
	State            types.RunState               `json:"state"`
	Prompt           string                       `json:"prompt"`
	Result           string                       `json:"result,omitempty"`
	Error            string                       `json:"error,omitempty"`
	CreatedAt        string                       `json:"created_at"`
	UpdatedAt        string                       `json:"updated_at"`
	FinishedAt       *string                      `json:"finished_at,omitempty"`
	ActiveChildRuns  int                          `json:"active_child_runs,omitempty"`
	Metadata         map[string]any               `json:"metadata,omitempty"`
	Trajectory       *runTrajectoryStatusResponse `json:"trajectory,omitempty"`
}

type runTrajectoryStatusResponse struct {
	TrajectoryID      string                 `json:"trajectory_id"`
	Status            types.TrajectoryStatus `json:"status"`
	SettlementReady   bool                   `json:"settlement_ready"`
	WaitingOn         []string               `json:"waiting_on,omitempty"`
	OpenWorkItemCount int                    `json:"open_work_item_count"`
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
	ComputerID            string                   `json:"computer_id"`
	RuntimeHealth         types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns           int                      `json:"running_runs"`
	DeskPendingMutations  int                      `json:"desk_pending_mutations,omitempty"`
	SelfdevActiveOps      int                      `json:"selfdev_active_operations,omitempty"`
	ActiveProvider        string                   `json:"active_provider"`
	PersistentDisk        *persistentdisk.Status   `json:"persistent_disk,omitempty"`
	Build                 buildinfo.Info           `json:"build"`
	SelfDevelopmentMarker string                   `json:"self_development_marker,omitempty"`
	EventSchemaVersion    uint64                   `json:"event_schema_version,omitempty"`
	ReducerVersion        uint64                   `json:"reducer_version,omitempty"`
	ReleaseDigest         string                   `json:"release_digest,omitempty"`
	// LayeringBootguard carries the release key whose layered exec the
	// boot-loop guard refused (S2-e). Non-empty means the base runtime is
	// serving instead of current/'s layered release; /health returns 503 so
	// the updater probe fails closed and hosts see the trip, not a healthy
	// report from a binary that is not the applied release.
	LayeringBootguard string `json:"layering_bootguard,omitempty"`
	// EngineMutex exposes cumulative per-call-path wait/hold counters for the
	// embedded-Dolt engine lock (write + read pools share it). The latency
	// mission's drain probe uses wait-vs-hold attribution to decide between
	// read-pool separation and committed-state projection work.
	EngineMutex map[string]objectgraph.EngineMutexOpStats `json:"engine_mutex,omitempty"`
	// DoltGC is the guest store's last Dolt GC decision record
	// (store.ReadDoltGCDisposition): outcome, journal and free space, and
	// compaction_needed when GC alone cannot keep the disk free.
	DoltGC json.RawMessage `json:"dolt_gc,omitempty"`
	// DoltCompaction is the guest store's last history compaction record
	// (store.ReadDoltCompactionRecord): outcome, history and budget, the
	// heads digest and event index count it preserved.
	DoltCompaction json.RawMessage `json:"dolt_compaction,omitempty"`
}

// APIHandler provides HTTP handlers for the runtime API endpoints.
type APIHandler struct {
	rt *Runtime
}

// NewAPIHandler creates an APIHandler for the given runtime.
func NewAPIHandler(rt *Runtime) *APIHandler {
	return &APIHandler{rt: rt}
}

// authenticateUser extracts a proxy-authenticated identity only when it
// arrived from the host side of the guest boundary; loopback is an in-guest
// caller, not the owner (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2).
var authenticateUser = server.AuthenticatedOwner

// requireInternalRuntimeCaller admits the host side of the guest boundary
// only. These routes write the planned-restart marker, hibernate, apply a
// platform update and append run events; an in-guest process must reach
// none of them.
func requireInternalRuntimeCaller(r *http.Request) error {
	if !server.GuestHostCaller(r) {
		return fmt.Errorf("request was not host sourced")
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
	rawRole := strings.TrimSpace(r.URL.Query().Get("role"))
	if rawRole == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "missing role"})
		return
	}
	role := modelpolicy.NormalizeRole(rawRole)
	if role == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unknown role"})
		return
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

// modelPolicyVerifyMaxBodyBytes admits one image at the gateway's
// pre-resize limit, base64-encoded, plus the prompt.
const modelPolicyVerifyMaxBodyBytes = 28 << 20

// HandleModelPolicyVerify handles POST /api/model-policy/verify: one bounded
// model call through this computer's gateway, optionally with one image
// (image_base64, image_fixture or an image_ref in the computer's image
// store), as the owner. An inline image is stored by content hash and the
// call carries its ref; the response names the ref, never the bytes
// (docs/problems/models-configured-text-only-2026-10-15.md).
func (h *APIHandler) HandleModelPolicyVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, modelPolicyVerifyMaxBodyBytes))
	if err != nil {
		writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "request body too large"})
		return
	}
	out, err := h.rt.modelPolicy.VerifyForOwner(r.Context(), ownerID, body)
	if err != nil {
		status := http.StatusBadRequest
		var callErr *modelpolicy.ModelCallError
		if errors.As(err, &callErr) {
			status = http.StatusBadGateway
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, out)
}

func (h *APIHandler) HandleModelPolicyRouter(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/model-policy/resolve":
		h.HandleModelPolicyResolve(w, r)
	case r.URL.Path == "/api/model-policy/verify":
		h.HandleModelPolicyVerify(w, r)
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
// families without polling individual IDs one by one. ?trajectory_id=
// narrows the list to one trajectory's runs, including Management
// lifecycle-control runs bound through metadata.assignment_trajectory_id;
// ?channel_id= narrows it to one channel.
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
	trajectoryID := strings.TrimSpace(r.URL.Query().Get("trajectory_id"))
	var runs []types.RunRecord
	if trajectoryID != "" {
		runs, err = h.rt.ListRunsByTrajectory(r.Context(), ownerID, trajectoryID, limit)
	} else if channelID != "" {
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
			ComputerID:       rec.ComputerID,
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

// HandleRunResource handles /api/runs/{id} (GET the run record),
// /api/runs/{id}/events (GET the run's events, paged by seq) and
// /api/runs/{id}/cancel (POST).
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
	if len(parts) == 2 && parts[1] == "events" {
		h.handleRunEvents(w, r, ownerID, runID)
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

	// The layering boot-loop guard execs the base runtime after N rapid
	// failed starts of a layered release. Reporting that as healthy would
	// pass the updater's identity probe on a binary that is not the applied
	// release, so the guard marks the guest failed until a release that can
	// exec — restored prior or a new apply — replaces the guarded one.
	guard := strings.TrimSpace(os.Getenv("CHOIR_LAYERING_BOOTGUARD"))
	if guard != "" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	deskPending := h.rt.pendingDeskMutations(r.Context())
	selfdevActive := h.rt.activeSelfdevOperations(r.Context())
	resp := runtimeHealthResponse{
		Status:               string(health),
		Service:              "autoputer",
		ComputerID:           h.rt.cfg.ComputerID,
		RuntimeHealth:        health,
		RunningRuns:          h.rt.RunningCount(),
		DeskPendingMutations: deskPending,
		SelfdevActiveOps:     selfdevActive,
		ActiveProvider:       h.rt.provider.ProviderName(),
		Build:                buildinfo.Snapshot("autoputer"),
	}
	resp.SelfDevelopmentMarker = h.rt.selfdevStartupMarker
	resp.EventSchemaVersion = h.rt.selfdevStartupEventSchema
	resp.ReducerVersion = h.rt.selfdevStartupReducer
	resp.ReleaseDigest = h.rt.selfdevStartupReleaseDigest
	// Guard trip (computed above, before WriteHeader) marks the response.
	if guard != "" {
		resp.Status = "layering_bootguard"
		resp.LayeringBootguard = guard
	}
	if usage, err := persistentdisk.Statfs(filepath.Dir(h.rt.cfg.StorePath)); err == nil {
		status := persistentdisk.StatusFromGuestUsage(usage)
		resp.PersistentDisk = &status
	}
	resp.DoltGC = store.ReadDoltGCDisposition(filepath.Dir(h.rt.cfg.StorePath))
	resp.DoltCompaction = store.ReadDoltCompactionRecord(filepath.Dir(h.rt.cfg.StorePath))
	resp.EngineMutex = h.rt.store.EngineMutexStats()
	_ = json.NewEncoder(w).Encode(resp)
}

// guestObservabilityResponse is the product-API surface for the running
// guest's build identity and recent boot/reconcile log. It intentionally
// exposes no provider secrets, capability handles, or raw capsule material.
type guestObservabilityResponse struct {
	Service    string         `json:"service"`
	ComputerID string         `json:"computer_id"`
	Build      buildinfo.Info `json:"build"`
	RecentLog  []string       `json:"recent_log"`
	// ManagementTasks is the slot starter's state for the caller's
	// management (S2b): K, the slots in use and the queue.
	ManagementTasks *managementTaskObservability `json:"management_tasks,omitempty"`
	// EngineMutex is the engine lock's cumulative wait and hold per call
	// path since boot (H3: lock waits at K = 4).
	EngineMutex map[string]objectgraph.EngineMutexOpStats `json:"engine_mutex,omitempty"`
	// ClaudeDelegate is the Claude delegate's availability on this computer
	// and why not (docs/design/claude-delegate-slice1-2026-10-14.md). Never
	// the token.
	ClaudeDelegate *capsule.ClaudeDelegateStatus `json:"claude_delegate,omitempty"`
}

// HandleGuestObservability handles GET /api/runtime/observability: the guest's
// build commit and a bounded recent boot log. This closes the deploy-to-guest
// verification gap that otherwise requires shell access.
func (h *APIHandler) HandleGuestObservability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	resp := guestObservabilityResponse{
		Service:    "autoputer",
		ComputerID: h.rt.cfg.ComputerID,
		Build:      buildinfo.Snapshot("autoputer"),
		RecentLog:  h.rt.RecentBootLog(),
	}
	if h.rt.store != nil {
		resp.ManagementTasks = h.rt.managementTaskObservability(r.Context(), ownerID, h.rt.TextureComputerID())
		resp.EngineMutex = h.rt.store.EngineMutexStats()
	}
	if h.rt.capsuleExecutor != nil {
		status := h.rt.capsuleExecutor.ClaudeDelegateStatus()
		resp.ClaudeDelegate = &status
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleInternalRuntimeRunRouter dispatches internal service-to-service run
// event requests (/internal/runtime/runs/{id}/events). It is intentionally
// separate from /api/*.
func (h *APIHandler) HandleInternalRuntimeRunRouter(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/events") {
		h.HandleInternalRunEvents(w, r)
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
}
