package cycle

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/wire/processorkey"
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
		key := processorkey.SourceProcessorKey(item)
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
			sourceItemIDs := processorkey.OrderedSourceItemIDs(batch)
			req := ProcessorRequest{
				RequestID:         processorkey.StableRequestID("processor", cycleID, key, fmt.Sprintf("%d", batchIndex)),
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
				Prompt:            processorkey.ProcessorHandoffPrompt(key, batch),
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
