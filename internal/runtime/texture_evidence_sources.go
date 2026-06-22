package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// evidenceContentID extracts an owner-scoped content item id from an evidence
// record's metadata (content_id) so the citation/quote validator can retrieve the
// stored body to verify excerpts against. Returns "" when no content id is
// declared.
func evidenceContentID(rec types.EvidenceRecord) string {
	if len(rec.Metadata) == 0 {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal(rec.Metadata, &meta); err != nil {
		return ""
	}
	for _, key := range []string{"content_id", "content_item_id"} {
		if id := strings.TrimSpace(metadataString(meta, key)); id != "" {
			return id
		}
	}
	return ""
}

// evidenceTextQuote extracts an explicitly declared quote selector from evidence
// metadata. EvidenceRecord.Content is intentionally generic: live researchers use
// it for summaries, interpretations, and bounded notes as well as exact excerpts,
// so treating it as a literal quote creates false citation-validation failures.
func evidenceTextQuote(rec types.EvidenceRecord) string {
	if len(rec.Metadata) == 0 {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal(rec.Metadata, &meta); err != nil {
		return ""
	}
	for _, key := range []string{"text_quote", "source_quote", "quote", "selector_text"} {
		if quote := strings.TrimSpace(metadataString(meta, key)); quote != "" {
			return quote
		}
	}
	return ""
}

// evidenceRecordToSourceEntity turns a typed researcher evidence record into a
// collated source entity. Evidence with a retrievable content item becomes a
// whole_resource reference by default; if metadata explicitly declares a
// text_quote, the entity carries that quote selector and the deterministic
// citation/quote validator checks it against the stored body at write time.
// Evidence without a retrievable body becomes a whole_resource reference (cited
// resolution still validated, quote not). Returns a zero entity (EntityID == "")
// when there is nothing addressable to cite.
func evidenceRecordToSourceEntity(rec types.EvidenceRecord) textureSourceEntity {
	quote := evidenceTextQuote(rec)
	contentID := evidenceContentID(rec)
	sourceURI := strings.TrimSpace(rec.SourceURI)
	executionKind, executionID := executionEvidenceTarget(rec)

	var entity textureSourceEntity
	switch {
	case contentID != "":
		entity.EntityID = stableSourceEntityID("content_item", contentID)
		entity.Kind = "content_item"
		entity.Target = textureSourceEntityTarget{TargetKind: "content_item", ContentID: contentID}
		if isHTTPURL(sourceURI) {
			entity.Target.URL = sourceURI
			entity.Target.CanonicalURL = sourceURI
		}
		if quote != "" {
			entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "text_quote", TextQuote: quote}}
		} else {
			entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "whole_resource"}}
		}
	case isHTTPURL(sourceURI):
		entity.EntityID = stableSourceEntityID("content_item", sourceURI)
		entity.Kind = "web_url"
		entity.Target = textureSourceEntityTarget{TargetKind: "web_url", URL: sourceURI, CanonicalURL: sourceURI}
		entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "whole_resource"}}
	case executionKind != "" && executionID != "":
		entity = executionEvidenceSourceEntity(executionKind, executionID, strings.TrimSpace(firstNonEmpty(rec.Title, rec.SourceURI, rec.EvidenceID)), strings.TrimSpace(rec.AgentID))
	default:
		return textureSourceEntity{}
	}

	if entity.Label == "" {
		entity.Label = firstNonEmpty(strings.TrimSpace(rec.Title), contentID, sourceURI, "Coagent source")
	}
	if entity.Display.OpenSurface == "" {
		entity.Display = textureSourceEntityDisplay{
			InlineMode:       "collapsed_citation",
			ExpandedMode:     "source_card",
			OpenSurface:      sourcecontract.OpenSurfaceSource,
			DefaultCollapsed: true,
		}
	}
	if entity.Evidence.State == "" {
		entity.Evidence = textureSourceEntityEvidence{State: "available", ResearchState: "represented"}
	}
	if entity.Provenance.CreatedBy == "" {
		entity.Provenance = textureSourceEntityProvenance{
			CreatedBy:           firstNonEmpty(strings.TrimSpace(rec.AgentID), "coagent"),
			RightsScope:         "private_user_source",
			UntrustedSourceText: true,
		}
	}
	return entity
}

