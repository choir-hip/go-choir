package runtime

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

var bareTextureSourceRefRE = regexp.MustCompile(`\[source:([A-Za-z0-9_.:-]{1,160})\]`)

func normalizeWireArticleBareSourceRefs(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeTextureSourceEntities(meta["source_entities"])
	if len(entities) == 0 || !strings.Contains(content, "[source:") {
		return content, 0
	}
	labels := map[string]string{}
	for _, entity := range entities {
		id := strings.TrimSpace(entity.EntityID)
		if id == "" {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(entity.Label, entity.Kind, "source"))
		if label == "" {
			label = "source"
		}
		labels[id] = label
	}
	if len(labels) == 0 {
		return content, 0
	}
	count := 0
	normalized := bareTextureSourceRefRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := bareTextureSourceRefRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		id := strings.TrimSpace(parts[1])
		label := labels[id]
		if label == "" {
			return match
		}
		count++
		return "[" + label + "](source:" + id + ")"
	})
	return normalized, count
}

var wireArticleSourceServiceProseRE = regexp.MustCompile(`Source Service item (srcitem_[A-Za-z0-9_-]+)`)

func normalizeWireArticleSourceServiceProse(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int, []textureSourceEntity) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0, nil
	}
	if !wireArticleSourceServiceProseRE.MatchString(content) && !textureRawSourceServiceItemIDRE.MatchString(content) {
		return content, 0, nil
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeTextureSourceEntities(meta["source_entities"])
	labels := map[string]string{}
	entityByItem := map[string]textureSourceEntity{}
	for _, entity := range entities {
		id := strings.TrimSpace(entity.EntityID)
		itemID := strings.TrimSpace(entity.Target.ItemID)
		if id == "" {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(entity.Label, entity.Kind, "source"))
		if label == "" {
			label = "source"
		}
		labels[id] = label
		if itemID != "" {
			entityByItem[itemID] = entity
		}
	}
	count := 0
	normalized := wireArticleSourceServiceProseRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := wireArticleSourceServiceProseRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		itemID := strings.TrimSpace(parts[1])
		entity, ok := entityByItem[itemID]
		if !ok {
			entity = sourceServiceItemRefToSourceEntity(itemID, content)
			entities, _ = mergeTextureSourceEntities(entities, []textureSourceEntity{entity})
			entityByItem[itemID] = entity
		}
		entityID := strings.TrimSpace(entity.EntityID)
		label := strings.TrimSpace(firstNonEmpty(entity.Label, labels[entityID], "source"))
		if entityID == "" || label == "" {
			return match
		}
		labels[entityID] = label
		count++
		return "[" + label + "](source:" + entityID + ")"
	})
	return normalized, count, entities
}
