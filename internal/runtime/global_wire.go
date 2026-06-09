package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
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
	Edition      *globalWireEditionResponse    `json:"edition,omitempty"`
}

type globalWireEditionResponse struct {
	DocID          string   `json:"doc_id"`
	RevisionID     string   `json:"revision_id"`
	SourcePath     string   `json:"source_path"`
	Title          string   `json:"title"`
	IncludedDocIDs []string `json:"included_doc_ids"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
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
	Status             string                                  `json:"status"`
	Source             string                                  `json:"source"`
	Query              string                                  `json:"query,omitempty"`
	Message            string                                  `json:"message,omitempty"`
	RefreshRun         types.GlobalWireSourceRefreshRun        `json:"refresh_run"`
	ContentItem        *types.ContentItem                      `json:"content_item,omitempty"`
	Contribution       *types.GlobalWireContribution           `json:"contribution,omitempty"`
	Decision           *types.GlobalWireReconciliationDecision `json:"decision,omitempty"`
	Candidate          *types.GlobalWireGraphUpdateCandidate   `json:"candidate,omitempty"`
	ProjectionReviews  []types.GlobalWireProjectionReview      `json:"projection_reviews,omitempty"`
	ClaimRecord        *types.GlobalWireClaimRecord            `json:"claim_record,omitempty"`
	SourceReviewSignal *types.GlobalWireSourceReviewSignal     `json:"source_review_signal,omitempty"`
	ResearchTask       *types.GlobalWireResearchTask           `json:"research_task,omitempty"`
	ExtractionArtifact *types.GlobalWireExtractionArtifact     `json:"extraction_artifact,omitempty"`
}

type globalWireFetchCycleRequest struct {
	StoryIDs       []string `json:"story_ids,omitempty"`
	MaxStories     int      `json:"max_stories,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
	Trigger        string   `json:"trigger,omitempty"`
	SchedulerMode  bool     `json:"scheduler_mode,omitempty"`
	CadenceSeconds int      `json:"cadence_seconds,omitempty"`
	SchedulerRunID string   `json:"-"`
}

type globalWireFetchCycleResponse struct {
	Status              string                                 `json:"status"`
	Message             string                                 `json:"message"`
	FetchCycle          types.GlobalWireFetchCycleRun          `json:"fetch_cycle"`
	RegistryEntries     []types.GlobalWireSourceRegistryEntry  `json:"registry_entries"`
	RefreshRuns         []types.GlobalWireSourceRefreshRun     `json:"refresh_runs"`
	SchedulerRun        *types.GlobalWireSourceSchedulerRun    `json:"scheduler_run,omitempty"`
	SchedulerRuns       []types.GlobalWireSourceSchedulerRun   `json:"scheduler_runs,omitempty"`
	ContentItems        []types.ContentItem                    `json:"content_items,omitempty"`
	Contributions       []types.GlobalWireContribution         `json:"contributions,omitempty"`
	Candidates          []types.GlobalWireGraphUpdateCandidate `json:"candidates,omitempty"`
	ClaimRecords        []types.GlobalWireClaimRecord          `json:"claim_records,omitempty"`
	SourceReviewSignals []types.GlobalWireSourceReviewSignal   `json:"source_review_signals,omitempty"`
	ResearchTasks       []types.GlobalWireResearchTask         `json:"research_tasks,omitempty"`
	ExtractionArtifacts []types.GlobalWireExtractionArtifact   `json:"extraction_artifacts,omitempty"`
	RecentCycles        []types.GlobalWireFetchCycleRun        `json:"recent_cycles,omitempty"`
}

type globalWireSourceMaxxStatusResponse struct {
	Status                       string         `json:"status"`
	Source                       string         `json:"source"`
	Message                      string         `json:"message,omitempty"`
	CycleID                      string         `json:"cycle_id,omitempty"`
	CycleStatus                  string         `json:"cycle_status,omitempty"`
	StartedAt                    string         `json:"started_at,omitempty"`
	EndedAt                      string         `json:"ended_at,omitempty"`
	ItemCount                    int            `json:"item_count,omitempty"`
	FetchCount                   int            `json:"fetch_count,omitempty"`
	ProcessorRequestCount        int            `json:"processor_request_count,omitempty"`
	ReconcilerRequestCount       int            `json:"reconciler_request_count,omitempty"`
	ProcessorStatusCounts        map[string]int `json:"processor_status_counts,omitempty"`
	ReconcilerStatusCounts       map[string]int `json:"reconciler_status_counts,omitempty"`
	ProcessorRuntimeRunCount     int            `json:"processor_runtime_run_count,omitempty"`
	ReconcilerRuntimeRunCount    int            `json:"reconciler_runtime_run_count,omitempty"`
	ProcessorResolvedRunCount    int            `json:"processor_resolved_runtime_run_count,omitempty"`
	ReconcilerResolvedRunCount   int            `json:"reconciler_resolved_runtime_run_count,omitempty"`
	ProcessorUnresolvedRunCount  int            `json:"processor_unresolved_runtime_run_count,omitempty"`
	ReconcilerUnresolvedRunCount int            `json:"reconciler_unresolved_runtime_run_count,omitempty"`
	ProcessorRunStateCounts      map[string]int `json:"processor_run_state_counts,omitempty"`
	ReconcilerRunStateCounts     map[string]int `json:"reconciler_run_state_counts,omitempty"`
	ProcessorUpdateCount         int            `json:"processor_update_count,omitempty"`
	ReconcilerUpdateCount        int            `json:"reconciler_update_count,omitempty"`
	ProcessorChildProfileCounts  map[string]int `json:"processor_child_profile_counts,omitempty"`
	ReconcilerChildProfileCounts map[string]int `json:"reconciler_child_profile_counts,omitempty"`
	ProcessorKeys                []string       `json:"processor_keys,omitempty"`
	ReconcilerScopes             []string       `json:"reconciler_scopes,omitempty"`
	Topology                     string         `json:"topology,omitempty"`
	AuthorityRule                string         `json:"authority_rule,omitempty"`
	SourceServiceInternalOnly    bool           `json:"source_service_internal_only"`
}

type sourceMaxxRuntimeEvidenceClient struct {
	baseURL string
	ownerID string
	client  *http.Client
}

type sourceMaxxResolvedRun struct {
	RunID        string
	AgentProfile string
	State        types.RunState
	LocalRecord  *types.RunRecord
	Events       []types.EventRecord
}

const communityWireEditionSourcePath = "global-wire/Wire.vtext"

var vtextTransclusionRefRE = regexp.MustCompile(`vtext:([A-Za-z0-9_.:-]{1,160})`)

type globalWireReconciliationResponse struct {
	Contributions         []types.GlobalWireContribution              `json:"contributions"`
	SourceItems           map[string]types.ContentItem                `json:"source_items,omitempty"`
	SourceDossiers        []globalWireSourceDossier                   `json:"source_dossiers"`
	Decisions             []types.GlobalWireReconciliationDecision    `json:"decisions"`
	Candidates            []types.GlobalWireGraphUpdateCandidate      `json:"candidates"`
	Promotions            []types.GlobalWireGraphPromotionDecision    `json:"promotions"`
	Refreshes             []types.GlobalWireSourceRefreshRun          `json:"refreshes"`
	ClaimRecords          []types.GlobalWireClaimRecord               `json:"claim_records"`
	SourceReviewSignals   []types.GlobalWireSourceReviewSignal        `json:"source_review_signals"`
	ResearchTasks         []types.GlobalWireResearchTask              `json:"research_tasks"`
	ExtractionArtifacts   []types.GlobalWireExtractionArtifact        `json:"extraction_artifacts"`
	ResearchEvidence      []types.GlobalWireResearchTaskEvidence      `json:"research_evidence"`
	ResearchDecisions     []types.GlobalWireResearchEvidenceDecision  `json:"research_decisions"`
	PublicationUpdates    []types.GlobalWirePublicationUpdate         `json:"publication_updates"`
	PublicationArtifacts  []types.GlobalWirePublicationArtifact       `json:"publication_artifacts"`
	PublicationDeliveries []types.GlobalWirePublicationDelivery       `json:"publication_deliveries"`
	AutoradioScripts      []types.GlobalWireAutoradioScript           `json:"autoradio_scripts"`
	AutoradioEpisodes     []types.GlobalWireAutoradioEpisode          `json:"autoradio_episodes"`
	DeliveryExports       []types.GlobalWirePublicationDeliveryExport `json:"delivery_exports"`
	PublicLinks           []types.GlobalWirePublicationPublicLink     `json:"public_links"`
	NewsletterSubscribers []types.GlobalWireNewsletterSubscriber      `json:"newsletter_subscribers"`
	NewsletterIssues      []types.GlobalWireNewsletterIssue           `json:"newsletter_issues"`
	NewsletterDeliveries  []types.GlobalWireNewsletterDelivery        `json:"newsletter_deliveries"`
	NewsletterReceipts    []types.GlobalWireNewsletterProviderReceipt `json:"newsletter_provider_receipts"`
	ProjectionReviews     []types.GlobalWireProjectionReview          `json:"projection_reviews"`
}

type globalWireSourceDossierResponse struct {
	Dossiers []globalWireSourceDossier `json:"dossiers"`
	Status   string                    `json:"status"`
	Source   string                    `json:"source"`
}

type globalWireDossierManifestTier struct {
	Tier       string   `json:"tier"`
	Count      int      `json:"count"`
	SourceIDs  []string `json:"source_ids"`
	ContentIDs []string `json:"content_ids,omitempty"`
	Titles     []string `json:"titles"`
}

type globalWireDossierClaim struct {
	ClaimID               string   `json:"claim_id"`
	ClaimText             string   `json:"claim_text"`
	ClaimKind             string   `json:"claim_kind"`
	Status                string   `json:"status"`
	UncertaintyState      string   `json:"uncertainty_state"`
	DisputeState          string   `json:"dispute_state"`
	EvidenceGap           string   `json:"evidence_gap"`
	SourceContentID       string   `json:"source_content_id,omitempty"`
	CandidateID           string   `json:"candidate_id,omitempty"`
	ContributionID        string   `json:"contribution_id,omitempty"`
	RefreshID             string   `json:"refresh_id,omitempty"`
	ExtractionIDs         []string `json:"extraction_ids"`
	SourceReviewSignalIDs []string `json:"source_review_signal_ids"`
	ResearchTaskIDs       []string `json:"research_task_ids"`
	ResearchEvidenceIDs   []string `json:"research_evidence_ids"`
	ResearchDecisionIDs   []string `json:"research_decision_ids"`
	PublicationUpdateIDs  []string `json:"publication_update_ids"`
}

type globalWireDossierPublicationRefs struct {
	UpdateIDs             []string `json:"update_ids"`
	ArtifactIDs           []string `json:"artifact_ids"`
	DeliveryIDs           []string `json:"delivery_ids"`
	AutoradioScriptIDs    []string `json:"autoradio_script_ids"`
	AutoradioEpisodeIDs   []string `json:"autoradio_episode_ids"`
	DeliveryExportIDs     []string `json:"delivery_export_ids"`
	PublicLinkIDs         []string `json:"public_link_ids"`
	NewsletterIssueIDs    []string `json:"newsletter_issue_ids"`
	NewsletterDeliveryIDs []string `json:"newsletter_delivery_ids"`
	NewsletterReceiptIDs  []string `json:"newsletter_provider_receipt_ids"`
	CitationRefs          []string `json:"citation_refs"`
	RollbackRefs          []string `json:"rollback_refs"`
}

type globalWireSourceDossier struct {
	ID                  string                               `json:"id"`
	StoryID             string                               `json:"story_id"`
	Headline            string                               `json:"headline"`
	SourceState         string                               `json:"source_state"`
	ManifestTiers       []globalWireDossierManifestTier      `json:"manifest_tiers"`
	ClaimDossiers       []globalWireDossierClaim             `json:"claim_dossiers"`
	SourceReviewSignals []types.GlobalWireSourceReviewSignal `json:"source_review_signals"`
	ExtractionIDs       []string                             `json:"extraction_ids"`
	ResearchTaskIDs     []string                             `json:"research_task_ids"`
	ResearchEvidenceIDs []string                             `json:"research_evidence_ids"`
	CandidateIDs        []string                             `json:"candidate_ids"`
	ContributionIDs     []string                             `json:"contribution_ids"`
	RefreshRunIDs       []string                             `json:"refresh_run_ids"`
	PublicationRefs     globalWireDossierPublicationRefs     `json:"publication_refs"`
	SourceContentIDs    []string                             `json:"source_content_ids"`
	EntityTerms         []string                             `json:"entity_terms"`
	EventTerms          []string                             `json:"event_terms"`
	Timeline            []string                             `json:"timeline"`
	MissingFields       []string                             `json:"missing_fields"`
	ReviewState         string                               `json:"review_state"`
	ProvenanceRefs      []string                             `json:"provenance_refs"`
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

type globalWireResearchEvidenceDecisionRequest struct {
	EvidenceID string `json:"evidence_id"`
	Decision   string `json:"decision"`
	Note       string `json:"note,omitempty"`
}

type globalWireResearchEvidenceDecisionResponse struct {
	Decision  types.GlobalWireResearchEvidenceDecision `json:"decision"`
	Task      types.GlobalWireResearchTask             `json:"task"`
	Evidence  types.GlobalWireResearchTaskEvidence     `json:"evidence"`
	Candidate *types.GlobalWireGraphUpdateCandidate    `json:"candidate,omitempty"`
}

type globalWirePublicationUpdateRequest struct {
	ResearchDecisionID string `json:"research_decision_id"`
	Summary            string `json:"summary,omitempty"`
}

type globalWirePublicationUpdateResponse struct {
	Update            types.GlobalWirePublicationUpdate        `json:"update"`
	ResearchDecision  types.GlobalWireResearchEvidenceDecision `json:"research_decision"`
	Candidate         *types.GlobalWireGraphUpdateCandidate    `json:"candidate,omitempty"`
	ProjectionReviews []types.GlobalWireProjectionReview       `json:"projection_reviews,omitempty"`
	SourceItem        *types.ContentItem                       `json:"source_item,omitempty"`
}

type globalWirePublicationArtifactRequest struct {
	UpdateID string `json:"update_id"`
	Channel  string `json:"channel,omitempty"`
	Title    string `json:"title,omitempty"`
}

type globalWirePublicationArtifactResponse struct {
	Artifact          types.GlobalWirePublicationArtifact `json:"artifact"`
	Update            types.GlobalWirePublicationUpdate   `json:"update"`
	Story             types.GlobalWireStory               `json:"story"`
	ProjectionReviews []types.GlobalWireProjectionReview  `json:"projection_reviews,omitempty"`
	SourceItem        *types.ContentItem                  `json:"source_item,omitempty"`
}

type globalWirePublicationFeedItem struct {
	Artifact      types.GlobalWirePublicationArtifact `json:"artifact"`
	Story         types.GlobalWireStory               `json:"story"`
	SourceItem    *types.ContentItem                  `json:"source_item,omitempty"`
	CitationCount int                                 `json:"citation_count"`
	RollbackCount int                                 `json:"rollback_count"`
	Status        string                              `json:"status"`
}

type globalWirePublicationFeedResponse struct {
	FeedItems []globalWirePublicationFeedItem `json:"feed_items"`
	Channel   string                          `json:"channel"`
	Status    string                          `json:"status"`
}

type globalWirePublicationArtifactReviewRequest struct {
	ArtifactID string `json:"artifact_id"`
	Decision   string `json:"decision"`
	Note       string `json:"note,omitempty"`
}

type globalWirePublicationArtifactReviewResponse struct {
	Artifact types.GlobalWirePublicationArtifact `json:"artifact"`
	Status   string                              `json:"status"`
	Decision string                              `json:"decision"`
	Edition  *globalWireEditionResponse          `json:"edition,omitempty"`
}

type globalWirePublicationDeliveryRequest struct {
	ArtifactID string `json:"artifact_id"`
	Channel    string `json:"channel,omitempty"`
}

type globalWirePublicationDeliveryResponse struct {
	Delivery types.GlobalWirePublicationDelivery `json:"delivery"`
	Artifact types.GlobalWirePublicationArtifact `json:"artifact"`
	Story    types.GlobalWireStory               `json:"story"`
}

type globalWirePublicationDeliveryDetailResponse struct {
	Delivery   types.GlobalWirePublicationDelivery `json:"delivery"`
	Artifact   types.GlobalWirePublicationArtifact `json:"artifact"`
	Story      types.GlobalWireStory               `json:"story"`
	SourceItem *types.ContentItem                  `json:"source_item,omitempty"`
}

type globalWireAutoradioScriptRequest struct {
	ArtifactID string `json:"artifact_id"`
}

type globalWireAutoradioScriptResponse struct {
	Script     types.GlobalWireAutoradioScript     `json:"script"`
	Artifact   types.GlobalWirePublicationArtifact `json:"artifact"`
	Story      types.GlobalWireStory               `json:"story"`
	SourceItem *types.ContentItem                  `json:"source_item,omitempty"`
}

type globalWireAutoradioEpisodeRequest struct {
	ScriptID string `json:"script_id"`
}

type globalWireAutoradioEpisodeResponse struct {
	Episode    types.GlobalWireAutoradioEpisode    `json:"episode"`
	Script     types.GlobalWireAutoradioScript     `json:"script"`
	Artifact   types.GlobalWirePublicationArtifact `json:"artifact"`
	Story      types.GlobalWireStory               `json:"story"`
	SourceItem *types.ContentItem                  `json:"source_item,omitempty"`
}

type globalWirePublicationDeliveryExportRequest struct {
	DeliveryID string `json:"delivery_id"`
	Format     string `json:"format,omitempty"`
}

type globalWirePublicationDeliveryExportResponse struct {
	Export     types.GlobalWirePublicationDeliveryExport `json:"export"`
	Delivery   types.GlobalWirePublicationDelivery       `json:"delivery"`
	Artifact   types.GlobalWirePublicationArtifact       `json:"artifact"`
	Story      types.GlobalWireStory                     `json:"story"`
	Script     *types.GlobalWireAutoradioScript          `json:"script,omitempty"`
	SourceItem *types.ContentItem                        `json:"source_item,omitempty"`
}

type globalWirePublicationPublicLinkRequest struct {
	ExportID string `json:"export_id"`
}

type globalWirePublicationPublicLinkResponse struct {
	PublicLink types.GlobalWirePublicationPublicLink     `json:"public_link"`
	Export     types.GlobalWirePublicationDeliveryExport `json:"export,omitempty"`
}

type globalWireNewsletterSubscriberRequest struct {
	Email string `json:"email"`
	Label string `json:"label,omitempty"`
}

type globalWireNewsletterSubscriberResponse struct {
	Subscriber types.GlobalWireNewsletterSubscriber `json:"subscriber"`
}

type globalWireNewsletterIssueRequest struct {
	PublicLinkIDs []string `json:"public_link_ids,omitempty"`
	StoryID       string   `json:"story_id,omitempty"`
	Subject       string   `json:"subject,omitempty"`
}

type globalWireNewsletterIssueResponse struct {
	Issue       types.GlobalWireNewsletterIssue             `json:"issue"`
	Deliveries  []types.GlobalWireNewsletterDelivery        `json:"deliveries"`
	Receipts    []types.GlobalWireNewsletterProviderReceipt `json:"newsletter_provider_receipts"`
	PublicLinks []types.GlobalWirePublicationPublicLink     `json:"public_links"`
	Subscribers []types.GlobalWireNewsletterSubscriber      `json:"subscribers"`
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

// HandleGlobalWireStories returns authenticated Wire stories without inventing
// seeded front-page content.
func (h *APIHandler) HandleGlobalWireStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	_, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	stories := []types.GlobalWireStory{}
	styleSources := []types.GlobalWireStyleSource{}
	source := "community-wire-vtext-index"
	var edition *globalWireEditionResponse
	if editionStories, editionResp, err := h.communityWireEditionVTextStories(r.Context(), styleSources, 12); err == nil {
		edition = editionResp
		if len(editionStories) > 0 {
			stories = editionStories
			source = "community-wire-edition-vtext"
		} else if editionResp != nil {
			source = "community-wire-edition-vtext"
		}
	} else if err != nil {
		log.Printf("global wire: community wire edition unavailable: %v", err)
	}
	for i := range stories {
		stories[i] = normalizeGlobalWireStoryPresentation(stories[i])
	}
	writeAPIJSON(w, http.StatusOK, globalWireStoriesResponse{
		Stories:      stories,
		StyleSources: styleSources,
		Source:       source,
		Edition:      edition,
	})
}

func (h *APIHandler) communityWireEditionVTextStories(ctx context.Context, styleSources []types.GlobalWireStyleSource, limit int) ([]types.GlobalWireStory, *globalWireEditionResponse, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return nil, nil, nil
	}
	platformOwner := sourceMaxxPlatformOwnerID()
	editionDocID, err := h.rt.Store().GetDocumentAlias(ctx, platformOwner, communityWireEditionSourcePath)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	editionDoc, err := h.rt.Store().GetDocument(ctx, editionDocID, platformOwner)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if strings.TrimSpace(editionDoc.CurrentRevisionID) == "" {
		return nil, &globalWireEditionResponse{
			DocID:      editionDoc.DocID,
			SourcePath: communityWireEditionSourcePath,
			Title:      editionDoc.Title,
			UpdatedAt:  editionDoc.UpdatedAt.Format(time.RFC3339Nano),
		}, nil
	}
	editionRev, err := h.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, platformOwner)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	includedDocIDs := communityWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID)
	edition := &globalWireEditionResponse{
		DocID:          editionDoc.DocID,
		RevisionID:     editionRev.RevisionID,
		SourcePath:     communityWireEditionSourcePath,
		Title:          editionDoc.Title,
		IncludedDocIDs: includedDocIDs,
		UpdatedAt:      editionDoc.UpdatedAt.Format(time.RFC3339Nano),
	}
	stories := make([]types.GlobalWireStory, 0, min(len(includedDocIDs), limit))
	for _, docID := range includedDocIDs {
		if limit > 0 && len(stories) >= limit {
			break
		}
		doc, err := h.rt.Store().GetDocument(ctx, docID, platformOwner)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return nil, nil, err
		}
		if strings.TrimSpace(doc.CurrentRevisionID) == "" {
			continue
		}
		rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, platformOwner)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return nil, nil, err
		}
		story, ok := sourceMaxxVTextStoryFromCurrentRevision(ctx, doc, rev, styleSources)
		if !ok {
			continue
		}
		story.Prominence = 100 - len(stories)
		story.SourceState = "community-wire-edition-vtext"
		stories = append(stories, story)
	}
	return stories, edition, nil
}

