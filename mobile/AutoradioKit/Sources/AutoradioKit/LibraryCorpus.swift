import Foundation

// The Reader's books as passages a question can be answered from (join: the talk lane's SavedCorpus and the reader lane's ReaderShelf).
// Bounded: at most `maxPassages` passages, taken from the most recently added documents first. The retrieval index gets every
// chapter of those books as parts, so a handful of books are searched whole.
// The reading of the books is cached until the library changes (the corpus asks for its sources before every search).

public final class LibraryCorpusSource: @unchecked Sendable {
    public static let maxPassages = 6000
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
            // One part per section (chapter or page run), each keeping the block it starts at, so a hit can say where it is and open there.
            var parts: [RetrievalDoc.Part] = [], cur: [String] = [], curBlock = 0, curSection = -1, chars = 0
            func close() {
                guard !cur.isEmpty else { return }
                let title = curSection >= 0 && curSection < t.sections.count ? t.sections[curSection].title : ""
                let label = (d.kind == "pdf" ? "page " : "chapter ") + "\(curSection + 1)" + (title.isEmpty ? "" : ": " + title)
                parts.append(.init(label: label, text: cur.joined(separator: "\n"), block: curBlock)); cur = []
            }
            for (i, b) in t.blocks.enumerated() where b.kind != "code" && !b.text.isEmpty {
                if b.section != curSection { close(); curSection = b.section; curBlock = i }
                cur.append(b.text); chars += b.text.count
            }
            close()
            let passages = parts.reduce(0) { $0 + RetrievalText.passages($1.text).count }
            guard !parts.isEmpty else { continue }
            budget -= max(1, passages)
            out.append(SavedCorpus.Document(id: "book:" + d.id, title: d.title, address: d.address, text: "",
                                            kind: "book", source: "Reader", date: d.added, parts: parts))
        }
        lock.lock(); key = k; cached = out; lock.unlock()
        return out
    }
}
