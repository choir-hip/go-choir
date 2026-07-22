package agentcore

import (
	"encoding/json"
	"errors"
	"fmt"
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

type trajectoryCancelResponse struct {
	TrajectoryID    string                 `json:"trajectory_id"`
	Status          types.TrajectoryStatus `json:"status"`
	CancelledRunIDs []string               `json:"cancelled_run_ids"`
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
	trajectories, err := h.rt.store.ListTrajectoriesByOwner(r.Context(), ownerID, 200)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list trajectories"})
		return
	}
	writeAPIJSON(w, http.StatusOK, trajectoryListResponse{Trajectories: trajectories})
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
		h.enrichLifecycleActivation(r, ownerID, &snapshot)
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

func (h *APIHandler) enrichLifecycleActivation(r *http.Request, ownerID string, snapshot *types.LifecycleSnapshot) {
	if snapshot == nil || len(snapshot.Agents) == 0 {
		return
	}
	agentID := snapshot.Agents[0].AgentID
	snapshot.Activation = types.LifecycleActivationProjection{AgentID: agentID, State: types.RunPassivated}
	if active, found, err := h.rt.activeRunByAgent(r.Context(), ownerID, agentID); err == nil && found {
		snapshot.Activation.RunID = active.RunID
		snapshot.Activation.State = active.State
	}
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
	trajectory, cancelledRunIDs, err := h.rt.CancelTrajectory(r.Context(), trajectoryID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to cancel trajectory"})
		return
	}
	writeAPIJSON(w, http.StatusOK, trajectoryCancelResponse{
		TrajectoryID:    trajectory.TrajectoryID,
		Status:          trajectory.Status,
		CancelledRunIDs: cancelledRunIDs,
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
