import Foundation

// Retrieval for the on-device model: before it answers, the phone searches everything it holds with BM25 and puts the best matches,
// newest first, into the model's context (owner, 2026-11-16: "tell me about the Senate" must pick up every story about the Senate, ordered
// by recency, not only what is in the queue). Nothing leaves the phone: there is no model and no network call here.
//
// ONE INDEX over every store: a document is {id, kind, title, text, date, source name, address to open or play it}. A story is indexed
// whole; a long text (a book, a PDF, a transcript) is cut into passages of about 180 words that keep their place (chapter label and the
// address of the block, `doc:<id>@<block>`), so a hit can say where it is. The index lives in memory, is built per document when a store
// changes (a document whose key is unchanged is not read again) and answers a query in a few milliseconds (RetrievalTimingTests).
//
// RANKING RULE (decided here, tested in RetrievalTests):
//   1. BM25 (k1 1.2, b 0.75) over the listener's words; title words count three times a body word; the title of what is on air adds half-weight
//      terms when the question is short (two or fewer content words) or leans on a pronoun ("tell me more about that").
//   2. Candidates are the top `candidates` (20) by score that clear the relevance floor: at least min(2, n) of the listener's n content words
//      are in the item, at least half of them, and a score of at least a fifth of the best score. Nothing below the floor goes in.
//   3. Of those, the best by score are taken until the budget is spent (selection is by relevance, so truncation never drops the best hit).
//   4. The chosen items are presented NEWEST FIRST (undated last), because the owner asked for recency order and a model reads the top
//      of a context most closely: the freshest story leads, which is what "tell me about the Senate" means on a news station.
//   5. If nothing clears the floor the context says so in one plain sentence; the model is told to say it has nothing, not to use the queue.
//
// BUDGET: the on-device model's window is 2048 tokens (LlamaLocalModel's engine). Retrieved material gets 1024 of them (half), counted at 3.5
// characters a token (a measured-safe average for English news with ids and dates): 3,584 characters including every header. The rest holds
// the instructions (about 170 tokens), the question and on-air line (about 100) and the answer (at least 300, in practice 2 or 3 sentences).
//
// WAYS THIS CAN FAIL (written before the code; each has a test in RetrievalTests / RetrievalTimingTests):
//  1. "tell me about the Senate" returns an unrelated story, or the Senate stories out of date order.
//  2. A short follow-up ("tell me more about that") has no content word, so nothing matches; it must use the title on air.
//  3. A question with no match answers from the queue: it must yield the explicit nothing-found context, never a nearest guess.
//  4. A book hit names no place: it must carry the chapter label and an address that opens that block.
//  5. The context grows past the budget (many long items, a very long title): it never exceeds `budgetChars`.
//  6. An id the model names cannot be opened: every item's address round-trips to the item.
//  7. A store changes and the index keeps the old text: upsert replaces, remove drops, an unchanged key is a no-op.
//  8. The same story from two sources (front headline and cached full text) appears twice: one item per address.
//  9. Slow: a cold build of the real data is over 200 ms without a snapshot, or a query is over 20 ms (timing test, fails at 5x).
// 10. Empty index, empty question, only stop words, odd Unicode, a huge document: no crash, no hit.
// 11. One book fills the context with ten passages: at most `perParent` passages of one parent.
// 12. The turn cannot be diagnosed afterwards: every retrieval is recorded (query, terms, ids, scores) and goes into context.json.

public struct RetrievalDoc: Sendable, Equatable {
    public var id: String
    /// story | headline | page | book | podcast | transcript | runsheet | file | mail | note
    public var kind: String
    public var title: String
    /// Whole text (indexed whole), unless `parts` is given.
    public var text: String
    public var date: Date?
    /// "Autopaper", "Reader", "Podcasts" ...: said in the context so the model names where it came from.
    public var source: String
    /// What the existing open/play/queue path takes: `pub:<route>`, `doc:<id>@<block>`, a page address.
    public var address: String
    /// A long text as (label, text, block): each part is cut into passages and keeps its label and block.
    public var parts: [Part]
    /// Changes when the content changes; an unchanged key is not indexed again. Nil: derived from the text.
    public var key: String?
    public struct Part: Sendable, Equatable {
        public var label: String, text: String, block: Int?
        public init(label: String, text: String, block: Int? = nil) { self.label = label; self.text = text; self.block = block }
    }
    public init(id: String, kind: String, title: String, text: String = "", date: Date? = nil, source: String = "", address: String? = nil,
                parts: [Part] = [], key: String? = nil) {
        self.id = id; self.kind = kind; self.title = title; self.text = text; self.date = date; self.source = source
        self.address = address ?? id; self.parts = parts; self.key = key
    }
}

