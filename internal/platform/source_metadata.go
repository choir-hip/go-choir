package platform

import (
	"encoding/json"
	"fmt"
	"strings"
)

type publicationSourceMetadata struct {
	AccessPolicy   json.RawMessage
	ExportPolicy   json.RawMessage
	SourceEntities []publicationSourceEntityInput
	Transclusions  []publicationTransclusionInput
	MetadataHash   string
}

type publicationSourceEntityInput struct {
	SourceEntityID string
	Kind           string
	TargetKind     string
	TargetID       string
	DisplayPolicy  string
	OpenSurface    string
	EntityJSON     json.RawMessage
}

type publicationTransclusionInput struct {
	SourceEntityID     string
	HostSelector       json.RawMessage
	SourceSelector     json.RawMessage
	RelationType       string
	DefaultDisplayMode string
	SnapshotText       string
	ContentHash        string
	EntityJSON         json.RawMessage
}

func buildPublicationSourceMetadata(req PublishVTextRequest) (publicationSourceMetadata, error) {
	metadata := publicationSourceMetadata{
		AccessPolicy: defaultPublicationAccessPolicy(),
		ExportPolicy: defaultPublicationExportPolicy(),
	}
	if len(req.AccessPolicy) > 0 {
		access, err := validJSONOrObject(req.AccessPolicy, "access_policy")
		if err != nil {
			return metadata, err
		}
		metadata.AccessPolicy = access
	}
	if len(req.ExportPolicy) > 0 {
		export, err := validJSONOrObject(req.ExportPolicy, "export_policy")
		if err != nil {
			return metadata, err
		}
		metadata.ExportPolicy = export
	}
	if len(req.Metadata) == 0 {
		metadata.MetadataHash = sha256Hex([]byte("{}"))
		return metadata, nil
	}
	if !json.Valid(req.Metadata) {
		return metadata, fmt.Errorf("metadata must be valid JSON")
	}
	metadata.MetadataHash = sha256Hex(req.Metadata)

	var revisionMetadata map[string]any
	if err := json.Unmarshal(req.Metadata, &revisionMetadata); err != nil {
		return metadata, fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	if value, ok := revisionMetadata["access_policy"]; ok && len(req.AccessPolicy) == 0 {
		raw, err := marshalJSONObject(value, "metadata.access_policy")
		if err != nil {
			return metadata, err
		}
		metadata.AccessPolicy = raw
	}
	if value, ok := revisionMetadata["route_policy"]; ok && len(req.AccessPolicy) == 0 {
		raw, err := marshalJSONObject(value, "metadata.route_policy")
		if err != nil {
			return metadata, err
		}
		metadata.AccessPolicy = raw
	}
	if value, ok := revisionMetadata["export_policy"]; ok && len(req.ExportPolicy) == 0 {
		raw, err := marshalJSONObject(value, "metadata.export_policy")
		if err != nil {
			return metadata, err
		}
		metadata.ExportPolicy = raw
	}

	rawEntities, ok := revisionMetadata["source_entities"]
	if !ok || rawEntities == nil {
		return metadata, nil
	}
	entityValues, ok := rawEntities.([]any)
	if !ok {
		return metadata, fmt.Errorf("metadata.source_entities must be an array")
	}
	for _, value := range entityValues {
		entity, transclusion, ok, err := normalizePublicationSourceEntity(value)
		if err != nil {
			return metadata, err
		}
		if !ok {
			continue
		}
		metadata.SourceEntities = append(metadata.SourceEntities, entity)
		metadata.Transclusions = append(metadata.Transclusions, transclusion)
	}
	return metadata, nil
}

func normalizePublicationSourceEntity(value any) (publicationSourceEntityInput, publicationTransclusionInput, bool, error) {
	var entity publicationSourceEntityInput
	var transclusion publicationTransclusionInput
	raw, err := json.Marshal(value)
	if err != nil {
		return entity, transclusion, false, fmt.Errorf("marshal source entity: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return entity, transclusion, false, fmt.Errorf("source entity must be an object: %w", err)
	}
	entityID := firstString(m, "entity_id", "id")
	if entityID == "" {
		return entity, transclusion, false, nil
	}
	target := mapValue(m["target"])
	display := mapValue(m["display"])
	selectors := sliceValue(m["selectors"])
	firstSelector := map[string]any{"selector_kind": "whole_resource"}
	if len(selectors) > 0 {
		if selectorMap := mapValue(selectors[0]); len(selectorMap) > 0 {
			firstSelector = selectorMap
		}
	}
	evidenceState := publicationSourceEvidenceState(m)
	sourceSelector, err := marshalPublicationSourceSelector(selectors, firstSelector, evidenceState)
	if err != nil {
		return entity, transclusion, false, fmt.Errorf("marshal source selector: %w", err)
	}
	hostSelector := mustJSONRaw(map[string]any{
		"type":             "citation_marker",
		"source_entity_id": entityID,
	})
	displayPolicy := normalizePublicationDisplayPolicy(firstString(m, "display_policy", "inline_mode"), display, firstSelector)
	targetKind := firstNonEmpty(firstString(target, "target_kind"), firstString(m, "target_kind"))
	targetID := firstNonEmpty(
		firstString(target, "item_id"),
		firstString(target, "content_id"),
		firstString(target, "public_record_id"),
		firstString(target, "publication_version_id"),
		firstString(target, "revision_id"),
		firstString(target, "doc_id"),
		firstString(target, "canonical_url"),
		firstString(target, "url"),
		firstString(m, "target_id"),
	)
	entity = publicationSourceEntityInput{
		SourceEntityID: entityID,
		Kind:           firstString(m, "kind", "source_kind"),
		TargetKind:     targetKind,
		TargetID:       targetID,
		DisplayPolicy:  displayPolicy,
		OpenSurface:    firstString(display, "open_surface"),
		EntityJSON:     raw,
	}
	transclusion = publicationTransclusionInput{
		SourceEntityID:     entityID,
		HostSelector:       hostSelector,
		SourceSelector:     sourceSelector,
		RelationType:       firstNonEmpty(firstString(m, "relation_type"), "references"),
		DefaultDisplayMode: displayPolicy,
		SnapshotText:       firstNonEmpty(firstString(firstSelector, "text_quote"), firstString(m, "snapshot_text")),
		ContentHash:        firstNonEmpty(firstString(firstSelector, "content_hash"), firstString(m, "content_hash")),
		EntityJSON:         raw,
	}
	return entity, transclusion, true, nil
}

func marshalPublicationSourceSelector(selectors []any, firstSelector map[string]any, evidenceState map[string]any) (json.RawMessage, error) {
	if len(selectors) <= 1 {
		selector := copyStringAnyMap(firstSelector)
		if len(evidenceState) > 0 {
			selector["evidence_state"] = evidenceState
		}
		return json.Marshal(selector)
	}
	selectorSet := map[string]any{
		"selector_kind": "selector_set",
		"selectors":     selectors,
	}
	if len(evidenceState) > 0 {
		selectorSet["evidence_state"] = evidenceState
	}
	return json.Marshal(selectorSet)
}

func copyStringAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func publicationSourceEvidenceState(entity map[string]any) map[string]any {
	evidence := mapValue(entity["evidence"])
	if len(evidence) == 0 {
		return nil
	}
	state := normalizePublicationEvidenceState(firstString(evidence, "state", "relation", "research_state"))
	if state == "" {
		return nil
	}
	out := map[string]any{"state": state}
	if relation := normalizePublicationEvidenceState(firstString(evidence, "relation")); relation == "confirms" || relation == "refutes" || relation == "qualifies" {
		out["relation"] = relation
	}
	if researchState := strings.TrimSpace(firstString(evidence, "research_state")); researchState != "" {
		out["research_state"] = researchState
	}
	if uncertainty := strings.TrimSpace(firstString(evidence, "uncertainty")); uncertainty != "" {
		out["uncertainty"] = uncertainty
	}
	return out
}

func normalizePublicationEvidenceState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "candidate", "available", "confirms", "refutes", "qualifies", "no_source_needed", "stale", "blocked_by_access", "unavailable":
		return strings.ToLower(strings.TrimSpace(value))
	case "confirming", "confirmed", "represented", "owner_supplied":
		return "confirms"
	case "refuting", "refuted":
		return "refutes"
	case "qualifying", "qualified":
		return "qualifies"
	case "blocked", "blocked_access", "access_blocked":
		return "blocked_by_access"
	case "not_needed", "no-source-needed", "no_source":
		return "no_source_needed"
	default:
		return ""
	}
}