func communityWireEditionIncludedDocIDs(content, editionDocID string) []string {
	seen := map[string]bool{}
	editionDocID = strings.TrimSpace(editionDocID)
	out := []string{}
	for _, match := range vtextTransclusionRefRE.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		docID := strings.TrimSpace(match[1])
		if docID == "" || docID == editionDocID || seen[docID] {
			continue
		}
		seen[docID] = true
		out = append(out, docID)
	}
	return out
}

func (h *APIHandler) sourceMaxxVTextStories(ctx context.Context, styleSources []types.GlobalWireStyleSource, limit int) ([]types.GlobalWireStory, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return nil, nil
	}
	platformOwner := sourceMaxxPlatformOwnerID()
	docs, err := h.rt.Store().ListDocumentsByOwner(ctx, platformOwner, 200)
	if err != nil {
		return nil, err
	}
	out := make([]types.GlobalWireStory, 0, limit)
	for _, doc := range docs {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(doc.CurrentRevisionID) == "" {
			continue
		}
		rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, platformOwner)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return nil, err
		}
		story, ok := sourceMaxxVTextStoryFromCurrentRevision(ctx, doc, rev, styleSources)
		if ok {
			out = append(out, story)
		}
	}
	return out, nil
}

func sourceMaxxPlatformOwnerID() string {
	ownerID := strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "global-wire-platform"
	}
	return ownerID
}

func sourceMaxxVTextStoryFromCurrentRevision(ctx context.Context, doc types.Document, rev types.Revision, styleSources []types.GlobalWireStyleSource) (types.GlobalWireStory, bool) {
	meta := decodeRevisionMetadata(rev.Metadata)
	cycleID := sourceNetworkCycleID(meta)
	if metadataString(meta, "source") != "edit_vtext" || cycleID == "" {
		return types.GlobalWireStory{}, false
	}
	content := strings.TrimSpace(rev.Content)
	if content == "" || sourceMaxxContentLooksLikeSeed(content) {
		return types.GlobalWireStory{}, false
	}
	styleID, styleTitle := sourceMaxxSelectedStyle(meta, styleSources)
	headline := sourceMaxxArticleHeadline(doc.Title, content)
	dek := sourceMaxxArticleDek(content)
	projection := sourceMaxxArticleProjection(content)
	manifest := sourceMaxxManifestFromRevision(ctx, meta, content, headline)
	if len(manifest.Lead) == 0 &&
		len(manifest.Supporting) == 0 &&
		len(manifest.Contrary) == 0 &&
		len(manifest.Context) == 0 {
		manifest.Context = append(manifest.Context, types.GlobalWireSourceItem{
			ID:       "source-network-cycle:" + cycleID,
			Title:    "Source network cycle " + cycleID,
			Standing: "source firehose cycle",
			Role:     "context",
		})
	}
	projections := map[string]string{styleID: projection}
	if styleID != "wire-style" {
		projections["wire-style"] = projection
	}
	return types.GlobalWireStory{
		ID:                  "source-network-vtext-" + doc.DocID,
		OwnerID:             doc.OwnerID,
		Headline:            headline,
		Dek:                 dek,
		Freshness:           sourceMaxxFreshness(doc.UpdatedAt),
		Prominence:          90,
		Tension:             "source-network article",
		ChangeState:         "vtext published",
		NodeTone:            "live",
		Related:             []string{},
		Manifest:            manifest,
		Claims:              sourceMaxxArticleClaims(content, styleTitle, meta),
		Projections:         projections,
		ProjectionVTextDocs: map[string]string{styleID: doc.DocID},
		StyleSources:        styleSources,
		StoryVTextDoc:       doc.DocID,
		VTextContent:        content,
		SourceState:         "source-network-vtext-index",
		CreatedAt:           doc.CreatedAt,
		UpdatedAt:           doc.UpdatedAt,
	}, true
}

func sourceMaxxContentLooksLikeSeed(content string) bool {
	return strings.Contains(content, "## Source Brief") ||
		strings.Contains(content, "## SourceMaxx Brief") ||
		strings.Contains(content, "## Evidence Gathering") ||
		strings.Contains(content, "## Working Revision")
}

func sourceMaxxSelectedStyle(meta map[string]any, styles []types.GlobalWireStyleSource) (string, string) {
	title := "Style.vtext: Global Wire"
	if selected, ok := meta["selected_style_sources"].([]any); ok && len(selected) > 0 {
		if first, ok := selected[0].(map[string]any); ok {
			if raw := strings.TrimSpace(stringValue(first["title"])); raw != "" {
				title = raw
			}
		}
	}
	for _, style := range styles {
		if strings.EqualFold(strings.TrimSpace(style.Title), title) {
			return style.ID, style.Title
		}
	}
	return "wire-style", title
}

func sourceMaxxMetadataStringSlice(value any) []string {
	out := []string{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if str := strings.TrimSpace(stringValue(item)); str != "" {
				out = append(out, str)
			}
		}
	case []string:
		for _, item := range typed {
			if str := strings.TrimSpace(item); str != "" {
				out = append(out, str)
			}
		}
	}
	return out
}

func sourceMaxxManifestFromRevision(ctx context.Context, meta map[string]any, content, headline string) types.GlobalWireSourceManifest {
	entities := sourceMaxxVisibleSourceEntities(ctx, meta, content)
	if len(entities) > 0 {
		return sourceMaxxManifestFromSourceEntities(entities)
	}
	return sourceMaxxManifestFromCycleProvenance(meta, headline)
}

func sourceMaxxVisibleSourceEntities(ctx context.Context, meta map[string]any, content string) []vtextSourceEntity {
	entities := decodeVTextSourceEntities(meta["source_entities"])
	if len(entities) == 0 {
		return nil
	}
	refs := sourceMaxxInlineSourceRefs(sourceMaxxArticleProseForSourceRefs(content))
	if len(refs) == 0 {
		return nil
	}
	out := []vtextSourceEntity{}
	seen := map[string]bool{}
	for _, entity := range entities {
		id := strings.TrimSpace(entity.EntityID)
		if id == "" || !refs[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, entity)
	}
	enrichSourceServiceEntities(ctx, out)
	return out
}

func sourceMaxxArticleProseForSourceRefs(content string) string {
	var b strings.Builder
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if sourceMaxxArticleLineStartsInventorySection(line) {
			break
		}
		b.WriteString(raw)
		b.WriteString("\n")
	}
	return b.String()
}

func sourceMaxxInlineSourceRefs(content string) map[string]bool {
	out := map[string]bool{}
	rest := content
	for {
		idx := strings.Index(rest, "(source:")
		if idx < 0 {
			break
		}
		rest = rest[idx+len("(source:"):]
		end := strings.Index(rest, ")")
		if end < 0 {
			break
		}
		id := strings.TrimSpace(rest[:end])
		if id != "" {
			out[id] = true
		}
		rest = rest[end+1:]
	}
	rest = content
	for {
		idx := strings.Index(rest, "[source:")
		if idx < 0 {
			break
		}
		rest = rest[idx+len("[source:"):]
		end := strings.Index(rest, "]")
		if end < 0 {
			break
		}
		id := strings.TrimSpace(rest[:end])
		if id != "" {
			out[id] = true
		}
		rest = rest[end+1:]
	}
	return out
}

func sourceMaxxManifestFromSourceEntities(entities []vtextSourceEntity) types.GlobalWireSourceManifest {
	manifest := types.GlobalWireSourceManifest{}
	for i, entity := range entities {
		id := sourceMaxxSourceEntityManifestID(entity)
		if id == "" {
			continue
		}
		item := types.GlobalWireSourceItem{
			ID:           id,
			Title:        sourceMaxxSourceEntityManifestTitle(entity),
			Standing:     sourceMaxxSourceEntityManifestStanding(entity),
			Role:         "lead",
			SourceID:     strings.TrimSpace(entity.Target.SourceID),
			FetchID:      strings.TrimSpace(entity.Target.FetchID),
			CanonicalURL: firstNonEmpty(entity.Target.CanonicalURL, entity.Target.URL),
		}
		if i >= 3 {
			item.Role = "context"
			manifest.Context = append(manifest.Context, item)
			continue
		}
		manifest.Lead = append(manifest.Lead, item)
	}
	return manifest
}

func sourceMaxxSourceEntityManifestID(entity vtextSourceEntity) string {
	return firstNonEmpty(entity.Target.ItemID, entity.Target.ContentID, entity.Target.DocID, entity.EntityID)
}

func sourceMaxxSourceEntityManifestTitle(entity vtextSourceEntity) string {
	return firstNonEmpty(entity.Label, entity.Target.CanonicalURL, entity.Target.URL, sourceMaxxSourceEntityManifestID(entity))
}

func sourceMaxxSourceEntityManifestStanding(entity vtextSourceEntity) string {
	switch strings.TrimSpace(entity.Kind) {
	case "content_item":
		return "embedded source"
	case "source_service_item":
		return "source-service handle"
	case "vtext":
		return "related VText"
	default:
		return firstNonEmpty(entity.Kind, "source handle")
	}
}

func sourceMaxxManifestFromCycleProvenance(meta map[string]any, headline string) types.GlobalWireSourceManifest {
	manifest := types.GlobalWireSourceManifest{}
	cycleID := sourceNetworkCycleID(meta)
	sourceIDs := sourceMaxxMetadataStringSlice(meta["source_item_ids"])
	switch {
	case cycleID != "":
		standing := "source firehose cycle"
		if len(sourceIDs) > 0 {
			standing = fmt.Sprintf("source firehose cycle; %d source handles retained in revision provenance", len(sourceIDs))
		}
		manifest.Context = append(manifest.Context, types.GlobalWireSourceItem{
			ID:       "source-network-cycle:" + cycleID,
			Title:    "Source network cycle " + cycleID,
			Standing: standing,
			Role:     "context",
		})
	case strings.TrimSpace(headline) != "":
		manifest.Context = append(manifest.Context, types.GlobalWireSourceItem{
			ID:       "source-network-vtext:" + headline,
			Title:    "Global Wire VText article head",
			Standing: "platform VText current revision",
			Role:     "context",
		})
	}
	return manifest
}

func sourceMaxxArticleHeadline(title, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	title = strings.TrimSpace(strings.TrimSuffix(title, ".vtext"))
	if title != "" {
		return title
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "# -*\t"))
		if line != "" {
			return truncateRunes(line, 120)
		}
	}
	return "Global Wire article"
}

func sourceMaxxArticleDek(content string) string {
	for _, paragraph := range sourceMaxxArticleParagraphs(content) {
		return truncateRunes(paragraph, 220)
	}
	return "Global Wire VText article with source and style provenance on its current revision."
}

func sourceMaxxArticleProjection(content string) string {
	paragraphs := sourceMaxxArticleParagraphs(content)
	if len(paragraphs) == 0 {
		return truncateRunes(content, 520)
	}
	return truncateRunes(strings.Join(paragraphs, "\n\n"), 900)
}

func sourceMaxxArticleParagraphs(content string) []string {
	out := []string{}
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraph := strings.TrimSpace(strings.Join(current, " "))
		current = nil
		if paragraph != "" {
			out = append(out, paragraph)
		}
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if sourceMaxxArticleLineIsScaffold(line) {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, ">") {
			flush()
			continue
		}
		current = append(current, line)
		if len(out) >= 4 {
			break
		}
	}
	flush()
	return out
}

func sourceMaxxArticleLineIsScaffold(line string) bool {
	trimmed := strings.TrimSpace(line)
	plain := strings.Trim(trimmed, "*_ \t")
	lower := strings.ToLower(plain)
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(trimmed, "**", ""), "__", "")))
	if plain == "---" || plain == "***" {
		return true
	}
	if strings.HasPrefix(lower, "published:") ||
		strings.HasPrefix(lower, "date:") ||
		strings.HasPrefix(lower, "status:") ||
		strings.HasPrefix(lower, "by ") ||
		strings.HasPrefix(lower, "source:") ||
		strings.HasPrefix(lower, "style.vtext source") ||
		strings.HasPrefix(lower, "style source:") ||
		strings.HasPrefix(lower, "selection rationale:") ||
		strings.HasPrefix(lower, "story id:") ||
		strings.HasPrefix(lower, "state:") {
		return true
	}
	if lower == "source handles" ||
		lower == "source manifest" ||
		lower == "style.vtext source" {
		return true
	}
	if strings.HasPrefix(normalized, "published:") ||
		strings.HasPrefix(normalized, "date:") ||
		strings.HasPrefix(normalized, "status:") ||
		strings.HasPrefix(normalized, "by ") ||
		strings.HasPrefix(normalized, "source:") {
		return true
	}
	return sourceMaxxArticleLineStartsInventorySection(trimmed)
}

func sourceMaxxArticleLineStartsInventorySection(line string) bool {
	plain := strings.TrimSpace(strings.TrimLeft(line, "#*_ \t"))
	lower := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(plain, "**", ""), "__", "")))
	if lower == "source handles" ||
		lower == "source manifest" ||
		lower == "sources" ||
		lower == "style.vtext source" ||
		lower == "style source" {
		return true
	}
	if strings.HasPrefix(lower, "source handles:") ||
		strings.HasPrefix(lower, "source manifest:") ||
		strings.HasPrefix(lower, "style.vtext source:") ||
		strings.HasPrefix(lower, "style source:") {
		return true
	}
	return false
}

func sourceMaxxArticleClaims(content, _ string, meta map[string]any) []string {
	claims := []string{
		"Current head is a normal VText article revision owned by the Global Wire platform agent.",
		"Source and style provenance are carried by the VText revision metadata and citations.",
	}
	if cycleID := sourceNetworkCycleID(meta); cycleID != "" {
		claims = append(claims, "Source network cycle: "+cycleID)
	}
	if rationale := metadataString(meta, "selected_style_rationale"); rationale != "" {
		claims = append(claims, "Style rationale: "+truncateRunes(rationale, 180))
	}
	if len(claims) > 4 {
		return claims[:4]
	}
	_ = content
	return claims
}

func sourceNetworkCycleID(meta map[string]any) string {
	return firstNonEmptyString(metadataString(meta, "source_network_cycle_id"), metadataString(meta, "source_maxx_cycle_id"))
}

func sourceMaxxFreshness(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return "source-network current"
	}
	delta := time.Since(updatedAt)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return "updated just now"
	case delta < time.Hour:
		return fmt.Sprintf("updated %d min ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("updated %d hr ago", int(delta.Hours()))
	default:
		return updatedAt.UTC().Format("2006-01-02")
	}
}

func normalizeGlobalWireStoryPresentation(story types.GlobalWireStory) types.GlobalWireStory {
	if globalWireStoryFreshnessLooksAuto(story.Freshness) {
		if strings.EqualFold(strings.TrimSpace(story.SourceState), "seeded-source-neighborhood") {
			story.Freshness = "seed source neighborhood"
			return story
		}
		story.Freshness = sourceMaxxFreshness(story.UpdatedAt)
	}
	return story
}

func globalWireStoryFreshnessLooksAuto(freshness string) bool {
	freshness = strings.TrimSpace(strings.ToLower(freshness))
	return freshness == "" || strings.HasPrefix(freshness, "updated ")
}

// HandleGlobalWireSourceStatus reports non-sensitive source-service aggregate
// status through the product API while preserving the private /internal source
// service boundary.
func (h *APIHandler) HandleGlobalWireSourceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	sourceClient := newSourceSearchClientFromEnv()
	statusClient, ok := sourceClient.(sourceMaxxStatusClient)
	if sourceClient == nil || !ok {
		writeAPIJSON(w, http.StatusServiceUnavailable, globalWireSourceMaxxStatusResponse{
			Status:                    "unavailable",
			Source:                    "source-service",
			Message:                   "Source Service is not configured for this runtime.",
			SourceServiceInternalOnly: true,
		})
		return
	}
	resp, err := statusClient.SourceMaxxLatest(r.Context())
	if err != nil {
		writeAPIJSON(w, http.StatusBadGateway, globalWireSourceMaxxStatusResponse{
			Status:                    "unavailable",
			Source:                    "source-service",
			Message:                   err.Error(),
			SourceServiceInternalOnly: true,
		})
		return
	}
	status := globalWireSourceMaxxStatusFromAPI(resp)
	h.addSourceMaxxRuntimeEvidence(r.Context(), resp, &status)
	writeAPIJSON(w, http.StatusOK, status)
}

// HandleGlobalWireSourceMaxxStatus is kept as a compatibility alias for older
// clients. New product surfaces should use /api/global-wire/source-status.
func (h *APIHandler) HandleGlobalWireSourceMaxxStatus(w http.ResponseWriter, r *http.Request) {
	h.HandleGlobalWireSourceStatus(w, r)
}

func globalWireSourceMaxxStatusFromAPI(resp *sourceapi.SourceMaxxResponse) globalWireSourceMaxxStatusResponse {
	if resp == nil {
		return globalWireSourceMaxxStatusResponse{
			Status:                    "unavailable",
			Source:                    "source-service",
			Message:                   "Source Service did not return SourceMaxx status.",
			SourceServiceInternalOnly: true,
		}
	}
	processorKeys := make([]string, 0, len(resp.ProcessorRequests))
	processorStatusCounts := make(map[string]int)
	processorRuntimeRunCount := 0
	for _, req := range resp.ProcessorRequests {
		if strings.TrimSpace(req.ProcessorKey) != "" {
			processorKeys = append(processorKeys, req.ProcessorKey)
		}
		processorStatusCounts[normalizedSourceMaxxRequestStatus(req.Status)]++
		if strings.TrimSpace(req.RuntimeRunID) != "" {
			processorRuntimeRunCount++
		}
	}
	reconcilerScopes := make([]string, 0, len(resp.ReconcilerRequests))
	reconcilerStatusCounts := make(map[string]int)
	reconcilerRuntimeRunCount := 0
	for _, req := range resp.ReconcilerRequests {
		if strings.TrimSpace(req.Scope) != "" {
			reconcilerScopes = append(reconcilerScopes, req.Scope)
		}
		reconcilerStatusCounts[normalizedSourceMaxxRequestStatus(req.Status)]++
		if strings.TrimSpace(req.RuntimeRunID) != "" {
			reconcilerRuntimeRunCount++
		}
	}
	status := "ok"
	if strings.TrimSpace(resp.Cycle.Status) != "" && resp.Cycle.Status != "completed" {
		status = resp.Cycle.Status
	}
	return globalWireSourceMaxxStatusResponse{
		Status:                    status,
		Source:                    firstNonEmptyString(resp.Provider, sourceapi.ProviderName),
		CycleID:                   resp.Cycle.CycleID,
		CycleStatus:               resp.Cycle.Status,
		StartedAt:                 resp.Cycle.StartedAt,
		EndedAt:                   resp.Cycle.EndedAt,
		ItemCount:                 resp.Cycle.ItemCount,
		FetchCount:                resp.Cycle.FetchCount,
		ProcessorRequestCount:     len(resp.ProcessorRequests),
		ReconcilerRequestCount:    len(resp.ReconcilerRequests),
		ProcessorStatusCounts:     emptyMapToNil(processorStatusCounts),
		ReconcilerStatusCounts:    emptyMapToNil(reconcilerStatusCounts),
		ProcessorRuntimeRunCount:  processorRuntimeRunCount,
		ReconcilerRuntimeRunCount: reconcilerRuntimeRunCount,
		ProcessorKeys:             processorKeys,
		ReconcilerScopes:          reconcilerScopes,
		Topology:                  resp.Metadata.Topology,
		AuthorityRule:             resp.Metadata.AuthorityRule,
		SourceServiceInternalOnly: true,
	}
}

func normalizedSourceMaxxRequestStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "unknown"
	}
	return status
}

func emptyMapToNil(m map[string]int) map[string]int {
	if len(m) == 0 {
		return nil
	}
	return m
}