func sourceEntityFromWorkerUpdateRef(ctx context.Context, rt *Runtime, ownerID, ref string) textureSourceEntity {
	key, value := splitTypedWorkerUpdateRef(ref)
	if key == "" || value == "" {
		return textureSourceEntity{}
	}
	switch key {
	case "source_service_item":
		if !textureRawSourceServiceItemIDRE.MatchString(value) || textureRawSourceServiceItemIDRE.FindString(value) != value {
			return textureSourceEntity{}
		}
		return sourceServiceItemRefToSourceEntity(value, ref)
	case "content_id", "content_item":
		if rt == nil || rt.store == nil {
			return textureSourceEntity{}
		}
		item, err := rt.store.GetContentItem(ctx, ownerID, value)
		if err != nil {
			return textureSourceEntity{}
		}
		return contentItemRefToSourceEntity(item)
	case "evidence", "evidence_id":
		if rt == nil || rt.store == nil {
			return textureSourceEntity{}
		}
		rec, err := rt.store.GetEvidence(ctx, value, ownerID)
		if err != nil {
			return textureSourceEntity{}
		}
		return evidenceRecordToSourceEntity(rec)
	case "command_output", "shell_session", "diff_hunk", "patch", "test_run", "app_change_package", "screenshot", "video_artifact", "benchmark_log", "file_artifact":
		return executionEvidenceSourceEntity(key, value, value, "coagent")
	default:
		return textureSourceEntity{}
	}
}

func (rt *Runtime) evidenceSourceEntitiesFromWorkerUpdates(ctx context.Context, ownerID string, updates []types.CoagentSourcePacket) []textureSourceEntity {
	entities, _ := rt.evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(ctx, ownerID, updates)
	return entities
}

func (rt *Runtime) evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(ctx context.Context, ownerID string, updates []types.CoagentSourcePacket) ([]textureSourceEntity, []coagentSourceRejection) {
	if len(updates) == 0 {
		return nil, nil
	}
	entities := []textureSourceEntity{}
	rejections := []coagentSourceRejection{}
	seenEntity := map[string]bool{}
	seenRejection := map[string]bool{}
	for _, update := range updates {
		if ownerID != "" && strings.TrimSpace(update.OwnerID) != strings.TrimSpace(ownerID) {
			continue
		}
		for _, packetSource := range update.Packet.Sources {
			entity := sourceEntityFromCoagentPacketSource(ctx, rt, ownerID, packetSource, update)
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" {
				rejection := coagentSourceRejectionFromPacketSource(update, packetSource)
				rejectionKey := sourceRejectionKey(rejection)
				if rejectionKey != "" && !seenRejection[rejectionKey] {
					seenRejection[rejectionKey] = true
					rejections = append(rejections, rejection)
				}
				continue
			}
			if seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
	}
	return entities, rejections
}