public struct RetrievalItem: Codable, Sendable, Equatable {
    public var id: String           // the document id (a passage's is `<doc id>#<n>`)
    public var address: String
    public var kind: String
    public var title: String
    public var source: String
    public var date: Date?
    public var place: String        // "chapter 3" for a passage, else ""
    public var score: Double
    public var text: String         // the passage most relevant to the question
}

public struct RetrievalRecord: Codable, Sendable, Equatable {
    public struct Hit: Codable, Sendable, Equatable { public var id: String, address: String, kind: String, score: Double, date: Date?, chars: Int }
    public var at: Date
    public var query: String
    public var onAir: String
    public var terms: [String]
    public var usedOnAir: Bool
    public var considered: Int          // how many items cleared the floor before the budget
    public var indexed: Int             // how many items the index held
    public var hits: [Hit]
    public var budgetChars: Int
    public var usedChars: Int
    public var millis: Double
    /// The sources the index held ("Autopaper", "Reader" ...), so a nothing-found answer can say what was searched.
    public var sources: [String]?
    public var nothing: Bool { hits.isEmpty }
}

public struct RetrievalContext: Sendable {
    /// Ready for the model's prompt (header, one block per item, or the nothing-found sentence).
    public var text: String
    public var items: [RetrievalItem]
    public var record: RetrievalRecord
    /// "On air now: X. Up next: A; B." (the run sheet), shown to both desks with the passages.
    public var brief = ""
    public var isEmpty: Bool { items.isEmpty }
}

/// What the last retrievals were, so a kept turn can say what the model was shown.
public final class RetrievalLog: @unchecked Sendable {
    public static let shared = RetrievalLog()
    private let lock = NSLock()
    private var records: [RetrievalRecord] = []
    public init() {}
    public func add(_ r: RetrievalRecord) { lock.lock(); records.append(r); if records.count > 20 { records.removeFirst() }; lock.unlock() }
    public var last: RetrievalRecord? { lock.lock(); defer { lock.unlock() }; return records.last }
    public func clear() { lock.lock(); records = []; lock.unlock() }
}

public enum RetrievalBudget {
    public static let windowTokens = 2048
    public static let share = 0.5
    public static let charsPerToken = 3.5
    public static var chars: Int { Int(Double(windowTokens) * share * charsPerToken) }       // 3,584
    public static let candidates = 20
    public static let perParent = 2
    public static let maxItemChars = 800
    public static let minItemChars = 240
}

// MARK: tokens

public enum RetrievalText {
    static let stop: Set<String> = ["a", "an", "the", "and", "or", "but", "of", "to", "in", "on", "at", "for", "with", "by", "from", "about", "as", "is", "are", "was", "were",
        "be", "been", "it", "its", "this", "that", "these", "those", "what", "which", "who", "whom", "whose", "when", "where", "why", "how", "do", "does", "did", "i", "me",
        "my", "you", "your", "we", "our", "they", "their", "he", "she", "his", "her", "him", "them", "tell", "say", "said", "says", "please", "can", "could", "would", "will",
        "any", "there", "has", "have", "had", "not", "no", "so", "if", "than", "then", "just", "more", "some", "much", "many", "know", "show", "give", "like", "want", "now",
        "latest", "news", "anything", "everything", "all", "into", "out", "up", "over"]
    /// Words that point at something already on air instead of naming it.
    static let pronouns: Set<String> = ["that", "this", "it", "its", "them", "they", "he", "she", "him", "her", "those", "these", "there", "one", "more", "again", "same"]

