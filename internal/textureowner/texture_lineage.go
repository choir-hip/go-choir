package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func buildMarkdownLineageRevisionMetadata(sourcePath string, version textureMarkdownLineageVersion, content, contentID, contentHashValue, contentPath, contentSource string, index, count int, lineage []map[string]any, sourceEntities []textureSourceEntity, resolutions []textureCitationMarkerResolution) (json.RawMessage, error) {
	sourceMeta := map[string]any{}
	if len(bytes.TrimSpace(version.Metadata)) > 0 {
		if err := json.Unmarshal(version.Metadata, &sourceMeta); err != nil {
			return nil, fmt.Errorf("decode version metadata: %w", err)
		}
	}
	metadata := map[string]any{
		"source_path":  sourcePath,
		"created_from": "markdown_lineage_import",
		"migration_manifest": map[string]any{
			"source_path":              sourcePath,
			"source_kind":              "markdown",
			"source_media_type":        "text/markdown",
			"projection_kind":          "texture",
			"migration_adapter":        "markdown_lineage_to_texture_revisions",
			"migration_version":        1,
			"lineage_index":            index,
			"lineage_count":            count,
			"source_label":             strings.TrimSpace(version.Label),
			"source_revision_id":       strings.TrimSpace(version.SourceRevisionID),
			"source_content_item_id":   strings.TrimSpace(version.ContentItemID),
			"original_content_id":      contentID,
			"original_content_hash":    "sha256:" + contentHashValue,
			"original_content_path":    contentPath,
			"original_content_source":  contentSource,
			"version_lineage":          lineage,
			"source_gap_policy":        "repairable_gap_no_invented_citations",
			"source_gap_detector":      "markdown_lineage_numeric_citation_scan_v1",
			"citation_resolution_rule": "do_not_invent_sources",
			"citation_resolutions":     markdownLineageResolutionManifest(resolutions),
		},
	}
	if len(sourceMeta) > 0 {
		metadata["source_metadata"] = sourceMeta
	}
	raw, _ := json.Marshal(metadata)
	return raw, nil
}

var textureMarkdownLineageCitationRefRE = regexp.MustCompile(`\[(?:\d{1,3}|\^[A-Za-z0-9_-]{1,40})\]`)

const textureCitationResolutionOmitSentinel = "__texture_omit_citation__"

var markdownLineageSourceLinkOrMarkerRE = regexp.MustCompile(`\[[^\]\n]{1,160}\]\(source:[^) \t\r\n]{1,160}\)|\[(?:\d{1,3}|\^[A-Za-z0-9_-]{1,40})\]`)
var markdownLineageSourceLinkRE = regexp.MustCompile(`^\[([^\]\n]{1,160})\]\(source:([^) \t\r\n]{1,160})\)$`)
var markdownLineageHeadingRE = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
var markdownLineageBulletRE = regexp.MustCompile(`^[-*]\s+(.+)$`)
var markdownLineageOrderedRE = regexp.MustCompile(`^(\d{1,3})\.\s+(.+)$`)