func sourceEntityFromCoagentPacketSource(ctx context.Context, rt *Runtime, ownerID string, source types.CoagentPacketSource, update types.CoagentSourcePacket) textureSourceEntity {
	uri := strings.TrimSpace(source.Target.URI)
	kind := strings.TrimSpace(source.Kind)
	var entity textureSourceEntity
	if uri != "" {
		entity = sourceEntityFromWorkerUpdateRef(ctx, rt, ownerID, uri)
		if entity.EntityID == "" && isHTTPURL(uri) {
			entity = textureSourceEntity{
				EntityID:  stableSourceEntityID("content_item", uri),
				Kind:      "web_url",
				Label:     firstNonEmpty(strings.TrimSpace(source.Target.Title), uri),
				Target:    textureSourceEntityTarget{TargetKind: "web_url", URL: uri, CanonicalURL: uri},
				Selectors: []textureSourceEntitySelector{{SelectorKind: sourcecontract.SelectorKindWholeResource}},
				Display: textureSourceEntityDisplay{
					InlineMode:       "collapsed_citation",
					ExpandedMode:     "source_card",
					OpenSurface:      sourcecontract.OpenSurfaceSource,
					DefaultCollapsed: true,
				},
				Evidence: textureSourceEntityEvidence{State: sourcecontract.EvidenceStateAvailable, ResearchState: "represented"},
				Provenance: textureSourceEntityProvenance{
					CreatedBy:           firstNonEmpty(strings.TrimSpace(update.AgentID), "coagent"),
					RightsScope:         "private_user_source",
					UntrustedSourceText: true,
				},
			}
		}
	}
	if entity.EntityID == "" {
		identity := strings.TrimSpace(source.SourceID)
		if identity == "" {
			identity = strings.TrimSpace(source.Target.Title)
		}
		if executionTargetKind(kind) && identity != "" {
			entity = executionEvidenceSourceEntity(kind, identity, firstNonEmpty(source.Target.Title, identity), firstNonEmpty(update.AgentID, update.Role))
		}
	}
	if entity.EntityID == "" {
		return textureSourceEntity{}
	}
	if label := strings.TrimSpace(source.Target.Title); coagentPacketSourceTitleUsable(label) {
		entity.Label = label
	}
	if len(source.Selectors) > 0 {
		selectors := make([]textureSourceEntitySelector, 0, len(source.Selectors))
		for _, selector := range source.Selectors {
			switch strings.TrimSpace(selector.Kind) {
			case sourcecontract.SelectorKindTextQuote:
				if quote := strings.TrimSpace(selector.Quote); quote != "" {
					selectors = append(selectors, textureSourceEntitySelector{SelectorKind: sourcecontract.SelectorKindTextQuote, TextQuote: quote})
				}
			case sourcecontract.SelectorKindWholeResource, "":
				selectors = append(selectors, textureSourceEntitySelector{SelectorKind: sourcecontract.SelectorKindWholeResource})
			default:
				selectors = append(selectors, textureSourceEntitySelector{SelectorKind: strings.TrimSpace(selector.Kind)})
			}
		}
		if len(selectors) > 0 {
			entity.Selectors = selectors
		}
	}
	applyCoagentPacketSourceContent(&entity, source)
	if state := strings.TrimSpace(source.Evidence.State); state != "" {
		entity.Evidence.State = state
	}
	if rights := strings.TrimSpace(source.Evidence.RightsScope); rights != "" {
		entity.Provenance.RightsScope = rights
	}
	return entity
}

func coagentPacketSourceTitleUsable(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	key, value := splitTypedWorkerUpdateRef(title)
	if key != "" && value != "" {
		return false
	}
	return true
}

func applyCoagentPacketSourceContent(entity *textureSourceEntity, source types.CoagentPacketSource) {
	if entity == nil || strings.TrimSpace(entity.EntityID) == "" {
		return
	}
	excerpt := coagentPacketSourceExcerpt(source)
	readerText := coagentPacketSourceReaderText(source, excerpt)
	if excerpt != "" && !sourceEntityHasTextQuote(entity.Selectors) {
		entity.Selectors = append([]textureSourceEntitySelector{{
			SelectorKind: sourcecontract.SelectorKindTextQuote,
			TextQuote:    excerpt,
		}}, entity.Selectors...)
	}
	if readerText == "" {
		return
	}
	snapshot := map[string]any{
		"text_content": readerText,
		"source_url":   firstNonEmpty(coagentPacketSourceReaderSnapshotSourceURL(source), strings.TrimSpace(source.Target.URI)),
		"snapshot_kind": firstNonEmpty(
			coagentPacketSourceReaderSnapshotKind(source),
			"researcher_read_source_text",
		),
		"media_type":   firstNonEmpty(coagentPacketSourceReaderSnapshotMediaType(source), strings.TrimSpace(source.Target.MediaType), "text/markdown"),
		"access_scope": firstNonEmpty(coagentPacketSourceReaderSnapshotAccessScope(source), "private_user_source"),
		"rights_scope": firstNonEmpty(strings.TrimSpace(source.Evidence.RightsScope), entity.Provenance.RightsScope),
		"excerpt_text": excerpt,
		"source_title": strings.TrimSpace(source.Target.Title),
		"source_id":    strings.TrimSpace(source.SourceID),
	}
	if source.ReaderSnapshot != nil {
		if source.ReaderSnapshot.OriginalMediaType != "" {
			snapshot["original_media_type"] = strings.TrimSpace(source.ReaderSnapshot.OriginalMediaType)
		}
		if source.ReaderSnapshot.Truncated {
			snapshot["truncated"] = true
		}
	}
	entity.ReaderSnapshot = pruneEmptyMap(snapshot)
	state := sourcecontract.ReaderArtifactStateReady
	if source.ReaderSnapshot == nil || strings.TrimSpace(source.ReaderSnapshot.TextContent) == "" {
		state = sourcecontract.ReaderArtifactStateBoundedExcerptOnly
	}
	status := map[string]any{"state": state}
	if source.ReaderSnapshot != nil && source.ReaderSnapshot.Truncated {
		status["truncated"] = true
	}
	entity.ReaderSnapshotStatus = status
	entity.Evidence.ReaderSnapshot = true
	entity.Evidence.BodyKind = "reader_snapshot"
	entity.Evidence.BodyLength = len([]rune(readerText))
	entity.Evidence.SourceRepresentationID = state
}

