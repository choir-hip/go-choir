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
type Ref struct {
	Doc     string `json:"doc"`
	Version string `json:"version"`
	CID     string `json:"cid"`
	Hash    string `json:"hash"`
	Quote   string `json:"quote"`
}