func markdownLineageStructuredRevision(docID, revisionID, content string, sourceEntities []textureSourceEntity, resolutions []textureCitationMarkerResolution) (json.RawMessage, json.RawMessage, string, error) {
	structuredEntities := structuredSourceEntitiesFromRuntimeSources(sourceEntities)
	entityByID := make(map[string]texturedoc.SourceEntity, len(structuredEntities))
	for _, entity := range structuredEntities {
		entityByID[strings.TrimSpace(entity.SourceEntityID)] = entity
	}
	resolved := markdownLineageResolutionMap(resolutions)
	used := map[string]bool{}
	refSeq := 0
	blockSeq := 0
	addText := func(nodes *[]texturedoc.Node, segment string) {
		if segment == "" {
			return
		}
		parts := strings.Split(segment, "\n")
		for i, part := range parts {
			if i > 0 {
				*nodes = append(*nodes, texturedoc.Node{Type: "hard_break"})
			}
			if part != "" {
				*nodes = append(*nodes, texturedoc.Node{Type: "text", Text: part})
			}
		}
	}
	addSourceRef := func(nodes *[]texturedoc.Node, entityID, label string) error {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			return fmt.Errorf("source_ref requires source_entity_id")
		}
		if _, ok := entityByID[entityID]; !ok {
			return fmt.Errorf("source_ref references unknown source entity %s", entityID)
		}
		refSeq++
		used[entityID] = true
		attrs := map[string]any{
			"id":               "source-ref-" + revisionID + "-" + strconv.Itoa(refSeq),
			"source_entity_id": entityID,
			"display_mode":     "numbered_ref",
		}
		if label = strings.TrimSpace(label); label != "" {
			attrs["label"] = label
		}
		*nodes = append(*nodes, texturedoc.Node{Type: "source_ref", Attrs: attrs})
		return nil
	}
	parseInline := func(text string) ([]texturedoc.Node, error) {
		inlineNodes := []texturedoc.Node{}
		last := 0
		for _, match := range markdownLineageSourceLinkOrMarkerRE.FindAllStringIndex(text, -1) {
			token := text[match[0]:match[1]]
			addText(&inlineNodes, text[last:match[0]])
			if parts := markdownLineageSourceLinkRE.FindStringSubmatch(token); len(parts) == 3 {
				if err := addSourceRef(&inlineNodes, parts[2], parts[1]); err != nil {
					return nil, err
				}
			} else {
				entityID := strings.TrimSpace(resolved[token])
				if entityID == "" {
					return nil, fmt.Errorf("unresolved markdown citation marker %s requires a source_ref resolution or no_source_needed action", token)
				}
				if entityID != textureCitationResolutionOmitSentinel {
					label := strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
					if err := addSourceRef(&inlineNodes, entityID, label); err != nil {
						return nil, err
					}
				} else {
					trimTrailingInlineHorizontalSpace(&inlineNodes)
				}
			}
			last = match[1]
		}
		addText(&inlineNodes, text[last:])
		return inlineNodes, nil
	}
	nextBlockID := func(prefix string) string {
		blockSeq++
		return prefix + "-" + revisionID + "-" + strconv.Itoa(blockSeq)
	}
	paragraphNode := func(text string) (texturedoc.Node, bool, error) {
		inlineNodes, err := parseInline(strings.TrimSpace(text))
		if err != nil {
			return texturedoc.Node{}, false, err
		}
		if len(inlineNodes) == 0 {
			return texturedoc.Node{}, false, nil
		}
		return texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": nextBlockID("p")}, Content: inlineNodes}, true, nil
	}
	blocks, err := markdownLineageBodyDocBlocks(content, parseInline, nextBlockID, paragraphNode)
	if err != nil {
		return nil, nil, "", err
	}
	if len(blocks) == 0 {
		return nil, nil, "", fmt.Errorf("structured markdown lineage revision would be empty")
	}
	referencedEntities := make([]texturedoc.SourceEntity, 0, len(used))
	for _, entity := range structuredEntities {
		if used[strings.TrimSpace(entity.SourceEntityID)] {
			referencedEntities = append(referencedEntities, entity)
		}
	}
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:    "doc",
			Attrs:   map[string]any{"id": "doc-" + docID + "-" + revisionID},
			Content: blocks,
		},
	}
	projection, err := texturedoc.Project(doc, referencedEntities)
	if err != nil {
		return nil, nil, "", err
	}
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, "", err
	}
	sourceEntityJSON, err := json.Marshal(referencedEntities)
	if err != nil {
		return nil, nil, "", err
	}
	return bodyDoc, sourceEntityJSON, projection.Text, nil
}