func coagentPacketSourceExcerpt(source types.CoagentPacketSource) string {
	if excerpt := strings.TrimSpace(source.Excerpt); excerpt != "" {
		return truncateRunes(excerpt, 2000)
	}
	for _, selector := range source.Selectors {
		if sourcecontract.NormalizeSelectorKind(selector.Kind) == sourcecontract.SelectorKindTextQuote {
			if quote := strings.TrimSpace(selector.Quote); quote != "" {
				return truncateRunes(quote, 2000)
			}
		}
	}
	if source.ReaderSnapshot != nil {
		if text := strings.TrimSpace(source.ReaderSnapshot.TextContent); text != "" {
			return truncateRunes(text, 720)
		}
	}
	return ""
}

func coagentPacketSourceReaderText(source types.CoagentPacketSource, excerpt string) string {
	if source.ReaderSnapshot != nil {
		if text := strings.TrimSpace(source.ReaderSnapshot.TextContent); text != "" {
			return text
		}
	}
	return strings.TrimSpace(excerpt)
}

func coagentPacketSourceReaderSnapshotKind(source types.CoagentPacketSource) string {
	if source.ReaderSnapshot == nil {
		return ""
	}
	return strings.TrimSpace(source.ReaderSnapshot.SnapshotKind)
}

func coagentPacketSourceReaderSnapshotMediaType(source types.CoagentPacketSource) string {
	if source.ReaderSnapshot == nil {
		return ""
	}
	return strings.TrimSpace(source.ReaderSnapshot.MediaType)
}

func coagentPacketSourceReaderSnapshotAccessScope(source types.CoagentPacketSource) string {
	if source.ReaderSnapshot == nil {
		return ""
	}
	return strings.TrimSpace(source.ReaderSnapshot.AccessScope)
}

func coagentPacketSourceReaderSnapshotSourceURL(source types.CoagentPacketSource) string {
	if source.ReaderSnapshot == nil {
		return ""
	}
	return strings.TrimSpace(source.ReaderSnapshot.SourceURL)
}

func sourceEntityHasTextQuote(selectors []textureSourceEntitySelector) bool {
	for _, selector := range selectors {
		if sourcecontract.NormalizeSelectorKind(selector.SelectorKind) == sourcecontract.SelectorKindTextQuote && strings.TrimSpace(selector.TextQuote) != "" {
			return true
		}
	}
	return false
}

func pruneEmptyMap(values map[string]any) map[string]any {
	for key, value := range values {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				delete(values, key)
			}
		case nil:
			delete(values, key)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func (rt *Runtime) evidenceSourceEntitiesFromWorkerUpdateIDs(ctx context.Context, ownerID, targetAgentID string, updateIDs []string, limit int) []textureSourceEntity {
	if rt == nil || rt.store == nil || len(updateIDs) == 0 {
		return nil
	}
	ownerID = strings.TrimSpace(ownerID)
	targetAgentID = strings.TrimSpace(targetAgentID)
	if ownerID == "" || targetAgentID == "" {
		return nil
	}
	if limit <= 0 || limit > len(updateIDs) {
		limit = len(updateIDs)
	}
	updates := make([]types.CoagentSourcePacket, 0, limit)
	seen := map[string]bool{}
	for _, updateID := range updateIDs {
		updateID = strings.TrimSpace(updateID)
		if updateID == "" || seen[updateID] {
			continue
		}
		seen[updateID] = true
		update, err := rt.store.GetWorkerUpdate(ctx, ownerID, updateID)
		if err != nil {
			continue
		}
		if strings.TrimSpace(update.TargetAgentID) != targetAgentID {
			continue
		}
		updates = append(updates, update)
		if len(updates) >= limit {
			break
		}
	}
	return rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, updates)
}