    /// Lowercased words of letters and digits (any script), a small stop list, light stemming. A stem never changes the first letters,
    /// so a prefix and a plural meet ("senators" and "senator"; "rates" and "rate"; "voting" and "vote" do not, and that is accepted).
    public static func tokens(_ s: String, keepStops: Bool = false) -> [String] {
        var ascii = true
        for b in s.utf8 where b >= 128 { ascii = false; break }
        if ascii { return asciiTokens(s, keepStops: keepStops) }
        var out: [String] = []
        var cur = String.UnicodeScalarView()
        func flush() {
            guard !cur.isEmpty else { return }
            let w = String(cur); cur = String.UnicodeScalarView()
            accept(w, keepStops, &out)
        }
        for u in s.unicodeScalars {
            let v = u.value
            if v < 128 {
                if (v >= 97 && v <= 122) || (v >= 48 && v <= 57) { cur.append(u) }
                else if v >= 65 && v <= 90 { cur.append(Unicode.Scalar(v + 32)!) }
                else { flush() }
            } else if u.properties.isAlphabetic || u.properties.numericType != nil {
                cur.append(contentsOf: String(u).lowercased().unicodeScalars)
            } else { flush() }
        }
        flush()
        return out
    }
    private static func accept(_ w: String, _ keepStops: Bool, _ out: inout [String]) {
        let t = stem(w)
        guard keepStops || (!stop.contains(w) && !stop.contains(t)) else { return }
        if t.unicodeScalars.count > 1 || t.unicodeScalars.first.map({ $0.properties.numericType != nil }) == true { out.append(t) }
    }
    /// The fast path for text that is all ASCII (most of it): bytes in, no per-character String work.
    private static func asciiTokens(_ s: String, keepStops: Bool) -> [String] {
        var out: [String] = []
        var buf: [UInt8] = []; buf.reserveCapacity(24)
        func flush() {
            guard !buf.isEmpty else { return }
            let w = String(decoding: buf, as: UTF8.self); buf.removeAll(keepingCapacity: true)
            accept(w, keepStops, &out)
        }
        var str = s
        str.withUTF8 { bytes in
            for v in bytes {
                if (v >= 97 && v <= 122) || (v >= 48 && v <= 57) { buf.append(v) }
                else if v >= 65 && v <= 90 { buf.append(v + 32) }
                else { flush() }
            }
        }
        flush()
        return out
    }

    static func stem(_ w: String) -> String {
        let n = w.utf8.count
        guard n > 3 else { return w }
        if w.hasSuffix("ies"), n > 4 { return String(w.dropLast(3)) + "y" }
        if w.hasSuffix("sses") { return String(w.dropLast(2)) }
        if w.hasSuffix("ss") || w.hasSuffix("us") || w.hasSuffix("is") { return w }
        if w.hasSuffix("s") { return String(w.dropLast()) }
        return w
    }

    /// Paragraph-ish units: blank-line or newline separated lines, long ones cut at sentence ends to at most `max` characters.
    public static func paragraphs(_ text: String, max: Int = 600) -> [String] {
        var out: [String] = []
        for raw in text.components(separatedBy: "\n") {
            var rest = raw.trimmingCharacters(in: .whitespaces)
            while rest.count > max {
                var cut = rest.index(rest.startIndex, offsetBy: max)
                if let r = rest[..<cut].lastIndex(where: { ".!?".contains($0) }), rest.distance(from: rest.startIndex, to: r) > max / 3 { cut = rest.index(after: r) }
                else if let r = rest[..<cut].lastIndex(of: " ") { cut = r }
                out.append(String(rest[..<cut]).trimmingCharacters(in: .whitespaces))
                rest = String(rest[cut...]).trimmingCharacters(in: .whitespaces)
            }
            if !rest.isEmpty { out.append(rest) }
        }
        return out
    }

    /// Passages of about `words` words, cut at paragraph or sentence ends, each with the paragraph index it starts at.
    public static func passages(_ text: String, words: Int = 180) -> [String] {
        var out: [String] = [], buf: [String] = [], count = 0
        for p in paragraphs(text, max: 900) {
            let n = p.split(whereSeparator: \.isWhitespace).count
            if count > 0, count + n > words * 3 / 2 || count >= words { out.append(buf.joined(separator: "\n")); buf = []; count = 0 }
            buf.append(p); count += n
        }
        if !buf.isEmpty { out.append(buf.joined(separator: "\n")) }
        return out
    }
}

// MARK: index

public final class RetrievalIndex: @unchecked Sendable {
    public static let k1 = 1.2, b = 0.75
    /// A title word counts this many times a body word (field weighting).
    public static let titleWeight = 3

