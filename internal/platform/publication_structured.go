package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

func normalizePublishTextureStructuredInput(req *PublishTextureRequest) error {
	if req == nil || strings.TrimSpace(string(req.BodyDoc)) == "" {
		return nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(req.BodyDoc, &doc); err != nil {
		return fmt.Errorf("body_doc must be valid StructuredTextureDoc JSON: %w", err)
	}
	var entities []texturedoc.SourceEntity
	sourceEntitiesText := strings.TrimSpace(string(req.SourceEntities))
	if sourceEntitiesText != "" && sourceEntitiesText != "null" {
		if err := json.Unmarshal(req.SourceEntities, &entities); err != nil {
			return fmt.Errorf("source_entities must be valid SourceEntity JSON: %w", err)
		}
	}
	projection, err := texturedoc.Project(doc, entities)
	if err != nil {
		return fmt.Errorf("body_doc/source_entities are invalid: %w", err)
	}
	if strings.TrimSpace(req.Content) != "" && req.Content != projection.Text {
		return fmt.Errorf("content must match derived body_doc projection")
	}
	req.Content = projection.Text
	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal normalized body_doc: %w", err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return fmt.Errorf("marshal normalized source_entities: %w", err)
	}
	req.BodyDoc = json.RawMessage(bodyDocJSON)
	req.SourceEntities = json.RawMessage(sourceEntitiesJSON)
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
