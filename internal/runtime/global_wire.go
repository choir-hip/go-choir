package runtime

import (
	"encoding/json"
	"fmt"
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
	ClaimRecord  *types.GlobalWireClaimRecord            `json:"claim_record,omitempty"`
	ResearchTask *types.GlobalWireResearchTask           `json:"research_task,omitempty"`
}

type globalWireFetchCycleRequest struct {
	StoryIDs   []string `json:"story_ids,omitempty"`
	MaxStories int      `json:"max_stories,omitempty"`
	MaxResults int      `json:"max_results,omitempty"`
	Trigger    string   `json:"trigger,omitempty"`
}

type globalWireFetchCycleResponse struct {
	Status          string                                 `json:"status"`
	Message         string                                 `json:"message"`
	FetchCycle      types.GlobalWireFetchCycleRun          `json:"fetch_cycle"`
	RegistryEntries []types.GlobalWireSourceRegistryEntry  `json:"registry_entries"`
	RefreshRuns     []types.GlobalWireSourceRefreshRun     `json:"refresh_runs"`
	ContentItems    []types.ContentItem                    `json:"content_items,omitempty"`
	Contributions   []types.GlobalWireContribution         `json:"contributions,omitempty"`
	Candidates      []types.GlobalWireGraphUpdateCandidate `json:"candidates,omitempty"`
	ClaimRecords    []types.GlobalWireClaimRecord          `json:"claim_records,omitempty"`
	ResearchTasks   []types.GlobalWireResearchTask         `json:"research_tasks,omitempty"`
	RecentCycles    []types.GlobalWireFetchCycleRun        `json:"recent_cycles,omitempty"`
}

type globalWireReconciliationResponse struct {
	Contributions     []types.GlobalWireContribution           `json:"contributions"`
	SourceItems       map[string]types.ContentItem             `json:"source_items,omitempty"`
	Decisions         []types.GlobalWireReconciliationDecision `json:"decisions"`
	Candidates        []types.GlobalWireGraphUpdateCandidate   `json:"candidates"`
	Promotions        []types.GlobalWireGraphPromotionDecision `json:"promotions"`
	Refreshes         []types.GlobalWireSourceRefreshRun       `json:"refreshes"`
	ClaimRecords      []types.GlobalWireClaimRecord            `json:"claim_records"`
	ResearchTasks     []types.GlobalWireResearchTask           `json:"research_tasks"`
	ResearchEvidence  []types.GlobalWireResearchTaskEvidence   `json:"research_evidence"`
	ProjectionReviews []types.GlobalWireProjectionReview       `json:"projection_reviews"`
}

type globalWireResearchTaskLifecycleRequest struct {
	TaskID          string `json:"task_id"`
	Action          string `json:"action"`
	EvidenceSummary string `json:"evidence_summary,omitempty"`
	ReviewerNote    string `json:"reviewer_note,omitempty"`
	SourceContentID string `json:"source_content_id,omitempty"`
	EvidenceLevel   string `json:"evidence_level,omitempty"`
}

type globalWireResearchTaskLifecycleResponse struct {
	Task     types.GlobalWireResearchTask         `json:"task"`
	Evidence types.GlobalWireResearchTaskEvidence `json:"evidence"`
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

type globalWireStyleSourceRequest struct {
	StoryID        string   `json:"story_id"`
	Action         string   `json:"action"`
	BaseStyleIDs   []string `json:"base_style_ids,omitempty"`
	ReplaceStyleID string   `json:"replace_style_id,omitempty"`
	Title          string   `json:"title,omitempty"`
	Label          string   `json:"label,omitempty"`
	Summary        string   `json:"summary,omitempty"`
}

type globalWireStyleSourceResponse struct {
	Story      types.GlobalWireStory           `json:"story"`
	Style      types.GlobalWireStyleSource     `json:"style"`
	Document   types.Document                  `json:"document"`
	Revision   types.Revision                  `json:"revision"`
	Projection types.GlobalWireStoryProjection `json:"projection"`
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

type globalWireSourceUpdateClassification struct {
	UpdateClassification string
	ContributionKind     string
	CandidateKind        string
	SourceTier           string
	EdgeKind             string
	StoryGraphAction     string
	ProjectionAction     string
	Status               string
	Message              string
	Rationale            string
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
	classification := classifyGlobalWireSourceRefresh(story, item)
	if classification.UpdateClassification == "no-visible-change" {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:                   "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:              ownerID,
			StoryID:              story.ID,
			Query:                firstNonEmptyString(resp.Query, query),
			Status:               classification.Status,
			Provider:             provider,
			Message:              classification.Message,
			UpdateClassification: classification.UpdateClassification,
			StoryGraphAction:     classification.StoryGraphAction,
			ProjectionAction:     classification.ProjectionAction,
			SourceContentID:      item.ContentID,
		})
		if runErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireSourceRefreshResponse{
			Status:      run.Status,
			Source:      run.Provider,
			Query:       run.Query,
			Message:     run.Message,
			RefreshRun:  run,
			ContentItem: &item,
		})
		return
	}
	contribution, decision, candidate, err := h.createGlobalWireSourceRefreshArtifacts(r, ownerID, story, item, classification)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create source refresh artifacts"})
		return
	}
	run, err := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
		ID:                   "global-wire-source-refresh-" + uuid.NewString(),
		OwnerID:              ownerID,
		StoryID:              story.ID,
		Query:                firstNonEmptyString(resp.Query, query),
		Status:               classification.Status,
		Provider:             provider,
		Message:              classification.Message,
		UpdateClassification: classification.UpdateClassification,
		StoryGraphAction:     classification.StoryGraphAction,
		ProjectionAction:     classification.ProjectionAction,
		SourceContentID:      item.ContentID,
		ContributionID:       contribution.ID,
		DecisionID:           decision.ID,
		CandidateID:          candidate.ID,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
		return
	}
	claimRecord, researchTask, err := h.createGlobalWireClaimResearchArtifacts(r, ownerID, story, item, classification, run, contribution, decision, candidate)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create claim research artifacts"})
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
		ClaimRecord:  &claimRecord,
		ResearchTask: &researchTask,
	})
}