    private struct Slot {
        var doc: String            // parent document id
        var id: String
        var item: RetrievalItem    // text holds the whole text of this slot (a story or a passage)
        var terms: [String]
        var length: Int
        var live = true
    }
    private struct Posting { var slot: Int32; var tf: Float }
    private let lock = NSLock()
    private var slots: [Slot] = []
    private var free: [Int] = []
    private var postings: [String: [Posting]] = [:]
    private var children: [String: [Int]] = [:]          // parent id -> slots
    private var keys: [String: String] = [:]
    private var liveCount = 0, totalLength = 0
    public init() {}

    public var count: Int { lock.lock(); defer { lock.unlock() }; return liveCount }
    public var documentCount: Int { lock.lock(); defer { lock.unlock() }; return keys.count }
    public func key(of id: String) -> String? { lock.lock(); defer { lock.unlock() }; return keys[id] }
    public func contains(_ id: String) -> Bool { key(of: id) != nil }

    private struct Prepared { var id: String, key: String; var pieces: [(item: RetrievalItem, counts: [String: Int], length: Int)] }
    private static func key(_ d: RetrievalDoc) -> String {
        d.key ?? "\(d.text.utf8.count)|\(d.title)|\(d.parts.count)|\(d.parts.reduce(0) { $0 &+ $1.text.utf8.count })|\(d.date?.timeIntervalSince1970 ?? 0)"
    }
    /// The slow part (tokenising), free of the lock: a query never waits for a build.
    private static func prepare(_ d: RetrievalDoc, key k: String) -> Prepared {
        var pieces: [(item: RetrievalItem, counts: [String: Int], length: Int)] = []
        let titleTokens = RetrievalText.tokens(d.title)
        func make(_ id: String, _ text: String, address: String, place: String) {
            var tf: [String: Int] = [:], n = 0
            for t in RetrievalText.tokens(text) { tf[t, default: 0] += 1; n += 1 }
            for t in titleTokens { tf[t, default: 0] += titleWeight }
            pieces.append((RetrievalItem(id: id, address: address, kind: d.kind, title: d.title, source: d.source, date: d.date, place: place, score: 0, text: text),
                           tf, n + titleTokens.count))
        }
        if d.parts.isEmpty { make(d.id, d.text, address: d.address, place: "") }
        else {
            var n = 0
            for p in d.parts {
                // A passage opens at its part's first block (the part is the unit the shelf knows).
                let address = p.block.map { (d.address.split(separator: "@").first.map(String.init) ?? d.address) + "@\($0)" } ?? d.address
                for (j, text) in RetrievalText.passages(p.text).enumerated() {
                    n += 1
                    make("\(d.id)#\(n)", text, address: address, place: j == 0 ? p.label : (p.label.isEmpty ? "" : p.label + ", passage \(j + 1)"))
                }
            }
        }
        return Prepared(id: d.id, key: k, pieces: pieces)
    }

    /// Index a document, or index it again when its key changed. Returns false when it was unchanged.
    @discardableResult
    public func upsert(_ d: RetrievalDoc) -> Bool {
        let k = Self.key(d)
        lock.lock(); let same = keys[d.id] == k; lock.unlock()
        if same { return false }
        commit(Self.prepare(d, key: k))
        return true
    }

    /// Many documents at once (a cold build): tokenised on every core, committed one after another.
    public func upsertAll(_ docs: [RetrievalDoc]) {
        lock.lock(); let known = keys; lock.unlock()
        let todo = docs.compactMap { d -> (RetrievalDoc, String)? in let k = Self.key(d); return known[d.id] == k ? nil : (d, k) }
        guard !todo.isEmpty else { return }
        var out = [Prepared?](repeating: nil, count: todo.count)
        out.withUnsafeMutableBufferPointer { buf in
            let base = buf.baseAddress!
            DispatchQueue.concurrentPerform(iterations: todo.count) { i in base[i] = Self.prepare(todo[i].0, key: todo[i].1) }
        }
        for p in out { if let p { commit(p) } }
    }

    private func commit(_ p: Prepared) {
        let d = p
        lock.lock(); defer { lock.unlock() }
        removeLocked(d.id)
        keys[d.id] = d.key
        var mine: [Int] = []
        for p in d.pieces {
            let idx: Int
            let s = Slot(doc: d.id, id: p.item.id, item: p.item, terms: Array(p.counts.keys), length: p.length)
            if let f = free.popLast() { slots[f] = s; idx = f } else { slots.append(s); idx = slots.count - 1 }
            for (t, c) in p.counts { postings[t, default: []].append(Posting(slot: Int32(idx), tf: Float(c))) }
            mine.append(idx); liveCount += 1; totalLength += p.length
        }
        children[d.id] = mine
    }

