package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type universalWireStoriesResponse struct {
	Stories      []types.WireStory             `json:"stories"`
	StyleSources []types.WireStyleSource       `json:"style_sources"`
	Source       string                        `json:"source"`
	Edition      *universalWireEditionResponse `json:"edition,omitempty"`
}

type universalWireEditionResponse struct {
	DocID          string   `json:"doc_id"`
	RevisionID     string   `json:"revision_id"`
	SourcePath     string   `json:"source_path"`
	Title          string   `json:"title"`
	IncludedDocIDs []string `json:"included_doc_ids"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}

const universalWireEditionSourcePath = "universal-wire/Wire.vtext"

var vtextTransclusionRefRE = regexp.MustCompile(`vtext:([A-Za-z0-9_.:-]{1,160})`)

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
	source := "universal-wire-vtext-index"
	var edition *universalWireEditionResponse
	if editionStories, editionResp, err := h.universalWireEditionVTextStories(r.Context(), styleSources, 12); err == nil {
		edition = editionResp
		if len(editionStories) > 0 {
			stories = editionStories
			source = "universal-wire-edition-vtext"
		} else if editionResp != nil {
			source = "universal-wire-edition-vtext"
		}
	} else if err != nil {
		log.Printf("universal wire: edition unavailable: %v", err)
	}
	for i := range stories {
		stories[i] = normalizeWireStoryPresentation(stories[i])
	}
	writeAPIJSON(w, http.StatusOK, universalWireStoriesResponse{
		Stories:      stories,
		StyleSources: styleSources,
		Source:       source,
		Edition:      edition,
	})
}

func (h *APIHandler) universalWireEditionVTextStories(ctx context.Context, styleSources []types.WireStyleSource, limit int) ([]types.WireStory, *universalWireEditionResponse, error) {
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
		story, ok := wireArticleVTextStoryFromCurrentRevision(ctx, doc, rev, styleSources)
		if !ok {
			continue
		}
		if h.platformdStoryVerificationEnabled() && !h.platformdHasPublishedVText(ctx, story.StoryVTextDoc, doc.CurrentRevisionID) {
			continue
		}
		story.Prominence = 100 - len(stories)
		story.SourceState = "universal-wire-edition-vtext"
		stories = append(stories, story)
	}
	return stories, edition, nil
}

func (h *APIHandler) platformdStoryVerificationEnabled() bool {
	if h == nil || h.rt == nil {
		return false
	}
	return strings.TrimSpace(platformdReadBaseURL()) != ""
}

func (h *APIHandler) platformdHasPublishedVText(ctx context.Context, docID, revisionID string) bool {
	base := strings.TrimRight(strings.TrimSpace(platformdReadBaseURL()), "/")
	if base == "" || strings.TrimSpace(docID) == "" {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{
		"/internal/platform/vtext/documents/" + url.PathEscape(strings.TrimSpace(docID)),
		"/internal/platform/vtext/revisions/" + url.PathEscape(strings.TrimSpace(revisionID)),
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

func universalWirePlatformOwnerID() string {
	ownerID := strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

// resolveUniversalWireVTextReadOwner returns the document owner to use for a
// read-only VText API request. Authenticated users may read platform-owned
// article VTexts that are transcluded in the Universal Wire edition.
func (h *APIHandler) resolveUniversalWireVTextReadOwner(ctx context.Context, requesterOwnerID, docID string) (string, error) {
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

func wireArticleVTextStoryFromCurrentRevision(ctx context.Context, doc types.Document, rev types.Revision, styleSources []types.WireStyleSource) (types.WireStory, bool) {
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
	manifest := wireArticleManifestFromRevision(ctx, meta, content, headline)
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
		ID:                  "source-network-vtext-" + doc.DocID,
		OwnerID:             doc.OwnerID,
		Headline:            headline,
		Dek:                 dek,
		Freshness:           wireArticleFreshness(doc.UpdatedAt),
		Prominence:          90,
		Tension:             "source-network article",
		ChangeState:         changeState,
		PlatformRoutePath:   platformRoute,
		NodeTone:            "live",
		Related:             []string{},
		Manifest:            manifest,
		Claims:              wireArticleArticleClaims(content, styleTitle, meta),
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

func wireArticleContentLooksLikeSeed(content string) bool {
	return strings.Contains(content, "## Source Brief") ||
		strings.Contains(content, "## Evidence Gathering") ||
		strings.Contains(content, "## Working Revision")
}

func wireArticleSelectedStyle(meta map[string]any, styles []types.WireStyleSource) (string, string) {
	title := "Style.vtext: Universal Wire"
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

func wireArticleManifestFromRevision(ctx context.Context, meta map[string]any, content, headline string) types.WireSourceManifest {
	entities := wireArticleVisibleSourceEntities(ctx, meta, content)
	if len(entities) > 0 {
		return wireArticleManifestFromSourceEntities(entities)
	}
	return wireArticleManifestFromCycleProvenance(meta, headline)
}

func wireArticleVisibleSourceEntities(ctx context.Context, meta map[string]any, content string) []vtextSourceEntity {
	entities := decodeVTextSourceEntities(meta["source_entities"])
	if len(entities) == 0 {
		return nil
	}
	refs := wireArticleInlineSourceRefs(wireArticleArticleProseForSourceRefs(content))
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

func wireArticleArticleProseForSourceRefs(content string) string {
	var b strings.Builder
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if wireArticleArticleLineStartsInventorySection(line) {
			break
		}
		b.WriteString(raw)
		b.WriteString("\n")
	}
	return b.String()
}

func wireArticleInlineSourceRefs(content string) map[string]bool {
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

func wireArticleManifestFromSourceEntities(entities []vtextSourceEntity) types.WireSourceManifest {
	manifest := types.WireSourceManifest{}
	for i, entity := range entities {
		id := wireArticleSourceEntityManifestID(entity)
		if id == "" {
			continue
		}
		item := types.WireSourceItem{
			ID:           id,
			Title:        wireArticleSourceEntityManifestTitle(entity),
			Standing:     wireArticleSourceEntityManifestStanding(entity),
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

func wireArticleSourceEntityManifestID(entity vtextSourceEntity) string {
	return firstNonEmpty(entity.Target.ItemID, entity.Target.ContentID, entity.Target.DocID, entity.EntityID)
}

func wireArticleSourceEntityManifestTitle(entity vtextSourceEntity) string {
	return firstNonEmpty(entity.Label, entity.Target.CanonicalURL, entity.Target.URL, wireArticleSourceEntityManifestID(entity))
}

func wireArticleSourceEntityManifestStanding(entity vtextSourceEntity) string {
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

func wireArticleManifestFromCycleProvenance(meta map[string]any, headline string) types.WireSourceManifest {
	manifest := types.WireSourceManifest{}
	cycleID := sourceNetworkCycleID(meta)
	sourceIDs := wireArticleMetadataStringSlice(meta["source_item_ids"])
	switch {
	case cycleID != "":
		standing := "source firehose cycle"
		if len(sourceIDs) > 0 {
			standing = fmt.Sprintf("source firehose cycle; %d source handles retained in revision provenance", len(sourceIDs))
		}
		manifest.Context = append(manifest.Context, types.WireSourceItem{
			ID:       "source-network-cycle:" + cycleID,
			Title:    "Source network cycle " + cycleID,
			Standing: standing,
			Role:     "context",
		})
	case strings.TrimSpace(headline) != "":
		manifest.Context = append(manifest.Context, types.WireSourceItem{
			ID:       "source-network-vtext:" + headline,
			Title:    "Universal Wire VText article head",
			Standing: "platform VText current revision",
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
	return "Universal Wire article"
}

func wireArticleArticleDek(content string) string {
	for _, paragraph := range wireArticleArticleParagraphs(content) {
		return truncateRunes(paragraph, 220)
	}
	return "Universal Wire VText article with source and style provenance on its current revision."
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
	return wireArticleArticleLineStartsInventorySection(trimmed)
}

func wireArticleArticleLineStartsInventorySection(line string) bool {
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

func wireArticleArticleClaims(content, _ string, meta map[string]any) []string {
	claims := []string{
		"Current head is a normal VText article revision owned by the Universal Wire platform agent.",
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

// normalizeWireArticleRevisionForRead repairs reader-facing source refs in
// platform wire article revisions without mutating stored revision content.
func normalizeWireArticleRevisionForRead(rev types.Revision) types.Revision {
	meta := decodeRevisionMetadata(rev.Metadata)
	if !wireRevisionSourceIsTextureEdit(meta) || sourceNetworkCycleID(meta) == "" {
		return rev
	}
	rec := &types.RunRecord{
		OwnerID: strings.TrimSpace(rev.OwnerID),
		Metadata: map[string]any{
			"request_intent":             "integrate_worker_findings",
			"type":                       "vtext_agent_revision",
			"ingestion_handoff_cycle_id": sourceNetworkCycleID(meta),
		},
	}
	content := rev.Content
	if normalized, count := normalizeWireArticleBareSourceRefs(content, rev.Metadata, rec); count > 0 {
		content = normalized
	}
	if normalized, count, entities := normalizeWireArticleSourceServiceProse(content, rev.Metadata, rec); count > 0 {
		content = normalized
		if len(entities) > 0 {
			meta["source_entities"] = entities
			if patched, err := json.Marshal(meta); err == nil {
				rev.Metadata = patched
			}
		}
	}
	rev.Content = content
	return rev
}

func wireRevisionSourceIsTextureEdit(meta map[string]any) bool {
	switch metadataString(meta, "source") {
	case "edit_texture":
		return true
	case "edit_vtext": // texture-cutover-allow: deletion receipt remove after legacy revision metadata migration
		return true
	default:
		return false
	}
}

func universalWireStoryFreshnessLooksAuto(freshness string) bool {
	freshness = strings.TrimSpace(strings.ToLower(freshness))
	return freshness == "" || strings.HasPrefix(freshness, "updated ")
}
