package agentcore

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func firstNonEmptySourceItemIDs(values ...[]string) []string {
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			out = append(out, item)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func resolveWireProcessorSourceItemIDs(rec *types.RunRecord, requested []string, requireExplicitForMulti bool) ([]string, error) {
	available := wireProcessorSourceItemIDs(rec)
	if len(requested) == 0 {
		switch {
		case len(available) == 0:
			return nil, nil
		case len(available) == 1:
			return append([]string(nil), available...), nil
		case requireExplicitForMulti:
			return nil, fmt.Errorf("source_item_ids required when processor request contains %d source items", len(available))
		default:
			return nil, nil
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("source_item_ids were provided but the processor run has no source_item_ids to bind")
	}
	allowed := make(map[string]bool, len(available))
	for _, itemID := range available {
		allowed[itemID] = true
	}
	seen := make(map[string]bool, len(requested))
	out := make([]string, 0, len(requested))
	for _, itemID := range requested {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || seen[itemID] {
			continue
		}
		if !allowed[itemID] {
			return nil, fmt.Errorf("source_item_id %s is not part of this processor request", itemID)
		}
		seen[itemID] = true
		out = append(out, itemID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source_item_ids must not be empty")
	}
	return out, nil
}

func wireProcessorSourceItemIDs(rec *types.RunRecord) []string {
	if rec == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, itemID := range metadataStringSlice(rec.Metadata["source_item_ids"]) {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || seen[itemID] {
			continue
		}
		seen[itemID] = true
		out = append(out, itemID)
	}
	return out
}