func coagentSourceRejectionFromPacketSource(update types.CoagentSourcePacket, source types.CoagentPacketSource) coagentSourceRejection {
	reason := "packet source did not materialize into a Texture source entity"
	if strings.TrimSpace(source.Kind) == "" {
		reason = "packet source kind is missing"
	} else if strings.TrimSpace(source.Target.URI) == "" {
		reason = "packet source target.uri is missing"
	}
	return coagentSourceRejection{
		UpdateID:  strings.TrimSpace(update.UpdateID),
		SourceID:  strings.TrimSpace(source.SourceID),
		Kind:      strings.TrimSpace(source.Kind),
		TargetURI: strings.TrimSpace(source.Target.URI),
		Reason:    reason,
	}
}

func sourceRejectionKey(rejection coagentSourceRejection) string {
	parts := []string{
		strings.TrimSpace(rejection.UpdateID),
		strings.TrimSpace(rejection.SourceID),
		strings.TrimSpace(rejection.Kind),
		strings.TrimSpace(rejection.TargetURI),
	}
	return strings.Join(parts, "\x00")
}

func decodeAvailableTextureSourceEntities(metadata map[string]any) []textureSourceEntity {
	if metadata == nil {
		return nil
	}
	return decodeTextureSourceEntities(metadata[textureAvailableSourceEntitiesKey])
}

func mergeTextureSourceEntitiesIntoAvailableContext(metadata map[string]any, incoming []textureSourceEntity) bool {
	if metadata == nil || len(incoming) == 0 {
		return false
	}
	existing := decodeAvailableTextureSourceEntities(metadata)
	merged, changed := mergeTextureSourceEntities(existing, incoming)
	if len(merged) > 0 {
		metadata[textureAvailableSourceEntitiesKey] = merged
	}
	return changed
}

