package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
		Freshness:   "updated 18 min ago",
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
		Freshness:   "updated 41 min ago",
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
		Freshness:   "updated 1 hr ago",
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
			`SELECT style_id, story_vtext_doc_id
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
			var styleID, docID string
			if err := rows.Scan(&styleID, &docID); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan global wire projection ref: %w", err)
			}
			refs[styleID] = docID
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
		return nil
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
		docID, err := s.createGlobalWireSeedVText(ctx, ownerID, sourceBackedStory.Headline, globalWireStoryVTextContent(sourceBackedStory), append([]types.Citation{
			{ID: "style-source", Type: "vtext", Value: styleSources[0].SourcePath, Label: styleSources[0].Title},
			{ID: "storygraph-node", Type: "storygraph", Value: sourceBackedStory.ID, Label: sourceBackedStory.Headline},
		}, globalWireSourceCitations(sourceBackedStory)...), map[string]any{
			"created_from":     "global_wire_storygraph_seed",
			"storygraph_id":    sourceBackedStory.ID,
			"source_state":     globalWireSeedState,
			"immutable_notice": "User edits must fork and never mutate this platform story.",
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
	return nil
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
			docID, err := s.createGlobalWireSeedVText(ctx, ownerID, story.Headline+" - "+style.Label+" projection", content, append([]types.Citation{
				{ID: "style-source", Type: "vtext", Value: style.SourcePath, Label: style.Title},
				{ID: "storygraph-node", Type: "storygraph", Value: story.ID, Label: story.Headline},
			}, globalWireSourceCitations(story)...), map[string]any{
				"created_from":  "global_wire_style_projection_seed",
				"storygraph_id": story.ID,
				"style_id":      style.ID,
				"style_doc_id":  style.DocID,
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
			"StoryGraph id: " + story.ID,
			"Source id: " + item.ID,
			"Evidence tier: " + tier,
			"Standing: " + item.Standing,
			"",
			"This normalized SourceItem backs a Global Wire source-neighborhood manifest entry. It is seed evidence until replaced by live Source Service ingestion.",
		}, "\n")
		metadata, err := json.Marshal(map[string]any{
			"schema":        "choir.global_wire_source_item.v1",
			"story_id":      story.ID,
			"source_id":     item.ID,
			"tier":          tier,
			"standing":      item.Standing,
			"source_class":  item.Role,
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

func globalWireStoryVTextContent(story types.GlobalWireStory) string {
	styleTitle := "Style.vtext: Global Wire"
	if len(story.StyleSources) > 0 {
		styleTitle = story.StyleSources[0].Title
	}
	sourceLines := func(label string, items []types.GlobalWireSourceItem) []string {
		lines := make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- %s: %s (%s; %s)", label, item.Title, item.Standing, item.ID))
		}
		return lines
	}
	lines := []string{
		"# " + story.Headline,
		"",
		story.Dek,
		"",
		"Style source: " + styleTitle,
		"StoryGraph id: " + story.ID,
		"State: " + story.ChangeState + "; " + story.Tension,
		"",
		"## Projection",
		"",
		story.Projections["wire-style"],
		"",
		"## Claims",
		"",
	}
	for _, claim := range story.Claims {
		lines = append(lines, "- "+claim)
	}
	lines = append(lines, "", "## Source Manifest", "")
	lines = append(lines, sourceLines("lead", story.Manifest.Lead)...)
	lines = append(lines, sourceLines("supporting", story.Manifest.Supporting)...)
	lines = append(lines, sourceLines("contrary or qualifying", story.Manifest.Contrary)...)
	lines = append(lines, sourceLines("ambient context", story.Manifest.Context)...)
	lines = append(lines, "", "## Related Story VTexts", "")
	for _, related := range story.Related {
		lines = append(lines, "- "+related)
	}
	lines = append(lines,
		"",
		"## Non-oracle note",
		"",
		"This story is a source-grounded VText projection. User edits create user-owned versions and do not mutate the platform story.",
	)
	return strings.Join(lines, "\n")
}

func globalWireStyleVTextContent(style types.GlobalWireStyleSource) string {
	return strings.Join([]string{
		"# " + style.Title,
		"",
		style.Summary,
		"",
		"## Applies To",
		"",
		"- StoryGraph projections",
		"- Story VText revision prompts",
		"- News reader and Autoradio traversal",
		"",
		"## Guardrails",
		"",
		"- Preserve lead, supporting, contrary, and ambient source tiers.",
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
		"Style source: " + style.Title,
		"StoryGraph id: " + story.ID,
		"Projection relation: StoryGraph + Style.vtext + audience/task context -> Story VText",
		"",
		"## Projection",
		"",
		projection,
		"",
		"## Claims Preserved",
		"",
	}
	for _, claim := range story.Claims {
		lines = append(lines, "- "+claim)
	}
	lines = append(lines,
		"",
		"## Evidence Invariant",
		"",
		"This projection may change framing and salience, but it must not invent evidence, hide contrary evidence, or mutate the platform StoryGraph.",
	)
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
			source_content_id, contribution_id, decision_id, candidate_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.OwnerID,
		rec.ID,
		rec.StoryID,
		sanitizeStoreText(rec.Query),
		rec.Status,
		rec.Provider,
		sanitizeStoreText(rec.Message),
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
