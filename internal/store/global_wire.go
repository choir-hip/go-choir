package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const globalWireSeedState = "seeded-source-neighborhood"

var defaultGlobalWireStyleSources = []types.GlobalWireStyleSource{
	{
		ID:         "wire-style",
		Title:      "Style.vtext: Global Wire",
		Label:      "Wire",
		Summary:    "Fast public brief, direct sourcing, visible uncertainty, no oracle voice.",
		SourcePath: "styles/global-wire.style.vtext",
	},
	{
		ID:         "claim-audit-style",
		Title:      "Style.vtext: Claim Audit",
		Label:      "Audit",
		Summary:    "Foregrounds dispute state, evidence gaps, counterclaims, and source standing.",
		SourcePath: "styles/claim-audit.style.vtext",
	},
	{
		ID:         "market-brief-style",
		Title:      "Style.vtext: Market Brief",
		Label:      "Market",
		Summary:    "Emphasizes exposure, second-order effects, timing, and unresolved risks.",
		SourcePath: "styles/market-brief.style.vtext",
	},
}

var defaultGlobalWireStories = []types.GlobalWireStory{
	{
		ID:          "story-supply-resilience",
		Headline:    "Port backlog recedes as carriers warn of uneven inland recovery",
		Dek:         "Lead port indicators improved this week, while rail dwell and warehouse reports still show regional stress.",
		Freshness:   "seed source neighborhood",
		Prominence:  82,
		Tension:     "qualifying evidence",
		ChangeState: "claim narrowed",
		NodeTone:    "live",
		Related:     []string{"story-energy-grid", "story-retail-margins"},
		Manifest: types.GlobalWireSourceManifest{
			Lead: []types.GlobalWireSourceItem{
				{ID: "source-port-authority", Title: "Port authority throughput bulletin", Standing: "official operations bulletin", Role: "lead"},
				{ID: "source-carrier-note", Title: "Carrier service advisory", Standing: "operator disclosure", Role: "lead"},
			},
			Supporting: []types.GlobalWireSourceItem{
				{ID: "source-rail-dwell", Title: "Rail dwell dashboard", Standing: "public logistics metric", Role: "supporting"},
				{ID: "source-warehouse-index", Title: "Warehouse vacancy index", Standing: "industry data", Role: "supporting"},
			},
			Contrary: []types.GlobalWireSourceItem{
				{ID: "source-regional-exporters", Title: "Regional exporters report delays", Standing: "trade association survey", Role: "contrary"},
			},
			Context: []types.GlobalWireSourceItem{
				{ID: "source-ambient-brief", Title: "Ambient corpus: shipping and retail filings", Standing: "bounded context packet", Role: "context"},
			},
		},
		Claims: []string{
			"Container queue times have improved at the port complex.",
			"Inland recovery remains uneven and should not be summarized as resolved.",
			"Retail margin impact depends on regional warehouse exposure.",
		},
		Projections: map[string]string{
			"wire-style":         "Port congestion indicators eased this week, but the recovery remains uneven once inland rail dwell and warehouse data are included. The current platform story treats the port bulletin as lead evidence and keeps the exporter delay survey visible as qualifying evidence.",
			"claim-audit-style":  "The strongest supported claim is narrower than the headline risk suggests: vessel queues have improved. A broader claim that supply chains are normal again is not supported because rail dwell, warehouse vacancy, and exporter surveys still show regional delays.",
			"market-brief-style": "The market read is mixed. Port improvement lowers near-term shipping pressure, but inland bottlenecks leave margin risk concentrated in retailers with regionally exposed inventories and limited warehouse flexibility.",
		},
	},
	{
		ID:          "story-energy-grid",
		Headline:    "Grid operators add reserve alerts as heat forecast shifts north",
		Dek:         "Forecast changes moved stress from the southern peak window toward northern reserve margins.",
		Freshness:   "seed source neighborhood",
		Prominence:  74,
		Tension:     "forecast changed",
		ChangeState: "timeline updated",
		NodeTone:    "changed",
		Related:     []string{"story-supply-resilience", "story-city-air"},
		Manifest: types.GlobalWireSourceManifest{
			Lead: []types.GlobalWireSourceItem{
				{ID: "source-grid-notice", Title: "Regional grid operator reserve notice", Standing: "official grid notice", Role: "lead"},
				{ID: "source-weather-update", Title: "National forecast update", Standing: "meteorological update", Role: "lead"},
			},
			Supporting: []types.GlobalWireSourceItem{
				{ID: "source-demand-model", Title: "Demand forecast model", Standing: "operator model packet", Role: "supporting"},
			},
			Contrary: []types.GlobalWireSourceItem{
				{ID: "source-utility-comment", Title: "Utility says local capacity is adequate", Standing: "utility statement", Role: "contrary"},
			},
			Context: []types.GlobalWireSourceItem{
				{ID: "source-grid-history", Title: "Prior reserve-alert history", Standing: "timeline context", Role: "context"},
			},
		},
		Claims: []string{
			"Reserve concern shifted north with the updated heat forecast.",
			"The alert is operational risk, not proof of shortage.",
			"Local utility statements should be read against regional reserve margins.",
		},
		Projections: map[string]string{
			"wire-style":         "Grid operators issued reserve alerts after the heat forecast moved north. The story is not a shortage call; it is an operational watch with utility statements and prior alert history kept in the evidence neighborhood.",
			"claim-audit-style":  "The alert supports a risk claim, not a failure claim. The contrary utility statement does not negate the regional notice, but it narrows the geography and should stay attached to the StoryGraph.",
			"market-brief-style": "The exposure is timing-sensitive: reserve alerts can move power prices before any outage occurs. The practical signal is regional load stress and hedging pressure rather than confirmed infrastructure failure.",
		},
	},
	{
		ID:          "story-city-air",
		Headline:    "City air monitors show sharp overnight improvement after smoke plume disperses",
		Dek:         "Monitors improved by morning, but health agencies kept cautions for sensitive groups while plume models update.",
		Freshness:   "seed source neighborhood",
		Prominence:  63,
		Tension:     "public guidance lag",
		ChangeState: "status improved",
		NodeTone:    "cooling",
		Related:     []string{"story-energy-grid"},
		Manifest: types.GlobalWireSourceManifest{
			Lead: []types.GlobalWireSourceItem{
				{ID: "source-air-monitors", Title: "City air-quality monitor readings", Standing: "public sensor network", Role: "lead"},
				{ID: "source-health-agency", Title: "Health agency advisory", Standing: "public health guidance", Role: "lead"},
			},
			Supporting: []types.GlobalWireSourceItem{
				{ID: "source-plume-model", Title: "Smoke plume model update", Standing: "forecast model", Role: "supporting"},
			},
			Contrary: []types.GlobalWireSourceItem{
				{ID: "source-community-reports", Title: "Community reports of local haze", Standing: "local observations", Role: "contrary"},
			},
			Context: []types.GlobalWireSourceItem{
				{ID: "source-prior-air-event", Title: "Prior air-quality event timeline", Standing: "historical context", Role: "context"},
			},
		},
		Claims: []string{
			"Sensor readings improved materially overnight.",
			"Sensitive-group caution remains because public-health guidance lags and local haze reports persist.",
			"The story should track monitor changes over time instead of freezing the morning state.",
		},
		Projections: map[string]string{
			"wire-style":         "Air-quality readings improved sharply after the smoke plume dispersed overnight. Health guidance remains more cautious for sensitive groups, so the story keeps monitor data, plume models, and local reports in view.",
			"claim-audit-style":  "The evidence supports improvement, not all-clear. The health advisory and community haze reports qualify the monitor trend and prevent the platform story from flattening a changing condition into a single verdict.",
			"market-brief-style": "The operational effect is localized but real: school, transit, and outdoor-work decisions may lag sensor improvement because public guidance and local observations update on different cadences.",
		},
	},
}

// ListGlobalWireStories returns the owner's durable StoryGraph records, seeding
// the initial source-neighborhood graph if the owner has not opened Global Wire
// before.
func (s *Store) ListGlobalWireStories(ctx context.Context, ownerID string) ([]types.GlobalWireStory, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if err := s.ensureDefaultGlobalWireStories(ctx, ownerID); err != nil {
		return nil, err
	}
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT owner_id, story_id, headline, dek, freshness, prominence, tension, change_state,
		        node_tone, related_json, manifest_json, claims_json, projections_json,
		        style_sources_json, story_vtext_doc_id, source_state, created_at, updated_at
		   FROM global_wire_story_graphs
		  WHERE owner_id = ?
		  ORDER BY prominence DESC, updated_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list global wire stories: %w", err)
	}
	defer rows.Close()

	var stories []types.GlobalWireStory
	for rows.Next() {
		story, err := scanGlobalWireStory(rows)
		if err != nil {
			return nil, err
		}
		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire stories: %w", err)
	}
	if err := s.attachGlobalWireProjectionRefs(ctx, ownerID, stories); err != nil {
		return nil, err
	}
	return stories, nil
}

// GetGlobalWireStory returns one owner-scoped StoryGraph node.
func (s *Store) GetGlobalWireStory(ctx context.Context, ownerID, storyID string) (types.GlobalWireStory, error) {
	ownerID = strings.TrimSpace(ownerID)
	storyID = strings.TrimSpace(storyID)
	if ownerID == "" || storyID == "" {
		return types.GlobalWireStory{}, ErrNotFound
	}
	if err := s.ensureDefaultGlobalWireStories(ctx, ownerID); err != nil {
		return types.GlobalWireStory{}, err
	}
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, story_id, headline, dek, freshness, prominence, tension, change_state,
		        node_tone, related_json, manifest_json, claims_json, projections_json,
		        style_sources_json, story_vtext_doc_id, source_state, created_at, updated_at
		   FROM global_wire_story_graphs
		  WHERE owner_id = ? AND story_id = ?`,
		ownerID,
		storyID,
	)
	story, err := scanGlobalWireStory(row)
	if err != nil {
		return types.GlobalWireStory{}, err
	}
	stories := []types.GlobalWireStory{story}
	if err := s.attachGlobalWireProjectionRefs(ctx, ownerID, stories); err != nil {
		return types.GlobalWireStory{}, err
	}
	return stories[0], nil
}

func (s *Store) attachGlobalWireProjectionRefs(ctx context.Context, ownerID string, stories []types.GlobalWireStory) error {
	for i := range stories {
		rows, err := s.readDB.QueryContext(ctx,
			`SELECT style_id, story_vtext_doc_id, projection_text
			   FROM global_wire_story_projections
			  WHERE owner_id = ? AND story_id = ?`,
			ownerID,
			stories[i].ID,
		)
		if err != nil {
			return fmt.Errorf("list global wire projection refs: %w", err)
		}
		refs := map[string]string{}
		for rows.Next() {
			var styleID, docID, projectionText string
			if err := rows.Scan(&styleID, &docID, &projectionText); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan global wire projection ref: %w", err)
			}
			refs[styleID] = docID
			if strings.TrimSpace(styleID) != "" && strings.TrimSpace(projectionText) != "" {
				if stories[i].Projections == nil {
					stories[i].Projections = map[string]string{}
				}
				stories[i].Projections[styleID] = projectionText
			}
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close global wire projection refs: %w", err)
		}
		if len(refs) > 0 {
			stories[i].ProjectionVTextDocs = refs
		}
	}
	return nil
}

func (s *Store) ensureDefaultGlobalWireStories(ctx context.Context, ownerID string) error {
	var count int
	if err := s.readDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM global_wire_story_graphs WHERE owner_id = ?`,
		ownerID,
	).Scan(&count); err != nil {
		return fmt.Errorf("count global wire stories: %w", err)
	}
	if count > 0 {
		return s.ensureExistingGlobalWireArticleVTextRevisions(ctx, ownerID)
	}
	now := time.Now().UTC()
	styleSources, err := s.ensureDefaultGlobalWireStyleVTexts(ctx, ownerID, now)
	if err != nil {
		return err
	}
	for _, seed := range defaultGlobalWireStories {
		seed.OwnerID = ownerID
		seed.StyleSources = styleSources
		seed.SourceState = globalWireSeedState
		seed.CreatedAt = now
		seed.UpdatedAt = now
		sourceBackedStory, err := s.ensureGlobalWireSourceItems(ctx, ownerID, seed, now)
		if err != nil {
			return err
		}
		sourceBackedStory.StyleSources = styleSources
		docID, err := s.createGlobalWireSeedVText(ctx, ownerID, sourceBackedStory.Headline, globalWireStoryVTextContent(sourceBackedStory, nil), globalWireStoryVTextCitations(sourceBackedStory), map[string]any{
			"created_from":    "global_wire_storygraph_seed",
			"storygraph_id":   sourceBackedStory.ID,
			"source_state":    globalWireSeedState,
			"source_entities": globalWireSourceEntities(sourceBackedStory),
		}, now)
		if err != nil {
			return err
		}
		sourceBackedStory.StoryVTextDoc = docID
		projectionDocs, err := s.ensureGlobalWireStoryProjections(ctx, ownerID, sourceBackedStory, styleSources, now)
		if err != nil {
			return err
		}
		sourceBackedStory.ProjectionVTextDocs = projectionDocs
		if err := s.UpsertGlobalWireStory(ctx, sourceBackedStory); err != nil {
			return err
		}
	}
	return s.ensureExistingGlobalWireArticleVTextRevisions(ctx, ownerID)
}

