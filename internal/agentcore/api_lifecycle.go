package agentcore

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const lifecycleRequestMaxBytes = 2 << 20

func (h *APIHandler) HandleLifecycle(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	computerID := strings.TrimSpace(h.rt.TextureSandboxID())
	if computerID == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer identity unavailable"})
		return
	}

	path := r.URL.EscapedPath()
	if r.Method == http.MethodGet && strings.HasPrefix(path, "/api/lifecycle/trajectories/") && strings.HasSuffix(path, "/events") {
		escapedID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/lifecycle/trajectories/"), "/events")
		trajectoryID, decodeErr := url.PathUnescape(escapedID)
		if decodeErr != nil || strings.TrimSpace(trajectoryID) == "" || strings.Contains(trajectoryID, "/") {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid trajectory id"})
			return
		}
		after, parseErr := strconv.ParseInt(firstNonEmpty(r.URL.Query().Get("after"), "0"), 10, 64)
		if parseErr != nil || after < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid event cursor"})
			return
		}
		limit, parseErr := strconv.Atoi(firstNonEmpty(r.URL.Query().Get("limit"), "100"))
		if parseErr != nil || limit <= 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid event limit"})
			return
		}
		page, getErr := h.rt.Store().ListLifecycleEventPage(r.Context(), ownerID, computerID, trajectoryID, after, limit)
		if getErr != nil {
			writeLifecycleAPIError(w, getErr)
			return
		}
		writeAPIJSON(w, http.StatusOK, page)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(path, "/api/lifecycle/trajectories/") {
		escapedID := strings.TrimPrefix(path, "/api/lifecycle/trajectories/")
		trajectoryID, decodeErr := url.PathUnescape(escapedID)
		if decodeErr != nil || strings.TrimSpace(trajectoryID) == "" || strings.Contains(trajectoryID, "/") {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid trajectory id"})
			return
		}
		snapshot, getErr := h.rt.Store().GetLifecycleSnapshot(r.Context(), ownerID, computerID, trajectoryID)
		if getErr != nil {
			writeLifecycleAPIError(w, getErr)
			return
		}
		h.enrichLifecycleActivation(r, ownerID, &snapshot)
		writeAPIJSON(w, http.StatusOK, snapshot)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, lifecycleRequestMaxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var result types.LifecycleResult
	switch path {
	case "/api/lifecycle/work/open":
		var req types.OpenLifecycleWorkRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle work request"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().OpenLifecycleWork(r.Context(), req)
	case "/api/lifecycle/work/amend":
		var req types.AmendLifecycleWorkRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle work amendment"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().AmendLifecycleWork(r.Context(), req)
	case "/api/lifecycle/refs/record":
		var req types.RecordLifecycleRefsRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle refs request"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().RecordLifecycleRefs(r.Context(), req)
	case "/api/lifecycle/updates/apply":
		var req types.ApplyLifecycleUpdateRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle update disposition"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().ApplyLifecycleUpdate(r.Context(), req)
	case "/api/lifecycle/work/settle":
		var req types.SettleLifecycleWorkRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle work settlement"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().SettleLifecycleWork(r.Context(), req)
	case "/api/lifecycle/work/refuse":
		var req types.RefuseLifecycleWorkRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle work refusal"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().RefuseLifecycleWork(r.Context(), req)
	case "/api/lifecycle/trajectories/settle":
		var req types.SettleLifecycleTrajectoryRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle trajectory settlement"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().SettleLifecycleTrajectory(r.Context(), req)
	case "/api/lifecycle/artifacts/archive":
		var req types.ArchiveLifecycleArtifactRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle artifact archive"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().ArchiveLifecycleArtifact(r.Context(), req)
	case "/api/lifecycle/updates/queue":
		var req types.QueueLifecycleUpdateRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle update request"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().QueueLifecycleUpdate(r.Context(), req)
	case "/api/lifecycle/trajectories/cancel":
		var req types.CancelLifecycleRequest
		if decodeErr := decoder.Decode(&req); decodeErr != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle cancellation request"})
			return
		}
		req.OwnerID, req.ComputerID = ownerID, computerID
		result, err = h.rt.Store().CancelLifecycleTrajectory(r.Context(), req)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
		return
	}
	if err != nil {
		writeLifecycleAPIError(w, err)
		return
	}
	result.Schema = types.DurableWorkSchemaV1
	writeAPIJSON(w, http.StatusOK, result)
}

func writeLifecycleAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrLifecycleCursorExpired):
		writeAPIJSON(w, http.StatusConflict, types.LifecycleEventPage{Schema: types.DurableWorkSchemaV1, CursorExpired: true, ReplayRequired: true})
	case errors.Is(err, store.ErrNotFound):
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "lifecycle object not found"})
	case errors.Is(err, store.ErrLifecycleCommandConflict), errors.Is(err, store.ErrLifecycleInvalidTransition), errors.Is(err, store.ErrConcurrentStateChange):
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle transition rejected", Reason: err.Error()})
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle transition", Reason: err.Error()})
	}
}