    public func remove(_ id: String) { lock.lock(); removeLocked(id); lock.unlock() }
    private func removeLocked(_ id: String) {
        keys[id] = nil
        guard let mine = children.removeValue(forKey: id) else { return }
        for idx in mine {
            for t in slots[idx].terms {
                if var list = postings[t] {
                    list.removeAll { Int($0.slot) == idx }
                    postings[t] = list.isEmpty ? nil : list
                }
            }
            totalLength -= slots[idx].length; liveCount -= 1
            slots[idx].live = false; slots[idx].terms = []; slots[idx].item.text = ""
            free.append(idx)
        }
    }
    public func removeAll(except keep: Set<String>, prefix: String? = nil) {
        lock.lock(); defer { lock.unlock() }
        for id in Array(keys.keys) where !keep.contains(id) && (prefix.map { id.hasPrefix($0) } ?? true) { removeLocked(id) }
    }

    /// Candidates over the whole index for these weighted terms (BM25), best first. No floor here.
    func scores(_ terms: [(term: String, weight: Double)], matchTerms: Set<String>, boost: (String) -> Double) -> [(slot: Int, score: Double, matched: Int)] {
        lock.lock(); defer { lock.unlock() }
        guard liveCount > 0 else { return [] }
        let n = Double(liveCount), avg = max(1, Double(totalLength) / n)
        var acc: [Int32: (Double, Int)] = [:]
        for (t, w) in terms {
            guard let list = postings[t] else { continue }
            let d = Double(list.count)
            let idf = log(1 + (n - d + 0.5) / (d + 0.5))
            let isMatch = matchTerms.contains(t)
            for p in list {
                let tf = Double(p.tf), len = Double(slots[Int(p.slot)].length)
                let s = w * idf * tf * (Self.k1 + 1) / (tf + Self.k1 * (1 - Self.b + Self.b * len / avg))
                var e = acc[p.slot] ?? (0, 0)
                e.0 += s; if isMatch { e.1 += 1 }
                acc[p.slot] = e
            }
        }
        var out: [(Int, Double, Int)] = []
        out.reserveCapacity(acc.count)
        for (k, v) in acc { out.append((Int(k), v.0 * boost(slots[Int(k)].doc), v.1)) }
        out.sort { $0.1 != $1.1 ? $0.1 > $1.1 : $0.0 < $1.0 }
        return out
    }

    func item(_ slot: Int) -> RetrievalItem { lock.lock(); defer { lock.unlock() }; return slots[slot].item }
    public func idf(_ t: String) -> Double {
        lock.lock(); defer { lock.unlock() }
        let n = Double(max(liveCount, 1)), d = Double(postings[t]?.count ?? 0)
        return log(1 + (n - d + 0.5) / (d + 0.5))
    }
    func parent(_ slot: Int) -> String { lock.lock(); defer { lock.unlock() }; return slots[slot].doc }

    // MARK: query

    public typealias Query = RetrievalQuery

    /// The listener's words become terms; the on-air title joins when the question is short or leans on a pronoun.
    public static func terms(for q: RetrievalQuery) -> (listener: [String], onAir: [String], usedOnAir: Bool) {
        let listener = unique(RetrievalText.tokens(q.words))
        let raw = q.words.lowercased().split(whereSeparator: { !($0.isLetter || $0.isNumber) }).map(String.init)
        let pointing = raw.contains { RetrievalText.pronouns.contains($0) }
        let air = unique(RetrievalText.tokens(q.onAirTitle)).filter { !listener.contains($0) }
        let use = !air.isEmpty && (listener.count <= 2 || pointing)
        return (listener, use ? air : [], use)
    }
    private static func unique(_ a: [String]) -> [String] { var s = Set<String>(); return a.filter { s.insert($0).inserted } }

    /// The names of the sources the index holds, with how many documents each (for "I searched the paper, your books ...").
    public func sources() -> [String: Int] {
        lock.lock(); defer { lock.unlock() }
        var out: [String: Int] = [:]
        for (_, idxs) in children { if let f = idxs.first, slots[f].live { out[slots[f].item.source, default: 0] += 1 } }
        return out
    }