func markdownLineageBodyDocBlocks(content string, parseInline func(string) ([]texturedoc.Node, error), nextBlockID func(string) string, paragraphNode func(string) (texturedoc.Node, bool, error)) ([]texturedoc.Node, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	blocks := []texturedoc.Node{}
	paragraph := []string{}
	bulletItems := []texturedoc.Node{}
	orderedItems := []texturedoc.Node{}
	orderedStart := 1

	flushParagraph := func() error {
		if len(paragraph) == 0 {
			return nil
		}
		block, ok, err := paragraphNode(strings.Join(paragraph, " "))
		if err != nil {
			return err
		}
		if ok {
			blocks = append(blocks, block)
		}
		paragraph = nil
		return nil
	}
	flushBulletList := func() {
		if len(bulletItems) == 0 {
			return
		}
		blocks = append(blocks, texturedoc.Node{
			Type:    "bullet_list",
			Attrs:   map[string]any{"id": nextBlockID("ul")},
			Content: bulletItems,
		})
		bulletItems = nil
	}
	flushOrderedList := func() {
		if len(orderedItems) == 0 {
			return
		}
		blocks = append(blocks, texturedoc.Node{
			Type:    "ordered_list",
			Attrs:   map[string]any{"id": nextBlockID("ol"), "start": orderedStart},
			Content: orderedItems,
		})
		orderedItems = nil
		orderedStart = 1
	}
	flushLists := func() {
		flushBulletList()
		flushOrderedList()
	}
	flushAll := func() error {
		if err := flushParagraph(); err != nil {
			return err
		}
		flushLists()
		return nil
	}
	listItem := func(text string) (texturedoc.Node, error) {
		block, ok, err := paragraphNode(text)
		if err != nil {
			return texturedoc.Node{}, err
		}
		if !ok {
			block = texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": nextBlockID("p")}}
		}
		return texturedoc.Node{
			Type:    "list_item",
			Attrs:   map[string]any{"id": nextBlockID("li")},
			Content: []texturedoc.Node{block},
		}, nil
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err := flushAll(); err != nil {
				return nil, err
			}
			continue
		}
		if heading := markdownLineageHeadingRE.FindStringSubmatch(trimmed); len(heading) == 3 {
			if err := flushAll(); err != nil {
				return nil, err
			}
			inlineNodes, err := parseInline(heading[2])
			if err != nil {
				return nil, err
			}
			if len(inlineNodes) > 0 {
				blocks = append(blocks, texturedoc.Node{
					Type:    "heading",
					Attrs:   map[string]any{"id": nextBlockID("h"), "level": len(heading[1])},
					Content: inlineNodes,
				})
			}
			continue
		}
		if bullet := markdownLineageBulletRE.FindStringSubmatch(trimmed); len(bullet) == 2 {
			if err := flushParagraph(); err != nil {
				return nil, err
			}
			flushOrderedList()
			item, err := listItem(bullet[1])
			if err != nil {
				return nil, err
			}
			bulletItems = append(bulletItems, item)
			continue
		}
		if ordered := markdownLineageOrderedRE.FindStringSubmatch(trimmed); len(ordered) == 3 {
			if err := flushParagraph(); err != nil {
				return nil, err
			}
			flushBulletList()
			if len(orderedItems) == 0 {
				if start, err := strconv.Atoi(ordered[1]); err == nil && start > 0 {
					orderedStart = start
				}
			}
			item, err := listItem(ordered[2])
			if err != nil {
				return nil, err
			}
			orderedItems = append(orderedItems, item)
			continue
		}
		flushLists()
		paragraph = append(paragraph, trimmed)
	}
	if err := flushAll(); err != nil {
		return nil, err
	}
	return blocks, nil
}

func trimTrailingInlineHorizontalSpace(nodes *[]texturedoc.Node) {
	for len(*nodes) > 0 {
		last := &(*nodes)[len(*nodes)-1]
		if last.Type != "text" {
			return
		}
		last.Text = strings.TrimRight(last.Text, " \t")
		if last.Text != "" {
			return
		}
		*nodes = (*nodes)[:len(*nodes)-1]
	}
}

