import Foundation

// Adapters that turn what each store holds into documents for the retrieval index (Retrieval.swift). Each is a pure function of the
// store's data, so a test can build the store and read the documents. Where the phone holds only a title or a headline and dek (not the full
// text) the document says so by being short: that is a gap in the store, not in the index.
//
// WAYS THIS CAN FAIL (tests in RetrievalSourcesTests):
//  1. A cache file that is missing or corrupt: no documents, no crash.
//  2. A story with no title or route: skipped (it could not be opened).
//  3. An episode with no date: undated (listed last), never today.
//  4. A transcript is long: it is passages with the time they start, not one document.

/// When each paper story was published, by address, so the full text the reader cached (which carries no date) is ordered by recency too.
public final class StoryDates: @unchecked Sendable {
    public static let shared = StoryDates()
    private let lock = NSLock()
    private var dates: [String: Date] = [:]
    public init() {}
    public func note(_ address: String, _ d: Date?) { guard let d else { return }; lock.lock(); dates[address] = d; lock.unlock() }
    public func date(_ address: String) -> Date? { lock.lock(); defer { lock.unlock() }; return dates[address] }
}

public enum RetrievalSources {
    /// The run sheet's entries on air and ahead (what "that" and "this" point at), with the address they play from. Heard and skipped entries are history.
    public static func runSheet(_ sheet: RunSheet) -> (docs: [SavedCorpus.Document], onAirTitle: String) {
        var onAir = ""
        let docs: [SavedCorpus.Document] = sheet.entries.compactMap { e in
            guard e.state == .onAir || e.state == .ahead, !e.title.isEmpty || !(e.text ?? "").isEmpty else { return nil }
            let air = e.state == .onAir
            if air, onAir.isEmpty { onAir = e.title }
            return SavedCorpus.Document(id: "runsheet:" + e.id, title: e.title, address: e.source?.ref ?? ("runsheet:" + e.id), text: e.text ?? "", kind: "runsheet",
                                        source: air ? "On air" : "Up next", date: nil)
        }
        return (docs, onAir)
    }

    /// The reader's chosen edition topics, as one small note.
    public static func editionTopics(_ topics: [String]) -> [SavedCorpus.Document] {
        topics.isEmpty ? [] : [SavedCorpus.Document(id: "edition-topics", title: "My edition topics", address: "paper:mine", text: "The reader follows these topics: " + topics.joined(separator: ", "),
                                                    kind: "note", source: "Autopaper", date: nil)]
    }
    public static func openPages(_ pages: [Page]) -> [SavedCorpus.Document] {
        pages.filter { !$0.title.isEmpty }.map { SavedCorpus.Document(id: "openpage:" + $0.address, title: $0.title, address: $0.address, text: "An open \($0.kind) page.", kind: "page", source: "Open pages", date: nil) }
    }
}

public extension SavedCorpus {
    /// Read the run sheet before every question (it lives on the main actor), and name what is on air.
    func installRunSheet() {
        preparers.append { [unowned self] in
            let sheet = RadioBridge.shared.sheet() ?? RunSheet()
            let r = RetrievalSources.runSheet(sheet)
            hold(group: "runsheet", r.docs); onAirHint = r.onAirTitle
            let ahead = r.docs.filter { $0.source == "Up next" }.prefix(4).map(\.title)
            onAirBrief = (r.onAirTitle.isEmpty ? "Nothing is on air." : "On air now: \(r.onAirTitle).") + (ahead.isEmpty ? "" : " Up next: " + ahead.joined(separator: "; ") + ".")
        }
    }
}
