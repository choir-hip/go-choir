import Foundation

// A small in-memory index of paragraph-sized passages, ranked by BM25 (Robertson and Zaragoza; k1 1.2, b 0.75). No dependency, no
// model: it lets the phone answer first from what it already holds (the stories it has fetched, the page in front, library passages).
//
// WAYS THIS CAN FAIL (written before the code; each has a test in PassageIndexTests):
//  1. A question the corpus cannot answer ("what is the weather on Mars") returns nothing: a hit needs at least `minMatched` distinct
//     question terms in the passage and `minCoverage` of the question's terms, so one shared common word is not relevance.
//  2. Stop words and question words ("what", "the", "about") count for nothing; plural and singular match ("rates", "rate").
//  3. An empty index, an empty question, a question of only stop words: no hit, no crash.
//  4. A document indexed again (the page's text grew) replaces its passages; it never doubles.
//  5. A very long document is cut into passages of bounded length; a document of one huge line too.
//  6. Concurrent add and search: one lock, no torn state.
//  7. Non-ASCII text (accents, other scripts) is tokenised on letters and digits, not dropped.

public struct Passage: Equatable, Sendable {
    public var docID: String
    public var title: String
    public var address: String
    public var text: String
    public var ordinal: Int
}
public struct PassageHit: Equatable, Sendable {
    public var passage: Passage
    public var score: Double
    public var matched: Int
}

public final class PassageIndex: @unchecked Sendable {
    public static let minMatched = 2
    public static let minCoverage = 0.5
    public static let maxPassageChars = 700
    public static let minPassageChars = 80

    private struct Entry { var passage: Passage; var terms: [String: Int]; var length: Int }
    private let lock = NSLock()
    private var entries: [Entry] = []
    private var df: [String: Int] = [:]
    private var totalLength = 0
    private var keys: [String: String] = [:]       // docID -> content key, so an unchanged document is not indexed again
    public init() {}

    public var passageCount: Int { lock.lock(); defer { lock.unlock() }; return entries.count }
    public func contentKey(of docID: String) -> String? { lock.lock(); defer { lock.unlock() }; return keys[docID] }

    static let stop: Set<String> = ["a", "an", "the", "and", "or", "but", "of", "to", "in", "on", "at", "for", "with", "by", "from", "about", "as", "is", "are", "was", "were",
        "be", "been", "it", "its", "this", "that", "these", "those", "what", "which", "who", "whom", "whose", "when", "where", "why", "how", "do", "does", "did", "i", "me",
        "my", "you", "your", "we", "our", "they", "their", "he", "she", "his", "her", "tell", "say", "said", "says", "please", "can", "could", "would", "will", "any",
        "there", "has", "have", "had", "not", "no", "so", "if", "than", "then", "story", "stories", "paper", "article", "page", "just", "more", "some", "much", "many"]

    public static func tokens(_ s: String) -> [String] {
        var out: [String] = [], cur = ""
        func flush() {
            guard !cur.isEmpty else { return }
            var t = cur; cur = ""
            if t.count > 3, t.hasSuffix("ies") { t = String(t.dropLast(3)) + "y" }
            else if t.count > 3, t.hasSuffix("s"), !t.hasSuffix("ss") { t.removeLast() }
            if !stop.contains(t), t.count > 1 || t.first?.isNumber == true { out.append(t) }
        }
        for ch in s.lowercased() { if ch.isLetter || ch.isNumber { cur.append(ch) } else { flush() } }
        flush()
        return out
    }

