package agentcore

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type checkpointListingResponse struct {
	Cadence     CheckpointCadenceStatus             `json:"cadence"`
	Checkpoints []selfdevprotocol.CheckpointSummary `json:"checkpoints"`
}

// listComputerCheckpoints serves GET /api/computers/{id}/lifecycle/checkpoints:
// the computer's checkpoints with the host's verdict on each and the state of
// the cadence that binds them, or, with ?checkpoint_digest=, one checkpoint
// with its receipt, the operand of restore. It reads; nothing here binds.
func (h *APIHandler) listComputerCheckpoints(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	rt := h.rt
	if rt == nil || rt.ownerRecoveryControl == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint authority unavailable"})
		return
	}
	if computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "computer binding does not match runtime"})
		return
	}
	if digest := strings.TrimSpace(r.URL.Query().Get("checkpoint_digest")); digest != "" {
		if !computerevent.IsSHA256(digest) {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "checkpoint_digest must be a sha256 digest"})
			return
		}
		response, err := rt.ownerRecoveryControl.FetchCheckpointByDigest(r.Context(), computerID, digest)
		if err != nil {
			writeAPIJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
			return
		}
		if response == nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "checkpoint not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, response)
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid limit"})
			return
		}
		limit = n
	}
	list, err := rt.ownerRecoveryControl.ListCheckpoints(r.Context(), computerID, limit)
	if err != nil {
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, checkpointListingResponse{Cadence: rt.CheckpointCadenceStatus(), Checkpoints: list.Checkpoints})
}

// resumeCheckpointCadence serves POST /api/computers/{id}/lifecycle/resume-checkpoints,
// the owner act that reopens a cadence closed by two unfinished binds. The proxy
// requires the owner's own authority for it.
func (h *APIHandler) resumeCheckpointCadence(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	rt := h.rt
	if rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint authority unavailable"})
		return
	}
	if computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "computer binding does not match runtime"})
		return
	}
	if err := rt.resumeCheckpointCadence(); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rt.CheckpointCadenceStatus())
}