func (s *Store) ensureExistingGlobalWireArticleVTextRevisions(ctx context.Context, ownerID string) error {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT owner_id, story_id, headline, dek, freshness, prominence, tension,
		        change_state, node_tone, related_json, manifest_json, claims_json,
		        projections_json, style_sources_json, story_vtext_doc_id,
		        source_state, created_at, updated_at
		   FROM global_wire_story_graphs
		  WHERE owner_id = ?`,
		ownerID,
	)
	if err != nil {
		return fmt.Errorf("list existing global wire stories for vtext repair: %w", err)
	}
	var stories []types.GlobalWireStory
	for rows.Next() {
		story, err := scanGlobalWireStory(rows)
		if err != nil {
			_ = rows.Close()
			return err
		}
		stories = append(stories, story)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close existing global wire stories for vtext repair: %w", err)
	}
	if err := s.attachGlobalWireProjectionRefs(ctx, ownerID, stories); err != nil {
		return err
	}
	relatedStories := globalWireStoriesByID(stories)
	now := time.Now().UTC()
	for _, story := range stories {
		if strings.TrimSpace(story.StoryVTextDoc) != "" {
			if err := s.repairGlobalWireArticleVTextRevision(ctx, ownerID, story.StoryVTextDoc, story.Headline, globalWireStoryVTextContent(story, relatedStories), globalWireStoryVTextCitations(story), map[string]any{
				"created_from":    "global_wire_article_body_repair",
				"storygraph_id":   story.ID,
				"source_state":    story.SourceState,
				"source_entities": globalWireSourceEntities(story),
				"related_vtexts":  globalWireRelatedVTextEntities(story, relatedStories),
			}, now); err != nil {
				return err
			}
		}
		for _, style := range story.StyleSources {
			docID := strings.TrimSpace(story.ProjectionVTextDocs[style.ID])
			if docID == "" || docID == story.StoryVTextDoc {
				continue
			}
			projection := strings.TrimSpace(story.Projections[style.ID])
			if projection == "" {
				continue
			}
			if err := s.repairGlobalWireArticleVTextRevision(ctx, ownerID, docID, story.Headline+" - "+style.Label+" projection", globalWireProjectionVTextContent(story, style, projection), globalWireProjectionVTextCitations(story, style), map[string]any{
				"created_from":    "global_wire_projection_body_repair",
				"storygraph_id":   story.ID,
				"style_id":        style.ID,
				"style_doc_id":    style.DocID,
				"source_entities": globalWireSourceEntities(story),
			}, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) repairGlobalWireArticleVTextRevision(ctx context.Context, ownerID, docID, title, content string, citations []types.Citation, metadata map[string]any, now time.Time) error {
	doc, err := s.GetDocument(ctx, docID, ownerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return nil
	}
	current, err := s.GetRevision(ctx, doc.CurrentRevisionID, ownerID)
	if err != nil {
		return err
	}
	if !globalWireArticleVTextNeedsBodyRepair(current.Content) {
		return nil
	}
	if strings.TrimSpace(title) != "" && doc.Title != title {
		doc.Title = title
		doc.UpdatedAt = now
		if err := s.UpdateDocument(ctx, doc); err != nil {
			return err
		}
	}
	metadata["repaired_from_revision_id"] = current.RevisionID
	citationsJSON, err := json.Marshal(citations)
	if err != nil {
		return fmt.Errorf("marshal global wire repaired citations: %w", err)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal global wire repaired metadata: %w", err)
	}
	return s.CreateRevision(ctx, types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "Global Wire",
		Content:          content,
		Citations:        citationsJSON,
		Metadata:         metadataJSON,
		ParentRevisionID: current.RevisionID,
		CreatedAt:        now,
	})
}

func globalWireArticleVTextNeedsBodyRepair(content string) bool {
	for _, token := range []string{
		"Style source:",
		"Story id:",
		"## Projection",
		"\nProjection\n",
		"\nClaims\n",
		"Source Manifest",
		"Related VTexts",
		"Non-oracle note",
		"My Edit",
		"Projection relation:",
		"Evidence Invariant",
		"Draft state:",
		"Projection review id:",
		"Projection Review Approval",
		"The current version keeps",
		"This VText should be read alongside the related",
	} {
		if strings.Contains(content, token) {
			return true
		}
	}
	return false
}

func globalWireStoriesByID(stories []types.GlobalWireStory) map[string]types.GlobalWireStory {
	out := make(map[string]types.GlobalWireStory, len(stories))
	for _, story := range stories {
		if strings.TrimSpace(story.ID) != "" {
			out[story.ID] = story
		}
	}
	return out
}

func (s *Store) ensureGlobalWireStoryProjections(ctx context.Context, ownerID string, story types.GlobalWireStory, styles []types.GlobalWireStyleSource, now time.Time) (map[string]string, error) {
	out := map[string]string{}
	for i, style := range styles {
		text := strings.TrimSpace(story.Projections[style.ID])
		if text == "" {
			continue
		}
		storyDocID := story.StoryVTextDoc
		if i > 0 {
			content := globalWireProjectionVTextContent(story, style, text)
			docID, err := s.createGlobalWireSeedVText(ctx, ownerID, story.Headline+" - "+style.Label+" projection", content, globalWireProjectionVTextCitations(story, style), map[string]any{
				"created_from":    "global_wire_style_projection_seed",
				"storygraph_id":   story.ID,
				"style_id":        style.ID,
				"style_doc_id":    style.DocID,
				"source_entities": globalWireSourceEntities(story),
			}, now)
			if err != nil {
				return nil, err
			}
			storyDocID = docID
		}
		out[style.ID] = storyDocID
		if err := s.UpsertGlobalWireStoryProjection(ctx, types.GlobalWireStoryProjection{
			ID:          "global-wire-projection-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+story.ID+":"+style.ID)).String(),
			OwnerID:     ownerID,
			StoryID:     story.ID,
			StyleID:     style.ID,
			StyleDocID:  style.DocID,
			StoryDocID:  storyDocID,
			ContextJSON: `{"audience":"global-wire","task":"news_projection"}`,
			Text:        text,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) ensureGlobalWireSourceItems(ctx context.Context, ownerID string, story types.GlobalWireStory, now time.Time) (types.GlobalWireStory, error) {
	var err error
	story.Manifest.Lead, err = s.ensureGlobalWireSourceTier(ctx, ownerID, story, "lead", story.Manifest.Lead, now)
	if err != nil {
		return types.GlobalWireStory{}, err
	}
	story.Manifest.Supporting, err = s.ensureGlobalWireSourceTier(ctx, ownerID, story, "supporting", story.Manifest.Supporting, now)
	if err != nil {
		return types.GlobalWireStory{}, err
	}
	story.Manifest.Contrary, err = s.ensureGlobalWireSourceTier(ctx, ownerID, story, "contrary", story.Manifest.Contrary, now)
	if err != nil {
		return types.GlobalWireStory{}, err
	}
	story.Manifest.Context, err = s.ensureGlobalWireSourceTier(ctx, ownerID, story, "context", story.Manifest.Context, now)
	if err != nil {
		return types.GlobalWireStory{}, err
	}
	return story, nil
}

func (s *Store) ensureGlobalWireSourceTier(ctx context.Context, ownerID string, story types.GlobalWireStory, tier string, items []types.GlobalWireSourceItem, now time.Time) ([]types.GlobalWireSourceItem, error) {
	out := make([]types.GlobalWireSourceItem, 0, len(items))
	for _, item := range items {
		item.Role = firstNonEmptyString(item.Role, tier)
		item.CanonicalURL = "choir://global-wire/source/" + item.ID
		content := strings.Join([]string{
			"# " + item.Title,
			"",
			"This source backs a Global Wire source-neighborhood entry for " + story.Headline + ".",
			"",
			"It is seed evidence until replaced by live Source Service ingestion.",
		}, "\n")
		metadata, err := json.Marshal(map[string]any{
			"schema":        "choir.global_wire_source_item.v1",
			"story_id":      story.ID,
			"source_id":     item.ID,
			"relation":      tier,
			"source_medium": item.Role,
			"source_state":  globalWireSeedState,
			"source_system": "global_wire",
		})
		if err != nil {
			return nil, fmt.Errorf("marshal global wire source metadata: %w", err)
		}
		provenance, err := json.Marshal(map[string]any{
			"created_from": "global_wire_seed_source_item",
			"story_id":     story.ID,
			"source_id":    item.ID,
			"created_at":   now.UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal global wire source provenance: %w", err)
		}
		contentItem := types.ContentItem{
			ContentID:    uuid.NewString(),
			OwnerID:      ownerID,
			SourceType:   "text",
			MediaType:    "text/markdown",
			AppHint:      "global-wire",
			Title:        item.Title,
			CanonicalURL: item.CanonicalURL,
			TextContent:  content,
			ContentHash:  uuid.NewSHA1(uuid.NameSpaceURL, []byte(ownerID+":"+story.ID+":"+item.ID)).String(),
			Metadata:     metadata,
			Provenance:   provenance,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.CreateContentItem(ctx, contentItem); err != nil {
			return nil, err
		}
		item.ContentID = contentItem.ContentID
		out = append(out, item)
	}
	return out, nil
}

func globalWireSourceCitations(story types.GlobalWireStory) []types.Citation {
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

func globalWireStoryVTextCitations(story types.GlobalWireStory) []types.Citation {
	citations := []types.Citation{
		{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
	}
	if len(story.StyleSources) > 0 {
		style := story.StyleSources[0]
		citations = append([]types.Citation{{ID: "style-source", Type: "vtext", Value: style.SourcePath, Label: style.Title}}, citations...)
	}
	return append(citations, globalWireSourceCitations(story)...)
}

func globalWireProjectionVTextCitations(story types.GlobalWireStory, style types.GlobalWireStyleSource) []types.Citation {
	citations := []types.Citation{
		{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
	}
	if strings.TrimSpace(style.SourcePath) != "" || strings.TrimSpace(style.Title) != "" {
		citations = append([]types.Citation{{ID: "style-source", Type: "vtext", Value: style.SourcePath, Label: style.Title}}, citations...)
	}
	return append(citations, globalWireSourceCitations(story)...)
}

func globalWireSourceEntities(story types.GlobalWireStory) []map[string]any {
	all := []types.GlobalWireSourceItem{}
	all = append(all, story.Manifest.Lead...)
	all = append(all, story.Manifest.Supporting...)
	all = append(all, story.Manifest.Contrary...)
	all = append(all, story.Manifest.Context...)
	entities := make([]map[string]any, 0, len(all))
	for _, item := range all {
		if strings.TrimSpace(item.ContentID) == "" {
			continue
		}
		entityID := globalWireSourceEntityID(item)
		if entityID == "" {
			continue
		}
		entities = append(entities, map[string]any{
			"entity_id": entityID,
			"kind":      "content_item",
			"label":     item.Title,
			"target": map[string]any{
				"target_kind":   "content_item",
				"content_id":    item.ContentID,
				"canonical_url": item.CanonicalURL,
			},
			"selectors": []map[string]any{{"selector_kind": "whole_resource"}},
			"display": map[string]any{
				"inline_mode":       "collapsed_citation",
				"expanded_mode":     "source_card",
				"open_surface":      "source",
				"default_collapsed": true,
			},
			"evidence": map[string]any{
				"state":          "available",
				"research_state": "represented",
				"relation":       firstNonEmptyString(item.Role, "context"),
			},
			"provenance": map[string]any{
				"created_by":            "global_wire",
				"rights_scope":          "private_user_source",
				"untrusted_source_text": true,
			},
		})
	}
	return entities
}

func globalWireSourceEntityID(item types.GlobalWireSourceItem) string {
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

func globalWireSourceRefLabel(item types.GlobalWireSourceItem, fallback int) string {
	label := strings.TrimSpace(item.Title)
	if label == "" {
		label = fmt.Sprintf("source %d", fallback)
	}
	entityID := globalWireSourceEntityID(item)
	if entityID == "" {
		return label
	}
	return fmt.Sprintf("[%s](source:%s)", label, entityID)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Store) ensureDefaultGlobalWireStyleVTexts(ctx context.Context, ownerID string, now time.Time) ([]types.GlobalWireStyleSource, error) {
	out := make([]types.GlobalWireStyleSource, 0, len(defaultGlobalWireStyleSources))
	for _, style := range defaultGlobalWireStyleSources {
		docID, err := s.createGlobalWireSeedVText(ctx, ownerID, style.Title, globalWireStyleVTextContent(style), []types.Citation{
			{ID: "style-vtext", Type: "vtext", Value: style.SourcePath, Label: style.Title},
		}, map[string]any{
			"created_from": "global_wire_style_seed",
			"style_id":     style.ID,
			"source_path":  style.SourcePath,
		}, now)
		if err != nil {
			return nil, err
		}
		style.DocID = docID
		out = append(out, style)
	}
	return out, nil
}

func (s *Store) createGlobalWireSeedVText(ctx context.Context, ownerID, title, content string, citations []types.Citation, metadata map[string]any, now time.Time) (string, error) {
	docID := uuid.NewString()
	revID := uuid.NewString()
	doc := types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		return "", err
	}
	citationsJSON, err := json.Marshal(citations)
	if err != nil {
		return "", fmt.Errorf("marshal global wire seed citations: %w", err)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal global wire seed metadata: %w", err)
	}
	rev := types.Revision{
		RevisionID:  revID,
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "Global Wire",
		Content:     content,
		Citations:   citationsJSON,
		Metadata:    metadataJSON,
		CreatedAt:   now,
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		return "", err
	}
	return docID, nil
}

func globalWireStoryVTextContent(story types.GlobalWireStory, relatedStories map[string]types.GlobalWireStory) string {
	lead := globalWireFirstSourceRef(story.Manifest.Lead, 1)
	secondLead := globalWireNthSourceRef(story.Manifest.Lead, 1, 2)
	support := globalWireFirstSourceRef(story.Manifest.Supporting, 3)
	qualifier := globalWireFirstSourceRef(story.Manifest.Contrary, 5)
	context := globalWireFirstSourceRef(story.Manifest.Context, 6)
	styleProjection := strings.TrimSpace(story.Projections["wire-style"])
	auditProjection := strings.TrimSpace(story.Projections["claim-audit-style"])
	marketProjection := strings.TrimSpace(story.Projections["market-brief-style"])

	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
	}
	if lead != "" {
		leadSentence := "The lead signal is still the narrowest one: " + lead + " supports the update without turning it into an all-clear."
		if secondLead != "" {
			leadSentence += " " + secondLead + " keeps the operator view attached to the story, so the article is anchored in both public and operational evidence."
		}
		lines = append(lines, leadSentence, "")
	}
	if styleProjection != "" {
		lines = append(lines, styleProjection, "")
	}
	if support != "" || qualifier != "" {
		paragraph := "The source neighborhood keeps the story open rather than flattening it into a verdict."
		if support != "" {
			paragraph += " " + support + " adds the supporting context that explains why the headline improvement still has limits."
		}
		if qualifier != "" {
			paragraph += " " + qualifier + " is retained as qualifying evidence, not because it outranks the lead source, but because it prevents the platform article from overstating certainty."
		}
		lines = append(lines, paragraph, "")
	}
	if len(story.Claims) > 0 {
		lines = append(lines, "The article's working claims are deliberately bounded. "+globalWireClaimsSentence(story.Claims), "")
	}
	if auditProjection != "" {
		lines = append(lines, "A claim-audit reading narrows the public takeaway: "+auditProjection, "")
	}
	if marketProjection != "" {
		lines = append(lines, "The market and second-order read is different from the wire lead. "+marketProjection, "")
	}
	if context != "" {
		lines = append(lines, "Background remains part of the article rather than a hidden appendix. "+context+" supplies the broader context that future revisions can walk when the story updates.", "")
	}
	if related := globalWireRelatedVTextSentence(story, relatedStories); related != "" {
		lines = append(lines, related, "")
	}
	lines = append(lines,
		"This is a living Global Wire VText. Later processor and reconciler updates should revise this article as ordinary VText versions, with corrections treated as progress rather than as a separate product surface.",
		"",
	)
	return strings.Join(lines, "\n")
}

func globalWireFirstSourceRef(items []types.GlobalWireSourceItem, marker int) string {
	return globalWireNthSourceRef(items, 0, marker)
}

func globalWireNthSourceRef(items []types.GlobalWireSourceItem, index, marker int) string {
	if index < 0 || index >= len(items) {
		return ""
	}
	return globalWireSourceRefLabel(items[index], marker)
}

func globalWireClaimsSentence(claims []string) string {
	clean := make([]string, 0, len(claims))
	for _, claim := range claims {
		claim = strings.TrimSpace(claim)
		if claim != "" {
			clean = append(clean, claim)
		}
	}
	switch len(clean) {
	case 0:
		return ""
	case 1:
		return clean[0]
	case 2:
		return clean[0] + " " + clean[1]
	default:
		return clean[0] + " " + clean[1] + " " + clean[2]
	}
}

func globalWireRelatedVTextSentence(story types.GlobalWireStory, relatedStories map[string]types.GlobalWireStory) string {
	labels := make([]string, 0, len(story.Related))
	for _, id := range story.Related {
		if label := globalWireRelatedVTextRefLabel(id, relatedStories); label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return ""
	}
	if len(labels) == 1 {
		return "This article transcludes the related " + labels[0] + " VText so reconcilers can review cross-story updates without flattening the relationship into a list."
	}
	return "This article transcludes the related " + strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1] + " VTexts so reconcilers can review cross-story updates without flattening the relationship into a list."
}

func globalWireRelatedVTextRefLabel(id string, relatedStories map[string]types.GlobalWireStory) string {
	related, ok := relatedStories[strings.TrimSpace(id)]
	if !ok || strings.TrimSpace(related.StoryVTextDoc) == "" {
		return globalWireRelatedStoryLabel(id)
	}
	label := globalWireRelatedStoryLabel(id)
	if label == "" {
		label = related.Headline
	}
	return fmt.Sprintf("[%s](vtext:%s)", label, related.StoryVTextDoc)
}

func globalWireRelatedVTextEntities(story types.GlobalWireStory, relatedStories map[string]types.GlobalWireStory) []map[string]any {
	entities := make([]map[string]any, 0, len(story.Related))
	for _, id := range story.Related {
		related, ok := relatedStories[strings.TrimSpace(id)]
		if !ok || strings.TrimSpace(related.StoryVTextDoc) == "" {
			continue
		}
		label := globalWireRelatedStoryLabel(id)
		if label == "" {
			label = related.Headline
		}
		entities = append(entities, map[string]any{
			"entity_id": "gw-vtext-" + strings.TrimPrefix(globalWireSourceEntityID(types.GlobalWireSourceItem{ID: related.ID}), "gw-src-"),
			"label":     label,
			"title":     related.Headline,
			"target": map[string]any{
				"target_kind": "vtext_document",
				"doc_id":      related.StoryVTextDoc,
				"story_id":    related.ID,
			},
			"transclusion": map[string]any{
				"snapshot_text": related.Dek,
				"relation":      "related_story",
			},
			"provenance": map[string]any{
				"created_by": "global_wire",
				"source":     "global_wire_related_story_index",
			},
		})
	}
	return entities
}

func globalWireRelatedStoryLabel(id string) string {
	switch strings.TrimSpace(id) {
	case "story-supply-resilience":
		return "port and inland recovery"
	case "story-energy-grid":
		return "grid reserve-alert"
	case "story-city-air":
		return "city air-quality"
	case "story-retail-margins":
		return "retail margin exposure"
	default:
		return ""
	}
}

func globalWireStyleVTextContent(style types.GlobalWireStyleSource) string {
	return strings.Join([]string{
		"# " + style.Title,
		"",
		style.Summary,
		"",
		"## Applies To",
		"",
		"- Global Wire article revisions",
		"- VText revision prompts",
		"- News reader and Autoradio traversal",
		"",
		"## Guardrails",
		"",
		"- Preserve the article's source neighborhood and open uncertainty.",
		"- Change framing and salience without inventing evidence.",
		"- Keep uncertainty and corrections visible.",
		"- Cite this Style.vtext when it materially shapes a projection.",
	}, "\n")
}

func globalWireProjectionVTextContent(story types.GlobalWireStory, style types.GlobalWireStyleSource, projection string) string {
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		projection,
		"",
	}
	if len(story.Manifest.Lead) > 0 {
		lines = append(lines, "The lead source for this version is "+globalWireSourceRefLabel(story.Manifest.Lead[0], 1)+".")
	}
	return strings.Join(lines, "\n")
}

// UpsertGlobalWireStory persists one StoryGraph node.
func (s *Store) UpsertGlobalWireStory(ctx context.Context, rec types.GlobalWireStory) error {
	now := rec.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	relatedJSON, err := json.Marshal(rec.Related)
	if err != nil {
		return fmt.Errorf("marshal global wire related stories: %w", err)
	}
	manifestJSON, err := json.Marshal(rec.Manifest)
	if err != nil {
		return fmt.Errorf("marshal global wire manifest: %w", err)
	}
	claimsJSON, err := json.Marshal(rec.Claims)
	if err != nil {
		return fmt.Errorf("marshal global wire claims: %w", err)
	}
	projectionsJSON, err := json.Marshal(rec.Projections)
	if err != nil {
		return fmt.Errorf("marshal global wire projections: %w", err)
	}
	styleSourcesJSON, err := json.Marshal(rec.StyleSources)
	if err != nil {
		return fmt.Errorf("marshal global wire style sources: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_story_graphs (
			owner_id, story_id, headline, dek, freshness, prominence, tension, change_state,
			node_tone, related_json, manifest_json, claims_json, projections_json,
			style_sources_json, story_vtext_doc_id, source_state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			headline = VALUES(headline),
			dek = VALUES(dek),
			freshness = VALUES(freshness),
			prominence = VALUES(prominence),
			tension = VALUES(tension),
			change_state = VALUES(change_state),
			node_tone = VALUES(node_tone),
			related_json = VALUES(related_json),
			manifest_json = VALUES(manifest_json),
			claims_json = VALUES(claims_json),
			projections_json = VALUES(projections_json),
			style_sources_json = VALUES(style_sources_json),
			story_vtext_doc_id = VALUES(story_vtext_doc_id),
			source_state = VALUES(source_state),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ID,
		sanitizeStoreText(rec.Headline),
		sanitizeStoreText(rec.Dek),
		rec.Freshness,
		rec.Prominence,
		rec.Tension,
		rec.ChangeState,
		rec.NodeTone,
		string(relatedJSON),
		string(manifestJSON),
		string(claimsJSON),
		string(projectionsJSON),
		string(styleSourcesJSON),
		rec.StoryVTextDoc,
		rec.SourceState,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert global wire story: %w", err)
	}
	return nil
}

// CreateGlobalWireContribution stores a user-owned contribution in the
// research/reconciliation queue.
func (s *Store) CreateGlobalWireContribution(ctx context.Context, rec types.GlobalWireContribution) (types.GlobalWireContribution, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.ResearchState == "" {
		rec.ResearchState = "pending-researcher-review"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_contributions (
			owner_id, contribution_id, story_id, kind, headline, content,
			source_content_id, user_vtext_doc_id, research_state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.Kind,
		sanitizeStoreText(rec.Headline),
		sanitizeStoreText(rec.Text),
		rec.SourceContentID,
		rec.UserVTextDocID,
		rec.ResearchState,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireContribution{}, fmt.Errorf("create global wire contribution: %w", err)
	}
	return rec, nil
}

// GetGlobalWireContribution returns one owner-scoped contribution.
func (s *Store) GetGlobalWireContribution(ctx context.Context, ownerID, contributionID string) (types.GlobalWireContribution, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, contribution_id, story_id, kind, headline, content,
		        source_content_id, user_vtext_doc_id, research_state, created_at, updated_at
		   FROM global_wire_contributions
		  WHERE owner_id = ? AND contribution_id = ?`,
		ownerID,
		contributionID,
	)
	return scanGlobalWireContribution(row)
}

