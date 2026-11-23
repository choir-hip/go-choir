import Foundation
@testable import AutoradioKit

// A stand-in for shared code's answer (AutoradioLab/retrieval-run.ts) in the voice tests: it ranks nothing the way the real one does (that is
// search.test.ts); it only lets a test hand the turn a context of the right shape: documents that share the question's words, newest first.
struct FakeDoc: Sendable {
    var id: String, title: String, address: String, text: String
    var kind = "story", source = "Autopaper", date: Date? = nil
}
enum FakeRetrieval {
    static let stop: Set<String> = ["what", "about", "tell", "with", "that", "this", "does", "have", "from", "your", "how", "many", "did", "say", "the"]
    static func terms(_ s: String) -> [String] {
        s.lowercased().split { !$0.isLetter && !$0.isNumber }.map(String.init).filter { $0.count > 3 && !stop.contains($0) }.map { $0.hasSuffix("s") ? String($0.dropLast()) : $0 }
    }
    static func context(_ docs: [FakeDoc], _ words: String, onAir: String = "", brief: String = "", now: Date = Date(timeIntervalSince1970: 1_794_700_000)) -> RetrievalContext {
        var q = terms(words)
        let usedOnAir = q.isEmpty || words.lowercased().contains("that") || words.lowercased().contains("this")
        if usedOnAir { q += terms(onAir) }
        let need = max(1, (q.count + 1) / 2)
        let hits = docs.filter { d in let hay = (d.title + " " + d.text).lowercased(); return q.filter { hay.contains($0) }.count >= need }
            .sorted { ($0.date ?? .distantPast) > ($1.date ?? .distantPast) }
        let items = hits.map { RetrievalItem(id: $0.id, address: $0.address, kind: $0.kind, title: $0.title, source: $0.source, date: $0.date, place: "", score: 1, text: $0.text) }
        var text = "Today is 2026-11-14.\n" + (onAir.isEmpty ? "" : "On air now: \"\(onAir)\".\n")
        for (n, i) in items.enumerated() { text += "Passage \(n + 1) | \(i.kind) | \(i.source) | id \(i.address) | \"\(i.title)\"\n\(i.text)\n" }
        if items.isEmpty { text = "Nothing on this phone matches \"\(words)\". Say you have nothing on it.\n" }
        let rec = RetrievalRecord(at: now, query: words, onAir: onAir, terms: q, usedOnAir: usedOnAir, considered: items.count, indexed: docs.count,
                                  hits: items.map { .init(id: $0.id, address: $0.address, kind: $0.kind, score: 1, date: $0.date, chars: $0.text.count) },
                                  budgetChars: 3584, usedChars: text.count, sources: Array(Set(docs.map(\.source))).sorted())
        return RetrievalContext(text: text, items: items, record: rec, brief: brief)
    }
    /// What the holder asks: the same fake for any question; the on-air title is the page title when it is a content page, as the real one does.
    static func provider(_ docs: [FakeDoc], hint: String = "", brief: String = "") -> @Sendable (String, String, String) async -> RetrievalContext? {
        { words, title, address in
            let air = (address.isEmpty || ["pub:", "doc:", "texture:", "mail:", "podcast:", "file:"].contains { address.hasPrefix($0) }) && !title.isEmpty ? title : hint
            return context(docs, words, onAir: air, brief: brief)
        }
    }
}
