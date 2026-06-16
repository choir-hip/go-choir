package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func buildMarkdownLineageRevisionMetadata(sourcePath string, version vtextMarkdownLineageVersion, content, contentID, contentHashValue, contentPath, contentSource string, index, count int, lineage []map[string]any, sourceEntities []vtextSourceEntity, resolutions []vtextCitationMarkerResolution) (json.RawMessage, error) {
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
			"projection_kind":          "vtext",
			"migration_adapter":        "markdown_lineage_to_vtext_revisions",
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
	if len(sourceEntities) > 0 {
		metadata["source_entities"] = sourceEntities
	}
	if gaps := detectMarkdownLineageSourceGaps(content, resolutions); len(gaps) > 0 {
		metadata["source_gaps"] = gaps
	}
	raw, _ := json.Marshal(metadata)
	return raw, nil
}

var vtextMarkdownLineageCitationRefRE = regexp.MustCompile(`\[(?:\d{1,3}|\^[A-Za-z0-9_-]{1,40})\]`)

const vtextCitationResolutionOmitSentinel = "__vtext_omit_citation__"

func detectMarkdownLineageSourceGaps(content string, resolutions []vtextCitationMarkerResolution) []map[string]any {
	matches := vtextMarkdownLineageCitationRefRE.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}
	resolved := markdownLineageResolutionMap(resolutions)
	gaps := make([]map[string]any, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		marker := content[match[0]:match[1]]
		if seen[marker] || resolved[marker] != "" {
			continue
		}
		seen[marker] = true
		gaps = append(gaps, map[string]any{
			"kind":           "unresolved_markdown_citation_marker",
			"marker":         marker,
			"policy":         "repairable_gap_no_invented_citations",
			"evidence_state": vtextSourceEvidenceStateRecord("candidate", "", "unresolved markdown citation marker"),
		})
	}
	return gaps
}

func markdownLineageProjectionContent(content string, resolutions []vtextCitationMarkerResolution) string {
	return applyVTextCitationResolutions(content, resolutions)
}

func applyVTextCitationResolutions(content string, resolutions []vtextCitationMarkerResolution) string {
	resolved := markdownLineageResolutionMap(resolutions)
	if len(resolved) == 0 {
		return content
	}
	matches := vtextMarkdownLineageCitationRefRE.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var b strings.Builder
	last := 0
	changed := false
	for _, match := range matches {
		marker := content[match[0]:match[1]]
		entityID := resolved[marker]
		if entityID == "" || strings.HasPrefix(content[match[1]:], "(source:") {
			continue
		}
		b.WriteString(content[last:match[0]])
		if entityID == vtextCitationResolutionOmitSentinel {
			trimTrailingHorizontalSpace(&b)
			last = match[1]
			changed = true
			continue
		}
		label := strings.TrimSuffix(strings.TrimPrefix(marker, "["), "]")
		b.WriteString(fmt.Sprintf("[%s](source:%s)", label, entityID))
		last = match[1]
		changed = true
	}
	if !changed {
		return content
	}
	b.WriteString(content[last:])
	return b.String()
}

func trimTrailingHorizontalSpace(b *strings.Builder) {
	value := b.String()
	trimmed := strings.TrimRight(value, " \t")
	if len(trimmed) == len(value) {
		return
	}
	b.Reset()
	b.WriteString(trimmed)
}

func markdownLineageResolutionMap(resolutions []vtextCitationMarkerResolution) map[string]string {
	out := map[string]string{}
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeVTextCitationResolutionAction(resolution.Action, entityID)
		if marker == "" {
			continue
		}
		if action == "no_source_needed" {
			out[marker] = vtextCitationResolutionOmitSentinel
			continue
		}
		if entityID != "" {
			out[marker] = entityID
		}
	}
	return out
}

func markdownLineageResolutionManifest(resolutions []vtextCitationMarkerResolution) []map[string]string {
	if len(resolutions) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(resolutions))
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeVTextCitationResolutionAction(resolution.Action, entityID)
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

