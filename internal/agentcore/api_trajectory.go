package agentcore

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

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
	const prefix = "/api/trajectories/"
	trajectoryID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if trajectoryID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
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