// HandleGlobalWireFetchCycles lists or runs a bounded source-registry fetch
// cycle over StoryGraph headline neighborhoods. It records scheduler/fetch
// evidence and reuses the non-mutating source-refresh artifact path.
func (h *APIHandler) HandleGlobalWireFetchCycles(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		registry, err := h.rt.Store().ListGlobalWireSourceRegistryEntries(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list source registry"})
			return
		}
		cycles, err := h.rt.Store().ListGlobalWireFetchCycleRuns(r.Context(), ownerID, 20)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list fetch cycles"})
			return
		}
		status := "empty"
		if len(cycles) > 0 {
			status = cycles[0].Status
		}
		writeAPIJSON(w, http.StatusOK, globalWireFetchCycleResponse{
			Status:          status,
			Message:         "Global Wire source registry and recent bounded fetch cycles.",
			RegistryEntries: registry,
			RecentCycles:    cycles,
		})
	case http.MethodPost:
		var req globalWireFetchCycleRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid fetch cycle request"})
			return
		}
		if req.MaxStories <= 0 {
			req.MaxStories = 3
		}
		if req.MaxStories > 10 {
			req.MaxStories = 10
		}
		if req.MaxResults <= 0 {
			req.MaxResults = 2
		}
		if req.MaxResults > 10 {
			req.MaxResults = 10
		}
		resp, err := h.runGlobalWireFetchCycle(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no StoryGraph stories found for fetch cycle"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to run fetch cycle"})
			return
		}
		status := http.StatusCreated
		if resp.FetchCycle.Status == "unavailable" {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, resp)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
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
		claimRecords, err := h.rt.Store().ListGlobalWireClaimRecords(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list claim records"})
			return
		}
		researchTasks, err := h.rt.Store().ListGlobalWireResearchTasks(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research tasks"})
			return
		}
		researchEvidence, err := h.rt.Store().ListGlobalWireResearchTaskEvidence(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research task evidence"})
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
			ClaimRecords:      claimRecords,
			ResearchTasks:     researchTasks,
			ResearchEvidence:  researchEvidence,
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

// HandleGlobalWireResearchTasks records researcher lifecycle transitions and
// reconciliation-visible evidence packets without mutating platform stories.
func (h *APIHandler) HandleGlobalWireResearchTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireResearchTaskLifecycleRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid research task request"})
		return
	}
	req.TaskID = strings.TrimSpace(req.TaskID)
	action := normalizeGlobalWireResearchTaskAction(req.Action)
	req.EvidenceSummary = strings.TrimSpace(req.EvidenceSummary)
	req.ReviewerNote = strings.TrimSpace(req.ReviewerNote)
	req.SourceContentID = strings.TrimSpace(req.SourceContentID)
	req.EvidenceLevel = normalizeGlobalWireResearchEvidenceLevel(req.EvidenceLevel)
	if req.TaskID == "" || action == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "task_id and action are required"})
		return
	}
	task, err := h.rt.Store().GetGlobalWireResearchTask(r.Context(), ownerID, req.TaskID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "research task not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load research task"})
		return
	}
	status := globalWireResearchTaskStatusForAction(action)
	if req.SourceContentID == "" {
		req.SourceContentID = task.SourceContentID
	}
	if req.EvidenceSummary == "" {
		req.EvidenceSummary = globalWireResearchTaskDefaultSummary(action, task)
	}
	updatedTask, err := h.rt.Store().UpdateGlobalWireResearchTaskStatus(r.Context(), ownerID, task.ID, status)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update research task"})
		return
	}
	evidence, err := h.rt.Store().CreateGlobalWireResearchTaskEvidence(r.Context(), types.GlobalWireResearchTaskEvidence{
		ID:              "global-wire-research-evidence-" + uuid.NewString(),
		OwnerID:         ownerID,
		TaskID:          task.ID,
		StoryID:         task.StoryID,
		ClaimID:         task.ClaimID,
		SourceContentID: req.SourceContentID,
		Status:          status,
		EvidenceLevel:   req.EvidenceLevel,
		Summary:         req.EvidenceSummary,
		ReviewerNote:    req.ReviewerNote,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create research task evidence"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireResearchTaskLifecycleResponse{
		Task:     updatedTask,
		Evidence: evidence,
	})
}

func normalizeGlobalWireResearchTaskAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "assign", "assigned":
		return "assign"
	case "complete", "completed":
		return "complete"
	case "block", "blocked":
		return "block"
	default:
		return ""
	}
}

func globalWireResearchTaskStatusForAction(action string) string {
	switch action {
	case "assign":
		return "assigned"
	case "complete":
		return "completed"
	case "block":
		return "blocked"
	default:
		return "open"
	}
}

func normalizeGlobalWireResearchEvidenceLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "source-level", "claim-level", "reconciliation-level":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "reconciliation-level"
	}
}

func globalWireResearchTaskDefaultSummary(action string, task types.GlobalWireResearchTask) string {
	switch action {
	case "assign":
		return "Research task assigned for owner-scoped review; no platform StoryGraph mutation was applied."
	case "complete":
		return "Research task completed with reconciliation evidence; platform StoryGraph stories remain unchanged pending explicit review."
	case "block":
		return "Research task blocked; reconciliation should treat the claim as unresolved until more source evidence is available."
	default:
		return strings.TrimSpace(task.Prompt)
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

// HandleGlobalWireStyleSources creates composed or replacement Style.vtext
// artifacts and attaches them to a StoryGraph projection relation.
func (h *APIHandler) HandleGlobalWireStyleSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireStyleSourceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid style source request"})
		return
	}
	req.StoryID = strings.TrimSpace(req.StoryID)
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))
	req.ReplaceStyleID = strings.TrimSpace(req.ReplaceStyleID)
	req.Title = strings.TrimSpace(req.Title)
	req.Label = strings.TrimSpace(req.Label)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.StoryID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "story_id is required"})
		return
	}
	if req.Action != "compose" && req.Action != "replace" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "action must be compose or replace"})
		return
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
	doc, rev, style, projection, story, err := h.createGlobalWireComposedStyleSource(r, ownerID, story, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "base or replacement style not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create Style.vtext source"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireStyleSourceResponse{
		Story:      story,
		Style:      style,
		Document:   doc,
		Revision:   rev,
		Projection: projection,
	})
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
	appliedChanges := []string{}
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
	if added {
		appliedChanges = append(appliedChanges, "appended source_content_id "+item.ContentID+" to "+tier+" manifest tier")
	} else {
		appliedChanges = append(appliedChanges, "source already present in "+tier+" manifest tier; promotion recorded without duplicate source")
	}
	relatedStoryID, relatedStoryAdded := h.applyGlobalWireClassifiedStoryUpdate(r, ownerID, &story, candidate, item)
	if relatedStoryAdded {
		appliedChanges = append(appliedChanges, "added related Story VText edge to "+relatedStoryID)
	}
	story.SourceState = "platform-review-promoted-source"
	if strings.TrimSpace(story.Freshness) == "" || strings.Contains(strings.ToLower(story.Freshness), "updated") {
		story.Freshness = "updated just now"
	}
	story.UpdatedAt = time.Now().UTC()
	if err := h.rt.Store().UpsertGlobalWireStory(r.Context(), story); err != nil {
		return types.GlobalWireStory{}, "", err
	}
	revisionID, err := h.createGlobalWirePlatformStoryRevision(r, ownerID, story, candidate, item, appliedChanges)
	if err != nil {
		return types.GlobalWireStory{}, "", err
	}
	if revisionID != "" {
		appliedChanges = append(appliedChanges, "created PlatformStory VText revision "+revisionID)
	}
	return story, strings.Join(appliedChanges, "; "), nil
}

