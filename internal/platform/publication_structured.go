package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

func normalizePublishTextureStructuredInput(req *PublishTextureRequest) error {
	if req == nil {
		return nil
	}
	return normalizePublishTextureStructuredFields(&req.Content, &req.BodyDoc, &req.SourceEntities)
}

func normalizePublishTextureStructuredFields(content *string, bodyDoc *json.RawMessage, sourceEntities *json.RawMessage) error {
	if bodyDoc == nil || sourceEntities == nil {
		return nil
	}
	bodyDocText := strings.TrimSpace(string(*bodyDoc))
	sourceEntitiesText := strings.TrimSpace(string(*sourceEntities))
	if bodyDocText == "" {
		if sourceEntitiesText != "" && sourceEntitiesText != "null" && sourceEntitiesText != "[]" {
			return fmt.Errorf("source_entities require body_doc source_ref nodes")
		}
		return nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(*bodyDoc, &doc); err != nil {
		return fmt.Errorf("body_doc must be valid StructuredTextureDoc JSON: %w", err)
	}
	var entities []texturedoc.SourceEntity
	if sourceEntitiesText != "" && sourceEntitiesText != "null" {
		if err := json.Unmarshal(*sourceEntities, &entities); err != nil {
			return fmt.Errorf("source_entities must be valid SourceEntity JSON: %w", err)
		}
	}
	projection, err := texturedoc.Project(doc, entities)
	if err != nil {
		return fmt.Errorf("body_doc/source_entities are invalid: %w", err)
	}
	// Refused, not stripped (station TE, docs/design/texture-evidence-
	// embeds-te-2026-10-25.md E6): a publication names its source revision
	// and its projection, so publishing the document without its evidence
	// would publish something other than the revision, silently.
	if private := texturedoc.PrivateEmbeds(doc); len(private) > 0 {
		return fmt.Errorf("%s", PrivateEmbedRefusal(private))
	}
	if content != nil && strings.TrimSpace(*content) != "" && *content != projection.Text {
		return fmt.Errorf("content must match derived body_doc projection")
	}
	if content != nil {
		*content = projection.Text
	}
	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal normalized body_doc: %w", err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return fmt.Errorf("marshal normalized source_entities: %w", err)
	}
	*bodyDoc = json.RawMessage(bodyDocJSON)
	*sourceEntities = json.RawMessage(sourceEntitiesJSON)
	return nil
}

func structuredArtifactFieldsFromManifest(manifestJSON string) (json.RawMessage, json.RawMessage) {
	var envelope struct {
		BodyDoc                  json.RawMessage `json:"body_doc"`
		StructuredSourceEntities json.RawMessage `json:"structured_source_entities"`
	}
	if err := json.Unmarshal([]byte(manifestJSON), &envelope); err != nil {
		return nil, nil
	}
	return envelope.BodyDoc, envelope.StructuredSourceEntities
}

// PrivateEmbedRefusal says why a document that holds private embeds is not
// published: capsule_evidence is supervision material and never published,
// and other kinds wait for public asset pinning.
func PrivateEmbedRefusal(private []texturedoc.Embed) string {
	kinds := map[string]bool{}
	var names []string
	for _, e := range private {
		if !kinds[e.Kind] {
			kinds[e.Kind] = true
			names = append(names, e.Kind)
		}
	}
	return fmt.Sprintf("the document holds %d private embed(s) (%s: %s); evidence and owned media are never published, so remove them from the document before publishing it",
		len(private), strings.Join(names, ", "), private[0].ID)
}
