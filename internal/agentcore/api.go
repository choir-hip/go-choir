package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"os"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/guestmem"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/steptrace"
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
	Fallbacks       string `json:"fallbacks,omitempty"` // provider/model, comma-separated; empty for a pin or a pool with no failover
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

// effectiveReleaseIdentity is /health's account of the running release.
type effectiveReleaseIdentity struct {
	ReleaseDigest        string `json:"release_digest"`
	Layered              bool   `json:"layered"`
	CodeCommit           string `json:"code_commit,omitempty"`
	BaseCommit           string `json:"base_commit,omitempty"`
	BaseImageDigest      string `json:"base_image_manifest_digest,omitempty"`
	ClosureDigest        string `json:"closure_digest,omitempty"`
	BuilderReceiptDigest string `json:"builder_receipt_digest,omitempty"`
}

// runtimeHealthResponse is the JSON structure returned by GET /health.
// It reports runtime readiness for real run handling, and surfaces
// degraded state rather than hiding it behind a generic healthy response
// (VAL-RUNTIME-001). The active provider name is included so operators
// can distinguish real-provider paths from stub/canned paths.
type runtimeHealthResponse struct {
	Status               string                   `json:"status"`
	Service              string                   `json:"service"`
	ComputerID           string                   `json:"computer_id"`
	RuntimeHealth        types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns          int                      `json:"running_runs"`
	DeskPendingMutations int                      `json:"desk_pending_mutations,omitempty"`
	SelfdevActiveOps     int                      `json:"selfdev_active_operations,omitempty"`
	// LiveCapsules is the count of engineering capsules a restart would
	// destroy, the one planned_restart.go counts (density D3a). A pointer so
	// 0 is reported: a guest that predates the field omits it, and vmctl
	// keeps the legacy idle rule for such a guest. A parked run, a capsule
	// before its first Freeze and a frozen candidate awaiting approval all
	// read 0 in the three fields above.
	LiveCapsules          *int                   `json:"live_capsules,omitempty"`
	ActiveProvider        string                 `json:"active_provider"`
	PersistentDisk        *persistentdisk.Status `json:"persistent_disk,omitempty"`
	Build                 buildinfo.Info         `json:"build"`
	SelfDevelopmentMarker string                 `json:"self_development_marker,omitempty"`
	EventSchemaVersion    uint64                 `json:"event_schema_version,omitempty"`
	ReducerVersion        uint64                 `json:"reducer_version,omitempty"`
	ReleaseDigest         string                 `json:"release_digest,omitempty"`
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
	// EffectiveRelease is the release, tree, event head and CodeRef this
	// computer runs (P6, docs/reports/p6-p8-release-adopt-2026-10-15.md).
	EffectiveRelease *EffectiveRelease `json:"effective_release,omitempty"`
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
		Fallbacks:       metadataStringValue(metadata, modelpolicy.MetadataFallbacks),
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

	// The store reads come before WriteHeader and are bounded: a held write
	// connection answers 503 store_stalled at once, never a silent hang and
	// never a 200 (F1, docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md).
	reads := h.rt.readHealthStore(r.Context(), healthStoreBudget)
	if len(reads.unanswered) > 0 {
		stalled := runtimeHealthStalledResponse{
			Status:        "store_stalled",
			Service:       "autoputer",
			ComputerID:    h.rt.cfg.ComputerID,
			RuntimeHealth: health,
			RunningRuns:   h.rt.RunningCount(),
			Build:         buildinfo.Snapshot("autoputer"),
			StoreStalled: healthStoreStalled{
				Unanswered:      reads.unanswered,
				BudgetMS:        healthStoreBudget.Milliseconds(),
				EngineMutexLive: h.rt.store.EngineMutexLive(),
			},
		}
		if guard != "" {
			stalled.Status = "layering_bootguard"
			stalled.LayeringBootguard = guard
		}
		writeAPIJSON(w, http.StatusServiceUnavailable, stalled)
		// After the answer, never before it: name who holds the connection
		// while it is still held, at most once a minute.
		if h.rt.storeStall.allow(time.Now()) {
			go h.rt.logStoreStallHolders(reads.unanswered)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	resp := runtimeHealthResponse{
		Status:               string(health),
		Service:              "autoputer",
		ComputerID:           h.rt.cfg.ComputerID,
		RuntimeHealth:        health,
		RunningRuns:          h.rt.RunningCount(),
		DeskPendingMutations: reads.deskPending,
		SelfdevActiveOps:     reads.selfdevActive,
		LiveCapsules:         &reads.liveCapsules,
		ActiveProvider:       h.rt.provider.ProviderName(),
		Build:                buildinfo.Snapshot("autoputer"),
	}
	startup := h.rt.startupManifest()
	resp.SelfDevelopmentMarker = startup.Marker
	resp.EventSchemaVersion = startup.EventSchema
	resp.ReducerVersion = startup.Reducer
	resp.ReleaseDigest = startup.Digest
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
	resp.EngineMutex = h.rt.store.EngineMutexStats()
	resp.EffectiveRelease = &reads.identity
	_ = json.NewEncoder(w).Encode(resp)
}

// healthStoreBudget bounds /health's store reads, which run concurrently
// under it: the proxy's runtime probe gives up at 2 s and vmmanager's at 3 s.
const healthStoreBudget = time.Second

// runtimeHealthStalledResponse is /health's 503 when a store read missed its
// budget. The counts that were not read are left out: a 0 would read as idle.
type runtimeHealthStalledResponse struct {
	Status            string                   `json:"status"`
	Service           string                   `json:"service"`
	ComputerID        string                   `json:"computer_id"`
	RuntimeHealth     types.RuntimeHealthState `json:"runtime_health"`
	RunningRuns       int                      `json:"running_runs"`
	Build             buildinfo.Info           `json:"build"`
	LayeringBootguard string                   `json:"layering_bootguard,omitempty"`
	StoreStalled      healthStoreStalled       `json:"store_stalled"`
}

// healthStoreStalled names the reads that missed and the engine lock's live
// holder. The write connection can also be held outside the engine lock (the
// online GC, a raw query); then the holder is empty or a waiter on it.
type healthStoreStalled struct {
	Unanswered      []string                     `json:"unanswered"`
	BudgetMS        int64                        `json:"budget_ms"`
	EngineMutexLive *objectgraph.EngineMutexLive `json:"engine_mutex_live,omitempty"`
}

type healthStoreReads struct {
	deskPending   int
	selfdevActive int
	liveCapsules  int
	identity      EffectiveRelease
	unanswered    []string
}

// readHealthStore runs /health's three store reads concurrently and gives up
// on any that has not answered within budget. A read's value counts only if
// it finished before its context ended: the read helpers turn a cancelled
// query into 0, which must never pass as an answer. Cancelling the context on
// return frees a read still waiting for the connection; one already inside a
// statement ends with it and writes only to its own buffered channel.
func (rt *Runtime) readHealthStore(parent context.Context, budget time.Duration) healthStoreReads {
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	type answer[T any] struct {
		value T
		ok    bool
	}
	start := func(read func(context.Context) int) chan answer[int] {
		done := make(chan answer[int], 1)
		go func() {
			value := read(ctx)
			done <- answer[int]{value, ctx.Err() == nil}
		}()
		return done
	}
	desk := start(rt.pendingDeskMutations)
	selfdevOps := start(rt.activeSelfdevOperations)
	// The capsule count is a fail-closed read: an error is unanswered, not 0.
	capsules := make(chan answer[int], 1)
	go func() {
		value, err := rt.liveCapsuleCountForHealth(ctx)
		capsules <- answer[int]{value, err == nil && ctx.Err() == nil}
	}()
	release := make(chan answer[EffectiveRelease], 1)
	go func() {
		value := rt.EffectiveRelease(ctx)
		release <- answer[EffectiveRelease]{value, ctx.Err() == nil}
	}()

	var out healthStoreReads
	awaitInt := func(name string, done chan answer[int]) int {
		select {
		case got := <-done:
			if got.ok {
				return got.value
			}
		case <-ctx.Done():
		}
		out.unanswered = append(out.unanswered, name)
		return 0
	}
	out.deskPending = awaitInt("desk_pending_mutations", desk)
	out.selfdevActive = awaitInt("selfdev_active_operations", selfdevOps)
	out.liveCapsules = awaitInt("live_capsules", capsules)
	select {
	case got := <-release:
		if got.ok {
			out.identity = got.value
			break
		}
		out.unanswered = append(out.unanswered, "effective_release")
	case <-ctx.Done():
		out.unanswered = append(out.unanswered, "effective_release")
	}
	return out
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
	// EngineMutexLive is who holds the engine lock now and who waits for it,
	// with ages: a store call that never returns shows here, where the
	// cumulative stats above record a wait only after it ends.
	EngineMutexLive *objectgraph.EngineMutexLive `json:"engine_mutex_live,omitempty"`
	// StorePools is the store's write and read pools' sql.DBStats (one
	// connection each): how often and how long callers waited for the
	// connection since boot, the load behind /health's store_stalled 503s.
	StorePools *store.PoolStats `json:"store_pools,omitempty"`
	// Steps is the named steps of long operations (a cell, a Complete's
	// terminal saga, the executor's freeze and teardown) running now with
	// their start times, and the last finished ones: where a hung Complete
	// stopped (engineering-completion-path-cluster-2026-10-20.md, failure 3).
	Steps *steptrace.Snapshot `json:"steps,omitempty"`
	// TelemetryBatching is the run-telemetry batching instrumentation
	// (docs/reports/telemetry-batching-design-2026-10-20.md section 9): the
	// per-kind event counts and tape batch sizes that price batching, and the
	// sealer's counters when it is on. Read-only.
	TelemetryBatching *store.TelemetryStats `json:"telemetry_batching,omitempty"`
	// StreamSeqLease is the stream_seq lease: state, the durable block,
	// whether the lease was trusted and why not, how the process seeded and
	// how long it took (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md,
	// section 6). Served from a snapshot: reading it never waits on the
	// allocator.
	StreamSeqLease *store.StreamSeqLeaseStats `json:"stream_seq_lease,omitempty"`
	// TraceLog is the trace segment log's counters: lines and bytes by kind,
	// fsyncs, the ordering barrier's fsyncs and p99, segments and retained
	// bytes, the retention floor, dropped, fallbacks and poisoned (trace-log
	// design S2). Read-only.
	TraceLog *store.TraceLogStats `json:"trace_log,omitempty"`
	// GuestCheckpoint is the guest's Dolt history-commit cadence: the mode,
	// inline commits (time writers waited) and background commits, and
	// what is not yet committed (ARCH-13). Read-only.
	GuestCheckpoint *store.CheckpointStats `json:"guest_checkpoint,omitempty"`
	// EventTable is store.event_table as applied (event_table.go): the host
	// grant, the owner's setting, and whether the writer stamps. Absent on a
	// computer with neither, so the response is unchanged there.
	EventTable *EventTableStatus `json:"event_table,omitempty"`
	// ClaudeDelegate is the Claude delegate's availability on this computer
	// and why not (docs/design/claude-delegate-slice1-2026-10-14.md). Never
	// the token.
	ClaudeDelegate *capsule.ClaudeDelegateStatus `json:"claude_delegate,omitempty"`
	// CapsuleMemory is the last capsules' memory receipts (RES-04): the
	// policy, request, limits, memory.peak and memory.events of each at its
	// end, and whether doctrine's memory.high would have throttled it.
	CapsuleMemory []capsule.CapsuleMemoryReceipt `json:"capsule_memory,omitempty"`
	// Unanswered names the reads that did not return within
	// observabilityReadBudget, most likely waiting on a held lock; the rest
	// of the response is still current (failure 9).
	Unanswered []string `json:"unanswered,omitempty"`
	// Goroutines is the runtime's goroutine stacks, only with ?goroutines=1:
	// where a hang is stopped, by call (failure 9). Bounded.
	Goroutines string `json:"goroutines,omitempty"`
	// EffectiveRelease is /health's identity block on the product path
	// (P6): what the tracer and owners read without /internal/*.
	EffectiveRelease *EffectiveRelease `json:"effective_release,omitempty"`
	// Memory is the guest's /proc/meminfo view, read per request: a memory
	// hotplug lease changes MemTotal, and the staging probe reads it here
	// instead of over SSH (docs/reports/memory-hotplug-controller-design-2026-10-14.md).
	Memory *guestMemoryObservability `json:"memory,omitempty"`
	// MemoryDetail is where the guest's memory goes, only with
	// ?memory=detail: the meminfo split, resident memory by process name
	// and by cgroup, and how much of the page cache is this computer's
	// store versus the shared nix store (SDC RES-07, launch checklist
	// L1-02). Bounded to a few seconds; it takes no lock.
	MemoryDetail *guestmem.Detail `json:"memory_detail,omitempty"`
	// ObjectGraphWriterGate is the object graph writer gate's mode and the
	// violations it has seen since boot, keyed "kind|declared writer|reason"
	// (ARCH-04): the oracle for the observe-to-enforce flip, read here
	// instead of over SSH. This process's counts only (corpusd keeps its own).
	ObjectGraphWriterGate *writerGateObservability `json:"objectgraph_writer_gate,omitempty"`
}

type writerGateObservability struct {
	Mode   string            `json:"mode"`
	Counts map[string]uint64 `json:"counts"`
}

// guestMemoryObservability is the guest's own memory as the kernel reports it.
type guestMemoryObservability struct {
	MemTotalBytes     uint64 `json:"mem_total_bytes"`
	MemAvailableBytes uint64 `json:"mem_available_bytes"`
	MemFreeBytes      uint64 `json:"mem_free_bytes"`
	// VirtioMemLoaded and VirtioMemBound say whether the virtio_mem driver
	// is in the kernel and has a device: a hotplugged computer can be
	// resized only when both are true (docs/reports/memory-hotplug-
	// controller-design-2026-10-14.md section 9.1).
	VirtioMemLoaded bool `json:"virtio_mem_loaded"`
	VirtioMemBound  bool `json:"virtio_mem_bound"`
}

// guestVirtioMemState reads the virtio_mem driver's state under a sysfs
// root: loaded when /sys/module/virtio_mem exists, bound when its driver
// directory lists a virtio device.
func guestVirtioMemState(sysRoot string) (loaded, bound bool) {
	if _, err := os.Stat(filepath.Join(sysRoot, "module", "virtio_mem")); err == nil {
		loaded = true
	}
	entries, err := os.ReadDir(filepath.Join(sysRoot, "bus", "virtio", "drivers", "virtio_mem"))
	if err != nil {
		return loaded, false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "virtio") {
			return true, true
		}
	}
	return loaded, false
}

// parseGuestMeminfo reads MemTotal, MemAvailable and MemFree (kB) from
// /proc/meminfo text; nil when MemTotal is missing.
func parseGuestMeminfo(data string) *guestMemoryObservability {
	var out guestMemoryObservability
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			out.MemTotalBytes = kib << 10
		case "MemAvailable:":
			out.MemAvailableBytes = kib << 10
		case "MemFree:":
			out.MemFreeBytes = kib << 10
		}
	}
	if out.MemTotalBytes == 0 {
		return nil
	}
	return &out
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
	// Steps first: the route exists to show a hang, so nothing that can wait
	// on a lock comes before it, and every read that can is bounded.
	steps := h.rt.stepTrace().Snapshot()
	resp := guestObservabilityResponse{
		Service:    "autoputer",
		ComputerID: h.rt.cfg.ComputerID,
		Build:      buildinfo.Snapshot("autoputer"),
		RecentLog:  h.rt.RecentBootLog(),
		Steps:      &steps,
		ObjectGraphWriterGate: &writerGateObservability{
			Mode: objectgraph.CurrentWriterGateMode().String(), Counts: objectgraph.WriterGateCounts(),
		},
	}
	if r.URL.Query().Get("goroutines") == "1" {
		resp.Goroutines = goroutineDump()
	}
	if r.URL.Query().Get("memory") == "detail" {
		// StorePath is a 0-byte marker file; the Dolt workspace directory
		// beside it holds the data.
		resp.MemoryDetail = guestmem.Read(r.Context(), []guestmem.Tree{
			{Label: "store", Root: store.TextureWorkspacePath(h.rt.cfg.StorePath)},
			{Label: "nix_store", Root: "/nix/store"},
		}, guestmem.DefaultLimits)
	}
	if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		resp.Memory = parseGuestMeminfo(string(raw))
		if resp.Memory != nil {
			resp.Memory.VirtioMemLoaded, resp.Memory.VirtioMemBound = guestVirtioMemState("/sys")
		}
	}
	unanswered := func(name string) { resp.Unanswered = append(resp.Unanswered, name) }
	if h.rt.store != nil {
		resp.EngineMutex = h.rt.store.EngineMutexStats()
		resp.EngineMutexLive = h.rt.store.EngineMutexLive()
		resp.StorePools = h.rt.store.PoolStats()
		telemetry := h.rt.store.TelemetryStats()
		resp.TelemetryBatching = &telemetry
		lease := h.rt.store.StreamSeqLeaseStats()
		resp.StreamSeqLease = &lease
		traceLog := h.rt.store.TraceLogStats()
		resp.TraceLog = &traceLog
		checkpoint := h.rt.store.CheckpointStats()
		resp.GuestCheckpoint = &checkpoint
		if st := h.rt.eventTableStatus(); st.HostGranted || st.Setting || st.StoreFloor > 0 {
			resp.EventTable = &st
		}
		if tasks, ok := boundedObservabilityRead(observabilityReadBudget, func() *managementTaskObservability {
			return h.rt.managementTaskObservability(r.Context(), ownerID, h.rt.TextureComputerID())
		}); ok {
			resp.ManagementTasks = tasks
		} else {
			unanswered("management_tasks")
		}
	}
	if h.rt.capsuleExecutor != nil {
		if status, ok := boundedObservabilityRead(observabilityReadBudget, h.rt.capsuleExecutor.ClaudeDelegateStatus); ok {
			resp.ClaudeDelegate = &status
		} else {
			unanswered("claude_delegate")
		}
		if receipts, ok := boundedObservabilityRead(observabilityReadBudget, h.rt.capsuleExecutor.MemoryReceipts); ok {
			resp.CapsuleMemory = receipts
		} else {
			unanswered("capsule_memory")
		}
	}
	if identity, ok := boundedObservabilityRead(observabilityReadBudget, func() EffectiveRelease {
		return h.rt.EffectiveRelease(r.Context())
	}); ok {
		resp.EffectiveRelease = &identity
	} else {
		unanswered("effective_release")
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// observabilityReadBudget bounds each observability read that can wait on a
// lock (the store's engine lock, the executor's).
const observabilityReadBudget = 3 * time.Second

// boundedObservabilityRead runs read and gives up after budget. A read it
// gives up on keeps running until its lock frees and then writes only to its
// own buffered channel, so the caller's response is never touched late.
func boundedObservabilityRead[T any](budget time.Duration, read func() T) (T, bool) {
	done := make(chan T, 1)
	go func() { done <- read() }()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case value := <-done:
		return value, true
	case <-timer.C:
		var zero T
		return zero, false
	}
}

// goroutineDumpMax bounds the owner-requested goroutine dump.
const goroutineDumpMax = 1 << 20

func goroutineDump() string {
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 2); err != nil {
		return "goroutine dump failed: " + err.Error()
	}
	return boundGoroutineDump(buf.String(), goroutineDumpMax)
}

func boundGoroutineDump(dump string, max int) string {
	if len(dump) <= max {
		return dump
	}
	return dump[:max] + fmt.Sprintf("\n[... truncated: %d of %d bytes]", max, len(dump))
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
