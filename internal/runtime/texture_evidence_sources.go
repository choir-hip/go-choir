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
		entity.Kind = "content_item"
		entity.Target = textureSourceEntityTarget{TargetKind: "content_item", URL: sourceURI, CanonicalURL: sourceURI}
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

func (rt *Runtime) evidenceSourceEntitiesFromWorkerUpdates(ctx context.Context, ownerID string, updates []types.WorkerUpdateRecord) []textureSourceEntity {
	if len(updates) == 0 {
		return nil
	}
	seenEvidence := map[string]bool{}
	entities := []textureSourceEntity{}
	seenEntity := map[string]bool{}
	for _, update := range updates {
		if ownerID != "" && strings.TrimSpace(update.OwnerID) != strings.TrimSpace(ownerID) {
			continue
		}
		for _, evidenceID := range update.EvidenceIDs {
			evidenceID = strings.TrimSpace(evidenceID)
			if evidenceID == "" || seenEvidence[evidenceID] {
				continue
			}
			seenEvidence[evidenceID] = true
			if rt == nil || rt.store == nil {
				continue
			}
			rec, err := rt.store.GetEvidence(ctx, evidenceID, ownerID)
			if err != nil {
				continue
			}
			entity := evidenceRecordToSourceEntity(rec)
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" || seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
		for _, ref := range workerUpdateSourceRefCandidates(update) {
			entity := sourceEntityFromWorkerUpdateRef(ctx, rt, ownerID, ref)
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" || seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
		for _, entity := range workerUpdateDirectSourceEntities(update) {
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" || seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
	}
	return entities
}

func workerUpdateSourceRefCandidates(update types.WorkerUpdateRecord) []string {
	out := make([]string, 0, len(update.Refs)+len(update.Artifacts)+len(update.Tests))
	out = append(out, update.Refs...)
	for _, artifact := range update.Artifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact == "" {
			continue
		}
		if key, _ := splitTypedWorkerUpdateRef(artifact); key != "" {
			out = append(out, artifact)
			continue
		}
		if looksLikeArtifactPath(artifact) {
			out = append(out, "file_artifact:"+artifact)
		}
	}
	for _, test := range update.Tests {
		test = strings.TrimSpace(test)
		if test == "" {
			continue
		}
		if key, _ := splitTypedWorkerUpdateRef(test); key != "" {
			out = append(out, test)
		}
	}
	return out
}

func workerUpdateDirectSourceEntities(update types.WorkerUpdateRecord) []textureSourceEntity {
	out := []textureSourceEntity{}
	for _, test := range update.Tests {
		test = strings.TrimSpace(test)
		if test == "" {
			continue
		}
		if key, _ := splitTypedWorkerUpdateRef(test); key != "" {
			continue
		}
		out = append(out, executionEvidenceSourceEntity("test_run", stableSourceEntityID("test_run_text", test), test, firstNonEmpty(update.AgentID, update.Role)))
	}
	return out
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
	updates := make([]types.WorkerUpdateRecord, 0, limit)
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
	if rt == nil || rt.store == nil {
		return nil
	}
	textureAgentID = strings.TrimSpace(textureAgentID)
	ownerID = strings.TrimSpace(ownerID)
	if textureAgentID == "" || ownerID == "" {
		return nil
	}
	updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, textureAgentID, limit)
	if err != nil || len(updates) == 0 {
		return nil
	}
	return rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, updates)
}
