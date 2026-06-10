package runtime

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

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

const communityWireEditionSourcePath = "global-wire/Wire.vtext"

var vtextTransclusionRefRE = regexp.MustCompile(`vtext:([A-Za-z0-9_.:-]{1,160})`)

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
	platformOwner := communityWirePlatformOwnerID()
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

func communityWirePlatformOwnerID() string {
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
		story.Freshness = sourceMaxxFreshness(story.UpdatedAt)
	}
	return story
}

func globalWireStoryFreshnessLooksAuto(freshness string) bool {
	freshness = strings.TrimSpace(strings.ToLower(freshness))
	return freshness == "" || strings.HasPrefix(freshness, "updated ")
}