    /// Paragraph-sized pieces: lines are joined while short, long ones are cut at sentence ends.
    public static func split(_ text: String) -> [String] {
        var out: [String] = [], buf = ""
        func push() { let t = buf.trimmingCharacters(in: .whitespacesAndNewlines); if !t.isEmpty { out.append(t) }; buf = "" }
        for raw in text.components(separatedBy: .newlines) {
            let line = raw.trimmingCharacters(in: .whitespaces)
            if line.isEmpty { if buf.count >= minPassageChars { push() }; continue }
            var rest = line
            while rest.count > maxPassageChars {
                var cut = rest.index(rest.startIndex, offsetBy: maxPassageChars)
                if let r = rest[..<cut].lastIndex(where: { ".!?".contains($0) }) { cut = rest.index(after: r) }
                else if let r = rest[..<cut].lastIndex(of: " ") { cut = r }
                if !buf.isEmpty { push() }
                buf = String(rest[..<cut]); push()
                rest = String(rest[cut...]).trimmingCharacters(in: .whitespaces)
            }
            if buf.count + rest.count + 1 > maxPassageChars { push() }
            buf += (buf.isEmpty ? "" : " ") + rest
            if buf.count >= minPassageChars * 2 { push() }
        }
        push()
        return out
    }

    /// Index (or index again) a document. `key` says whether it changed: an unchanged key is a no-op.
    public func add(docID: String, title: String, address: String, text: String, key: String? = nil) {
        let k = key ?? "\(text.utf8.count)|\(text.hashValue)"
        lock.lock()
        if keys[docID] == k { lock.unlock(); return }
        removeLocked(docID)
        keys[docID] = k
        let titleTerms = Self.tokens(title)
        for (i, p) in Self.split(text).enumerated() {
            var tf: [String: Int] = [:]
            let toks = Self.tokens(p)
            for t in toks { tf[t, default: 0] += 1 }
            for t in titleTerms { tf[t, default: 0] += 1 }       // the story's title says what each of its passages is about
            let len = toks.count + titleTerms.count
            entries.append(Entry(passage: Passage(docID: docID, title: title, address: address, text: p, ordinal: i), terms: tf, length: len))
            for t in tf.keys { df[t, default: 0] += 1 }
            totalLength += len
        }
        lock.unlock()
    }

    public func remove(docID: String) { lock.lock(); removeLocked(docID); lock.unlock() }
    private func removeLocked(_ docID: String) {
        keys[docID] = nil
        var keep: [Entry] = []
        for e in entries {
            if e.passage.docID == docID { for t in e.terms.keys { if let n = df[t] { df[t] = n <= 1 ? nil : n - 1 } }; totalLength -= e.length }
            else { keep.append(e) }
        }
        entries = keep
    }

    /// The best passages for a question, best first, those that clear the relevance floor only. `prefer` is a document (the page in front)
    /// whose passages get a modest boost: a question asked on a page is usually about it.
    public func search(_ question: String, limit: Int = 4, prefer: String? = nil) -> [PassageHit] {
        let q = Array(Set(Self.tokens(question)))
        guard !q.isEmpty, limit > 0 else { return [] }
        lock.lock(); defer { lock.unlock() }
        guard !entries.isEmpty else { return [] }
        let n = Double(entries.count), avg = max(1, Double(totalLength) / n)
        let need = max(1, min(Self.minMatched, q.count))
        var hits: [PassageHit] = []
        for e in entries {
            var score = 0.0, matched = 0
            for t in q {
                guard let f = e.terms[t] else { continue }
                matched += 1
                let d = Double(df[t] ?? 1)
                let idf = log(1 + (n - d + 0.5) / (d + 0.5))
                let tf = Double(f)
                score += idf * tf * 2.2 / (tf + 1.2 * (0.25 + 0.75 * Double(e.length) / avg))
            }
            guard matched >= need, Double(matched) / Double(q.count) >= Self.minCoverage else { continue }
            if let p = prefer, e.passage.docID == p { score *= 1.25 }
            hits.append(PassageHit(passage: e.passage, score: score, matched: matched))
        }
        hits.sort { $0.score != $1.score ? $0.score > $1.score : ($0.passage.docID, $0.passage.ordinal) < ($1.passage.docID, $1.passage.ordinal) }
        return Array(hits.prefix(limit))
    }
}