func (h *APIHandler) applyGlobalWireClassifiedStoryUpdate(r *http.Request, ownerID string, story *types.GlobalWireStory, candidate types.GlobalWireGraphUpdateCandidate, item types.ContentItem) (string, bool) {
	kind := strings.TrimSpace(candidate.CandidateKind)
	summary := firstNonEmptyString(candidate.Summary, item.TextContent, item.Title)
	relatedStoryID := ""
	relatedStoryAdded := false
	switch kind {
	case "claim-changed":
		story.Claims = appendStringIfMissing(story.Claims, "Reviewed update: "+summary)
		story.ChangeState = "claim changed"
		story.Tension = "claim update"
		story.NodeTone = "changed"
		story.Prominence = clampGlobalWireProminence(story.Prominence + 4)
	case "contradiction-added":
		story.Claims = appendStringIfMissing(story.Claims, "Contrary evidence: "+summary)
		story.ChangeState = "contradiction added"
		story.Tension = "contradiction added"
		story.NodeTone = "hot"
		story.Prominence = clampGlobalWireProminence(story.Prominence + 6)
	case "front-page-prominence-changed":
		story.Claims = appendStringIfMissing(story.Claims, "Prominence review: "+summary)
		story.ChangeState = "front-page prominence changed"
		story.Tension = "prominence changed"
		story.NodeTone = "live"
		story.Prominence = clampGlobalWireProminence(story.Prominence + 12)
	case "related-story-edge-added":
		relatedStoryID = h.findGlobalWireRelatedStoryID(r, ownerID, story.ID, summary+" "+item.Title)
		if relatedStoryID != "" {
			story.Related, relatedStoryAdded = appendStringIfMissingWithStatus(story.Related, relatedStoryID)
		}
		story.ChangeState = "related story edge added"
		story.Tension = "source neighborhood expanded"
		story.NodeTone = "changed"
		story.Prominence = clampGlobalWireProminence(story.Prominence + 2)
	case "source-manifest-update":
		story.ChangeState = "source manifest updated"
		story.Tension = firstNonEmptyString(story.Tension, "new supporting evidence")
		story.NodeTone = firstNonEmptyString(story.NodeTone, "changed")
		story.Prominence = clampGlobalWireProminence(story.Prominence + 2)
	default:
		story.ChangeState = firstNonEmptyString(story.ChangeState, "source manifest updated")
	}
	return relatedStoryID, relatedStoryAdded
}

func (h *APIHandler) findGlobalWireRelatedStoryID(r *http.Request, ownerID, currentStoryID, evidenceText string) string {
	stories, err := h.rt.Store().ListGlobalWireStories(r.Context(), ownerID)
	if err != nil {
		return ""
	}
	evidenceText = strings.ToLower(evidenceText)
	for _, candidate := range stories {
		if candidate.ID == currentStoryID {
			continue
		}
		if strings.Contains(evidenceText, strings.ToLower(candidate.ID)) || strings.Contains(evidenceText, strings.ToLower(candidate.Headline)) {
			return candidate.ID
		}
		for _, token := range strings.Fields(strings.ToLower(candidate.Headline)) {
			token = strings.Trim(token, ".,:;!?()[]{}\"'")
			if len(token) >= 6 && strings.Contains(evidenceText, token) {
				return candidate.ID
			}
		}
	}
	return ""
}

func (h *APIHandler) createGlobalWirePlatformStoryRevision(r *http.Request, ownerID string, story types.GlobalWireStory, candidate types.GlobalWireGraphUpdateCandidate, item types.ContentItem, appliedChanges []string) (string, error) {
	if strings.TrimSpace(story.StoryVTextDoc) == "" {
		return "", nil
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), story.StoryVTextDoc, ownerID)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	citations, err := json.Marshal(globalWirePlatformStoryRevisionCitations(story, candidate, item))
	if err != nil {
		return "", err
	}
	metadata, err := json.Marshal(map[string]any{
		"created_from":      "global_wire_graph_candidate_promotion",
		"storygraph_id":     story.ID,
		"candidate_id":      candidate.ID,
		"source_content_id": item.ContentID,
		"candidate_kind":    candidate.CandidateKind,
		"edge_kind":         candidate.EdgeKind,
		"source_tier":       candidate.SourceTier,
		"applied_changes":   appliedChanges,
		"mutation_boundary": "platform-review-only",
	})
	if err != nil {
		return "", err
	}
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            doc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Global Wire",
		Content:          globalWirePlatformStoryRevisionContent(story, candidate, item, appliedChanges),
		Citations:        citations,
		Metadata:         metadata,
		CreatedAt:        now,
		ParentRevisionID: doc.CurrentRevisionID,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return "", err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return "", err
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	return storedRev.RevisionID, nil
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

func appendStringIfMissing(values []string, value string) []string {
	out, _ := appendStringIfMissingWithStatus(values, value)
	return out
}

func appendStringIfMissingWithStatus(values []string, value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return values, false
	}
	for _, existing := range values {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return values, false
		}
	}
	return append(values, value), true
}

func clampGlobalWireProminence(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func globalWirePlatformStoryRevisionCitations(story types.GlobalWireStory, candidate types.GlobalWireGraphUpdateCandidate, item types.ContentItem) []types.Citation {
	citations := []types.Citation{
		{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
		{ID: "graph-candidate", Type: "global_wire_graph_candidate", Value: candidate.ID, Label: firstNonEmptyString(candidate.CandidateKind, "graph candidate")},
		{ID: "promoted-source", Type: "content_item", Value: item.ContentID, Label: firstNonEmptyString(item.Title, candidate.Title, "Promoted source")},
	}
	for _, style := range story.StyleSources {
		if strings.TrimSpace(style.DocID) != "" {
			citations = append(citations, types.Citation{
				ID:    "style-" + style.ID,
				Type:  "vtext",
				Value: style.DocID,
				Label: firstNonEmptyString(style.Title, style.Label),
			})
		}
	}
	return citations
}

func globalWirePlatformStoryRevisionContent(story types.GlobalWireStory, candidate types.GlobalWireGraphUpdateCandidate, item types.ContentItem, appliedChanges []string) string {
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		"StoryGraph id: " + story.ID,
		"Platform review state: " + story.ChangeState,
		"Tension: " + story.Tension,
		"Prominence: " + fmt.Sprintf("%d", story.Prominence),
		"Candidate kind: " + firstNonEmptyString(candidate.CandidateKind, "source-manifest-update"),
		"Graph candidate id: " + candidate.ID,
		"Promoted source content id: " + item.ContentID,
		"",
		"## Platform Review Update",
		"",
	}
	for _, change := range appliedChanges {
		lines = append(lines, "- "+change)
	}
	lines = append(lines, "", "## Claims", "")
	for _, claim := range story.Claims {
		lines = append(lines, "- "+claim)
	}
	lines = append(lines, "", "## Source Manifest", "")
	lines = append(lines, globalWireRuntimeSourceLines("lead", story.Manifest.Lead)...)
	lines = append(lines, globalWireRuntimeSourceLines("supporting", story.Manifest.Supporting)...)
	lines = append(lines, globalWireRuntimeSourceLines("contrary or qualifying", story.Manifest.Contrary)...)
	lines = append(lines, globalWireRuntimeSourceLines("ambient context", story.Manifest.Context)...)
	lines = append(lines, "", "## Related Story VTexts", "")
	for _, related := range story.Related {
		lines = append(lines, "- "+related)
	}
	lines = append(lines,
		"",
		"## Ownership Boundary",
		"",
		"This PlatformStory VText revision was created by explicit platform review. User-owned forks, edits, and contributions remain separate and are not mutated by this revision.",
	)
	return strings.Join(lines, "\n")
}

func globalWireRuntimeSourceLines(label string, items []types.GlobalWireSourceItem) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s; %s)", label, item.Title, item.Standing, firstNonEmptyString(item.ContentID, item.ID)))
	}
	return lines
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

