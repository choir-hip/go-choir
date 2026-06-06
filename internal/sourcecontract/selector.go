package sourcecontract

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
	normalized := normalizeToken(value)
	if normalized == "" {
		return SelectorKindWholeResource
	}
	if canonical := canonicalFromSchema(embeddedSourceContractSchema.SelectorKinds, value); canonical != "" {
		return canonical
	}
	return normalized
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