// UpdateGlobalWireContributionResearchState updates queue state without
// changing the platform StoryGraph.
func (s *Store) UpdateGlobalWireContributionResearchState(ctx context.Context, ownerID, contributionID, researchState string) (types.GlobalWireContribution, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_contributions
		    SET research_state = ?, updated_at = ?
		  WHERE owner_id = ? AND contribution_id = ?`,
		strings.TrimSpace(researchState),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		contributionID,
	)
	if err != nil {
		return types.GlobalWireContribution{}, fmt.Errorf("update global wire contribution state: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireContribution{}, ErrNotFound
	}
	return s.GetGlobalWireContribution(ctx, ownerID, contributionID)
}

// CreateGlobalWireReconciliationDecision records a reviewer/researcher decision
// artifact over a contribution.
func (s *Store) CreateGlobalWireReconciliationDecision(ctx context.Context, rec types.GlobalWireReconciliationDecision) (types.GlobalWireReconciliationDecision, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_reconciliation_decisions (
			owner_id, decision_id, contribution_id, story_id, decision, note,
			source_content_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.ContributionID,
		rec.StoryID,
		rec.Decision,
		sanitizeStoreText(rec.Note),
		rec.SourceContentID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireReconciliationDecision{}, fmt.Errorf("create global wire reconciliation decision: %w", err)
	}
	return rec, nil
}

// ListGlobalWireReconciliationDecisions lists recent owner-scoped decision
// artifacts, optionally narrowed to one story.
func (s *Store) ListGlobalWireReconciliationDecisions(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireReconciliationDecision, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, decision_id, contribution_id, story_id, decision, note,
	                source_content_id, created_at
	           FROM global_wire_reconciliation_decisions
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire reconciliation decisions: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireReconciliationDecision
	for rows.Next() {
		rec, err := scanGlobalWireReconciliationDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire reconciliation decisions: %w", err)
	}
	return out, nil
}

// UpsertGlobalWireGraphUpdateCandidate persists a non-mutating StoryGraph
// update proposal produced from a reconciliation decision.
func (s *Store) UpsertGlobalWireGraphUpdateCandidate(ctx context.Context, rec types.GlobalWireGraphUpdateCandidate) (types.GlobalWireGraphUpdateCandidate, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	if rec.Status == "" {
		rec.Status = "candidate-review"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_graph_update_candidates (
			owner_id, candidate_id, story_id, contribution_id, decision_id,
			source_content_id, candidate_kind, title, summary, source_tier,
			edge_kind, projection_action, status, rationale, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			source_content_id = VALUES(source_content_id),
			candidate_kind = VALUES(candidate_kind),
			title = VALUES(title),
			summary = VALUES(summary),
			source_tier = VALUES(source_tier),
			edge_kind = VALUES(edge_kind),
			projection_action = VALUES(projection_action),
			status = VALUES(status),
			rationale = VALUES(rationale),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.ContributionID,
		rec.DecisionID,
		rec.SourceContentID,
		rec.CandidateKind,
		sanitizeStoreText(rec.Title),
		sanitizeStoreText(rec.Summary),
		rec.SourceTier,
		rec.EdgeKind,
		rec.ProjectionAction,
		rec.Status,
		sanitizeStoreText(rec.Rationale),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireGraphUpdateCandidate{}, fmt.Errorf("upsert global wire graph update candidate: %w", err)
	}
	return rec, nil
}

// GetGlobalWireGraphUpdateCandidate returns one owner-scoped graph proposal.
func (s *Store) GetGlobalWireGraphUpdateCandidate(ctx context.Context, ownerID, candidateID string) (types.GlobalWireGraphUpdateCandidate, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, candidate_id, story_id, contribution_id, decision_id,
		        source_content_id, candidate_kind, title, summary, source_tier,
		        edge_kind, projection_action, status, rationale, created_at, updated_at
		   FROM global_wire_graph_update_candidates
		  WHERE owner_id = ? AND candidate_id = ?`,
		ownerID,
		candidateID,
	)
	return scanGlobalWireGraphUpdateCandidate(row)
}

