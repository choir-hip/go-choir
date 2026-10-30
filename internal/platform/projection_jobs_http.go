package platform

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

func (h *Handler) HandleProjectionJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, 503, apiError{Error: "checkpoint control unavailable"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	var input struct {
		ComputerID    string `json:"computer_id"`
		Reason        string `json:"reason"`
		GenesisRepair bool   `json:"genesis_repair"`
	}
	switch r.Method {
	case http.MethodGet:
		if !h.authorizeFileCAS(r, computerID, "event:read") && !h.authorizeFileCAS(r, computerID, "computer:lifecycle") {
			writeJSON(w, 403, apiError{Error: "computer capability required"})
			return
		}
		j, err := h.service.store.ProjectionJob(r.Context(), computerID)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSON(w, 503, apiError{Error: "checkpoint state unavailable"})
			return
		}
		writeJSON(w, 200, j)
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if !decodeFileCASJSON(r, &input) || !safeFileCASComponent(input.ComputerID) {
			writeJSON(w, 400, apiError{Error: "invalid checkpoint request"})
			return
		}
		if !h.authorizeFileCAS(r, input.ComputerID, "event:append") && !h.authorizeFileCAS(r, input.ComputerID, "computer:lifecycle") {
			writeJSON(w, 403, apiError{Error: "computer capability required"})
			return
		}
		// Genesis reconstruction is exceptional repair, not the scheduler's fallback.
		if input.GenesisRepair && !trustedInternalCaller(r) {
			writeJSON(w, 403, apiError{Error: "genesis repair requires internal operator authority"})
			return
		}
		j, err := h.service.store.EnqueueProjectionJob(r.Context(), input.ComputerID, input.Reason, input.GenesisRepair)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSON(w, 503, apiError{Error: "failed to enqueue checkpoint"})
			return
		}
		writeJSON(w, http.StatusAccepted, j)
	default:
		writeJSON(w, 405, apiError{Error: "method not allowed"})
	}
}
