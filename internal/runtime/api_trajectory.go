package runtime

import (
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type trajectoryListResponse struct {
	Trajectories []types.TrajectoryRecord `json:"trajectories"`
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

// HandleTrajectoryDetail handles GET /api/trajectories/{trajectory_id}: the
// trajectory record with its open obligations — "what is this trajectory
// waiting on?" answered from durable state alone.
func (h *APIHandler) HandleTrajectoryDetail(w http.ResponseWriter, r *http.Request) {
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
