package types

// Ref is the one reference shape for a citation or a transclusion: one block
// of one version of a document (a Texture or a capture), with the quote the
// citing text relies on. It replaces both selector shapes for new citations
// (texturedoc.SourceSelector and CoagentPacketSourceSelector); existing
// citations keep their selectors.
//
//   - Doc names the cited document.
//   - Version names the revision or capture of it that was cited.
//   - CID is the block's content id from internal/normtext (cid, or cid.k
//     for the k-th duplicate in the document).
//   - Hash is the cited block's content hash, the key that memoizes quote
//     matches and support judgements for an unchanged pair.
//   - Quote is the quoted text; internal/quotematch checks it against the
//     block, and the rendered quote is the source's own span.
//   - Span, when present, is where Quote sits in the block, as the quote
//     check found it, so a reader marks the exact words without matching
//     again. Refs written before it have none (H-OG3,
//     docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
type Ref struct {
	Doc     string   `json:"doc"`
	Version string   `json:"version"`
	CID     string   `json:"cid"`
	Hash    string   `json:"hash"`
	Quote   string   `json:"quote"`
	Span    *RefSpan `json:"span,omitempty"`
}

// RefSpan locates a Ref's quote in its block (Ref.CID) of the text whose
// hash is Ref.Hash. Offsets count Unicode code points, as the platform's
// TextPositionSelector does, half-open [Start, End). Start and End cover
// the whole quote; an ellipsis quote also lists one range per segment.
// TextHash is "sha256:" + hex of the UTF-8 bytes of the block text from
// Start to End, so a reader can check the offsets still point at the same
// words.
type RefSpan struct {
	Start    int        `json:"start"`
	End      int        `json:"end"`
	Segments []RefRange `json:"segments,omitempty"`
	TextHash string     `json:"text_hash"`
}

// RefRange is one ellipsis segment of a RefSpan, in code points.
type RefRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
