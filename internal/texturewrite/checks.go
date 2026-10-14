package texturewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
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

// Checks lists every citation of doc in document order with its quote
// check, and the support pairs of the captured ones, one per key. A
// transclusion with no quote ({{embed}} of a whole capture) cites no
// passage, so it is neither checked nor judged.
func Checks(doc texturedoc.StructuredTextureDoc, docID, revisionID string) ([]QuoteCheck, []types.CitationPair) {
	var checks []QuoteCheck
	var pairs []types.CitationPair
	seen := map[string]bool{}
	for _, block := range DocBlocks(doc) {
		text := CitingText(block.Node)
		blockHash := TextHash(text)
		for _, ref := range sourceRefs(block.Node) {
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
				check.Pair = PairKey(blockHash, cited.Hash, cited.Quote)
				if !seen[check.Pair] {
					seen[check.Pair] = true
					pairs = append(pairs, types.CitationPair{
						Key: check.Pair, DocID: docID, RevisionID: revisionID, BlockID: block.ID, BlockHash: blockHash,
						Text: text, Cited: cited, Matcher: quotematch.Version, Tier: string(check.Tier),
					})
				}
			}
			checks = append(checks, check)
		}
	}
	return checks, pairs
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