// UpdateGlobalWireGraphUpdateCandidateStatus updates review state without
// applying a StoryGraph mutation by itself.
func (s *Store) UpdateGlobalWireGraphUpdateCandidateStatus(ctx context.Context, ownerID, candidateID, status string) (types.GlobalWireGraphUpdateCandidate, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_graph_update_candidates
		    SET status = ?, updated_at = ?
		  WHERE owner_id = ? AND candidate_id = ?`,
		strings.TrimSpace(status),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		candidateID,
	)
	if err != nil {
		return types.GlobalWireGraphUpdateCandidate{}, fmt.Errorf("update global wire graph candidate status: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireGraphUpdateCandidate{}, ErrNotFound
	}
	return s.GetGlobalWireGraphUpdateCandidate(ctx, ownerID, candidateID)
}

// ListGlobalWireGraphUpdateCandidates lists owner-scoped non-mutating graph
// proposals, optionally narrowed to one story.
func (s *Store) ListGlobalWireGraphUpdateCandidates(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireGraphUpdateCandidate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, candidate_id, story_id, contribution_id, decision_id,
	                source_content_id, candidate_kind, title, summary, source_tier,
	                edge_kind, projection_action, status, rationale, created_at, updated_at
	           FROM global_wire_graph_update_candidates
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire graph update candidates: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireGraphUpdateCandidate
	for rows.Next() {
		rec, err := scanGlobalWireGraphUpdateCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire graph update candidates: %w", err)
	}
	return out, nil
}

// CreateGlobalWireGraphPromotionDecision records an explicit platform review
// decision over a graph-update candidate.
func (s *Store) CreateGlobalWireGraphPromotionDecision(ctx context.Context, rec types.GlobalWireGraphPromotionDecision) (types.GlobalWireGraphPromotionDecision, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_graph_promotion_decisions (
			owner_id, promotion_id, candidate_id, story_id, decision, note,
			applied_change, source_content_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.CandidateID,
		rec.StoryID,
		rec.Decision,
		sanitizeStoreText(rec.Note),
		sanitizeStoreText(rec.AppliedChange),
		rec.SourceContentID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireGraphPromotionDecision{}, fmt.Errorf("create global wire graph promotion decision: %w", err)
	}
	return rec, nil
}

// ListGlobalWireGraphPromotionDecisions lists platform review decisions,
// optionally narrowed to one story.
func (s *Store) ListGlobalWireGraphPromotionDecisions(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireGraphPromotionDecision, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, promotion_id, candidate_id, story_id, decision, note,
	                applied_change, source_content_id, created_at
	           FROM global_wire_graph_promotion_decisions
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire graph promotion decisions: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireGraphPromotionDecision
	for rows.Next() {
		rec, err := scanGlobalWireGraphPromotionDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire graph promotion decisions: %w", err)
	}
	return out, nil
}

// CreateGlobalWireSourceRefreshRun records a bounded Source Service refresh
// pass and the review artifacts it produced.
func (s *Store) CreateGlobalWireSourceRefreshRun(ctx context.Context, rec types.GlobalWireSourceRefreshRun) (types.GlobalWireSourceRefreshRun, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_source_refresh_runs (
			owner_id, refresh_id, story_id, query, status, provider, message,
			update_classification, storygraph_action, projection_action,
			source_content_id, contribution_id, decision_id, candidate_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		sanitizeStoreText(rec.Query),
		rec.Status,
		rec.Provider,
		sanitizeStoreText(rec.Message),
		rec.UpdateClassification,
		rec.StoryGraphAction,
		rec.ProjectionAction,
		rec.SourceContentID,
		rec.ContributionID,
		rec.DecisionID,
		rec.CandidateID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireSourceRefreshRun{}, fmt.Errorf("create global wire source refresh run: %w", err)
	}
	return rec, nil
}

// ListGlobalWireSourceRefreshRuns lists recent source refresh/classification
// passes, optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireSourceRefreshRuns(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireSourceRefreshRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT owner_id, refresh_id, story_id, query, status, provider, message,
	                update_classification, storygraph_action, projection_action,
	                source_content_id, contribution_id, decision_id, candidate_id,
	                created_at, updated_at
	           FROM global_wire_source_refresh_runs
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire source refresh runs: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireSourceRefreshRun
	for rows.Next() {
		rec, err := scanGlobalWireSourceRefreshRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire source refresh runs: %w", err)
	}
	return out, nil
}

// CreateGlobalWireClaimRecord stores a provisional non-oracle claim/dispute
// record tied to source refresh and reconciliation evidence.
func (s *Store) CreateGlobalWireClaimRecord(ctx context.Context, rec types.GlobalWireClaimRecord) (types.GlobalWireClaimRecord, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "research-review-required"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_claim_records (
			owner_id, claim_id, story_id, refresh_id, source_content_id,
			contribution_id, decision_id, candidate_id, claim_text, claim_kind,
			uncertainty_state, dispute_state, evidence_gap, source_standing,
			update_classification, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.RefreshID,
		rec.SourceContentID,
		rec.ContributionID,
		rec.DecisionID,
		rec.CandidateID,
		sanitizeStoreText(rec.ClaimText),
		rec.ClaimKind,
		rec.UncertaintyState,
		rec.DisputeState,
		sanitizeStoreText(rec.EvidenceGap),
		rec.SourceStanding,
		rec.UpdateClassification,
		rec.Status,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireClaimRecord{}, fmt.Errorf("create global wire claim record: %w", err)
	}
	return rec, nil
}

// ListGlobalWireClaimRecords lists provisional claim/dispute/evidence-gap
// records, optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireClaimRecords(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireClaimRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, claim_id, story_id, refresh_id, source_content_id,
	                contribution_id, decision_id, candidate_id, claim_text, claim_kind,
	                uncertainty_state, dispute_state, evidence_gap, source_standing,
	                update_classification, status, created_at, updated_at
	           FROM global_wire_claim_records
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire claim records: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireClaimRecord
	for rows.Next() {
		rec, err := scanGlobalWireClaimRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire claim records: %w", err)
	}
	return out, nil
}

// CreateGlobalWireSourceReviewSignal stores a durable non-oracle source
// normalization signal tied to refresh/claim/candidate lineage.
func (s *Store) CreateGlobalWireSourceReviewSignal(ctx context.Context, rec types.GlobalWireSourceReviewSignal) (types.GlobalWireSourceReviewSignal, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "review-signal-open"
	}
	evidenceRefsJSON, err := json.Marshal(rec.EvidenceRefs)
	if err != nil {
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("marshal global wire source review signal evidence refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_source_review_signals (
			owner_id, signal_id, story_id, refresh_id, claim_id,
			source_content_id, candidate_id, signal_kind, update_classification,
			source_standing, overlap_state, contradiction_state, related_story_id,
			projection_action, status, rationale, evidence_refs_json, created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.RefreshID,
		rec.ClaimID,
		rec.SourceContentID,
		rec.CandidateID,
		rec.SignalKind,
		rec.UpdateClassification,
		rec.SourceStanding,
		rec.OverlapState,
		rec.ContradictionState,
		rec.RelatedStoryID,
		rec.ProjectionAction,
		rec.Status,
		sanitizeStoreText(rec.Rationale),
		string(evidenceRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("create global wire source review signal: %w", err)
	}
	return rec, nil
}

// ListGlobalWireSourceReviewSignals lists source normalization review signals,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireSourceReviewSignals(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireSourceReviewSignal, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, signal_id, story_id, refresh_id, claim_id,
	                source_content_id, candidate_id, signal_kind,
	                update_classification, source_standing, overlap_state,
	                contradiction_state, related_story_id, projection_action,
	                status, rationale, evidence_refs_json, created_at, updated_at
	           FROM global_wire_source_review_signals
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire source review signals: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireSourceReviewSignal
	for rows.Next() {
		rec, err := scanGlobalWireSourceReviewSignal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire source review signals: %w", err)
	}
	return out, nil
}

// CreateGlobalWireResearchTask stores a reviewer/researcher follow-up task
// derived from source refresh classification.
func (s *Store) CreateGlobalWireResearchTask(ctx context.Context, rec types.GlobalWireResearchTask) (types.GlobalWireResearchTask, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "open"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_research_tasks (
			owner_id, task_id, story_id, claim_id, refresh_id, source_content_id,
			contribution_id, candidate_id, task_kind, prompt, status, priority,
			update_classification, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.ClaimID,
		rec.RefreshID,
		rec.SourceContentID,
		rec.ContributionID,
		rec.CandidateID,
		rec.TaskKind,
		sanitizeStoreText(rec.Prompt),
		rec.Status,
		rec.Priority,
		rec.UpdateClassification,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireResearchTask{}, fmt.Errorf("create global wire research task: %w", err)
	}
	return rec, nil
}

// ListGlobalWireResearchTasks lists open/recent research tasks, optionally
// narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireResearchTasks(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireResearchTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, task_id, story_id, claim_id, refresh_id,
	                source_content_id, contribution_id, candidate_id, task_kind,
	                prompt, status, priority, update_classification, created_at, updated_at
	           FROM global_wire_research_tasks
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire research tasks: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireResearchTask
	for rows.Next() {
		rec, err := scanGlobalWireResearchTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire research tasks: %w", err)
	}
	return out, nil
}

// CreateGlobalWireExtractionArtifact stores a provisional source/claim
// extraction overlay. It is review data, not a StoryGraph node mutation.
func (s *Store) CreateGlobalWireExtractionArtifact(ctx context.Context, rec types.GlobalWireExtractionArtifact) (types.GlobalWireExtractionArtifact, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "provisional-review"
	}
	entitiesJSON, err := json.Marshal(rec.Entities)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("marshal global wire extraction entities: %w", err)
	}
	eventsJSON, err := json.Marshal(rec.Events)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("marshal global wire extraction events: %w", err)
	}
	timelineJSON, err := json.Marshal(rec.Timeline)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("marshal global wire extraction timeline: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_extraction_artifacts (
			owner_id, extraction_id, story_id, claim_id, refresh_id,
			source_content_id, candidate_id, entities_json, events_json,
			timeline_json, uncertainty, rationale, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.ClaimID,
		rec.RefreshID,
		rec.SourceContentID,
		rec.CandidateID,
		string(entitiesJSON),
		string(eventsJSON),
		string(timelineJSON),
		sanitizeStoreText(rec.Uncertainty),
		sanitizeStoreText(rec.Rationale),
		rec.Status,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("create global wire extraction artifact: %w", err)
	}
	return rec, nil
}

// ListGlobalWireExtractionArtifacts lists provisional extraction overlays,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireExtractionArtifacts(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireExtractionArtifact, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, extraction_id, story_id, claim_id, refresh_id,
	                source_content_id, candidate_id, entities_json, events_json,
	                timeline_json, uncertainty, rationale, status, created_at,
	                updated_at
	           FROM global_wire_extraction_artifacts
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire extraction artifacts: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireExtractionArtifact
	for rows.Next() {
		rec, err := scanGlobalWireExtractionArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire extraction artifacts: %w", err)
	}
	return out, nil
}

// GetGlobalWireResearchTask returns one owner-scoped research task.
func (s *Store) GetGlobalWireResearchTask(ctx context.Context, ownerID, taskID string) (types.GlobalWireResearchTask, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, task_id, story_id, claim_id, refresh_id,
		        source_content_id, contribution_id, candidate_id, task_kind,
		        prompt, status, priority, update_classification, created_at, updated_at
		   FROM global_wire_research_tasks
		  WHERE owner_id = ? AND task_id = ?`,
		ownerID,
		taskID,
	)
	return scanGlobalWireResearchTask(row)
}

// UpdateGlobalWireResearchTaskStatus advances the owner-scoped research queue
// lifecycle without mutating StoryGraph records.
func (s *Store) UpdateGlobalWireResearchTaskStatus(ctx context.Context, ownerID, taskID, status string) (types.GlobalWireResearchTask, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_research_tasks
		    SET status = ?, updated_at = ?
		  WHERE owner_id = ? AND task_id = ?`,
		strings.TrimSpace(status),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		taskID,
	)
	if err != nil {
		return types.GlobalWireResearchTask{}, fmt.Errorf("update global wire research task status: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireResearchTask{}, ErrNotFound
	}
	return s.GetGlobalWireResearchTask(ctx, ownerID, taskID)
}

// CreateGlobalWireResearchTaskEvidence stores a reconciliation-visible packet
// for a research-task lifecycle transition.
func (s *Store) CreateGlobalWireResearchTaskEvidence(ctx context.Context, rec types.GlobalWireResearchTaskEvidence) (types.GlobalWireResearchTaskEvidence, error) {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_research_task_evidence (
			owner_id, evidence_id, task_id, story_id, claim_id, source_content_id,
			status, evidence_level, summary, reviewer_note, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.TaskID,
		rec.StoryID,
		rec.ClaimID,
		rec.SourceContentID,
		rec.Status,
		rec.EvidenceLevel,
		sanitizeStoreText(rec.Summary),
		sanitizeStoreText(rec.ReviewerNote),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireResearchTaskEvidence{}, fmt.Errorf("create global wire research task evidence: %w", err)
	}
	return rec, nil
}

// GetGlobalWireResearchTaskEvidence returns one owner-scoped research evidence
// packet.
func (s *Store) GetGlobalWireResearchTaskEvidence(ctx context.Context, ownerID, evidenceID string) (types.GlobalWireResearchTaskEvidence, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, evidence_id, task_id, story_id, claim_id,
		        source_content_id, status, evidence_level, summary,
		        reviewer_note, created_at
		   FROM global_wire_research_task_evidence
		  WHERE owner_id = ? AND evidence_id = ?`,
		ownerID,
		evidenceID,
	)
	return scanGlobalWireResearchTaskEvidence(row)
}

// ListGlobalWireResearchTaskEvidence lists recent task evidence packets,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireResearchTaskEvidence(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireResearchTaskEvidence, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, evidence_id, task_id, story_id, claim_id,
	                source_content_id, status, evidence_level, summary,
	                reviewer_note, created_at
	           FROM global_wire_research_task_evidence
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire research task evidence: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireResearchTaskEvidence
	for rows.Next() {
		rec, err := scanGlobalWireResearchTaskEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire research task evidence: %w", err)
	}
	return out, nil
}

// CreateGlobalWireResearchEvidenceDecision stores a reviewer handoff decision
// over completed research evidence.
func (s *Store) CreateGlobalWireResearchEvidenceDecision(ctx context.Context, rec types.GlobalWireResearchEvidenceDecision) (types.GlobalWireResearchEvidenceDecision, error) {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_research_evidence_decisions (
			owner_id, decision_id, evidence_id, task_id, story_id, claim_id,
			candidate_id, source_content_id, decision, note, result_state,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.EvidenceID,
		rec.TaskID,
		rec.StoryID,
		rec.ClaimID,
		rec.CandidateID,
		rec.SourceContentID,
		rec.Decision,
		sanitizeStoreText(rec.Note),
		rec.ResultState,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireResearchEvidenceDecision{}, fmt.Errorf("create global wire research evidence decision: %w", err)
	}
	return rec, nil
}

// ListGlobalWireResearchEvidenceDecisions lists recent handoff decisions,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireResearchEvidenceDecisions(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireResearchEvidenceDecision, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, decision_id, evidence_id, task_id, story_id,
	                claim_id, candidate_id, source_content_id, decision, note,
	                result_state, created_at
	           FROM global_wire_research_evidence_decisions
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire research evidence decisions: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireResearchEvidenceDecision
	for rows.Next() {
		rec, err := scanGlobalWireResearchEvidenceDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire research evidence decisions: %w", err)
	}
	return out, nil
}

