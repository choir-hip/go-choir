package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type universalWireSynthesisClusterRequest struct {
	ClusterID         string
	Headline          string
	Summary           string
	Tension           string
	PlatformRoutePath string
	Sources           []universalWireSynthesisSource
	Now               time.Time
}

type universalWireSynthesisSource struct {
	CaptureObjectID string
	ItemID          string
	SourceID        string
	FetchID         string
	Title           string
	URL             string
	CanonicalURL    string
	Language        string
	Body            string
	FetchedAt       time.Time
}

func (rt *Runtime) synthesizeUniversalWireSourceClusterTextureArticle(ctx context.Context, req universalWireSynthesisClusterRequest) (types.Document, types.Revision, string, error) {
	if rt == nil || rt.store == nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("runtime store unavailable")
	}
	sources := normalizedUniversalWireSynthesisSources(req.Sources)
	if len(sources) < 2 {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("universal wire synthesis requires at least two source items")
	}
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	headline := strings.TrimSpace(req.Headline)
	if headline == "" {
		headline = universalWireSynthesisHeadline(sources)
	}
	clusterID := strings.TrimSpace(req.ClusterID)
	if clusterID == "" {
		clusterID = stableSourceEntityID("universal_wire_cluster", headline+"|"+sources[0].ItemID+"|"+sources[1].ItemID)
	}
	aliasPath := "universal-wire/articles/" + universalWireSlug(clusterID) + ".texture"
	ownerID := universalWirePlatformOwnerID()
	doc, err := rt.getOrCreateUniversalWireSynthesisDocument(ctx, ownerID, aliasPath, headline, now)
	if err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	storyClusterObjectID := universalWireStoryClusterObjectID(ownerID, clusterID)

	revisionID := uuid.NewString()
	sourceEntities := universalWireSynthesisSourceEntities(sources, now)
	content := universalWireSynthesisArticleMarkdown(headline, req.Summary, req.Tension, sources, sourceEntities)
	bodyDoc, sourceEntitiesJSON, projectedContent, err := markdownLineageStructuredRevision(doc.DocID, revisionID, content, sourceEntities, nil)
	if err != nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("create synthesis body_doc: %w", err)
	}
	sourceItemIDs := make([]string, 0, len(sources))
	languages := make([]string, 0, len(sources))
	for _, source := range sources {
		sourceItemIDs = append(sourceItemIDs, source.ItemID)
		if source.Language != "" && !containsWireString(languages, source.Language) {
			languages = append(languages, source.Language)
		}
	}
	routePath := strings.TrimSpace(req.PlatformRoutePath)
	if routePath == "" {
		routePath = "/pub/texture/universal-wire/" + universalWireSlug(clusterID)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":                                 "edit_texture",
		"revision_role":                          textureRevisionRoleCanonical,
		"artifact_kind":                          "article_revision",
		"texture_version_stage":                  "article_revision",
		"source_network_cycle_id":                clusterID,
		"ingestion_handoff_cycle_id":             clusterID,
		"ingestion_handoff_request_id":           "universal-wire-synthesis-cluster:" + clusterID,
		"ingestion_handoff_request_kind":         "synthesis_cluster",
		"universal_wire_synthesis":               true,
		"universal_wire_story_cluster_id":        clusterID,
		"universal_wire_story_cluster_object_id": storyClusterObjectID,
		"universal_wire_article_alias_path":      aliasPath,
		"synthesis_source_count":                 len(sources),
		"synthesis_languages":                    languages,
		"source_item_ids":                        sourceItemIDs,
		"selected_style_sources":                 []map[string]any{{"title": "Style.texture: Universal Wire"}},
		"selected_style_rationale":               "Bounded local Universal Wire synthesis slice over clustered source captures.",
		"platformd_route_path":                   routePath,
	})
	rev := types.Revision{
		RevisionID:       revisionID,
		DocID:            doc.DocID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "universal_wire_synthesis",
		Content:          projectedContent,
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntitiesJSON,
		Citations:        json.RawMessage("[]"),
		Metadata:         meta,
		ParentRevisionID: doc.CurrentRevisionID,
		CreatedAt:        now,
	}
	if err := rt.store.CreateRevision(ctx, rev); err != nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("create synthesis revision: %w", err)
	}
	doc, err = rt.store.GetDocument(ctx, doc.DocID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("reload synthesis document: %w", err)
	}
	rev, err = rt.store.GetRevision(ctx, doc.CurrentRevisionID, ownerID)
	if err != nil {
		return types.Document{}, types.Revision{}, "", fmt.Errorf("reload synthesis revision: %w", err)
	}
	editionRef, err := rt.ensureUniversalWireEditionIncludes(ctx, doc, rev, now)
	if err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	if err := rt.upsertUniversalWireStoryCluster(ctx, clusterID, aliasPath, doc, rev, editionRef, sources, now); err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	return doc, rev, editionRef, nil
}

