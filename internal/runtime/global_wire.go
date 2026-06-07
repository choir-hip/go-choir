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

type globalWireSourceRefreshRequest struct {
	StoryID    string `json:"story_id"`
	Query      string `json:"query,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type globalWireSourceRefreshResponse struct {
	Status       string                                  `json:"status"`
	Source       string                                  `json:"source"`
	Query        string                                  `json:"query,omitempty"`
	Message      string                                  `json:"message,omitempty"`
	RefreshRun   types.GlobalWireSourceRefreshRun        `json:"refresh_run"`
	ContentItem  *types.ContentItem                      `json:"content_item,omitempty"`
	Contribution *types.GlobalWireContribution           `json:"contribution,omitempty"`
	Decision     *types.GlobalWireReconciliationDecision `json:"decision,omitempty"`
	Candidate    *types.GlobalWireGraphUpdateCandidate   `json:"candidate,omitempty"`
}

type globalWireReconciliationResponse struct {
	Contributions     []types.GlobalWireContribution           `json:"contributions"`
	SourceItems       map[string]types.ContentItem             `json:"source_items,omitempty"`
	Decisions         []types.GlobalWireReconciliationDecision `json:"decisions"`
	Candidates        []types.GlobalWireGraphUpdateCandidate   `json:"candidates"`
	Promotions        []types.GlobalWireGraphPromotionDecision `json:"promotions"`
	Refreshes         []types.GlobalWireSourceRefreshRun       `json:"refreshes"`
	ProjectionReviews []types.GlobalWireProjectionReview       `json:"projection_reviews"`
}

type globalWireReconciliationCreateRequest struct {
	ContributionID string `json:"contribution_id"`
	Decision       string `json:"decision"`
	Note           string `json:"note,omitempty"`
}

type globalWireReconciliationCreateResponse struct {
	Decision     types.GlobalWireReconciliationDecision `json:"decision"`
	Contribution types.GlobalWireContribution           `json:"contribution"`
	SourceItem   *types.ContentItem                     `json:"source_item,omitempty"`
	Candidate    *types.GlobalWireGraphUpdateCandidate  `json:"candidate,omitempty"`
}

type globalWireGraphCandidateReviewRequest struct {
	CandidateID string `json:"candidate_id"`
	Decision    string `json:"decision"`
	Note        string `json:"note,omitempty"`
}

type globalWireProjectionReviewDraftRequest struct {
	ReviewID string `json:"review_id"`
	Action   string `json:"action,omitempty"`
}

type globalWireProjectionReviewDraftResponse struct {
	Review     types.GlobalWireProjectionReview `json:"review"`
	Document   types.Document                   `json:"document"`
	Revision   types.Revision                   `json:"revision"`
	Projection types.GlobalWireStoryProjection  `json:"projection,omitempty"`
}

type globalWireGraphCandidateReviewResponse struct {
	Candidate         types.GlobalWireGraphUpdateCandidate   `json:"candidate"`
	Promotion         types.GlobalWireGraphPromotionDecision `json:"promotion"`
	Story             types.GlobalWireStory                  `json:"story,omitempty"`
	ProjectionReviews []types.GlobalWireProjectionReview     `json:"projection_reviews,omitempty"`
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

// HandleGlobalWireSourceRefresh runs a bounded source-ingestion/classification
// pass for one StoryGraph node. It creates review artifacts and a graph-update
// candidate, but does not mutate the StoryGraph manifest.
func (h *APIHandler) HandleGlobalWireSourceRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireSourceRefreshRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid source refresh request"})
		return
	}
	req.StoryID = strings.TrimSpace(req.StoryID)
	req.Query = strings.TrimSpace(req.Query)
	if req.StoryID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "story_id is required"})
		return
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 3
	}
	if req.MaxResults > 10 {
		req.MaxResults = 10
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, req.StoryID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "story not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load StoryGraph"})
		return
	}
	query := req.Query
	if query == "" {
		query = story.Headline
	}
	sourceClient := newSourceSearchClientFromEnv()
	if sourceClient == nil {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			StoryID:  story.ID,
			Query:    query,
			Status:   "unavailable",
			Provider: "source-service",
			Message:  "Source Service is not configured for this runtime.",
		})
		if runErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
			return
		}
		writeAPIJSON(w, http.StatusServiceUnavailable, globalWireSourceRefreshResponse{
			Status:     run.Status,
			Source:     run.Provider,
			Query:      run.Query,
			Message:    run.Message,
			RefreshRun: run,
		})
		return
	}
	resp, err := sourceClient.SearchSources(r.Context(), query, req.MaxResults)
	if err != nil {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			StoryID:  story.ID,
			Query:    query,
			Status:   "unavailable",
			Provider: "source-service",
			Message:  err.Error(),
		})
		if runErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
			return
		}
		writeAPIJSON(w, http.StatusBadGateway, globalWireSourceRefreshResponse{
			Status:     run.Status,
			Source:     run.Provider,
			Query:      run.Query,
			Message:    run.Message,
			RefreshRun: run,
		})
		return
	}
	provider := firstNonEmptyString(resp.Provider, sourceapi.ProviderName)
	if len(resp.Results) == 0 {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			StoryID:  story.ID,
			Query:    firstNonEmptyString(resp.Query, query),
			Status:   "no-evidence",
			Provider: provider,
			Message:  "Source Service returned no matching evidence.",
		})
		if runErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireSourceRefreshResponse{
			Status:     run.Status,
			Source:     run.Provider,
			Query:      run.Query,
			Message:    run.Message,
			RefreshRun: run,
		})
		return
	}
	item, err := h.ensureGlobalWireSourceServiceContentItem(r, ownerID, resp.Results[0])
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to import source refresh result"})
		return
	}
	contribution, decision, candidate, err := h.createGlobalWireSourceRefreshArtifacts(r, ownerID, story, item)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create source refresh artifacts"})
		return
	}
	run, err := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
		ID:              "global-wire-source-refresh-" + uuid.NewString(),
		OwnerID:         ownerID,
		StoryID:         story.ID,
		Query:           firstNonEmptyString(resp.Query, query),
		Status:          "candidate-review",
		Provider:        provider,
		Message:         "Source refresh imported live evidence and created a non-mutating graph-update candidate for platform review.",
		SourceContentID: item.ContentID,
		ContributionID:  contribution.ID,
		DecisionID:      decision.ID,
		CandidateID:     candidate.ID,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireSourceRefreshResponse{
		Status:       run.Status,
		Source:       run.Provider,
		Query:        run.Query,
		Message:      run.Message,
		RefreshRun:   run,
		ContentItem:  &item,
		Contribution: &contribution,
		Decision:     &decision,
		Candidate:    &candidate,
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

// HandleGlobalWireReconciliation exposes the research/reconciliation queue and
// records owner-scoped decisions without mutating platform StoryGraph stories.
func (h *APIHandler) HandleGlobalWireReconciliation(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		contributions, err := h.rt.Store().ListGlobalWireContributions(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list reconciliation contributions"})
			return
		}
		decisions, err := h.rt.Store().ListGlobalWireReconciliationDecisions(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list reconciliation decisions"})
			return
		}
		candidates, err := h.rt.Store().ListGlobalWireGraphUpdateCandidates(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list graph update candidates"})
			return
		}
		promotions, err := h.rt.Store().ListGlobalWireGraphPromotionDecisions(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list graph promotion decisions"})
			return
		}
		refreshes, err := h.rt.Store().ListGlobalWireSourceRefreshRuns(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list source refresh runs"})
			return
		}
		projectionReviews, err := h.rt.Store().ListGlobalWireProjectionReviews(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list projection reviews"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireReconciliationResponse{
			Contributions:     contributions,
			SourceItems:       h.globalWireContributionSourceItems(r, ownerID, contributions),
			Decisions:         decisions,
			Candidates:        candidates,
			Promotions:        promotions,
			Refreshes:         refreshes,
			ProjectionReviews: projectionReviews,
		})
	case http.MethodPost:
		var req globalWireReconciliationCreateRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid reconciliation request"})
			return
		}
		req.ContributionID = strings.TrimSpace(req.ContributionID)
		req.Decision = normalizeGlobalWireReconciliationDecision(req.Decision)
		req.Note = strings.TrimSpace(req.Note)
		if req.ContributionID == "" || req.Decision == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "contribution_id and decision are required"})
			return
		}
		contribution, err := h.rt.Store().GetGlobalWireContribution(r.Context(), ownerID, req.ContributionID)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "contribution not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load contribution"})
			return
		}
		state := "rejected-by-review"
		if req.Decision == "accepted" {
			state = "accepted-for-graph-review"
		}
		updatedContribution, err := h.rt.Store().UpdateGlobalWireContributionResearchState(r.Context(), ownerID, req.ContributionID, state)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update contribution state"})
			return
		}
		decision, err := h.rt.Store().CreateGlobalWireReconciliationDecision(r.Context(), types.GlobalWireReconciliationDecision{
			ID:              "global-wire-reconciliation-" + uuid.NewString(),
			OwnerID:         ownerID,
			ContributionID:  contribution.ID,
			StoryID:         contribution.StoryID,
			Decision:        req.Decision,
			Note:            req.Note,
			SourceContentID: contribution.SourceContentID,
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create reconciliation decision"})
			return
		}
		var candidate *types.GlobalWireGraphUpdateCandidate
		if req.Decision == "accepted" {
			story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, contribution.StoryID)
			if err != nil {
				writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load StoryGraph for graph update candidate"})
				return
			}
			rec := h.globalWireGraphUpdateCandidate(ownerID, story, updatedContribution, decision)
			saved, err := h.rt.Store().UpsertGlobalWireGraphUpdateCandidate(r.Context(), rec)
			if err != nil {
				writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create graph update candidate"})
				return
			}
			candidate = &saved
		}
		var sourceItem *types.ContentItem
		if contribution.SourceContentID != "" {
			item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, contribution.SourceContentID)
			if err == nil {
				sourceItem = &item
			}
		}
		writeAPIJSON(w, http.StatusCreated, globalWireReconciliationCreateResponse{
			Decision:     decision,
			Contribution: updatedContribution,
			SourceItem:   sourceItem,
			Candidate:    candidate,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func normalizeGlobalWireReconciliationDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "accept", "accepted":
		return "accepted"
	case "reject", "rejected":
		return "rejected"
	default:
		return ""
	}
}

// HandleGlobalWireGraphCandidates records explicit platform review over graph
// update candidates. Promotion may apply a bounded source-manifest update;
// rejection records review state without changing the StoryGraph.
func (h *APIHandler) HandleGlobalWireGraphCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireGraphCandidateReviewRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid graph candidate review request"})
		return
	}
	req.CandidateID = strings.TrimSpace(req.CandidateID)
	req.Decision = normalizeGlobalWirePromotionDecision(req.Decision)
	req.Note = strings.TrimSpace(req.Note)
	if req.CandidateID == "" || req.Decision == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "candidate_id and decision are required"})
		return
	}
	candidate, err := h.rt.Store().GetGlobalWireGraphUpdateCandidate(r.Context(), ownerID, req.CandidateID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "graph candidate not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load graph candidate"})
		return
	}
	status := "promotion-rejected"
	appliedChange := "no StoryGraph mutation; candidate rejected by platform review"
	var story types.GlobalWireStory
	if req.Decision == "promoted" {
		story, appliedChange, err = h.applyGlobalWireGraphCandidate(r, ownerID, candidate)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate source or story not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to apply graph candidate"})
			return
		}
		status = "promoted-to-storygraph"
	}
	updatedCandidate, err := h.rt.Store().UpdateGlobalWireGraphUpdateCandidateStatus(r.Context(), ownerID, candidate.ID, status)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update graph candidate status"})
		return
	}
	promotion, err := h.rt.Store().CreateGlobalWireGraphPromotionDecision(r.Context(), types.GlobalWireGraphPromotionDecision{
		ID:              "global-wire-graph-promotion-" + uuid.NewString(),
		OwnerID:         ownerID,
		CandidateID:     candidate.ID,
		StoryID:         candidate.StoryID,
		Decision:        req.Decision,
		Note:            req.Note,
		AppliedChange:   appliedChange,
		SourceContentID: candidate.SourceContentID,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create graph promotion decision"})
		return
	}
	projectionReviews := []types.GlobalWireProjectionReview{}
	if req.Decision == "promoted" && strings.TrimSpace(candidate.ProjectionAction) == "projection-review-required" {
		projectionReviews, err = h.createGlobalWireProjectionReviews(r, ownerID, story, updatedCandidate, promotion)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create projection reviews"})
			return
		}
	}
	writeAPIJSON(w, http.StatusCreated, globalWireGraphCandidateReviewResponse{
		Candidate:         updatedCandidate,
		Promotion:         promotion,
		Story:             story,
		ProjectionReviews: projectionReviews,
	})
}

func normalizeGlobalWirePromotionDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "promote", "promoted":
		return "promoted"
	case "reject", "rejected":
		return "rejected"
	default:
		return ""
	}
}

// HandleGlobalWireProjectionReviews creates ordinary VText drafts from
// projection-review obligations without publishing or mutating platform stories.
func (h *APIHandler) HandleGlobalWireProjectionReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireProjectionReviewDraftRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid projection review request"})
		return
	}
	req.ReviewID = strings.TrimSpace(req.ReviewID)
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))
	if req.Action == "" {
		req.Action = "draft"
	}
	if req.ReviewID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "review_id is required"})
		return
	}
	if req.Action != "draft" && req.Action != "approve" && req.Action != "reject" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "action must be draft, approve, or reject"})
		return
	}
	review, err := h.rt.Store().GetGlobalWireProjectionReview(r.Context(), ownerID, req.ReviewID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "projection review not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load projection review"})
		return
	}
	if req.Action == "reject" {
		review, err = h.rt.Store().MarkGlobalWireProjectionReviewRejected(r.Context(), ownerID, review.ID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to reject projection review"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireProjectionReviewDraftResponse{Review: review})
		return
	}
	if req.Action == "approve" {
		doc, rev, projection, review, err := h.approveGlobalWireProjectionReview(r, ownerID, review)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "projection review, draft, story, or projection not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to approve projection review"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireProjectionReviewDraftResponse{
			Review:     review,
			Document:   doc,
			Revision:   rev,
			Projection: projection,
		})
		return
	}
	doc, rev, review, err := h.ensureGlobalWireProjectionReviewDraft(r, ownerID, review)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "projection review source, story, or draft not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create projection draft"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireProjectionReviewDraftResponse{
		Review:   review,
		Document: doc,
		Revision: rev,
	})
}

func (h *APIHandler) applyGlobalWireGraphCandidate(r *http.Request, ownerID string, candidate types.GlobalWireGraphUpdateCandidate) (types.GlobalWireStory, string, error) {
	if strings.TrimSpace(candidate.SourceContentID) == "" {
		return types.GlobalWireStory{}, "", store.ErrNotFound
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, candidate.StoryID)
	if err != nil {
		return types.GlobalWireStory{}, "", err
	}
	item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, candidate.SourceContentID)
	if err != nil {
		return types.GlobalWireStory{}, "", err
	}
	source := globalWireSourceItemFromContentItem(candidate, item)
	tier := strings.TrimSpace(candidate.SourceTier)
	added := false
	switch tier {
	case "lead":
		story.Manifest.Lead, added = appendGlobalWireSourceIfMissing(story.Manifest.Lead, source)
	case "supporting":
		story.Manifest.Supporting, added = appendGlobalWireSourceIfMissing(story.Manifest.Supporting, source)
	case "contrary":
		story.Manifest.Contrary, added = appendGlobalWireSourceIfMissing(story.Manifest.Contrary, source)
	default:
		tier = "context"
		story.Manifest.Context, added = appendGlobalWireSourceIfMissing(story.Manifest.Context, source)
	}
	story.SourceState = "platform-review-promoted-source"
	story.ChangeState = firstNonEmptyString(story.ChangeState, "source manifest updated")
	story.UpdatedAt = time.Now().UTC()
	if err := h.rt.Store().UpsertGlobalWireStory(r.Context(), story); err != nil {
		return types.GlobalWireStory{}, "", err
	}
	if !added {
		return story, "source already present in " + tier + " manifest tier; promotion recorded without duplicate source", nil
	}
	return story, "appended source_content_id " + item.ContentID + " to " + tier + " manifest tier", nil
}

func globalWireSourceItemFromContentItem(candidate types.GlobalWireGraphUpdateCandidate, item types.ContentItem) types.GlobalWireSourceItem {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = firstNonEmptyString(candidate.Title, "Promoted Global Wire source")
	}
	return types.GlobalWireSourceItem{
		ID:           "source-" + item.ContentID,
		ContentID:    item.ContentID,
		Title:        title,
		Standing:     firstNonEmptyString(item.SourceType, "reviewed source artifact"),
		Role:         firstNonEmptyString(candidate.SourceTier, "context"),
		CanonicalURL: firstNonEmptyString(item.CanonicalURL, item.SourceURL),
	}
}

func appendGlobalWireSourceIfMissing(items []types.GlobalWireSourceItem, source types.GlobalWireSourceItem) ([]types.GlobalWireSourceItem, bool) {
	for _, item := range items {
		if strings.TrimSpace(item.ContentID) == source.ContentID && source.ContentID != "" {
			return items, false
		}
	}
	return append(items, source), true
}

func (h *APIHandler) globalWireGraphUpdateCandidate(ownerID string, story types.GlobalWireStory, contribution types.GlobalWireContribution, decision types.GlobalWireReconciliationDecision) types.GlobalWireGraphUpdateCandidate {
	sourceTier := "context"
	edgeKind := "source-neighborhood-update"
	projectionAction := "projection-review-required"
	candidateKind := "source-manifest-update"
	switch contribution.Kind {
	case "source":
		sourceTier = "supporting"
		edgeKind = "shared-source-neighborhood"
	case "counter-source", "claim-dispute":
		sourceTier = "contrary"
		edgeKind = "contradiction-or-qualification"
		candidateKind = "contradiction-added"
	case "argument":
		sourceTier = "context"
		edgeKind = "claim-overlap"
		candidateKind = "claim-context-update"
	case "research-request":
		sourceTier = "context"
		edgeKind = "retrieval-demand"
		candidateKind = "research-followup"
		projectionAction = "no-projection-change-yet"
	}
	title := strings.TrimSpace(contribution.Headline)
	if title == "" {
		title = story.Headline
	}
	summary := strings.TrimSpace(contribution.Text)
	if summary == "" {
		summary = "Accepted contribution queued as a graph update candidate."
	}
	if len(summary) > 280 {
		summary = summary[:280]
	}
	return types.GlobalWireGraphUpdateCandidate{
		ID:               "global-wire-graph-candidate-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+decision.ID)).String(),
		OwnerID:          ownerID,
		StoryID:          story.ID,
		ContributionID:   contribution.ID,
		DecisionID:       decision.ID,
		SourceContentID:  contribution.SourceContentID,
		CandidateKind:    candidateKind,
		Title:            title,
		Summary:          summary,
		SourceTier:       sourceTier,
		EdgeKind:         edgeKind,
		ProjectionAction: projectionAction,
		Status:           "candidate-review",
		Rationale:        "Accepted reconciliation decision created a non-mutating StoryGraph update candidate; platform StoryGraph review is still required before manifest, edge, prominence, or projection changes.",
	}
}

func (h *APIHandler) createGlobalWireSourceRefreshArtifacts(r *http.Request, ownerID string, story types.GlobalWireStory, item types.ContentItem) (types.GlobalWireContribution, types.GlobalWireReconciliationDecision, types.GlobalWireGraphUpdateCandidate, error) {
	contribution, err := h.rt.Store().CreateGlobalWireContribution(r.Context(), types.GlobalWireContribution{
		ID:              "global-wire-contribution-" + uuid.NewString(),
		OwnerID:         ownerID,
		StoryID:         story.ID,
		Kind:            "source",
		Headline:        firstNonEmptyString(item.Title, story.Headline),
		Text:            firstNonEmptyString(item.TextContent, "Source refresh imported evidence for graph review."),
		SourceContentID: item.ContentID,
		ResearchState:   "accepted-for-graph-review",
	})
	if err != nil {
		return types.GlobalWireContribution{}, types.GlobalWireReconciliationDecision{}, types.GlobalWireGraphUpdateCandidate{}, err
	}
	decision, err := h.rt.Store().CreateGlobalWireReconciliationDecision(r.Context(), types.GlobalWireReconciliationDecision{
		ID:              "global-wire-reconciliation-" + uuid.NewString(),
		OwnerID:         ownerID,
		ContributionID:  contribution.ID,
		StoryID:         story.ID,
		Decision:        "accepted",
		Note:            "Source refresh classified this Source Service item as candidate evidence for StoryGraph platform review.",
		SourceContentID: item.ContentID,
	})
	if err != nil {
		return types.GlobalWireContribution{}, types.GlobalWireReconciliationDecision{}, types.GlobalWireGraphUpdateCandidate{}, err
	}
	candidate := h.globalWireGraphUpdateCandidate(ownerID, story, contribution, decision)
	candidate.Rationale = "Source refresh imported Source Service evidence and classified it as a non-mutating StoryGraph update candidate; platform review is required before manifest, edge, prominence, or projection changes."
	saved, err := h.rt.Store().UpsertGlobalWireGraphUpdateCandidate(r.Context(), candidate)
	if err != nil {
		return types.GlobalWireContribution{}, types.GlobalWireReconciliationDecision{}, types.GlobalWireGraphUpdateCandidate{}, err
	}
	return contribution, decision, saved, nil
}

func (h *APIHandler) createGlobalWireSourceRefreshRun(r *http.Request, rec types.GlobalWireSourceRefreshRun) (types.GlobalWireSourceRefreshRun, error) {
	return h.rt.Store().CreateGlobalWireSourceRefreshRun(r.Context(), rec)
}

func (h *APIHandler) createGlobalWireProjectionReviews(r *http.Request, ownerID string, story types.GlobalWireStory, candidate types.GlobalWireGraphUpdateCandidate, promotion types.GlobalWireGraphPromotionDecision) ([]types.GlobalWireProjectionReview, error) {
	styleSources := story.StyleSources
	if len(styleSources) == 0 {
		styleSources = defaultGlobalWireStyleSourcesForRuntime()
	}
	reviews := make([]types.GlobalWireProjectionReview, 0, len(styleSources))
	for _, style := range styleSources {
		styleID := strings.TrimSpace(style.ID)
		if styleID == "" {
			continue
		}
		rec, err := h.rt.Store().CreateGlobalWireProjectionReview(r.Context(), types.GlobalWireProjectionReview{
			ID:               "global-wire-projection-review-" + uuid.NewString(),
			OwnerID:          ownerID,
			StoryID:          story.ID,
			CandidateID:      candidate.ID,
			PromotionID:      promotion.ID,
			SourceContentID:  candidate.SourceContentID,
			StyleID:          styleID,
			StyleDocID:       style.DocID,
			StyleTitle:       firstNonEmptyString(style.Title, style.Label),
			ProjectionAction: candidate.ProjectionAction,
			Status:           "projection-review-required",
			Rationale:        "Promoted StoryGraph evidence may change salience, uncertainty, or ordering for this Style.vtext projection; review before revising the Story VText.",
		})
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, rec)
	}
	return reviews, nil
}

func (h *APIHandler) ensureGlobalWireProjectionReviewDraft(r *http.Request, ownerID string, review types.GlobalWireProjectionReview) (types.Document, types.Revision, types.GlobalWireProjectionReview, error) {
	if strings.TrimSpace(review.DraftStoryDocID) != "" {
		doc, err := h.rt.Store().GetDocument(r.Context(), review.DraftStoryDocID, ownerID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
		}
		rev, err := h.rt.Store().GetRevision(r.Context(), doc.CurrentRevisionID, ownerID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
		}
		return doc, rev, review, nil
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, review.StoryID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(review.SourceContentID) != "" {
		item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, review.SourceContentID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
		}
		sourceItem = &item
	}
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     "Draft projection: " + story.Headline + " - " + firstNonEmptyString(review.StyleTitle, review.StyleID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	citations, err := json.Marshal(globalWireProjectionDraftCitations(review, story, sourceItem))
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"created_from":      "global_wire_projection_review_draft",
		"storygraph_id":     story.ID,
		"projection_review": review.ID,
		"candidate_id":      review.CandidateID,
		"promotion_id":      review.PromotionID,
		"source_content_id": review.SourceContentID,
		"style_id":          review.StyleID,
		"style_doc_id":      review.StyleDocID,
		"draft_state":       "review-draft-not-published",
	})
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	rev := types.Revision{
		RevisionID:  uuid.NewString(),
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "Global Wire",
		Content:     globalWireProjectionDraftVTextContent(review, story, sourceItem),
		Citations:   citations,
		Metadata:    metadata,
		CreatedAt:   now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	sourcePath := "global-wire/projection-drafts/" + review.ID + ".vtext"
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	doc, err = h.rt.Store().GetDocument(r.Context(), doc.DocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	rev, err = h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	updatedReview, err := h.rt.Store().MarkGlobalWireProjectionReviewDraftCreated(r.Context(), ownerID, review.ID, doc.DocID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	return doc, rev, updatedReview, nil
}

func (h *APIHandler) approveGlobalWireProjectionReview(r *http.Request, ownerID string, review types.GlobalWireProjectionReview) (types.Document, types.Revision, types.GlobalWireStoryProjection, types.GlobalWireProjectionReview, error) {
	if strings.TrimSpace(review.ApprovedStoryDocID) != "" && strings.TrimSpace(review.ApprovedRevisionID) != "" {
		doc, err := h.rt.Store().GetDocument(r.Context(), review.ApprovedStoryDocID, ownerID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
		}
		rev, err := h.rt.Store().GetRevision(r.Context(), review.ApprovedRevisionID, ownerID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
		}
		projection, err := h.rt.Store().GetGlobalWireStoryProjection(r.Context(), ownerID, review.StoryID, review.StyleID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
		}
		return doc, rev, projection, review, nil
	}
	if strings.TrimSpace(review.DraftStoryDocID) == "" {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, store.ErrNotFound
	}
	draftDoc, err := h.rt.Store().GetDocument(r.Context(), review.DraftStoryDocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	draftRev, err := h.rt.Store().GetRevision(r.Context(), draftDoc.CurrentRevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	projection, err := h.rt.Store().GetGlobalWireStoryProjection(r.Context(), ownerID, review.StoryID, review.StyleID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	projectionDoc, err := h.rt.Store().GetDocument(r.Context(), projection.StoryDocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	now := time.Now().UTC()
	metadata, err := json.Marshal(map[string]any{
		"created_from":       "global_wire_projection_review_approval",
		"storygraph_id":      review.StoryID,
		"projection_review":  review.ID,
		"draft_story_doc_id": review.DraftStoryDocID,
		"draft_revision_id":  draftRev.RevisionID,
		"candidate_id":       review.CandidateID,
		"promotion_id":       review.PromotionID,
		"source_content_id":  review.SourceContentID,
		"style_id":           review.StyleID,
		"style_doc_id":       review.StyleDocID,
		"approval_state":     "approved_projection_revision",
	})
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            projectionDoc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Global Wire",
		Content:          globalWireApprovedProjectionVTextContent(review, draftRev.Content),
		Citations:        draftRev.Citations,
		Metadata:         metadata,
		CreatedAt:        now,
		ParentRevisionID: projectionDoc.CurrentRevisionID,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	projection.StoryDocID = projectionDoc.DocID
	projection.Text = storedRev.Content
	projection.StyleDocID = firstNonEmptyString(review.StyleDocID, projection.StyleDocID)
	projection.ContextJSON = firstNonEmptyString(projection.ContextJSON, `{"audience":"global-wire","task":"news_projection"}`)
	projection.UpdatedAt = now
	if err := h.rt.Store().UpsertGlobalWireStoryProjection(r.Context(), projection); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	updatedReview, err := h.rt.Store().MarkGlobalWireProjectionReviewApproved(r.Context(), ownerID, review.ID, projectionDoc.DocID, storedRev.RevisionID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	projection, err = h.rt.Store().GetGlobalWireStoryProjection(r.Context(), ownerID, review.StoryID, review.StyleID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), projectionDoc.DocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	return doc, storedRev, projection, updatedReview, nil
}

func globalWireProjectionDraftCitations(review types.GlobalWireProjectionReview, story types.GlobalWireStory, sourceItem *types.ContentItem) []types.Citation {
	citations := []types.Citation{
		{ID: "projection-review", Type: "global_wire_projection_review", Value: review.ID, Label: "Projection review"},
		{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
		{ID: "graph-candidate", Type: "global_wire_graph_candidate", Value: review.CandidateID, Label: "Graph update candidate"},
		{ID: "promotion-decision", Type: "global_wire_graph_promotion", Value: review.PromotionID, Label: "Promotion decision"},
	}
	if strings.TrimSpace(review.StyleDocID) != "" {
		citations = append(citations, types.Citation{ID: "style-source", Type: "vtext", Value: review.StyleDocID, Label: firstNonEmptyString(review.StyleTitle, review.StyleID)})
	} else if strings.TrimSpace(review.StyleID) != "" {
		citations = append(citations, types.Citation{ID: "style-source", Type: "style_vtext", Value: review.StyleID, Label: firstNonEmptyString(review.StyleTitle, review.StyleID)})
	}
	if sourceItem != nil {
		citations = append(citations, types.Citation{ID: "promoted-source", Type: "content_item", Value: sourceItem.ContentID, Label: sourceItem.Title})
	}
	return citations
}

func globalWireApprovedProjectionVTextContent(review types.GlobalWireProjectionReview, draftContent string) string {
	content := strings.TrimSpace(draftContent)
	if content == "" {
		content = "Projection draft content was empty at approval time."
	}
	return strings.Join([]string{
		content,
		"",
		"## Projection Review Approval",
		"",
		"Review status: approved",
		"Projection review id: " + review.ID,
		"Approved state: this normal Story VText revision advances the StoryGraph + Style.vtext projection relation.",
		"Publication guardrail: user-owned forks remain separate and are not mutated by this platform projection review.",
	}, "\n")
}

func globalWireProjectionDraftVTextContent(review types.GlobalWireProjectionReview, story types.GlobalWireStory, sourceItem *types.ContentItem) string {
	projection := strings.TrimSpace(story.Projections[review.StyleID])
	if projection == "" {
		projection = strings.TrimSpace(story.Projections["wire-style"])
	}
	sourceTitle := "No promoted source content item was linked."
	sourceBody := ""
	if sourceItem != nil {
		sourceTitle = sourceItem.Title
		sourceBody = strings.TrimSpace(sourceItem.TextContent)
	}
	sourceContentID := strings.TrimSpace(review.SourceContentID)
	if sourceContentID == "" && sourceItem != nil {
		sourceContentID = sourceItem.ContentID
	}
	lines := []string{
		"# Draft projection: " + story.Headline,
		"",
		"StoryGraph id: " + story.ID,
		"Style.vtext source: " + firstNonEmptyString(review.StyleTitle, review.StyleID),
		"Projection review id: " + review.ID,
		"Graph candidate id: " + review.CandidateID,
		"Promotion decision id: " + review.PromotionID,
		"Draft state: review draft, not platform publication",
		"",
		"## Current Projection Baseline",
		"",
		projection,
		"",
		"## Newly Promoted Evidence",
		"",
		"Promoted source content id: " + firstNonEmptyString(sourceContentID, "none"),
		"",
		"- " + sourceTitle,
	}
	if sourceBody != "" {
		lines = append(lines, "", sourceBody)
	}
	lines = append(lines,
		"",
		"## Review Rationale",
		"",
		review.Rationale,
		"",
		"## Draft Revision Notes",
		"",
		"- Re-check salience, uncertainty, and source ordering for this Style.vtext projection.",
		"- Preserve lead, supporting, contrary, and context tiers.",
		"- Do not invent evidence or hide contrary evidence.",
		"- This draft does not mutate the platform StoryGraph or publish a revised platform story.",
	)
	return strings.Join(lines, "\n")
}

func defaultGlobalWireStyleSourcesForRuntime() []types.GlobalWireStyleSource {
	return []types.GlobalWireStyleSource{
		{ID: "wire-style", Title: "Style.vtext: Global Wire", Label: "Wire", SourcePath: "styles/global-wire.style.vtext"},
		{ID: "claim-audit-style", Title: "Style.vtext: Claim Audit", Label: "Audit", SourcePath: "styles/claim-audit.style.vtext"},
		{ID: "market-brief-style", Title: "Style.vtext: Market Brief", Label: "Market", SourcePath: "styles/market-brief.style.vtext"},
	}
}

func (h *APIHandler) globalWireContributionSourceItems(r *http.Request, ownerID string, contributions []types.GlobalWireContribution) map[string]types.ContentItem {
	items := map[string]types.ContentItem{}
	for _, contribution := range contributions {
		contentID := strings.TrimSpace(contribution.SourceContentID)
		if contentID == "" {
			continue
		}
		if _, ok := items[contentID]; ok {
			continue
		}
		item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, contentID)
		if err != nil {
			continue
		}
		items[contentID] = item
	}
	if len(items) == 0 {
		return nil
	}
	return items
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
