import Foundation

public extension RetrievalIndex {
    /// The keyword list for hybrid ranking: BM25 over the whole index for these words, best first, with no relevance floor and no
    /// character budget (those belong to `retrieve`). The id is the slot's id (a document's, or a passage's `<doc id>#<n>`).
    func bm25Ranking(_ words: String, limit: Int = 50) -> [(id: String, score: Double)] {
        let (listener, _, _) = Self.terms(for: Query(words: words))
        guard !listener.isEmpty else { return [] }
        let r = scores(listener.map { ($0, 1.0) }, matchTerms: Set(listener), boost: { _ in 1 })
        return r.prefix(limit).map { (item($0.slot).id, $0.score) }
    }
}