func mergeTextureSourceEntitiesIntoRunMetadata(rec *types.RunRecord, incoming []textureSourceEntity) bool {
	if rec == nil || len(incoming) == 0 {
		return false
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	return mergeTextureSourceEntitiesIntoAvailableContext(rec.Metadata, incoming)
}

const textureSourceRejectionsKey = "texture_source_rejections"

func decodeCoagentSourceRejections(value any) []coagentSourceRejection {
	if value == nil {
		return nil
	}
	var rejections []coagentSourceRejection
	switch typed := value.(type) {
	case []coagentSourceRejection:
		rejections = typed
	case []any:
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(raw, &rejections); err != nil {
			return nil
		}
	case json.RawMessage:
		if err := json.Unmarshal(typed, &rejections); err != nil {
			return nil
		}
	case []byte:
		if err := json.Unmarshal(typed, &rejections); err != nil {
			return nil
		}
	case string:
		if err := json.Unmarshal([]byte(typed), &rejections); err != nil {
			return nil
		}
	default:
		return nil
	}
	out := make([]coagentSourceRejection, 0, len(rejections))
	for _, rejection := range rejections {
		if strings.TrimSpace(rejection.Reason) == "" {
			continue
		}
		out = append(out, rejection)
	}
	return out
}

func mergeCoagentSourceRejectionsIntoRunMetadata(rec *types.RunRecord, incoming []coagentSourceRejection) bool {
	if rec == nil || len(incoming) == 0 {
		return false
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	existing := decodeCoagentSourceRejections(rec.Metadata[textureSourceRejectionsKey])
	merged, changed := mergeCoagentSourceRejections(existing, incoming)
	if changed {
		rec.Metadata[textureSourceRejectionsKey] = merged
	}
	return changed
}

func mergeCoagentSourceRejectionsIntoMetadata(metadata map[string]any, incoming []coagentSourceRejection) bool {
	if metadata == nil || len(incoming) == 0 {
		return false
	}
	existing := decodeCoagentSourceRejections(metadata[textureSourceRejectionsKey])
	merged, changed := mergeCoagentSourceRejections(existing, incoming)
	if changed {
		metadata[textureSourceRejectionsKey] = merged
	}
	return changed
}

func mergeCoagentSourceRejections(existing, incoming []coagentSourceRejection) ([]coagentSourceRejection, bool) {
	merged := append([]coagentSourceRejection{}, existing...)
	seen := map[string]bool{}
	for _, rejection := range merged {
		if key := sourceRejectionKey(rejection); key != "" {
			seen[key] = true
		}
	}
	changed := false
	for _, rejection := range incoming {
		key := sourceRejectionKey(rejection)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, rejection)
		changed = true
	}
	return merged, changed
}

func formatCoagentSourceRejectionsForPrompt(rejections []coagentSourceRejection) string {
	if len(rejections) == 0 {
		return ""
	}
	var b strings.Builder
	for _, rejection := range rejections {
		reason := strings.TrimSpace(rejection.Reason)
		if reason == "" {
			continue
		}
		b.WriteString("- ")
		if rejection.SourceID != "" {
			b.WriteString("source_id=")
			b.WriteString(rejection.SourceID)
			b.WriteString(" ")
		}
		if rejection.UpdateID != "" {
			b.WriteString("update_id=")
			b.WriteString(rejection.UpdateID)
			b.WriteString(" ")
		}
		if rejection.Kind != "" {
			b.WriteString("kind=")
			b.WriteString(rejection.Kind)
			b.WriteString(" ")
		}
		if rejection.TargetURI != "" {
			b.WriteString("target_uri=")
			b.WriteString(rejection.TargetURI)
			b.WriteString(" ")
		}
		b.WriteString("reason=")
		b.WriteString(reason)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func splitTypedWorkerUpdateRef(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	for _, sep := range []string{":", "="} {
		if before, after, ok := strings.Cut(ref, sep); ok {
			key := normalizeWorkerUpdateRefKey(before)
			value := strings.TrimSpace(after)
			if key == "" || value == "" || strings.ContainsAny(value, " \t\r\n") {
				return "", ""
			}
			return key, value
		}
	}
	return "", ""
}

func normalizeWorkerUpdateRefKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "source_service_item", "source_item", "item_id":
		return "source_service_item"
	case "content_id", "content_item", "content_item_id":
		return "content_id"
	case "evidence", "evidence_id":
		return "evidence"
	case "command", "command_output", "cmd_output", "shell_command":
		return "command_output"
	case "shell", "shell_session", "terminal_session":
		return "shell_session"
	case "diff", "diff_hunk", "patch_hunk":
		return "diff_hunk"
	case "patch":
		return "patch"
	case "test", "tests", "test_run", "test_result":
		return "test_run"
	case "app_change_package", "change_package", "package":
		return "app_change_package"
	case "screenshot", "image_artifact":
		return "screenshot"
	case "video_artifact", "video_proof":
		return "video_artifact"
	case "benchmark", "benchmark_log":
		return "benchmark_log"
	case "file", "file_artifact", "artifact":
		return "file_artifact"
	default:
		return ""
	}
}

func executionEvidenceTarget(rec types.EvidenceRecord) (string, string) {
	if key, value := splitTypedWorkerUpdateRef(rec.SourceURI); key != "" && executionTargetKind(key) {
		return key, value
	}
	kind := normalizeWorkerUpdateRefKey(rec.Kind)
	if !executionTargetKind(kind) {
		return "", ""
	}
	identity := firstNonEmpty(strings.TrimSpace(rec.SourceURI), strings.TrimSpace(rec.EvidenceID))
	if identity == "" {
		return "", ""
	}
	if key, value := splitTypedWorkerUpdateRef(identity); key != "" && executionTargetKind(key) {
		return key, value
	}
	return kind, identity
}

func executionTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "command_output", "shell_session", "diff_hunk", "patch", "test_run", "app_change_package", "screenshot", "video_artifact", "benchmark_log", "file_artifact":
		return true
	default:
		return false
	}
}

