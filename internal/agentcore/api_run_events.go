package agentcore

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Owner-scoped run-event read for the Cut 0 trace export
// (docs/reports/reeng-trace-bundle-format-2026-10-12.md, layer run_events).
// It reads the primary run-event reader, Store.ListEventsAfter; the guest
// store stays the authority and the route is read-only.

const (
	defaultRunEventsLimit = 200
	maxRunEventsLimit     = 1000
)

// runEventsResponse is the JSON body of GET /api/runs/{id}/events.
// NextAfter is the seq to pass as ?after= for the next page, or 0 when the
// page is the last one.
type runEventsResponse struct {
	Events    []types.EventRecord `json:"events"`
	NextAfter int64               `json:"next_after"`
}

// clampRunEventsLimit maps a raw ?limit= to a page size: missing, zero,
// negative or malformed values take the default; values above the maximum
// take the maximum.
func clampRunEventsLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultRunEventsLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultRunEventsLimit
	}
	if n > maxRunEventsLimit {
		return maxRunEventsLimit
	}
	return n
}

// handleRunEvents serves GET /api/runs/{id}/events?after=<seq>&limit=<n>.
// A run the caller does not own answers exactly like a missing run.
func (h *APIHandler) handleRunEvents(w http.ResponseWriter, r *http.Request, ownerID, runID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	after := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid run event cursor"})
			return
		}
		after = parsed
	}
	limit := clampRunEventsLimit(r.URL.Query().Get("limit"))
	if _, err := h.rt.GetRun(r.Context(), runID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found"})
		return
	}
	// Read one past the page to learn whether another page exists.
	events, err := h.rt.Store().ListEventsAfter(r.Context(), runID, after, limit+1)
	if err != nil {
		log.Printf("runtime api: list run events: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list run events"})
		return
	}
	resp := runEventsResponse{Events: events}
	if len(events) > limit {
		resp.Events = events[:limit]
		resp.NextAfter = resp.Events[limit-1].Seq
	}
	if resp.Events == nil {
		resp.Events = []types.EventRecord{}
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// runBelongsToTrajectory reports whether a run is part of a trajectory: its
// own TrajectoryID (or the legacy metadata trajectory_id) names it, or it is
// a Management lifecycle-control run whose metadata.assignment_trajectory_id
// names it.
func runBelongsToTrajectory(rec types.RunRecord, trajectoryID string) bool {
	return strings.TrimSpace(rec.TrajectoryID) == trajectoryID ||
		metadataStringValue(rec.Metadata, runMetadataTrajectoryID) == trajectoryID ||
		metadataStringValue(rec.Metadata, "assignment_trajectory_id") == trajectoryID
}

// ListRunsByTrajectory returns the owner's runs of one trajectory, newest
// first, from both the legacy run rows and this computer's lifecycle run
// projections. The scan is unbounded before the trajectory filter so a busy
// owner's other runs cannot push a trajectory's runs out of the window.
func (rt *Runtime) ListRunsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.RunRecord, error) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if limit <= 0 {
		limit = 100
	}
	legacy, err := rt.store.ListRunsByOwner(ctx, ownerID, 1<<30)
	if err != nil {
		return nil, err
	}
	candidates := legacy
	if computerID := strings.TrimSpace(rt.cfg.ComputerID); computerID != "" {
		lifecycle, err := rt.store.ListLifecycleRunsByOwner(ctx, ownerID, computerID, 0)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, lifecycle...)
	}
	seen := make(map[string]bool, len(candidates))
	runs := make([]types.RunRecord, 0)
	for _, rec := range candidates {
		if seen[rec.RunID] || !runBelongsToTrajectory(rec, trajectoryID) {
			continue
		}
		seen[rec.RunID] = true
		runs = append(runs, rec)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}
