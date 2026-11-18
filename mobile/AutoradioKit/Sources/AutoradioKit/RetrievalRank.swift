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

public extension RetrievalIndex {
    /// BM25 for a question: best first, ids, the floor of `candidates` applied (the keyword list of a fusion).
    func keywordIDs(_ q: RetrievalQuery, limit: Int) -> [RetrievalItem] { candidates(q, limit: limit) }

    /// The passages that hold a RARE word of the question (a name, a number, a place): a word that fewer than `max(2, 1% of the items)` items
    /// contain. Best BM25 first. No floor: one rare word is enough, the vectors do not decide this list.
    func exactTermHits(_ q: RetrievalQuery, limit: Int = 20) -> [String] {
        let (listener, _, _) = Self.terms(for: q)
        let n = max(count, 1), maxDF = max(2, n / 100)
        var best: [String: Double] = [:]
        for t in listener {
            let r = scores([(t, 1.0)], matchTerms: [t], boost: { _ in 1 })
            guard !r.isEmpty, r.count <= maxDF else { continue }
            for h in r { best[item(h.slot).id] = max(best[item(h.slot).id] ?? 0, h.score) }
        }
        return best.sorted { $0.value != $1.value ? $0.value > $1.value : $0.key < $1.key }.prefix(limit).map(\.key)
    }
}
