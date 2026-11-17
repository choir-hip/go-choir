import Foundation

// What the phone already holds, as one searchable body: the stories it has fetched (ReaderStore), the front list's headlines, the page in
// front, and library passages when the reader lane has them. The first answer to a question comes from here (SavedAnswerHolder).
//
// WAYS THIS CAN FAIL (written before the code; each has a test in SavedCorpusTests):
//  1. A source that is empty or not wired: the corpus is just smaller; nothing throws.
//  2. A story fetched after the corpus was last read: `refresh` runs before every search and indexes what is new or changed.
//  3. A story that left the cache: its passages leave the index.
//  4. The page in front has text the cache does not: the holder passes it in (`offer`), it is indexed under its address and preferred.
//  5. A library that is slow: it is asked once per search, after the local index answered, and its hits join the list (never block the
//     first sentence on it beyond its own answer).

public final class SavedCorpus: @unchecked Sendable {
    public static let shared = SavedCorpus()
    public struct Document: Sendable { public var id: String, title: String, address: String, text: String
        /// For the retrieval index (Retrieval.swift): what kind of thing it is, when, where it came from, and a long text's parts.
        public var kind = "story", source = "Autopaper", date: Date? = nil, parts: [RetrievalDoc.Part] = []
        public init(id: String, title: String, address: String, text: String) { self.id = id; self.title = title; self.address = address; self.text = text }
        public init(id: String, title: String, address: String, text: String, kind: String, source: String, date: Date?, parts: [RetrievalDoc.Part] = []) {
            self.id = id; self.title = title; self.address = address; self.text = text; self.kind = kind; self.source = source; self.date = date; self.parts = parts
        }
        var retrieval: RetrievalDoc { RetrievalDoc(id: id, kind: kind, title: title, text: text, date: date, source: source, address: address, parts: parts) }
    }

    public let index = PassageIndex()
    /// The index the on-device model's context is retrieved from: every store, one BM25 index (Retrieval.swift).
    public let retrieval = RetrievalIndex()
    private let lock = NSLock()
    private var sources: [String: @Sendable () -> [Document]] = [:]
    private var known = Set<String>()
    /// Set by the reader lane when `Library.passages(matching:limit:)` exists: (question, limit) -> passages.
    public var library: (@Sendable (String, Int) async -> [PassageHit])?

    public init(readerStore: ReaderStore? = .shared) {
        if let store = readerStore {
            setSource("reader") {
                store.readyDocuments().map { d in
                    let body = d.blocks.filter { !["code", "rule", "embed"].contains($0.kind) }.map(\.text).filter { !$0.isEmpty }.joined(separator: "\n")
                    return Document(id: d.address, title: d.title, address: d.address, text: ([d.dek, body].filter { !$0.isEmpty }).joined(separator: "\n"), kind: d.kind == "document" ? "document" : "story", source: d.kind == "document" ? "Texture" : "Autopaper", date: StoryDates.shared.date(d.address))
                }
            }
        }
    }

    /// Name a source of documents. Replaces the source of that name.
    public func setSource(_ name: String, _ provider: @escaping @Sendable () -> [Document]) {
        lock.lock(); sources[name] = provider; lock.unlock()
    }

    /// Read every source: index what is new or changed, drop what is gone.
    public func refresh() {
        lock.lock(); let all = sources.values; lock.unlock()
        var seen = Set<String>(), docs: [RetrievalDoc] = []
        for provider in all {
            for d in provider() where !d.text.isEmpty || !d.title.isEmpty || !d.parts.isEmpty {
                seen.insert(d.id)
                index.add(docID: d.id, title: d.title, address: d.address, text: d.text)
                docs.append(d.retrieval)
            }
        }
        retrieval.upsertAll(docs)
        lock.lock()
        let gone = known.subtracting(seen).filter { !$0.hasPrefix("page:") }
        known = known.filter { $0.hasPrefix("page:") }.union(seen)
        lock.unlock()
        for id in gone { index.remove(docID: id); retrieval.remove(id) }
    }

    /// The page in front, with the text the phone has for it.
    public func offer(address: String, title: String, text: String) {
        guard !address.isEmpty, !text.isEmpty else { return }
        let id = "page:" + address
        index.add(docID: id, title: title, address: address, text: text)
        retrieval.upsert(RetrievalDoc(id: id, kind: "page", title: title, text: text, date: Date(), source: "the page in front", address: address, key: "\(text.utf8.count)|\(text.hashValue)"))
        lock.lock(); known.insert(id); lock.unlock()
    }

    /// What the on-device model is shown for this question: every store searched with BM25, newest first, within the budget.
    public func retrieve(_ question: String, onAir: String = "", pageAddress: String = "", now: Date = Date()) -> RetrievalContext {
        refresh()
        return retrieval.retrieve(.init(words: question, onAirTitle: onAir, pageID: pageAddress.isEmpty ? nil : "page:" + pageAddress, now: now))
    }

    public func search(_ question: String, pageAddress: String = "", limit: Int = 4) async -> [PassageHit] {
        refresh()
        var hits = index.search(question, limit: limit, prefer: pageAddress.isEmpty ? nil : "page:" + pageAddress)
        if let lib = library { hits += await lib(question, limit); hits.sort { $0.score > $1.score }; hits = Array(hits.prefix(limit)) }
        // The page and its cached copy are one story: keep the better passage once.
        var seen = Set<String>(), out: [PassageHit] = []
        for h in hits where seen.insert(String(h.passage.text.prefix(80))).inserted { out.append(h) }
        return out
    }
}