func (h *APIHandler) createGlobalWireSourceRefreshArtifacts(r *http.Request, ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification) (types.GlobalWireContribution, types.GlobalWireReconciliationDecision, types.GlobalWireGraphUpdateCandidate, error) {
	contribution, err := h.rt.Store().CreateGlobalWireContribution(r.Context(), types.GlobalWireContribution{
		ID:              "global-wire-contribution-" + uuid.NewString(),
		OwnerID:         ownerID,
		StoryID:         story.ID,
		Kind:            firstNonEmptyString(classification.ContributionKind, "source"),
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
		Note:            "Source refresh classified this Source Service item as " + firstNonEmptyString(classification.UpdateClassification, "source-manifest-update") + " for StoryGraph platform review.",
		SourceContentID: item.ContentID,
	})
	if err != nil {
		return types.GlobalWireContribution{}, types.GlobalWireReconciliationDecision{}, types.GlobalWireGraphUpdateCandidate{}, err
	}
	candidate := h.globalWireGraphUpdateCandidate(ownerID, story, contribution, decision)
	candidate.CandidateKind = firstNonEmptyString(classification.CandidateKind, candidate.CandidateKind)
	candidate.SourceTier = firstNonEmptyString(classification.SourceTier, candidate.SourceTier)
	candidate.EdgeKind = firstNonEmptyString(classification.EdgeKind, candidate.EdgeKind)
	candidate.ProjectionAction = firstNonEmptyString(classification.ProjectionAction, candidate.ProjectionAction)
	candidate.Rationale = firstNonEmptyString(classification.Rationale, "Source refresh imported Source Service evidence and classified it as a non-mutating StoryGraph update candidate; platform review is required before manifest, edge, prominence, or projection changes.")
	saved, err := h.rt.Store().UpsertGlobalWireGraphUpdateCandidate(r.Context(), candidate)
	if err != nil {
		return types.GlobalWireContribution{}, types.GlobalWireReconciliationDecision{}, types.GlobalWireGraphUpdateCandidate{}, err
	}
	return contribution, decision, saved, nil
}

func (h *APIHandler) runGlobalWireFetchCycle(r *http.Request, ownerID string, req globalWireFetchCycleRequest) (globalWireFetchCycleResponse, error) {
	stories, err := h.rt.Store().ListGlobalWireStories(r.Context(), ownerID)
	if err != nil {
		return globalWireFetchCycleResponse{}, err
	}
	stories = selectGlobalWireFetchCycleStories(stories, req.StoryIDs, req.MaxStories)
	if len(stories) == 0 {
		return globalWireFetchCycleResponse{}, store.ErrNotFound
	}
	cycleID := "global-wire-fetch-cycle-" + uuid.NewString()
	registry := make([]types.GlobalWireSourceRegistryEntry, 0, len(stories))
	refreshes := []types.GlobalWireSourceRefreshRun{}
	contentItems := []types.ContentItem{}
	contributions := []types.GlobalWireContribution{}
	candidates := []types.GlobalWireGraphUpdateCandidate{}
	claimRecords := []types.GlobalWireClaimRecord{}
	researchTasks := []types.GlobalWireResearchTask{}
	storyIDs := []string{}
	registryIDs := []string{}
	refreshIDs := []string{}
	sourceIDs := []string{}
	cycleStatus := "completed"
	messages := []string{}
	sourceClient := newSourceSearchClientFromEnv()

	for _, story := range stories {
		storyIDs = append(storyIDs, story.ID)
		entry := types.GlobalWireSourceRegistryEntry{
			ID:          globalWireSourceRegistryEntryID(ownerID, story.ID),
			OwnerID:     ownerID,
			StoryID:     story.ID,
			Query:       story.Headline,
			SourceScope: "story-neighborhood",
			Status:      "active",
			LastCycleID: cycleID,
		}
		entry, err = h.rt.Store().UpsertGlobalWireSourceRegistryEntry(r.Context(), entry)
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		registry = append(registry, entry)
		registryIDs = append(registryIDs, entry.ID)

		if sourceClient == nil {
			run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
				ID:       "global-wire-source-refresh-" + uuid.NewString(),
				OwnerID:  ownerID,
				StoryID:  story.ID,
				Query:    entry.Query,
				Status:   "unavailable",
				Provider: "source-service",
				Message:  "Source Service is not configured for this runtime; fetch cycle recorded scheduler evidence only.",
			})
			if runErr != nil {
				return globalWireFetchCycleResponse{}, runErr
			}
			refreshes = append(refreshes, run)
			refreshIDs = append(refreshIDs, run.ID)
			cycleStatus = "unavailable"
			messages = append(messages, story.ID+": unavailable")
			continue
		}

		searchResp, searchErr := sourceClient.SearchSources(r.Context(), entry.Query, req.MaxResults)
		provider := sourceapi.ProviderName
		if searchResp.Provider != "" {
			provider = searchResp.Provider
		}
		if searchErr != nil {
			run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
				ID:       "global-wire-source-refresh-" + uuid.NewString(),
				OwnerID:  ownerID,
				StoryID:  story.ID,
				Query:    entry.Query,
				Status:   "unavailable",
				Provider: "source-service",
				Message:  searchErr.Error(),
			})
			if runErr != nil {
				return globalWireFetchCycleResponse{}, runErr
			}
			refreshes = append(refreshes, run)
			refreshIDs = append(refreshIDs, run.ID)
			if cycleStatus == "completed" {
				cycleStatus = "completed-with-gaps"
			}
			messages = append(messages, story.ID+": unavailable")
			continue
		}
		query := firstNonEmptyString(searchResp.Query, entry.Query)
		if len(searchResp.Results) == 0 {
			run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
				ID:       "global-wire-source-refresh-" + uuid.NewString(),
				OwnerID:  ownerID,
				StoryID:  story.ID,
				Query:    query,
				Status:   "no-evidence",
				Provider: provider,
				Message:  "Fetch cycle searched the source registry query but Source Service returned no matching evidence.",
			})
			if runErr != nil {
				return globalWireFetchCycleResponse{}, runErr
			}
			refreshes = append(refreshes, run)
			refreshIDs = append(refreshIDs, run.ID)
			if cycleStatus == "completed" {
				cycleStatus = "completed-with-gaps"
			}
			messages = append(messages, story.ID+": no-evidence")
			continue
		}
		item, err := h.ensureGlobalWireSourceServiceContentItem(r, ownerID, searchResp.Results[0])
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		contentItems = append(contentItems, item)
		sourceIDs = appendStringIfMissing(sourceIDs, item.ContentID)
		classification := classifyGlobalWireSourceRefresh(story, item)
		if classification.UpdateClassification == "no-visible-change" {
			run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
				ID:                   "global-wire-source-refresh-" + uuid.NewString(),
				OwnerID:              ownerID,
				StoryID:              story.ID,
				Query:                query,
				Status:               classification.Status,
				Provider:             provider,
				Message:              classification.Message,
				UpdateClassification: classification.UpdateClassification,
				StoryGraphAction:     classification.StoryGraphAction,
				ProjectionAction:     classification.ProjectionAction,
				SourceContentID:      item.ContentID,
			})
			if runErr != nil {
				return globalWireFetchCycleResponse{}, runErr
			}
			refreshes = append(refreshes, run)
			refreshIDs = append(refreshIDs, run.ID)
			messages = append(messages, story.ID+": no-visible-change")
			continue
		}
		contribution, decision, candidate, err := h.createGlobalWireSourceRefreshArtifacts(r, ownerID, story, item, classification)
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		run, err := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:                   "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:              ownerID,
			StoryID:              story.ID,
			Query:                query,
			Status:               classification.Status,
			Provider:             provider,
			Message:              classification.Message,
			UpdateClassification: classification.UpdateClassification,
			StoryGraphAction:     classification.StoryGraphAction,
			ProjectionAction:     classification.ProjectionAction,
			SourceContentID:      item.ContentID,
			ContributionID:       contribution.ID,
			DecisionID:           decision.ID,
			CandidateID:          candidate.ID,
		})
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		claim, task, err := h.createGlobalWireClaimResearchArtifacts(r, ownerID, story, item, classification, run, contribution, decision, candidate)
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		refreshes = append(refreshes, run)
		refreshIDs = append(refreshIDs, run.ID)
		contributions = append(contributions, contribution)
		candidates = append(candidates, candidate)
		claimRecords = append(claimRecords, claim)
		researchTasks = append(researchTasks, task)
		messages = append(messages, story.ID+": "+classification.UpdateClassification)
	}

	cycle := types.GlobalWireFetchCycleRun{
		ID:               cycleID,
		OwnerID:          ownerID,
		Trigger:          firstNonEmptyString(strings.TrimSpace(req.Trigger), "manual-bounded-cycle"),
		Status:           cycleStatus,
		StoryIDs:         storyIDs,
		RegistryEntryIDs: registryIDs,
		RefreshRunIDs:    refreshIDs,
		SourceContentIDs: sourceIDs,
		Message:          strings.Join(messages, "; "),
	}
	cycle, err = h.rt.Store().CreateGlobalWireFetchCycleRun(r.Context(), cycle)
	if err != nil {
		return globalWireFetchCycleResponse{}, err
	}
	return globalWireFetchCycleResponse{
		Status:          cycle.Status,
		Message:         cycle.Message,
		FetchCycle:      cycle,
		RegistryEntries: registry,
		RefreshRuns:     refreshes,
		ContentItems:    contentItems,
		Contributions:   contributions,
		Candidates:      candidates,
		ClaimRecords:    claimRecords,
		ResearchTasks:   researchTasks,
	}, nil
}

