package runtime

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// inlineSourceRefRE matches a native Texture inline citation of the form
// [label](source:ENTITY_ID). The entity id grammar mirrors bareTextureSourceRefRE
// so the validator and the normalizers agree on what a citation target looks like.
var inlineSourceRefRE = regexp.MustCompile(`\[[^\]]*\]\(source:([A-Za-z0-9_.:-]{1,160})\)`)

// citationValidationReason enumerates the deterministic ways a model-inlined
// citation can fail validation against the runtime-collated source list.
type citationValidationReason string

const (
	// citationUnknownSource: the cited entity id is not in the collated source
	// list (researcher findings + deterministic media ingestion). The model
	// invented or mistyped a source handle.
	citationUnknownSource citationValidationReason = "unknown_source"
	// citationQuoteNotInSource: the citation targets a text-bodied source via a
	// text_quote selector, but the quoted text does not appear in the retrieved
	// source body. This is the key grounding invariant.
	citationQuoteNotInSource citationValidationReason = "quote_not_in_source"
	// citationMissingSourceBody: the citation targets a text-bodied source with a
	// text_quote selector, but no source body was retrieved to validate against.
	citationMissingSourceBody citationValidationReason = "missing_source_body"
)

// citationValidationIssue is one deterministic citation failure, suitable for
// surfacing back to the authoring model as a tool error so it can retry.
type citationValidationIssue struct {
	EntityID string                   `json:"entity_id"`
	Quote    string                   `json:"quote,omitempty"`
	Reason   citationValidationReason `json:"reason"`
}

