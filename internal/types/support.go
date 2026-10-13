package types

import (
	"crypto/sha256"
	"encoding/hex"
)

// Support judgement (Cut 2 2a (v)). Code checks that a citation's quote is
// the cited block's own words; management (or Jev, through management)
// judges whether that block supports the sentence that cites it. Both are
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

// CitationPair is one citation of a committed revision: the citing block
// (top-level block id, the hash and text of its words without citations)
// and the cited Ref. Matcher is the quote matcher's version, part of Key;
// Tier is the tier its quote matched at (exact or tolerant). Verdict and
// Why are empty in a list; management sets them on a pair it judges itself
// before passing it to choir.Judge.
type CitationPair struct {
	Key        string `json:"key"`
	DocID      string `json:"doc_id"`
	RevisionID string `json:"revision_id"`
	BlockID    string `json:"block_id"`
	BlockHash  string `json:"block_hash"`
	Text       string `json:"text"`
	Cited      Ref    `json:"cited"`
	Matcher    string `json:"matcher"`
	Tier       string `json:"tier,omitempty"`
	Verdict    string `json:"verdict,omitempty"`
	Why        string `json:"why,omitempty"`
}

// SupportPairKey keys a pair by (citing block hash, cited block hash, quote
// hash, matcher version).
func SupportPairKey(blockHash, citedHash, quote, matcher string) string {
	quoteSum := sha256.Sum256([]byte(quote))
	sum := sha256.Sum256([]byte(blockHash + "\x00" + citedHash + "\x00" + hex.EncodeToString(quoteSum[:]) + "\x00" + matcher))
	return "sp-" + hex.EncodeToString(sum[:16])
}

// SupportVerdict is one judgement of one pair. Judge names who judged:
// "management", or "jev:<model>" for the decision model management called.
// PSupportedBP is the decision model's probability of support in basis
// points. Pair is the listed pair the store recorded; RecordID names the
// failure record a failing verdict sent the Texture. Memo marks a verdict
// choir.Judge returned from the memo instead of judging.
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
}

// CommitmentSupport is the typed body of the two support records: the
// system list a Texture commit sends management (Pairs), and management's
// failing verdicts it sends the Texture (Failures).
type CommitmentSupport struct {
	Pairs    []CitationPair   `json:"pairs,omitempty"`
	Failures []SupportVerdict `json:"failures,omitempty"`
}

// SupportPairsRecordID is the list record a revision's commit sends.
func SupportPairsRecordID(revisionID string) string {
	return "support-pairs:" + revisionID
}

// SupportSystemAgentID is the provenance of a pairs list: the store
// speaking for the commit.
const SupportSystemAgentID = "system:support"