func markdownLineageResolutionMap(resolutions []textureCitationMarkerResolution) map[string]string {
	out := map[string]string{}
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeTextureCitationResolutionAction(resolution.Action, entityID)
		if marker == "" {
			continue
		}
		if action == "no_source_needed" {
			out[marker] = textureCitationResolutionOmitSentinel
			continue
		}
		if entityID != "" {
			out[marker] = entityID
		}
	}
	return out
}

func markdownLineageResolutionManifest(resolutions []textureCitationMarkerResolution) []map[string]string {
	if len(resolutions) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(resolutions))
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeTextureCitationResolutionAction(resolution.Action, entityID)
		reason := strings.TrimSpace(resolution.Reason)
		if marker == "" {
			continue
		}
		item := map[string]string{
			"marker": marker,
			"action": action,
		}
		if entityID != "" {
			item["entity_id"] = entityID
		}
		if reason != "" {
			item["reason"] = reason
		}
		out = append(out, item)
	}
	return out
}

func markdownLineageSourceEntities(global, local []textureSourceEntity) []textureSourceEntity {
	entities, _ := mergeTextureSourceEntities(append([]textureSourceEntity{}, global...), local)
	return entities
}

func markdownLineageCitationResolutions(global, local []textureCitationMarkerResolution) []textureCitationMarkerResolution {
	seen := map[string]int{}
	out := make([]textureCitationMarkerResolution, 0, len(global)+len(local))
	add := func(resolution textureCitationMarkerResolution) {
		resolution.Marker = strings.TrimSpace(resolution.Marker)
		resolution.EntityID = strings.TrimSpace(resolution.EntityID)
		resolution.Action = normalizeTextureCitationResolutionAction(resolution.Action, resolution.EntityID)
		resolution.Reason = strings.TrimSpace(resolution.Reason)
		resolution.EvidenceState = normalizeTextureEvidenceState(resolution.EvidenceState)
		if resolution.Marker == "" || (resolution.EntityID == "" && resolution.Action != "no_source_needed") {
			return
		}
		if idx, ok := seen[resolution.Marker]; ok {
			out[idx] = resolution
			return
		}
		seen[resolution.Marker] = len(out)
		out = append(out, resolution)
	}
	for _, resolution := range global {
		add(resolution)
	}
	for _, resolution := range local {
		add(resolution)
	}
	return out
}

func normalizeTextureEvidenceState(value string) string {
	return sourcecontract.NormalizeEvidenceState(value)
}

func normalizeTextureCitationResolutionAction(action, entityID string) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	switch normalized {
	case "", "source", "source_entity", "link_source", "confirming_source":
		if strings.TrimSpace(entityID) == "" {
			return normalized
		}
		return "link_source"
	case "omit", "remove", "remove_marker", "no_source", "no_source_needed", "not_needed":
		return "no_source_needed"
	default:
		return normalized
	}
}

func validateMarkdownLineageCitationResolutions(entities []textureSourceEntity, resolutions []textureCitationMarkerResolution) error {
	entityIDs := map[string]bool{}
	for _, entity := range entities {
		if strings.TrimSpace(entity.EntityID) != "" {
			entityIDs[strings.TrimSpace(entity.EntityID)] = true
		}
	}
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeTextureCitationResolutionAction(resolution.Action, entityID)
		if marker == "" {
			return fmt.Errorf("citation resolutions require marker")
		}
		if !textureMarkdownLineageCitationRefRE.MatchString(marker) || textureMarkdownLineageCitationRefRE.FindString(marker) != marker {
			return fmt.Errorf("citation resolution marker %q is not a supported markdown citation marker", marker)
		}
		if action == "no_source_needed" {
			continue
		}
		if action != "link_source" {
			return fmt.Errorf("citation resolution marker %s has unsupported action %q", marker, resolution.Action)
		}
		if entityID == "" {
			return fmt.Errorf("citation resolution marker %s requires entity_id", marker)
		}
		if !entityIDs[entityID] {
			return fmt.Errorf("citation resolution marker %s references unknown source entity %s", marker, entityID)
		}
	}
	return nil
}

