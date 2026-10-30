package types

import (
	"crypto/sha256"
	"encoding/hex"
)

// Support judgement (Cut 2 2a (v); support-judgement-to-jev-design-2026-10-14).
// Code checks that a citation's quote is the cited block's own words; Jev,
// the decision model, judges whether that block supports the sentence that
// cites it, off any desk's turn, after the Texture commit that listed the
// pair. Both are
// keyed by the hash pair, so an unchanged citation is checked and judged
// once, and an edited sentence or a re-captured block is a new pair.
// Checks and judgements never edit text or mint a version: a pass attaches
// to the revision that listed it, and a failure is a record to the Texture,
// which corrects with Write.

// Support verdicts.
const (
	SupportSupported   = "supported"
	SupportUnsupported = "unsupported"
)

// CitationPair is one citation of a committed revision: the citing span
// (its top-level block id, and the hash and text of the words the citation
// closes, without citations), the block's words as context, and the cited
// Ref. BlockHash keeps its name but hashes the span, not the block
// (docs/problems/support-pair-judges-whole-paragraph-against-one-quote-
// 2026-10-19.md); pairs listed before then hash their whole block, which
// was their text. Matcher is the quote matcher's version, part of Key; Tier
// is the tier its quote matched at (exact or tolerant).
type CitationPair struct {
	Key        string `json:"key"`
	DocID      string `json:"doc_id"`
	RevisionID string `json:"revision_id"`
	BlockID    string `json:"block_id"`
	BlockHash  string `json:"block_hash"`
	Text       string `json:"text"`
	Context    string `json:"context,omitempty"`
	Cited      Ref    `json:"cited"`
	Matcher    string `json:"matcher"`
	Tier       string `json:"tier,omitempty"`
}

// SupportPairKey keys a pair by (citing block hash, cited block hash, quote
// hash, matcher version).
func SupportPairKey(blockHash, citedHash, quote, matcher string) string {
	quoteSum := sha256.Sum256([]byte(quote))
	sum := sha256.Sum256([]byte(blockHash + "\x00" + citedHash + "\x00" + hex.EncodeToString(quoteSum[:]) + "\x00" + matcher))
	return "sp-" + hex.EncodeToString(sum[:16])
}

// SupportVerdict is one judgement of one pair. Judge names who judged:
// "jev:<model>" for the decision model ("management" on verdicts recorded
// before support judgement left management). PSupportedBP is the decision
// model's probability of support in basis points. Pair is the listed pair
// the store recorded; RecordID names the failure record a failing verdict
// sent the Texture. Memo marks a verdict read from the memo instead of
// judged.
type SupportVerdict struct {
	Key          string        `json:"key"`
	Verdict      string        `json:"verdict"`
	Why          string        `json:"why,omitempty"`
	Judge        string        `json:"judge"`
	PSupportedBP int64         `json:"p_supported_bp,omitempty"`
	Pair         *CitationPair `json:"pair,omitempty"`
	RecordID     string        `json:"record_id,omitempty"`
	Memo         bool          `json:"memo,omitempty"`
}

// CitationCheck is the memo of one pair's quote check, written when a
// Texture commit first lists the pair. Its presence means the pair is
// listed; a pair with a check and no verdict is pending. A citation of a
// source with no capture is never checked here, so it is recomputed once a
// capture lands (pre-flight review A1).
type CitationCheck struct {
	Pair     CitationPair `json:"pair"`
	Outcome  string       `json:"outcome"`
	Tier     string       `json:"tier,omitempty"`
	ListedBy string       `json:"listed_by"`
	// SelfP is the Texture's own probability, stated with the Write that
	// listed the pair, that the judge finds the passage supports the
	// sentence; ModelID the model that wrote it
	// (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md). Absent
	// when the Write stated none.
	SelfP   *float64 `json:"self_p,omitempty"`
	ModelID string   `json:"model_id,omitempty"`
}

// CommitmentSupport is the typed body of the support records: the failing
// verdicts the Jev job sends a Texture (Failures). Pairs is the list a
// Texture commit sent management before support judgement left it; such a
// record may still sit unread in management's inbox, where it is shown
// closed and its pairs are handed to Jev.
type CommitmentSupport struct {
	Pairs    []CitationPair   `json:"pairs,omitempty"`
	Failures []SupportVerdict `json:"failures,omitempty"`
}

// SupportSystemAgentID is the support judge's identity: the runtime's Jev
// job recording verdicts as a system act, and the sender of the failure
// records it sends a Texture.
const SupportSystemAgentID = "system:support"
