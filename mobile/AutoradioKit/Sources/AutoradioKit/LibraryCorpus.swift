import Foundation

// The Reader's books as passages a question can be answered from (join: the talk lane's SavedCorpus and the reader lane's ReaderShelf).
// Bounded: at most `maxPassages` passages, taken from the most recently added documents first, so a large library never makes a search slow.
// The reading of the books is cached until the library changes (the corpus asks for its sources before every search).

public final class LibraryCorpusSource: @unchecked Sendable {
    public static let maxPassages = 200
    private let docs: @Sendable () -> [ShelfDoc]
    private let text: @Sendable (String) -> DocText?
    private let lock = NSLock()
    private var key: String?
    private var cached: [SavedCorpus.Document] = []

    public init(docs: @escaping @Sendable () -> [ShelfDoc], text: @escaping @Sendable (String) -> DocText?) { self.docs = docs; self.text = text }
    public convenience init(shelf: ReaderShelf) { self.init(docs: { shelf.list() }, text: { shelf.text($0) }) }

    public func documents() -> [SavedCorpus.Document] {
        let list = docs().sorted { $0.added > $1.added }
        let k = list.map { "\($0.id)/\($0.length)" }.joined(separator: ",")
        lock.lock(); if key == k { let c = cached; lock.unlock(); return c }; lock.unlock()
        var budget = Self.maxPassages
        var out: [SavedCorpus.Document] = []
        for d in list where budget > 0 {
            guard let t = text(d.id) else { continue }
            var chars = 0, parts: [String] = []
            for b in t.blocks where b.kind != "code" && !b.text.isEmpty {       // no more words than the budget can use
                parts.append(b.text); chars += b.text.count
                if chars > budget * PassageIndex.maxPassageChars { break }
            }
            let pieces = Array(PassageIndex.split(parts.joined(separator: "\n")).prefix(budget))
            guard !pieces.isEmpty else { continue }
            budget -= pieces.count
            out.append(SavedCorpus.Document(id: "book:" + d.id, title: d.title, address: d.address, text: pieces.joined(separator: "\n")))
        }
        lock.lock(); key = k; cached = out; lock.unlock()
        return out
    }
}
