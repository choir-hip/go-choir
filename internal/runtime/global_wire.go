package runtime

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/store"
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
	StoryID         string `json:"story_id"`
	Kind            string `json:"kind"`
	Headline        string `json:"headline"`
	Text            string `json:"text"`
	SourceContentID string `json:"source_content_id,omitempty"`
	UserVTextDocID  string `json:"user_vtext_doc_id,omitempty"`
}

type globalWireSourceSearchRequest struct {
	Query          string `json:"query"`
	MaxResults     int    `json:"max_results,omitempty"`
	StoryID        string `json:"story_id,omitempty"`
	QueueTopResult bool   `json:"queue_top_result,omitempty"`
}

type globalWireSourceSearchResponse struct {
	Status       string                        `json:"status"`
	Source       string                        `json:"source"`
	Query        string                        `json:"query,omitempty"`
	Message      string                        `json:"message,omitempty"`
	Results      []map[string]any              `json:"results,omitempty"`
	ContentItems []types.ContentItem           `json:"content_items,omitempty"`
	Contribution *types.GlobalWireContribution `json:"contribution,omitempty"`
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

// HandleGlobalWireSourceSearch imports configured Source Service evidence into
// owner-scoped Global Wire source artifacts, optionally queueing the top result
// as a source contribution for researcher/reconciler review.
func (h *APIHandler) HandleGlobalWireSourceSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireSourceSearchRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid source search request"})
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	req.StoryID = strings.TrimSpace(req.StoryID)
	if req.Query == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "query is required"})
		return
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 5
	}
	if req.MaxResults > 20 {
		req.MaxResults = 20
	}
	sourceClient := newSourceSearchClientFromEnv()
	if sourceClient == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, globalWireSourceSearchResponse{
			Status:  "unavailable",
			Source:  "source-service",
			Query:   req.Query,
			Message: "Source Service is not configured for this runtime.",
		})
		return
	}
	resp, err := sourceClient.SearchSources(r.Context(), req.Query, req.MaxResults)
	if err != nil {
		writeAPIJSON(w, http.StatusBadGateway, globalWireSourceSearchResponse{
			Status:  "unavailable",
			Source:  "source-service",
			Query:   req.Query,
			Message: err.Error(),
		})
		return
	}
	if len(resp.Results) == 0 {
		writeAPIJSON(w, http.StatusOK, globalWireSourceSearchResponse{
			Status:  "no-evidence",
			Source:  firstNonEmptyString(resp.Provider, sourceapi.ProviderName),
			Query:   resp.Query,
			Results: []map[string]any{},
			Message: "Source Service returned no matching evidence.",
		})
		return
	}
	items := make([]types.ContentItem, 0, len(resp.Results))
	for _, result := range resp.Results {
		item, err := h.ensureGlobalWireSourceServiceContentItem(r, ownerID, result)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to import source service result"})
			return
		}
		items = append(items, item)
	}
	var contribution *types.GlobalWireContribution
	if req.QueueTopResult && req.StoryID != "" && len(items) > 0 {
		top := items[0]
		rec, err := h.rt.Store().CreateGlobalWireContribution(r.Context(), types.GlobalWireContribution{
			ID:              "global-wire-contribution-" + uuid.NewString(),
			OwnerID:         ownerID,
			StoryID:         req.StoryID,
			Kind:            "source",
			Headline:        top.Title,
			Text:            firstNonEmptyString(top.TextContent, "Source Service item queued for researcher review."),
			SourceContentID: top.ContentID,
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to queue source service contribution"})
			return
		}
		contribution = &rec
	}
	writeAPIJSON(w, http.StatusOK, globalWireSourceSearchResponse{
		Status:       "ok",
		Source:       firstNonEmptyString(resp.Provider, sourceapi.ProviderName),
		Query:        resp.Query,
		Results:      resp.Results,
		ContentItems: items,
		Contribution: contribution,
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
		req.SourceContentID = strings.TrimSpace(req.SourceContentID)
		if req.StoryID == "" || req.Kind == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "story_id and kind are required"})
			return
		}
		if req.Text == "" {
			req.Text = "Draft contribution awaiting detail."
		}
		sourceContentID, err := h.createGlobalWireContributionSourceItem(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_content_id was not found for this owner"})
				return
			}
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
	if req.SourceContentID != "" {
		if _, err := h.rt.Store().GetContentItem(r.Context(), ownerID, req.SourceContentID); err != nil {
			return "", err
		}
		return req.SourceContentID, nil
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

func (h *APIHandler) ensureGlobalWireSourceServiceContentItem(r *http.Request, ownerID string, result map[string]any) (types.ContentItem, error) {
	itemID := strings.TrimSpace(stringValue(result["item_id"]))
	if itemID == "" {
		itemID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(researchMustJSON(result))).String()
	}
	contentID := "global-wire-source-service-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+itemID)).String()
	if existing, err := h.rt.Store().GetContentItem(r.Context(), ownerID, contentID); err == nil {
		return existing, nil
	} else if err != store.ErrNotFound {
		return types.ContentItem{}, err
	}
	now := time.Now().UTC()
	body := strings.TrimSpace(stringValue(result["body"]))
	metadata, err := json.Marshal(map[string]any{
		"schema":           "choir.global_wire_source_service_item.v1",
		"target_kind":      firstNonEmptyString(stringValue(result["target_kind"]), sourceapi.TargetKind),
		"source_item_id":   itemID,
		"source_id":        stringValue(result["source_id"]),
		"fetch_id":         stringValue(result["fetch_id"]),
		"original_id":      stringValue(result["original_id"]),
		"published_at":     stringValue(result["published_at"]),
		"fetched_at":       stringValue(result["fetched_at"]),
		"verticals":        result["verticals"],
		"language":         stringValue(result["language"]),
		"region":           stringValue(result["region"]),
		"evidence_level":   stringValue(result["evidence_level"]),
		"vintage_policy":   stringValue(result["vintage_policy"]),
		"lookahead_status": stringValue(result["lookahead_status"]),
		"release_date":     stringValue(result["release_date"]),
		"research_use":     "pending-reconciliation-review",
	})
	if err != nil {
		return types.ContentItem{}, err
	}
	provenance, err := json.Marshal(map[string]any{
		"created_from":        "source_service_search",
		"provider":            sourceapi.ProviderName,
		"source_service_item": itemID,
		"created_at":          now.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return types.ContentItem{}, err
	}
	item := types.ContentItem{
		ContentID:    contentID,
		OwnerID:      ownerID,
		SourceType:   "source_service_item",
		MediaType:    "text/plain",
		AppHint:      "global-wire",
		Title:        firstNonEmptyString(stringValue(result["title"]), "Source Service item "+itemID),
		SourceURL:    stringValue(result["url"]),
		CanonicalURL: stringValue(result["canonical_url"]),
		TextContent:  body,
		ContentHash:  firstNonEmptyString(stringValue(result["content_hash"]), contentHash(body)),
		Metadata:     metadata,
		Provenance:   provenance,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if item.CanonicalURL == "" {
		item.CanonicalURL = item.SourceURL
	}
	if err := h.rt.Store().CreateContentItem(r.Context(), item); err != nil {
		return types.ContentItem{}, err
	}
	_, _ = h.rt.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventContentItemCreated, contentItemEventPayload(item))
	return item, nil
}
