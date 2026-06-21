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
	if err := normalizePublishTextureStructuredFields(&req.Content, &req.BodyDoc, &req.SourceEntities); err != nil {
		return err
	}
	for i := range req.History {
		if err := normalizePublishTextureStructuredFields(&req.History[i].Content, &req.History[i].BodyDoc, &req.History[i].SourceEntities); err != nil {
			return fmt.Errorf("history revision %s structured fields are invalid: %w", firstNonEmpty(req.History[i].RevisionID, fmt.Sprintf("%d", i)), err)
		}
	}
	return nil
}

func normalizePublishTextureStructuredFields(content *string, bodyDoc *json.RawMessage, sourceEntities *json.RawMessage) error {
	if bodyDoc == nil || sourceEntities == nil {
		return nil
	}
	bodyDocText := strings.TrimSpace(string(*bodyDoc))
	sourceEntitiesText := strings.TrimSpace(string(*sourceEntities))
	if bodyDocText == "" {
		if sourceEntitiesText != "" && sourceEntitiesText != "null" && sourceEntitiesText != "[]" {
			return fmt.Errorf("source_entities require body_doc source_ref/source_embed nodes")
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