// GetGlobalWireResearchEvidenceDecision returns one owner-scoped handoff
// decision.
func (s *Store) GetGlobalWireResearchEvidenceDecision(ctx context.Context, ownerID, decisionID string) (types.GlobalWireResearchEvidenceDecision, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, decision_id, evidence_id, task_id, story_id,
		        claim_id, candidate_id, source_content_id, decision, note,
		        result_state, created_at
		   FROM global_wire_research_evidence_decisions
		  WHERE owner_id = ? AND decision_id = ?`,
		ownerID,
		decisionID,
	)
	return scanGlobalWireResearchEvidenceDecision(row)
}

// UpsertGlobalWireSourceRegistryEntry stores the source/query basis for a
// StoryGraph neighborhood fetch cycle.
func (s *Store) UpsertGlobalWireSourceRegistryEntry(ctx context.Context, rec types.GlobalWireSourceRegistryEntry) (types.GlobalWireSourceRegistryEntry, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	if rec.Status == "" {
		rec.Status = "active"
	}
	if rec.CadenceSeconds < 0 {
		rec.CadenceSeconds = 0
	}
	var nextDue any
	if !rec.NextDueAt.IsZero() {
		nextDue = rec.NextDueAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_source_registry (
			owner_id, registry_id, story_id, query, source_scope, status,
			source_standing_policy, source_standing_rationale, cadence_seconds,
			next_due_at, last_cycle_id, last_scheduled_run_id, created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			query = VALUES(query),
			source_scope = VALUES(source_scope),
			status = VALUES(status),
			source_standing_policy = VALUES(source_standing_policy),
			source_standing_rationale = VALUES(source_standing_rationale),
			cadence_seconds = VALUES(cadence_seconds),
			next_due_at = VALUES(next_due_at),
			last_cycle_id = VALUES(last_cycle_id),
			last_scheduled_run_id = IF(VALUES(last_scheduled_run_id) = '', last_scheduled_run_id, VALUES(last_scheduled_run_id)),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		sanitizeStoreText(rec.Query),
		rec.SourceScope,
		rec.Status,
		sanitizeStoreText(rec.SourceStandingPolicy),
		sanitizeStoreText(rec.SourceStandingRationale),
		rec.CadenceSeconds,
		nextDue,
		rec.LastCycleID,
		rec.LastScheduledRunID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireSourceRegistryEntry{}, fmt.Errorf("upsert global wire source registry entry: %w", err)
	}
	return rec, nil
}

// ListGlobalWireSourceRegistryEntries lists source registry entries,
// optionally narrowed to one story.
func (s *Store) ListGlobalWireSourceRegistryEntries(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireSourceRegistryEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, registry_id, story_id, query, source_scope, status,
	                source_standing_policy, source_standing_rationale,
	                cadence_seconds, next_due_at, last_cycle_id,
	                last_scheduled_run_id, created_at, updated_at
	           FROM global_wire_source_registry
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire source registry entries: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireSourceRegistryEntry
	for rows.Next() {
		rec, err := scanGlobalWireSourceRegistryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire source registry entries: %w", err)
	}
	return out, nil
}

// CreateGlobalWireFetchCycleRun records a bounded source-registry cycle.
func (s *Store) CreateGlobalWireFetchCycleRun(ctx context.Context, rec types.GlobalWireFetchCycleRun) (types.GlobalWireFetchCycleRun, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	storyIDsJSON, err := json.Marshal(rec.StoryIDs)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("marshal global wire fetch cycle story ids: %w", err)
	}
	registryIDsJSON, err := json.Marshal(rec.RegistryEntryIDs)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("marshal global wire fetch cycle registry ids: %w", err)
	}
	refreshIDsJSON, err := json.Marshal(rec.RefreshRunIDs)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("marshal global wire fetch cycle refresh ids: %w", err)
	}
	sourceIDsJSON, err := json.Marshal(rec.SourceContentIDs)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("marshal global wire fetch cycle source ids: %w", err)
	}
	if rec.Status == "" {
		rec.Status = "recorded"
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_fetch_cycle_runs (
			owner_id, cycle_id, trigger_kind, status, story_ids_json,
			registry_ids_json, refresh_ids_json, source_ids_json, message,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.Trigger,
		rec.Status,
		string(storyIDsJSON),
		string(registryIDsJSON),
		string(refreshIDsJSON),
		string(sourceIDsJSON),
		sanitizeStoreText(rec.Message),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("create global wire fetch cycle run: %w", err)
	}
	return rec, nil
}

// ListGlobalWireFetchCycleRuns lists recent bounded fetch cycles.
func (s *Store) ListGlobalWireFetchCycleRuns(ctx context.Context, ownerID string, limit int) ([]types.GlobalWireFetchCycleRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT owner_id, cycle_id, trigger_kind, status, story_ids_json,
		        registry_ids_json, refresh_ids_json, source_ids_json, message,
		        created_at, updated_at
		   FROM global_wire_fetch_cycle_runs
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list global wire fetch cycle runs: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireFetchCycleRun
	for rows.Next() {
		rec, err := scanGlobalWireFetchCycleRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire fetch cycle runs: %w", err)
	}
	return out, nil
}

// CreateGlobalWireSourceSchedulerRun records the scheduler policy pass that
// selected source registry entries for a fetch cycle.
func (s *Store) CreateGlobalWireSourceSchedulerRun(ctx context.Context, rec types.GlobalWireSourceSchedulerRun) (types.GlobalWireSourceSchedulerRun, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "recorded"
	}
	storyIDsJSON, err := json.Marshal(rec.StoryIDs)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("marshal global wire scheduler story ids: %w", err)
	}
	registryIDsJSON, err := json.Marshal(rec.RegistryEntryIDs)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("marshal global wire scheduler registry ids: %w", err)
	}
	standingPoliciesJSON, err := json.Marshal(rec.StandingPolicies)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("marshal global wire scheduler standing policies: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_source_scheduler_runs (
			owner_id, scheduler_run_id, trigger_kind, status, story_ids_json,
			registry_ids_json, fetch_cycle_id, standing_policies_json, message,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.Trigger,
		rec.Status,
		string(storyIDsJSON),
		string(registryIDsJSON),
		rec.FetchCycleID,
		string(standingPoliciesJSON),
		sanitizeStoreText(rec.Message),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("create global wire source scheduler run: %w", err)
	}
	return rec, nil
}

// ListGlobalWireSourceSchedulerRuns lists recent scheduler-policy passes.
func (s *Store) ListGlobalWireSourceSchedulerRuns(ctx context.Context, ownerID string, limit int) ([]types.GlobalWireSourceSchedulerRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT owner_id, scheduler_run_id, trigger_kind, status, story_ids_json,
		        registry_ids_json, fetch_cycle_id, standing_policies_json,
		        message, created_at, updated_at
		   FROM global_wire_source_scheduler_runs
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list global wire source scheduler runs: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireSourceSchedulerRun
	for rows.Next() {
		rec, err := scanGlobalWireSourceSchedulerRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire source scheduler runs: %w", err)
	}
	return out, nil
}

// CreateGlobalWireProjectionReview records a projection obligation created by
// a StoryGraph evidence change.
func (s *Store) CreateGlobalWireProjectionReview(ctx context.Context, rec types.GlobalWireProjectionReview) (types.GlobalWireProjectionReview, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "projection-review-required"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_projection_reviews (
			owner_id, review_id, story_id, candidate_id, promotion_id,
			source_content_id, style_id, style_doc_id, style_title,
			projection_action, status, rationale, draft_story_doc_id,
			approved_story_doc_id, approved_revision_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.CandidateID,
		rec.PromotionID,
		rec.SourceContentID,
		rec.StyleID,
		rec.StyleDocID,
		sanitizeStoreText(rec.StyleTitle),
		rec.ProjectionAction,
		rec.Status,
		sanitizeStoreText(rec.Rationale),
		rec.DraftStoryDocID,
		rec.ApprovedStoryDocID,
		rec.ApprovedRevisionID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("create global wire projection review: %w", err)
	}
	return rec, nil
}

// ListGlobalWireProjectionReviews lists projection obligations, optionally
// narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireProjectionReviews(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireProjectionReview, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, review_id, story_id, candidate_id, promotion_id,
	                source_content_id, style_id, style_doc_id, style_title,
	                projection_action, status, rationale, draft_story_doc_id,
	                approved_story_doc_id, approved_revision_id, created_at, updated_at
	           FROM global_wire_projection_reviews
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire projection reviews: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireProjectionReview
	for rows.Next() {
		rec, err := scanGlobalWireProjectionReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire projection reviews: %w", err)
	}
	return out, nil
}

// GetGlobalWireProjectionReview returns one owner-scoped projection review.
func (s *Store) GetGlobalWireProjectionReview(ctx context.Context, ownerID, reviewID string) (types.GlobalWireProjectionReview, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, review_id, story_id, candidate_id, promotion_id,
		        source_content_id, style_id, style_doc_id, style_title,
		        projection_action, status, rationale, draft_story_doc_id,
		        approved_story_doc_id, approved_revision_id, created_at, updated_at
		   FROM global_wire_projection_reviews
		  WHERE owner_id = ? AND review_id = ?`,
		ownerID,
		reviewID,
	)
	return scanGlobalWireProjectionReview(row)
}

// MarkGlobalWireProjectionReviewDraftCreated links a projection review to the
// ordinary VText draft created for it.
func (s *Store) MarkGlobalWireProjectionReviewDraftCreated(ctx context.Context, ownerID, reviewID, draftDocID string) (types.GlobalWireProjectionReview, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_projection_reviews
		    SET status = 'draft-created',
		        draft_story_doc_id = ?,
		        updated_at = ?
		  WHERE owner_id = ? AND review_id = ?`,
		strings.TrimSpace(draftDocID),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		reviewID,
	)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("mark global wire projection review draft created: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireProjectionReview{}, ErrNotFound
	}
	return s.GetGlobalWireProjectionReview(ctx, ownerID, reviewID)
}

// MarkGlobalWireProjectionReviewApproved links a projection review to the
// approved ProjectionStory VText revision and records review state.
func (s *Store) MarkGlobalWireProjectionReviewApproved(ctx context.Context, ownerID, reviewID, approvedDocID, approvedRevisionID string) (types.GlobalWireProjectionReview, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_projection_reviews
		    SET status = 'approved',
		        approved_story_doc_id = ?,
		        approved_revision_id = ?,
		        updated_at = ?
		  WHERE owner_id = ? AND review_id = ?`,
		strings.TrimSpace(approvedDocID),
		strings.TrimSpace(approvedRevisionID),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		reviewID,
	)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("mark global wire projection review approved: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireProjectionReview{}, ErrNotFound
	}
	return s.GetGlobalWireProjectionReview(ctx, ownerID, reviewID)
}

// MarkGlobalWireProjectionReviewRejected records reviewer rejection without
// changing the projection relation.
func (s *Store) MarkGlobalWireProjectionReviewRejected(ctx context.Context, ownerID, reviewID string) (types.GlobalWireProjectionReview, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_projection_reviews
		    SET status = 'rejected',
		        updated_at = ?
		  WHERE owner_id = ? AND review_id = ?`,
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		reviewID,
	)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("mark global wire projection review rejected: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWireProjectionReview{}, ErrNotFound
	}
	return s.GetGlobalWireProjectionReview(ctx, ownerID, reviewID)
}

// CreateGlobalWirePublicationUpdate stores an owner-visible package for
// publication/update-feed review without publishing by itself.
func (s *Store) CreateGlobalWirePublicationUpdate(ctx context.Context, rec types.GlobalWirePublicationUpdate) (types.GlobalWirePublicationUpdate, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "packaged-for-publication-review"
	}
	extractionIDsJSON, err := json.Marshal(rec.ExtractionIDs)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("marshal global wire publication extraction ids: %w", err)
	}
	projectionReviewIDsJSON, err := json.Marshal(rec.ProjectionReviewIDs)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("marshal global wire publication projection review ids: %w", err)
	}
	projectionStatesJSON, err := json.Marshal(rec.ProjectionStates)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("marshal global wire publication projection states: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("marshal global wire publication rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_publication_updates (
			owner_id, update_id, story_id, candidate_id, research_decision_id,
			evidence_id, source_content_id, extraction_ids_json,
			projection_review_ids_json, projection_states_json, rollback_refs_json,
			status, summary, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.CandidateID,
		rec.ResearchDecisionID,
		rec.EvidenceID,
		rec.SourceContentID,
		string(extractionIDsJSON),
		string(projectionReviewIDsJSON),
		string(projectionStatesJSON),
		string(rollbackRefsJSON),
		rec.Status,
		sanitizeStoreText(rec.Summary),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("create global wire publication update: %w", err)
	}
	return rec, nil
}

// ListGlobalWirePublicationUpdates lists publication/update-feed packages,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWirePublicationUpdates(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWirePublicationUpdate, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, update_id, story_id, candidate_id,
	                research_decision_id, evidence_id, source_content_id,
	                extraction_ids_json, projection_review_ids_json,
	                projection_states_json, rollback_refs_json, status, summary,
	                created_at, updated_at
	           FROM global_wire_publication_updates
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire publication updates: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWirePublicationUpdate
	for rows.Next() {
		rec, err := scanGlobalWirePublicationUpdate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire publication updates: %w", err)
	}
	return out, nil
}

// GetGlobalWirePublicationUpdate returns one owner-scoped publication package.
func (s *Store) GetGlobalWirePublicationUpdate(ctx context.Context, ownerID, updateID string) (types.GlobalWirePublicationUpdate, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, update_id, story_id, candidate_id,
	                research_decision_id, evidence_id, source_content_id,
	                extraction_ids_json, projection_review_ids_json,
	                projection_states_json, rollback_refs_json, status, summary,
	                created_at, updated_at
		   FROM global_wire_publication_updates
		  WHERE owner_id = ? AND update_id = ?`,
		ownerID,
		updateID,
	)
	return scanGlobalWirePublicationUpdate(row)
}

// CreateGlobalWirePublicationArtifact stores a review-ready publication/feed
// artifact derived from a publication package.
func (s *Store) CreateGlobalWirePublicationArtifact(ctx context.Context, rec types.GlobalWirePublicationArtifact) (types.GlobalWirePublicationArtifact, error) {
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	}
	if rec.Status == "" {
		rec.Status = "publication-review-ready"
	}
	if rec.Channel == "" {
		rec.Channel = "newsletter"
	}
	styleDocIDsJSON, err := json.Marshal(rec.StyleDocIDs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact style doc ids: %w", err)
	}
	projectionReviewIDsJSON, err := json.Marshal(rec.ProjectionReviewIDs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact projection review ids: %w", err)
	}
	extractionIDsJSON, err := json.Marshal(rec.ExtractionIDs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact extraction ids: %w", err)
	}
	schedulerRunIDsJSON, err := json.Marshal(rec.SchedulerRunIDs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact scheduler run ids: %w", err)
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("marshal global wire publication artifact rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_publication_artifacts (
			owner_id, artifact_id, update_id, story_id, candidate_id,
			story_vtext_doc_id, source_content_id, channel, status, title, body,
			style_doc_ids_json, projection_review_ids_json, extraction_ids_json,
			scheduler_run_ids_json, citation_refs_json, rollback_refs_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.UpdateID,
		rec.StoryID,
		rec.CandidateID,
		rec.StoryVTextDocID,
		rec.SourceContentID,
		rec.Channel,
		rec.Status,
		sanitizeStoreText(rec.Title),
		sanitizeStoreText(rec.Body),
		string(styleDocIDsJSON),
		string(projectionReviewIDsJSON),
		string(extractionIDsJSON),
		string(schedulerRunIDsJSON),
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("create global wire publication artifact: %w", err)
	}
	return rec, nil
}

// ListGlobalWirePublicationArtifacts lists review-ready publication artifacts,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWirePublicationArtifacts(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWirePublicationArtifact, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, artifact_id, update_id, story_id, candidate_id,
	                story_vtext_doc_id, source_content_id, channel, status,
	                title, body, style_doc_ids_json, projection_review_ids_json,
	                extraction_ids_json, scheduler_run_ids_json,
	                citation_refs_json, rollback_refs_json, created_at,
	                updated_at
	           FROM global_wire_publication_artifacts
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire publication artifacts: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWirePublicationArtifact
	for rows.Next() {
		rec, err := scanGlobalWirePublicationArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire publication artifacts: %w", err)
	}
	return out, nil
}

// GetGlobalWirePublicationArtifact returns one owner-scoped publication
// artifact.
func (s *Store) GetGlobalWirePublicationArtifact(ctx context.Context, ownerID, artifactID string) (types.GlobalWirePublicationArtifact, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, artifact_id, update_id, story_id, candidate_id,
		        story_vtext_doc_id, source_content_id, channel, status,
		        title, body, style_doc_ids_json, projection_review_ids_json,
		        extraction_ids_json, scheduler_run_ids_json,
		        citation_refs_json, rollback_refs_json, created_at,
		        updated_at
		   FROM global_wire_publication_artifacts
		  WHERE owner_id = ? AND artifact_id = ?`,
		ownerID,
		artifactID,
	)
	return scanGlobalWirePublicationArtifact(row)
}

// UpdateGlobalWirePublicationArtifactStatus records owner review state without
// publishing publicly or mutating StoryGraph records.
func (s *Store) UpdateGlobalWirePublicationArtifactStatus(ctx context.Context, ownerID, artifactID, status string) (types.GlobalWirePublicationArtifact, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE global_wire_publication_artifacts
		    SET status = ?, updated_at = ?
		  WHERE owner_id = ? AND artifact_id = ?`,
		strings.TrimSpace(status),
		now.UTC().Format(time.RFC3339Nano),
		ownerID,
		artifactID,
	)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("update global wire publication artifact status: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return types.GlobalWirePublicationArtifact{}, ErrNotFound
	}
	return s.GetGlobalWirePublicationArtifact(ctx, ownerID, artifactID)
}