func markdownLineageSourceRepairResolutionManifest(resolutions []vtextCitationMarkerResolution) []map[string]any {
	if len(resolutions) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(resolutions))
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeVTextCitationResolutionAction(resolution.Action, entityID)
		reason := strings.TrimSpace(resolution.Reason)
		if marker == "" {
			continue
		}
		state := normalizeVTextEvidenceState(resolution.EvidenceState)
		if state == "" {
			state = vtextEvidenceStateForCitationResolution(action, "")
		}
		item := map[string]any{
			"marker":         marker,
			"action":         action,
			"evidence_state": vtextSourceEvidenceStateRecord(state, entityID, reason),
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

func markdownLineageSourceEntities(global, local []vtextSourceEntity) []vtextSourceEntity {
	entities, _ := mergeVTextSourceEntities(append([]vtextSourceEntity{}, global...), local)
	return entities
}

func markdownLineageCitationResolutions(global, local []vtextCitationMarkerResolution) []vtextCitationMarkerResolution {
	seen := map[string]int{}
	out := make([]vtextCitationMarkerResolution, 0, len(global)+len(local))
	add := func(resolution vtextCitationMarkerResolution) {
		resolution.Marker = strings.TrimSpace(resolution.Marker)
		resolution.EntityID = strings.TrimSpace(resolution.EntityID)
		resolution.Action = normalizeVTextCitationResolutionAction(resolution.Action, resolution.EntityID)
		resolution.Reason = strings.TrimSpace(resolution.Reason)
		resolution.EvidenceState = normalizeVTextEvidenceState(resolution.EvidenceState)
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

func normalizeVTextEvidenceState(value string) string {
	return sourcecontract.NormalizeEvidenceState(value)
}

func vtextEvidenceStateForCitationResolution(action, relation string) string {
	relationState := normalizeVTextEvidenceState(relation)
	if sourcecontract.IsRelationalEvidenceState(relationState) {
		return relationState
	}
	if normalizeVTextCitationResolutionAction(action, "") == "no_source_needed" {
		return sourcecontract.EvidenceStateNoSourceNeeded
	}
	return sourcecontract.EvidenceStateConfirms
}

func vtextSourceEvidenceStateRecord(state, targetID, reason string) map[string]any {
	normalized := normalizeVTextEvidenceState(state)
	if normalized == "" {
		normalized = "candidate"
	}
	record := map[string]any{"state": normalized}
	if targetID = strings.TrimSpace(targetID); targetID != "" {
		record["target_id"] = targetID
	}
	if reason = strings.TrimSpace(reason); reason != "" {
		record["reason"] = reason
	}
	return record
}

func normalizeVTextSourceRepairEvidence(entities []vtextSourceEntity, resolutions []vtextCitationMarkerResolution) []vtextSourceEntity {
	if len(entities) == 0 {
		return nil
	}
	stateByEntityID := map[string]string{}
	for _, resolution := range resolutions {
		entityID := strings.TrimSpace(resolution.EntityID)
		if entityID == "" {
			continue
		}
		state := normalizeVTextEvidenceState(resolution.EvidenceState)
		if state == "" {
			state = vtextEvidenceStateForCitationResolution(resolution.Action, "")
		}
		stateByEntityID[entityID] = state
	}
	out := append([]vtextSourceEntity{}, entities...)
	for i := range out {
		entityID := strings.TrimSpace(out[i].EntityID)
		relation := normalizeVTextEvidenceState(out[i].Evidence.Relation)
		if !sourcecontract.IsRelationalEvidenceState(relation) {
			relation = normalizeVTextEvidenceState(out[i].Evidence.State)
		}
		if !sourcecontract.IsRelationalEvidenceState(relation) {
			relation = stateByEntityID[entityID]
		}
		if !sourcecontract.IsRelationalEvidenceState(relation) {
			relation = sourcecontract.EvidenceStateConfirms
		}
		out[i].Evidence.Relation = relation
		out[i].Evidence.State = relation
		if strings.TrimSpace(out[i].Evidence.ResearchState) == "" {
			out[i].Evidence.ResearchState = "owner_supplied"
		}
	}
	return out
}

func normalizeVTextCitationResolutionAction(action, entityID string) string {
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

func validateMarkdownLineageCitationResolutions(entities []vtextSourceEntity, resolutions []vtextCitationMarkerResolution) error {
	entityIDs := map[string]bool{}
	for _, entity := range entities {
		if strings.TrimSpace(entity.EntityID) != "" {
			entityIDs[strings.TrimSpace(entity.EntityID)] = true
		}
	}
	for _, resolution := range resolutions {
		marker := strings.TrimSpace(resolution.Marker)
		entityID := strings.TrimSpace(resolution.EntityID)
		action := normalizeVTextCitationResolutionAction(resolution.Action, entityID)
		if marker == "" {
			return fmt.Errorf("citation resolutions require marker")
		}
		if !vtextMarkdownLineageCitationRefRE.MatchString(marker) || vtextMarkdownLineageCitationRefRE.FindString(marker) != marker {
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

func filterVTextSourceGaps(value any, repaired map[string]string) []map[string]any {
	if len(repaired) == 0 || value == nil {
		return decodeVTextSourceGaps(value)
	}
	gaps := decodeVTextSourceGaps(value)
	if len(gaps) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(gaps))
	for _, gap := range gaps {
		marker, _ := gap["marker"].(string)
		if repaired[strings.TrimSpace(marker)] != "" {
			continue
		}
		out = append(out, gap)
	}
	return out
}

func decodeVTextSourceGaps(value any) []map[string]any {
	if value == nil {
		return nil
	}
	var gaps []map[string]any
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		data, _ := json.Marshal(typed)
		_ = json.Unmarshal(data, &gaps)
	case json.RawMessage:
		_ = json.Unmarshal(typed, &gaps)
	default:
		data, _ := json.Marshal(typed)
		_ = json.Unmarshal(data, &gaps)
	}
	return gaps
}

func buildMarkdownLineageContentItem(ownerID, sourcePath, title string, version vtextMarkdownLineageVersion, content string, now time.Time) types.ContentItem {
	label := strings.TrimSpace(version.Label)
	if label == "" {
		label = strings.TrimSpace(version.SourceRevisionID)
	}
	if label == "" {
		label = "snapshot"
	}
	hash := contentHash(content)
	meta, _ := json.Marshal(map[string]any{
		"source_path":        sourcePath,
		"source_label":       label,
		"source_revision_id": strings.TrimSpace(version.SourceRevisionID),
		"snapshot_hash":      "sha256:" + hash,
	})
	prov, _ := json.Marshal(map[string]any{
		"created_from":       "vtext_markdown_lineage_import",
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
		AppHint:     AgentProfileTexture,
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

func (h *APIHandler) resolveMarkdownLineageVersion(ctx context.Context, ownerID string, version vtextMarkdownLineageVersion) (resolvedMarkdownLineageVersion, error) {
	resolved := resolvedMarkdownLineageVersion{
		Version:       version,
		Content:       version.Content,
		ContentHash:   contentHash(version.Content),
		ContentSource: "request_content",
	}
	contentItemID := strings.TrimSpace(version.ContentItemID)
	if contentItemID == "" {
		return resolved, nil
	}
	item, err := h.rt.Store().GetContentItem(ctx, ownerID, contentItemID)
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
		hash = contentHash(content)
	}
	resolved.Content = item.TextContent
	resolved.ContentItem = &item
	resolved.ContentID = item.ContentID
	resolved.ContentHash = hash
	resolved.ContentPath = firstNonEmpty(item.FilePath, item.SourceURL, item.CanonicalURL)
	resolved.ContentSource = "content_item"
	return resolved, nil
}
