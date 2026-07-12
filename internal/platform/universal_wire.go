package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const universalWireStoryLimit = 100

// UniversalWireStoriesResponse is the product response consumed by the
// Universal Wire app. corpusd derives it exclusively from active, public
// publication objects in the shared world-wire object graph.
type UniversalWireStoriesResponse struct {
	Stories      []types.WireStory             `json:"stories"`
	StyleSources []types.WireStyleSource       `json:"style_sources"`
	Source       string                        `json:"source"`
	Edition      *UniversalWireEditionResponse `json:"edition,omitempty"`
	Diagnostics  *UniversalWireFeedDiagnostics `json:"diagnostics,omitempty"`
}

type UniversalWireEditionResponse struct {
	DocID          string   `json:"doc_id"`
	RevisionID     string   `json:"revision_id"`
	SourcePath     string   `json:"source_path"`
	Title          string   `json:"title"`
	IncludedDocIDs []string `json:"included_doc_ids"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

type UniversalWireFeedDiagnostics struct {
	Status     string                                 `json:"status"`
	Summary    string                                 `json:"summary"`
	Substrates []UniversalWireFeedSubstrateDiagnostic `json:"substrates"`
}

type UniversalWireFeedSubstrateDiagnostic struct {
	Substrate      string `json:"substrate"`
	State          string `json:"state"`
	CandidateCount int    `json:"candidate_count"`
	StoryCount     int    `json:"story_count"`
	FilteredCount  int    `json:"filtered_count,omitempty"`
	Reason         string `json:"reason"`
}

type universalWirePublication struct {
	route       objectgraph.Object
	routePath   string
	sourceDocID string
	bundle      *PublicationBundle
}

// ListUniversalWireStories lists canonical publications newest-first. It does
// not consult platform_texture_documents or any VM-local Texture state.
func (s *Service) ListUniversalWireStories(ctx context.Context) (*UniversalWireStoriesResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	tombstone := false
	routes, err := og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:      "choir.public_route",
		Limit:     universalWireStoryLimit,
		Tombstone: &tombstone,
	})
	if err != nil {
		return nil, fmt.Errorf("list universal wire routes: %w", err)
	}

	publications := make([]universalWirePublication, 0, len(routes))
	for _, route := range routes {
		var routeMeta struct {
			RoutePath       string `json:"route_path"`
			TargetVersionID string `json:"target_version_id"`
			State           string `json:"state"`
		}
		if err := json.Unmarshal(route.Metadata, &routeMeta); err != nil {
			return nil, fmt.Errorf("decode universal wire route %s: %w", route.CanonicalID, err)
		}
		if routeMeta.State != "active" || strings.TrimSpace(routeMeta.RoutePath) == "" {
			continue
		}
		bundle, err := s.GetPublicationBundleByRoute(ctx, routeMeta.RoutePath)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, fmt.Errorf("load universal wire publication %s: %w", routeMeta.RoutePath, err)
		}
		versions, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", routeMeta.TargetVersionID, 1)
		if err != nil {
			return nil, fmt.Errorf("resolve universal wire publication version %s: %w", routeMeta.TargetVersionID, err)
		}
		if len(versions) != 1 {
			return nil, fmt.Errorf("universal wire publication version %s is missing", routeMeta.TargetVersionID)
		}
		var versionMeta struct {
			SourceDocID string `json:"source_doc_id"`
		}
		if err := json.Unmarshal(versions[0].Metadata, &versionMeta); err != nil {
			return nil, fmt.Errorf("decode universal wire publication version %s: %w", routeMeta.TargetVersionID, err)
		}
		if strings.TrimSpace(versionMeta.SourceDocID) == "" {
			return nil, fmt.Errorf("universal wire publication version %s has no source document", routeMeta.TargetVersionID)
		}
		publications = append(publications, universalWirePublication{
			route:       route,
			routePath:   routeMeta.RoutePath,
			sourceDocID: versionMeta.SourceDocID,
			bundle:      bundle,
		})
	}

	sort.Slice(publications, func(i, j int) bool {
		if !publications[i].bundle.Version.PublishedAt.Equal(publications[j].bundle.Version.PublishedAt) {
			return publications[i].bundle.Version.PublishedAt.After(publications[j].bundle.Version.PublishedAt)
		}
		return publications[i].route.CanonicalID < publications[j].route.CanonicalID
	})

	stories := make([]types.WireStory, 0, len(publications))
	for i, publication := range publications {
		stories = append(stories, universalWireStoryFromPublication(publication, 100-i))
	}
	return &UniversalWireStoriesResponse{
		Stories:      stories,
		StyleSources: []types.WireStyleSource{},
		Source:       "corpusd-publications",
	}, nil
}

func universalWireStoryFromPublication(publication universalWirePublication, prominence int) types.WireStory {
	bundle := publication.bundle
	content := strings.TrimSpace(bundle.Artifact.Content)
	updatedAt := bundle.Version.PublishedAt
	if updatedAt.IsZero() {
		updatedAt = publication.route.UpdatedAt
	}
	createdAt := publication.route.CreatedAt
	if createdAt.IsZero() {
		createdAt = updatedAt
	}
	projection := truncateUniversalWireText(strings.Join(universalWireParagraphs(content), "\n\n"), 900)
	if projection == "" {
		projection = truncateUniversalWireText(content, 900)
	}
	manifest := universalWireManifest(bundle.SourceEntities)
	return types.WireStory{
		ID:                    "source-network-texture-" + publication.sourceDocID,
		OwnerID:               publication.route.OwnerID,
		Headline:              universalWireHeadline(bundle.Publication.Title, content),
		Dek:                   universalWireDek(content),
		Freshness:             universalWireFreshness(updatedAt),
		Prominence:            prominence,
		Tension:               "source-network article",
		ChangeState:           "platform published",
		NodeTone:              "live",
		Related:               []string{},
		Manifest:              manifest,
		Claims:                []string{"This Texture article is published in the canonical corpusd world-wire store.", "Source provenance is carried by the canonical publication object graph."},
		Projections:           map[string]string{"wire-style": projection},
		ProjectionTextureDocs: map[string]string{"wire-style": publication.sourceDocID},
		StyleSources:          []types.WireStyleSource{},
		StoryTextureDoc:       publication.sourceDocID,
		TextureContent:        content,
		PlatformRoutePath:     publication.routePath,
		SourceState:           "corpusd-publication-index",
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}
}

func universalWireManifest(entities []PublicationSourceEntity) types.WireSourceManifest {
	manifest := types.WireSourceManifest{
		Lead:       []types.WireSourceItem{},
		Supporting: []types.WireSourceItem{},
		Contrary:   []types.WireSourceItem{},
		Context:    []types.WireSourceItem{},
	}
	for _, entity := range entities {
		var payload struct {
			Label string `json:"label"`
			Title string `json:"title"`
			URL   string `json:"url"`
			URI   string `json:"uri"`
		}
		_ = json.Unmarshal(entity.Entity, &payload)
		manifest.Context = append(manifest.Context, types.WireSourceItem{
			ID:           firstNonEmpty(entity.SourceEntityID, entity.ID),
			Title:        firstNonEmpty(payload.Label, payload.Title, entity.TargetID, entity.SourceEntityID),
			Standing:     firstNonEmpty(entity.Kind, "published source"),
			Role:         "context",
			CanonicalURL: firstNonEmpty(payload.URL, payload.URI),
			SourceKind:   entity.Kind,
			TargetKind:   entity.TargetKind,
			CanonicalID:  entity.TargetID,
			OpenSurface:  entity.OpenSurface,
		})
	}
	return manifest
}

func universalWireHeadline(title, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	if title = strings.TrimSpace(strings.TrimSuffix(title, ".texture")); title != "" {
		return title
	}
	return "Universal Wire article"
}

func universalWireDek(content string) string {
	if paragraphs := universalWireParagraphs(content); len(paragraphs) > 0 {
		return truncateUniversalWireText(paragraphs[0], 220)
	}
	return "Universal Wire Texture article with canonical publication provenance."
}

func universalWireParagraphs(content string) []string {
	var out []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		out = append(out, strings.TrimSpace(strings.Join(current, " ")))
		current = nil
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, ">") {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return out
}

func truncateUniversalWireText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func universalWireFreshness(updatedAt time.Time) string {
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
