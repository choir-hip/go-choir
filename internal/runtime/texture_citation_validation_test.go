package runtime

import (
	"strings"
	"testing"
)

func citationEntity(id, selectorKind, quote string) textureSourceEntity {
	entity := textureSourceEntity{EntityID: id, Kind: "content_item"}
	selector := textureSourceEntitySelector{SelectorKind: selectorKind}
	if quote != "" {
		selector.TextQuote = quote
	}
	entity.Selectors = []textureSourceEntitySelector{selector}
	return entity
}

func reasonsByID(issues []citationValidationIssue) map[string]citationValidationReason {
	out := map[string]citationValidationReason{}
	for _, issue := range issues {
		out[issue.EntityID] = issue.Reason
	}
	return out
}

func TestExtractInlineCitationEntityIDs(t *testing.T) {
	body := "See [Acme report](source:src_aaaa) and [again](source:src_aaaa) plus [b](source:src_bbbb). Bare [source:src_cccc] does not count."
	ids := extractInlineCitationEntityIDs(body)
	if len(ids) != 2 {
		t.Fatalf("expected 2 unique citation ids, got %v", ids)
	}
	if ids[0] != "src_aaaa" || ids[1] != "src_bbbb" {
		t.Fatalf("unexpected ids/order: %v", ids)
	}
}

func TestValidateTextureCitations_UnknownSourceFails(t *testing.T) {
	body := "Claim [x](source:src_missing)."
	issues := validateTextureCitations(body, nil, nil)
	if got := reasonsByID(issues)["src_missing"]; got != citationUnknownSource {
		t.Fatalf("expected unknown_source, got %q (issues=%v)", got, issues)
	}
}

func TestValidateTextureCitations_WholeResourcePasses(t *testing.T) {
	body := "Claim [x](source:src_ok)."
	entities := []textureSourceEntity{citationEntity("src_ok", "whole_resource", "")}
	if issues := validateTextureCitations(body, entities, nil); len(issues) != 0 {
		t.Fatalf("expected whole_resource citation to pass, got %v", issues)
	}
}

func TestValidateTextureCitations_QuotePresentPasses(t *testing.T) {
	body := "As reported, [it surged](source:src_q)."
	entities := []textureSourceEntity{citationEntity("src_q", "text_quote", "revenue surged 40% in Q3")}
	bodies := map[string]string{"src_q": "The company said revenue surged 40% in Q3 versus last year."}
	if issues := validateTextureCitations(body, entities, bodies); len(issues) != 0 {
		t.Fatalf("expected present quote to pass, got %v", issues)
	}
}

func TestValidateTextureCitations_QuotePresentToleratesWhitespaceAndCase(t *testing.T) {
	body := "[x](source:src_q)."
	entities := []textureSourceEntity{citationEntity("src_q", "text_quote", "Revenue   surged\n40%")}
	bodies := map[string]string{"src_q": "revenue surged 40% in q3"}
	if issues := validateTextureCitations(body, entities, bodies); len(issues) != 0 {
		t.Fatalf("expected whitespace/case-insensitive match to pass, got %v", issues)
	}
}

func TestValidateTextureCitations_QuoteAbsentFails(t *testing.T) {
	body := "[x](source:src_q)."
	entities := []textureSourceEntity{citationEntity("src_q", "text_quote", "profits tripled overnight")}
	bodies := map[string]string{"src_q": "The company said revenue surged 40% in Q3."}
	if got := reasonsByID(validateTextureCitations(body, entities, bodies))["src_q"]; got != citationQuoteNotInSource {
		t.Fatalf("expected quote_not_in_source, got %q", got)
	}
}

func TestValidateTextureCitations_QuoteWithoutBodyFails(t *testing.T) {
	body := "[x](source:src_q)."
	entities := []textureSourceEntity{citationEntity("src_q", "text_quote", "anything")}
	if got := reasonsByID(validateTextureCitations(body, entities, nil))["src_q"]; got != citationMissingSourceBody {
		t.Fatalf("expected missing_source_body, got %q", got)
	}
}

func TestFormatCitationValidationError(t *testing.T) {
	issues := []citationValidationIssue{
		{EntityID: "src_missing", Reason: citationUnknownSource},
		{EntityID: "src_q", Quote: "profits tripled", Reason: citationQuoteNotInSource},
	}
	msg := formatCitationValidationError(issues)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	for _, want := range []string{"src_missing", "unknown_source", "src_q", "quote_not_in_source", "profits tripled"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q: %s", want, msg)
		}
	}
	if formatCitationValidationError(nil) != "" {
		t.Fatal("expected empty message for no issues")
	}
}