func normalizePublicationDisplayPolicy(raw string, display map[string]any, selector map[string]any) string {
	raw = strings.TrimSpace(firstNonEmpty(raw, firstString(display, "display_policy"), firstString(display, "inline_mode")))
	switch raw {
	case "collapsed_citation", "embedded_excerpt", "embedded_preview", "expanded":
		return raw
	case "quote", "excerpt":
		return "embedded_excerpt"
	case "preview", "card":
		return "embedded_preview"
	}
	if strings.TrimSpace(firstString(selector, "text_quote")) != "" {
		return "embedded_excerpt"
	}
	if boolValue(display["default_collapsed"]) {
		return "collapsed_citation"
	}
	return "collapsed_citation"
}

func defaultPublicationAccessPolicy() json.RawMessage {
	return mustJSONRaw(map[string]any{
		"visibility": "public",
		"route":      "public",
	})
}

func defaultPublicationExportPolicy() json.RawMessage {
	return mustJSONRaw(map[string]any{
		"copy_allowed":     true,
		"download_allowed": true,
		"formats":          []string{"txt", "md", "html", "docx", "pdf"},
	})
}

func validJSONOrObject(raw json.RawMessage, label string) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%s must be valid JSON", label)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", label, err)
	}
	return raw, nil
}

func marshalJSONObject(value any, label string) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", label, err)
	}
	return validJSONOrObject(raw, label)
}

func mapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return nil
	}
}

func sliceValue(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if m == nil {
			continue
		}
		if value, ok := m[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true"
	default:
		return false
	}
}

func mustJSONRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
