package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type universalWireStoriesResponse struct {
	Stories      []types.WireStory             `json:"stories"`
	StyleSources []types.WireStyleSource       `json:"style_sources"`
	Source       string                        `json:"source"`
	Edition      *universalWireEditionResponse `json:"edition,omitempty"`
	Diagnostics  *universalWireFeedDiagnostics `json:"diagnostics,omitempty"`
}

type universalWireEditionResponse struct {
	DocID          string   `json:"doc_id"`
	RevisionID     string   `json:"revision_id"`
	SourcePath     string   `json:"source_path"`
	Title          string   `json:"title"`
	IncludedDocIDs []string `json:"included_doc_ids"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

type universalWireFeedDiagnostics struct {
	Status     string                                 `json:"status"`
	Summary    string                                 `json:"summary"`
	Substrates []universalWireFeedSubstrateDiagnostic `json:"substrates"`
}

type universalWireFeedSubstrateDiagnostic struct {
	Substrate      string `json:"substrate"`
	State          string `json:"state"`
	CandidateCount int    `json:"candidate_count"`
	StoryCount     int    `json:"story_count"`
	FilteredCount  int    `json:"filtered_count,omitempty"`
	Reason         string `json:"reason"`
}

const universalWireEditionSourcePath = "universal-wire/Wire.texture"

var textureTransclusionRefRE = regexp.MustCompile(`texture:([A-Za-z0-9_.:-]{1,160})`)

func (h *APIHandler) HandleUniversalWireStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	_, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	stories := []types.WireStory{}
	styleSources := []types.WireStyleSource{}
	source := "universal-wire-texture-index"
	var edition *universalWireEditionResponse
	diagnostics := universalWireFeedDiagnostics{
		Status:  "empty",
		Summary: "Universal Wire found no publishable Texture synthesis articles.",
	}
	editionStories, editionResp, editionErr := h.universalWireEditionTextureStories(r.Context(), styleSources, 12)
	if editionErr == nil && len(editionStories) == 0 {
		if synthesis, err := h.rt.synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures(r.Context(), time.Now().UTC()); err != nil {
			log.Printf("universal wire: graph capture materialization unavailable: %v", err)
		} else if synthesis.Triggered {
			editionStories, editionResp, editionErr = h.universalWireEditionTextureStories(r.Context(), styleSources, 12)
		}
	}
	if editionErr == nil && len(editionStories) > 0 && universalWireStoriesNeedArticleSurfaceRepair(editionStories) && h != nil && h.rt != nil {
		repaired, err := h.repairUniversalWireEditionArticleSurfaces(r.Context(), editionStories, time.Now().UTC())
		if err != nil {
			log.Printf("universal wire: article surface direct repair unavailable: %v", err)
		}
		if repaired {
			if refreshedStories, refreshedEdition, refreshedErr := h.universalWireEditionTextureStories(r.Context(), styleSources, 12); refreshedErr == nil {
				editionStories, editionResp = refreshedStories, refreshedEdition
			} else {
				log.Printf("universal wire: article surface direct repair reload unavailable: %v", refreshedErr)
			}
		}
		if !repaired {
			if synthesis, err := h.rt.synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures(r.Context(), time.Now().UTC()); err != nil {
				log.Printf("universal wire: article surface repair unavailable: %v", err)
			} else if synthesis.Triggered {
				if refreshedStories, refreshedEdition, refreshedErr := h.universalWireEditionTextureStories(r.Context(), styleSources, 12); refreshedErr == nil {
					editionStories, editionResp = refreshedStories, refreshedEdition
				} else {
					log.Printf("universal wire: article surface repair reload unavailable: %v", refreshedErr)
				}
			}
		}
	}
	if editionErr == nil {
		edition = editionResp
		diagnostics.Substrates = append(diagnostics.Substrates, universalWireEditionDiagnostic(editionResp, len(editionStories)))
		if len(editionStories) > 0 {
			stories = editionStories
			source = "universal-wire-edition-texture"
		} else if editionResp != nil {
			source = "universal-wire-edition-texture"
		}
	} else {
		log.Printf("universal wire: edition unavailable: %v", editionErr)
		diagnostics.Substrates = append(diagnostics.Substrates, universalWireFeedSubstrateDiagnostic{
			Substrate: "texture_edition",
			State:     "unavailable",
			Reason:    "Texture edition state could not be read through the public Wire route.",
		})
	}
	if len(stories) == 0 {
		if captureStories, captureDiagnostic, err := h.universalWireWebCaptureStories(r.Context(), 12); err != nil {
			log.Printf("universal wire: web capture graph unavailable: %v", err)
			diagnostics.Substrates = append(diagnostics.Substrates, universalWireFeedSubstrateDiagnostic{
				Substrate: "web_capture_graph",
				State:     "unavailable",
				Reason:    "Graph-backed web capture state could not be read through the public Wire route.",
			})
		} else {
			if len(captureStories) > 0 {
				captureDiagnostic.State = "diagnostic_only"
				captureDiagnostic.Reason = "Graph-backed web captures are available, but Universal Wire does not publish raw capture projections as articles; Texture synthesis has not published an edition yet."
			}
			diagnostics.Substrates = append(diagnostics.Substrates, captureDiagnostic)
		}
	}
	if len(stories) == 0 {
		diagnostics.Substrates = append(diagnostics.Substrates, universalWireFeedSubstrateDiagnostic{
			Substrate: "source_provenance",
			State:     "not_applicable",
			Reason:    "No Texture synthesis article is available for source citation provenance; raw capture provenance remains diagnostic substrate only.",
		})
	}
	for i := range stories {
		stories[i] = normalizeWireStoryPresentation(stories[i])
	}
	var emptyDiagnostics *universalWireFeedDiagnostics
	if len(stories) == 0 {
		emptyDiagnostics = &diagnostics
	}
	writeAPIJSON(w, http.StatusOK, universalWireStoriesResponse{
		Stories:      stories,
		StyleSources: styleSources,
		Source:       source,
		Edition:      edition,
		Diagnostics:  emptyDiagnostics,
	})
}

func (h *APIHandler) repairUniversalWireEditionArticleSurfaces(ctx context.Context, stories []types.WireStory, now time.Time) (bool, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return false, nil
	}
	repaired := false
	ownerID := universalWirePlatformOwnerID()
	for _, story := range stories {
		if !universalWireStoryNeedsArticleSurfaceRepair(story) || strings.TrimSpace(story.StoryTextureDoc) == "" {
			continue
		}
		doc, err := h.rt.Store().GetDocument(ctx, story.StoryTextureDoc, ownerID)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return repaired, err
		}
		if strings.TrimSpace(doc.CurrentRevisionID) == "" {
			continue
		}
		rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, ownerID)
		if err != nil {
			if err == store.ErrNotFound {
				continue
			}
			return repaired, err
		}
		if !universalWireRevisionNeedsArticleSurfaceRepair(rev) {
			continue
		}
		ok, err := h.rt.repairUniversalWireSynthesisArticleFromRevision(ctx, doc, rev, now)
		if err != nil {
			return repaired, err
		}
		repaired = repaired || ok
	}
	return repaired, nil
}

func (rt *Runtime) repairUniversalWireSynthesisArticleFromRevision(ctx context.Context, doc types.Document, rev types.Revision, now time.Time) (bool, error) {
	if rt == nil || rt.store == nil {
		return false, nil
	}
	if !wireRevisionIsUniversalWireSynthesis(rev) || !universalWireRevisionNeedsArticleSurfaceRepair(rev) {
		return false, nil
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	clusterID := firstNonEmpty(
		metadataString(meta, "universal_wire_story_cluster_id"),
		metadataString(meta, "source_network_cycle_id"),
		metadataString(meta, "ingestion_handoff_cycle_id"),
	)
	if clusterID == "" {
		return false, nil
	}
	sources := universalWireSynthesisSourcesFromTextureRevision(rev)
	if len(sources) < 2 {
		return false, nil
	}
	if _, _, _, err := rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
		ClusterID: clusterID,
		Headline:  universalWireSynthesisHeadline(sources),
		Sources:   sources,
		Now:       now,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func universalWireSynthesisSourcesFromTextureRevision(rev types.Revision) []universalWireSynthesisSource {
	entities := wireArticleVisibleStructuredSourceEntities(rev)
	if len(entities) == 0 && len(strings.TrimSpace(string(rev.SourceEntities))) > 0 {
		var structured []texturedoc.SourceEntity
		if err := json.Unmarshal(rev.SourceEntities, &structured); err == nil {
			for _, entity := range structured {
				entities = append(entities, provenanceSourceEntityFromStructured(entity))
			}
		}
	}
	sources := make([]universalWireSynthesisSource, 0, len(entities))
	for _, entity := range entities {
		source, ok := universalWireSynthesisSourceFromTextureSourceEntity(entity)
		if !ok {
			continue
		}
		sources = append(sources, source)
	}
	return normalizedUniversalWireSynthesisSources(sources)
}

func universalWireSynthesisSourceFromTextureSourceEntity(entity textureSourceEntity) (universalWireSynthesisSource, bool) {
	snapshot := wireSourceEntityReaderSnapshot(entity)
	body := ""
	if snapshot != nil {
		body = strings.TrimSpace(snapshot.TextContent)
	}
	title := firstNonEmpty(
		strings.TrimSpace(entity.Label),
		metadataString(entity.ReaderSnapshot, "source_title"),
		strings.TrimSpace(entity.Target.CanonicalURL),
		strings.TrimSpace(entity.Target.URL),
		wireArticleSourceEntityManifestID(entity),
	)
	itemID := wireArticleSourceEntityManifestID(entity)
	canonicalURL := firstNonEmpty(strings.TrimSpace(entity.Target.CanonicalURL), strings.TrimSpace(entity.Target.URL))
	if title == "" || body == "" || itemID == "" {
		return universalWireSynthesisSource{}, false
	}
	var fetchedAt time.Time
	for _, key := range []string{"fetched_at", "captured_at"} {
		if raw := metadataString(entity.ReaderSnapshot, key); raw != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				fetchedAt = parsed.UTC()
				break
			}
		}
	}
	return universalWireSynthesisSource{
		ItemID:       itemID,
		SourceID:     strings.TrimSpace(entity.Target.SourceID),
		FetchID:      strings.TrimSpace(entity.Target.FetchID),
		Title:        title,
		URL:          strings.TrimSpace(entity.Target.URL),
		CanonicalURL: canonicalURL,
		Language:     metadataString(entity.ReaderSnapshot, "language"),
		Body:         body,
		FetchedAt:    fetchedAt,
	}, true
}

func universalWireRevisionNeedsArticleSurfaceRepair(rev types.Revision) bool {
	if !universalWireRevisionTextNeedsArticleSurfaceRepair(rev.Content) {
		return false
	}
	return wireRevisionIsUniversalWireSynthesis(rev)
}

func universalWireRevisionTextNeedsArticleSurfaceRepair(text string) bool {
	return strings.Contains(text, "Universal Wire live synthesis:") ||
		strings.Contains(text, "Universal Wire selected ") ||
		strings.Contains(text, "graph-backed source captures") ||
		strings.Contains(text, "Universal Wire treats") ||
		strings.Contains(text, "incoming reports point to the same developing story") ||
		strings.Contains(text, "A second source in the cluster") ||
		strings.Contains(text, "reports read as one developing article")
}

func universalWireStoryNeedsArticleSurfaceRepair(story types.WireStory) bool {
	text := strings.Join([]string{story.Headline, story.Dek, story.TextureContent}, "\n")
	return universalWireRevisionTextNeedsArticleSurfaceRepair(text)
}

func (h *APIHandler) universalWireEditionTextureStories(ctx context.Context, styleSources []types.WireStyleSource, limit int) ([]types.WireStory, *universalWireEditionResponse, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return nil, nil, nil
	}
	platformOwner := universalWirePlatformOwnerID()
	editionDocID, err := h.rt.Store().GetDocumentAlias(ctx, platformOwner, universalWireEditionSourcePath)
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
		return nil, &universalWireEditionResponse{
			DocID:      editionDoc.DocID,
			SourcePath: universalWireEditionSourcePath,
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
	includedDocIDs := universalWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID)
	edition := &universalWireEditionResponse{
		DocID:          editionDoc.DocID,
		RevisionID:     editionRev.RevisionID,
		SourcePath:     universalWireEditionSourcePath,
		Title:          editionDoc.Title,
		IncludedDocIDs: includedDocIDs,
		UpdatedAt:      editionDoc.UpdatedAt.Format(time.RFC3339Nano),
	}
	stories := make([]types.WireStory, 0, min(len(includedDocIDs), limit))
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
		story, ok := wireArticleTextureStoryFromCurrentRevision(ctx, doc, rev, styleSources)
		if !ok {
			continue
		}
		story.SemanticStory = h.universalWireStorySemanticState(ctx, rev)
		if h.platformdStoryVerificationEnabled() &&
			!h.platformdHasPublishedTexture(ctx, story.StoryTextureDoc, doc.CurrentRevisionID) {
			continue
		}
		story.Prominence = 100 - len(stories)
		story.SourceState = "universal-wire-edition-texture"
		stories = append(stories, story)
	}
	return stories, edition, nil
}

func (h *APIHandler) universalWireStorySemanticState(ctx context.Context, rev types.Revision) *types.WireStorySemanticState {
	meta := decodeRevisionMetadata(rev.Metadata)
	clusterObjectID := metadataString(meta, "universal_wire_story_cluster_object_id")
	if h != nil && h.rt != nil && h.rt.ObjectGraph() != nil && clusterObjectID != "" {
		if obj, err := h.rt.ObjectGraph().GetObject(ctx, clusterObjectID); err == nil {
			var state universalWireSemanticStoryState
			if err := json.Unmarshal(obj.Body, &state); err == nil && state.StoryID != "" {
				return wireStorySemanticStateFromClusterState(state)
			}
		}
	}
	storyID := metadataString(meta, "universal_wire_semantic_story_id")
	changeType := metadataString(meta, "universal_wire_semantic_change_type")
	if storyID == "" && changeType == "" {
		return wireStoryLegacySemanticStateFromMetadata(meta)
	}
	count := wireMetadataInt(meta["synthesis_source_count"])
	return &types.WireStorySemanticState{
		StoryID:             storyID,
		ChangeType:          changeType,
		CurrentSourceCount:  count,
		SourceCount:         count,
		PreviousSourceCount: 0,
	}
}

func wireStoryLegacySemanticStateFromMetadata(meta map[string]any) *types.WireStorySemanticState {
	if meta == nil {
		return nil
	}
	clusterID := firstNonEmpty(
		metadataString(meta, "universal_wire_story_cluster_id"),
		metadataString(meta, "source_network_cycle_id"),
		metadataString(meta, "ingestion_handoff_cycle_id"),
	)
	if clusterID == "" {
		return nil
	}
	isWireSynthesis := metadataBoolValue(meta, "universal_wire_synthesis") ||
		metadataString(meta, "ingestion_handoff_request_kind") == "synthesis_cluster" ||
		metadataString(meta, "universal_wire_article_alias_path") != ""
	if !isWireSynthesis {
		return nil
	}
	sourceItemIDs := metadataStringSliceValue(meta, "source_item_ids")
	count := wireMetadataInt(meta["synthesis_source_count"])
	if count == 0 {
		count = len(sourceItemIDs)
	}
	signature := append([]string{clusterID}, sourceItemIDs...)
	return &types.WireStorySemanticState{
		SchemaVersion:       "choir.universal_wire_story_cluster.semantic.legacy.v1",
		WorldModelKind:      "universal_wire_semantic_story",
		StoryID:             stableSourceEntityID("universal_wire_semantic_story", strings.Join(signature, "|")),
		ChangeType:          "legacy_revision_projection",
		SemanticSignature:   signature,
		PreviousSourceCount: 0,
		CurrentSourceCount:  count,
		SourceCount:         count,
	}
}

func wireStorySemanticStateFromClusterState(state universalWireSemanticStoryState) *types.WireStorySemanticState {
	if state.StoryID == "" {
		return nil
	}
	return &types.WireStorySemanticState{
		SchemaVersion:       state.SchemaVersion,
		WorldModelKind:      state.WorldModelKind,
		StoryID:             state.StoryID,
		ChangeType:          state.LatestChange.ChangeType,
		SemanticSignature:   append([]string(nil), state.SemanticSignature...),
		TopicConcepts:       append([]string(nil), state.TopicConcepts...),
		SignalConcepts:      append([]string(nil), state.SignalConcepts...),
		PreviousSourceCount: state.LatestChange.PreviousSourceCount,
		CurrentSourceCount:  state.LatestChange.CurrentSourceCount,
		SourceCount:         state.SourceCount,
		ChangedAt:           state.LatestChange.ChangedAt,
	}
}

func wireMetadataInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

func (h *APIHandler) universalWireWebCaptureStories(ctx context.Context, limit int) ([]types.WireStory, universalWireFeedSubstrateDiagnostic, error) {
	diagnostic := universalWireFeedSubstrateDiagnostic{
		Substrate: "web_capture_graph",
		State:     "unavailable",
		Reason:    "Object graph state is not available for this runtime.",
	}
	if h == nil || h.rt == nil {
		return nil, diagnostic, nil
	}
	graph := h.rt.ObjectGraph()
	if graph == nil {
		return nil, diagnostic, nil
	}
	notTombstoned := false
	objects, err := graph.ListObjects(ctx, objectgraph.ListFilter{
		Kind:      objectgraph.WebCaptureObjectKind,
		OwnerID:   universalWirePlatformOwnerID(),
		Limit:     limit,
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return nil, diagnostic, err
	}
	diagnostic.State = "empty"
	diagnostic.CandidateCount = len(objects)
	diagnostic.Reason = "No non-tombstoned choir.web_capture objects were found for the Universal Wire platform."
	if len(objects) == 0 {
		tombstoned := true
		if filtered, err := graph.ListObjects(ctx, objectgraph.ListFilter{
			Kind:      objectgraph.WebCaptureObjectKind,
			OwnerID:   universalWirePlatformOwnerID(),
			Limit:     limit,
			Tombstone: &tombstoned,
		}); err == nil && len(filtered) > 0 {
			diagnostic.State = "filtered"
			diagnostic.FilteredCount = len(filtered)
			diagnostic.Reason = "Only tombstoned choir.web_capture objects were found for the Universal Wire platform."
		}
	}
	stories := make([]types.WireStory, 0, len(objects))
	for _, obj := range objects {
		story, ok := wireStoryFromWebCaptureObject(obj)
		if !ok {
			continue
		}
		sourceContext, err := wireCaptureSourceEntityContext(ctx, graph, obj)
		if err != nil {
			return nil, diagnostic, err
		}
		story.Manifest.Context = append(story.Manifest.Context, sourceContext...)
		story.Prominence = 100 - len(stories)
		stories = append(stories, story)
	}
	diagnostic.StoryCount = len(stories)
	switch {
	case len(stories) > 0:
		diagnostic.State = "available"
		diagnostic.Reason = "Non-tombstoned graph-backed web captures produced Wire cards."
	case len(objects) > 0:
		diagnostic.State = "filtered"
		diagnostic.FilteredCount = len(objects)
		diagnostic.Reason = "Graph-backed web capture candidates were present, but none had publishable metadata and extracted text for a Wire card."
	}
	return stories, diagnostic, nil
}

func universalWireEditionDiagnostic(edition *universalWireEditionResponse, storyCount int) universalWireFeedSubstrateDiagnostic {
	diagnostic := universalWireFeedSubstrateDiagnostic{
		Substrate:  "texture_edition",
		StoryCount: storyCount,
	}
	switch {
	case edition == nil:
		diagnostic.State = "missing"
		diagnostic.Reason = "No Universal Wire Texture edition alias is present."
	case storyCount > 0:
		diagnostic.State = "available"
		diagnostic.CandidateCount = len(edition.IncludedDocIDs)
		diagnostic.Reason = "The Universal Wire Texture edition produced publishable story cards."
	case len(edition.IncludedDocIDs) > 0:
		diagnostic.State = "filtered"
		diagnostic.CandidateCount = len(edition.IncludedDocIDs)
		diagnostic.FilteredCount = len(edition.IncludedDocIDs)
		diagnostic.Reason = "The Universal Wire Texture edition exists, but no transcluded Texture story is currently publishable."
	default:
		diagnostic.State = "empty"
		diagnostic.Reason = "The Universal Wire Texture edition exists but does not transclude any Texture stories."
	}
	return diagnostic
}

func wireStoryFromWebCaptureObject(obj objectgraph.Object) (types.WireStory, bool) {
	metadata, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return types.WireStory{}, false
	}
	body := strings.TrimSpace(string(obj.Body))
	if body == "" {
		return types.WireStory{}, false
	}
	headline := firstNonEmpty(metadata.Title, metadata.CanonicalURL, metadata.URL, "Graph-backed web capture")
	canonicalURL := firstNonEmpty(metadata.CanonicalURL, metadata.URL)
	fetchedAt := obj.UpdatedAt
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(metadata.FetchedAt)); err == nil {
		fetchedAt = parsed
	}
	dek := truncateRunes(firstNonEmpty(firstWireCaptureParagraph(body), canonicalURL), 220)
	projection := truncateRunes(body, 900)
	return types.WireStory{
		ID:          "web-capture-" + obj.CanonicalID,
		OwnerID:     obj.OwnerID,
		Headline:    truncateRunes(headline, 140),
		Dek:         dek,
		Freshness:   wireArticleFreshness(fetchedAt),
		Tension:     "graph-backed web capture",
		ChangeState: "captured",
		NodeTone:    "live",
		Related:     []string{},
		Manifest: types.WireSourceManifest{
			Lead: []types.WireSourceItem{{
				ID:                  obj.CanonicalID,
				Title:               headline,
				Standing:            "graph-backed web capture",
				Role:                "lead",
				CanonicalURL:        canonicalURL,
				SourceKind:          sourcecontract.SourceKindWebSource,
				TargetKind:          "web_url",
				ObjectKind:          string(obj.ObjectKind),
				CanonicalID:         obj.CanonicalID,
				VersionID:           obj.VersionID,
				ContentHash:         obj.ContentHash,
				OpenSurface:         sourcecontract.OpenSurfaceSource,
				LiveOpenSurface:     sourcecontract.OpenSurfaceWebLens,
				ReaderArtifactState: sourcecontract.ReaderArtifactStateReady,
				ReaderSnapshot:      wireReaderSnapshot(body, canonicalURL),
			}},
		},
		Claims: []string{
			"Universal Wire is reading a durable choir.web_capture object from the object graph.",
			"This card is a capture projection, not a Texture article publication or native source_ref citation.",
		},
		Projections:    map[string]string{"wire-style": projection},
		SourceState:    "objectgraph-web-capture",
		TextureContent: projection,
		CreatedAt:      obj.CreatedAt,
		UpdatedAt:      obj.UpdatedAt,
	}, true
}

func wireCaptureSourceEntityContext(ctx context.Context, graph *objectgraph.Service, capture objectgraph.Object) ([]types.WireSourceItem, error) {
	if graph == nil || strings.TrimSpace(capture.CanonicalID) == "" {
		return nil, nil
	}
	notTombstoned := false
	edges, err := graph.ListEdges(ctx, objectgraph.EdgeFilter{
		FromID:    capture.CanonicalID,
		Kind:      "captured_from",
		Tombstone: &notTombstoned,
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.WireSourceItem, 0, len(edges))
	for _, edge := range edges {
		sourceObj, err := graph.GetObject(ctx, edge.ToID)
		if err != nil {
			if errors.Is(err, objectgraph.ErrNotFound) {
				continue
			}
			return nil, err
		}
		item, ok := wireSourceItemFromGraphSourceEntity(sourceObj)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func wireSourceItemFromGraphSourceEntity(obj objectgraph.Object) (types.WireSourceItem, bool) {
	if obj.ObjectKind != "choir.source_entity" || obj.Tombstone {
		return types.WireSourceItem{}, false
	}
	var meta struct {
		SourceKind string         `json:"source_kind"`
		Target     map[string]any `json:"target"`
		Display    map[string]any `json:"display"`
		Evidence   map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return types.WireSourceItem{}, false
	}
	sourceKind := firstNonEmpty(
		sourcecontract.NormalizeSourceKind(meta.SourceKind),
		wireStringFromMap(meta.Target, "target_kind"),
		wireStringFromMap(meta.Target, "kind"),
		"source_entity",
	)
	targetKind := firstNonEmpty(
		wireStringFromMap(meta.Target, "target_kind"),
		wireStringFromMap(meta.Target, "kind"),
		sourceKind,
	)
	canonicalURL := firstNonEmpty(
		wireStringFromMap(meta.Display, "url"),
		wireStringFromMap(meta.Target, "canonical_url"),
		wireStringFromMap(meta.Target, "url"),
	)
	itemID := firstNonEmpty(
		wireStringFromMap(meta.Target, "item_id"),
		wireStringFromMap(meta.Target, "id"),
		wireStringFromMap(meta.Target, "identity"),
		obj.CanonicalID,
	)
	openSurface := sourcecontract.NormalizeOpenSurface(wireStringFromMap(meta.Evidence, "default_open_surface"))
	if openSurface == "" {
		openSurface = sourcecontract.OpenSurfaceSource
	}
	liveOpenSurface := sourcecontract.NormalizeOpenSurface(wireStringFromMap(meta.Evidence, "explicit_live_surface"))
	if liveOpenSurface == "" && canonicalURL != "" {
		liveOpenSurface = sourcecontract.OpenSurfaceWebLens
	}
	readerState := sourcecontract.NormalizeReaderArtifactState(wireStringFromMap(meta.Evidence, "reader_artifact_state"))
	if readerState == "" && wireBoolFromMap(meta.Evidence, "reader_snapshot") {
		readerState = sourcecontract.ReaderArtifactStateReady
	}
	title := firstNonEmpty(
		wireStringFromMap(meta.Display, "title"),
		wireStringFromMap(meta.Display, "label"),
		canonicalURL,
		itemID,
		"Source entity provenance",
	)
	return types.WireSourceItem{
		ID:                  itemID,
		ContentID:           wireStringFromMap(meta.Target, "item_id"),
		Title:               title,
		Standing:            "source entity provenance for graph-backed web capture",
		Role:                "context",
		SourceID:            wireStringFromMap(meta.Target, "source_id"),
		FetchID:             wireStringFromMap(meta.Target, "fetch_id"),
		CanonicalURL:        canonicalURL,
		SourceKind:          sourceKind,
		TargetKind:          targetKind,
		ObjectKind:          string(obj.ObjectKind),
		CanonicalID:         obj.CanonicalID,
		VersionID:           obj.VersionID,
		ContentHash:         obj.ContentHash,
		OpenSurface:         openSurface,
		LiveOpenSurface:     liveOpenSurface,
		ReaderArtifactState: readerState,
		ReaderSnapshot:      wireReaderSnapshot(string(obj.Body), canonicalURL),
	}, true
}

func wireReaderSnapshot(text, sourceURL string) *types.CoagentPacketSourceReaderSnapshot {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	const maxRunes = 12000
	truncated := false
	if len([]rune(text)) > maxRunes {
		text = truncateRunes(text, maxRunes)
		truncated = true
	}
	return &types.CoagentPacketSourceReaderSnapshot{
		TextContent:  text,
		SnapshotKind: "cleaned_reader_markdown",
		MediaType:    "text/markdown",
		SourceURL:    strings.TrimSpace(sourceURL),
		AccessScope:  "private_user_source",
		Truncated:    truncated,
	}
}

func wireStringFromMap(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func wireBoolFromMap(values map[string]any, key string) bool {
	if len(values) == 0 {
		return false
	}
	value, ok := values[key].(bool)
	return ok && value
}

func firstWireCaptureParagraph(content string) string {
	for _, paragraph := range wireArticleArticleParagraphs(content) {
		return paragraph
	}
	return strings.TrimSpace(content)
}

func (h *APIHandler) platformdStoryVerificationEnabled() bool {
	if h == nil || h.rt == nil {
		return false
	}
	return strings.TrimSpace(platformdReadBaseURL()) != ""
}

func wireRevisionIsUniversalWireSynthesis(rev types.Revision) bool {
	meta := decodeRevisionMetadata(rev.Metadata)
	value, ok := meta["universal_wire_synthesis"].(bool)
	return ok && value
}

func (h *APIHandler) platformdHasPublishedTexture(ctx context.Context, docID, revisionID string) bool {
	base := strings.TrimRight(strings.TrimSpace(platformdReadBaseURL()), "/")
	if base == "" || strings.TrimSpace(docID) == "" {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{
		"/internal/platform/texture/documents/" + url.PathEscape(strings.TrimSpace(docID)),
		"/internal/platform/texture/revisions/" + url.PathEscape(strings.TrimSpace(revisionID)),
	} {
		target := base + path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return false
		}
		req.Header.Set("X-Internal-Caller", "true")
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
	}
	return true
}

func platformdReadBaseURL() string {
	if url := directPlatformdBaseURL([]string{
		strings.TrimSpace(os.Getenv("RUNTIME_PLATFORMD_URL")),
		strings.TrimSpace(os.Getenv("PROXY_PLATFORMD_URL")),
	}); url != "" {
		return url
	}
	bases := []string{
		strings.TrimSpace(getenvFirst("RUNTIME_PLATFORMD_URL", "PROXY_PLATFORMD_URL")),
		strings.TrimSpace(getenvFirst("RUNTIME_VMCTL_URL", "PROXY_VMCTL_URL")),
		strings.TrimSpace(getenvFirst("RUNTIME_GATEWAY_URL")),
		strings.TrimSpace(getenvFirst("RUNTIME_MAILD_URL")),
	}
	if url := rewriteHostServicePort(bases, ":8086"); url != "" {
		return url
	}
	if data, err := os.ReadFile("/proc/cmdline"); err == nil {
		fields := strings.Fields(string(data))
		var cmdBases []string
		for _, field := range fields {
			switch {
			case strings.HasPrefix(field, "choir.vmctl_url="):
				cmdBases = append(cmdBases, strings.TrimPrefix(field, "choir.vmctl_url="))
			case strings.HasPrefix(field, "choir.gateway_url="):
				cmdBases = append(cmdBases, strings.TrimPrefix(field, "choir.gateway_url="))
			case strings.HasPrefix(field, "choir.maild_url="):
				cmdBases = append(cmdBases, strings.TrimPrefix(field, "choir.maild_url="))
			}
		}
		if url := rewriteHostServicePort(cmdBases, ":8086"); url != "" {
			return url
		}
	}
	return ""
}

func directPlatformdBaseURL(bases []string) string {
	for _, raw := range bases {
		base := strings.TrimRight(strings.TrimSpace(raw), "/")
		if base == "" {
			continue
		}
		parsed, err := url.Parse(base)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		if parsed.Port() == "8086" {
			return base
		}
	}
	return ""
}

func rewriteHostServicePort(bases []string, wantPort string) string {
	for _, raw := range bases {
		base := strings.TrimRight(strings.TrimSpace(raw), "/")
		if base == "" {
			continue
		}
		for _, suffix := range []string{":8082", ":8083", ":8084", ":8087"} {
			if strings.HasSuffix(base, suffix) {
				return strings.TrimSuffix(base, suffix) + wantPort
			}
		}
	}
	return ""
}

func universalWireEditionIncludedDocIDs(content, editionDocID string) []string {
	seen := map[string]bool{}
	editionDocID = strings.TrimSpace(editionDocID)
	out := []string{}
	for _, match := range textureTransclusionRefRE.FindAllStringSubmatch(content, -1) {
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

func universalWireStoriesNeedArticleSurfaceRepair(stories []types.WireStory) bool {
	for _, story := range stories {
		text := strings.Join([]string{story.Headline, story.Dek, story.TextureContent}, "\n")
		if strings.Contains(text, "Universal Wire live synthesis:") ||
			strings.Contains(text, "Universal Wire selected ") ||
			strings.Contains(text, "graph-backed source captures") ||
			strings.Contains(text, "Universal Wire treats") ||
			strings.Contains(text, "incoming reports point to the same developing story") ||
			strings.Contains(text, "A second source in the cluster") ||
			strings.Contains(text, "reports read as one developing article") {
			return true
		}
	}
	return false
}

func universalWirePlatformOwnerID() string {
	ownerID := strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

// resolveUniversalWireTextureReadOwner returns the document owner to use for a
// read-only Texture API request. Authenticated users may read platform-owned
// Texture articles that are transcluded in the Universal Wire edition.
func (h *APIHandler) resolveUniversalWireTextureReadOwner(ctx context.Context, requesterOwnerID, docID string) (string, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return "", store.ErrNotFound
	}
	requesterOwnerID = strings.TrimSpace(requesterOwnerID)
	docID = strings.TrimSpace(docID)
	if requesterOwnerID == "" || docID == "" {
		return "", store.ErrNotFound
	}
	if _, err := h.rt.Store().GetDocument(ctx, docID, requesterOwnerID); err == nil {
		return requesterOwnerID, nil
	} else if err != store.ErrNotFound {
		return "", err
	}
	platformOwner := universalWirePlatformOwnerID()
	if _, err := h.rt.Store().GetDocument(ctx, docID, platformOwner); err != nil {
		return "", err
	}
	if !h.universalWireEditionIncludesDoc(ctx, docID) {
		return "", store.ErrNotFound
	}
	return platformOwner, nil
}

func (h *APIHandler) universalWireEditionIncludesDoc(ctx context.Context, docID string) bool {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return false
	}
	platformOwner := universalWirePlatformOwnerID()
	editionDocID, err := h.rt.Store().GetDocumentAlias(ctx, platformOwner, universalWireEditionSourcePath)
	if err != nil {
		return false
	}
	editionDoc, err := h.rt.Store().GetDocument(ctx, editionDocID, platformOwner)
	if err != nil || strings.TrimSpace(editionDoc.CurrentRevisionID) == "" {
		return false
	}
	editionRev, err := h.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, platformOwner)
	if err != nil {
		return false
	}
	for _, included := range universalWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID) {
		if included == docID {
			return true
		}
	}
	return false
}

func wireArticleTextureStoryFromCurrentRevision(ctx context.Context, doc types.Document, rev types.Revision, styleSources []types.WireStyleSource) (types.WireStory, bool) {
	meta := decodeRevisionMetadata(rev.Metadata)
	cycleID := sourceNetworkCycleID(meta)
	if !wireRevisionSourceIsTextureEdit(meta) || cycleID == "" || !wireRevisionIsCanonicalArticle(meta) {
		return types.WireStory{}, false
	}
	content := strings.TrimSpace(rev.Content)
	if content == "" || wireArticleContentLooksLikeSeed(content) {
		return types.WireStory{}, false
	}
	styleID, styleTitle := wireArticleSelectedStyle(meta, styleSources)
	headline := wireArticleArticleHeadline(doc.Title, content)
	dek := wireArticleArticleDek(content)
	projection := wireArticleArticleProjection(content)
	manifest := wireArticleManifestFromRevision(ctx, rev, meta, content, headline)
	if len(manifest.Lead) == 0 &&
		len(manifest.Supporting) == 0 &&
		len(manifest.Contrary) == 0 &&
		len(manifest.Context) == 0 {
		manifest.Context = append(manifest.Context, types.WireSourceItem{
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
	platformRoute := wirePlatformRoutePath(meta)
	if platformRoute == "" {
		return types.WireStory{}, false
	}
	changeState := "platform published"
	return types.WireStory{
		ID:                    "source-network-texture-" + doc.DocID,
		OwnerID:               doc.OwnerID,
		Headline:              headline,
		Dek:                   dek,
		Freshness:             wireArticleFreshness(doc.UpdatedAt),
		Prominence:            90,
		Tension:               "source-network article",
		ChangeState:           changeState,
		PlatformRoutePath:     platformRoute,
		NodeTone:              "live",
		Related:               []string{},
		Manifest:              manifest,
		Claims:                wireArticleArticleClaims(content, styleTitle, meta),
		Projections:           projections,
		ProjectionTextureDocs: map[string]string{styleID: doc.DocID},
		StyleSources:          styleSources,
		StoryTextureDoc:       doc.DocID,
		TextureContent:        content,
		SourceState:           "source-network-texture-index",
		CreatedAt:             doc.CreatedAt,
		UpdatedAt:             doc.UpdatedAt,
	}, true
}

func wireArticleContentLooksLikeSeed(content string) bool {
	return strings.Contains(content, "## Source Brief") ||
		strings.Contains(content, "## Evidence Gathering") ||
		strings.Contains(content, "## Working Revision")
}

func wireArticleSelectedStyle(meta map[string]any, styles []types.WireStyleSource) (string, string) {
	title := "Style.texture: Universal Wire"
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

func wireArticleMetadataStringSlice(value any) []string {
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

func wireArticleManifestFromRevision(ctx context.Context, rev types.Revision, meta map[string]any, content, headline string) types.WireSourceManifest {
	entities := wireArticleVisibleSourceEntities(ctx, rev, meta, content)
	if len(entities) > 0 {
		return wireArticleManifestFromSourceEntities(entities)
	}
	return wireArticleManifestFromCycleProvenance(meta, headline)
}

func wireArticleVisibleSourceEntities(ctx context.Context, rev types.Revision, meta map[string]any, content string) []textureSourceEntity {
	if entities := wireArticleVisibleStructuredSourceEntities(rev); len(entities) > 0 {
		enrichSourceServiceEntities(ctx, entities)
		return entities
	}
	return nil
}

func wireArticleVisibleStructuredSourceEntities(rev types.Revision) []textureSourceEntity {
	if len(strings.TrimSpace(string(rev.BodyDoc))) == 0 || len(strings.TrimSpace(string(rev.SourceEntities))) == 0 {
		return nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(rev.BodyDoc, &doc); err != nil {
		return nil
	}
	var structured []texturedoc.SourceEntity
	if err := json.Unmarshal(rev.SourceEntities, &structured); err != nil {
		return nil
	}
	if len(structured) == 0 {
		return nil
	}
	refs := map[string]bool{}
	wireArticleCollectVisibleStructuredSourceRefs(doc.Doc, refs)
	if len(refs) == 0 {
		return nil
	}
	out := make([]textureSourceEntity, 0, len(structured))
	seen := map[string]bool{}
	for _, entity := range structured {
		id := strings.TrimSpace(entity.SourceEntityID)
		if id == "" || !refs[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, provenanceSourceEntityFromStructured(entity))
	}
	return out
}

func wireArticleCollectVisibleStructuredSourceRefs(node texturedoc.Node, refs map[string]bool) bool {
	for _, child := range node.Content {
		if wireArticleArticleLineStartsInventorySection(wireArticleStructuredNodeText(child)) {
			return true
		}
		if child.Type == "source_ref" {
			if id := textureNodeStringAttr(child, "source_entity_id"); id != "" {
				refs[id] = true
			}
		}
		if wireArticleCollectVisibleStructuredSourceRefs(child, refs) {
			return true
		}
	}
	return false
}

func wireArticleStructuredNodeText(node texturedoc.Node) string {
	if node.Type == "text" {
		return node.Text
	}
	var b strings.Builder
	for _, child := range node.Content {
		b.WriteString(wireArticleStructuredNodeText(child))
	}
	return b.String()
}

func wireArticleManifestFromSourceEntities(entities []textureSourceEntity) types.WireSourceManifest {
	manifest := types.WireSourceManifest{}
	for i, entity := range entities {
		id := wireArticleSourceEntityManifestID(entity)
		if id == "" {
			continue
		}
		item := types.WireSourceItem{
			ID:                  id,
			ContentID:           firstNonEmpty(strings.TrimSpace(entity.Target.ContentID), strings.TrimSpace(entity.Target.ItemID)),
			Title:               wireArticleSourceEntityManifestTitle(entity),
			Standing:            wireArticleSourceEntityManifestStanding(entity),
			Role:                "lead",
			SourceID:            strings.TrimSpace(entity.Target.SourceID),
			FetchID:             strings.TrimSpace(entity.Target.FetchID),
			CanonicalURL:        firstNonEmpty(entity.Target.CanonicalURL, entity.Target.URL),
			SourceKind:          strings.TrimSpace(entity.Kind),
			TargetKind:          strings.TrimSpace(entity.Target.TargetKind),
			OpenSurface:         sourcecontract.NormalizeOpenSurface(entity.Display.OpenSurface),
			ReaderArtifactState: sourcecontract.NormalizeReaderArtifactState(entity.Evidence.SourceRepresentationID),
			ReaderSnapshot:      wireSourceEntityReaderSnapshot(entity),
		}
		if item.OpenSurface == "" {
			item.OpenSurface = sourcecontract.OpenSurfaceSource
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

func wireArticleSourceEntityManifestID(entity textureSourceEntity) string {
	return firstNonEmpty(entity.Target.ItemID, entity.Target.ContentID, entity.Target.DocID, entity.EntityID)
}

func wireArticleSourceEntityManifestTitle(entity textureSourceEntity) string {
	return firstNonEmpty(entity.Label, entity.Target.CanonicalURL, entity.Target.URL, wireArticleSourceEntityManifestID(entity))
}

func wireArticleSourceEntityManifestStanding(entity textureSourceEntity) string {
	switch strings.TrimSpace(entity.Kind) {
	case "content_item":
		return "embedded source"
	case "source_service_item":
		return "source-service handle"
	case "texture":
		return "related Texture"
	default:
		return firstNonEmpty(entity.Kind, "source handle")
	}
}

func wireSourceEntityReaderSnapshot(entity textureSourceEntity) *types.CoagentPacketSourceReaderSnapshot {
	text := metadataString(entity.ReaderSnapshot, "text_content")
	if text == "" {
		text = metadataString(entity.ReaderSnapshot, "text")
	}
	if text == "" {
		return nil
	}
	return &types.CoagentPacketSourceReaderSnapshot{
		TextContent:       text,
		SnapshotKind:      firstNonEmpty(metadataString(entity.ReaderSnapshot, "snapshot_kind"), "cleaned_reader_markdown"),
		MediaType:         firstNonEmpty(metadataString(entity.ReaderSnapshot, "media_type"), "text/markdown"),
		OriginalMediaType: metadataString(entity.ReaderSnapshot, "original_media_type"),
		SourceURL:         firstNonEmpty(metadataString(entity.ReaderSnapshot, "source_url"), entity.Target.CanonicalURL, entity.Target.URL),
		AccessScope:       firstNonEmpty(metadataString(entity.ReaderSnapshot, "access_scope"), "private_user_source"),
		Truncated:         metadataBoolValue(entity.ReaderSnapshot, "truncated"),
	}
}

func wireArticleManifestFromCycleProvenance(meta map[string]any, headline string) types.WireSourceManifest {
	manifest := types.WireSourceManifest{}
	cycleID := sourceNetworkCycleID(meta)
	sourceIDs := wireArticleMetadataStringSlice(meta["source_item_ids"])
	switch {
	case cycleID != "":
		standing := "source firehose cycle"
		if len(sourceIDs) > 0 {
			standing = fmt.Sprintf("source firehose cycle; %d source ids retained in revision provenance", len(sourceIDs))
		}
		manifest.Context = append(manifest.Context, types.WireSourceItem{
			ID:       "source-network-cycle:" + cycleID,
			Title:    "Source network cycle " + cycleID,
			Standing: standing,
			Role:     "context",
		})
	case strings.TrimSpace(headline) != "":
		manifest.Context = append(manifest.Context, types.WireSourceItem{
			ID:       "source-network-texture:" + headline,
			Title:    "Universal Wire Texture article head",
			Standing: "platform Texture current revision",
			Role:     "context",
		})
	}
	return manifest
}

func wireArticleArticleHeadline(title, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	title = strings.TrimSpace(strings.TrimSuffix(title, ".texture"))
	if title != "" {
		return title
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "# -*\t"))
		if line != "" {
			return truncateRunes(line, 120)
		}
	}
	return "Universal Wire article"
}

func wireArticleArticleDek(content string) string {
	for _, paragraph := range wireArticleArticleParagraphs(content) {
		return truncateRunes(paragraph, 220)
	}
	return "Universal Wire Texture article with source and style provenance on its current revision."
}

func wireArticleArticleProjection(content string) string {
	paragraphs := wireArticleArticleParagraphs(content)
	if len(paragraphs) == 0 {
		return truncateRunes(content, 520)
	}
	return truncateRunes(strings.Join(paragraphs, "\n\n"), 900)
}

func wireArticleArticleParagraphs(content string) []string {
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
		if wireArticleArticleLineIsScaffold(line) {
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

func wireArticleArticleLineIsScaffold(line string) bool {
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
		strings.HasPrefix(lower, "style.texture source") ||
		strings.HasPrefix(lower, "style.texture source") ||
		strings.HasPrefix(lower, "style source:") ||
		strings.HasPrefix(lower, "selection rationale:") ||
		strings.HasPrefix(lower, "story id:") ||
		strings.HasPrefix(lower, "state:") {
		return true
	}
	if lower == "source handles" ||
		lower == "source manifest" ||
		lower == "style.texture source" {
		return true
	}
	if strings.HasPrefix(normalized, "published:") ||
		strings.HasPrefix(normalized, "date:") ||
		strings.HasPrefix(normalized, "status:") ||
		strings.HasPrefix(normalized, "by ") ||
		strings.HasPrefix(normalized, "source:") {
		return true
	}
	return wireArticleArticleLineStartsInventorySection(trimmed)
}

func wireArticleArticleLineStartsInventorySection(line string) bool {
	plain := strings.TrimSpace(strings.TrimLeft(line, "#*_ \t"))
	lower := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(plain, "**", ""), "__", "")))
	if lower == "source handles" ||
		lower == "source manifest" ||
		lower == "sources" ||
		lower == "style.texture source" ||
		lower == "style source" {
		return true
	}
	if strings.HasPrefix(lower, "source handles:") ||
		strings.HasPrefix(lower, "source manifest:") ||
		strings.HasPrefix(lower, "style.texture source:") ||
		strings.HasPrefix(lower, "style.texture source:") ||
		strings.HasPrefix(lower, "style source:") {
		return true
	}
	return false
}

func wireArticleArticleClaims(content, _ string, meta map[string]any) []string {
	claims := []string{
		"Current head is a normal Texture article revision owned by the Universal Wire platform agent.",
		"Source and style provenance are carried by the Texture revision metadata and citations.",
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
	return firstNonEmptyString(metadataString(meta, "source_network_cycle_id"), metadataString(meta, "ingestion_handoff_cycle_id"))
}

func wirePlatformRoutePath(meta map[string]any) string {
	if route := metadataString(meta, "platformd_route_path"); route != "" {
		return route
	}
	if ref, ok := meta["platformd_publication_ref"].(map[string]any); ok {
		return metadataString(ref, "route_path")
	}
	return ""
}

func wireArticleFreshness(updatedAt time.Time) string {
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

func normalizeWireStoryPresentation(story types.WireStory) types.WireStory {
	if universalWireStoryFreshnessLooksAuto(story.Freshness) {
		story.Freshness = wireArticleFreshness(story.UpdatedAt)
	}
	return story
}

// normalizeWireArticleRevisionForRead exists for the cross-owner Universal Wire
// read path. Source refs are no longer repaired from markdown/source-token
// syntax here; visible source identity must come from structured body_doc nodes.
func normalizeWireArticleRevisionForRead(rev types.Revision) types.Revision {
	return rev
}

func wireRevisionSourceIsTextureEdit(meta map[string]any) bool {
	switch metadataString(meta, "source") {
	case "patch_texture", "rewrite_texture", "edit_texture":
		return true
	default:
		return false
	}
}

func universalWireStoryFreshnessLooksAuto(freshness string) bool {
	freshness = strings.TrimSpace(strings.ToLower(freshness))
	return freshness == "" || strings.HasPrefix(freshness, "updated ")
}