func (rt *Runtime) upsertUniversalWireStoryCluster(ctx context.Context, clusterID, aliasPath string, doc types.Document, rev types.Revision, editionRef string, sources []universalWireSynthesisSource, now time.Time) error {
	if rt == nil || rt.ObjectGraph() == nil {
		return nil
	}
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("universal wire story cluster id is required")
	}
	sourceItemIDs := make([]string, 0, len(sources))
	captureObjectIDs := make([]string, 0, len(sources))
	languages := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.ItemID != "" {
			sourceItemIDs = append(sourceItemIDs, source.ItemID)
		}
		if source.CaptureObjectID != "" {
			captureObjectIDs = append(captureObjectIDs, source.CaptureObjectID)
		}
		if source.Language != "" && !containsWireString(languages, source.Language) {
			languages = append(languages, source.Language)
		}
	}
	body, _ := json.Marshal(map[string]any{
		"headline": strings.TrimSuffix(doc.Title, ".texture"),
		"summary":  fmt.Sprintf("Universal Wire story cluster %s currently synthesizes %d source captures into one Texture article.", clusterID, len(sources)),
	})
	cluster, err := rt.ObjectGraph().CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:        objectgraph.UniversalWireStoryClusterObjectKind,
		OwnerID:     universalWirePlatformOwnerID(),
		IdentityKey: clusterID,
		Body:        body,
		Metadata: map[string]any{
			"schema_version":      objectgraph.UniversalWireStoryClusterSchemaVersion,
			"cluster_id":          clusterID,
			"cluster_kind":        "live_sourcecycled_story",
			"article_doc_id":      doc.DocID,
			"article_revision_id": rev.RevisionID,
			"article_alias_path":  aliasPath,
			"wire_edition_ref":    editionRef,
			"source_count":        len(sources),
			"source_item_ids":     sourceItemIDs,
			"source_capture_ids":  captureObjectIDs,
			"synthesis_languages": languages,
			"updated_at":          now.UTC().Format(time.RFC3339Nano),
		},
		Now: now,
	})
	if err != nil {
		return fmt.Errorf("upsert universal wire story cluster: %w", err)
	}
	for _, captureObjectID := range captureObjectIDs {
		if _, err := rt.ObjectGraph().PutEdge(ctx, cluster.CanonicalID, captureObjectID, "contains", map[string]any{
			"schema_version": objectgraph.UniversalWireStoryClusterSchemaVersion,
			"relation":       "cluster_source_capture",
			"cluster_id":     clusterID,
		}); err != nil {
			return fmt.Errorf("link universal wire story cluster source capture: %w", err)
		}
	}
	return nil
}

func universalWireStoryClusterObjectID(ownerID, clusterID string) string {
	id, err := objectgraph.BuildCanonicalID(objectgraph.UniversalWireStoryClusterObjectKind, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(strings.TrimSpace(clusterID)))
	if err != nil {
		return ""
	}
	return id
}

