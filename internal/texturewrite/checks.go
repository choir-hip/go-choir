package texturewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Citation checks at commit (Cut 2 2a (v)). Write matched every new cite's
// quote against its capture block and refused one that did not match, so a
// committed citation with a Ref is matched, exactly or tolerantly, and its
// Ref is the source's own span; a carried citation keeps its Ref and tier.
// A citation with no Ref names a source no one captured: it is not
// captured, has no pair and is never memoized, so it is checked again once
// a Write cites a capture of it (pre-flight review A1).

// ChecksMetadataKey is the revision metadata key listing each citation's
// quote check, where the reader and the version overlay read it (2b).
const ChecksMetadataKey = "texture_citation_checks"

// QuoteCheck is one citation's quote check: the citation node, its
// top-level block, the outcome and tier, and its support pair key (empty
// when not captured).
type QuoteCheck struct {
	RefID   string             `json:"ref_id"`
	BlockID string             `json:"block_id"`
	Outcome quotematch.Outcome `json:"outcome"`
	Tier    quotematch.Tier    `json:"tier,omitempty"`
	Pair    string             `json:"pair,omitempty"`
}

// TextHash is the content hash of a block's text, the form a Ref's Hash
// takes.
func TextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// PairKey keys a support pair under this matcher.
func PairKey(blockHash, citedHash, quote string) string {
	return types.SupportPairKey(blockHash, citedHash, quote, quotematch.Version)
}

// CitingText is a block's words without its citations, the citing text a
// pair hashes and Jev judges: whitespace collapsed, and no space
// left before closing punctuation where a citation token stood, so adding
// or removing a citation beside a sentence keeps its hash.
func CitingText(n texturedoc.Node) string {
	text := strings.Join(strings.Fields(plainText(n)), " ")
	return spaceBeforeCloseRE.ReplaceAllString(text, "$1")
}

var spaceBeforeCloseRE = regexp.MustCompile(` ([,.;:!?)\]])`)

// citingSpans is the span each of a block's citations closes, in
// sourceRefs order: the block's words from the previous citation (or the
// start of the block, or of its list item) up to the citation, with the
// closing punctuation right after it. Citations side by side close the
// same span, so a sentence two sources both support is judged against
// each quote alone. A pair judged its whole block before
// (docs/problems/support-pair-judges-whole-paragraph-against-one-quote-
// 2026-10-19.md), so a paragraph drawing on two outlets failed against
// either quote.
func citingSpans(n texturedoc.Node) []string {
	spans, _ := citingSpanGroups(n)
	return spans
}

// citingSpanGroups is citingSpans with the group each citation belongs to:
// citations that close one span (side by side) share a group number. The
// support judge asks one question per group, with every cited passage of
// the group as evidence (docs/problems/unsupported-cites-duplicates-and-
// churn-2026-11-10.md, fix A).
func citingSpanGroups(n texturedoc.Node) ([]string, []int) {
	var spans []string
	var groups []int
	next := 0
	var buf strings.Builder
	pending := 0
	settle := func() {
		span := strings.Join(strings.Fields(buf.String()), " ")
		span = spaceBeforeCloseRE.ReplaceAllString(span, "$1")
		if pending > 0 {
			next++
		}
		for ; pending > 0; pending-- {
			spans = append(spans, span)
			groups = append(groups, next)
		}
		buf.Reset()
	}
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		switch n.Type {
		case "source_ref":
			pending++
		case "text":
			text := n.Text
			if pending > 0 {
				if strings.TrimSpace(text) == "" {
					return
				}
				close := closingPunctRE.FindString(text)
				buf.WriteString(close)
				settle()
				text = text[len(close):]
			}
			buf.WriteString(text)
		case "hard_break":
			buf.WriteByte(' ')
		}
		for _, c := range n.Content {
			if isBlockType(c.Type) {
				settle()
			}
			walk(c)
		}
	}
	walk(n)
	settle()
	return spans, groups
}

var closingPunctRE = regexp.MustCompile(`^\s*[,.;:!?)\]"”’]+`)

// maxGroupCites bounds how many cites one question carries as evidence. A
// span cited more heavily than this is judged in groups of this size, in
// order, so a question's evidence stays bounded.
const maxGroupCites = 6

// refIdentity names a cited quote: the same capture, block and quote is one
// piece of evidence however many times a sentence cites it.
func refIdentity(r types.Ref) string {
	return r.Doc + "\x00" + r.CID + "\x00" + r.Hash + "\x00" + r.Quote
}

// GroupHash is the hash a grouped pair keys on instead of the span hash
// alone: the span and the identities of every quote its group cites, so a
// changed group (a cite added, dropped or swapped) is a new question and an
// unchanged one stays memoized.
func GroupHash(text string, group []types.Ref) string {
	ids := make([]string, 0, len(group))
	for _, r := range group {
		ids = append(ids, refIdentity(r))
	}
	sort.Strings(ids)
	return TextHash(text + "\x00group\x00" + strings.Join(ids, "\x01"))
}

