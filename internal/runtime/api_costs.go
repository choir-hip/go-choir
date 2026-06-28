package runtime

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
)

// costsResponse is the JSON envelope for GET /api/costs. All cost figures are
// estimates derived from trace event payloads and the hardcoded pricing table;
// they are not provider invoices.
type costsResponse struct {
	Estimate    bool                   `json:"estimate"`
	Window      string                 `json:"window,omitempty"`
	From        string                 `json:"from,omitempty"`
	To          string                 `json:"to,omitempty"`
	Summary     llmcost.CostSummary    `json:"summary"`
	Entries     []llmcost.CostEntry    `json:"entries,omitempty"`
	KnownModels []llmcost.ModelPricing `json:"known_models,omitempty"`
}

// HandleCosts handles GET /api/costs, returning aggregated LLM cost estimates
// derived from trace events for the authenticated owner.
//
// Query parameters:
//   - limit: max events to scan (default 2000, max 10000)
//   - from:  RFC3339 lower bound on event timestamp (inclusive)
//   - to:    RFC3339 upper bound on event timestamp (inclusive)
//   - detail: if "1" or "true", include per-call entries in the response
//   - models: if "1" or "true", include the known pricing table
func (h *APIHandler) HandleCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	limit := 2000
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10000 {
			limit = n
		}
	}

	var fromTime, toTime time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			fromTime = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			toTime = parsed
		}
	}

	detail := strings.TrimSpace(r.URL.Query().Get("detail")) == "1" ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("detail")), "true")
	includeModels := strings.TrimSpace(r.URL.Query().Get("models")) == "1" ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("models")), "true")

	events, err := h.rt.Store().ListEventsByOwner(r.Context(), ownerID, limit)
	if err != nil {
		log.Printf("runtime costs: list events by owner: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load cost data"})
		return
	}

	// Filter by time window when bounds are provided.
	if !fromTime.IsZero() || !toTime.IsZero() {
		filtered := events[:0]
		for _, ev := range events {
			if !fromTime.IsZero() && ev.Timestamp.Before(fromTime) {
				continue
			}
			if !toTime.IsZero() && ev.Timestamp.After(toTime) {
				continue
			}
			filtered = append(filtered, ev)
		}
		events = filtered
	}

	entries := llmcost.ExtractCostEntries(events)
	summary := llmcost.Aggregate(entries)

	resp := costsResponse{
		Estimate: true,
		Summary:  summary,
	}
	if !fromTime.IsZero() {
		resp.From = fromTime.UTC().Format(time.RFC3339)
	}
	if !toTime.IsZero() {
		resp.To = toTime.UTC().Format(time.RFC3339)
	}
	if !fromTime.IsZero() || !toTime.IsZero() {
		resp.Window = "custom"
	} else {
		resp.Window = "recent"
	}
	if detail {
		resp.Entries = entries
	}
	if includeModels {
		resp.KnownModels = llmcost.KnownModels()
	}

	writeAPIJSON(w, http.StatusOK, resp)
}
