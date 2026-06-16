package cycle

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

const maxProcessorBatchItems = 50

type IngestionHandoff struct {
	ProcessorRequests  []ProcessorRequest
	ReconcilerRequests []ReconcilerRequest
}

func BuildIngestionHandoff(cycleID string, items []sources.Item, events []IngestionEvent, now time.Time) IngestionHandoff {
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" || len(items) == 0 || len(events) == 0 {
		return IngestionHandoff{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	batches := map[string][]sources.Item{}
	for _, item := range items {
		key := sourceProcessorKey(item)
		batches[key] = append(batches[key], item)
	}
	keys := make([]string, 0, len(batches))
	for key := range batches {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := IngestionHandoff{}
	for _, key := range keys {
		itemsForKey := batches[key]
		for batchIndex, batch := range chunkSourceItems(itemsForKey, maxProcessorBatchItems) {
			sourceItemIDs := orderedSourceItemIDs(batch)
			req := ProcessorRequest{
				RequestID:         stableRequestID("processor", cycleID, key, fmt.Sprintf("%d", batchIndex)),
				CycleID:           cycleID,
				ProcessorKey:      key,
				Status:            "queued",
				SourceItemIDs:     sourceItemIDs,
				IngestionEventIDs: ingestionEventIDsForItems(events, sourceItemIDs),
				SourceCount:       len(batch),
				SourceTypes:       sortedItemStrings(batch, func(item sources.Item) string { return string(item.SourceType) }),
				Verticals:         sortedItemStrings(batch, func(item sources.Item) string { return strings.Join(item.Verticals, ",") }),
				Regions:           sortedItemStrings(batch, func(item sources.Item) string { return item.Region }),
				ContinuityRef:     "sourcecycled://processor/" + key + "/latest",
				Prompt:            processorHandoffPrompt(key, batch),
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if !ProcessorRequestEligibleForDispatch(req) {
				continue
			}
			out.ProcessorRequests = append(out.ProcessorRequests, req)
		}
	}

	return out
}

func sourceProcessorKey(item sources.Item) string {
	if item.SourceType == sources.SourceTypeGDELT {
		region := strings.TrimSpace(strings.ToLower(item.Region))
		if region == "" {
			region = "global"
		}
		return "processor:global_firehose:" + safeKeyPart(region) + ":gdelt"
	}
	vertical := "general"
	for _, candidate := range item.Verticals {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate != "" {
			vertical = safeKeyPart(candidate)
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
	return "processor:" + vertical + ":" + safeKeyPart(region) + ":" + safeKeyPart(sourceType)
}

func stableRequestID(kind, cycleID string, parts ...string) string {
	segments := append([]string{kind, cycleID}, parts...)
	return kind + "_" + sources.ContentHash(segments...)[:24]
}

func orderedSourceItemIDs(items []sources.Item) []string {
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

func chunkSourceItems(items []sources.Item, size int) [][]sources.Item {
	if size <= 0 || len(items) <= size {
		return [][]sources.Item{items}
	}
	out := [][]sources.Item{}
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[start:end])
	}
	return out
}

func sortedItemStrings(items []sources.Item, value func(sources.Item) string) []string {
	seen := map[string]bool{}
	for _, item := range items {
		for _, raw := range strings.Split(value(item), ",") {
			raw = strings.TrimSpace(raw)
			if raw != "" {
				seen[raw] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func processorHandoffPrompt(key string, items []sources.Item) string {
	return fmt.Sprintf("Processor %s: ingest %d SourceItems by handle, update live understanding, preserve unresolved questions/watch items, and spawn Texture agents when a story should be opened or revised.", key, len(items))
}

func safeKeyPart(value string) string {
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