// CreateGlobalWirePublicationDelivery stores a delivery/availability record
// for an owner-approved publication artifact.
func (s *Store) CreateGlobalWirePublicationDelivery(ctx context.Context, rec types.GlobalWirePublicationDelivery) (types.GlobalWirePublicationDelivery, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "delivery-ready"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("marshal global wire publication delivery citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("marshal global wire publication delivery rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_publication_deliveries (
			owner_id, delivery_id, artifact_id, story_id, channel, status,
			delivery_ref, citation_count, rollback_count, citation_refs_json,
			rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.ArtifactID,
		rec.StoryID,
		rec.Channel,
		rec.Status,
		rec.DeliveryRef,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("create global wire publication delivery: %w", err)
	}
	return rec, nil
}

// ListGlobalWirePublicationDeliveries lists owner-scoped delivery records,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWirePublicationDeliveries(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWirePublicationDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, delivery_id, artifact_id, story_id, channel,
	                status, delivery_ref, citation_count, rollback_count,
	                citation_refs_json, rollback_refs_json, created_at,
	                updated_at
	           FROM global_wire_publication_deliveries
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire publication deliveries: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWirePublicationDelivery
	for rows.Next() {
		rec, err := scanGlobalWirePublicationDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire publication deliveries: %w", err)
	}
	return out, nil
}

// GetGlobalWirePublicationDelivery returns one owner-scoped delivery record.
func (s *Store) GetGlobalWirePublicationDelivery(ctx context.Context, ownerID, deliveryID string) (types.GlobalWirePublicationDelivery, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, delivery_id, artifact_id, story_id, channel,
		        status, delivery_ref, citation_count, rollback_count,
		        citation_refs_json, rollback_refs_json, created_at,
		        updated_at
		   FROM global_wire_publication_deliveries
		  WHERE owner_id = ? AND delivery_id = ?`,
		ownerID,
		deliveryID,
	)
	return scanGlobalWirePublicationDelivery(row)
}

// CreateGlobalWireAutoradioScript stores a durable owner-scoped script over an
// approved publication artifact.
func (s *Store) CreateGlobalWireAutoradioScript(ctx context.Context, rec types.GlobalWireAutoradioScript) (types.GlobalWireAutoradioScript, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "script-ready"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("marshal global wire autoradio script citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("marshal global wire autoradio script rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_autoradio_scripts (
			owner_id, script_id, artifact_id, story_id, source_content_id,
			status, title, script_body, voice_notes, citation_count,
			rollback_count, citation_refs_json, rollback_refs_json, created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.ArtifactID,
		rec.StoryID,
		rec.SourceContentID,
		rec.Status,
		rec.Title,
		rec.ScriptBody,
		rec.VoiceNotes,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("create global wire autoradio script: %w", err)
	}
	return rec, nil
}

// ListGlobalWireAutoradioScripts lists owner-scoped Autoradio scripts,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireAutoradioScripts(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireAutoradioScript, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, script_id, artifact_id, story_id,
	                source_content_id, status, title, script_body, voice_notes,
	                citation_count, rollback_count, citation_refs_json,
	                rollback_refs_json, created_at, updated_at
	           FROM global_wire_autoradio_scripts
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire autoradio scripts: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireAutoradioScript
	for rows.Next() {
		rec, err := scanGlobalWireAutoradioScript(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire autoradio scripts: %w", err)
	}
	return out, nil
}

// GetGlobalWireAutoradioScript returns one owner-scoped Autoradio script.
func (s *Store) GetGlobalWireAutoradioScript(ctx context.Context, ownerID, scriptID string) (types.GlobalWireAutoradioScript, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, script_id, artifact_id, story_id,
		        source_content_id, status, title, script_body, voice_notes,
		        citation_count, rollback_count, citation_refs_json,
		        rollback_refs_json, created_at, updated_at
		   FROM global_wire_autoradio_scripts
		  WHERE owner_id = ? AND script_id = ?`,
		ownerID,
		scriptID,
	)
	return scanGlobalWireAutoradioScript(row)
}

// CreateGlobalWireAutoradioEpisode stores a durable owner-scoped playback
// package over an Autoradio script.
func (s *Store) CreateGlobalWireAutoradioEpisode(ctx context.Context, rec types.GlobalWireAutoradioEpisode) (types.GlobalWireAutoradioEpisode, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "episode-ready"
	}
	if strings.TrimSpace(rec.PlaybackMode) == "" {
		rec.PlaybackMode = "browser-speech"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("marshal global wire autoradio episode citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("marshal global wire autoradio episode rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_autoradio_episodes (
			owner_id, episode_id, script_id, artifact_id, story_id,
			source_content_id, status, playback_mode, title, transcript,
			voice_notes, duration_seconds, citation_count, rollback_count,
			citation_refs_json, rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.ScriptID,
		rec.ArtifactID,
		rec.StoryID,
		rec.SourceContentID,
		rec.Status,
		rec.PlaybackMode,
		rec.Title,
		rec.Transcript,
		rec.VoiceNotes,
		rec.DurationSeconds,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("create global wire autoradio episode: %w", err)
	}
	return rec, nil
}

// ListGlobalWireAutoradioEpisodes lists owner-scoped playback packages,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireAutoradioEpisodes(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireAutoradioEpisode, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, episode_id, script_id, artifact_id, story_id,
	                source_content_id, status, playback_mode, title, transcript,
	                voice_notes, duration_seconds, citation_count, rollback_count,
	                citation_refs_json, rollback_refs_json, created_at, updated_at
	           FROM global_wire_autoradio_episodes
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire autoradio episodes: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireAutoradioEpisode
	for rows.Next() {
		rec, err := scanGlobalWireAutoradioEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire autoradio episodes: %w", err)
	}
	return out, nil
}

// GetGlobalWireAutoradioEpisode returns one owner-scoped playback package.
func (s *Store) GetGlobalWireAutoradioEpisode(ctx context.Context, ownerID, episodeID string) (types.GlobalWireAutoradioEpisode, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, episode_id, script_id, artifact_id, story_id,
		        source_content_id, status, playback_mode, title, transcript,
		        voice_notes, duration_seconds, citation_count, rollback_count,
		        citation_refs_json, rollback_refs_json, created_at, updated_at
		   FROM global_wire_autoradio_episodes
		  WHERE owner_id = ? AND episode_id = ?`,
		ownerID,
		episodeID,
	)
	return scanGlobalWireAutoradioEpisode(row)
}

// CreateGlobalWirePublicationDeliveryExport stores a portable owner-scoped
// export over a delivered publication.
func (s *Store) CreateGlobalWirePublicationDeliveryExport(ctx context.Context, rec types.GlobalWirePublicationDeliveryExport) (types.GlobalWirePublicationDeliveryExport, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "export-ready"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("marshal global wire publication delivery export citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("marshal global wire publication delivery export rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_publication_delivery_exports (
			owner_id, export_id, delivery_id, artifact_id, script_id, story_id,
			source_content_id, format, status, title, export_body,
			citation_count, rollback_count, citation_refs_json,
			rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.DeliveryID,
		rec.ArtifactID,
		rec.ScriptID,
		rec.StoryID,
		rec.SourceContentID,
		rec.Format,
		rec.Status,
		rec.Title,
		rec.ExportBody,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("create global wire publication delivery export: %w", err)
	}
	return rec, nil
}

// ListGlobalWirePublicationDeliveryExports lists owner-scoped delivery exports,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWirePublicationDeliveryExports(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWirePublicationDeliveryExport, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, export_id, delivery_id, artifact_id, script_id,
	                story_id, source_content_id, format, status, title,
	                export_body, citation_count, rollback_count,
	                citation_refs_json, rollback_refs_json, created_at,
	                updated_at
	           FROM global_wire_publication_delivery_exports
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire publication delivery exports: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWirePublicationDeliveryExport
	for rows.Next() {
		rec, err := scanGlobalWirePublicationDeliveryExport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire publication delivery exports: %w", err)
	}
	return out, nil
}

// GetGlobalWirePublicationDeliveryExport returns one owner-scoped delivery export.
func (s *Store) GetGlobalWirePublicationDeliveryExport(ctx context.Context, ownerID, exportID string) (types.GlobalWirePublicationDeliveryExport, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, export_id, delivery_id, artifact_id, script_id,
		        story_id, source_content_id, format, status, title,
		        export_body, citation_count, rollback_count, citation_refs_json,
		        rollback_refs_json, created_at, updated_at
		   FROM global_wire_publication_delivery_exports
		  WHERE owner_id = ? AND export_id = ?`,
		ownerID,
		exportID,
	)
	return scanGlobalWirePublicationDeliveryExport(row)
}

// CreateGlobalWirePublicationPublicLink stores an owner-created unlisted public
// link for a single delivery export.
func (s *Store) CreateGlobalWirePublicationPublicLink(ctx context.Context, rec types.GlobalWirePublicationPublicLink) (types.GlobalWirePublicationPublicLink, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "public-unlisted"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("marshal global wire publication public link citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("marshal global wire publication public link rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_publication_public_links (
			owner_id, link_id, token, export_id, delivery_id, artifact_id,
			story_id, status, route_path, title, export_body, citation_count,
			rollback_count, citation_refs_json, rollback_refs_json, created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.Token,
		rec.ExportID,
		rec.DeliveryID,
		rec.ArtifactID,
		rec.StoryID,
		rec.Status,
		rec.RoutePath,
		rec.Title,
		rec.ExportBody,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("create global wire publication public link: %w", err)
	}
	return rec, nil
}

// ListGlobalWirePublicationPublicLinks lists owner-created public links.
func (s *Store) ListGlobalWirePublicationPublicLinks(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWirePublicationPublicLink, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, link_id, token, export_id, delivery_id,
	                artifact_id, story_id, status, route_path, title,
	                export_body, citation_count, rollback_count,
	                citation_refs_json, rollback_refs_json, created_at,
	                updated_at
	           FROM global_wire_publication_public_links
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire publication public links: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWirePublicationPublicLink
	for rows.Next() {
		rec, err := scanGlobalWirePublicationPublicLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire publication public links: %w", err)
	}
	return out, nil
}

// GetGlobalWirePublicationPublicLinkByToken returns one unlisted public link.
func (s *Store) GetGlobalWirePublicationPublicLinkByToken(ctx context.Context, token string) (types.GlobalWirePublicationPublicLink, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, link_id, token, export_id, delivery_id,
		        artifact_id, story_id, status, route_path, title,
		        export_body, citation_count, rollback_count, citation_refs_json,
		        rollback_refs_json, created_at, updated_at
		   FROM global_wire_publication_public_links
		  WHERE token = ?`,
		token,
	)
	return scanGlobalWirePublicationPublicLink(row)
}

// CreateGlobalWireNewsletterSubscriber stores an owner-scoped newsletter
// destination for delivery bookkeeping.
func (s *Store) CreateGlobalWireNewsletterSubscriber(ctx context.Context, rec types.GlobalWireNewsletterSubscriber) (types.GlobalWireNewsletterSubscriber, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "active"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_newsletter_subscribers (
			owner_id, subscriber_id, email, label, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			label = VALUES(label),
			status = VALUES(status),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ID,
		strings.ToLower(strings.TrimSpace(rec.Email)),
		sanitizeStoreText(rec.Label),
		rec.Status,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireNewsletterSubscriber{}, fmt.Errorf("create global wire newsletter subscriber: %w", err)
	}
	return rec, nil
}

// ListGlobalWireNewsletterSubscribers lists owner-scoped newsletter
// destinations.
func (s *Store) ListGlobalWireNewsletterSubscribers(ctx context.Context, ownerID string, limit int) ([]types.GlobalWireNewsletterSubscriber, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT owner_id, subscriber_id, email, label, status, created_at, updated_at
		   FROM global_wire_newsletter_subscribers
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list global wire newsletter subscribers: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireNewsletterSubscriber
	for rows.Next() {
		rec, err := scanGlobalWireNewsletterSubscriber(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire newsletter subscribers: %w", err)
	}
	return out, nil
}

// CreateGlobalWireNewsletterIssue stores a durable owner-composed issue over
// public links.
func (s *Store) CreateGlobalWireNewsletterIssue(ctx context.Context, rec types.GlobalWireNewsletterIssue) (types.GlobalWireNewsletterIssue, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "issue-ready"
	}
	publicLinkIDsJSON, err := json.Marshal(rec.PublicLinkIDs)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("marshal global wire newsletter issue public links: %w", err)
	}
	deliveryIDsJSON, err := json.Marshal(rec.DeliveryIDs)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("marshal global wire newsletter issue deliveries: %w", err)
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("marshal global wire newsletter issue citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("marshal global wire newsletter issue rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_newsletter_issues (
			owner_id, issue_id, story_id, status, subject, issue_body,
			public_link_ids_json, delivery_ids_json, subscriber_count,
			citation_count, rollback_count, citation_refs_json,
			rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.Status,
		sanitizeStoreText(rec.Subject),
		sanitizeStoreText(rec.IssueBody),
		string(publicLinkIDsJSON),
		string(deliveryIDsJSON),
		rec.SubscriberCount,
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("create global wire newsletter issue: %w", err)
	}
	return rec, nil
}

// ListGlobalWireNewsletterIssues lists owner-scoped newsletter issues,
// optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireNewsletterIssues(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireNewsletterIssue, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, issue_id, story_id, status, subject, issue_body,
	                 public_link_ids_json, delivery_ids_json, subscriber_count,
	                 citation_count, rollback_count, citation_refs_json,
	                 rollback_refs_json, created_at, updated_at
	            FROM global_wire_newsletter_issues
	           WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire newsletter issues: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireNewsletterIssue
	for rows.Next() {
		rec, err := scanGlobalWireNewsletterIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire newsletter issues: %w", err)
	}
	return out, nil
}

// CreateGlobalWireNewsletterDelivery stores one issue delivery ledger row.
func (s *Store) CreateGlobalWireNewsletterDelivery(ctx context.Context, rec types.GlobalWireNewsletterDelivery) (types.GlobalWireNewsletterDelivery, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "delivery-ready"
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("marshal global wire newsletter delivery citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("marshal global wire newsletter delivery rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_newsletter_deliveries (
			owner_id, delivery_id, issue_id, subscriber_id, story_id, status,
			delivery_ref, citation_count, rollback_count, citation_refs_json,
			rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.IssueID,
		rec.SubscriberID,
		rec.StoryID,
		rec.Status,
		sanitizeStoreText(rec.DeliveryRef),
		rec.CitationCount,
		rec.RollbackCount,
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("create global wire newsletter delivery: %w", err)
	}
	return rec, nil
}

// ListGlobalWireNewsletterDeliveries lists issue delivery ledger rows.
func (s *Store) ListGlobalWireNewsletterDeliveries(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireNewsletterDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, delivery_id, issue_id, subscriber_id, story_id,
	                 status, delivery_ref, citation_count, rollback_count,
	                 citation_refs_json, rollback_refs_json, created_at, updated_at
	            FROM global_wire_newsletter_deliveries
	           WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire newsletter deliveries: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireNewsletterDelivery
	for rows.Next() {
		rec, err := scanGlobalWireNewsletterDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire newsletter deliveries: %w", err)
	}
	return out, nil
}

// CreateGlobalWireNewsletterProviderReceipt stores one provider-facing send
// attempt or dry-run receipt over a newsletter delivery.
func (s *Store) CreateGlobalWireNewsletterProviderReceipt(ctx context.Context, rec types.GlobalWireNewsletterProviderReceipt) (types.GlobalWireNewsletterProviderReceipt, error) {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	rec.CreatedAt = now
	if strings.TrimSpace(rec.Status) == "" {
		rec.Status = "provider-dry-run-recorded"
	}
	eventRefsJSON, err := json.Marshal(rec.EventRefs)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("marshal global wire newsletter provider receipt event refs: %w", err)
	}
	citationRefsJSON, err := json.Marshal(rec.CitationRefs)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("marshal global wire newsletter provider receipt citation refs: %w", err)
	}
	rollbackRefsJSON, err := json.Marshal(rec.RollbackRefs)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("marshal global wire newsletter provider receipt rollback refs: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO global_wire_newsletter_provider_receipts (
			owner_id, receipt_id, issue_id, delivery_id, subscriber_id, story_id,
			provider, provider_mode, status, message_id, recipient, delivery_ref,
			attempt_summary, event_refs_json, citation_refs_json,
			rollback_refs_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.IssueID,
		rec.DeliveryID,
		rec.SubscriberID,
		rec.StoryID,
		rec.Provider,
		rec.ProviderMode,
		rec.Status,
		sanitizeStoreText(rec.MessageID),
		strings.ToLower(strings.TrimSpace(rec.Recipient)),
		sanitizeStoreText(rec.DeliveryRef),
		sanitizeStoreText(rec.AttemptSummary),
		string(eventRefsJSON),
		string(citationRefsJSON),
		string(rollbackRefsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("create global wire newsletter provider receipt: %w", err)
	}
	return rec, nil
}

// ListGlobalWireNewsletterProviderReceipts lists provider-facing newsletter
// receipts, optionally narrowed to one StoryGraph node.
func (s *Store) ListGlobalWireNewsletterProviderReceipts(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireNewsletterProviderReceipt, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT owner_id, receipt_id, issue_id, delivery_id, subscriber_id,
	                 story_id, provider, provider_mode, status, message_id,
	                 recipient, delivery_ref, attempt_summary, event_refs_json,
	                 citation_refs_json, rollback_refs_json, created_at, updated_at
	            FROM global_wire_newsletter_provider_receipts
	           WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire newsletter provider receipts: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireNewsletterProviderReceipt
	for rows.Next() {
		rec, err := scanGlobalWireNewsletterProviderReceipt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire newsletter provider receipts: %w", err)
	}
	return out, nil
}

// UpsertGlobalWireStoryProjection persists the durable projection relation.
func (s *Store) UpsertGlobalWireStoryProjection(ctx context.Context, rec types.GlobalWireStoryProjection) error {
	now := rec.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	contextJSON := strings.TrimSpace(rec.ContextJSON)
	if contextJSON == "" {
		contextJSON = "{}"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO global_wire_story_projections (
			owner_id, projection_id, story_id, style_id, style_doc_id,
			story_vtext_doc_id, context_json, projection_text, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			style_doc_id = VALUES(style_doc_id),
			story_vtext_doc_id = VALUES(story_vtext_doc_id),
			context_json = VALUES(context_json),
			projection_text = VALUES(projection_text),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		rec.StyleID,
		rec.StyleDocID,
		rec.StoryDocID,
		contextJSON,
		sanitizeStoreText(rec.Text),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert global wire story projection: %w", err)
	}
	return nil
}

// GetGlobalWireStoryProjection returns one owner-scoped durable projection
// relation for a StoryGraph node and Style.vtext source.
func (s *Store) GetGlobalWireStoryProjection(ctx context.Context, ownerID, storyID, styleID string) (types.GlobalWireStoryProjection, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT owner_id, projection_id, story_id, style_id, style_doc_id,
		        story_vtext_doc_id, context_json, projection_text, created_at, updated_at
		   FROM global_wire_story_projections
		  WHERE owner_id = ? AND story_id = ? AND style_id = ?
		  ORDER BY updated_at DESC
		  LIMIT 1`,
		ownerID,
		storyID,
		styleID,
	)
	return scanGlobalWireStoryProjection(row)
}

