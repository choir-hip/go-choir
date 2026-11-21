import Foundation

// What the phone already holds, as one searchable body: the stories and documents the reader has shown (HeldPages), the front list's headlines, the page in
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

    /// The index the on-device model's context is retrieved from: every store, one BM25 index (Retrieval.swift).
    public let retrieval = RetrievalIndex()
    private let lock = NSLock()
    private var sources: [String: @Sendable () -> [Document]] = [:]
    private var known = Set<String>()

    // Documents other code hands over when it opens or loads something (Mail rows, a saved Texture document, a Files listing): kept per group
    // (a group is replaced whole), at most `maxHeld` documents in all, oldest group entries out first. Not persisted (see Retrieval notes).
    public static let maxHeld = 1500
    private var held: [String: [Document]] = [:]
    private var heldOrder: [String] = []
    public func hold(group: String, _ docs: [Document]) {
        lock.lock()
        held[group] = docs; heldOrder.removeAll { $0 == group }; heldOrder.append(group)
        while held.values.reduce(0, { $0 + $1.count }) > Self.maxHeld, let first = heldOrder.first, first != group { held[first] = nil; heldOrder.removeFirst() }
        if let d = held[group], d.count > Self.maxHeld { held[group] = Array(d.prefix(Self.maxHeld)) }
        lock.unlock()
    }
    /// Add one document to a group (a saved Texture document), replacing its earlier copy; the group keeps its newest `cap`.
    public func hold(group: String, adding d: Document, cap: Int = 200) {
        lock.lock(); var list = held[group] ?? []; lock.unlock()
        list.removeAll { $0.id == d.id }; list.append(d)
        hold(group: group, Array(list.suffix(cap)))
    }
    /// Work that must finish before a question reads the index (the run sheet lives on the main actor). Run by `prepare()`.
    public var preparers: [@Sendable () async -> Void] = []
    /// The title of what is on air, for "that" and "this" when no page is in front.
    public var onAirHint = ""
    /// "On air now: X. Up next: A; B." for the server desk when the page in front has no text.
    public var onAirBrief = ""
    public func prepare() async { for p in preparers { await p() } }
    /// The retriever the model's context is built from: BM25 over `retrieval` today; a fused retriever can be set here.
    public var retriever: (any Retriever)?
    /// Called by `reset()` (sign-out): other holders of derived data (the vector store) clear themselves.
    public var resetHooks: [@Sendable () -> Void] = []

    public init(heldPages: HeldPages? = .shared) {
        setSource("held") { [unowned self] in lock.lock(); defer { lock.unlock() }; return held.values.flatMap { $0 } }
        if let heldPages {
            setSource("reader") {
                heldPages.all().map { d in
                    return Document(id: d.address, title: d.title, address: d.address, text: ([d.dek, d.body].filter { !$0.isEmpty }).joined(separator: "\n"), kind: d.kind == "document" ? "document" : "story", source: d.kind == "document" ? "Texture" : "Autopaper", date: StoryDates.shared.date(d.address))
                }
            }
        }
    }

    /// A source that reports changes instead of its whole content (the ContentStore: far too big to rebuild before every question).
    private var deltas: [String: @Sendable () -> (up: [Document], removed: [String])] = [:]
    public func setDeltaSource(_ name: String, _ drain: @escaping @Sendable () -> (up: [Document], removed: [String])) {
        lock.lock(); deltas[name] = drain; lock.unlock()
    }
    /// Sign-out: nothing of the account stays in the index, the held groups, or what the last answer pointed at.
    public func reset() {
        lock.lock(); held = [:]; heldOrder = []; known = []; onAirHint = ""; lock.unlock()
        retrieval.removeAll(except: [])
        RetrievalLog.shared.clear()
        for h in resetHooks { h() }
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
                docs.append(d.retrieval)
            }
        }
        retrieval.upsertAll(docs)
        lock.lock(); let drains = Array(deltas.values); lock.unlock()
        for drain in drains {
            let d = drain()
            retrieval.upsertAll(d.up.map(\.retrieval))
            for id in d.removed { retrieval.remove("store:" + id) }
        }
        lock.lock()
        let gone = known.subtracting(seen).filter { !$0.hasPrefix("page:") }
        known = known.filter { $0.hasPrefix("page:") }.union(seen)
        lock.unlock()
        for id in gone { retrieval.remove(id) }
    }

    /// The page in front, with the text the phone has for it.
    public func offer(address: String, title: String, text: String) {
        guard !address.isEmpty, !text.isEmpty else { return }
        let id = "page:" + address
        retrieval.upsert(RetrievalDoc(id: id, kind: "page", title: title, text: text, date: Date(), source: "the page in front", address: address, key: "\(text.utf8.count)|\(text.hashValue)"))
        lock.lock(); known.insert(id); lock.unlock()
    }

    /// What the on-device model is shown for this question: every store searched with BM25, newest first, within the budget.
    public func retrieve(_ question: String, onAir: String = "", pageAddress: String = "", now: Date = Date()) -> RetrievalContext {
        refresh()
        var c = (retriever ?? retrieval).retrieve(.init(words: question, onAirTitle: onAir, pageID: pageAddress.isEmpty ? nil : "page:" + pageAddress, now: now))
        lock.lock(); c.brief = onAirBrief; lock.unlock()          // what is on air and ahead, for both desks
        return c
    }
}