func (rt *Runtime) getOrCreateUniversalWireSynthesisDocument(ctx context.Context, ownerID, aliasPath, headline string, now time.Time) (types.Document, error) {
	if docID, err := rt.store.GetDocumentAlias(ctx, ownerID, aliasPath); err == nil {
		return rt.store.GetDocument(ctx, docID, ownerID)
	} else if err != store.ErrNotFound {
		return types.Document{}, fmt.Errorf("resolve synthesis alias: %w", err)
	}
	doc := types.Document{
		DocID:     uuid.NewString(),
		OwnerID:   ownerID,
		Title:     strings.TrimSpace(firstNonEmpty(headline, "Universal Wire synthesis")) + ".texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := rt.store.CreateDocument(ctx, doc); err != nil {
		return types.Document{}, fmt.Errorf("create synthesis document: %w", err)
	}
	if err := rt.store.UpsertDocumentAlias(ctx, ownerID, aliasPath, doc.DocID, now); err != nil {
		return types.Document{}, fmt.Errorf("upsert synthesis alias: %w", err)
	}
	return doc, nil
}

func (rt *Runtime) ensureUniversalWireEditionIncludes(ctx context.Context, doc types.Document, rev types.Revision, now time.Time) (string, error) {
	ownerID := universalWirePlatformOwnerID()
	if _, err := rt.store.GetDocumentAlias(ctx, ownerID, universalWireEditionSourcePath); err == store.ErrNotFound {
		editionDoc := types.Document{
			DocID:     uuid.NewString(),
			OwnerID:   ownerID,
			Title:     "Wire.texture",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := rt.store.CreateDocument(ctx, editionDoc); err != nil {
			return "", fmt.Errorf("create wire edition document: %w", err)
		}
		revisionID := uuid.NewString()
		bodyDoc, sourceEntitiesJSON, projectedContent, err := markdownLineageStructuredRevision(editionDoc.DocID, revisionID, "# Wire\n\nUniversal Wire edition.", nil, nil)
		if err != nil {
			return "", fmt.Errorf("create wire edition body_doc: %w", err)
		}
		meta, _ := json.Marshal(map[string]any{
			"source":        "universal_wire_edition",
			"revision_role": textureRevisionRoleCanonical,
		})
		if err := rt.store.CreateRevision(ctx, types.Revision{
			RevisionID:     revisionID,
			DocID:          editionDoc.DocID,
			OwnerID:        ownerID,
			AuthorKind:     types.AuthorAppAgent,
			AuthorLabel:    "universal_wire_synthesis",
			Content:        projectedContent,
			BodyDoc:        bodyDoc,
			SourceEntities: sourceEntitiesJSON,
			Citations:      json.RawMessage("[]"),
			Metadata:       meta,
			CreatedAt:      now,
		}); err != nil {
			return "", fmt.Errorf("create wire edition revision: %w", err)
		}
		if err := rt.store.UpsertDocumentAlias(ctx, ownerID, universalWireEditionSourcePath, editionDoc.DocID, now); err != nil {
			return "", fmt.Errorf("upsert wire edition alias: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("resolve wire edition alias: %w", err)
	}
	editionRef, err := rt.autonomousPublishWireArticleToEdition(ctx, doc, rev)
	if err != nil {
		return "", fmt.Errorf("link synthesis article into wire edition: %w", err)
	}
	if editionRef == "" {
		return "", fmt.Errorf("wire edition ref unavailable for synthesis article %s", doc.DocID)
	}
	return editionRef, nil
}

func normalizedUniversalWireSynthesisSources(inputs []universalWireSynthesisSource) []universalWireSynthesisSource {
	out := make([]universalWireSynthesisSource, 0, len(inputs))
	seen := map[string]bool{}
	for _, input := range inputs {
		input.CaptureObjectID = strings.TrimSpace(input.CaptureObjectID)
		input.Title = strings.TrimSpace(input.Title)
		input.Body = strings.TrimSpace(input.Body)
		input.URL = strings.TrimSpace(input.URL)
		input.CanonicalURL = strings.TrimSpace(firstNonEmpty(input.CanonicalURL, input.URL))
		input.ItemID = strings.TrimSpace(input.ItemID)
		if input.ItemID == "" {
			input.ItemID = stableSourceEntityID("source_service_item", firstNonEmpty(input.CanonicalURL, input.Title, input.Body))
		}
		key := firstNonEmpty(input.ItemID, input.CanonicalURL)
		if key == "" || seen[key] || input.Title == "" || input.Body == "" {
			continue
		}
		seen[key] = true
		out = append(out, input)
	}
	return out
}

func universalWireSynthesisSourceEntities(sources []universalWireSynthesisSource, now time.Time) []textureSourceEntity {
	out := make([]textureSourceEntity, 0, len(sources))
	for _, source := range sources {
		entityID := stableSourceEntityID("source_service_item", source.ItemID)
		if entityID == "" {
			continue
		}
		readerSnapshot := map[string]any{
			"text_content":  source.Body,
			"snapshot_kind": "cleaned_reader_markdown",
			"media_type":    "text/markdown",
			"source_url":    source.CanonicalURL,
			"access_scope":  "private_user_source",
			"source_title":  source.Title,
			"source_id":     source.SourceID,
			"captured_at":   now.UTC().Format(time.RFC3339Nano),
		}
		if !source.FetchedAt.IsZero() {
			readerSnapshot["fetched_at"] = source.FetchedAt.UTC().Format(time.RFC3339Nano)
		}
		out = append(out, textureSourceEntity{
			EntityID: entityID,
			Kind:     "source_service_item",
			Label:    source.Title,
			Target: textureSourceEntityTarget{
				TargetKind:   "source_service_item",
				ItemID:       source.ItemID,
				SourceID:     strings.TrimSpace(source.SourceID),
				FetchID:      strings.TrimSpace(source.FetchID),
				URL:          source.URL,
				CanonicalURL: source.CanonicalURL,
			},
			Selectors: []textureSourceEntitySelector{{SelectorKind: sourcecontract.SelectorKindWholeResource}},
			Display: textureSourceEntityDisplay{
				InlineMode:       "collapsed_citation",
				ExpandedMode:     "source_card",
				OpenSurface:      sourcecontract.OpenSurfaceSource,
				DefaultCollapsed: true,
			},
			Evidence: textureSourceEntityEvidence{
				State:                  sourcecontract.EvidenceStateAvailable,
				ResearchState:          "represented",
				ReaderSnapshot:         true,
				BodyKind:               "reader_snapshot",
				BodyLength:             len([]rune(source.Body)),
				SourceRepresentationID: sourcecontract.ReaderArtifactStateReady,
			},
			Provenance: textureSourceEntityProvenance{
				CreatedBy:           "universal_wire_synthesis",
				RightsScope:         "private_user_source",
				UntrustedSourceText: true,
			},
			ReaderSnapshot:       pruneEmptyMap(readerSnapshot),
			ReaderSnapshotStatus: map[string]any{"state": sourcecontract.ReaderArtifactStateReady},
		})
	}
	return out
}

func universalWireSynthesisArticleMarkdown(headline, summary, tension string, sources []universalWireSynthesisSource, entities []textureSourceEntity) string {
	first := sources[0]
	second := sources[1]
	firstRef := entities[0].EntityID
	secondRef := entities[1].EntityID
	if summary = strings.TrimSpace(summary); summary == "" {
		summary = fmt.Sprintf("The source cluster points to one developing story: %s and %s describe related moves in the same news event rather than isolated items.", first.Title, second.Title)
	}
	if tension = strings.TrimSpace(tension); tension == "" {
		tension = "The live question is whether later source arrivals reinforce this combined reading or require the article to be revised."
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(headline)
	b.WriteString("\n\n")
	b.WriteString(summary)
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("[first source](source:%s)", firstRef))
	b.WriteString("\n\n")
	b.WriteString("A second source in the cluster adds a separate angle rather than repeating the same capture, so Universal Wire treats the pair as material for one English synthesis article instead of two raw cards. ")
	b.WriteString(fmt.Sprintf("[second source](source:%s)", secondRef))
	b.WriteString("\n\n")
	b.WriteString(tension)
	for i := 2; i < len(sources) && i < len(entities); i++ {
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("[additional source](source:%s)", entities[i].EntityID))
	}
	return b.String()
}

func universalWireSynthesisHeadline(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "Universal Wire synthesis"
	}
	return "Universal Wire synthesis: " + truncateRunes(sources[0].Title, 90)
}

var universalWireSlugInvalidRE = regexp.MustCompile(`[^a-z0-9]+`)

func universalWireSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = universalWireSlugInvalidRE.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return uuid.NewString()
	}
	return truncateRunes(value, 80)
}

func containsWireString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