func selectGlobalWireFetchCycleStories(stories []types.GlobalWireStory, ids []string, limit int) []types.GlobalWireStory {
	selected := []types.GlobalWireStory{}
	wanted := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = true
		}
	}
	for _, story := range stories {
		if len(wanted) > 0 && !wanted[story.ID] {
			continue
		}
		selected = append(selected, story)
		if limit > 0 && len(selected) >= limit {
			break
		}
	}
	return selected
}

func globalWireSourceRegistryEntryID(ownerID, storyID string) string {
	return "global-wire-source-registry-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+storyID)).String()
}

func (h *APIHandler) createGlobalWireClaimResearchArtifacts(r *http.Request, ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, contribution types.GlobalWireContribution, decision types.GlobalWireReconciliationDecision, candidate types.GlobalWireGraphUpdateCandidate) (types.GlobalWireClaimRecord, types.GlobalWireResearchTask, error) {
	claim := globalWireClaimRecordFromRefresh(ownerID, story, item, classification, run, contribution, decision, candidate)
	savedClaim, err := h.rt.Store().CreateGlobalWireClaimRecord(r.Context(), claim)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireResearchTask{}, err
	}
	task := globalWireResearchTaskFromClaim(ownerID, story, item, classification, run, contribution, candidate, savedClaim)
	savedTask, err := h.rt.Store().CreateGlobalWireResearchTask(r.Context(), task)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireResearchTask{}, err
	}
	return savedClaim, savedTask, nil
}

func globalWireClaimRecordFromRefresh(ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, contribution types.GlobalWireContribution, decision types.GlobalWireReconciliationDecision, candidate types.GlobalWireGraphUpdateCandidate) types.GlobalWireClaimRecord {
	claimKind := "evidence-update"
	uncertainty := "requires-review"
	dispute := "not-yet-assessed"
	evidenceGap := "Verify whether this SourceItem changes the StoryGraph claim set before changing any platform story."
	switch classification.UpdateClassification {
	case "claim-changed":
		claimKind = "claim-change"
		uncertainty = "material-change-unverified"
		dispute = "needs-comparison"
		evidenceGap = "Compare the imported evidence against existing lead/supporting/contrary source tiers and decide whether the claim should narrow, broaden, or stay unchanged."
	case "contradiction-added":
		claimKind = "contradiction"
		uncertainty = "contrary-evidence-unreviewed"
		dispute = "disputed"
		evidenceGap = "Check whether this source materially contradicts, qualifies, or only reframes the current StoryGraph claim set."
	case "front-page-prominence-changed":
		claimKind = "prominence-signal"
		uncertainty = "editorial-weight-unverified"
		dispute = "not-yet-assessed"
		evidenceGap = "Verify whether the source changes prominence or urgency without overstating the underlying facts."
	case "related-story-edge-added":
		claimKind = "related-edge"
		uncertainty = "relationship-unverified"
		dispute = "not-yet-assessed"
		evidenceGap = "Verify whether the shared source basis is enough to connect this story to a related headline neighborhood."
	case "source-manifest-update":
		claimKind = "source-support"
		uncertainty = "source-standing-unreviewed"
		dispute = "not-yet-assessed"
		evidenceGap = "Check source standing, freshness, and whether this belongs in lead, supporting, contrary, or context evidence."
	}
	return types.GlobalWireClaimRecord{
		ID:                   "global-wire-claim-" + uuid.NewString(),
		OwnerID:              ownerID,
		StoryID:              story.ID,
		RefreshID:            run.ID,
		SourceContentID:      item.ContentID,
		ContributionID:       contribution.ID,
		DecisionID:           decision.ID,
		CandidateID:          candidate.ID,
		ClaimText:            globalWireClaimTextFromSource(story, item, classification),
		ClaimKind:            claimKind,
		UncertaintyState:     uncertainty,
		DisputeState:         dispute,
		EvidenceGap:          evidenceGap,
		SourceStanding:       globalWireSourceStandingLabel(item),
		UpdateClassification: classification.UpdateClassification,
		Status:               "research-review-required",
	}
}

