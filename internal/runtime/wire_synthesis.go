package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var universalWireArticleSurfaceHelperPhrases = []string{
	"Universal Wire live synthesis:",
	"Universal Wire selected ",
	"graph-backed source captures",
	"Universal Wire treats",
	"incoming reports point to the same developing story",
	"A second source in the cluster",
	"reports read as one developing article",
	"gives the clearest current account",
	"second sourced angle",
	"The second account narrows what readers can trust now",
	"Multiple reports converge",
}

type universalWireSynthesisClusterRequest struct {
	ClusterID         string
	Headline          string
	Summary           string
	Tension           string
	PlatformRoutePath string
	Sources           []universalWireSynthesisSource
	SemanticState     universalWireSemanticStoryState
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
	if headline == "" || universalWireTextContainsArticleSurfaceHelper(headline) {
		headline = universalWireSynthesisHeadline(sources)
	}
	clusterID := strings.TrimSpace(req.ClusterID)
	if clusterID == "" {
		clusterID = stableSourceEntityID("universal_wire_cluster", headline+"|"+sources[0].ItemID+"|"+sources[1].ItemID)
	}
	semanticState := req.SemanticState
	if semanticState.StoryID == "" {
		semanticState = rt.universalWireSemanticStoryState(ctx, clusterID, sources, now)
	}
	if strings.TrimSpace(req.Headline) == "" || universalWireTextContainsArticleSurfaceHelper(req.Headline) {
		headline = firstNonEmpty(semanticState.Headline, headline)
	}
	aliasPath := "universal-wire/articles/" + universalWireSlug(clusterID) + ".texture"
	ownerID := universalWirePlatformOwnerID()
	doc, err := rt.getOrCreateUniversalWireSynthesisDocument(ctx, ownerID, aliasPath, headline, now)
	if err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	if wantTitle := universalWireSynthesisDocumentTitle(headline); wantTitle != "" && doc.Title != wantTitle {
		doc.Title = wantTitle
		doc.UpdatedAt = now
		if err := rt.store.UpdateDocument(ctx, doc); err != nil {
			return types.Document{}, types.Revision{}, "", fmt.Errorf("update synthesis document title: %w", err)
		}
		doc, err = rt.store.GetDocument(ctx, doc.DocID, ownerID)
		if err != nil {
			return types.Document{}, types.Revision{}, "", fmt.Errorf("reload synthesis document title: %w", err)
		}
	}
	storyClusterObjectID := universalWireStoryClusterObjectID(ownerID, clusterID)

	revisionID := uuid.NewString()
	sourceEntities := universalWireSynthesisSourceEntities(sources, now)
	content := universalWireSynthesisArticleMarkdown(headline, req.Summary, req.Tension, semanticState, sources, sourceEntities)
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
		"universal_wire_semantic_story_id":       semanticState.StoryID,
		"universal_wire_semantic_change_type":    semanticState.LatestChange.ChangeType,
		"universal_wire_update_decision":         semanticState.UpdateDecision,
		"universal_wire_article_alias_path":      aliasPath,
		"synthesis_source_count":                 len(sources),
		"synthesis_languages":                    languages,
		"synthesis_frame_kind":                   "semantic_world_model_source_map",
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
	if err := rt.publishUniversalWireSynthesisArticleToPlatform(ctx, doc, rev); err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	if err := rt.upsertUniversalWireStoryCluster(ctx, clusterID, aliasPath, doc, rev, editionRef, sources, semanticState, now); err != nil {
		return types.Document{}, types.Revision{}, "", err
	}
	return doc, rev, editionRef, nil
}

func (rt *Runtime) publishUniversalWireSynthesisArticleToPlatform(ctx context.Context, doc types.Document, rev types.Revision) error {
	if rt == nil {
		return fmt.Errorf("runtime unavailable")
	}
	if rt.wirePlatformPublisher == nil &&
		strings.TrimSpace(rt.cfg.WirePublishURL) == "" &&
		strings.TrimSpace(rt.cfg.PlatformdURL) == "" &&
		fallbackWirePublishURLFromEnv() == "" {
		return nil
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "universal-wire-synthesis:" + strings.TrimSpace(rev.RevisionID),
		Metadata: map[string]any{
			"request_intent":          "universal_wire_synthesis_article_revision",
			"type":                    "texture_agent_revision",
			"source_network_cycle_id": metadataString(meta, "source_network_cycle_id"),
		},
	}
	pub, err := rt.publishWireArticleToPlatform(ctx, doc, rev, rec)
	if err != nil {
		return fmt.Errorf("publish synthesis article to platform: %w", err)
	}
	if err := rt.persistWirePlatformPublicationRef(ctx, doc.OwnerID, rev, pub); err != nil {
		return fmt.Errorf("persist synthesis platform publication ref: %w", err)
	}
	return nil
}

