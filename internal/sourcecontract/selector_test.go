package sourcecontract

import "testing"

func TestNormalizeSelectorKind(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "missing", raw: "", want: SelectorKindWholeResource},
		{name: "whole resource", raw: "whole resource", want: SelectorKindWholeResource},
		{name: "text quote space", raw: "text quote", want: SelectorKindTextQuote},
		{name: "table range hyphen", raw: "table-range", want: SelectorKindTableRange},
		{name: "page range space", raw: "page range", want: SelectorKindPageRange},
		{name: "timestamp", raw: "timestamp", want: SelectorKindTimestampRange},
		{name: "table cell", raw: "table cell", want: SelectorKindTableCell},
		{name: "image region", raw: "image region", want: SelectorKindImageRegion},
		{name: "image area alias", raw: "image-area", want: SelectorKindImageRegion},
		{name: "custom", raw: "custom selector", want: "custom_selector"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSelectorKind(tc.raw); got != tc.want {
				t.Fatalf("NormalizeSelectorKind(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeSelectorPreservesPayload(t *testing.T) {
	selector := map[string]any{
		"selector_kind": "table-range",
		"table_id":      "appendix-a",
		"start_row":     3,
		"end_row":       7,
	}
	got := NormalizeSelector(selector)
	if got["selector_kind"] != SelectorKindTableRange {
		t.Fatalf("selector kind = %#v, want %q", got["selector_kind"], SelectorKindTableRange)
	}
	if got["table_id"] != "appendix-a" || got["start_row"] != 3 || got["end_row"] != 7 {
		t.Fatalf("selector payload not preserved: %#v", got)
	}
	if selector["selector_kind"] != "table-range" {
		t.Fatalf("NormalizeSelector mutated input: %#v", selector)
	}
}