func (h *APIHandler) addSourceMaxxRuntimeEvidence(ctx context.Context, resp *sourceapi.SourceMaxxResponse, out *globalWireSourceMaxxStatusResponse) {
	if h == nil || h.rt == nil || h.rt.Store() == nil || resp == nil || out == nil {
		return
	}
	runtimeClient := sourceMaxxRuntimeEvidenceClientFromEnv()
	childRunsByOwner := make(map[string][]types.RunRecord)
	for _, req := range resp.ProcessorRequests {
		rec, ok := h.sourceMaxxRuntimeRun(ctx, runtimeClient, req.RuntimeRunID)
		if !ok {
			if strings.TrimSpace(req.RuntimeRunID) != "" {
				out.ProcessorUnresolvedRunCount++
			}
			continue
		}
		out.ProcessorResolvedRunCount++
		if out.ProcessorRunStateCounts == nil {
			out.ProcessorRunStateCounts = make(map[string]int)
		}
		out.ProcessorRunStateCounts[normalizedRunState(rec.State)]++
		out.ProcessorUpdateCount += h.sourceMaxxWorkerUpdateCount(ctx, rec)
		addSourceMaxxChildProfileCounts(ctx, h.rt, rec, childRunsByOwner, &out.ProcessorChildProfileCounts)
	}
	for _, req := range resp.ReconcilerRequests {
		rec, ok := h.sourceMaxxRuntimeRun(ctx, runtimeClient, req.RuntimeRunID)
		if !ok {
			if strings.TrimSpace(req.RuntimeRunID) != "" {
				out.ReconcilerUnresolvedRunCount++
			}
			continue
		}
		out.ReconcilerResolvedRunCount++
		if out.ReconcilerRunStateCounts == nil {
			out.ReconcilerRunStateCounts = make(map[string]int)
		}
		out.ReconcilerRunStateCounts[normalizedRunState(rec.State)]++
		out.ReconcilerUpdateCount += h.sourceMaxxWorkerUpdateCount(ctx, rec)
		addSourceMaxxChildProfileCounts(ctx, h.rt, rec, childRunsByOwner, &out.ReconcilerChildProfileCounts)
	}
	out.ProcessorRunStateCounts = emptyMapToNil(out.ProcessorRunStateCounts)
	out.ReconcilerRunStateCounts = emptyMapToNil(out.ReconcilerRunStateCounts)
	out.ProcessorChildProfileCounts = emptyMapToNil(out.ProcessorChildProfileCounts)
	out.ReconcilerChildProfileCounts = emptyMapToNil(out.ReconcilerChildProfileCounts)
}

func sourceMaxxRuntimeEvidenceClientFromEnv() *sourceMaxxRuntimeEvidenceClient {
	baseURL := strings.TrimRight(strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_BASE_URL", "SOURCECYCLED_RUNTIME_BASE_URL")), "/")
	if baseURL == "" {
		return nil
	}
	ownerID := strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "global-wire-platform"
	}
	return &sourceMaxxRuntimeEvidenceClient{
		baseURL: baseURL,
		ownerID: ownerID,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (h *APIHandler) sourceMaxxRuntimeRun(ctx context.Context, remote *sourceMaxxRuntimeEvidenceClient, runID string) (sourceMaxxResolvedRun, bool) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return sourceMaxxResolvedRun{}, false
	}
	if remote != nil {
		if rec, ok := remote.getRun(ctx, runID); ok && isSourceMaxxRuntimeProfile(rec.AgentProfile) {
			resolved := sourceMaxxResolvedRun{
				RunID:        rec.RunID,
				AgentProfile: rec.AgentProfile,
				State:        rec.State,
			}
			resolved.Events, _ = remote.getRunEvents(ctx, runID)
			return resolved, true
		}
	}
	rec, err := h.rt.Store().GetRun(ctx, runID)
	if err != nil {
		return sourceMaxxResolvedRun{}, false
	}
	if !isSourceMaxxRuntimeProfile(rec.AgentProfile) {
		return sourceMaxxResolvedRun{}, false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "sourcecycled" {
		return sourceMaxxResolvedRun{}, false
	}
	return sourceMaxxResolvedRun{
		RunID:        rec.RunID,
		AgentProfile: rec.AgentProfile,
		State:        rec.State,
		LocalRecord:  &rec,
	}, true
}

func isSourceMaxxRuntimeProfile(profile string) bool {
	return profile == AgentProfileProcessor || profile == AgentProfileReconciler
}

func (c *sourceMaxxRuntimeEvidenceClient) getRun(ctx context.Context, runID string) (runStatusResponse, bool) {
	if c == nil || strings.TrimSpace(c.baseURL) == "" || strings.TrimSpace(c.ownerID) == "" {
		return runStatusResponse{}, false
	}
	endpoint, err := url.JoinPath(c.baseURL, "/internal/runtime/runs", runID)
	if err != nil {
		return runStatusResponse{}, false
	}
	values := url.Values{}
	values.Set("owner_id", c.ownerID)
	endpoint += "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return runStatusResponse{}, false
	}
	req.Header.Set("X-Internal-Caller", "true")
	res, err := c.client.Do(req)
	if err != nil {
		return runStatusResponse{}, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return runStatusResponse{}, false
	}
	var out runStatusResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return runStatusResponse{}, false
	}
	return out, true
}

func (c *sourceMaxxRuntimeEvidenceClient) getRunEvents(ctx context.Context, runID string) ([]types.EventRecord, bool) {
	if c == nil || strings.TrimSpace(c.baseURL) == "" || strings.TrimSpace(c.ownerID) == "" {
		return nil, false
	}
	endpoint, err := url.JoinPath(c.baseURL, "/internal/runtime/runs", runID, "events")
	if err != nil {
		return nil, false
	}
	values := url.Values{}
	values.Set("owner_id", c.ownerID)
	values.Set("limit", "1000")
	endpoint += "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("X-Internal-Caller", "true")
	res, err := c.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, false
	}
	var out eventListResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, false
	}
	return out.Events, true
}

func (h *APIHandler) sourceMaxxWorkerUpdateCount(ctx context.Context, rec sourceMaxxResolvedRun) int {
	if len(rec.Events) > 0 {
		return sourceMaxxToolInvocationCount(rec.Events, "submit_coagent_update")
	}
	if rec.LocalRecord == nil {
		return 0
	}
	trajectoryID := trajectoryIDForRun(rec.LocalRecord)
	if strings.TrimSpace(trajectoryID) == "" {
		return 0
	}
	updates, err := h.rt.Store().ListWorkerUpdatesByTrajectory(ctx, rec.LocalRecord.OwnerID, trajectoryID, 200)
	if err != nil {
		return 0
	}
	return len(updates)
}

func addSourceMaxxChildProfileCounts(ctx context.Context, rt *Runtime, rec sourceMaxxResolvedRun, byOwner map[string][]types.RunRecord, out *map[string]int) {
	if len(rec.Events) > 0 {
		addSourceMaxxChildProfileCountsFromEvents(rec.Events, out)
		return
	}
	if rec.LocalRecord == nil || rt == nil || rt.Store() == nil || strings.TrimSpace(rec.LocalRecord.OwnerID) == "" || strings.TrimSpace(rec.RunID) == "" {
		return
	}
	runs, ok := byOwner[rec.LocalRecord.OwnerID]
	if !ok {
		var err error
		runs, err = rt.Store().ListRunsByOwner(ctx, rec.LocalRecord.OwnerID, 1000)
		if err != nil {
			byOwner[rec.LocalRecord.OwnerID] = nil
			return
		}
		byOwner[rec.LocalRecord.OwnerID] = runs
	}
	for _, child := range runs {
		if strings.TrimSpace(child.ParentRunID) != rec.RunID {
			continue
		}
		profile := strings.TrimSpace(child.AgentProfile)
		if profile == "" {
			profile = "unknown"
		}
		if *out == nil {
			*out = make(map[string]int)
		}
		(*out)[profile]++
	}
}

func sourceMaxxToolInvocationCount(events []types.EventRecord, toolName string) int {
	count := 0
	for _, ev := range events {
		if ev.Kind != types.EventToolInvoked {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		if strings.TrimSpace(stringValue(payload["tool"])) == toolName {
			count++
		}
	}
	return count
}

func addSourceMaxxChildProfileCountsFromEvents(events []types.EventRecord, out *map[string]int) {
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		if strings.TrimSpace(stringValue(payload["tool"])) != "spawn_agent" {
			continue
		}
		output := strings.TrimSpace(stringValue(payload["output"]))
		if output == "" {
			continue
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			continue
		}
		profile := strings.TrimSpace(stringValue(result["profile"]))
		if profile == "" {
			profile = strings.TrimSpace(stringValue(result["role"]))
		}
		if profile == "" {
			profile = "unknown"
		}
		if *out == nil {
			*out = make(map[string]int)
		}
		(*out)[profile]++
	}
}

func normalizedRunState(state types.RunState) string {
	raw := strings.TrimSpace(string(state))
	if raw == "" {
		return "unknown"
	}
	return raw
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
	if req.MaxResults <= 0 {
		req.MaxResults = 3
	}
	if req.MaxResults > 10 {
		req.MaxResults = 10
	}
	if req.StoryID == "" {
		h.handleGlobalWireSourceNativeRefresh(w, r, ownerID, req)
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
	claimRecord, sourceReviewSignal, researchTask, extractionArtifact, err := h.createGlobalWireClaimResearchArtifacts(r, ownerID, story, item, classification, run, contribution, decision, candidate)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create claim research artifacts"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireSourceRefreshResponse{
		Status:             run.Status,
		Source:             run.Provider,
		Query:              run.Query,
		Message:            run.Message,
		RefreshRun:         run,
		ContentItem:        &item,
		Contribution:       &contribution,
		Decision:           &decision,
		Candidate:          &candidate,
		ClaimRecord:        &claimRecord,
		SourceReviewSignal: &sourceReviewSignal,
		ResearchTask:       &researchTask,
		ExtractionArtifact: &extractionArtifact,
	})
}

func (h *APIHandler) handleGlobalWireSourceNativeRefresh(w http.ResponseWriter, r *http.Request, ownerID string, req globalWireSourceRefreshRequest) {
	if req.Query == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "query is required when story_id is omitted"})
		return
	}
	sourceClient := newSourceSearchClientFromEnv()
	if sourceClient == nil {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			Query:    req.Query,
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
	resp, err := sourceClient.SearchSources(r.Context(), req.Query, req.MaxResults)
	if err != nil {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			Query:    req.Query,
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
	query := firstNonEmptyString(resp.Query, req.Query)
	if len(resp.Results) == 0 {
		run, runErr := h.createGlobalWireSourceRefreshRun(r, types.GlobalWireSourceRefreshRun{
			ID:       "global-wire-source-refresh-" + uuid.NewString(),
			OwnerID:  ownerID,
			Query:    query,
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
	story := globalWireSourceNativeStory(ownerID, item)
	classification := globalWireSourceNativeClassification(item)
	contribution, decision, candidate, err := h.createGlobalWireSourceRefreshArtifacts(r, ownerID, story, item, classification)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create source refresh artifacts"})
		return
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
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source refresh run"})
		return
	}
	claimRecord, sourceReviewSignal, researchTask, extractionArtifact, err := h.createGlobalWireClaimResearchArtifacts(r, ownerID, story, item, classification, run, contribution, decision, candidate)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create claim research artifacts"})
		return
	}
	reviews, err := h.createGlobalWireSourceNativeProjectionReviews(r, ownerID, story, item, candidate)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create source-native projection review"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireSourceRefreshResponse{
		Status:             run.Status,
		Source:             run.Provider,
		Query:              run.Query,
		Message:            run.Message,
		RefreshRun:         run,
		ContentItem:        &item,
		Contribution:       &contribution,
		Decision:           &decision,
		Candidate:          &candidate,
		ProjectionReviews:  reviews,
		ClaimRecord:        &claimRecord,
		SourceReviewSignal: &sourceReviewSignal,
		ResearchTask:       &researchTask,
		ExtractionArtifact: &extractionArtifact,
	})
}

func globalWireSourceNativeClassification(item types.ContentItem) globalWireSourceUpdateClassification {
	return globalWireSourceUpdateClassification{
		UpdateClassification: "source-native-article-candidate",
		CandidateKind:        "source-native-article-candidate",
		SourceTier:           "lead",
		EdgeKind:             "source-native",
		StoryGraphAction:     "source-native-review",
		ProjectionAction:     "projection-review-required",
		Status:               "candidate-review",
		Message:              "Source Service evidence imported as a source-native Community Wire article candidate; no seeded StoryGraph node is required or mutated.",
		Rationale:            "Review the imported source as article evidence and create owner-approved VText before publication.",
	}
}

func globalWireSourceNativeStory(ownerID string, item types.ContentItem) types.GlobalWireStory {
	storyID := globalWireSourceNativeStoryID(ownerID, item)
	title := firstNonEmptyString(strings.TrimSpace(item.Title), strings.TrimSpace(item.CanonicalURL), strings.TrimSpace(item.SourceURL), "Community Wire source candidate")
	body := strings.TrimSpace(item.TextContent)
	dek := truncateRunes(body, 220)
	if dek == "" {
		dek = "Source-native Community Wire article candidate imported from Source Service evidence."
	}
	source := types.GlobalWireSourceItem{
		ID:           firstNonEmptyString(item.ContentID, storyID),
		ContentID:    item.ContentID,
		Title:        title,
		Standing:     "source-service evidence",
		Role:         "lead",
		CanonicalURL: firstNonEmptyString(item.CanonicalURL, item.SourceURL),
	}
	projection := body
	if projection == "" {
		projection = dek
	}
	return types.GlobalWireStory{
		ID:                  storyID,
		OwnerID:             ownerID,
		Headline:            title,
		Dek:                 dek,
		Freshness:           "source-native",
		Prominence:          1,
		Tension:             "Owner review required before publication.",
		ChangeState:         "source-native-review",
		NodeTone:            "evidence-led",
		Manifest:            types.GlobalWireSourceManifest{Lead: []types.GlobalWireSourceItem{source}},
		Claims:              []string{dek},
		Projections:         map[string]string{"wire-style": projection},
		ProjectionVTextDocs: map[string]string{},
		StyleSources: []types.GlobalWireStyleSource{{
			ID:      "wire-style",
			Title:   "Community Wire source-native article",
			Label:   "Wire",
			Summary: "Default source-native article projection for Community Wire.",
		}},
		SourceState: "source-native-content-item",
	}
}

func globalWireSourceNativeStoryID(ownerID string, item types.ContentItem) string {
	key := firstNonEmptyString(item.ContentID, item.ContentHash, item.CanonicalURL, item.SourceURL, item.Title)
	return "source-native-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+key)).String()
}

func isGlobalWireSourceNativeStoryID(storyID string) bool {
	return strings.HasPrefix(strings.TrimSpace(storyID), "source-native-")
}

func (h *APIHandler) createGlobalWireSourceNativeProjectionReviews(r *http.Request, ownerID string, story types.GlobalWireStory, item types.ContentItem, candidate types.GlobalWireGraphUpdateCandidate) ([]types.GlobalWireProjectionReview, error) {
	review, err := h.rt.Store().CreateGlobalWireProjectionReview(r.Context(), types.GlobalWireProjectionReview{
		ID:               "global-wire-projection-review-" + uuid.NewString(),
		OwnerID:          ownerID,
		StoryID:          story.ID,
		CandidateID:      candidate.ID,
		SourceContentID:  item.ContentID,
		StyleID:          "wire-style",
		StyleTitle:       "Community Wire source-native article",
		ProjectionAction: "projection-review-required",
		Status:           "projection-review-required",
		Rationale:        "Source-native Community Wire article candidate requires owner-approved VText before publication.",
	})
	if err != nil {
		return nil, err
	}
	return []types.GlobalWireProjectionReview{review}, nil
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
		schedulerRuns, err := h.rt.Store().ListGlobalWireSourceSchedulerRuns(r.Context(), ownerID, 20)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list source scheduler runs"})
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
			SchedulerRuns:   schedulerRuns,
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
		if req.SchedulerMode {
			req.SchedulerRunID = "global-wire-source-scheduler-run-" + uuid.NewString()
			if strings.TrimSpace(req.Trigger) == "" {
				req.Trigger = "scheduled-source-standing-cycle"
			}
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
		if req.SchedulerMode {
			schedulerRun, err := h.createGlobalWireSourceSchedulerRun(r, ownerID, req, resp)
			if err != nil {
				writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record source scheduler run"})
				return
			}
			resp.SchedulerRun = &schedulerRun
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
		sourceReviewSignals, err := h.rt.Store().ListGlobalWireSourceReviewSignals(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list source review signals"})
			return
		}
		researchTasks, err := h.rt.Store().ListGlobalWireResearchTasks(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research tasks"})
			return
		}
		extractionArtifacts, err := h.rt.Store().ListGlobalWireExtractionArtifacts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list extraction artifacts"})
			return
		}
		researchEvidence, err := h.rt.Store().ListGlobalWireResearchTaskEvidence(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research task evidence"})
			return
		}
		researchDecisions, err := h.rt.Store().ListGlobalWireResearchEvidenceDecisions(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research evidence decisions"})
			return
		}
		publicationUpdates, err := h.rt.Store().ListGlobalWirePublicationUpdates(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication updates"})
			return
		}
		publicationArtifacts, err := h.rt.Store().ListGlobalWirePublicationArtifacts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication artifacts"})
			return
		}
		publicationDeliveries, err := h.rt.Store().ListGlobalWirePublicationDeliveries(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication deliveries"})
			return
		}
		autoradioScripts, err := h.rt.Store().ListGlobalWireAutoradioScripts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio scripts"})
			return
		}
		autoradioEpisodes, err := h.rt.Store().ListGlobalWireAutoradioEpisodes(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio episodes"})
			return
		}
		deliveryExports, err := h.rt.Store().ListGlobalWirePublicationDeliveryExports(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list delivery exports"})
			return
		}
		publicLinks, err := h.rt.Store().ListGlobalWirePublicationPublicLinks(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list public links"})
			return
		}
		for i := range publicLinks {
			hydrateGlobalWirePublicLinkDerivedFields(&publicLinks[i])
		}
		newsletterSubscribers, err := h.rt.Store().ListGlobalWireNewsletterSubscribers(r.Context(), ownerID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter subscribers"})
			return
		}
		newsletterIssues, err := h.rt.Store().ListGlobalWireNewsletterIssues(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter issues"})
			return
		}
		newsletterDeliveries, err := h.rt.Store().ListGlobalWireNewsletterDeliveries(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter deliveries"})
			return
		}
		newsletterReceipts, err := h.rt.Store().ListGlobalWireNewsletterProviderReceipts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter provider receipts"})
			return
		}
		projectionReviews, err := h.rt.Store().ListGlobalWireProjectionReviews(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list projection reviews"})
			return
		}
		dossiers, err := h.globalWireSourceDossiers(r, ownerID, storyID, contributions, refreshes, claimRecords, sourceReviewSignals, researchTasks, extractionArtifacts, researchEvidence, researchDecisions, candidates, publicationUpdates, publicationArtifacts, publicationDeliveries, autoradioScripts, autoradioEpisodes, deliveryExports, publicLinks, newsletterIssues, newsletterDeliveries, newsletterReceipts)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to build source dossiers"})
			return
		}
		writeAPIJSON(w, http.StatusOK, globalWireReconciliationResponse{
			Contributions:         contributions,
			SourceItems:           h.globalWireContributionSourceItems(r, ownerID, contributions),
			SourceDossiers:        dossiers,
			Decisions:             decisions,
			Candidates:            candidates,
			Promotions:            promotions,
			Refreshes:             refreshes,
			ClaimRecords:          claimRecords,
			SourceReviewSignals:   sourceReviewSignals,
			ResearchTasks:         researchTasks,
			ExtractionArtifacts:   extractionArtifacts,
			ResearchEvidence:      researchEvidence,
			ResearchDecisions:     researchDecisions,
			PublicationUpdates:    publicationUpdates,
			PublicationArtifacts:  publicationArtifacts,
			PublicationDeliveries: publicationDeliveries,
			AutoradioScripts:      autoradioScripts,
			AutoradioEpisodes:     autoradioEpisodes,
			DeliveryExports:       deliveryExports,
			PublicLinks:           publicLinks,
			NewsletterSubscribers: newsletterSubscribers,
			NewsletterIssues:      newsletterIssues,
			NewsletterDeliveries:  newsletterDeliveries,
			NewsletterReceipts:    newsletterReceipts,
			ProjectionReviews:     projectionReviews,
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

// HandleGlobalWireSourceDossiers exposes a deterministic, non-mutating dossier
// projection over existing reconciliation records.
func (h *APIHandler) HandleGlobalWireSourceDossiers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
	contributions, err := h.rt.Store().ListGlobalWireContributions(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list contributions"})
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
	sourceReviewSignals, err := h.rt.Store().ListGlobalWireSourceReviewSignals(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list source review signals"})
		return
	}
	researchTasks, err := h.rt.Store().ListGlobalWireResearchTasks(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research tasks"})
		return
	}
	extractionArtifacts, err := h.rt.Store().ListGlobalWireExtractionArtifacts(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list extraction artifacts"})
		return
	}
	researchEvidence, err := h.rt.Store().ListGlobalWireResearchTaskEvidence(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research evidence"})
		return
	}
	researchDecisions, err := h.rt.Store().ListGlobalWireResearchEvidenceDecisions(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list research decisions"})
		return
	}
	candidates, err := h.rt.Store().ListGlobalWireGraphUpdateCandidates(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list graph candidates"})
		return
	}
	publicationUpdates, err := h.rt.Store().ListGlobalWirePublicationUpdates(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication updates"})
		return
	}
	publicationArtifacts, err := h.rt.Store().ListGlobalWirePublicationArtifacts(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication artifacts"})
		return
	}
	publicationDeliveries, err := h.rt.Store().ListGlobalWirePublicationDeliveries(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication deliveries"})
		return
	}
	autoradioScripts, err := h.rt.Store().ListGlobalWireAutoradioScripts(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio scripts"})
		return
	}
	autoradioEpisodes, err := h.rt.Store().ListGlobalWireAutoradioEpisodes(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio episodes"})
		return
	}
	deliveryExports, err := h.rt.Store().ListGlobalWirePublicationDeliveryExports(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list delivery exports"})
		return
	}
	publicLinks, err := h.rt.Store().ListGlobalWirePublicationPublicLinks(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list public links"})
		return
	}
	newsletterIssues, err := h.rt.Store().ListGlobalWireNewsletterIssues(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter issues"})
		return
	}
	newsletterDeliveries, err := h.rt.Store().ListGlobalWireNewsletterDeliveries(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter deliveries"})
		return
	}
	newsletterReceipts, err := h.rt.Store().ListGlobalWireNewsletterProviderReceipts(r.Context(), ownerID, storyID, 100)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter provider receipts"})
		return
	}
	dossiers, err := h.globalWireSourceDossiers(r, ownerID, storyID, contributions, refreshes, claimRecords, sourceReviewSignals, researchTasks, extractionArtifacts, researchEvidence, researchDecisions, candidates, publicationUpdates, publicationArtifacts, publicationDeliveries, autoradioScripts, autoradioEpisodes, deliveryExports, publicLinks, newsletterIssues, newsletterDeliveries, newsletterReceipts)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to build source dossiers"})
		return
	}
	writeAPIJSON(w, http.StatusOK, globalWireSourceDossierResponse{
		Dossiers: dossiers,
		Status:   "ready",
		Source:   "derived-reconciliation-dossier",
	})
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

// HandleGlobalWireResearchEvidence records reviewer handoff decisions over
// research evidence without applying platform StoryGraph mutations.
func (h *APIHandler) HandleGlobalWireResearchEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req globalWireResearchEvidenceDecisionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid research evidence request"})
		return
	}
	req.EvidenceID = strings.TrimSpace(req.EvidenceID)
	req.Decision = normalizeGlobalWireResearchEvidenceDecision(req.Decision)
	req.Note = strings.TrimSpace(req.Note)
	if req.EvidenceID == "" || req.Decision == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "evidence_id and decision are required"})
		return
	}
	evidence, err := h.rt.Store().GetGlobalWireResearchTaskEvidence(r.Context(), ownerID, req.EvidenceID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "research evidence not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load research evidence"})
		return
	}
	task, err := h.rt.Store().GetGlobalWireResearchTask(r.Context(), ownerID, evidence.TaskID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "research task not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load research task"})
		return
	}
	if req.Decision == "accepted-for-review" && evidence.Status != "completed" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "only completed research evidence can be accepted for review"})
		return
	}
	resultState := globalWireResearchEvidenceResultState(req.Decision)
	var candidate *types.GlobalWireGraphUpdateCandidate
	if strings.TrimSpace(task.CandidateID) != "" {
		status := "research-evidence-accepted"
		if req.Decision == "blocked" {
			status = "research-evidence-blocked"
		}
		updatedCandidate, err := h.rt.Store().UpdateGlobalWireGraphUpdateCandidateStatus(r.Context(), ownerID, task.CandidateID, status)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "graph candidate not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update graph candidate review state"})
			return
		}
		candidate = &updatedCandidate
	}
	decision, err := h.rt.Store().CreateGlobalWireResearchEvidenceDecision(r.Context(), types.GlobalWireResearchEvidenceDecision{
		ID:              "global-wire-research-decision-" + uuid.NewString(),
		OwnerID:         ownerID,
		EvidenceID:      evidence.ID,
		TaskID:          task.ID,
		StoryID:         task.StoryID,
		ClaimID:         task.ClaimID,
		CandidateID:     task.CandidateID,
		SourceContentID: evidence.SourceContentID,
		Decision:        req.Decision,
		Note:            req.Note,
		ResultState:     resultState,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create research evidence decision"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, globalWireResearchEvidenceDecisionResponse{
		Decision:  decision,
		Task:      task,
		Evidence:  evidence,
		Candidate: candidate,
	})
}

func normalizeGlobalWireResearchEvidenceDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "accept", "accepted", "accepted-for-review":
		return "accepted-for-review"
	case "block", "blocked":
		return "blocked"
	default:
		return ""
	}
}

func globalWireResearchEvidenceResultState(decision string) string {
	if decision == "blocked" {
		return "research-evidence-blocked"
	}
	return "ready-for-platform-review"
}

// HandleGlobalWirePublicationUpdates packages review-ready evidence into an
// owner-visible update-feed artifact without publishing or mutating stories.
func (h *APIHandler) HandleGlobalWirePublicationUpdates(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		updates, err := h.rt.Store().ListGlobalWirePublicationUpdates(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication updates"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"publication_updates": updates,
		})
	case http.MethodPost:
		var req globalWirePublicationUpdateRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid publication update request"})
			return
		}
		req.ResearchDecisionID = strings.TrimSpace(req.ResearchDecisionID)
		req.Summary = strings.TrimSpace(req.Summary)
		if req.ResearchDecisionID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "research_decision_id is required"})
			return
		}
		update, decision, candidate, reviews, sourceItem, err := h.createGlobalWirePublicationUpdate(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication update source artifact not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create publication update"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWirePublicationUpdateResponse{
			Update:            update,
			ResearchDecision:  decision,
			Candidate:         candidate,
			ProjectionReviews: reviews,
			SourceItem:        sourceItem,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationArtifacts materializes a citeable publication/feed
// artifact from an owner-visible publication update package. It does not
// publish publicly and does not mutate the platform StoryGraph.
func (h *APIHandler) HandleGlobalWirePublicationArtifacts(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		artifacts, err := h.rt.Store().ListGlobalWirePublicationArtifacts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication artifacts"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"publication_artifacts": artifacts,
		})
	case http.MethodPost:
		var req globalWirePublicationArtifactRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid publication artifact request"})
			return
		}
		req.UpdateID = strings.TrimSpace(req.UpdateID)
		req.Channel = strings.TrimSpace(req.Channel)
		req.Title = strings.TrimSpace(req.Title)
		if req.UpdateID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "update_id is required"})
			return
		}
		artifact, update, story, reviews, sourceItem, err := h.createGlobalWirePublicationArtifact(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication artifact source package not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create publication artifact"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWirePublicationArtifactResponse{
			Artifact:          artifact,
			Update:            update,
			Story:             story,
			ProjectionReviews: reviews,
			SourceItem:        sourceItem,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationFeed composes review-ready publication artifacts
// into an owner-scoped feed surface without publishing publicly or mutating the
// platform StoryGraph.
func (h *APIHandler) HandleGlobalWirePublicationFeed(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
	channel := strings.TrimSpace(r.URL.Query().Get("channel"))
	limit := parseGlobalWireFeedLimit(r.URL.Query().Get("limit"))
	artifacts, err := h.rt.Store().ListGlobalWirePublicationArtifacts(r.Context(), ownerID, storyID, limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication feed"})
		return
	}
	items, err := h.globalWirePublicationFeedItems(r, ownerID, artifacts, channel)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to compose publication feed"})
		return
	}
	status := "empty"
	if len(items) > 0 {
		status = "ready"
	}
	writeAPIJSON(w, http.StatusOK, globalWirePublicationFeedResponse{
		FeedItems: items,
		Channel:   firstNonEmptyString(channel, "all"),
		Status:    status,
	})
}

// HandleGlobalWirePublicationArtifactReviews records owner review state for a
// publication artifact. It is not public delivery and does not mutate the
// platform StoryGraph.
func (h *APIHandler) HandleGlobalWirePublicationArtifactReviews(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req globalWirePublicationArtifactReviewRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid publication artifact review request"})
		return
	}
	req.ArtifactID = strings.TrimSpace(req.ArtifactID)
	status := normalizeGlobalWirePublicationArtifactReviewDecision(req.Decision)
	if req.ArtifactID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "artifact_id is required"})
		return
	}
	if status == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "decision must be approve or reject"})
		return
	}
	existing, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, req.ArtifactID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication artifact not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load publication artifact"})
		return
	}
	if existing.Status != "publication-review-ready" && existing.Status != status {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "publication artifact review state is already final"})
		return
	}
	artifact, err := h.rt.Store().UpdateGlobalWirePublicationArtifactStatus(r.Context(), ownerID, req.ArtifactID, status)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication artifact not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to review publication artifact"})
		return
	}
	var edition *globalWireEditionResponse
	if artifact.Status == "publication-approved" {
		edition, err = h.publishGlobalWireArtifactToCommunityEdition(r, ownerID, artifact)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update Community Wire edition"})
			return
		}
	}
	writeAPIJSON(w, http.StatusCreated, globalWirePublicationArtifactReviewResponse{
		Artifact: artifact,
		Status:   artifact.Status,
		Decision: req.Decision,
		Edition:  edition,
	})
}

// HandleGlobalWirePublicationDeliveries records owner-scoped delivery/
// availability evidence for approved publication artifacts.
func (h *APIHandler) HandleGlobalWirePublicationDeliveries(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		deliveries, err := h.rt.Store().ListGlobalWirePublicationDeliveries(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list publication deliveries"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"publication_deliveries": deliveries,
		})
	case http.MethodPost:
		var req globalWirePublicationDeliveryRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid publication delivery request"})
			return
		}
		req.ArtifactID = strings.TrimSpace(req.ArtifactID)
		req.Channel = strings.TrimSpace(req.Channel)
		if req.ArtifactID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "artifact_id is required"})
			return
		}
		delivery, artifact, story, err := h.createGlobalWirePublicationDelivery(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "approved publication artifact not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create publication delivery"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWirePublicationDeliveryResponse{
			Delivery: delivery,
			Artifact: artifact,
			Story:    story,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationDeliveryDetail returns an owner-scoped composed
// delivery publication object with artifact/story/source provenance.
func (h *APIHandler) HandleGlobalWirePublicationDeliveryDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	deliveryID := strings.TrimPrefix(r.URL.Path, "/api/global-wire/publication-deliveries/")
	deliveryID = strings.Trim(strings.TrimSpace(deliveryID), "/")
	if deliveryID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication delivery not found"})
		return
	}
	delivery, artifact, story, sourceItem, err := h.globalWirePublicationDeliveryDetail(r, ownerID, deliveryID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication delivery not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load publication delivery"})
		return
	}
	writeAPIJSON(w, http.StatusOK, globalWirePublicationDeliveryDetailResponse{
		Delivery:   delivery,
		Artifact:   artifact,
		Story:      story,
		SourceItem: sourceItem,
	})
}

// HandleGlobalWireAutoradioScripts materializes durable script artifacts over
// approved publication artifacts. It does not generate audio or mutate stories.
func (h *APIHandler) HandleGlobalWireAutoradioScripts(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		scripts, err := h.rt.Store().ListGlobalWireAutoradioScripts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio scripts"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"autoradio_scripts": scripts,
		})
	case http.MethodPost:
		var req globalWireAutoradioScriptRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid autoradio script request"})
			return
		}
		req.ArtifactID = strings.TrimSpace(req.ArtifactID)
		if req.ArtifactID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "artifact_id is required"})
			return
		}
		script, artifact, story, sourceItem, err := h.createGlobalWireAutoradioScript(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "approved publication artifact not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create autoradio script"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWireAutoradioScriptResponse{
			Script:     script,
			Artifact:   artifact,
			Story:      story,
			SourceItem: sourceItem,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWireAutoradioEpisodes materializes durable browser-speech
// playback packages over approved Autoradio scripts.
func (h *APIHandler) HandleGlobalWireAutoradioEpisodes(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		episodes, err := h.rt.Store().ListGlobalWireAutoradioEpisodes(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list autoradio episodes"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"autoradio_episodes": episodes,
		})
	case http.MethodPost:
		var req globalWireAutoradioEpisodeRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid autoradio episode request"})
			return
		}
		req.ScriptID = strings.TrimSpace(req.ScriptID)
		if req.ScriptID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "script_id is required"})
			return
		}
		episode, script, artifact, story, sourceItem, err := h.createGlobalWireAutoradioEpisode(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "autoradio script not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create autoradio episode"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWireAutoradioEpisodeResponse{
			Episode:    episode,
			Script:     script,
			Artifact:   artifact,
			Story:      story,
			SourceItem: sourceItem,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationDeliveryExports materializes owner-scoped export
// artifacts over delivered publication records.
func (h *APIHandler) HandleGlobalWirePublicationDeliveryExports(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		exports, err := h.rt.Store().ListGlobalWirePublicationDeliveryExports(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list delivery exports"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"delivery_exports": exports,
		})
	case http.MethodPost:
		var req globalWirePublicationDeliveryExportRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid delivery export request"})
			return
		}
		req.DeliveryID = strings.TrimSpace(req.DeliveryID)
		req.Format = strings.TrimSpace(req.Format)
		if req.DeliveryID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "delivery_id is required"})
			return
		}
		export, delivery, artifact, story, script, sourceItem, err := h.createGlobalWirePublicationDeliveryExport(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "publication delivery not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create delivery export"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWirePublicationDeliveryExportResponse{
			Export:     export,
			Delivery:   delivery,
			Artifact:   artifact,
			Story:      story,
			Script:     script,
			SourceItem: sourceItem,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationPublicLinks lets an owner create/list unlisted
// read-only public links for delivery exports.
func (h *APIHandler) HandleGlobalWirePublicationPublicLinks(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		links, err := h.rt.Store().ListGlobalWirePublicationPublicLinks(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list public links"})
			return
		}
		for i := range links {
			hydrateGlobalWirePublicLinkDerivedFields(&links[i])
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"public_links": links,
		})
	case http.MethodPost:
		var req globalWirePublicationPublicLinkRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid public link request"})
			return
		}
		req.ExportID = strings.TrimSpace(req.ExportID)
		if req.ExportID == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "export_id is required"})
			return
		}
		link, export, err := h.createGlobalWirePublicationPublicLink(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "delivery export not found"})
				return
			}
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create public link"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWirePublicationPublicLinkResponse{
			PublicLink: link,
			Export:     export,
		})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWirePublicationPublicLinkDetail returns a single unlisted public
// publication export by token. It intentionally does not expose owner queues.
func (h *APIHandler) HandleGlobalWirePublicationPublicLinkDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/api/global-wire/publication-public-links/")
	token = strings.Trim(strings.TrimSpace(token), "/")
	rssRequested := false
	if strings.HasSuffix(token, "/rss") {
		rssRequested = true
		token = strings.TrimSuffix(token, "/rss")
	} else if strings.HasSuffix(token, ".rss") {
		rssRequested = true
		token = strings.TrimSuffix(token, ".rss")
	}
	if token == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "public link not found"})
		return
	}
	link, err := h.rt.Store().GetGlobalWirePublicationPublicLinkByToken(r.Context(), token)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "public link not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load public link"})
		return
	}
	link.OwnerID = ""
	link.Token = token
	hydrateGlobalWirePublicLinkDerivedFields(&link)
	if rssRequested {
		writeGlobalWirePublicLinkRSS(w, r, link)
		return
	}
	writeAPIJSON(w, http.StatusOK, globalWirePublicationPublicLinkResponse{
		PublicLink: link,
	})
}

// HandleGlobalWireNewsletterSubscribers records owner-scoped newsletter
// destinations. It does not expose subscribers publicly or send email.
func (h *APIHandler) HandleGlobalWireNewsletterSubscribers(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		subscribers, err := h.rt.Store().ListGlobalWireNewsletterSubscribers(r.Context(), ownerID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter subscribers"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"newsletter_subscribers": subscribers})
	case http.MethodPost:
		var req globalWireNewsletterSubscriberRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid newsletter subscriber request"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || !strings.Contains(email, "@") {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "valid email is required"})
			return
		}
		subscriber, err := h.rt.Store().CreateGlobalWireNewsletterSubscriber(r.Context(), types.GlobalWireNewsletterSubscriber{
			ID:      "global-wire-newsletter-subscriber-" + uuid.NewString(),
			OwnerID: ownerID,
			Email:   email,
			Label:   firstNonEmptyString(strings.TrimSpace(req.Label), "Global Wire subscriber"),
			Status:  "active",
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create newsletter subscriber"})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWireNewsletterSubscriberResponse{Subscriber: subscriber})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleGlobalWireNewsletterIssues creates/lists owner-scoped newsletter issue
// ledgers from public links and subscriber destinations.
func (h *APIHandler) HandleGlobalWireNewsletterIssues(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		storyID := strings.TrimSpace(r.URL.Query().Get("story_id"))
		issues, err := h.rt.Store().ListGlobalWireNewsletterIssues(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter issues"})
			return
		}
		deliveries, err := h.rt.Store().ListGlobalWireNewsletterDeliveries(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter deliveries"})
			return
		}
		receipts, err := h.rt.Store().ListGlobalWireNewsletterProviderReceipts(r.Context(), ownerID, storyID, 100)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list newsletter provider receipts"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"newsletter_issues":            issues,
			"newsletter_deliveries":        deliveries,
			"newsletter_provider_receipts": receipts,
		})
	case http.MethodPost:
		var req globalWireNewsletterIssueRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid newsletter issue request"})
			return
		}
		issue, deliveries, receipts, links, subscribers, err := h.createGlobalWireNewsletterIssue(r, ownerID, req)
		if err != nil {
			if err == store.ErrNotFound {
				writeAPIJSON(w, http.StatusNotFound, apiError{Error: "newsletter public link not found"})
				return
			}
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusCreated, globalWireNewsletterIssueResponse{
			Issue:       issue,
			Deliveries:  deliveries,
			Receipts:    receipts,
			PublicLinks: links,
			Subscribers: subscribers,
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

func normalizeGlobalWirePublicationArtifactReviewDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "approve", "approved", "publish", "published":
		return "publication-approved"
	case "reject", "rejected", "pull", "pulled":
		return "publication-rejected"
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
		"source_entities":   globalWireRuntimeSourceEntities(story),
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

func appendStringListIfMissing(values []string, additions []string) []string {
	out := values
	for _, value := range additions {
		out = appendStringIfMissing(out, value)
	}
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
	sourceRef := globalWireRuntimeSourceRef(globalWireSourceItemFromContentItem(candidate, item), 1)
	updateSummary := strings.TrimSpace(candidate.Summary)
	if updateSummary == "" {
		updateSummary = strings.TrimSpace(item.TextContent)
	}
	if updateSummary == "" {
		updateSummary = "A newly reviewed source changed the story's source neighborhood."
	}
	if len(updateSummary) > 320 {
		updateSummary = strings.TrimSpace(updateSummary[:320]) + "..."
	}
	projection := strings.TrimSpace(story.Projections["wire-style"])
	if projection == "" {
		projection = strings.TrimSpace(story.Dek)
	}
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		projection,
		"",
		"The latest version incorporates " + sourceRef + " as reviewed source context. " + updateSummary,
	}
	return strings.Join(lines, "\n")
}

func globalWireRuntimeSourceEntities(story types.GlobalWireStory) []vtextSourceEntity {
	all := []types.GlobalWireSourceItem{}
	all = append(all, story.Manifest.Lead...)
	all = append(all, story.Manifest.Supporting...)
	all = append(all, story.Manifest.Contrary...)
	all = append(all, story.Manifest.Context...)
	entities := make([]vtextSourceEntity, 0, len(all))
	for _, item := range all {
		if entity, ok := globalWireRuntimeSourceEntity(item); ok {
			entities = append(entities, entity)
		}
	}
	return entities
}

func globalWireRuntimeSourceEntitiesWithPromotedItem(story types.GlobalWireStory, review types.GlobalWireProjectionReview, sourceItem *types.ContentItem) []vtextSourceEntity {
	entities := globalWireRuntimeSourceEntities(story)
	if sourceItem == nil || strings.TrimSpace(sourceItem.ContentID) == "" {
		return entities
	}
	promoted := globalWireSourceItemFromProjectionReview(review, *sourceItem)
	for _, entity := range entities {
		if entity.Target.ContentID == promoted.ContentID {
			return entities
		}
	}
	if entity, ok := globalWireRuntimeSourceEntity(promoted); ok {
		entities = append(entities, entity)
	}
	return entities
}

func globalWireRuntimeSourceEntity(item types.GlobalWireSourceItem) (vtextSourceEntity, bool) {
	contentID := strings.TrimSpace(item.ContentID)
	if contentID == "" {
		return vtextSourceEntity{}, false
	}
	entityID := globalWireRuntimeSourceEntityID(item)
	if entityID == "" {
		return vtextSourceEntity{}, false
	}
	return vtextSourceEntity{
		EntityID: entityID,
		Kind:     "content_item",
		Label:    strings.TrimSpace(item.Title),
		Target: vtextSourceEntityTarget{
			TargetKind:   "content_item",
			ContentID:    contentID,
			CanonicalURL: strings.TrimSpace(item.CanonicalURL),
		},
		Selectors: []vtextSourceEntitySelector{{SelectorKind: "whole_resource"}},
		Display: vtextSourceEntityDisplay{
			InlineMode:       "collapsed_citation",
			ExpandedMode:     "source_card",
			OpenSurface:      "source",
			DefaultCollapsed: true,
		},
		Evidence: vtextSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
			Relation:      firstNonEmptyString(item.Role, "context"),
		},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "global_wire",
			RightsScope:         "private_user_source",
			UntrustedSourceText: true,
		},
	}, true
}

func globalWireSourceItemFromProjectionReview(review types.GlobalWireProjectionReview, item types.ContentItem) types.GlobalWireSourceItem {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Promoted Global Wire source"
	}
	return types.GlobalWireSourceItem{
		ID:           "source-" + item.ContentID,
		ContentID:    item.ContentID,
		Title:        title,
		Standing:     firstNonEmptyString(item.SourceType, "reviewed source artifact"),
		Role:         "supporting",
		CanonicalURL: firstNonEmptyString(item.CanonicalURL, item.SourceURL),
	}
}

func globalWireLeadSourceRef(story types.GlobalWireStory) string {
	sourceGroups := [][]types.GlobalWireSourceItem{
		story.Manifest.Lead,
		story.Manifest.Supporting,
		story.Manifest.Contrary,
		story.Manifest.Context,
	}
	for _, group := range sourceGroups {
		for _, item := range group {
			if strings.TrimSpace(item.ContentID) != "" || strings.TrimSpace(item.ID) != "" {
				return globalWireRuntimeSourceRef(item, 1)
			}
		}
	}
	return "the current source neighborhood"
}