func globalWireResearchTaskFromClaim(ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, contribution types.GlobalWireContribution, candidate types.GlobalWireGraphUpdateCandidate, claim types.GlobalWireClaimRecord) types.GlobalWireResearchTask {
	taskKind := "source-standing-review"
	priority := "normal"
	switch classification.UpdateClassification {
	case "claim-changed":
		taskKind = "claim-change-review"
		priority = "high"
	case "contradiction-added":
		taskKind = "dispute-review"
		priority = "high"
	case "front-page-prominence-changed":
		taskKind = "prominence-review"
		priority = "high"
	case "related-story-edge-added":
		taskKind = "related-edge-review"
	case "source-manifest-update":
		taskKind = "source-tier-review"
	}
	return types.GlobalWireResearchTask{
		ID:                   "global-wire-research-task-" + uuid.NewString(),
		OwnerID:              ownerID,
		StoryID:              story.ID,
		ClaimID:              claim.ID,
		RefreshID:            run.ID,
		SourceContentID:      item.ContentID,
		ContributionID:       contribution.ID,
		CandidateID:          candidate.ID,
		TaskKind:             taskKind,
		Prompt:               globalWireResearchTaskPrompt(story, item, classification, claim),
		Status:               "open",
		Priority:             priority,
		UpdateClassification: classification.UpdateClassification,
	}
}

func globalWireClaimTextFromSource(story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification) string {
	sourceTitle := firstNonEmptyString(item.Title, item.CanonicalURL, item.SourceURL, "Imported SourceItem")
	switch classification.UpdateClassification {
	case "claim-changed":
		return "Provisional claim-change signal for \"" + story.Headline + "\" from " + sourceTitle + "."
	case "contradiction-added":
		return "Provisional contradiction or qualification signal for \"" + story.Headline + "\" from " + sourceTitle + "."
	case "front-page-prominence-changed":
		return "Provisional prominence-change signal for \"" + story.Headline + "\" from " + sourceTitle + "."
	case "related-story-edge-added":
		return "Provisional related-story edge signal for \"" + story.Headline + "\" from " + sourceTitle + "."
	default:
		return "Provisional source-support signal for \"" + story.Headline + "\" from " + sourceTitle + "."
	}
}

func globalWireResearchTaskPrompt(story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, claim types.GlobalWireClaimRecord) string {
	return strings.Join([]string{
		"Review this Global Wire refresh before any platform StoryGraph mutation.",
		"StoryGraph headline: " + story.Headline,
		"Classification: " + firstNonEmptyString(classification.UpdateClassification, "source-manifest-update"),
		"Claim state: " + claim.UncertaintyState + " / " + claim.DisputeState,
		"Source: " + firstNonEmptyString(item.Title, item.CanonicalURL, item.SourceURL, item.ContentID),
		"Evidence gap: " + claim.EvidenceGap,
		"Do not treat the source as an oracle; decide source standing, claim impact, contrary evidence, and projection-review needs from cited evidence.",
	}, " ")
}

func globalWireSourceStandingLabel(item types.ContentItem) string {
	if strings.TrimSpace(item.SourceType) != "" {
		return item.SourceType
	}
	if strings.TrimSpace(item.AppHint) != "" {
		return item.AppHint
	}
	return "unreviewed-source"
}

func classifyGlobalWireSourceRefresh(story types.GlobalWireStory, item types.ContentItem) globalWireSourceUpdateClassification {
	text := strings.ToLower(strings.Join([]string{
		item.Title,
		item.TextContent,
		item.CanonicalURL,
		item.SourceURL,
	}, " "))
	base := globalWireSourceUpdateClassification{
		UpdateClassification: "source-manifest-update",
		ContributionKind:     "source",
		CandidateKind:        "source-manifest-update",
		SourceTier:           "supporting",
		EdgeKind:             "shared-source-neighborhood",
		StoryGraphAction:     "source-manifest-update",
		ProjectionAction:     "projection-review-required",
		Status:               "candidate-review",
		Message:              "Source refresh classified live evidence as a source-manifest update and created a non-mutating graph-update candidate for platform review.",
		Rationale:            "Source refresh imported Source Service evidence and classified it as a source-manifest update; platform review is required before manifest, edge, prominence, or projection changes.",
	}
	if globalWireSourceAlreadyInManifest(story, item) || containsAny(text, "no visible change", "unchanged", "already reflected", "duplicate") {
		return globalWireSourceUpdateClassification{
			UpdateClassification: "no-visible-change",
			ContributionKind:     "source",
			CandidateKind:        "no-visible-change",
			SourceTier:           "context",
			EdgeKind:             "already-known-source",
			StoryGraphAction:     "no-storygraph-change",
			ProjectionAction:     "no-projection-change-yet",
			Status:               "no-visible-change",
			Message:              "Source refresh imported evidence but classified it as no visible StoryGraph change; no graph candidate was created.",
			Rationale:            "Source refresh classified this evidence as already reflected in the current StoryGraph source neighborhood.",
		}
	}
	if containsAny(text, "contradict", "contrary", "counter", "dispute", "denies", "denied", "false", "warning", "warns", "caution") {
		return globalWireSourceUpdateClassification{
			UpdateClassification: "contradiction-added",
			ContributionKind:     "counter-source",
			CandidateKind:        "contradiction-added",
			SourceTier:           "contrary",
			EdgeKind:             "contradiction-or-qualification",
			StoryGraphAction:     "contrary-source-review",
			ProjectionAction:     "projection-review-required",
			Status:               "candidate-review",
			Message:              "Source refresh classified live evidence as a contradiction or qualification and queued a contrary-source graph candidate.",
			Rationale:            "Source refresh found evidence that may qualify or contradict the current StoryGraph claim set; platform review must decide whether to attach it as contrary evidence.",
		}
	}
	if containsAny(text, "breaking", "urgent", "major", "front page", "front-page", "prominence", "surge", "plunge", "emergency") {
		return globalWireSourceUpdateClassification{
			UpdateClassification: "front-page-prominence-changed",
			ContributionKind:     "source",
			CandidateKind:        "front-page-prominence-changed",
			SourceTier:           "lead",
			EdgeKind:             "prominence-change",
			StoryGraphAction:     "prominence-review",
			ProjectionAction:     "projection-review-required",
			Status:               "candidate-review",
			Message:              "Source refresh classified live evidence as a possible front-page prominence change and queued a lead-source graph candidate.",
			Rationale:            "Source refresh found evidence that may change prominence or editorial weight; platform review must decide before the News front page changes.",
		}
	}
	if containsAny(text, "related", "linked", "spillover", "neighbor", "adjacent", "same source") {
		return globalWireSourceUpdateClassification{
			UpdateClassification: "related-story-edge-added",
			ContributionKind:     "argument",
			CandidateKind:        "related-story-edge-added",
			SourceTier:           "context",
			EdgeKind:             "related-story-edge",
			StoryGraphAction:     "related-edge-review",
			ProjectionAction:     "no-projection-change-yet",
			Status:               "candidate-review",
			Message:              "Source refresh classified live evidence as a possible related-story edge and queued a context graph candidate.",
			Rationale:            "Source refresh found evidence that may connect this StoryGraph node to another source neighborhood; platform review must decide before graph topology changes.",
		}
	}
	if containsAny(text, "claim changed", "revised", "updated", "correction", "corrected", "fell", "rose", "reduced", "increased", "improved", "worsened", "shifted") {
		base.UpdateClassification = "claim-changed"
		base.CandidateKind = "claim-changed"
		base.EdgeKind = "update-relation"
		base.StoryGraphAction = "claim-review"
		base.Message = "Source refresh classified live evidence as a claim change and queued a graph candidate for platform review."
		base.Rationale = "Source refresh found evidence that may alter the current claim set or timeline; platform review is required before StoryGraph or projection revisions."
	}
	return base
}