    /// BM25 over the index with the relevance floor: best first, full text, no budget. The seam a fused retriever wraps.
    public func candidates(_ q: RetrievalQuery, limit: Int) -> [RetrievalItem] {
        let (listener, onAir, _) = Self.terms(for: q)
        let matchTerms = Set(listener.isEmpty ? onAir : listener)
        var weighted = listener.map { ($0, 1.0) }
        weighted += onAir.map { ($0, listener.isEmpty ? 1.0 : 0.5) }
        let page = q.pageID
        var ranked = scores(weighted, matchTerms: matchTerms, boost: { $0 == page ? 1.25 : 1 })
        let need = min(2, matchTerms.count)
        // The floor is measured among the items that match the question's own words: an item that matches only the title on air must not set the bar.
        ranked = ranked.filter { r in r.matched >= need && Double(r.matched) / Double(max(1, matchTerms.count)) >= 0.5 }
        let best = ranked.first?.score ?? 0
        ranked = ranked.filter { $0.score >= best * 0.2 }
        return ranked.prefix(limit).map { var it = item($0.slot); it.score = $0.score; return it }
    }
    public var indexedCount: Int { count }
}

/// What a retrieval is asked: the listener's words, the title on air, the page in front (modest boost), the clock.
public struct RetrievalQuery: Sendable {
    public var words: String
    public var onAirTitle: String
    public var pageID: String?
    public var now: Date
    public init(words: String, onAirTitle: String = "", pageID: String? = nil, now: Date = Date()) {
        self.words = words; self.onAirTitle = onAirTitle; self.pageID = pageID; self.now = now
    }
}

/// THE SEAM. Anything that can rank the phone's items for a question: BM25 (`RetrievalIndex`) today; a fused retriever (BM25 plus vectors by
/// reciprocal rank fusion) wraps one or more of them and implements `candidates` only. `retrieve` (floor already applied by `candidates`,
/// budget, per-parent limit, one item per address, newest-first presentation, record, nothing-found sentence) is shared, so every retriever
/// fills the model's context the same way. `RetrievalItem` is the hit type (its `score` is the retriever's own scale; `id` is stable).
public protocol Retriever: Sendable {
    /// Best first, relevance floor applied, items carrying their FULL text (the context cuts the best window). At most `limit`.
    func candidates(_ q: RetrievalQuery, limit: Int) -> [RetrievalItem]
    /// How many items it holds, and which sources (name -> documents), for the nothing-found sentence and the record.
    var indexedCount: Int { get }
    func sources() -> [String: Int]
    /// Inverse document frequency of a normalised term (used only to choose the best window of a long item).
    func idf(_ term: String) -> Double
}

public extension Retriever {
    func retrieve(_ q: RetrievalQuery, record: Bool = true) -> RetrievalContext {
        let t0 = DispatchTime.now().uptimeNanoseconds
        let (listener, onAir, usedOnAir) = RetrievalIndex.terms(for: q)
        let matchTerms = Set(listener.isEmpty ? onAir : listener)
        let top = candidates(q, limit: RetrievalBudget.candidates)
        let indexed = indexedCount

        // Selection by relevance until the budget is spent; at most `perParent` passages of one document; one item per address.
        var chosen: [RetrievalItem] = [], used = 0, perParent: [String: Int] = [:], seenAddress = Set<String>()
        let budget = RetrievalBudget.chars
        let header = RetrievalIndex.header(query: q.words, onAir: q.onAirTitle, now: q.now)
        used += header.count
        for item in top {
            let parent = String(item.id.split(separator: "#", maxSplits: 1).first ?? "")
            guard perParent[parent, default: 0] < RetrievalBudget.perParent else { continue }
            // A headline and the cached full text of one story are one story. Passages of a book differ by id.
            if item.kind != "book", item.kind != "transcript", !seenAddress.insert(item.address).inserted { continue }
            let room = budget - used - RetrievalIndex.overhead(item)
            guard room >= RetrievalBudget.minItemChars else { break }
            var it = item
            it.text = RetrievalIndex.snippet(item.text, terms: matchTerms.union(onAir), idf: self.idf, limit: min(RetrievalBudget.maxItemChars, room))
            used += RetrievalIndex.overhead(it) + it.text.count
            perParent[parent, default: 0] += 1
            chosen.append(it)
        }
        // Presentation: newest first, undated last, ties by score.
        chosen.sort { a, b in
            switch (a.date, b.date) {
            case let (x?, y?): return x != y ? x > y : a.score > b.score
            case (_?, nil): return true
            case (nil, _?): return false
            default: return a.score > b.score
            }
        }
        let srcs = sources()
        let text: String
        if chosen.isEmpty {
            used = 0
            text = RetrievalIndex.nothing(query: q.words, searched: indexed)
        } else {
            text = header + chosen.enumerated().map { RetrievalIndex.block($0.offset + 1, $0.element) }.joined()
            used = text.count
        }
        let rec = RetrievalRecord(at: q.now, query: q.words, onAir: q.onAirTitle, terms: listener + onAir, usedOnAir: usedOnAir, considered: top.count, indexed: indexed,
                                  hits: chosen.map { .init(id: $0.id, address: $0.address, kind: $0.kind, score: ($0.score * 1000).rounded() / 1000, date: $0.date, chars: $0.text.count) },
                                  budgetChars: budget, usedChars: used, millis: Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e6,
                                  sources: srcs.keys.sorted())
        if record { RetrievalLog.shared.add(rec) }
        return RetrievalContext(text: text, items: chosen, record: rec)
    }
}