// Checks lists every citation of doc in document order with its quote
// check, and the support pairs of the captured ones, one per key. A
// stored embed with no quote (a whole capture, from before {{embed}} was
// retired) cites no passage, so it is neither checked nor judged.
func Checks(doc texturedoc.StructuredTextureDoc, docID, revisionID string) ([]QuoteCheck, []types.CitationPair) {
	var checks []QuoteCheck
	var pairs []types.CitationPair
	seen := map[string]bool{}
	for _, block := range DocBlocks(doc) {
		context := CitingText(block.Node)
		spans, spanGroups := citingSpanGroups(block.Node)
		refs := sourceRefs(block.Node)
		// The checkable cites of each side-by-side group, deduplicated, in
		// order. A pair carries its group (chunked to maxGroupCites) as
		// Group when the group holds more than one quote.
		groupOf := map[int][]types.Ref{}
		for i, ref := range refs {
			if i >= len(spanGroups) {
				break
			}
			cited, ok := RefOf(ref)
			if !ok || cited.Quote == "" || cited.Hash == "" {
				continue
			}
			dup := false
			for _, have := range groupOf[spanGroups[i]] {
				if refIdentity(have) == refIdentity(cited) {
					dup = true
				}
			}
			if !dup {
				groupOf[spanGroups[i]] = append(groupOf[spanGroups[i]], cited)
			}
		}
		for i, ref := range refs {
			text := context
			if i < len(spans) && spans[i] != "" {
				text = spans[i]
			}
			spanHash := TextHash(text)
			check := QuoteCheck{RefID: nodeID(ref), BlockID: block.ID}
			cited, ok := RefOf(ref)
			switch {
			case !ok:
				check.Outcome = quotematch.NotCaptured
			case cited.Quote == "" || cited.Hash == "":
				continue
			default:
				check.Outcome = quotematch.Matched
				check.Tier = quotematch.Tier(stringAttr(ref, tierAttr))
				if check.Tier == "" {
					check.Tier = quotematch.TierExact
				}
				var group []types.Ref
				keyHash := spanHash
				if i < len(spanGroups) {
					group = chunkOf(groupOf[spanGroups[i]], cited)
				}
				if len(group) > 1 {
					keyHash = GroupHash(text, group)
				} else {
					group = nil
				}
				check.Pair = PairKey(keyHash, cited.Hash, cited.Quote)
				if !seen[check.Pair] {
					seen[check.Pair] = true
					pair := types.CitationPair{
						Key: check.Pair, DocID: docID, RevisionID: revisionID, BlockID: block.ID, BlockHash: keyHash,
						Text: text, Cited: cited, Matcher: quotematch.Version, Tier: string(check.Tier),
						Group: group,
					}
					if text != context {
						pair.Context = context
					}
					pairs = append(pairs, pair)
				}
			}
			checks = append(checks, check)
		}
	}
	return checks, pairs
}

// chunkOf is the run of at most maxGroupCites quotes of group that holds
// cited, chunking the group in order.
func chunkOf(group []types.Ref, cited types.Ref) []types.Ref {
	for start := 0; start < len(group); start += maxGroupCites {
		end := min(start+maxGroupCites, len(group))
		for _, r := range group[start:end] {
			if refIdentity(r) == refIdentity(cited) {
				return group[start:end]
			}
		}
	}
	return nil
}

// CitesSource reports whether rev cites at least one source: a citation
// in its body that carries a Write Ref (Write refuses a cite or embed of
// anything not captured) or names one of the revision's source entities (a
// citation made before Write). A revision that cites nothing has no
// evidence a reader could check.
func CitesSource(rev types.Revision) (bool, error) {
	doc, entities, err := Base(rev)
	if err != nil {
		return false, err
	}
	known := make(map[string]bool, len(entities))
	for _, e := range entities {
		known[e.SourceEntityID] = true
	}
	for _, ref := range sourceRefs(doc.Doc) {
		if _, ok := RefOf(ref); ok || known[stringAttr(ref, "source_entity_id")] {
			return true, nil
		}
	}
	return false, nil
}

// CitedCaptures returns the distinct capture ids a revision's body cites
// through Write Refs, in first-citation order. A citation by source entity
// alone (made before Write) names no capture, so it is not listed.
func CitedCaptures(rev types.Revision) ([]string, error) {
	doc, _, err := Base(rev)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, node := range sourceRefs(doc.Doc) {
		if ref, ok := RefOf(node); ok && !seen[ref.Doc] {
			seen[ref.Doc] = true
			out = append(out, ref.Doc)
		}
	}
	return out, nil
}

// ChecksFrom reads the citation checks a revision recorded.
func ChecksFrom(rev types.Revision) []QuoteCheck {
	var meta map[string]json.RawMessage
	if json.Unmarshal(rev.Metadata, &meta) != nil {
		return nil
	}
	var out []QuoteCheck
	if raw, ok := meta[ChecksMetadataKey]; ok {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}