// extractInlineCitationEntityIDs returns the ordered, de-duplicated set of entity
// ids referenced by [label](source:ID) citations in body.
func extractInlineCitationEntityIDs(body string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, match := range inlineSourceRefRE.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		id := strings.TrimSpace(match[1])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// validateTextureCitations deterministically checks every inline [..](source:ID)
// citation in body:
//
//   - the entity id must resolve to a collated source entity; and
//   - for text-bodied sources whose selector is a text_quote, the quote must
//     verifiably appear in the retrieved source body (sourceBodies[entityID]).
//
// Media / whole_resource citations require only id+selector existence (no quote
// match). sourceBodies is keyed by entity id and carries the retrieved body for
// the text-bodied sources the caller resolved; absence of a body for a
// text_quote selector is itself a failure (we do not silently pass unverifiable
// quotes). The result is the ordered list of issues; empty means the body's
// citations are fully grounded.
func validateTextureCitations(body string, entities []textureSourceEntity, sourceBodies map[string]string) []citationValidationIssue {
	byID := make(map[string]textureSourceEntity, len(entities))
	for _, entity := range entities {
		if id := strings.TrimSpace(entity.EntityID); id != "" {
			byID[id] = entity
		}
	}
	issues := []citationValidationIssue{}
	for _, id := range extractInlineCitationEntityIDs(body) {
		entity, ok := byID[id]
		if !ok {
			issues = append(issues, citationValidationIssue{EntityID: id, Reason: citationUnknownSource})
			continue
		}
		quote, requiresQuote := textQuoteSelector(entity)
		if !requiresQuote {
			continue
		}
		sourceBody, hasBody := sourceBodies[id]
		if !hasBody || strings.TrimSpace(sourceBody) == "" {
			issues = append(issues, citationValidationIssue{EntityID: id, Quote: quote, Reason: citationMissingSourceBody})
			continue
		}
		if !quoteAppearsInSource(quote, sourceBody) {
			issues = append(issues, citationValidationIssue{EntityID: id, Quote: quote, Reason: citationQuoteNotInSource})
		}
	}
	return issues
}

// textQuoteSelector reports the entity's first text_quote selector text, and
// whether the entity carries one at all. Only text_quote selectors demand a
// quote-match against the source body; whole_resource / media selectors do not.
func textQuoteSelector(entity textureSourceEntity) (string, bool) {
	for _, selector := range entity.Selectors {
		if !strings.EqualFold(strings.TrimSpace(selector.SelectorKind), "text_quote") {
			continue
		}
		quote := strings.TrimSpace(selector.TextQuote)
		if quote == "" {
			continue
		}
		return quote, true
	}
	return "", false
}

// quoteAppearsInSource reports whether quote verifiably appears in sourceBody,
// comparing on collapsed whitespace and case so that benign reflowing
// (line wraps, double spaces, casing) does not produce false failures while a
// genuinely absent quote still fails.
func quoteAppearsInSource(quote, sourceBody string) bool {
	needle := collapseForQuoteMatch(quote)
	if needle == "" {
		return true
	}
	return strings.Contains(collapseForQuoteMatch(sourceBody), needle)
}

var quoteWhitespaceRE = regexp.MustCompile(`\s+`)

func collapseForQuoteMatch(s string) string {
	return strings.ToLower(strings.TrimSpace(quoteWhitespaceRE.ReplaceAllString(s, " ")))
}

// formatCitationValidationError renders citation issues into a deterministic,
// model-facing tool error message instructing the model to fix or drop the
// offending citations and retry.
func formatCitationValidationError(issues []citationValidationIssue) string {
	if len(issues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("citation validation failed: every inline [label](source:ENTITY_ID) citation must resolve to a collated source, and text_quote citations must quote text that appears in the source body. Fix or remove these citations and retry:")
	for _, issue := range issues {
		b.WriteString("\n- source:")
		b.WriteString(issue.EntityID)
		b.WriteString(" (")
		b.WriteString(string(issue.Reason))
		b.WriteString(")")
		if strings.TrimSpace(issue.Quote) != "" {
			b.WriteString(" quote=")
			b.WriteString(quoteForError(issue.Quote))
		}
	}
	return b.String()
}

// collateCitationSourceBodies retrieves the source body for each cited entity
// that carries a text_quote selector, so validateTextureCitations can verify the
// quote against ground truth. Bodies come from owner-scoped content items
// (Target.ContentID) or, for source-service projections, the resolved source
// item body. Entities that are not cited, lack a text_quote selector, or whose
// body cannot be retrieved are simply absent from the result (the validator then
// fails them as missing_source_body rather than silently passing).
func (rt *Runtime) collateCitationSourceBodies(ctx context.Context, ownerID string, citedIDs []string, entities []textureSourceEntity) map[string]string {
	bodies := map[string]string{}
	if rt == nil || len(citedIDs) == 0 {
		return bodies
	}
	byID := make(map[string]textureSourceEntity, len(entities))
	for _, entity := range entities {
		if id := strings.TrimSpace(entity.EntityID); id != "" {
			byID[id] = entity
		}
	}
	var resolveClient sourceItemResolveClient
	resolveClientChecked := false
	for _, id := range citedIDs {
		entity, ok := byID[id]
		if !ok {
			continue
		}
		if _, requiresQuote := textQuoteSelector(entity); !requiresQuote {
			continue
		}
		if cid := strings.TrimSpace(entity.Target.ContentID); cid != "" && rt.store != nil {
			if item, err := rt.Store().GetContentItem(ctx, ownerID, cid); err == nil {
				if body := strings.TrimSpace(item.TextContent); body != "" {
					bodies[id] = body
					continue
				}
			}
		}
		itemID := strings.TrimSpace(entity.Target.ItemID)
		if itemID == "" || !strings.EqualFold(strings.TrimSpace(entity.Target.TargetKind), "source_service_item") {
			continue
		}
		if !resolveClientChecked {
			resolveClient, _ = newSourceSearchClientFromEnv().(sourceItemResolveClient)
			resolveClientChecked = true
		}
		if resolveClient == nil {
			continue
		}
		resolveCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		item, err := resolveClient.ResolveSourceItem(resolveCtx, itemID)
		cancel()
		if err == nil && item != nil {
			if body := strings.TrimSpace(item.Body); body != "" {
				bodies[id] = body
			}
		}
	}
	return bodies
}

func quoteForError(quote string) string {
	quote = strings.TrimSpace(quote)
	const max = 160
	if len(quote) > max {
		quote = quote[:max] + "…"
	}
	return "\"" + quote + "\""
}
