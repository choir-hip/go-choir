package textureowner

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
)

// The owner's related-review act
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice d):
// POST {batch_id, limit, from} asks the research desks of this computer's stories
// with no entities record to record what each story is about, newest
// first. A repeat of a batch replays it.

const (
	relatedReviewPath   = "/api/texture/related-review"
	relatedReviewSchema = "choir.texture.related_review.v1"
)

type relatedReviewHTTPRequest struct {
	BatchID string `json:"batch_id"`
	Limit   int    `json:"limit"`
	// From is the previous act's next, to go on from it; empty starts at
	// the newest story.
	From agentcore.StoryEntitiesBackfillCursor `json:"from"`
}

type relatedReviewHTTPReceipt struct {
	Schema string `json:"schema"`
	agentcore.StoryEntitiesBackfillResult
}

// HandleTextureRelatedReview serves POST /api/texture/related-review.
func (h *Handler) HandleTextureRelatedReview(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var body relatedReviewHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.BatchID) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "batch_id and limit are required"})
		return
	}
	result, err := h.Core.StoryEntitiesBackfillFrom(r.Context(), ownerID, h.Core.TextureComputerID(), body.BatchID, body.Limit, body.From)
	switch {
	case errors.Is(err, agentcore.ErrStoryEntitiesBackfillOff):
		writeAPIJSON(w, http.StatusConflict, storyResearchRefusal{Error: err.Error(), Reason: "off"})
	case errors.Is(err, agentcore.ErrStoryEntitiesBackfillBadRequest):
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
	case err != nil:
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "related review failed"})
	default:
		writeAPIJSON(w, http.StatusAccepted, relatedReviewHTTPReceipt{Schema: relatedReviewSchema, StoryEntitiesBackfillResult: result})
	}
}
