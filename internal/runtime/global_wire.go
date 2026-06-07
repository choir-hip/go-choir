package runtime

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type globalWireStoriesResponse struct {
	Stories      []types.GlobalWireStory       `json:"stories"`
	StyleSources []types.GlobalWireStyleSource `json:"style_sources"`
	Source       string                        `json:"source"`
}

type globalWireContributionListResponse struct {
	Contributions []types.GlobalWireContribution `json:"contributions"`
}

type globalWireContributionCreateRequest struct {
	StoryID        string `json:"story_id"`
	Kind           string `json:"kind"`
	Headline       string `json:"headline"`
	Text           string `json:"text"`
	UserVTextDocID string `json:"user_vtext_doc_id,omitempty"`
}

// HandleGlobalWireStories returns the authenticated owner's durable StoryGraph.
func (h *APIHandler) HandleGlobalWireStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	stories, err := h.rt.Store().ListGlobalWireStories(r.Context(), ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load global wire StoryGraph"})
		return
	}
	styleSources := []types.GlobalWireStyleSource{}
	if len(stories) > 0 {
		styleSources = stories[0].StyleSources
	}
	writeAPIJSON(w, http.StatusOK, globalWireStoriesResponse{
		Stories:      stories,
		StyleSources: styleSources,
		Source:       "durable-storygraph",
	})
}

// HandleGlobalWireContributions lists and creates owner-owned contribution
// records for later research/reconciliation.
func (h *APIHandler) HandleGlobalWireContributions(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		contributions, err := h.rt.Store().ListGlobalWireContributions(r.Context(), ownerID, storyID, 20)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list global wire contributions"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireContributionListResponse{Contributions: contributions})
	case http.MethodPost:
		var req globalWireContributionCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid contribution request"})
			return
		}
		req.StoryID = strings.TrimSpace(req.StoryID)
		req.Kind = strings.TrimSpace(req.Kind)
		req.Text = strings.TrimSpace(req.Text)
		if req.StoryID == "" || req.Kind == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "story_id and kind are required"})
			return
		}
		if req.Text == "" {
			req.Text = "Draft contribution awaiting detail."
		}
		sourceContentID, err := h.createGlobalWireContributionSourceItem(r, ownerID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create contribution source item"})
			return
		}
		rec, err := h.rt.Store().CreateGlobalWireContribution(r.Context(), types.GlobalWireContribution{
			ID:              "global-wire-contribution-" + uuid.NewString(),
			OwnerID:         ownerID,
			StoryID:         req.StoryID,
			Kind:            req.Kind,
			Headline:        strings.TrimSpace(req.Headline),
			Text:            req.Text,
			SourceContentID: sourceContentID,
			UserVTextDocID:  strings.TrimSpace(req.UserVTextDocID),
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create global wire contribution"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, rec)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) createGlobalWireContributionSourceItem(r *http.Request, ownerID string, req globalWireContributionCreateRequest) (string, error) {
	switch req.Kind {
	case "source", "counter-source":
	default:
		return "", nil
	}
	now := time.Now().UTC()
	contentID := uuid.NewString()
	metadata, err := json.Marshal(map[string]any{
		"schema":       "choir.global_wire_user_source_contribution.v1",
		"story_id":     req.StoryID,
		"kind":         req.Kind,
		"research_use": "pending-reconciliation-review",
	})
	if err != nil {
		return "", err
	}
	provenance, err := json.Marshal(map[string]any{
		"created_from": "global_wire_user_contribution",
		"story_id":     req.StoryID,
		"created_at":   now.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return "", err
	}
	item := types.ContentItem{
		ContentID:   contentID,
		OwnerID:     ownerID,
		SourceType:  "text",
		MediaType:   "text/markdown",
		AppHint:     "global-wire",
		Title:       "Contribution source: " + strings.TrimSpace(req.Headline),
		TextContent: req.Text,
		ContentHash: uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+req.StoryID+":"+req.Kind+":"+req.Text)).String(),
		Metadata:    metadata,
		Provenance:  provenance,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if strings.TrimSpace(item.Title) == "Contribution source:" {
		item.Title = "Global Wire contribution source"
	}
	if err := h.rt.Store().CreateContentItem(r.Context(), item); err != nil {
		return "", err
	}
	_, _ = h.rt.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventContentItemCreated, contentItemEventPayload(item))
	return contentID, nil
}
