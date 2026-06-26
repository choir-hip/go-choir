package runtime

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type internalSourcecycledWebCapturesRequest struct {
	OwnerID    string         `json:"owner_id"`
	ComputerID string         `json:"computer_id,omitempty"`
	Items      []sources.Item `json:"items"`
	Now        string         `json:"now,omitempty"`
}

type internalSourcecycledWebCapturesResponse struct {
	Status            string `json:"status"`
	CaptureCount      int    `json:"capture_count"`
	SourceEntityCount int    `json:"source_entity_count"`
	CapturedFromEdges int    `json:"captured_from_edges"`
	SkippedItemCount  int    `json:"skipped_item_count"`
}

// HandleInternalSourcecycledWebCaptures projects source-service items into this
// runtime's durable objectgraph. It is internal-only; browser clients should
// consume the resulting objects through the normal Universal Wire read route.
func (h *APIHandler) HandleInternalSourcecycledWebCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if h == nil || h.rt == nil || h.rt.ObjectGraph() == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	var req internalSourcecycledWebCapturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		ownerID = universalWirePlatformOwnerID()
	}
	if ownerID != universalWirePlatformOwnerID() {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsupported sourcecycled owner"})
		return
	}
	now := time.Now().UTC()
	if rawNow := strings.TrimSpace(req.Now); rawNow != "" {
		parsed, err := time.Parse(time.RFC3339Nano, rawNow)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid now timestamp"})
			return
		}
		now = parsed.UTC()
	}
	result, err := sourcegraph.WriteWebCaptureGraphObjects(r.Context(), h.rt.ObjectGraph(), req.Items, sourcegraph.WebCaptureGraphProjectionConfig{
		OwnerID:    ownerID,
		ComputerID: strings.TrimSpace(req.ComputerID),
		Now:        now,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, internalSourcecycledWebCapturesResponse{
		Status:            "ok",
		CaptureCount:      len(result.Captures),
		SourceEntityCount: len(result.SourceEntities),
		CapturedFromEdges: result.EdgeCount,
		SkippedItemCount:  result.Skipped,
	})
}
