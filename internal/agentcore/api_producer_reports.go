package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/store"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// SettleLifecycleProducerReports settles pending producer reports via store-layer CAS transition.
// The runtime derives the command digest server-side from the canonical received
// fields — the same owner-facing command pattern as cancelTrajectoryAuthorityCommand —
// so product-path callers bind their commitment with the request body itself.
func (rt *Runtime) SettleLifecycleProducerReports(ctx context.Context, req types.SettleLifecycleProducerReportsRequest) (types.LifecycleResult, error) {
	if rt == nil || rt.store == nil {
		return types.LifecycleResult{}, fmt.Errorf("settle producer reports: store unavailable")
	}
	if req.IncludeDeliveredStale {
		// Delivered-stale settlement must never tombstone the live work of the
		// resident Super: reports delivered to the currently active run are that
		// run's execution input, everything else delivered is storm-era residue.
		ownerID := strings.TrimSpace(req.OwnerID)
		if resident, found, err := rt.activeRunByAgent(ctx, ownerID, persistentSuperAgentID(ownerID)); err != nil {
			return types.LifecycleResult{}, fmt.Errorf("settle producer reports: resolve resident super: %w", err)
		} else if found {
			req.ExcludeDeliveredToRunID = resident.RunID
		}
	}
	req.CommandID = strings.TrimSpace(req.CommandID)
	if req.CommandID == "" {
		req.CommandID = "lifecycle-settle-producer-reports:" + strings.TrimSpace(req.TrajectoryID)
	}
	digest, err := store.ComputeSettleLifecycleProducerReportsDigest(req)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("settle producer reports: %w", err)
	}
	req.CommandDigest = digest
	return rt.store.SettleLifecycleProducerReports(ctx, req)
}

// ListPendingProducerReports returns pending undelivered producer reports for the persistent Super.
func (rt *Runtime) ListPendingProducerReports(ctx context.Context, ownerID, computerID, requestedByAgentID string) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("list pending producer reports: store unavailable")
	}
	return rt.store.ListPendingProducerReports(ctx, ownerID, computerID, requestedByAgentID)
}

func (h *APIHandler) listPendingProducerReports(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "producer reports authority unavailable"})
		return
	}
	requestedByAgentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	reports, err := h.rt.ListPendingProducerReports(r.Context(), ownerID, computerID, requestedByAgentID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"count":   len(reports),
	})
}

func (h *APIHandler) settleProducerReports(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "producer reports authority unavailable"})
		return
	}
	var req types.SettleLifecycleProducerReportsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body: " + err.Error()})
		return
	}
	req.OwnerID = ownerID
	req.ComputerID = computerID
	result, err := h.rt.SettleLifecycleProducerReports(r.Context(), req)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, result)
}