func globalWireClaimSentence(story types.GlobalWireStory) string {
	claims := []string{}
	for _, claim := range story.Claims {
		claim = strings.TrimSpace(claim)
		if claim != "" {
			claims = append(claims, claim)
		}
		if len(claims) >= 2 {
			break
		}
	}
	if len(claims) == 0 {
		return ""
	}
	return "The article's working account is that " + strings.Join(claims, "; ") + "."
}

func globalWireRuntimeSourceRef(item types.GlobalWireSourceItem, fallback int) string {
	label := strings.TrimSpace(item.Title)
	if label == "" {
		label = fmt.Sprintf("source %d", fallback)
	}
	entityID := globalWireRuntimeSourceEntityID(item)
	if entityID == "" {
		return label
	}
	return fmt.Sprintf("[%s](source:%s)", label, entityID)
}

func globalWireRuntimeSourceEntityID(item types.GlobalWireSourceItem) string {
	base := firstNonEmptyString(item.ID, item.ContentID, item.Title)
	if base == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-_")
	if cleaned == "" {
		return ""
	}
	return "gw-src-" + cleaned
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
	sourceReviewSignals := []types.GlobalWireSourceReviewSignal{}
	researchTasks := []types.GlobalWireResearchTask{}
	extractionArtifacts := []types.GlobalWireExtractionArtifact{}
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
			ID:                      globalWireSourceRegistryEntryID(ownerID, story.ID),
			OwnerID:                 ownerID,
			StoryID:                 story.ID,
			Query:                   story.Headline,
			SourceScope:             "story-neighborhood",
			Status:                  "active",
			SourceStandingPolicy:    globalWireSourceStandingPolicy(story),
			SourceStandingRationale: globalWireSourceStandingRationale(story),
			CadenceSeconds:          globalWireSourceCadenceSeconds(req),
			NextDueAt:               time.Now().UTC().Add(time.Duration(globalWireSourceCadenceSeconds(req)) * time.Second),
			LastCycleID:             cycleID,
			LastScheduledRunID:      req.SchedulerRunID,
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
		claim, sourceReviewSignal, task, extraction, err := h.createGlobalWireClaimResearchArtifacts(r, ownerID, story, item, classification, run, contribution, decision, candidate)
		if err != nil {
			return globalWireFetchCycleResponse{}, err
		}
		refreshes = append(refreshes, run)
		refreshIDs = append(refreshIDs, run.ID)
		contributions = append(contributions, contribution)
		candidates = append(candidates, candidate)
		claimRecords = append(claimRecords, claim)
		sourceReviewSignals = append(sourceReviewSignals, sourceReviewSignal)
		researchTasks = append(researchTasks, task)
		extractionArtifacts = append(extractionArtifacts, extraction)
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
		Status:              cycle.Status,
		Message:             cycle.Message,
		FetchCycle:          cycle,
		RegistryEntries:     registry,
		RefreshRuns:         refreshes,
		ContentItems:        contentItems,
		Contributions:       contributions,
		Candidates:          candidates,
		ClaimRecords:        claimRecords,
		SourceReviewSignals: sourceReviewSignals,
		ResearchTasks:       researchTasks,
		ExtractionArtifacts: extractionArtifacts,
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

func globalWireSourceCadenceSeconds(req globalWireFetchCycleRequest) int {
	if req.CadenceSeconds > 0 {
		if req.CadenceSeconds < 900 {
			return 900
		}
		if req.CadenceSeconds > 86400 {
			return 86400
		}
		return req.CadenceSeconds
	}
	if req.SchedulerMode {
		return 3600
	}
	return 21600
}

func globalWireSourceStandingPolicy(story types.GlobalWireStory) string {
	if len(story.Manifest.Lead) > 0 {
		return "lead-first-source-standing-review"
	}
	if len(story.Manifest.Supporting) > 0 {
		return "supporting-source-standing-review"
	}
	return "context-source-standing-review"
}

func globalWireSourceStandingRationale(story types.GlobalWireStory) string {
	leadCount := len(story.Manifest.Lead)
	supportingCount := len(story.Manifest.Supporting)
	contraryCount := len(story.Manifest.Contrary)
	return fmt.Sprintf("Refresh the %q headline neighborhood using source standing as review input: %d lead, %d supporting, %d contrary source(s) currently frame the story. New SourceItems create review artifacts, not automatic StoryGraph mutations.", story.Headline, leadCount, supportingCount, contraryCount)
}

func (h *APIHandler) createGlobalWireSourceSchedulerRun(r *http.Request, ownerID string, req globalWireFetchCycleRequest, resp globalWireFetchCycleResponse) (types.GlobalWireSourceSchedulerRun, error) {
	policies := []string{}
	for _, entry := range resp.RegistryEntries {
		policies = appendStringIfMissing(policies, firstNonEmptyString(entry.SourceStandingPolicy, "source-standing-review"))
	}
	status := "scheduled-cycle-recorded"
	if resp.FetchCycle.Status == "unavailable" {
		status = "scheduled-cycle-unavailable"
	} else if resp.FetchCycle.Status == "completed-with-gaps" {
		status = "scheduled-cycle-completed-with-gaps"
	}
	message := resp.FetchCycle.Message
	if strings.TrimSpace(message) == "" {
		message = "Scheduled source-standing cycle recorded; no platform StoryGraph mutation applied."
	}
	runID := firstNonEmptyString(req.SchedulerRunID, "global-wire-source-scheduler-run-"+uuid.NewString())
	return h.rt.Store().CreateGlobalWireSourceSchedulerRun(r.Context(), types.GlobalWireSourceSchedulerRun{
		ID:               runID,
		OwnerID:          ownerID,
		Trigger:          firstNonEmptyString(strings.TrimSpace(req.Trigger), "scheduled-source-standing-cycle"),
		Status:           status,
		StoryIDs:         resp.FetchCycle.StoryIDs,
		RegistryEntryIDs: resp.FetchCycle.RegistryEntryIDs,
		FetchCycleID:     resp.FetchCycle.ID,
		StandingPolicies: policies,
		Message:          message,
	})
}

func (h *APIHandler) createGlobalWireClaimResearchArtifacts(r *http.Request, ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, contribution types.GlobalWireContribution, decision types.GlobalWireReconciliationDecision, candidate types.GlobalWireGraphUpdateCandidate) (types.GlobalWireClaimRecord, types.GlobalWireSourceReviewSignal, types.GlobalWireResearchTask, types.GlobalWireExtractionArtifact, error) {
	claim := globalWireClaimRecordFromRefresh(ownerID, story, item, classification, run, contribution, decision, candidate)
	savedClaim, err := h.rt.Store().CreateGlobalWireClaimRecord(r.Context(), claim)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireSourceReviewSignal{}, types.GlobalWireResearchTask{}, types.GlobalWireExtractionArtifact{}, err
	}
	signal := globalWireSourceReviewSignalFromClaim(ownerID, story, item, classification, run, candidate, savedClaim)
	savedSignal, err := h.rt.Store().CreateGlobalWireSourceReviewSignal(r.Context(), signal)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireSourceReviewSignal{}, types.GlobalWireResearchTask{}, types.GlobalWireExtractionArtifact{}, err
	}
	task := globalWireResearchTaskFromClaim(ownerID, story, item, classification, run, contribution, candidate, savedClaim)
	savedTask, err := h.rt.Store().CreateGlobalWireResearchTask(r.Context(), task)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireSourceReviewSignal{}, types.GlobalWireResearchTask{}, types.GlobalWireExtractionArtifact{}, err
	}
	extraction := globalWireExtractionArtifactFromClaim(ownerID, story, item, classification, run, candidate, savedClaim)
	savedExtraction, err := h.rt.Store().CreateGlobalWireExtractionArtifact(r.Context(), extraction)
	if err != nil {
		return types.GlobalWireClaimRecord{}, types.GlobalWireSourceReviewSignal{}, types.GlobalWireResearchTask{}, types.GlobalWireExtractionArtifact{}, err
	}
	return savedClaim, savedSignal, savedTask, savedExtraction, nil
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
		evidenceGap = "Compare the imported evidence against the existing source neighborhood and decide whether the claim should narrow, broaden, or stay unchanged."
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

func globalWireSourceReviewSignalFromClaim(ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, candidate types.GlobalWireGraphUpdateCandidate, claim types.GlobalWireClaimRecord) types.GlobalWireSourceReviewSignal {
	signalKind := "source-standing"
	overlapState := "source-neighborhood-overlap-unreviewed"
	contradictionState := "no-contradiction-claimed"
	status := "review-signal-open"
	relatedStoryID := ""
	switch classification.UpdateClassification {
	case "claim-changed":
		signalKind = "claim-change"
		overlapState = "claim-overlap-review-required"
	case "contradiction-added":
		signalKind = "contradiction"
		overlapState = "contrary-source-neighborhood"
		contradictionState = "contradiction-or-qualification-review-required"
	case "front-page-prominence-changed":
		signalKind = "prominence"
		overlapState = "freshness-prominence-review-required"
	case "related-story-edge-added":
		signalKind = "related-story-edge"
		overlapState = "related-story-overlap-review-required"
		if len(story.Related) > 0 {
			relatedStoryID = story.Related[0]
		}
	case "source-manifest-update":
		signalKind = "source-manifest"
		overlapState = "source-tier-placement-review-required"
	}
	evidenceRefs := []string{
		"story:" + story.ID,
		"refresh:" + run.ID,
		"claim:" + claim.ID,
		"source_content:" + item.ContentID,
		"candidate:" + candidate.ID,
	}
	return types.GlobalWireSourceReviewSignal{
		ID:                   "global-wire-source-review-signal-" + uuid.NewString(),
		OwnerID:              ownerID,
		StoryID:              story.ID,
		RefreshID:            run.ID,
		ClaimID:              claim.ID,
		SourceContentID:      item.ContentID,
		CandidateID:          candidate.ID,
		SignalKind:           signalKind,
		UpdateClassification: classification.UpdateClassification,
		SourceStanding:       claim.SourceStanding,
		OverlapState:         overlapState,
		ContradictionState:   contradictionState,
		RelatedStoryID:       relatedStoryID,
		ProjectionAction:     classification.ProjectionAction,
		Status:               status,
		Rationale:            globalWireSourceReviewSignalRationale(story, item, classification, claim),
		EvidenceRefs:         evidenceRefs,
	}
}

func globalWireSourceReviewSignalRationale(story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, claim types.GlobalWireClaimRecord) string {
	sourceTitle := firstNonEmptyString(item.Title, item.CanonicalURL, item.SourceURL, item.ContentID, "Imported SourceItem")
	return strings.Join([]string{
		"Review signal for StoryGraph headline \"" + story.Headline + "\".",
		"Source: " + sourceTitle + ".",
		"Update class: " + firstNonEmptyString(classification.UpdateClassification, "source-manifest-update") + ".",
		"Source standing: " + firstNonEmptyString(claim.SourceStanding, "unreviewed") + ".",
		"Evidence gap: " + claim.EvidenceGap,
		"This signal is non-oracle review input and does not mutate platform StoryGraph stories.",
	}, " ")
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

func globalWireExtractionArtifactFromClaim(ownerID string, story types.GlobalWireStory, item types.ContentItem, classification globalWireSourceUpdateClassification, run types.GlobalWireSourceRefreshRun, candidate types.GlobalWireGraphUpdateCandidate, claim types.GlobalWireClaimRecord) types.GlobalWireExtractionArtifact {
	sourceLabel := firstNonEmptyString(item.Title, item.CanonicalURL, item.SourceURL, item.ContentID, "Imported SourceItem")
	createdAt := run.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	entities := []string{}
	entities = appendStringIfMissing(entities, story.Headline)
	if sourceLabel != story.Headline {
		entities = appendStringIfMissing(entities, sourceLabel)
	}
	if strings.TrimSpace(candidate.Title) != "" {
		entities = appendStringIfMissing(entities, candidate.Title)
	}
	events := []string{
		firstNonEmptyString(classification.UpdateClassification, "source-manifest-update") + " signal for " + story.Headline,
		"SourceItem imported for claim review: " + sourceLabel,
	}
	timeline := []string{
		createdAt.UTC().Format(time.RFC3339) + " source refresh " + run.ID,
	}
	return types.GlobalWireExtractionArtifact{
		ID:              "global-wire-extraction-" + uuid.NewString(),
		OwnerID:         ownerID,
		StoryID:         story.ID,
		ClaimID:         claim.ID,
		RefreshID:       run.ID,
		SourceContentID: item.ContentID,
		CandidateID:     candidate.ID,
		Entities:        entities,
		Events:          events,
		Timeline:        timeline,
		Uncertainty:     claim.UncertaintyState,
		Rationale:       "Low-resolution extraction overlay from SourceItem title/body, StoryGraph headline, and refresh classification; review before publication. This enriches claim review and does not create or replace StoryGraph nodes.",
		Status:          "provisional-review",
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

func (h *APIHandler) createGlobalWirePublicationUpdate(r *http.Request, ownerID string, req globalWirePublicationUpdateRequest) (types.GlobalWirePublicationUpdate, types.GlobalWireResearchEvidenceDecision, *types.GlobalWireGraphUpdateCandidate, []types.GlobalWireProjectionReview, *types.ContentItem, error) {
	decision, err := h.rt.Store().GetGlobalWireResearchEvidenceDecision(r.Context(), ownerID, req.ResearchDecisionID)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
	}
	if decision.Decision != "accepted-for-review" || decision.ResultState != "ready-for-platform-review" {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, store.ErrNotFound
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(decision.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, decision.SourceContentID)
		if err != nil {
			return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
		}
		sourceItem = &rec
	}
	story, err := h.globalWireStoryContext(r, ownerID, decision.StoryID, sourceItem)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
	}
	var candidate *types.GlobalWireGraphUpdateCandidate
	if strings.TrimSpace(decision.CandidateID) != "" {
		rec, err := h.rt.Store().GetGlobalWireGraphUpdateCandidate(r.Context(), ownerID, decision.CandidateID)
		if err != nil {
			return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
		}
		candidate = &rec
	}
	allReviews, err := h.rt.Store().ListGlobalWireProjectionReviews(r.Context(), ownerID, decision.StoryID, 100)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
	}
	reviews := make([]types.GlobalWireProjectionReview, 0, len(allReviews))
	for _, review := range allReviews {
		if decision.CandidateID == "" || review.CandidateID == decision.CandidateID {
			reviews = append(reviews, review)
		}
	}
	reviewIDs := make([]string, 0, len(reviews))
	reviewStates := make([]string, 0, len(reviews))
	for _, review := range reviews {
		reviewIDs = append(reviewIDs, review.ID)
		reviewStates = append(reviewStates, review.Status)
	}
	extractions, err := h.rt.Store().ListGlobalWireExtractionArtifacts(r.Context(), ownerID, decision.StoryID, 100)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
	}
	extractionIDs := globalWirePublicationExtractionIDs(decision, extractions)
	summary := req.Summary
	if summary == "" {
		summary = globalWirePublicationUpdateSummary(story, decision, candidate, reviews)
	}
	update, err := h.rt.Store().CreateGlobalWirePublicationUpdate(r.Context(), types.GlobalWirePublicationUpdate{
		ID:                  "global-wire-publication-update-" + uuid.NewString(),
		OwnerID:             ownerID,
		StoryID:             decision.StoryID,
		CandidateID:         decision.CandidateID,
		ResearchDecisionID:  decision.ID,
		EvidenceID:          decision.EvidenceID,
		SourceContentID:     decision.SourceContentID,
		ExtractionIDs:       extractionIDs,
		ProjectionReviewIDs: reviewIDs,
		ProjectionStates:    reviewStates,
		RollbackRefs:        globalWirePublicationRollbackRefs(story, decision, candidate, reviews, extractionIDs),
		Status:              "packaged-for-publication-review",
		Summary:             summary,
	})
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, types.GlobalWireResearchEvidenceDecision{}, nil, nil, nil, err
	}
	return update, decision, candidate, reviews, sourceItem, nil
}

func globalWirePublicationExtractionIDs(decision types.GlobalWireResearchEvidenceDecision, extractions []types.GlobalWireExtractionArtifact) []string {
	ids := []string{}
	for _, extraction := range extractions {
		if strings.TrimSpace(extraction.ID) == "" {
			continue
		}
		if strings.TrimSpace(decision.CandidateID) != "" && extraction.CandidateID == decision.CandidateID {
			ids = appendStringIfMissing(ids, extraction.ID)
			continue
		}
		if strings.TrimSpace(decision.SourceContentID) != "" && extraction.SourceContentID == decision.SourceContentID {
			ids = appendStringIfMissing(ids, extraction.ID)
		}
	}
	return ids
}

func globalWirePublicationRollbackRefs(story types.GlobalWireStory, decision types.GlobalWireResearchEvidenceDecision, candidate *types.GlobalWireGraphUpdateCandidate, reviews []types.GlobalWireProjectionReview, extractionIDs []string) []string {
	refs := []string{
		"story:" + story.ID,
		"story_vtext:" + story.StoryVTextDoc,
		"research_decision:" + decision.ID,
		"research_evidence:" + decision.EvidenceID,
	}
	if candidate != nil {
		refs = append(refs, "candidate:"+candidate.ID, "candidate_status:"+candidate.Status)
	}
	for _, review := range reviews {
		refs = append(refs, "projection_review:"+review.ID+":"+review.Status)
		if strings.TrimSpace(review.ApprovedStoryDocID) != "" {
			refs = append(refs, "approved_projection_doc:"+review.ApprovedStoryDocID)
		}
		if strings.TrimSpace(review.ApprovedRevisionID) != "" {
			refs = append(refs, "approved_projection_revision:"+review.ApprovedRevisionID)
		}
		if strings.TrimSpace(review.DraftStoryDocID) != "" {
			refs = append(refs, "draft_projection_doc:"+review.DraftStoryDocID)
		}
	}
	for _, id := range extractionIDs {
		refs = append(refs, "extraction:"+id)
	}
	out := []string{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" && !strings.HasSuffix(ref, ":") {
			out = append(out, ref)
		}
	}
	return out
}

func globalWirePublicationUpdateSummary(story types.GlobalWireStory, decision types.GlobalWireResearchEvidenceDecision, candidate *types.GlobalWireGraphUpdateCandidate, reviews []types.GlobalWireProjectionReview) string {
	candidateState := "no linked candidate"
	if candidate != nil {
		candidateState = candidate.Status
	}
	return fmt.Sprintf("Publication update package for %q: research decision %s, candidate state %s, %d projection review(s). This is queued for owner/platform publication review and does not publish or mutate the platform story.", story.Headline, decision.ID, candidateState, len(reviews))
}

func (h *APIHandler) globalWireStoryContext(r *http.Request, ownerID, storyID string, sourceItem *types.ContentItem) (types.GlobalWireStory, error) {
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, storyID)
	if err == nil {
		return story, nil
	}
	if err != store.ErrNotFound || !isGlobalWireSourceNativeStoryID(storyID) {
		return types.GlobalWireStory{}, err
	}
	if sourceItem != nil {
		return globalWireSourceNativeStory(ownerID, *sourceItem), nil
	}
	return types.GlobalWireStory{}, store.ErrNotFound
}

func (h *APIHandler) createGlobalWirePublicationArtifact(r *http.Request, ownerID string, req globalWirePublicationArtifactRequest) (types.GlobalWirePublicationArtifact, types.GlobalWirePublicationUpdate, types.GlobalWireStory, []types.GlobalWireProjectionReview, *types.ContentItem, error) {
	update, err := h.rt.Store().GetGlobalWirePublicationUpdate(r.Context(), ownerID, req.UpdateID)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
	}
	if update.Status != "packaged-for-publication-review" {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, store.ErrNotFound
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(update.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, update.SourceContentID)
		if err != nil {
			return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
		}
		sourceItem = &rec
	}
	story, err := h.globalWireStoryContext(r, ownerID, update.StoryID, sourceItem)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
	}
	reviews, err := h.globalWirePublicationArtifactProjectionReviews(r, ownerID, update)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
	}
	styleDocIDs := []string{}
	for _, review := range reviews {
		if strings.TrimSpace(review.StyleDocID) != "" {
			styleDocIDs = appendStringIfMissing(styleDocIDs, review.StyleDocID)
		}
	}
	schedulerRunIDs, err := h.globalWirePublicationArtifactSchedulerRunIDs(r, ownerID, update.StoryID)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
	}
	channel := firstNonEmptyString(req.Channel, "newsletter")
	title := firstNonEmptyString(req.Title, "Publication review: "+story.Headline)
	rollbackRefs := append([]string{}, update.RollbackRefs...)
	rollbackRefs = appendStringIfMissing(rollbackRefs, "publication_update:"+update.ID)
	artifact, err := h.rt.Store().CreateGlobalWirePublicationArtifact(r.Context(), types.GlobalWirePublicationArtifact{
		ID:                  "global-wire-publication-artifact-" + uuid.NewString(),
		OwnerID:             ownerID,
		UpdateID:            update.ID,
		StoryID:             update.StoryID,
		CandidateID:         update.CandidateID,
		StoryVTextDocID:     story.StoryVTextDoc,
		SourceContentID:     update.SourceContentID,
		Channel:             channel,
		Status:              "publication-review-ready",
		Title:               title,
		Body:                globalWirePublicationArtifactBody(story, update, reviews, sourceItem, schedulerRunIDs),
		StyleDocIDs:         styleDocIDs,
		ProjectionReviewIDs: update.ProjectionReviewIDs,
		ExtractionIDs:       update.ExtractionIDs,
		SchedulerRunIDs:     schedulerRunIDs,
		CitationRefs:        globalWirePublicationArtifactCitationRefs(story, update, styleDocIDs, schedulerRunIDs),
		RollbackRefs:        rollbackRefs,
	})
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, types.GlobalWirePublicationUpdate{}, types.GlobalWireStory{}, nil, nil, err
	}
	return artifact, update, story, reviews, sourceItem, nil
}

