// Package processorkey provides the shared processor-key derivation logic used
// by both cycle (ingestion handoff) and runtime (universal wire dispatch). It
// lives in a leaf package to break the import cycle cycle -> provider -> runtime.
package processorkey

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

// SourceProcessorKey derives the processor routing key for a source item.
// GDELT items route to the global firehose processor by region; everything
// else routes by primary vertical:region:type.
func SourceProcessorKey(item sources.Item) string {
	if item.SourceType == sources.SourceTypeGDELT {
		region := strings.TrimSpace(strings.ToLower(item.Region))
		if region == "" {
			region = "global"
		}
		return "processor:global_firehose:" + SafeKeyPart(region) + ":gdelt"
	}
	vertical := "general"
	for _, candidate := range item.Verticals {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate != "" {
			vertical = SafeKeyPart(candidate)
			break
		}
	}
	region := strings.TrimSpace(strings.ToLower(item.Region))
	if region == "" {
		region = "global"
	}
	sourceType := strings.TrimSpace(strings.ToLower(string(item.SourceType)))
	if sourceType == "" {
		sourceType = "source"
	}
	return "processor:" + vertical + ":" + SafeKeyPart(region) + ":" + SafeKeyPart(sourceType)
}

// StableRequestID produces a content-hash-derived stable request identifier
// from the kind, cycle ID, and additional parts.
func StableRequestID(kind, cycleID string, parts ...string) string {
	segments := append([]string{kind, cycleID}, parts...)
	return kind + "_" + sources.ContentHash(segments...)[:24]
}

// SafeKeyPart normalizes a string into a safe key component: lowercase,
// alphanumeric plus _ - . , trimmed, with "unknown" as the fallback.
func SafeKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "unknown"
	}
	return out
}

// OrderedSourceItemIDs returns the sorted, de-duplicated list of source item
// IDs from the given items.
func OrderedSourceItemIDs(items []sources.Item) []string {
	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ProcessorHandoffPrompt builds the standard processor handoff prompt for a
// given processor key and batch of source items.
func ProcessorHandoffPrompt(key string, items []sources.Item) string {
	return fmt.Sprintf("Processor %s: ingest %d SourceItems by handle, update live understanding, preserve unresolved questions/watch items, and spawn Texture agents when a story should be opened or revised.", key, len(items))
}