func (rt *Runtime) upsertUniversalWireStoryCluster(ctx context.Context, clusterID, aliasPath string, doc types.Document, rev types.Revision, editionRef string, sources []universalWireSynthesisSource, semanticState universalWireSemanticStoryState, now time.Time) error {
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
	if semanticState.StoryID == "" {
		semanticState = rt.universalWireSemanticStoryState(ctx, clusterID, sources, now)
	}
	body, _ := json.Marshal(semanticState)
	cluster, err := rt.ObjectGraph().CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:        objectgraph.UniversalWireStoryClusterObjectKind,
		OwnerID:     universalWirePlatformOwnerID(),
		IdentityKey: clusterID,
		Body:        body,
		Metadata: map[string]any{
			"schema_version":      objectgraph.UniversalWireStoryClusterSchemaVersion,
			"cluster_id":          clusterID,
			"cluster_kind":        "live_sourcecycled_story",
			"world_model_kind":    semanticState.WorldModelKind,
			"semantic_story_id":   semanticState.StoryID,
			"semantic_signature":  semanticState.SemanticSignature,
			"semantic_topics":     semanticState.TopicConcepts,
			"semantic_signals":    semanticState.SignalConcepts,
			"semantic_change":     semanticState.LatestChange,
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
		Title:     universalWireSynthesisDocumentTitle(headline),
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

func universalWireSynthesisDocumentTitle(headline string) string {
	return strings.TrimSpace(firstNonEmpty(headline, "Universal Wire synthesis")) + ".texture"
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

func universalWireSynthesisArticleMarkdown(headline, summary, tension string, state universalWireSemanticStoryState, sources []universalWireSynthesisSource, entities []textureSourceEntity) string {
	summary = strings.TrimSpace(summary)
	if summary == "" || universalWireTextContainsArticleSurfaceHelper(summary) {
		summary = firstNonEmpty(state.SynthesisFrame.SharedAccount, universalWireSynthesisSummaryFromSources(sources))
	}
	tension = strings.TrimSpace(tension)
	if tension == "" || universalWireTextContainsArticleSurfaceHelper(tension) {
		tension = firstNonEmpty(state.SynthesisFrame.Reconciliation, universalWireSynthesisRevisionSentence(sources))
	}
	latestUpdate := strings.TrimSpace(state.SynthesisFrame.LatestUpdate)
	if latestUpdate == "" && len(sources) > 1 {
		latestUpdate = universalWireSourceFactSentence(sources[1], "The second account")
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(headline)
	b.WriteString("\n\n")
	b.WriteString(universalWireEnsureSentence(summary))
	b.WriteString("\n\n")
	if latestUpdate != "" {
		b.WriteString(universalWireEnsureSentence(latestUpdate))
		b.WriteString("\n\n")
	}
	if decision := universalWireSynthesisUpdateDecisionSentence(state); decision != "" {
		b.WriteString(decision)
		b.WriteString("\n\n")
	}
	b.WriteString("Source map:\n")
	for i := 0; i < len(sources) && i < len(entities); i++ {
		account := universalWireSemanticSourceAccount{}
		if i < len(state.SynthesisFrame.SourceAccounts) {
			account = state.SynthesisFrame.SourceAccounts[i]
		}
		b.WriteString("- ")
		b.WriteString(universalWireSourceAccountSentence(sources[i], account))
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("[source](source:%s)", entities[i].EntityID))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(universalWireEnsureSentence(tension))
	return b.String()
}

func universalWireSynthesisUpdateDecisionSentence(state universalWireSemanticStoryState) string {
	decision := state.UpdateDecision
	if decision.Decision == "" {
		return ""
	}
	action := "This article opens a new account"
	switch decision.Decision {
	case "update_existing_story":
		action = "This article updates the existing account"
	case "keep_existing_story":
		action = "This account remains current"
	}
	predicates := universalWireHumanList(universalWireHumanizedPredicates(decision.ContinuityPredicates))
	if predicates == "" {
		predicates = "the observed story continuity"
	}
	split := universalWireHumanList(universalWireHumanizedPredicates(decision.SplitPredicates))
	if split == "" {
		split = "a different timeline, location, affected group, or explanation"
	}
	return universalWireEnsureSentence(action + " because later reporting matched " + predicates + "; it should split only on " + split)
}

func universalWireHumanizedPredicates(predicates []string) []string {
	out := make([]string, 0, len(predicates))
	for _, predicate := range predicates {
		label := strings.ReplaceAll(strings.TrimSpace(predicate), "_", " ")
		if label != "" {
			out = append(out, label)
		}
	}
	return out
}

func universalWireSourceAccountSentence(source universalWireSynthesisSource, account universalWireSemanticSourceAccount) string {
	language := strings.TrimSpace(account.Language)
	if language == "" {
		language = universalWireLanguageName(source.Language)
	}
	if language == "" {
		language = "Source"
	} else {
		language += " account"
	}
	title := strings.TrimSpace(firstNonEmpty(account.Title, source.Title))
	contribution := strings.TrimSpace(account.Contribution)
	if contribution == "" {
		contribution = universalWireHumanList(universalWireArticleSignalPhrases(universalWireKnownConceptSet(strings.Join([]string{source.Title, source.Body}, " "))))
	}
	if contribution == "" {
		contribution = "the developing account"
	}
	if title == "" {
		return universalWireEnsureSentence(language + " contributes " + contribution)
	}
	return universalWireEnsureSentence(language + " " + title + " contributes " + contribution)
}

func universalWireSynthesisSummaryFromSources(sources []universalWireSynthesisSource) string {
	concepts := universalWireSynthesisConcepts(sources)
	topic := universalWireArticleTopicPhrase(concepts)
	signals := universalWireArticleSignalPhrases(concepts)
	switch {
	case len(signals) == 0:
		return "The available reporting describes " + topic + " that remains open to revision as more details arrive."
	case len(signals) == 1:
		return "The available reporting describes " + topic + ", with " + signals[0] + " shaping the latest account."
	default:
		return "The available reporting describes " + topic + ", with " + strings.Join(signals[:len(signals)-1], ", ") + " and " + signals[len(signals)-1] + " shaping the latest account."
	}
}

func universalWireSynthesisRevisionSentence(sources []universalWireSynthesisSource) string {
	concepts := universalWireSynthesisConcepts(sources)
	switch universalWireArticlePrimaryTopic(concepts) {
	case "transport":
		return "This article should be revised if later reporting changes the reopening timetable, passenger impact, or official explanation."
	case "harbor":
		return "This article should be revised if later reporting changes the access restrictions, vessel impact, or official explanation."
	case "energy":
		return "This article should be revised if later reporting changes the outage timeline, affected customers, or official explanation."
	case "health":
		return "This article should be revised if later reporting changes the patient impact, service timeline, or official explanation."
	default:
		return "This article should be revised if later reporting changes the timeline, affected people, or official explanation."
	}
}

func universalWireSourceFactSentence(source universalWireSynthesisSource, prefix string) string {
	concepts := universalWireKnownConceptSet(strings.Join([]string{source.Title, source.Body}, " "))
	signals := universalWireArticleSignalPhrases(concepts)
	title := strings.TrimSpace(source.Title)
	if len(signals) == 0 {
		if title == "" {
			return universalWireEnsureSentence(prefix + " adds another verified detail to the developing account")
		}
		return universalWireEnsureSentence(prefix + " centers on " + title)
	}
	label := prefix
	if title != "" {
		label += " from " + title
	}
	switch len(signals) {
	case 1:
		return universalWireEnsureSentence(label + " adds " + signals[0])
	default:
		return universalWireEnsureSentence(label + " adds " + strings.Join(signals[:len(signals)-1], ", ") + " and " + signals[len(signals)-1])
	}
}

func universalWireSynthesisConcepts(sources []universalWireSynthesisSource) map[string]bool {
	concepts := map[string]bool{}
	for _, source := range sources {
		for concept := range universalWireKnownConceptSet(strings.Join([]string{source.Title, source.Body}, " ")) {
			concepts[concept] = true
		}
	}
	return concepts
}

func universalWireArticleTopicPhrase(concepts map[string]bool) string {
	switch universalWireArticlePrimaryTopic(concepts) {
	case "transport":
		if concepts["signal:rail-corridor"] {
			return "a disrupted rail corridor"
		}
		return "a regional transport disruption"
	case "harbor":
		return "a harbor access disruption"
	case "flood":
		return "a flood-driven disruption"
	case "energy":
		return "a power and grid disruption"
	case "health":
		return "a health-service disruption"
	default:
		return "a developing story"
	}
}

func universalWireArticlePrimaryTopic(concepts map[string]bool) string {
	topics := []string{}
	for concept := range concepts {
		if strings.HasPrefix(concept, "topic:") {
			topics = append(topics, strings.TrimPrefix(concept, "topic:"))
		}
	}
	sort.Strings(topics)
	if len(topics) == 0 {
		return ""
	}
	return topics[0]
}

func universalWireArticleSignalPhrases(concepts map[string]bool) []string {
	candidates := []struct {
		concept string
		phrase  string
	}{
		{"signal:flood", "flooding pressure"},
		{"signal:reopening", "partial reopening"},
		{"signal:delay", "regional delays"},
		{"signal:inspection", "continued inspections"},
		{"signal:strike", "strike disruption"},
		{"signal:harbor-access", "constrained harbor access"},
	}
	out := []string{}
	for _, candidate := range candidates {
		if concepts[candidate.concept] {
			out = append(out, candidate.phrase)
		}
	}
	return out
}

func universalWireTextContainsArticleSurfaceHelper(text string) bool {
	for _, phrase := range universalWireArticleSurfaceHelperPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func universalWireEnsureSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	switch text[len(text)-1] {
	case '.', '!', '?':
		return text
	default:
		return text + "."
	}
}

func universalWireSynthesisHeadline(sources []universalWireSynthesisSource) string {
	if len(sources) == 0 {
		return "Developing story"
	}
	return truncateRunes(sources[0].Title, 96)
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
