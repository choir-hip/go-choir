package sourcecontract

import "strings"

const (
	SelectorKindWholeResource     = "whole_resource"
	SelectorKindTextQuote         = "text_quote"
	SelectorKindTextPosition      = "text_position"
	SelectorKindParagraphHeading  = "paragraph_heading"
	SelectorKindByteRange         = "byte_range"
	SelectorKindPageRange         = "page_range"
	SelectorKindTimestampRange    = "timestamp_range"
	SelectorKindTranscriptSegment = "transcript_segment"
	SelectorKindTableRange        = "table_range"
	SelectorKindTableCell         = "table_cell"
	SelectorKindDataVintage       = "data_vintage"
	SelectorKindSelectorSet       = "selector_set"
)

func NormalizeSelectorKind(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "":
		return SelectorKindWholeResource
	case SelectorKindWholeResource, "whole", "resource", "whole_document", "whole_source":
		return SelectorKindWholeResource
	case SelectorKindTextQuote, "quote", "quoted_text":
		return SelectorKindTextQuote
	case SelectorKindTextPosition, "text_range", "char_range", "character_range":
		return SelectorKindTextPosition
	case SelectorKindParagraphHeading, "paragraph", "heading", "heading_range", "paragraph_range":
		return SelectorKindParagraphHeading
	case SelectorKindByteRange, "bytes":
		return SelectorKindByteRange
	case SelectorKindPageRange, "pages":
		return SelectorKindPageRange
	case SelectorKindTimestampRange, "timestamp", "time_range", "media_range":
		return SelectorKindTimestampRange
	case SelectorKindTranscriptSegment, "transcript", "segment", "transcript_segments":
		return SelectorKindTranscriptSegment
	case SelectorKindTableRange, "table", "table_rows", "row_range":
		return SelectorKindTableRange
	case SelectorKindTableCell, "cell", "table_cells":
		return SelectorKindTableCell
	case SelectorKindDataVintage, "vintage", "data_release_vintage":
		return SelectorKindDataVintage
	case SelectorKindSelectorSet, "selectors":
		return SelectorKindSelectorSet
	default:
		return normalized
	}
}

func NormalizeSelector(selector map[string]any) map[string]any {
	out := make(map[string]any, len(selector)+1)
	for key, value := range selector {
		out[key] = value
	}
	out["selector_kind"] = NormalizeSelectorKind(stringValue(out["selector_kind"]))
	return out
}

func NormalizeSelectors(selectors []map[string]any) []map[string]any {
	if len(selectors) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(selectors))
	for _, selector := range selectors {
		out = append(out, NormalizeSelector(selector))
	}
	return out
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