// ListGlobalWireContributions lists recent owner-owned contributions, optionally
// narrowed to one story.
func (s *Store) ListGlobalWireContributions(ctx context.Context, ownerID, storyID string, limit int) ([]types.GlobalWireContribution, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT owner_id, contribution_id, story_id, kind, headline, content,
	                source_content_id, user_vtext_doc_id, research_state, created_at, updated_at
	           FROM global_wire_contributions
	          WHERE owner_id = ?`
	args := []any{ownerID}
	if strings.TrimSpace(storyID) != "" {
		query += ` AND story_id = ?`
		args = append(args, storyID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list global wire contributions: %w", err)
	}
	defer rows.Close()
	var out []types.GlobalWireContribution
	for rows.Next() {
		rec, err := scanGlobalWireContribution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global wire contributions: %w", err)
	}
	return out, nil
}

func scanGlobalWireStory(row interface{ Scan(...any) error }) (types.GlobalWireStory, error) {
	var rec types.GlobalWireStory
	var relatedJSON, manifestJSON, claimsJSON, projectionsJSON, styleSourcesJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.Headline,
		&rec.Dek,
		&rec.Freshness,
		&rec.Prominence,
		&rec.Tension,
		&rec.ChangeState,
		&rec.NodeTone,
		&relatedJSON,
		&manifestJSON,
		&claimsJSON,
		&projectionsJSON,
		&styleSourcesJSON,
		&rec.StoryVTextDoc,
		&rec.SourceState,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireStory{}, ErrNotFound
		}
		return types.GlobalWireStory{}, fmt.Errorf("scan global wire story: %w", err)
	}
	if err := json.Unmarshal([]byte(relatedJSON), &rec.Related); err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("unmarshal global wire related stories: %w", err)
	}
	if err := json.Unmarshal([]byte(manifestJSON), &rec.Manifest); err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("unmarshal global wire manifest: %w", err)
	}
	if err := json.Unmarshal([]byte(claimsJSON), &rec.Claims); err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("unmarshal global wire claims: %w", err)
	}
	if err := json.Unmarshal([]byte(projectionsJSON), &rec.Projections); err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("unmarshal global wire projections: %w", err)
	}
	if err := json.Unmarshal([]byte(styleSourcesJSON), &rec.StyleSources); err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("unmarshal global wire styles: %w", err)
	}
	if rec.StyleSources == nil {
		rec.StyleSources = []types.GlobalWireStyleSource{}
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("parse global wire created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireStory{}, fmt.Errorf("parse global wire updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireContribution(row interface{ Scan(...any) error }) (types.GlobalWireContribution, error) {
	var rec types.GlobalWireContribution
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.Kind,
		&rec.Headline,
		&rec.Text,
		&rec.SourceContentID,
		&rec.UserVTextDocID,
		&rec.ResearchState,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireContribution{}, ErrNotFound
		}
		return types.GlobalWireContribution{}, fmt.Errorf("scan global wire contribution: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireContribution{}, fmt.Errorf("parse global wire contribution created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireContribution{}, fmt.Errorf("parse global wire contribution updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireReconciliationDecision(row interface{ Scan(...any) error }) (types.GlobalWireReconciliationDecision, error) {
	var rec types.GlobalWireReconciliationDecision
	var createdAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.ContributionID,
		&rec.StoryID,
		&rec.Decision,
		&rec.Note,
		&rec.SourceContentID,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireReconciliationDecision{}, ErrNotFound
		}
		return types.GlobalWireReconciliationDecision{}, fmt.Errorf("scan global wire reconciliation decision: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireReconciliationDecision{}, fmt.Errorf("parse global wire reconciliation created_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	return rec, nil
}

func scanGlobalWireGraphUpdateCandidate(row interface{ Scan(...any) error }) (types.GlobalWireGraphUpdateCandidate, error) {
	var rec types.GlobalWireGraphUpdateCandidate
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.ContributionID,
		&rec.DecisionID,
		&rec.SourceContentID,
		&rec.CandidateKind,
		&rec.Title,
		&rec.Summary,
		&rec.SourceTier,
		&rec.EdgeKind,
		&rec.ProjectionAction,
		&rec.Status,
		&rec.Rationale,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireGraphUpdateCandidate{}, ErrNotFound
		}
		return types.GlobalWireGraphUpdateCandidate{}, fmt.Errorf("scan global wire graph update candidate: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireGraphUpdateCandidate{}, fmt.Errorf("parse global wire graph update candidate created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireGraphUpdateCandidate{}, fmt.Errorf("parse global wire graph update candidate updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireGraphPromotionDecision(row interface{ Scan(...any) error }) (types.GlobalWireGraphPromotionDecision, error) {
	var rec types.GlobalWireGraphPromotionDecision
	var createdAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.CandidateID,
		&rec.StoryID,
		&rec.Decision,
		&rec.Note,
		&rec.AppliedChange,
		&rec.SourceContentID,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireGraphPromotionDecision{}, ErrNotFound
		}
		return types.GlobalWireGraphPromotionDecision{}, fmt.Errorf("scan global wire graph promotion decision: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireGraphPromotionDecision{}, fmt.Errorf("parse global wire graph promotion created_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	return rec, nil
}

func scanGlobalWireSourceRefreshRun(row interface{ Scan(...any) error }) (types.GlobalWireSourceRefreshRun, error) {
	var rec types.GlobalWireSourceRefreshRun
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.Query,
		&rec.Status,
		&rec.Provider,
		&rec.Message,
		&rec.UpdateClassification,
		&rec.StoryGraphAction,
		&rec.ProjectionAction,
		&rec.SourceContentID,
		&rec.ContributionID,
		&rec.DecisionID,
		&rec.CandidateID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireSourceRefreshRun{}, ErrNotFound
		}
		return types.GlobalWireSourceRefreshRun{}, fmt.Errorf("scan global wire source refresh run: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireSourceRefreshRun{}, fmt.Errorf("parse global wire source refresh created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireSourceRefreshRun{}, fmt.Errorf("parse global wire source refresh updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireClaimRecord(row interface{ Scan(...any) error }) (types.GlobalWireClaimRecord, error) {
	var rec types.GlobalWireClaimRecord
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.RefreshID,
		&rec.SourceContentID,
		&rec.ContributionID,
		&rec.DecisionID,
		&rec.CandidateID,
		&rec.ClaimText,
		&rec.ClaimKind,
		&rec.UncertaintyState,
		&rec.DisputeState,
		&rec.EvidenceGap,
		&rec.SourceStanding,
		&rec.UpdateClassification,
		&rec.Status,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireClaimRecord{}, ErrNotFound
		}
		return types.GlobalWireClaimRecord{}, fmt.Errorf("scan global wire claim record: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireClaimRecord{}, fmt.Errorf("parse global wire claim record created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireClaimRecord{}, fmt.Errorf("parse global wire claim record updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireSourceReviewSignal(row interface{ Scan(...any) error }) (types.GlobalWireSourceReviewSignal, error) {
	var rec types.GlobalWireSourceReviewSignal
	var evidenceRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.RefreshID,
		&rec.ClaimID,
		&rec.SourceContentID,
		&rec.CandidateID,
		&rec.SignalKind,
		&rec.UpdateClassification,
		&rec.SourceStanding,
		&rec.OverlapState,
		&rec.ContradictionState,
		&rec.RelatedStoryID,
		&rec.ProjectionAction,
		&rec.Status,
		&rec.Rationale,
		&evidenceRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireSourceReviewSignal{}, ErrNotFound
		}
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("scan global wire source review signal: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceRefsJSON), &rec.EvidenceRefs); err != nil {
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("unmarshal global wire source review signal evidence refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("parse global wire source review signal created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireSourceReviewSignal{}, fmt.Errorf("parse global wire source review signal updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireResearchTask(row interface{ Scan(...any) error }) (types.GlobalWireResearchTask, error) {
	var rec types.GlobalWireResearchTask
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.ClaimID,
		&rec.RefreshID,
		&rec.SourceContentID,
		&rec.ContributionID,
		&rec.CandidateID,
		&rec.TaskKind,
		&rec.Prompt,
		&rec.Status,
		&rec.Priority,
		&rec.UpdateClassification,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireResearchTask{}, ErrNotFound
		}
		return types.GlobalWireResearchTask{}, fmt.Errorf("scan global wire research task: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireResearchTask{}, fmt.Errorf("parse global wire research task created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireResearchTask{}, fmt.Errorf("parse global wire research task updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireExtractionArtifact(row interface{ Scan(...any) error }) (types.GlobalWireExtractionArtifact, error) {
	var rec types.GlobalWireExtractionArtifact
	var entitiesJSON, eventsJSON, timelineJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.ClaimID,
		&rec.RefreshID,
		&rec.SourceContentID,
		&rec.CandidateID,
		&entitiesJSON,
		&eventsJSON,
		&timelineJSON,
		&rec.Uncertainty,
		&rec.Rationale,
		&rec.Status,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireExtractionArtifact{}, ErrNotFound
		}
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("scan global wire extraction artifact: %w", err)
	}
	if err := json.Unmarshal([]byte(entitiesJSON), &rec.Entities); err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("unmarshal global wire extraction entities: %w", err)
	}
	if err := json.Unmarshal([]byte(eventsJSON), &rec.Events); err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("unmarshal global wire extraction events: %w", err)
	}
	if err := json.Unmarshal([]byte(timelineJSON), &rec.Timeline); err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("unmarshal global wire extraction timeline: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("parse global wire extraction artifact created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireExtractionArtifact{}, fmt.Errorf("parse global wire extraction artifact updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireResearchTaskEvidence(row interface{ Scan(...any) error }) (types.GlobalWireResearchTaskEvidence, error) {
	var rec types.GlobalWireResearchTaskEvidence
	var createdAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.TaskID,
		&rec.StoryID,
		&rec.ClaimID,
		&rec.SourceContentID,
		&rec.Status,
		&rec.EvidenceLevel,
		&rec.Summary,
		&rec.ReviewerNote,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireResearchTaskEvidence{}, ErrNotFound
		}
		return types.GlobalWireResearchTaskEvidence{}, fmt.Errorf("scan global wire research task evidence: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireResearchTaskEvidence{}, fmt.Errorf("parse global wire research task evidence created_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	return rec, nil
}

func scanGlobalWireResearchEvidenceDecision(row interface{ Scan(...any) error }) (types.GlobalWireResearchEvidenceDecision, error) {
	var rec types.GlobalWireResearchEvidenceDecision
	var createdAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.EvidenceID,
		&rec.TaskID,
		&rec.StoryID,
		&rec.ClaimID,
		&rec.CandidateID,
		&rec.SourceContentID,
		&rec.Decision,
		&rec.Note,
		&rec.ResultState,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireResearchEvidenceDecision{}, ErrNotFound
		}
		return types.GlobalWireResearchEvidenceDecision{}, fmt.Errorf("scan global wire research evidence decision: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireResearchEvidenceDecision{}, fmt.Errorf("parse global wire research evidence decision created_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	return rec, nil
}

func scanGlobalWireSourceRegistryEntry(row interface{ Scan(...any) error }) (types.GlobalWireSourceRegistryEntry, error) {
	var rec types.GlobalWireSourceRegistryEntry
	var nextDue sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.Query,
		&rec.SourceScope,
		&rec.Status,
		&rec.SourceStandingPolicy,
		&rec.SourceStandingRationale,
		&rec.CadenceSeconds,
		&nextDue,
		&rec.LastCycleID,
		&rec.LastScheduledRunID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireSourceRegistryEntry{}, ErrNotFound
		}
		return types.GlobalWireSourceRegistryEntry{}, fmt.Errorf("scan global wire source registry entry: %w", err)
	}
	if nextDue.Valid && strings.TrimSpace(nextDue.String) != "" {
		parsedNextDue, err := time.Parse(time.RFC3339Nano, nextDue.String)
		if err != nil {
			return types.GlobalWireSourceRegistryEntry{}, fmt.Errorf("parse global wire source registry next_due_at: %w", err)
		}
		rec.NextDueAt = parsedNextDue.UTC()
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireSourceRegistryEntry{}, fmt.Errorf("parse global wire source registry created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireSourceRegistryEntry{}, fmt.Errorf("parse global wire source registry updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireSourceSchedulerRun(row interface{ Scan(...any) error }) (types.GlobalWireSourceSchedulerRun, error) {
	var rec types.GlobalWireSourceSchedulerRun
	var storyIDsJSON, registryIDsJSON, standingPoliciesJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.Trigger,
		&rec.Status,
		&storyIDsJSON,
		&registryIDsJSON,
		&rec.FetchCycleID,
		&standingPoliciesJSON,
		&rec.Message,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireSourceSchedulerRun{}, ErrNotFound
		}
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("scan global wire source scheduler run: %w", err)
	}
	if err := json.Unmarshal([]byte(storyIDsJSON), &rec.StoryIDs); err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("unmarshal global wire scheduler story ids: %w", err)
	}
	if err := json.Unmarshal([]byte(registryIDsJSON), &rec.RegistryEntryIDs); err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("unmarshal global wire scheduler registry ids: %w", err)
	}
	if err := json.Unmarshal([]byte(standingPoliciesJSON), &rec.StandingPolicies); err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("unmarshal global wire scheduler standing policies: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("parse global wire scheduler created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireSourceSchedulerRun{}, fmt.Errorf("parse global wire scheduler updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireFetchCycleRun(row interface{ Scan(...any) error }) (types.GlobalWireFetchCycleRun, error) {
	var rec types.GlobalWireFetchCycleRun
	var storyIDsJSON, registryIDsJSON, refreshIDsJSON, sourceIDsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.Trigger,
		&rec.Status,
		&storyIDsJSON,
		&registryIDsJSON,
		&refreshIDsJSON,
		&sourceIDsJSON,
		&rec.Message,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireFetchCycleRun{}, ErrNotFound
		}
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("scan global wire fetch cycle run: %w", err)
	}
	if err := json.Unmarshal([]byte(storyIDsJSON), &rec.StoryIDs); err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("unmarshal global wire fetch cycle story ids: %w", err)
	}
	if err := json.Unmarshal([]byte(registryIDsJSON), &rec.RegistryEntryIDs); err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("unmarshal global wire fetch cycle registry ids: %w", err)
	}
	if err := json.Unmarshal([]byte(refreshIDsJSON), &rec.RefreshRunIDs); err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("unmarshal global wire fetch cycle refresh ids: %w", err)
	}
	if err := json.Unmarshal([]byte(sourceIDsJSON), &rec.SourceContentIDs); err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("unmarshal global wire fetch cycle source ids: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("parse global wire fetch cycle created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireFetchCycleRun{}, fmt.Errorf("parse global wire fetch cycle updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireProjectionReview(row interface{ Scan(...any) error }) (types.GlobalWireProjectionReview, error) {
	var rec types.GlobalWireProjectionReview
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.CandidateID,
		&rec.PromotionID,
		&rec.SourceContentID,
		&rec.StyleID,
		&rec.StyleDocID,
		&rec.StyleTitle,
		&rec.ProjectionAction,
		&rec.Status,
		&rec.Rationale,
		&rec.DraftStoryDocID,
		&rec.ApprovedStoryDocID,
		&rec.ApprovedRevisionID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireProjectionReview{}, ErrNotFound
		}
		return types.GlobalWireProjectionReview{}, fmt.Errorf("scan global wire projection review: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("parse global wire projection review created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireProjectionReview{}, fmt.Errorf("parse global wire projection review updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWirePublicationUpdate(row interface{ Scan(...any) error }) (types.GlobalWirePublicationUpdate, error) {
	var rec types.GlobalWirePublicationUpdate
	var extractionIDsJSON, projectionReviewIDsJSON, projectionStatesJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.CandidateID,
		&rec.ResearchDecisionID,
		&rec.EvidenceID,
		&rec.SourceContentID,
		&extractionIDsJSON,
		&projectionReviewIDsJSON,
		&projectionStatesJSON,
		&rollbackRefsJSON,
		&rec.Status,
		&rec.Summary,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWirePublicationUpdate{}, ErrNotFound
		}
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("scan global wire publication update: %w", err)
	}
	if err := json.Unmarshal([]byte(extractionIDsJSON), &rec.ExtractionIDs); err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("unmarshal global wire publication extraction ids: %w", err)
	}
	if err := json.Unmarshal([]byte(projectionReviewIDsJSON), &rec.ProjectionReviewIDs); err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("unmarshal global wire publication projection review ids: %w", err)
	}
	if err := json.Unmarshal([]byte(projectionStatesJSON), &rec.ProjectionStates); err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("unmarshal global wire publication projection states: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("unmarshal global wire publication rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("parse global wire publication update created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWirePublicationUpdate{}, fmt.Errorf("parse global wire publication update updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWirePublicationArtifact(row interface{ Scan(...any) error }) (types.GlobalWirePublicationArtifact, error) {
	var rec types.GlobalWirePublicationArtifact
	var styleDocIDsJSON, projectionReviewIDsJSON, extractionIDsJSON string
	var schedulerRunIDsJSON, citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.UpdateID,
		&rec.StoryID,
		&rec.CandidateID,
		&rec.StoryVTextDocID,
		&rec.SourceContentID,
		&rec.Channel,
		&rec.Status,
		&rec.Title,
		&rec.Body,
		&styleDocIDsJSON,
		&projectionReviewIDsJSON,
		&extractionIDsJSON,
		&schedulerRunIDsJSON,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWirePublicationArtifact{}, ErrNotFound
		}
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("scan global wire publication artifact: %w", err)
	}
	if err := json.Unmarshal([]byte(styleDocIDsJSON), &rec.StyleDocIDs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact style doc ids: %w", err)
	}
	if err := json.Unmarshal([]byte(projectionReviewIDsJSON), &rec.ProjectionReviewIDs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact projection review ids: %w", err)
	}
	if err := json.Unmarshal([]byte(extractionIDsJSON), &rec.ExtractionIDs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact extraction ids: %w", err)
	}
	if err := json.Unmarshal([]byte(schedulerRunIDsJSON), &rec.SchedulerRunIDs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact scheduler run ids: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("unmarshal global wire publication artifact rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("parse global wire publication artifact created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWirePublicationArtifact{}, fmt.Errorf("parse global wire publication artifact updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWirePublicationDelivery(row interface{ Scan(...any) error }) (types.GlobalWirePublicationDelivery, error) {
	var rec types.GlobalWirePublicationDelivery
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.ArtifactID,
		&rec.StoryID,
		&rec.Channel,
		&rec.Status,
		&rec.DeliveryRef,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWirePublicationDelivery{}, ErrNotFound
		}
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("scan global wire publication delivery: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("unmarshal global wire publication delivery citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("unmarshal global wire publication delivery rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("parse global wire publication delivery created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWirePublicationDelivery{}, fmt.Errorf("parse global wire publication delivery updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireAutoradioScript(row interface{ Scan(...any) error }) (types.GlobalWireAutoradioScript, error) {
	var rec types.GlobalWireAutoradioScript
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.ArtifactID,
		&rec.StoryID,
		&rec.SourceContentID,
		&rec.Status,
		&rec.Title,
		&rec.ScriptBody,
		&rec.VoiceNotes,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireAutoradioScript{}, ErrNotFound
		}
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("scan global wire autoradio script: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("unmarshal global wire autoradio script citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("unmarshal global wire autoradio script rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("parse global wire autoradio script created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireAutoradioScript{}, fmt.Errorf("parse global wire autoradio script updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireAutoradioEpisode(row interface{ Scan(...any) error }) (types.GlobalWireAutoradioEpisode, error) {
	var rec types.GlobalWireAutoradioEpisode
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.ScriptID,
		&rec.ArtifactID,
		&rec.StoryID,
		&rec.SourceContentID,
		&rec.Status,
		&rec.PlaybackMode,
		&rec.Title,
		&rec.Transcript,
		&rec.VoiceNotes,
		&rec.DurationSeconds,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireAutoradioEpisode{}, ErrNotFound
		}
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("scan global wire autoradio episode: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("unmarshal global wire autoradio episode citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("unmarshal global wire autoradio episode rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("parse global wire autoradio episode created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireAutoradioEpisode{}, fmt.Errorf("parse global wire autoradio episode updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWirePublicationDeliveryExport(row interface{ Scan(...any) error }) (types.GlobalWirePublicationDeliveryExport, error) {
	var rec types.GlobalWirePublicationDeliveryExport
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.DeliveryID,
		&rec.ArtifactID,
		&rec.ScriptID,
		&rec.StoryID,
		&rec.SourceContentID,
		&rec.Format,
		&rec.Status,
		&rec.Title,
		&rec.ExportBody,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWirePublicationDeliveryExport{}, ErrNotFound
		}
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("scan global wire publication delivery export: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("unmarshal global wire publication delivery export citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("unmarshal global wire publication delivery export rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("parse global wire publication delivery export created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWirePublicationDeliveryExport{}, fmt.Errorf("parse global wire publication delivery export updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWirePublicationPublicLink(row interface{ Scan(...any) error }) (types.GlobalWirePublicationPublicLink, error) {
	var rec types.GlobalWirePublicationPublicLink
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.Token,
		&rec.ExportID,
		&rec.DeliveryID,
		&rec.ArtifactID,
		&rec.StoryID,
		&rec.Status,
		&rec.RoutePath,
		&rec.Title,
		&rec.ExportBody,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWirePublicationPublicLink{}, ErrNotFound
		}
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("scan global wire publication public link: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("unmarshal global wire publication public link citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("unmarshal global wire publication public link rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("parse global wire publication public link created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWirePublicationPublicLink{}, fmt.Errorf("parse global wire publication public link updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireNewsletterSubscriber(row interface{ Scan(...any) error }) (types.GlobalWireNewsletterSubscriber, error) {
	var rec types.GlobalWireNewsletterSubscriber
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.Email,
		&rec.Label,
		&rec.Status,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireNewsletterSubscriber{}, ErrNotFound
		}
		return types.GlobalWireNewsletterSubscriber{}, fmt.Errorf("scan global wire newsletter subscriber: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireNewsletterSubscriber{}, fmt.Errorf("parse global wire newsletter subscriber created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireNewsletterSubscriber{}, fmt.Errorf("parse global wire newsletter subscriber updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireNewsletterIssue(row interface{ Scan(...any) error }) (types.GlobalWireNewsletterIssue, error) {
	var rec types.GlobalWireNewsletterIssue
	var publicLinkIDsJSON, deliveryIDsJSON, citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.Status,
		&rec.Subject,
		&rec.IssueBody,
		&publicLinkIDsJSON,
		&deliveryIDsJSON,
		&rec.SubscriberCount,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireNewsletterIssue{}, ErrNotFound
		}
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("scan global wire newsletter issue: %w", err)
	}
	if err := json.Unmarshal([]byte(publicLinkIDsJSON), &rec.PublicLinkIDs); err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("unmarshal global wire newsletter issue public links: %w", err)
	}
	if err := json.Unmarshal([]byte(deliveryIDsJSON), &rec.DeliveryIDs); err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("unmarshal global wire newsletter issue deliveries: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("unmarshal global wire newsletter issue citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("unmarshal global wire newsletter issue rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("parse global wire newsletter issue created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireNewsletterIssue{}, fmt.Errorf("parse global wire newsletter issue updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireNewsletterDelivery(row interface{ Scan(...any) error }) (types.GlobalWireNewsletterDelivery, error) {
	var rec types.GlobalWireNewsletterDelivery
	var citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.IssueID,
		&rec.SubscriberID,
		&rec.StoryID,
		&rec.Status,
		&rec.DeliveryRef,
		&rec.CitationCount,
		&rec.RollbackCount,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireNewsletterDelivery{}, ErrNotFound
		}
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("scan global wire newsletter delivery: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("unmarshal global wire newsletter delivery citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("unmarshal global wire newsletter delivery rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("parse global wire newsletter delivery created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireNewsletterDelivery{}, fmt.Errorf("parse global wire newsletter delivery updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireNewsletterProviderReceipt(row interface{ Scan(...any) error }) (types.GlobalWireNewsletterProviderReceipt, error) {
	var rec types.GlobalWireNewsletterProviderReceipt
	var eventRefsJSON, citationRefsJSON, rollbackRefsJSON string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.IssueID,
		&rec.DeliveryID,
		&rec.SubscriberID,
		&rec.StoryID,
		&rec.Provider,
		&rec.ProviderMode,
		&rec.Status,
		&rec.MessageID,
		&rec.Recipient,
		&rec.DeliveryRef,
		&rec.AttemptSummary,
		&eventRefsJSON,
		&citationRefsJSON,
		&rollbackRefsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireNewsletterProviderReceipt{}, ErrNotFound
		}
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("scan global wire newsletter provider receipt: %w", err)
	}
	if err := json.Unmarshal([]byte(eventRefsJSON), &rec.EventRefs); err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("unmarshal global wire newsletter provider receipt event refs: %w", err)
	}
	if err := json.Unmarshal([]byte(citationRefsJSON), &rec.CitationRefs); err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("unmarshal global wire newsletter provider receipt citation refs: %w", err)
	}
	if err := json.Unmarshal([]byte(rollbackRefsJSON), &rec.RollbackRefs); err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("unmarshal global wire newsletter provider receipt rollback refs: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("parse global wire newsletter provider receipt created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireNewsletterProviderReceipt{}, fmt.Errorf("parse global wire newsletter provider receipt updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}

func scanGlobalWireStoryProjection(row interface{ Scan(...any) error }) (types.GlobalWireStoryProjection, error) {
	var rec types.GlobalWireStoryProjection
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ID,
		&rec.StoryID,
		&rec.StyleID,
		&rec.StyleDocID,
		&rec.StoryDocID,
		&rec.ContextJSON,
		&rec.Text,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.GlobalWireStoryProjection{}, ErrNotFound
		}
		return types.GlobalWireStoryProjection{}, fmt.Errorf("scan global wire story projection: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.GlobalWireStoryProjection{}, fmt.Errorf("parse global wire story projection created_at: %w", err)
	}
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.GlobalWireStoryProjection{}, fmt.Errorf("parse global wire story projection updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreated.UTC()
	rec.UpdatedAt = parsedUpdated.UTC()
	return rec, nil
}