func parseGlobalWireFeedLimit(raw string) int {
	limit := 20
	if strings.TrimSpace(raw) == "" {
		return limit
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return limit
	}
	if parsed <= 0 {
		return limit
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func (h *APIHandler) globalWirePublicationFeedItems(r *http.Request, ownerID string, artifacts []types.GlobalWirePublicationArtifact, channel string) ([]globalWirePublicationFeedItem, error) {
	channel = strings.TrimSpace(channel)
	items := make([]globalWirePublicationFeedItem, 0, len(artifacts))
	for _, artifact := range artifacts {
		if channel != "" && artifact.Channel != channel {
			continue
		}
		var sourceItem *types.ContentItem
		if strings.TrimSpace(artifact.SourceContentID) != "" {
			rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, artifact.SourceContentID)
			if err != nil {
				return nil, err
			}
			sourceItem = &rec
		}
		story, err := h.globalWireStoryContext(r, ownerID, artifact.StoryID, sourceItem)
		if err != nil {
			return nil, err
		}
		items = append(items, globalWirePublicationFeedItem{
			Artifact:      artifact,
			Story:         story,
			SourceItem:    sourceItem,
			CitationCount: len(artifact.CitationRefs),
			RollbackCount: len(artifact.RollbackRefs),
			Status:        artifact.Status,
		})
	}
	return items, nil
}

func (h *APIHandler) publishGlobalWireArtifactToCommunityEdition(r *http.Request, ownerID string, artifact types.GlobalWirePublicationArtifact) (*globalWireEditionResponse, error) {
	articleDoc, articleRev, review, ok, err := h.approvedGlobalWireArticleForArtifact(r, ownerID, artifact)
	if err != nil || !ok {
		return nil, err
	}
	platformOwner := sourceMaxxPlatformOwnerID()
	now := time.Now().UTC()
	platformDoc, platformRev, err := h.ensureCommunityWirePlatformArticle(r, platformOwner, ownerID, artifact, articleDoc, articleRev, review, now)
	if err != nil {
		return nil, err
	}
	editionDoc, editionRev, err := h.ensureCommunityWireEditionIncludesArticle(r, platformOwner, platformDoc, platformRev, artifact, now)
	if err != nil {
		return nil, err
	}
	return &globalWireEditionResponse{
		DocID:          editionDoc.DocID,
		RevisionID:     editionRev.RevisionID,
		SourcePath:     communityWireEditionSourcePath,
		Title:          editionDoc.Title,
		IncludedDocIDs: communityWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID),
		UpdatedAt:      editionDoc.UpdatedAt.Format(time.RFC3339Nano),
	}, nil
}

func (h *APIHandler) approvedGlobalWireArticleForArtifact(r *http.Request, ownerID string, artifact types.GlobalWirePublicationArtifact) (types.Document, types.Revision, types.GlobalWireProjectionReview, bool, error) {
	for _, reviewID := range artifact.ProjectionReviewIDs {
		reviewID = strings.TrimSpace(reviewID)
		if reviewID == "" {
			continue
		}
		review, err := h.rt.Store().GetGlobalWireProjectionReview(r.Context(), ownerID, reviewID)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, false, err
		}
		if strings.TrimSpace(review.ApprovedStoryDocID) == "" || strings.TrimSpace(review.ApprovedRevisionID) == "" {
			continue
		}
		doc, err := h.rt.Store().GetDocument(r.Context(), review.ApprovedStoryDocID, ownerID)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, false, err
		}
		rev, err := h.rt.Store().GetRevision(r.Context(), review.ApprovedRevisionID, ownerID)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, false, err
		}
		return doc, rev, review, true, nil
	}
	return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, false, nil
}

func (h *APIHandler) ensureCommunityWirePlatformArticle(r *http.Request, platformOwner, approvingOwner string, artifact types.GlobalWirePublicationArtifact, sourceDoc types.Document, sourceRev types.Revision, review types.GlobalWireProjectionReview, now time.Time) (types.Document, types.Revision, error) {
	sourcePath := "global-wire/articles/" + artifact.ID + ".vtext"
	if existingDocID, err := h.rt.Store().GetDocumentAlias(r.Context(), platformOwner, sourcePath); err == nil {
		doc, err := h.rt.Store().GetDocument(r.Context(), existingDocID, platformOwner)
		if err != nil {
			return types.Document{}, types.Revision{}, err
		}
		if strings.TrimSpace(doc.CurrentRevisionID) == "" {
			return types.Document{}, types.Revision{}, store.ErrNotFound
		}
		rev, err := h.rt.Store().GetRevision(r.Context(), doc.CurrentRevisionID, platformOwner)
		if err != nil {
			return types.Document{}, types.Revision{}, err
		}
		return doc, rev, nil
	} else if err != store.ErrNotFound {
		return types.Document{}, types.Revision{}, err
	}
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   platformOwner,
		Title:     firstNonEmptyString(artifact.Title, sourceDoc.Title, artifact.StoryID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	meta, err := communityWirePlatformArticleMetadata(approvingOwner, artifact, sourceDoc, sourceRev, review)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	rev := types.Revision{
		RevisionID:  uuid.NewString(),
		DocID:       doc.DocID,
		OwnerID:     platformOwner,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "Community Wire",
		Content:     strings.TrimSpace(sourceRev.Content),
		Citations:   sourceRev.Citations,
		Metadata:    meta,
		CreatedAt:   now,
	}
	if strings.TrimSpace(rev.Content) == "" {
		rev.Content = "The approved article revision was empty at publication approval time."
	}
	if len(rev.Citations) == 0 {
		rev.Citations = json.RawMessage("[]")
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), platformOwner, sourcePath, doc.DocID, now); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	doc, err = h.rt.Store().GetDocument(r.Context(), doc.DocID, platformOwner)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, platformOwner)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	return doc, storedRev, nil
}

func communityWirePlatformArticleMetadata(approvingOwner string, artifact types.GlobalWirePublicationArtifact, sourceDoc types.Document, sourceRev types.Revision, review types.GlobalWireProjectionReview) (json.RawMessage, error) {
	sourceMeta := decodeRevisionMetadata(sourceRev.Metadata)
	sourceEntities := sourceMeta["source_entities"]
	if sourceEntities == nil {
		sourceEntities = []any{}
	}
	selectedStyleTitle := firstNonEmptyString(review.StyleTitle, "Style.vtext: Global Wire")
	return json.Marshal(map[string]any{
		"source":                      "edit_vtext",
		"source_network_cycle_id":     "publication-artifact:" + artifact.ID,
		"source_network_request_id":   artifact.UpdateID,
		"source_network_request_kind": "community_wire_publication_approval",
		"created_from":                "global_wire_publication_artifact_approval",
		"artifact_kind":               "article_revision",
		"article_version":             true,
		"approving_owner_id":          approvingOwner,
		"publication_artifact_id":     artifact.ID,
		"publication_update_id":       artifact.UpdateID,
		"source_owner_doc_id":         sourceDoc.DocID,
		"source_owner_revision_id":    sourceRev.RevisionID,
		"projection_review":           review.ID,
		"storygraph_id":               artifact.StoryID,
		"candidate_id":                artifact.CandidateID,
		"source_content_id":           artifact.SourceContentID,
		"selected_style_sources":      []map[string]any{{"title": selectedStyleTitle}},
		"selected_style_rationale":    "Owner-approved Community Wire publication artifact.",
		"source_entities":             sourceEntities,
	})
}

func (h *APIHandler) ensureCommunityWireEditionIncludesArticle(r *http.Request, platformOwner string, articleDoc types.Document, articleRev types.Revision, artifact types.GlobalWirePublicationArtifact, now time.Time) (types.Document, types.Revision, error) {
	line := fmt.Sprintf("- [%s](vtext:%s)", firstNonEmptyString(articleDoc.Title, artifact.Title, artifact.StoryID), articleDoc.DocID)
	if editionDocID, err := h.rt.Store().GetDocumentAlias(r.Context(), platformOwner, communityWireEditionSourcePath); err == nil {
		doc, err := h.rt.Store().GetDocument(r.Context(), editionDocID, platformOwner)
		if err != nil {
			return types.Document{}, types.Revision{}, err
		}
		currentContent := ""
		currentRevID := ""
		if strings.TrimSpace(doc.CurrentRevisionID) != "" {
			rev, err := h.rt.Store().GetRevision(r.Context(), doc.CurrentRevisionID, platformOwner)
			if err != nil {
				return types.Document{}, types.Revision{}, err
			}
			currentContent = strings.TrimSpace(rev.Content)
			currentRevID = rev.RevisionID
			if slices.Contains(communityWireEditionIncludedDocIDs(currentContent, doc.DocID), articleDoc.DocID) {
				return doc, rev, nil
			}
		}
		content := currentContent
		if content == "" {
			content = "# Wire\n\nCommunity Wire edition."
		}
		content = strings.TrimSpace(content) + "\n\n" + line
		rev, err := h.createCommunityWireEditionRevision(r, platformOwner, doc.DocID, currentRevID, content, artifact, articleDoc, articleRev, now)
		if err != nil {
			return types.Document{}, types.Revision{}, err
		}
		doc, err = h.rt.Store().GetDocument(r.Context(), doc.DocID, platformOwner)
		if err != nil {
			return types.Document{}, types.Revision{}, err
		}
		return doc, rev, nil
	} else if err != store.ErrNotFound {
		return types.Document{}, types.Revision{}, err
	}
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   platformOwner,
		Title:     "Wire.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	content := "# Wire\n\nCommunity Wire edition.\n\n" + line
	rev, err := h.createCommunityWireEditionRevision(r, platformOwner, doc.DocID, "", content, artifact, articleDoc, articleRev, now)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), platformOwner, communityWireEditionSourcePath, doc.DocID, now); err != nil {
		return types.Document{}, types.Revision{}, err
	}
	doc, err = h.rt.Store().GetDocument(r.Context(), doc.DocID, platformOwner)
	if err != nil {
		return types.Document{}, types.Revision{}, err
	}
	return doc, rev, nil
}

func (h *APIHandler) createCommunityWireEditionRevision(r *http.Request, platformOwner, docID, parentRevisionID, content string, artifact types.GlobalWirePublicationArtifact, articleDoc types.Document, articleRev types.Revision, now time.Time) (types.Revision, error) {
	citations, err := json.Marshal([]types.Citation{
		{ID: "publication-artifact", Type: "global_wire_publication_artifact", Value: artifact.ID, Label: artifact.Title},
		{ID: "article-vtext", Type: "vtext", Value: articleDoc.DocID, Label: articleDoc.Title},
		{ID: "article-revision", Type: "vtext_revision", Value: articleRev.RevisionID, Label: "Approved article revision"},
	})
	if err != nil {
		return types.Revision{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"source":                  "community_wire_edition",
		"created_from":            "global_wire_publication_artifact_approval",
		"publication_artifact_id": artifact.ID,
		"article_doc_id":          articleDoc.DocID,
		"article_revision_id":     articleRev.RevisionID,
	})
	if err != nil {
		return types.Revision{}, err
	}
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            docID,
		OwnerID:          platformOwner,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Community Wire",
		Content:          content,
		Citations:        citations,
		Metadata:         metadata,
		ParentRevisionID: parentRevisionID,
		CreatedAt:        now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Revision{}, err
	}
	return h.rt.Store().GetRevision(r.Context(), rev.RevisionID, platformOwner)
}

func (h *APIHandler) createGlobalWirePublicationDelivery(r *http.Request, ownerID string, req globalWirePublicationDeliveryRequest) (types.GlobalWirePublicationDelivery, types.GlobalWirePublicationArtifact, types.GlobalWireStory, error) {
	artifact, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, req.ArtifactID)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, err
	}
	if artifact.Status != "publication-approved" {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, store.ErrNotFound
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, artifact.StoryID)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, err
	}
	channel := firstNonEmptyString(req.Channel, artifact.Channel, "newsletter")
	deliveryID := "global-wire-publication-delivery-" + uuid.NewString()
	deliveryRef := fmt.Sprintf("global-wire/%s/publications/%s", artifact.StoryID, deliveryID)
	rollbackRefs := appendStringIfMissing(artifact.RollbackRefs, "publication_artifact:"+artifact.ID)
	delivery, err := h.rt.Store().CreateGlobalWirePublicationDelivery(r.Context(), types.GlobalWirePublicationDelivery{
		ID:            deliveryID,
		OwnerID:       ownerID,
		ArtifactID:    artifact.ID,
		StoryID:       artifact.StoryID,
		Channel:       channel,
		Status:        "delivery-ready",
		DeliveryRef:   deliveryRef,
		CitationCount: len(artifact.CitationRefs),
		RollbackCount: len(rollbackRefs),
		CitationRefs:  artifact.CitationRefs,
		RollbackRefs:  rollbackRefs,
	})
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, err
	}
	return delivery, artifact, story, nil
}

func (h *APIHandler) globalWirePublicationDeliveryDetail(r *http.Request, ownerID, deliveryID string) (types.GlobalWirePublicationDelivery, types.GlobalWirePublicationArtifact, types.GlobalWireStory, *types.ContentItem, error) {
	delivery, err := h.rt.Store().GetGlobalWirePublicationDelivery(r.Context(), ownerID, deliveryID)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	artifact, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, delivery.ArtifactID)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, delivery.StoryID)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(artifact.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, artifact.SourceContentID)
		if err != nil {
			return types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
		}
		sourceItem = &rec
	}
	return delivery, artifact, story, sourceItem, nil
}

func (h *APIHandler) createGlobalWireAutoradioScript(r *http.Request, ownerID string, req globalWireAutoradioScriptRequest) (types.GlobalWireAutoradioScript, types.GlobalWirePublicationArtifact, types.GlobalWireStory, *types.ContentItem, error) {
	artifact, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, req.ArtifactID)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	if artifact.Status != "publication-approved" {
		return types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, store.ErrNotFound
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, artifact.StoryID)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(artifact.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, artifact.SourceContentID)
		if err != nil {
			return types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
		}
		sourceItem = &rec
	}
	rollbackRefs := appendStringIfMissing(artifact.RollbackRefs, "publication_artifact:"+artifact.ID)
	scriptID := "global-wire-autoradio-script-" + uuid.NewString()
	script, err := h.rt.Store().CreateGlobalWireAutoradioScript(r.Context(), types.GlobalWireAutoradioScript{
		ID:              scriptID,
		OwnerID:         ownerID,
		ArtifactID:      artifact.ID,
		StoryID:         artifact.StoryID,
		SourceContentID: artifact.SourceContentID,
		Status:          "script-ready",
		Title:           "Autoradio script: " + story.Headline,
		ScriptBody:      globalWireAutoradioScriptBody(story, artifact, sourceItem),
		VoiceNotes:      "Speak in Global Wire mode: concise, source-aware, and explicit about uncertainty. Do not add claims absent from the publication artifact.",
		CitationCount:   len(artifact.CitationRefs),
		RollbackCount:   len(rollbackRefs),
		CitationRefs:    artifact.CitationRefs,
		RollbackRefs:    rollbackRefs,
	})
	if err != nil {
		return types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	return script, artifact, story, sourceItem, nil
}

func (h *APIHandler) createGlobalWireAutoradioEpisode(r *http.Request, ownerID string, req globalWireAutoradioEpisodeRequest) (types.GlobalWireAutoradioEpisode, types.GlobalWireAutoradioScript, types.GlobalWirePublicationArtifact, types.GlobalWireStory, *types.ContentItem, error) {
	script, err := h.rt.Store().GetGlobalWireAutoradioScript(r.Context(), ownerID, req.ScriptID)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	if script.Status != "script-ready" {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, store.ErrNotFound
	}
	artifact, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, script.ArtifactID)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	if artifact.Status != "publication-approved" {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, store.ErrNotFound
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, script.StoryID)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(script.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, script.SourceContentID)
		if err != nil {
			return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
		}
		sourceItem = &rec
	}
	rollbackRefs := appendStringIfMissing(script.RollbackRefs, "autoradio_script:"+script.ID)
	episodeID := "global-wire-autoradio-episode-" + uuid.NewString()
	episode, err := h.rt.Store().CreateGlobalWireAutoradioEpisode(r.Context(), types.GlobalWireAutoradioEpisode{
		ID:              episodeID,
		OwnerID:         ownerID,
		ScriptID:        script.ID,
		ArtifactID:      script.ArtifactID,
		StoryID:         script.StoryID,
		SourceContentID: script.SourceContentID,
		Status:          "episode-ready",
		PlaybackMode:    "browser-speech",
		Title:           "Autoradio episode: " + story.Headline,
		Transcript:      script.ScriptBody,
		VoiceNotes:      script.VoiceNotes + " Playback mode: browser speech synthesis; no external TTS/audio-file provider receipt is claimed.",
		DurationSeconds: globalWireEstimateSpokenDurationSeconds(script.ScriptBody),
		CitationCount:   len(script.CitationRefs),
		RollbackCount:   len(rollbackRefs),
		CitationRefs:    script.CitationRefs,
		RollbackRefs:    rollbackRefs,
	})
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, types.GlobalWireAutoradioScript{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, err
	}
	return episode, script, artifact, story, sourceItem, nil
}

func (h *APIHandler) createGlobalWirePublicationDeliveryExport(r *http.Request, ownerID string, req globalWirePublicationDeliveryExportRequest) (types.GlobalWirePublicationDeliveryExport, types.GlobalWirePublicationDelivery, types.GlobalWirePublicationArtifact, types.GlobalWireStory, *types.GlobalWireAutoradioScript, *types.ContentItem, error) {
	delivery, err := h.rt.Store().GetGlobalWirePublicationDelivery(r.Context(), ownerID, req.DeliveryID)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
	}
	artifact, err := h.rt.Store().GetGlobalWirePublicationArtifact(r.Context(), ownerID, delivery.ArtifactID)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, delivery.StoryID)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(artifact.SourceContentID) != "" {
		rec, err := h.rt.Store().GetContentItem(r.Context(), ownerID, artifact.SourceContentID)
		if err != nil {
			return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
		}
		sourceItem = &rec
	}
	var script *types.GlobalWireAutoradioScript
	scripts, err := h.rt.Store().ListGlobalWireAutoradioScripts(r.Context(), ownerID, delivery.StoryID, 100)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
	}
	for _, candidate := range scripts {
		if candidate.ArtifactID == artifact.ID {
			rec := candidate
			script = &rec
			break
		}
	}
	format := strings.ToLower(firstNonEmptyString(req.Format, "md"))
	if format != "md" && format != "markdown" {
		format = "md"
	}
	rollbackRefs := appendStringIfMissing(delivery.RollbackRefs, "publication_delivery:"+delivery.ID)
	if script != nil {
		rollbackRefs = appendStringIfMissing(rollbackRefs, "autoradio_script:"+script.ID)
	}
	exportID := "global-wire-publication-delivery-export-" + uuid.NewString()
	export, err := h.rt.Store().CreateGlobalWirePublicationDeliveryExport(r.Context(), types.GlobalWirePublicationDeliveryExport{
		ID:              exportID,
		OwnerID:         ownerID,
		DeliveryID:      delivery.ID,
		ArtifactID:      artifact.ID,
		ScriptID:        scriptIDForGlobalWireExport(script),
		StoryID:         delivery.StoryID,
		SourceContentID: artifact.SourceContentID,
		Format:          format,
		Status:          "export-ready",
		Title:           "Global Wire export: " + story.Headline,
		ExportBody:      globalWirePublicationDeliveryExportBody(story, delivery, artifact, script, sourceItem),
		CitationCount:   len(delivery.CitationRefs),
		RollbackCount:   len(rollbackRefs),
		CitationRefs:    delivery.CitationRefs,
		RollbackRefs:    rollbackRefs,
	})
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, types.GlobalWirePublicationDelivery{}, types.GlobalWirePublicationArtifact{}, types.GlobalWireStory{}, nil, nil, err
	}
	return export, delivery, artifact, story, script, sourceItem, nil
}

func (h *APIHandler) createGlobalWirePublicationPublicLink(r *http.Request, ownerID string, req globalWirePublicationPublicLinkRequest) (types.GlobalWirePublicationPublicLink, types.GlobalWirePublicationDeliveryExport, error) {
	export, err := h.rt.Store().GetGlobalWirePublicationDeliveryExport(r.Context(), ownerID, req.ExportID)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, types.GlobalWirePublicationDeliveryExport{}, err
	}
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	linkID := "global-wire-public-link-" + uuid.NewString()
	routePath := "/global-wire/publications/" + token
	rollbackRefs := appendStringIfMissing(export.RollbackRefs, "delivery_export:"+export.ID)
	link, err := h.rt.Store().CreateGlobalWirePublicationPublicLink(r.Context(), types.GlobalWirePublicationPublicLink{
		ID:            linkID,
		OwnerID:       ownerID,
		Token:         token,
		ExportID:      export.ID,
		DeliveryID:    export.DeliveryID,
		ArtifactID:    export.ArtifactID,
		StoryID:       export.StoryID,
		Status:        "public-unlisted",
		RoutePath:     routePath,
		Title:         export.Title,
		ExportBody:    export.ExportBody,
		CitationCount: export.CitationCount,
		RollbackCount: len(rollbackRefs),
		CitationRefs:  export.CitationRefs,
		RollbackRefs:  rollbackRefs,
	})
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, types.GlobalWirePublicationDeliveryExport{}, err
	}
	hydrateGlobalWirePublicLinkDerivedFields(&link)
	return link, export, nil
}

func hydrateGlobalWirePublicLinkDerivedFields(link *types.GlobalWirePublicationPublicLink) {
	if link == nil {
		return
	}
	token := strings.TrimSpace(link.Token)
	if token == "" {
		return
	}
	link.FeedPath = "/api/global-wire/publication-public-links/" + token + "/rss"
}