func buildMarkdownLineageContentItem(ownerID, sourcePath, title string, version textureMarkdownLineageVersion, content string, now time.Time) types.ContentItem {
	label := strings.TrimSpace(version.Label)
	if label == "" {
		label = strings.TrimSpace(version.SourceRevisionID)
	}
	if label == "" {
		label = "snapshot"
	}
	hash := contentowner.ContentHash(content)
	meta, _ := json.Marshal(map[string]any{
		"source_path":        sourcePath,
		"source_label":       label,
		"source_revision_id": strings.TrimSpace(version.SourceRevisionID),
		"snapshot_hash":      "sha256:" + hash,
	})
	prov, _ := json.Marshal(map[string]any{
		"created_from":       "texture_markdown_lineage_import",
		"original_preserved": true,
		"source_path":        sourcePath,
		"source_label":       label,
		"source_revision_id": strings.TrimSpace(version.SourceRevisionID),
	})
	return types.ContentItem{
		ContentID:   uuid.New().String(),
		OwnerID:     ownerID,
		SourceType:  "file_version",
		MediaType:   "text/markdown",
		AppHint:     agentprofile.Texture,
		Title:       fmt.Sprintf("%s %s", title, label),
		FilePath:    fmt.Sprintf("%s#%s", sourcePath, label),
		TextContent: content,
		ContentHash: hash,
		Metadata:    meta,
		Provenance:  prov,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func buildMarkdownLineageSummary(versions []resolvedMarkdownLineageVersion) []map[string]any {
	lineage := make([]map[string]any, 0, len(versions))
	for i, resolved := range versions {
		version := resolved.Version
		lineage = append(lineage, map[string]any{
			"index":                   i,
			"label":                   strings.TrimSpace(version.Label),
			"source_revision_id":      strings.TrimSpace(version.SourceRevisionID),
			"source_content_item_id":  strings.TrimSpace(version.ContentItemID),
			"content_hash":            "sha256:" + resolved.ContentHash,
			"original_content_id":     resolved.ContentID,
			"original_content_path":   resolved.ContentPath,
			"original_content_source": resolved.ContentSource,
		})
	}
	return lineage
}

func (h *Handler) resolveMarkdownLineageVersion(ctx context.Context, ownerID string, version textureMarkdownLineageVersion) (resolvedMarkdownLineageVersion, error) {
	resolved := resolvedMarkdownLineageVersion{
		Version:       version,
		Content:       version.Content,
		ContentHash:   contentowner.ContentHash(version.Content),
		ContentSource: "request_content",
	}
	contentItemID := strings.TrimSpace(version.ContentItemID)
	if contentItemID == "" {
		return resolved, nil
	}
	item, err := h.Store.GetContentItem(ctx, ownerID, contentItemID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return resolvedMarkdownLineageVersion{}, fmt.Errorf("content_item_id %s not found", contentItemID)
		}
		return resolvedMarkdownLineageVersion{}, fmt.Errorf("load content_item_id %s: %w", contentItemID, err)
	}
	content := strings.TrimSpace(item.TextContent)
	if content == "" {
		return resolvedMarkdownLineageVersion{}, fmt.Errorf("content_item_id %s has no text_content", contentItemID)
	}
	hash := strings.TrimSpace(item.ContentHash)
	if hash == "" {
		hash = contentowner.ContentHash(content)
	}
	resolved.Content = item.TextContent
	resolved.ContentItem = &item
	resolved.ContentID = item.ContentID
	resolved.ContentHash = hash
	resolved.ContentPath = firstNonEmpty(item.FilePath, item.SourceURL, item.CanonicalURL)
	resolved.ContentSource = "content_item"
	return resolved, nil
}