func globalWireSourceAlreadyInManifest(story types.GlobalWireStory, item types.ContentItem) bool {
	candidates := []string{strings.TrimSpace(item.ContentID), strings.TrimSpace(item.CanonicalURL), strings.TrimSpace(item.SourceURL), strings.TrimSpace(item.Title)}
	for _, source := range append(append(append(story.Manifest.Lead, story.Manifest.Supporting...), story.Manifest.Contrary...), story.Manifest.Context...) {
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if strings.EqualFold(source.ContentID, candidate) || strings.EqualFold(source.CanonicalURL, candidate) || strings.EqualFold(source.Title, candidate) {
				return true
			}
		}
	}
	return false
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
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

func (h *APIHandler) createGlobalWireComposedStyleSource(r *http.Request, ownerID string, story types.GlobalWireStory, req globalWireStyleSourceRequest) (types.Document, types.Revision, types.GlobalWireStyleSource, types.GlobalWireStoryProjection, types.GlobalWireStory, error) {
	baseStyles := selectGlobalWireBaseStyles(story.StyleSources, req.BaseStyleIDs)
	if len(baseStyles) == 0 {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, store.ErrNotFound
	}
	if req.Action == "replace" && findGlobalWireStyleSource(story.StyleSources, req.ReplaceStyleID).ID == "" {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	styleID := "composed-style-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+story.ID+":"+req.Action+":"+strings.Join(req.BaseStyleIDs, ",")+":"+req.Title+":"+req.ReplaceStyleID+":"+now.Format(time.RFC3339Nano))).String()
	if req.Action == "replace" {
		styleID = "replacement-style-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+story.ID+":"+req.ReplaceStyleID+":"+req.Title+":"+now.Format(time.RFC3339Nano))).String()
	}
	title := firstNonEmptyString(req.Title, defaultGlobalWireComposedStyleTitle(req.Action, baseStyles))
	label := firstNonEmptyString(req.Label, defaultGlobalWireComposedStyleLabel(req.Action, baseStyles))
	summary := firstNonEmptyString(req.Summary, defaultGlobalWireComposedStyleSummary(req.Action, baseStyles))
	sourcePath := "global-wire/styles/" + styleID + ".style.vtext"

	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	citations, err := json.Marshal(globalWireComposedStyleCitations(story, baseStyles))
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"created_from":     "global_wire_style_source_" + req.Action,
		"storygraph_id":    story.ID,
		"style_id":         styleID,
		"base_style_ids":   globalWireStyleIDs(baseStyles),
		"replace_style_id": req.ReplaceStyleID,
		"source_path":      sourcePath,
	})
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	rev := types.Revision{
		RevisionID:  uuid.NewString(),
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "Global Wire",
		Content:     globalWireComposedStyleVTextContent(req.Action, title, summary, story, baseStyles),
		Citations:   citations,
		Metadata:    metadata,
		CreatedAt:   now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	style := types.GlobalWireStyleSource{
		ID:         styleID,
		Title:      title,
		Label:      label,
		Summary:    summary,
		SourcePath: sourcePath,
		DocID:      doc.DocID,
	}
	story.StyleSources = applyGlobalWireStyleSourceTransition(story.StyleSources, style, req)
	projectionText := globalWireComposedStyleProjectionText(story, style, baseStyles)
	if story.Projections == nil {
		story.Projections = map[string]string{}
	}
	story.Projections[style.ID] = projectionText
	story.SourceState = "style-source-" + req.Action
	story.UpdatedAt = now
	if err := h.rt.Store().UpsertGlobalWireStory(r.Context(), story); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	projectionDoc, projectionRev, err := h.createGlobalWireComposedProjectionVText(r, ownerID, story, style, baseStyles, projectionText, now)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	projection := types.GlobalWireStoryProjection{
		ID:          "global-wire-projection-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+story.ID+":"+style.ID)).String(),
		OwnerID:     ownerID,
		StoryID:     story.ID,
		StyleID:     style.ID,
		StyleDocID:  style.DocID,
		StoryDocID:  projectionDoc.DocID,
		ContextJSON: `{"audience":"global-wire","task":"news_projection","style_transition":"` + req.Action + `"}`,
		Text:        projectionRev.Content,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.rt.Store().UpsertGlobalWireStoryProjection(r.Context(), projection); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStyleSource{}, types.GlobalWireStoryProjection{}, types.GlobalWireStory{}, err
	}
	story.ProjectionVTextDocs = map[string]string{style.ID: projectionDoc.DocID}
	return doc, storedRev, style, projection, story, nil
}

func selectGlobalWireBaseStyles(styles []types.GlobalWireStyleSource, ids []string) []types.GlobalWireStyleSource {
	out := []types.GlobalWireStyleSource{}
	if len(ids) == 0 {
		return styles[:globalWireMinInt(len(styles), 2)]
	}
	for _, id := range ids {
		style := findGlobalWireStyleSource(styles, id)
		if style.ID != "" {
			out = append(out, style)
		}
	}
	return out
}

