import Foundation

// Retrieval with vectors: the three ways of combining the keyword list and the vector list that were measured against each other
// (docs/reference/EMBEDDINGGEMMA.md section 0.6), one function so a test and the Mac trial can run any of them. The app uses ONE (`HybridRank.shipped`);
// there is no setting.
//
// ONE FLOOR PER RANKING SOURCE (none prunes twice): the keyword list is floored by `RetrievalIndex.candidates` (two matched terms, at least half of the
// question's terms, 20 percent of the best score among those: the rag-fix version); the vector list by the cosine floor, and only that; the exact-term
// list (a rare word of the question) by nothing, one rare word is the evidence. The fused list is a union and an order, it adds no floor of its own.
//
// WAYS THIS CAN FAIL (each has a test in HybridRetrievalTests):
//  1. No model, no vectors, or a partly embedded backlog: BM25 alone, and nothing says so.
//  2. A long item embedded as windows: the windows collapse to the item.
//  3. A question the vectors rank low but names something rare: the exact-term list keeps it.
//  4. A question nothing matches: nothing comes back (cosine floor, and BM25's own floor).

public enum FusionStrategy: String, CaseIterable, Sendable {
    /// (a) the vector order only.
    case vectorsOnly
    /// (b) reciprocal rank fusion of the keyword and vector lists, k = 60.
    case rrf
    /// (c) the vector order, and every passage that holds a rare word of the question is kept in the list (at most half of it).
    case vectorsWithExactTerms
}

public enum HybridRank {
    /// The strategy the app runs: chosen by the numbers in the reference page, section 0.6.
    public static let shipped: FusionStrategy = .vectorsWithExactTerms

    /// Ids best first, at most `limit`. With no vectors, the keyword list as it is.
    public static func fuse(_ s: FusionStrategy, vectors: [String], bm25: [String], exact: [String], limit: Int) -> [String] {
        guard !vectors.isEmpty else { return Array(bm25.prefix(limit)) }
        switch s {
        case .vectorsOnly:
            return Array(vectors.prefix(limit))
        case .rrf:
            return Array(RankFusion.rrf([bm25, vectors]).prefix(limit))
        case .vectorsWithExactTerms:
            let inVectors = Set(vectors.prefix(limit))
            let missing = Array(exact.filter { !inVectors.contains($0) }.prefix(limit / 2))
            var out = Array(vectors.prefix(max(0, limit - missing.count)))
            let have = Set(out)
            out += missing.filter { !have.contains($0) }
            // an exact hit the vectors list holds below the cut is already in `missing`; one already in the cut stays where it is
            return Array(out.prefix(limit))
        }
    }
}

/// The retriever the model's context is built from once vectors exist. Same contract as the BM25 retriever (`Retriever`): the shared
/// `retrieve` fills the context; this decides which items and in which order.
public final class FusedRetriever: Retriever, @unchecked Sendable {
    public let index: RetrievalIndex
    public let vectors: VectorStore
    private let embedQuery: @Sendable (String) -> [Float]?
    private let coverage: @Sendable () -> Double
    /// Best cosine below this is not a match. Set from the unanswerable questions of the trial (section 0.6).
    public var cosineFloor: Float
    /// The share of items that must hold a vector before the vectors are used at all (until then: BM25 alone).
    public static let minCoverage = 0.9
    public static let pool = 50
    public var say: @Sendable (String) -> Void = { fputs($0 + "\n", stderr) }
    private let lock = NSLock()
    private var line = ""
    public var lastLine: String { lock.lock(); defer { lock.unlock() }; return line }

    public init(index: RetrievalIndex, vectors: VectorStore, embedQuery: @escaping @Sendable (String) -> [Float]?, coverage: @escaping @Sendable () -> Double,
                cosineFloor: Float = KnownModels.embeddingCosineFloor) {
        self.index = index; self.vectors = vectors; self.embedQuery = embedQuery; self.coverage = coverage; self.cosineFloor = cosineFloor
    }
    public var usesVectors: Bool { vectors.count > 0 && coverage() >= Self.minCoverage }

    public var indexedCount: Int { index.indexedCount }
    public func sources() -> [String: Int] { index.sources() }
    public func idf(_ term: String) -> Double { index.idf(term) }

    /// The item id a vector's id stands for (a long item is embedded as windows `<id>~w<n>`).
    public static func itemID(_ vectorID: String) -> String {
        guard let r = vectorID.range(of: "~w", options: .backwards), vectorID[r.upperBound...].allSatisfy(\.isNumber), !vectorID[r.upperBound...].isEmpty else { return vectorID }
        return String(vectorID[..<r.lowerBound])
    }

    public func candidates(_ q: RetrievalQuery, limit: Int) -> [RetrievalItem] {
        let t0 = DispatchTime.now().uptimeNanoseconds
        func ms() -> Int { Int(Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6) }
        let bm = index.candidates(q, limit: Self.pool)
        guard usesVectors else {
            let out = Array(bm.prefix(limit))
            record("AUTORADIO retrieve retrieved=\(out.count) vec=0 bm25=\(bm.count) ms=\(ms())"); return out
        }
        let (listener, onAir, usedOnAir) = RetrievalIndex.terms(for: q)
        _ = listener; _ = onAir
        let text = usedOnAir && !q.onAirTitle.isEmpty ? q.words + " " + q.onAirTitle : q.words
        var vecIDs: [String] = []
        if let qv = embedQuery(text) {
            var seen = Set<String>()
            for h in vectors.search(qv, k: Self.pool) where h.score >= cosineFloor {
                let id = Self.itemID(h.id)
                if seen.insert(id).inserted { vecIDs.append(id) }
            }
        }
        let exact = index.exactTermHits(q, limit: limit)
        let ids = HybridRank.fuse(HybridRank.shipped, vectors: vecIDs, bm25: bm.map(\.id), exact: exact, limit: limit)
        let byID = Dictionary(bm.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        var out: [RetrievalItem] = []
        for (rank, id) in ids.enumerated() {
            guard var it = byID[id] ?? index.item(id: id) else { continue }
            it.score = 1 / Double(rank + 1)
            out.append(it)
        }
        record("AUTORADIO retrieve retrieved=\(out.count) vec=\(vecIDs.count) bm25=\(bm.count) ms=\(ms())")
        return out
    }
    private func record(_ s: String) { lock.lock(); line = s; lock.unlock(); say(s) }
}