func executionEvidenceSourceEntity(kind, identity, label, createdBy string) textureSourceEntity {
	kind = strings.TrimSpace(kind)
	identity = strings.TrimSpace(identity)
	if !executionTargetKind(kind) || identity == "" {
		return textureSourceEntity{}
	}
	entity := textureSourceEntity{
		EntityID:  stableSourceEntityID(kind, identity),
		Kind:      kind,
		Label:     firstNonEmpty(strings.TrimSpace(label), executionSourceDefaultLabel(kind)),
		Target:    textureSourceTargetForExecution(kind, identity),
		Selectors: []textureSourceEntitySelector{{SelectorKind: sourcecontract.SelectorKindWholeResource}},
		Display: textureSourceEntityDisplay{
			InlineMode:       "collapsed_citation",
			ExpandedMode:     "source_window",
			OpenSurface:      executionSourceOpenSurface(kind),
			DefaultCollapsed: true,
		},
		Evidence: textureSourceEntityEvidence{State: sourcecontract.EvidenceStateAvailable, ResearchState: "represented"},
		Provenance: textureSourceEntityProvenance{
			CreatedBy:           firstNonEmpty(strings.TrimSpace(createdBy), "coagent"),
			RightsScope:         "private_user_source",
			UntrustedSourceText: true,
		},
	}
	return entity
}

func textureSourceTargetForExecution(kind, identity string) textureSourceEntityTarget {
	target := textureSourceEntityTarget{TargetKind: kind}
	switch kind {
	case "file_artifact", "screenshot", "video_artifact", "benchmark_log", "patch":
		target.FilePath = identity
	default:
		target.PublicRecordID = identity
	}
	return target
}

func executionSourceOpenSurface(kind string) string {
	switch kind {
	case "screenshot":
		return sourcecontract.OpenSurfaceImage
	case "video_artifact":
		return sourcecontract.OpenSurfaceVideo
	case "file_artifact", "benchmark_log", "patch":
		return sourcecontract.OpenSurfaceFile
	default:
		return sourcecontract.OpenSurfaceSourceWindow
	}
}

func executionSourceDefaultLabel(kind string) string {
	switch kind {
	case "command_output":
		return "Command output"
	case "shell_session":
		return "Shell session"
	case "diff_hunk":
		return "Diff hunk"
	case "patch":
		return "Patch"
	case "test_run":
		return "Test run"
	case "app_change_package":
		return "AppChangePackage"
	case "screenshot":
		return "Screenshot"
	case "video_artifact":
		return "Video artifact"
	case "benchmark_log":
		return "Benchmark log"
	case "file_artifact":
		return "File artifact"
	default:
		return "Coagent source"
	}
}

func looksLikeArtifactPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	return strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.Contains(value, "/")
}

func isHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// evidenceSourceEntitiesFromPendingUpdates collates source entities from the
// typed evidence records attached to the pending update_coagent records addressed
// to a Texture coagent. This is the typed replacement for the deleted regex
// researcher-prose scraping: sources (and their text_quote excerpts) come from
// structured researcher evidence, not from parsing message text.
func (rt *Runtime) evidenceSourceEntitiesFromPendingUpdates(ctx context.Context, ownerID, textureAgentID string, limit int) []textureSourceEntity {
	entities, _ := rt.evidenceSourceEntitiesAndRejectionsFromPendingUpdates(ctx, ownerID, textureAgentID, limit)
	return entities
}

func (rt *Runtime) evidenceSourceEntitiesAndRejectionsFromPendingUpdates(ctx context.Context, ownerID, textureAgentID string, limit int) ([]textureSourceEntity, []coagentSourceRejection) {
	if rt == nil || rt.store == nil {
		return nil, nil
	}
	textureAgentID = strings.TrimSpace(textureAgentID)
	ownerID = strings.TrimSpace(ownerID)
	if textureAgentID == "" || ownerID == "" {
		return nil, nil
	}
	updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, textureAgentID, limit)
	if err != nil || len(updates) == 0 {
		return nil, nil
	}
	return rt.evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(ctx, ownerID, updates)
}
