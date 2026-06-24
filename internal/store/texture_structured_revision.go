package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func prepareTextureRevisionV2(rev types.Revision) (types.Revision, string, string, error) {
	hasBodyDoc := len(strings.TrimSpace(string(rev.BodyDoc))) > 0
	sourceEntitiesRaw := strings.TrimSpace(string(rev.SourceEntities))

	var (
		doc      texturedoc.StructuredTextureDoc
		entities []texturedoc.SourceEntity
		err      error
	)
	if hasBodyDoc {
		doc, entities, err = decodeStructuredTextureRevision(rev.BodyDoc, rev.SourceEntities, rev.Metadata)
		if err != nil {
			return types.Revision{}, "", "", err
		}
	} else {
		if sourceEntitiesRaw != "" && sourceEntitiesRaw != "[]" && sourceEntitiesRaw != "null" {
			return types.Revision{}, "", "", fmt.Errorf("%w: source_entities require body_doc", ErrInvalidTextureRevision)
		}
		if rev.AuthorKind != types.AuthorUser {
			return types.Revision{}, "", "", fmt.Errorf("%w: body_doc is required for non-user Texture revisions", ErrInvalidTextureRevision)
		}
		doc = userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, rev.Content)
		entities = []texturedoc.SourceEntity{}
	}
	if err := rejectLegacySourceSidecars(rev); err != nil {
		return types.Revision{}, "", "", err
	}

	unusedIDs := textureRevisionUnusedSourceEntityIDs(rev.Metadata)
	projection, err := texturedoc.Project(doc, entities, unusedIDs...)
	if err != nil {
		return types.Revision{}, "", "", fmt.Errorf("%w: %v", ErrInvalidTextureRevision, err)
	}
	if hasBodyDoc && strings.TrimSpace(rev.Content) != "" && rev.Content != projection.Text {
		return types.Revision{}, "", "", fmt.Errorf("%w: content must match derived body_doc projection", ErrInvalidTextureRevision)
	}
	rev.Content = projection.Text

	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		return types.Revision{}, "", "", fmt.Errorf("%w: marshal body_doc: %v", ErrInvalidTextureRevision, err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return types.Revision{}, "", "", fmt.Errorf("%w: marshal source_entities: %v", ErrInvalidTextureRevision, err)
	}
	rev.BodyDoc = json.RawMessage(bodyDocJSON)
	rev.SourceEntities = json.RawMessage(sourceEntitiesJSON)
	return rev, string(bodyDocJSON), string(sourceEntitiesJSON), nil
}

func decodeStructuredTextureRevision(bodyDocRaw, sourceEntitiesRaw json.RawMessage, metadataRaw ...json.RawMessage) (texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity, error) {
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(bodyDocRaw, &doc); err != nil {
		return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("%w: body_doc must be valid StructuredTextureDoc JSON: %v", ErrInvalidTextureRevision, err)
	}

	var entities []texturedoc.SourceEntity
	sourceEntitiesText := strings.TrimSpace(string(sourceEntitiesRaw))
	if sourceEntitiesText == "" || sourceEntitiesText == "null" {
		entities = []texturedoc.SourceEntity{}
	} else if err := json.Unmarshal(sourceEntitiesRaw, &entities); err != nil {
		return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("%w: source_entities must be a valid SourceEntity array: %v", ErrInvalidTextureRevision, err)
	}

	var unusedIDs []string
	if len(metadataRaw) > 0 {
		unusedIDs = textureRevisionUnusedSourceEntityIDs(metadataRaw[0])
	}
	if err := texturedoc.Validate(doc, entities, unusedIDs...); err != nil {
		return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("%w: %v", ErrInvalidTextureRevision, err)
	}
	return doc, entities, nil
}

// textureRevisionUnusedSourceEntityIDs reads the unused_source_entity_ids list
// from revision metadata so the tri-state source invariant round-trips through
// the store's v2 revision preparation.
func textureRevisionUnusedSourceEntityIDs(metadataRaw json.RawMessage) []string {
	if len(strings.TrimSpace(string(metadataRaw))) == 0 {
		return nil
	}
	var meta map[string]any
	if err := json.Unmarshal(metadataRaw, &meta); err != nil {
		return nil
	}
	value, ok := meta["unused_source_entity_ids"]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func rejectLegacySourceSidecars(rev types.Revision) error {
	if rawJSONCarriesData(rev.Citations, "[]") {
		return fmt.Errorf("%w: citations_json is legacy source identity; use body_doc source_ref nodes plus top-level source_entities", ErrInvalidTextureRevision)
	}
	metadata := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(rev.Metadata))) > 0 {
		if err := json.Unmarshal(rev.Metadata, &metadata); err != nil {
			return fmt.Errorf("%w: metadata must be a JSON object: %v", ErrInvalidTextureRevision, err)
		}
	}
	for _, key := range legacySourceSidecarMetadataKeys {
		raw, ok := metadata[key]
		if !ok || !rawJSONCarriesData(raw, "{}") {
			continue
		}
		return fmt.Errorf("%w: metadata.%s is legacy source identity; use body_doc source_ref nodes plus top-level source_entities", ErrInvalidTextureRevision, key)
	}
	return nil
}

var legacySourceSidecarMetadataKeys = []string{
	"source_entities",
	"media_source_refs",
	"source_gaps",
	"source_repair_resolutions",
	"source_attachment_manifest",
	"source_ref_normalization",
}

func rawJSONCarriesData(raw json.RawMessage, emptyObjectDefault string) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	if trimmed == "[]" || trimmed == "{}" || trimmed == emptyObjectDefault {
		return false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return true
	}
	switch typed := value.(type) {
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func userAuthoredTextStructuredTextureDoc(docID, revisionID, content string) texturedoc.StructuredTextureDoc {
	docNodeID := stableStructuredNodeID("doc", docID, revisionID, "root")
	paragraphID := stableStructuredNodeID("p", docID, revisionID, "0")
	return texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": docNodeID},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": paragraphID},
				Content: plainTextInlineNodes(content),
			}},
		},
	}
}

func plainTextInlineNodes(content string) []texturedoc.Node {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	nodes := make([]texturedoc.Node, 0, len(parts)*2-1)
	for i, part := range parts {
		if i > 0 {
			nodes = append(nodes, texturedoc.Node{Type: "hard_break"})
		}
		if part != "" {
			nodes = append(nodes, texturedoc.Node{Type: "text", Text: part})
		}
	}
	return nodes
}

func stableStructuredNodeID(prefix, docID, revisionID, suffix string) string {
	parts := []string{strings.TrimSpace(prefix), strings.TrimSpace(docID), strings.TrimSpace(revisionID), strings.TrimSpace(suffix)}
	for i, part := range parts {
		if part == "" {
			parts[i] = "unknown"
		}
	}
	return strings.Join(parts, "-")
}