func findGlobalWireStyleSource(styles []types.GlobalWireStyleSource, id string) types.GlobalWireStyleSource {
	id = strings.TrimSpace(id)
	for _, style := range styles {
		if style.ID == id {
			return style
		}
	}
	return types.GlobalWireStyleSource{}
}

func applyGlobalWireStyleSourceTransition(styles []types.GlobalWireStyleSource, style types.GlobalWireStyleSource, req globalWireStyleSourceRequest) []types.GlobalWireStyleSource {
	if req.Action == "replace" {
		out := make([]types.GlobalWireStyleSource, 0, len(styles))
		replaced := false
		for _, existing := range styles {
			if existing.ID == req.ReplaceStyleID {
				out = append(out, style)
				replaced = true
				continue
			}
			out = append(out, existing)
		}
		if !replaced {
			out = append(out, style)
		}
		return out
	}
	return append(styles, style)
}

func globalWireComposedStyleCitations(story types.GlobalWireStory, styles []types.GlobalWireStyleSource) []types.Citation {
	citations := []types.Citation{
		{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
	}
	for _, style := range styles {
		citations = append(citations, types.Citation{
			ID:    "base-style-" + style.ID,
			Type:  "vtext",
			Value: firstNonEmptyString(style.DocID, style.SourcePath),
			Label: firstNonEmptyString(style.Title, style.Label),
		})
	}
	return citations
}

func globalWireStyleIDs(styles []types.GlobalWireStyleSource) []string {
	out := make([]string, 0, len(styles))
	for _, style := range styles {
		out = append(out, style.ID)
	}
	return out
}

func defaultGlobalWireComposedStyleTitle(action string, styles []types.GlobalWireStyleSource) string {
	if action == "replace" {
		return "Style.vtext: Replacement " + firstNonEmptyString(styles[0].Label, styles[0].Title)
	}
	secondIndex := globalWireMinInt(len(styles)-1, 1)
	return "Style.vtext: " + firstNonEmptyString(styles[0].Label, styles[0].Title) + " + " + firstNonEmptyString(styles[secondIndex].Label, styles[secondIndex].Title)
}

func defaultGlobalWireComposedStyleLabel(action string, styles []types.GlobalWireStyleSource) string {
	if action == "replace" {
		return "Replace"
	}
	if len(styles) >= 2 {
		return firstNonEmptyString(styles[0].Label, "A") + "+" + firstNonEmptyString(styles[1].Label, "B")
	}
	return "Hybrid"
}

func defaultGlobalWireComposedStyleSummary(action string, styles []types.GlobalWireStyleSource) string {
	names := []string{}
	for _, style := range styles {
		names = append(names, firstNonEmptyString(style.Title, style.Label))
	}
	if action == "replace" {
		return "Replacement Style.vtext source derived from " + strings.Join(names, ", ") + " with explicit provenance."
	}
	return "Hybrid Style.vtext source composed from " + strings.Join(names, ", ") + " while preserving source provenance and non-oracle guardrails."
}

func globalWireComposedStyleVTextContent(action, title, summary string, story types.GlobalWireStory, styles []types.GlobalWireStyleSource) string {
	lines := []string{
		"# " + title,
		"",
		summary,
		"",
		"Style transition: " + action,
		"StoryGraph id: " + story.ID,
		"",
		"## Parent Style.vtext Sources",
		"",
	}
	for _, style := range styles {
		lines = append(lines, "- "+firstNonEmptyString(style.Title, style.Label)+" ("+firstNonEmptyString(style.DocID, style.SourcePath)+")")
	}
	lines = append(lines,
		"",
		"## Projection Guardrails",
		"",
		"- Preserve the same StoryGraph evidence manifest.",
		"- Change framing, salience, rhythm, and uncertainty emphasis without inventing facts.",
		"- Keep contrary and qualifying evidence visible.",
		"- Cite this composed Style.vtext when it materially shapes a projection.",
	)
	return strings.Join(lines, "\n")
}

func globalWireComposedStyleProjectionText(story types.GlobalWireStory, style types.GlobalWireStyleSource, baseStyles []types.GlobalWireStyleSource) string {
	return strings.Join([]string{
		"Composed Style.vtext projection for " + story.Headline + ".",
		"Style source: " + style.Title + ".",
		"Parent styles: " + strings.Join(globalWireStyleIDs(baseStyles), ", ") + ".",
		"Evidence manifest unchanged: lead, supporting, contrary, and context tiers remain attached to the StoryGraph.",
		"Projection emphasis: " + style.Summary,
	}, " ")
}

func (h *APIHandler) createGlobalWireComposedProjectionVText(r *http.Request, ownerID string, story types.GlobalWireStory, style types.GlobalWireStyleSource, baseStyles []types.GlobalWireStyleSource, projectionText string, now time.Time) (types.Document, types.Revision, error) {
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     story.Headline + " - " + style.Label + " projection",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	citations := globalWireComposedStyleCitations(story, append([]types.GlobalWireStyleSource{style}, baseStyles...))
	citations = append(citations, globalWireRuntimeSourceCitations(story)...)
	citationsJSON, err := json.Marshal(citations)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"created_from":  "global_wire_composed_style_projection",
		"storygraph_id": story.ID,
		"style_id":      style.ID,
		"style_doc_id":  style.DocID,
		"base_styles":   globalWireStyleIDs(baseStyles),
	})
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	rev := types.Revision{
		RevisionID:  uuid.NewString(),
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "Global Wire",
		Content:     globalWireComposedProjectionVTextContent(story, style, projectionText),
		Citations:   citationsJSON,
		Metadata:    metadata,
		CreatedAt:   now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	sourcePath := "global-wire/" + story.ID + "." + style.ID + ".story.vtext"
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	return doc, storedRev, nil
}

func globalWireRuntimeSourceCitations(story types.GlobalWireStory) []types.Citation {
	all := []types.GlobalWireSourceItem{}
	all = append(all, story.Manifest.Lead...)
	all = append(all, story.Manifest.Supporting...)
	all = append(all, story.Manifest.Contrary...)
	all = append(all, story.Manifest.Context...)
	citations := make([]types.Citation, 0, len(all))
	for _, item := range all {
		if strings.TrimSpace(item.ContentID) == "" {
			continue
		}
		citations = append(citations, types.Citation{
			ID:    item.ID,
			Type:  "content_item",
			Value: item.ContentID,
			Label: item.Title,
		})
	}
	return citations
}

func globalWireComposedProjectionVTextContent(story types.GlobalWireStory, style types.GlobalWireStyleSource, projectionText string) string {
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		"Style source: " + style.Title,
		"StoryGraph id: " + story.ID,
		"Projection relation: StoryGraph + composed Style.vtext + audience/task context -> Story VText",
		"",
		"## Projection",
		"",
		projectionText,
		"",
		"## Evidence Invariant",
		"",
		"This projection cites a composed/replacement Style.vtext source. It changes framing and salience without changing the StoryGraph evidence manifest or mutating user-owned forks.",
	}
	return strings.Join(lines, "\n")
}

func globalWireMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