extension RetrievalIndex: Retriever {}

extension RetrievalIndex {
    // MARK: format

    static func day(_ d: Date?) -> String {
        guard let d else { return "undated" }
        let c = Calendar(identifier: .gregorian).dateComponents(in: TimeZone(identifier: "UTC")!, from: d)
        return String(format: "%04d-%02d-%02d", c.year ?? 0, c.month ?? 0, c.day ?? 0)
    }
    static func flat(_ s: String) -> String { s.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ").replacingOccurrences(of: "\"", with: "'") }
    static func header(query: String, onAir: String, now: Date) -> String {
        var s = "Today is \(day(now)). Passages found on this phone for \"\(String(flat(query).prefix(120)))\", newest first.\n"
        if !onAir.isEmpty { s += "On air now: \"\(String(flat(onAir).prefix(120)))\".\n" }
        return s
    }
    static func nothing(query: String, searched: Int) -> String {
        "Nothing on this phone matches \"\(String(flat(query).prefix(120)))\" (searched \(searched) items). Say you have nothing on it; do not answer from memory or from what is playing.\n"
    }
    static func label(_ i: RetrievalItem) -> String {
        var parts = [i.kind, day(i.date)]
        if !i.source.isEmpty { parts.append(i.source) }
        if !i.place.isEmpty { parts.append(i.place) }
        return parts.joined(separator: " | ")
    }
    static func headline(_ n: Int, _ i: RetrievalItem) -> String { "Passage \(n) | \(label(i)) | id \(i.address) | \"\(String(flat(i.title).prefix(120)))\"\n" }
    static func overhead(_ i: RetrievalItem) -> Int { headline(9, i).count + 1 }
    static func block(_ n: Int, _ i: RetrievalItem) -> String { headline(n, i) + flat(i.text) + "\n" }

    /// The window of consecutive paragraphs that holds the most (idf-weighted) question words, cut to `limit` characters at a word.
    static func snippet(_ text: String, terms: Set<String>, idf: (String) -> Double, limit: Int) -> String {
        let paras = RetrievalText.paragraphs(text, max: 500)
        guard !paras.isEmpty else { return "" }
        if text.count <= limit { return flat(text) }
        var bestStart = 0, bestScore = -1.0
        let scored: [Double] = paras.map { p in
            var seen = Set<String>(), s = 0.0
            for t in RetrievalText.tokens(p) where terms.contains(t) { if seen.insert(t).inserted { s += idf(t) } else { s += 0.1 * idf(t) } }
            return s
        }
        for i in paras.indices {
            var len = 0, s = 0.0, j = i
            while j < paras.count, len + paras[j].count <= limit || j == i { len += paras[j].count + 1; s += scored[j]; j += 1 }
            if s > bestScore + 1e-9 { bestScore = s; bestStart = i }
        }
        var out = "", j = bestStart
        while j < paras.count, out.count + paras[j].count + 1 <= limit || out.isEmpty { out += (out.isEmpty ? "" : " ") + paras[j]; j += 1 }
        if out.count > limit {
            var cut = out.index(out.startIndex, offsetBy: limit)
            if let sp = out[..<cut].lastIndex(of: " ") { cut = sp }
            out = String(out[..<cut]) + "…"
        }
        return (bestStart > 0 ? "… " : "") + flat(out)
    }
}