func writeGlobalWirePublicLinkRSS(w http.ResponseWriter, r *http.Request, link types.GlobalWirePublicationPublicLink) {
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	origin := globalWireRequestOrigin(r)
	publicURL := origin + link.RoutePath
	descriptionLines := []string{
		link.ExportBody,
		"",
		"Provenance:",
		fmt.Sprintf("Citation count: %d", link.CitationCount),
		fmt.Sprintf("Rollback count: %d", link.RollbackCount),
		"Citation refs: " + strings.Join(link.CitationRefs, ", "),
		"Rollback refs: " + strings.Join(link.RollbackRefs, ", "),
	}
	body := strings.Join([]string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<rss version="2.0">`,
		`  <channel>`,
		`    <title>` + xmlText("Choir Global Wire") + `</title>`,
		`    <link>` + xmlText(origin+"/global-wire/publications/"+strings.TrimSpace(link.Token)) + `</link>`,
		`    <description>` + xmlText("Token-scoped Global Wire publication feed") + `</description>`,
		`    <item>`,
		`      <title>` + xmlText(link.Title) + `</title>`,
		`      <link>` + xmlText(publicURL) + `</link>`,
		`      <guid isPermaLink="false">` + xmlText(link.ID) + `</guid>`,
		`      <description>` + xmlText(strings.Join(descriptionLines, "\n")) + `</description>`,
		`    </item>`,
		`  </channel>`,
		`</rss>`,
		``,
	}, "\n")
	_, _ = w.Write([]byte(body))
}

func globalWireRequestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}

func xmlText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func (h *APIHandler) createGlobalWireNewsletterIssue(r *http.Request, ownerID string, req globalWireNewsletterIssueRequest) (types.GlobalWireNewsletterIssue, []types.GlobalWireNewsletterDelivery, []types.GlobalWireNewsletterProviderReceipt, []types.GlobalWirePublicationPublicLink, []types.GlobalWireNewsletterSubscriber, error) {
	publicLinkIDSet := map[string]bool{}
	for _, id := range req.PublicLinkIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			publicLinkIDSet[id] = true
		}
	}
	storyID := strings.TrimSpace(req.StoryID)
	allLinks, err := h.rt.Store().ListGlobalWirePublicationPublicLinks(r.Context(), ownerID, storyID, 100)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, err
	}
	var links []types.GlobalWirePublicationPublicLink
	for _, link := range allLinks {
		if len(publicLinkIDSet) > 0 && !publicLinkIDSet[link.ID] {
			continue
		}
		hydrateGlobalWirePublicLinkDerivedFields(&link)
		links = append(links, link)
	}
	if len(links) == 0 {
		return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, store.ErrNotFound
	}
	if storyID == "" {
		storyID = links[0].StoryID
	}
	var selected []types.GlobalWirePublicationPublicLink
	for _, link := range links {
		if link.StoryID == storyID {
			selected = append(selected, link)
		}
	}
	if len(selected) == 0 {
		selected = links
	}
	subscribers, err := h.rt.Store().ListGlobalWireNewsletterSubscribers(r.Context(), ownerID, 100)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, err
	}
	var activeSubscribers []types.GlobalWireNewsletterSubscriber
	for _, subscriber := range subscribers {
		if subscriber.Status == "active" {
			activeSubscribers = append(activeSubscribers, subscriber)
		}
	}
	if len(activeSubscribers) == 0 {
		return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, fmt.Errorf("create global wire newsletter issue: active subscriber is required")
	}

	issueID := "global-wire-newsletter-issue-" + uuid.NewString()
	var publicLinkIDs, citationRefs, rollbackRefs, bodySections []string
	for _, link := range selected {
		publicLinkIDs = appendStringIfMissing(publicLinkIDs, link.ID)
		citationRefs = appendStringListIfMissing(citationRefs, link.CitationRefs)
		rollbackRefs = appendStringListIfMissing(rollbackRefs, link.RollbackRefs)
		rollbackRefs = appendStringIfMissing(rollbackRefs, "public_link:"+link.ID)
		bodySections = append(bodySections,
			"## "+link.Title+"\n\n"+
				link.ExportBody+"\n\n"+
				"Public reader: "+link.RoutePath+"\n"+
				"RSS: "+link.FeedPath,
		)
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = "Global Wire newsletter: " + selected[0].Title
	}
	issueBody := strings.Join([]string{
		"# " + subject,
		"",
		strings.Join(bodySections, "\n\n"),
		"",
		"## Delivery Ledger",
		fmt.Sprintf("Subscribers: %d", len(activeSubscribers)),
		fmt.Sprintf("Public links: %d", len(selected)),
		"Guardrail: this issue is owner-scoped delivery evidence over public links, not platform StoryGraph mutation.",
	}, "\n")
	var deliveryIDs []string
	var deliveries []types.GlobalWireNewsletterDelivery
	var receipts []types.GlobalWireNewsletterProviderReceipt
	for _, subscriber := range activeSubscribers {
		deliveryID := "global-wire-newsletter-delivery-" + uuid.NewString()
		deliveryIDs = append(deliveryIDs, deliveryID)
		delivery, err := h.rt.Store().CreateGlobalWireNewsletterDelivery(r.Context(), types.GlobalWireNewsletterDelivery{
			ID:            deliveryID,
			OwnerID:       ownerID,
			IssueID:       issueID,
			SubscriberID:  subscriber.ID,
			StoryID:       storyID,
			Status:        "delivery-ready",
			DeliveryRef:   "global-wire/newsletter/" + issueID + "/subscribers/" + subscriber.ID,
			CitationCount: len(citationRefs),
			RollbackCount: len(rollbackRefs),
			CitationRefs:  citationRefs,
			RollbackRefs:  rollbackRefs,
		})
		if err != nil {
			return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, err
		}
		deliveries = append(deliveries, delivery)
		receiptID := "global-wire-newsletter-provider-receipt-" + uuid.NewString()
		eventRefs := []string{
			"newsletter_issue:" + issueID,
			"newsletter_delivery:" + delivery.ID,
			"newsletter_subscriber:" + subscriber.ID,
		}
		for _, linkID := range publicLinkIDs {
			eventRefs = appendStringIfMissing(eventRefs, "public_link:"+linkID)
		}
		receiptRollbackRefs := appendStringListIfMissing(delivery.RollbackRefs, []string{
			"newsletter_issue:" + issueID,
			"newsletter_delivery:" + delivery.ID,
		})
		receipt, err := h.rt.Store().CreateGlobalWireNewsletterProviderReceipt(r.Context(), types.GlobalWireNewsletterProviderReceipt{
			ID:             receiptID,
			OwnerID:        ownerID,
			IssueID:        issueID,
			DeliveryID:     delivery.ID,
			SubscriberID:   subscriber.ID,
			StoryID:        storyID,
			Provider:       "choir-dry-run-mailer",
			ProviderMode:   "dry-run",
			Status:         "provider-dry-run-recorded",
			MessageID:      "dryrun-" + receiptID,
			Recipient:      subscriber.Email,
			DeliveryRef:    delivery.DeliveryRef + "/provider-receipts/" + receiptID,
			AttemptSummary: "Provider dry-run receipt recorded for staging/product-path proof; no external email provider send is claimed.",
			EventRefs:      eventRefs,
			CitationRefs:   delivery.CitationRefs,
			RollbackRefs:   receiptRollbackRefs,
		})
		if err != nil {
			return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, err
		}
		receipts = append(receipts, receipt)
	}
	issue, err := h.rt.Store().CreateGlobalWireNewsletterIssue(r.Context(), types.GlobalWireNewsletterIssue{
		ID:              issueID,
		OwnerID:         ownerID,
		StoryID:         storyID,
		Status:          "issue-ready",
		Subject:         subject,
		IssueBody:       issueBody,
		PublicLinkIDs:   publicLinkIDs,
		DeliveryIDs:     deliveryIDs,
		SubscriberCount: len(activeSubscribers),
		CitationCount:   len(citationRefs),
		RollbackCount:   len(rollbackRefs),
		CitationRefs:    citationRefs,
		RollbackRefs:    rollbackRefs,
	})
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, nil, nil, nil, nil, err
	}
	return issue, deliveries, receipts, selected, activeSubscribers, nil
}

func (h *APIHandler) globalWirePublicationArtifactProjectionReviews(r *http.Request, ownerID string, update types.GlobalWirePublicationUpdate) ([]types.GlobalWireProjectionReview, error) {
	if len(update.ProjectionReviewIDs) == 0 {
		return []types.GlobalWireProjectionReview{}, nil
	}
	reviewIDs := map[string]bool{}
	for _, id := range update.ProjectionReviewIDs {
		if strings.TrimSpace(id) != "" {
			reviewIDs[id] = true
		}
	}
	allReviews, err := h.rt.Store().ListGlobalWireProjectionReviews(r.Context(), ownerID, update.StoryID, 100)
	if err != nil {
		return nil, err
	}
	reviews := []types.GlobalWireProjectionReview{}
	for _, review := range allReviews {
		if reviewIDs[review.ID] {
			reviews = append(reviews, review)
		}
	}
	return reviews, nil
}

func (h *APIHandler) globalWirePublicationArtifactSchedulerRunIDs(r *http.Request, ownerID, storyID string) ([]string, error) {
	runs, err := h.rt.Store().ListGlobalWireSourceSchedulerRuns(r.Context(), ownerID, 20)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, run := range runs {
		if len(ids) >= 3 {
			break
		}
		for _, runStoryID := range run.StoryIDs {
			if runStoryID == storyID {
				ids = appendStringIfMissing(ids, run.ID)
				break
			}
		}
	}
	return ids, nil
}

func globalWirePublicationArtifactCitationRefs(story types.GlobalWireStory, update types.GlobalWirePublicationUpdate, styleDocIDs, schedulerRunIDs []string) []string {
	refs := []string{
		"story:" + story.ID,
		"story_vtext:" + story.StoryVTextDoc,
		"publication_update:" + update.ID,
	}
	if strings.TrimSpace(update.SourceContentID) != "" {
		refs = append(refs, "source_item:"+update.SourceContentID)
	}
	if strings.TrimSpace(update.CandidateID) != "" {
		refs = append(refs, "candidate:"+update.CandidateID)
	}
	for _, id := range styleDocIDs {
		refs = append(refs, "style_vtext:"+id)
	}
	for _, id := range update.ProjectionReviewIDs {
		refs = append(refs, "projection_review:"+id)
	}
	for _, id := range update.ExtractionIDs {
		refs = append(refs, "extraction:"+id)
	}
	for _, id := range schedulerRunIDs {
		refs = append(refs, "scheduler_run:"+id)
	}
	out := []string{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" && !strings.HasSuffix(ref, ":") {
			out = appendStringIfMissing(out, ref)
		}
	}
	return out
}

func globalWirePublicationArtifactBody(story types.GlobalWireStory, update types.GlobalWirePublicationUpdate, reviews []types.GlobalWireProjectionReview, sourceItem *types.ContentItem, schedulerRunIDs []string) string {
	sourceLabel := "none"
	if sourceItem != nil {
		sourceLabel = firstNonEmptyString(sourceItem.Title, sourceItem.CanonicalURL, sourceItem.SourceURL, sourceItem.ContentID)
	}
	return strings.Join([]string{
		"Global Wire publication artifact for \"" + story.Headline + "\".",
		"Status: review-ready artifact; this is not public publication and does not mutate the platform story.",
		"Story VText: " + firstNonEmptyString(story.StoryVTextDoc, "missing"),
		"Publication package: " + update.ID,
		"Source: " + sourceLabel,
		fmt.Sprintf("Style.vtext projection reviews: %d", len(reviews)),
		fmt.Sprintf("Extraction refs: %d", len(update.ExtractionIDs)),
		fmt.Sprintf("Scheduler/source-standing refs: %d", len(schedulerRunIDs)),
		"Non-oracle note: this artifact cites source-neighborhood evidence and review state; it is not a final verdict.",
	}, "\n")
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
	var sourceItem *types.ContentItem
	if strings.TrimSpace(review.SourceContentID) != "" {
		item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, review.SourceContentID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
		}
		sourceItem = &item
	}
	story, err := h.globalWireStoryContext(r, ownerID, review.StoryID, sourceItem)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireProjectionReview{}, err
	}
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     story.Headline,
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
		"artifact_kind":     "article_revision_draft",
		"article_version":   true,
		"storygraph_id":     story.ID,
		"projection_review": review.ID,
		"candidate_id":      review.CandidateID,
		"promotion_id":      review.PromotionID,
		"source_content_id": review.SourceContentID,
		"style_id":          review.StyleID,
		"style_doc_id":      review.StyleDocID,
		"draft_state":       "review-draft-not-published",
		"source_entities":   globalWireRuntimeSourceEntitiesWithPromotedItem(story, review, sourceItem),
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
		if isGlobalWireSourceNativeStoryID(review.StoryID) {
			return doc, rev, globalWireSourceNativeProjectionFromReview(review, doc, rev), review, nil
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
	if isGlobalWireSourceNativeStoryID(review.StoryID) {
		return h.approveGlobalWireSourceNativeProjectionReview(r, ownerID, review)
	}
	story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, review.StoryID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
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
		"artifact_kind":      "article_revision",
		"article_version":    true,
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
		"source_entities":    globalWireRuntimeSourceEntities(story),
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

func (h *APIHandler) approveGlobalWireSourceNativeProjectionReview(r *http.Request, ownerID string, review types.GlobalWireProjectionReview) (types.Document, types.Revision, types.GlobalWireStoryProjection, types.GlobalWireProjectionReview, error) {
	draftDoc, err := h.rt.Store().GetDocument(r.Context(), review.DraftStoryDocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	draftRev, err := h.rt.Store().GetRevision(r.Context(), draftDoc.CurrentRevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	var sourceItem *types.ContentItem
	if strings.TrimSpace(review.SourceContentID) != "" {
		item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, review.SourceContentID)
		if err != nil {
			return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
		}
		sourceItem = &item
	}
	story, err := h.globalWireStoryContext(r, ownerID, review.StoryID, sourceItem)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	now := time.Now().UTC()
	metadata, err := json.Marshal(map[string]any{
		"created_from":       "global_wire_source_native_projection_review_approval",
		"artifact_kind":      "article_revision",
		"article_version":    true,
		"source_native":      true,
		"storygraph_id":      review.StoryID,
		"projection_review":  review.ID,
		"draft_story_doc_id": review.DraftStoryDocID,
		"draft_revision_id":  draftRev.RevisionID,
		"candidate_id":       review.CandidateID,
		"source_content_id":  review.SourceContentID,
		"style_id":           review.StyleID,
		"approval_state":     "approved_source_native_article_revision",
		"source_entities":    globalWireRuntimeSourceEntitiesWithPromotedItem(story, review, sourceItem),
	})
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            draftDoc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Global Wire",
		Content:          globalWireApprovedProjectionVTextContent(review, draftRev.Content),
		Citations:        draftRev.Citations,
		Metadata:         metadata,
		CreatedAt:        now,
		ParentRevisionID: draftDoc.CurrentRevisionID,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	updatedReview, err := h.rt.Store().MarkGlobalWireProjectionReviewApproved(r.Context(), ownerID, review.ID, draftDoc.DocID, storedRev.RevisionID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), draftDoc.DocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, types.GlobalWireStoryProjection{}, types.GlobalWireProjectionReview{}, err
	}
	return doc, storedRev, globalWireSourceNativeProjectionFromReview(updatedReview, doc, storedRev), updatedReview, nil
}

func globalWireSourceNativeProjectionFromReview(review types.GlobalWireProjectionReview, doc types.Document, rev types.Revision) types.GlobalWireStoryProjection {
	return types.GlobalWireStoryProjection{
		ID:          "global-wire-source-native-projection-" + review.ID,
		OwnerID:     review.OwnerID,
		StoryID:     review.StoryID,
		StyleID:     firstNonEmptyString(review.StyleID, "wire-style"),
		StyleDocID:  review.StyleDocID,
		StoryDocID:  doc.DocID,
		ContextJSON: `{"audience":"global-wire","task":"source_native_article_projection"}`,
		Text:        rev.Content,
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   rev.CreatedAt,
	}
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

func globalWireApprovedProjectionVTextContent(_ types.GlobalWireProjectionReview, draftContent string) string {
	content := strings.TrimSpace(draftContent)
	if content == "" {
		content = "The approved article revision was empty at approval time."
	}
	return content
}

func globalWireProjectionDraftVTextContent(review types.GlobalWireProjectionReview, story types.GlobalWireStory, sourceItem *types.ContentItem) string {
	projection := strings.TrimSpace(story.Projections[review.StyleID])
	if projection == "" {
		projection = strings.TrimSpace(story.Projections["wire-style"])
	}
	if projection == "" {
		projection = strings.TrimSpace(story.Dek)
	}
	sourceRef := globalWireLeadSourceRef(story)
	sourceBody := strings.TrimSpace(review.Rationale)
	if sourceItem != nil {
		sourceRef = globalWireRuntimeSourceRef(globalWireSourceItemFromProjectionReview(review, *sourceItem), 1)
		sourceBody = firstNonEmptyString(strings.TrimSpace(sourceItem.TextContent), sourceBody)
	}
	if sourceBody == "" {
		sourceBody = "The latest reviewed source changes the article's source neighborhood without resolving every open question."
	}
	if len(sourceBody) > 420 {
		sourceBody = strings.TrimSpace(sourceBody[:420]) + "..."
	}
	claimSentence := globalWireClaimSentence(story)
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		projection,
		"",
		"The newest revision is grounded in " + sourceRef + ". " + sourceBody,
	}
	if claimSentence != "" {
		lines = append(lines, "", claimSentence)
	}
	return strings.Join(lines, "\n")
}

func globalWireAutoradioScriptBody(story types.GlobalWireStory, artifact types.GlobalWirePublicationArtifact, sourceItem *types.ContentItem) string {
	sourceLabel := artifact.SourceContentID
	if sourceItem != nil && strings.TrimSpace(sourceItem.Title) != "" {
		sourceLabel = sourceItem.Title
	}
	if strings.TrimSpace(sourceLabel) == "" {
		sourceLabel = "StoryGraph source neighborhood"
	}
	lines := []string{
		"Autoradio script for \"" + story.Headline + "\".",
		"Open with the headline and name the evidence boundary: " + sourceLabel + ".",
		"",
		artifact.Body,
		"",
		fmt.Sprintf("Citations to name or display: %d.", len(artifact.CitationRefs)),
		fmt.Sprintf("Rollback refs to keep attached: %d.", len(artifact.RollbackRefs)),
		"Guardrail: speak only from this citeable publication artifact and preserve unresolved uncertainty.",
	}
	if story.Dek != "" {
		lines = append(lines[:1], append([]string{"Context: " + story.Dek}, lines[1:]...)...)
	}
	return strings.Join(lines, "\n")
}

func scriptIDForGlobalWireExport(script *types.GlobalWireAutoradioScript) string {
	if script == nil {
		return ""
	}
	return script.ID
}

func globalWirePublicationDeliveryExportBody(story types.GlobalWireStory, delivery types.GlobalWirePublicationDelivery, artifact types.GlobalWirePublicationArtifact, script *types.GlobalWireAutoradioScript, sourceItem *types.ContentItem) string {
	sourceLabel := artifact.SourceContentID
	if sourceItem != nil && strings.TrimSpace(sourceItem.Title) != "" {
		sourceLabel = sourceItem.Title
	}
	if strings.TrimSpace(sourceLabel) == "" {
		sourceLabel = "StoryGraph source neighborhood"
	}
	lines := []string{
		"# " + story.Headline,
		"",
		"Delivery: " + delivery.DeliveryRef,
		"Status: " + delivery.Status,
		"Channel: " + delivery.Channel,
		"Source: " + sourceLabel,
		"",
		"## Publication Artifact",
		"",
		artifact.Body,
		"",
		"## Provenance",
		"",
		fmt.Sprintf("- Artifact id: %s", artifact.ID),
		fmt.Sprintf("- Delivery id: %s", delivery.ID),
		fmt.Sprintf("- Citation count: %d", len(delivery.CitationRefs)),
		fmt.Sprintf("- Rollback count: %d", len(delivery.RollbackRefs)),
		"- Citation refs: " + strings.Join(delivery.CitationRefs, ", "),
		"- Rollback refs: " + strings.Join(delivery.RollbackRefs, ", "),
	}
	if script != nil {
		lines = append(lines,
			"",
			"## Autoradio Script",
			"",
			script.ScriptBody,
			"",
			fmt.Sprintf("- Autoradio script id: %s", script.ID),
		)
	}
	lines = append(lines,
		"",
		"Guardrail: this export is owner-scoped publication evidence, not an unauthenticated public permalink.",
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
		"Applies to: " + story.Headline,
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

func globalWireComposedStyleProjectionText(story types.GlobalWireStory, style types.GlobalWireStyleSource, _ []types.GlobalWireStyleSource) string {
	projection := strings.TrimSpace(story.Projections["wire-style"])
	if projection == "" {
		projection = strings.TrimSpace(story.Dek)
	}
	if strings.TrimSpace(style.Summary) == "" {
		return projection
	}
	return projection + " " + style.Summary
}

func (h *APIHandler) createGlobalWireComposedProjectionVText(r *http.Request, ownerID string, story types.GlobalWireStory, style types.GlobalWireStyleSource, baseStyles []types.GlobalWireStyleSource, projectionText string, now time.Time) (types.Document, types.Revision, error) {
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     story.Headline,
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
		"created_from":    "global_wire_composed_style_projection",
		"artifact_kind":   "article_revision",
		"article_version": true,
		"storygraph_id":   story.ID,
		"style_id":        style.ID,
		"style_doc_id":    style.DocID,
		"base_styles":     globalWireStyleIDs(baseStyles),
		"source_entities": globalWireRuntimeSourceEntities(story),
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

func globalWireComposedProjectionVTextContent(story types.GlobalWireStory, _ types.GlobalWireStyleSource, projectionText string) string {
	sourceRef := globalWireLeadSourceRef(story)
	claimSentence := globalWireClaimSentence(story)
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		projectionText,
		"",
		"The article keeps its lead evidence anchored to " + sourceRef + " while the editorial voice changes.",
	}
	if claimSentence != "" {
		lines = append(lines, "", claimSentence)
	}
	return strings.Join(lines, "\n")
}

func globalWireMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func globalWireEstimateSpokenDurationSeconds(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	seconds := (words * 60) / 155
	if seconds < 10 {
		return 10
	}
	return seconds
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

func (h *APIHandler) globalWireSourceDossiers(
	r *http.Request,
	ownerID string,
	storyID string,
	contributions []types.GlobalWireContribution,
	refreshes []types.GlobalWireSourceRefreshRun,
	claimRecords []types.GlobalWireClaimRecord,
	sourceReviewSignals []types.GlobalWireSourceReviewSignal,
	researchTasks []types.GlobalWireResearchTask,
	extractionArtifacts []types.GlobalWireExtractionArtifact,
	researchEvidence []types.GlobalWireResearchTaskEvidence,
	researchDecisions []types.GlobalWireResearchEvidenceDecision,
	candidates []types.GlobalWireGraphUpdateCandidate,
	publicationUpdates []types.GlobalWirePublicationUpdate,
	publicationArtifacts []types.GlobalWirePublicationArtifact,
	publicationDeliveries []types.GlobalWirePublicationDelivery,
	autoradioScripts []types.GlobalWireAutoradioScript,
	autoradioEpisodes []types.GlobalWireAutoradioEpisode,
	deliveryExports []types.GlobalWirePublicationDeliveryExport,
	publicLinks []types.GlobalWirePublicationPublicLink,
	newsletterIssues []types.GlobalWireNewsletterIssue,
	newsletterDeliveries []types.GlobalWireNewsletterDelivery,
	newsletterReceipts []types.GlobalWireNewsletterProviderReceipt,
) ([]globalWireSourceDossier, error) {
	var stories []types.GlobalWireStory
	if storyID != "" {
		story, err := h.rt.Store().GetGlobalWireStory(r.Context(), ownerID, storyID)
		if err != nil {
			return nil, err
		}
		stories = []types.GlobalWireStory{story}
	} else {
		var err error
		stories, err = h.rt.Store().ListGlobalWireStories(r.Context(), ownerID)
		if err != nil {
			return nil, err
		}
	}
	dossiers := make([]globalWireSourceDossier, 0, len(stories))
	for _, story := range stories {
		dossiers = append(dossiers, globalWireBuildSourceDossier(
			story,
			contributions,
			refreshes,
			claimRecords,
			sourceReviewSignals,
			researchTasks,
			extractionArtifacts,
			researchEvidence,
			researchDecisions,
			candidates,
			publicationUpdates,
			publicationArtifacts,
			publicationDeliveries,
			autoradioScripts,
			autoradioEpisodes,
			deliveryExports,
			publicLinks,
			newsletterIssues,
			newsletterDeliveries,
			newsletterReceipts,
		))
	}
	return dossiers, nil
}

func globalWireBuildSourceDossier(
	story types.GlobalWireStory,
	contributions []types.GlobalWireContribution,
	refreshes []types.GlobalWireSourceRefreshRun,
	claimRecords []types.GlobalWireClaimRecord,
	sourceReviewSignals []types.GlobalWireSourceReviewSignal,
	researchTasks []types.GlobalWireResearchTask,
	extractionArtifacts []types.GlobalWireExtractionArtifact,
	researchEvidence []types.GlobalWireResearchTaskEvidence,
	researchDecisions []types.GlobalWireResearchEvidenceDecision,
	candidates []types.GlobalWireGraphUpdateCandidate,
	publicationUpdates []types.GlobalWirePublicationUpdate,
	publicationArtifacts []types.GlobalWirePublicationArtifact,
	publicationDeliveries []types.GlobalWirePublicationDelivery,
	autoradioScripts []types.GlobalWireAutoradioScript,
	autoradioEpisodes []types.GlobalWireAutoradioEpisode,
	deliveryExports []types.GlobalWirePublicationDeliveryExport,
	publicLinks []types.GlobalWirePublicationPublicLink,
	newsletterIssues []types.GlobalWireNewsletterIssue,
	newsletterDeliveries []types.GlobalWireNewsletterDelivery,
	newsletterReceipts []types.GlobalWireNewsletterProviderReceipt,
) globalWireSourceDossier {
	dossier := globalWireSourceDossier{
		ID:            "global-wire-source-dossier:" + story.ID,
		StoryID:       story.ID,
		Headline:      story.Headline,
		SourceState:   story.SourceState,
		ManifestTiers: globalWireDossierManifestTiers(story.Manifest),
		ReviewState:   "source-dossier-ready",
	}
	dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"story:" + story.ID})
	for _, tier := range dossier.ManifestTiers {
		for _, sourceID := range tier.SourceIDs {
			dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"manifest_source:" + sourceID})
		}
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, tier.ContentIDs)
	}
	for _, contribution := range contributions {
		if contribution.StoryID != story.ID {
			continue
		}
		dossier.ContributionIDs = appendStringListIfMissing(dossier.ContributionIDs, []string{contribution.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{contribution.SourceContentID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"contribution:" + contribution.ID})
	}
	for _, refresh := range refreshes {
		if refresh.StoryID != story.ID {
			continue
		}
		dossier.RefreshRunIDs = appendStringListIfMissing(dossier.RefreshRunIDs, []string{refresh.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{refresh.SourceContentID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"refresh:" + refresh.ID})
	}
	for _, candidate := range candidates {
		if candidate.StoryID != story.ID {
			continue
		}
		dossier.CandidateIDs = appendStringListIfMissing(dossier.CandidateIDs, []string{candidate.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{candidate.SourceContentID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"candidate:" + candidate.ID})
	}
	for _, task := range researchTasks {
		if task.StoryID != story.ID {
			continue
		}
		dossier.ResearchTaskIDs = appendStringListIfMissing(dossier.ResearchTaskIDs, []string{task.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{task.SourceContentID})
	}
	for _, evidence := range researchEvidence {
		if evidence.StoryID != story.ID {
			continue
		}
		dossier.ResearchEvidenceIDs = appendStringListIfMissing(dossier.ResearchEvidenceIDs, []string{evidence.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{evidence.SourceContentID})
	}
	for _, extraction := range extractionArtifacts {
		if extraction.StoryID != story.ID {
			continue
		}
		dossier.ExtractionIDs = appendStringListIfMissing(dossier.ExtractionIDs, []string{extraction.ID})
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{extraction.SourceContentID})
		dossier.EntityTerms = appendStringListIfMissing(dossier.EntityTerms, extraction.Entities)
		dossier.EventTerms = appendStringListIfMissing(dossier.EventTerms, extraction.Events)
		dossier.Timeline = appendStringListIfMissing(dossier.Timeline, extraction.Timeline)
	}
	for _, claim := range claimRecords {
		if claim.StoryID != story.ID {
			continue
		}
		dossier.ClaimDossiers = append(dossier.ClaimDossiers, globalWireDossierClaimForRecord(claim, sourceReviewSignals, researchTasks, extractionArtifacts, researchEvidence, researchDecisions, publicationUpdates))
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{claim.SourceContentID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"claim:" + claim.ID})
	}
	for _, signal := range sourceReviewSignals {
		if signal.StoryID != story.ID {
			continue
		}
		dossier.SourceReviewSignals = append(dossier.SourceReviewSignals, signal)
		dossier.SourceContentIDs = appendStringListIfMissing(dossier.SourceContentIDs, []string{signal.SourceContentID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, []string{"source_review_signal:" + signal.ID})
		dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, signal.EvidenceRefs)
	}
	dossier.PublicationRefs = globalWireBuildDossierPublicationRefs(story.ID, publicationUpdates, publicationArtifacts, publicationDeliveries, autoradioScripts, autoradioEpisodes, deliveryExports, publicLinks, newsletterIssues, newsletterDeliveries, newsletterReceipts)
	dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, dossier.PublicationRefs.CitationRefs)
	dossier.ProvenanceRefs = appendStringListIfMissing(dossier.ProvenanceRefs, dossier.PublicationRefs.RollbackRefs)
	dossier.MissingFields = globalWireDossierMissingFields(dossier)
	return dossier
}

func globalWireDossierManifestTiers(manifest types.GlobalWireSourceManifest) []globalWireDossierManifestTier {
	return []globalWireDossierManifestTier{
		globalWireDossierManifestTierForSources("lead", manifest.Lead),
		globalWireDossierManifestTierForSources("supporting", manifest.Supporting),
		globalWireDossierManifestTierForSources("contrary", manifest.Contrary),
		globalWireDossierManifestTierForSources("context", manifest.Context),
	}
}

func globalWireDossierManifestTierForSources(tier string, sources []types.GlobalWireSourceItem) globalWireDossierManifestTier {
	out := globalWireDossierManifestTier{Tier: tier, Count: len(sources)}
	for _, source := range sources {
		out.SourceIDs = appendStringListIfMissing(out.SourceIDs, []string{source.ID})
		out.ContentIDs = appendStringListIfMissing(out.ContentIDs, []string{source.ContentID})
		out.Titles = appendStringListIfMissing(out.Titles, []string{source.Title})
	}
	return out
}

func globalWireDossierClaimForRecord(
	claim types.GlobalWireClaimRecord,
	sourceReviewSignals []types.GlobalWireSourceReviewSignal,
	researchTasks []types.GlobalWireResearchTask,
	extractionArtifacts []types.GlobalWireExtractionArtifact,
	researchEvidence []types.GlobalWireResearchTaskEvidence,
	researchDecisions []types.GlobalWireResearchEvidenceDecision,
	publicationUpdates []types.GlobalWirePublicationUpdate,
) globalWireDossierClaim {
	out := globalWireDossierClaim{
		ClaimID:          claim.ID,
		ClaimText:        claim.ClaimText,
		ClaimKind:        claim.ClaimKind,
		Status:           claim.Status,
		UncertaintyState: claim.UncertaintyState,
		DisputeState:     claim.DisputeState,
		EvidenceGap:      claim.EvidenceGap,
		SourceContentID:  claim.SourceContentID,
		CandidateID:      claim.CandidateID,
		ContributionID:   claim.ContributionID,
		RefreshID:        claim.RefreshID,
	}
	for _, extraction := range extractionArtifacts {
		if extraction.ClaimID == claim.ID {
			out.ExtractionIDs = appendStringListIfMissing(out.ExtractionIDs, []string{extraction.ID})
		}
	}
	for _, signal := range sourceReviewSignals {
		if signal.ClaimID == claim.ID {
			out.SourceReviewSignalIDs = appendStringListIfMissing(out.SourceReviewSignalIDs, []string{signal.ID})
		}
	}
	for _, task := range researchTasks {
		if task.ClaimID == claim.ID {
			out.ResearchTaskIDs = appendStringListIfMissing(out.ResearchTaskIDs, []string{task.ID})
		}
	}
	for _, evidence := range researchEvidence {
		if evidence.ClaimID == claim.ID {
			out.ResearchEvidenceIDs = appendStringListIfMissing(out.ResearchEvidenceIDs, []string{evidence.ID})
		}
	}
	for _, decision := range researchDecisions {
		if decision.ClaimID == claim.ID {
			out.ResearchDecisionIDs = appendStringListIfMissing(out.ResearchDecisionIDs, []string{decision.ID})
		}
	}
	for _, update := range publicationUpdates {
		if stringListContainsAny(update.ExtractionIDs, out.ExtractionIDs) ||
			stringListContains(out.ResearchEvidenceIDs, update.EvidenceID) {
			out.PublicationUpdateIDs = appendStringListIfMissing(out.PublicationUpdateIDs, []string{update.ID})
		}
	}
	return out
}

func stringListContains(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func stringListContainsAny(values []string, targets []string) bool {
	for _, target := range targets {
		if stringListContains(values, target) {
			return true
		}
	}
	return false
}

func globalWireBuildDossierPublicationRefs(
	storyID string,
	publicationUpdates []types.GlobalWirePublicationUpdate,
	publicationArtifacts []types.GlobalWirePublicationArtifact,
	publicationDeliveries []types.GlobalWirePublicationDelivery,
	autoradioScripts []types.GlobalWireAutoradioScript,
	autoradioEpisodes []types.GlobalWireAutoradioEpisode,
	deliveryExports []types.GlobalWirePublicationDeliveryExport,
	publicLinks []types.GlobalWirePublicationPublicLink,
	newsletterIssues []types.GlobalWireNewsletterIssue,
	newsletterDeliveries []types.GlobalWireNewsletterDelivery,
	newsletterReceipts []types.GlobalWireNewsletterProviderReceipt,
) globalWireDossierPublicationRefs {
	var out globalWireDossierPublicationRefs
	for _, update := range publicationUpdates {
		if update.StoryID != storyID {
			continue
		}
		out.UpdateIDs = appendStringListIfMissing(out.UpdateIDs, []string{update.ID})
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, update.RollbackRefs)
	}
	for _, artifact := range publicationArtifacts {
		if artifact.StoryID != storyID {
			continue
		}
		out.ArtifactIDs = appendStringListIfMissing(out.ArtifactIDs, []string{artifact.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, artifact.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, artifact.RollbackRefs)
	}
	for _, delivery := range publicationDeliveries {
		if delivery.StoryID != storyID {
			continue
		}
		out.DeliveryIDs = appendStringListIfMissing(out.DeliveryIDs, []string{delivery.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, delivery.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, delivery.RollbackRefs)
	}
	for _, script := range autoradioScripts {
		if script.StoryID != storyID {
			continue
		}
		out.AutoradioScriptIDs = appendStringListIfMissing(out.AutoradioScriptIDs, []string{script.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, script.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, script.RollbackRefs)
	}
	for _, episode := range autoradioEpisodes {
		if episode.StoryID != storyID {
			continue
		}
		out.AutoradioEpisodeIDs = appendStringListIfMissing(out.AutoradioEpisodeIDs, []string{episode.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, episode.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, episode.RollbackRefs)
	}
	for _, export := range deliveryExports {
		if export.StoryID != storyID {
			continue
		}
		out.DeliveryExportIDs = appendStringListIfMissing(out.DeliveryExportIDs, []string{export.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, export.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, export.RollbackRefs)
	}
	for _, link := range publicLinks {
		if link.StoryID != storyID {
			continue
		}
		out.PublicLinkIDs = appendStringListIfMissing(out.PublicLinkIDs, []string{link.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, link.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, link.RollbackRefs)
	}
	for _, issue := range newsletterIssues {
		if issue.StoryID != storyID {
			continue
		}
		out.NewsletterIssueIDs = appendStringListIfMissing(out.NewsletterIssueIDs, []string{issue.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, issue.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, issue.RollbackRefs)
	}
	for _, delivery := range newsletterDeliveries {
		if delivery.StoryID != storyID {
			continue
		}
		out.NewsletterDeliveryIDs = appendStringListIfMissing(out.NewsletterDeliveryIDs, []string{delivery.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, delivery.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, delivery.RollbackRefs)
	}
	for _, receipt := range newsletterReceipts {
		if receipt.StoryID != storyID {
			continue
		}
		out.NewsletterReceiptIDs = appendStringListIfMissing(out.NewsletterReceiptIDs, []string{receipt.ID})
		out.CitationRefs = appendStringListIfMissing(out.CitationRefs, receipt.CitationRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, receipt.RollbackRefs)
		out.RollbackRefs = appendStringListIfMissing(out.RollbackRefs, receipt.EventRefs)
	}
	return out
}

func globalWireDossierMissingFields(dossier globalWireSourceDossier) []string {
	missing := []string{}
	if len(dossier.ClaimDossiers) == 0 {
		missing = append(missing, "claim_dossiers")
	}
	if len(dossier.SourceReviewSignals) == 0 {
		missing = append(missing, "source_review_signals")
	}
	if len(dossier.ExtractionIDs) == 0 {
		missing = append(missing, "extraction_overlays")
	}
	if len(dossier.ResearchTaskIDs) == 0 {
		missing = append(missing, "research_tasks")
	}
	if len(dossier.PublicationRefs.ArtifactIDs) == 0 {
		missing = append(missing, "publication_artifacts")
	}
	if len(dossier.PublicationRefs.NewsletterIssueIDs) == 0 {
		missing = append(missing, "newsletter_issues")
	}
	if len(dossier.PublicationRefs.NewsletterReceiptIDs) == 0 {
		missing = append(missing, "newsletter_provider_receipts")
	}
	if len(dossier.PublicationRefs.CitationRefs) == 0 {
		missing = append(missing, "citation_refs")
	}
	return missing
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
	metadataFields := map[string]any{
		"schema":               "choir.global_wire_source_service_item.v1",
		"target_kind":          firstNonEmptyString(stringValue(result["target_kind"]), sourceapi.TargetKind),
		"source_item_id":       itemID,
		"source_id":            stringValue(result["source_id"]),
		"source_type":          stringValue(result["source_type"]),
		"fetch_id":             stringValue(result["fetch_id"]),
		"original_id":          stringValue(result["original_id"]),
		"published_at":         stringValue(result["published_at"]),
		"fetched_at":           stringValue(result["fetched_at"]),
		"verticals":            result["verticals"],
		"language":             stringValue(result["language"]),
		"region":               stringValue(result["region"]),
		"body_kind":            stringValue(result["body_kind"]),
		"body_length":          result["body_length"],
		"reader_snapshot":      result["reader_snapshot"],
		"source_tos_class":     stringValue(result["source_tos_class"]),
		"source_robots_policy": stringValue(result["source_robots_policy"]),
		"source_auth_policy":   stringValue(result["source_auth_policy"]),
		"store_body_policy":    stringValue(result["store_body_policy"]),
		"evidence_level":       stringValue(result["evidence_level"]),
		"vintage_policy":       stringValue(result["vintage_policy"]),
		"lookahead_status":     stringValue(result["lookahead_status"]),
		"release_date":         stringValue(result["release_date"]),
		"research_use":         "pending-reconciliation-review",
	}
	readerSnapshot := boolValue(result["reader_snapshot"])
	sourceURL := firstNonEmptyString(stringValue(result["canonical_url"]), stringValue(result["url"]))
	if !readerSnapshot {
		enrichedBody, enrichedHash := body, firstNonEmptyString(stringValue(result["content_hash"]), contentHash(body))
		h.enrichGlobalWireSourceServiceReaderSnapshot(r, ownerID, sourceURL, body, result, metadataFields, &enrichedBody, &enrichedHash)
		body = enrichedBody
		result["content_hash"] = enrichedHash
	}
	metadata, err := json.Marshal(metadataFields)
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

func (h *APIHandler) enrichGlobalWireSourceServiceReaderSnapshot(r *http.Request, ownerID, sourceURL, currentBody string, result map[string]any, metadata map[string]any, bodyOut *string, hashOut *string) {
	bodyKind := strings.TrimSpace(stringValue(result["body_kind"]))
	policy := strings.TrimSpace(stringValue(result["store_body_policy"]))
	if !globalWireShouldAttemptReaderSnapshot(bodyKind, policy, sourceURL) {
		metadata["reader_snapshot_status"] = globalWireReaderSnapshotSkipReason(bodyKind, policy, sourceURL)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	imported, err := h.rt.ImportURLContent(ctx, ownerID, sourceURL, "")
	if err != nil {
		metadata["reader_snapshot_status"] = "fetch_failed"
		metadata["reader_snapshot_error"] = truncateString(err.Error(), 240)
		return
	}
	importedText := strings.TrimSpace(imported.TextContent)
	if len(importedText) < 400 || len(importedText) <= len(strings.TrimSpace(currentBody))*2 {
		metadata["reader_snapshot_status"] = "low_content"
		metadata["reader_snapshot_content_id"] = imported.ContentID
		metadata["reader_snapshot_content_chars"] = len(importedText)
		return
	}
	*bodyOut = importedText
	*hashOut = firstNonEmptyString(imported.ContentHash, contentHash(importedText))
	result["body_kind"] = "reader_snapshot"
	result["body_length"] = len([]rune(importedText))
	result["reader_snapshot"] = true
	metadata["body_kind"] = "reader_snapshot"
	metadata["body_length"] = len([]rune(importedText))
	metadata["reader_snapshot"] = true
	metadata["reader_snapshot_status"] = "imported"
	metadata["reader_snapshot_content_id"] = imported.ContentID
	metadata["reader_snapshot_content_hash"] = imported.ContentHash
	metadata["reader_snapshot_source_type"] = imported.SourceType
	metadata["reader_snapshot_media_type"] = imported.MediaType
}

func globalWireShouldAttemptReaderSnapshot(bodyKind, policy, sourceURL string) bool {
	if strings.TrimSpace(sourceURL) == "" {
		return false
	}
	switch strings.TrimSpace(policy) {
	case "bounded_text", "bounded_release_text":
	default:
		return false
	}
	switch strings.TrimSpace(bodyKind) {
	case "", "empty", "feed_summary":
		return true
	default:
		return false
	}
}

func globalWireReaderSnapshotSkipReason(bodyKind, policy, sourceURL string) string {
	if strings.TrimSpace(sourceURL) == "" {
		return "skipped_no_url"
	}
	switch strings.TrimSpace(policy) {
	case "bounded_text", "bounded_release_text":
	default:
		return "skipped_store_body_policy"
	}
	switch strings.TrimSpace(bodyKind) {
	case "", "empty", "feed_summary":
		return "eligible"
	default:
		return "skipped_body_kind"
	}
}
