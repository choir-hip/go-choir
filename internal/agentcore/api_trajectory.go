package agentcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type trajectoryListResponse struct {
	Trajectories []types.TrajectoryRecord `json:"trajectories"`
}

type trajectoryCancelRequest struct {
	IdempotencyKey           string `json:"idempotency_key"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string `json:"expected_head_revision_id"`
	Reason                   string `json:"reason,omitempty"`
}

type trajectoryCancelResponse struct {
	types.LifecycleSnapshot
	TrajectoryID    string                        `json:"trajectory_id"`
	Status          types.TrajectoryStatus        `json:"status"`
	CancelledRunIDs []string                      `json:"cancelled_run_ids"`
	Receipt         types.LifecycleCommandReceipt `json:"receipt"`
	Replay          bool                          `json:"replay"`
}

// HandleTrajectoriesRoot handles GET /api/trajectories: the owner's
// trajectories, most recently updated first.
func (h *APIHandler) HandleTrajectoriesRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	trajectories, err := h.rt.ListTrajectoriesByOwner(r.Context(), ownerID, 200)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list trajectories"})
		return
	}
	writeAPIJSON(w, http.StatusOK, trajectoryListResponse{Trajectories: trajectories})
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

// HandleTrajectoryDetail handles the registered /api/trajectories/{trajectory_id}
// subtree. The cancel child is dispatched separately; GET detail continues to
// answer open obligations from durable state alone.
func (h *APIHandler) HandleTrajectoryDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := trajectoryCancelIDFromPath(r.URL.EscapedPath()); ok || r.Method == http.MethodPost {
		h.HandleTrajectoryCancel(w, r)
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/trajectories/"), "/")
	if strings.HasSuffix(tail, "/stream") {
		trajectoryID := strings.TrimSuffix(tail, "/stream")
		h.streamLifecycleEvents(w, r, ownerID, trajectoryID)
		return
	}
	if strings.HasSuffix(tail, "/events") {
		trajectoryID := strings.TrimSuffix(tail, "/events")
		after, parseErr := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		if r.URL.Query().Get("after") == "" {
			after, parseErr = 0, nil
		}
		limit, limitErr := strconv.Atoi(r.URL.Query().Get("limit"))
		if r.URL.Query().Get("limit") == "" {
			limit, limitErr = 100, nil
		}
		if trajectoryID == "" || parseErr != nil || limitErr != nil || after < 0 || limit <= 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle event cursor"})
			return
		}
		page, pageErr := h.rt.Store().ListLifecycleEventPage(r.Context(), ownerID, h.rt.TextureSandboxID(), trajectoryID, after, limit)
		if pageErr != nil {
			writeLifecycleAPIError(w, pageErr)
			return
		}
		writeAPIJSON(w, http.StatusOK, page)
		return
	}
	const prefix = "/api/trajectories/"
	trajectoryID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if trajectoryID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
		return
	}
	if snapshot, snapshotErr := h.rt.Store().GetLifecycleSnapshot(r.Context(), ownerID, h.rt.TextureSandboxID(), trajectoryID); snapshotErr == nil {
		writeAPIJSON(w, http.StatusOK, snapshot)
		return
	} else if !errors.Is(snapshotErr, store.ErrNotFound) {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load durable lifecycle"})
		return
	}
	obligations, err := h.rt.TrajectoryObligations(r.Context(), ownerID, trajectoryID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load trajectory"})
		return
	}
	writeAPIJSON(w, http.StatusOK, obligations)
}

func (h *APIHandler) streamLifecycleEvents(w http.ResponseWriter, r *http.Request, ownerID, trajectoryID string) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	after, err := strconv.ParseInt(firstNonEmpty(r.URL.Query().Get("after"), "0"), 10, 64)
	if trajectoryID == "" || strings.Contains(trajectoryID, "/") || err != nil || after < 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle stream cursor"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "lifecycle streaming unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	cursor := after
	for {
		page, pageErr := h.rt.Store().ListLifecycleEventPage(r.Context(), ownerID, h.rt.TextureSandboxID(), trajectoryID, cursor, 256)
		if errors.Is(pageErr, store.ErrLifecycleCursorExpired) {
			payload, _ := json.Marshal(types.LifecycleEventPage{Schema: types.DurableWorkSchemaV1, CursorExpired: true, ReplayRequired: true, NextCursor: cursor, Watermark: page.Watermark})
			fmt.Fprintf(w, "event: replay_required\ndata: %s\n\n", payload)
			flusher.Flush()
			return
		}
		if pageErr != nil {
			return
		}
		for _, event := range page.Events {
			event.Schema = types.DurableWorkSchemaV1
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: lifecycle\ndata: %s\n\n", event.ReducerSeq, payload)
			cursor = event.ReducerSeq
		}
		if len(page.Events) > 0 {
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-ticker.C:
		}
	}
}

// HandleTrajectoryCancel handles POST
// /api/trajectories/{trajectory_id}/cancel. The trajectory lookup and all
// cancellation effects are scoped to the authenticated owner.
func (h *APIHandler) HandleTrajectoryCancel(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	trajectoryID, ok := trajectoryCancelIDFromPath(r.URL.EscapedPath())
	if !ok {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
		return
	}
	var request trajectoryCancelRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decodeErr := decoder.Decode(&request); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid cancellation command"})
		return
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.ExpectedHeadRevisionID = strings.TrimSpace(request.ExpectedHeadRevisionID)
	if request.IdempotencyKey == "" || request.ExpectedLifecycleVersion <= 0 || request.ExpectedHeadRevisionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "idempotency_key, expected_lifecycle_version, and expected_head_revision_id are required"})
		return
	}
	if len(request.IdempotencyKey) > 256 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "idempotency_key is too long"})
		return
	}
	result, cancelledRunIDs, err := h.rt.CancelTrajectoryCommand(
		r.Context(), trajectoryID, ownerID, "public-cancel:"+request.IdempotencyKey, strings.TrimSpace(request.Reason),
		request.ExpectedLifecycleVersion, request.ExpectedHeadRevisionID,
	)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
			return
		}
		writeLifecycleAPIError(w, err)
		return
	}
	snapshot, snapshotErr := h.rt.Store().GetLifecycleSnapshot(r.Context(), ownerID, h.rt.TextureSandboxID(), trajectoryID)
	if snapshotErr != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load cancelled lifecycle snapshot"})
		return
	}
	writeAPIJSON(w, http.StatusOK, trajectoryCancelResponse{
		LifecycleSnapshot: snapshot, TrajectoryID: result.Trajectory.TrajectoryID,
		Status: result.Trajectory.Status, CancelledRunIDs: cancelledRunIDs, Receipt: result.Receipt, Replay: result.Replay,
	})
}

func trajectoryCancelIDFromPath(escapedPath string) (string, bool) {
	const prefix = "/api/trajectories/"
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(escapedPath, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "cancel" {
		return "", false
	}
	trajectoryID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(trajectoryID) == "" {
		return "", false
	}
	return trajectoryID, true
}
