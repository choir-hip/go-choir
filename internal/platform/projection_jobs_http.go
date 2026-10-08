package platform

import (
	"database/sql"
	"encoding/json"
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

func (h *Handler) HandleProjectionBasePins(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, 503, apiError{Error: "checkpoint control unavailable"})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeJSON(w, 405, apiError{Error: "method not allowed"})
		return
	}
	var input struct {
		ComputerID string `json:"computer_id"`
		BaseRef    string `json:"base_ref"`
		Reference  string `json:"reference"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if !decodeFileCASJSON(r, &input) || !safeFileCASComponent(input.ComputerID) || !validFileCASDigest(input.BaseRef) || input.Reference == "" || len(input.Reference) > 255 {
		writeJSON(w, 400, apiError{Error: "invalid pin request"})
		return
	}
	if !h.authorizeFileCAS(r, input.ComputerID, "computer:lifecycle") {
		writeJSON(w, 403, apiError{Error: "computer lifecycle capability required"})
		return
	}
	h.service.writeMu.Lock()
	defer h.service.writeMu.Unlock()
	raw, err := h.projectionBaseSidecar(input.BaseRef)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// A base cannot be pinned or released through another computer's scope.
	var descriptor struct {
		ComputerID string `json:"computer_id"`
	}
	if json.Unmarshal(raw, &descriptor) != nil || descriptor.ComputerID != input.ComputerID {
		writeJSON(w, 409, apiError{Error: "base computer mismatch"})
		return
	}
	if r.Method == http.MethodPost {
		err = h.service.store.PinProjectionBase(r.Context(), input.ComputerID, input.BaseRef, input.Reference)
	} else {
		_, err = h.service.store.db.ExecContext(r.Context(), `DELETE FROM computer_projection_base_pins WHERE computer_id=? AND base_ref=? AND reference_id=?`, input.ComputerID, input.BaseRef, input.Reference)
		if err == nil {
			err = h.service.store.commitBoundary(r.Context(), "release projection base pin "+input.ComputerID)
		}
	}
	if err != nil {
		writeJSON(w, 503, apiError{Error: "checkpoint pin unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"computer_id": input.ComputerID, "base_ref": input.BaseRef, "reference": input.Reference, "pinned": r.Method == http.MethodPost})
}
