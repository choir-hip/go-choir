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
    /// The front list the phone last cached (`paper-front.json`): title, dek and date of up to a hundred stories. No full text.
    public static func paperFront(file: URL) -> [SavedCorpus.Document] {
        guard let d = try? Data(contentsOf: file), let o = (try? JSONSerialization.jsonObject(with: d)) as? [String: Any] else { return [] }
        return PaperStory.parseList(o).compactMap { s in
            guard !s.route.isEmpty, !s.title.isEmpty else { return nil }
            StoryDates.shared.note("pub:" + s.route, s.updated)
            return SavedCorpus.Document(id: "front:" + s.route, title: s.title, address: "pub:" + s.route, text: s.dek,
                                        kind: "story", source: "Autopaper", date: s.updated)
        }
    }

    /// Subscribed shows and their cached episodes (title and show notes, dated), from the feeds already on the phone.
    public static func podcasts(shows: [PodcastShow], feed: (String) -> PodcastFeed?, transcript: (String, String) -> PodcastTranscript?) -> [SavedCorpus.Document] {
        var out: [SavedCorpus.Document] = []
        for show in shows {
            out.append(SavedCorpus.Document(id: "podcast:" + show.feedURL, title: show.title, address: PodcastService.showAddress(show.feedURL),
                                            text: [show.author, show.summary].filter { !$0.isEmpty }.joined(separator: ". "), kind: "podcast", source: "Podcasts", date: nil))
            guard let f = feed(show.feedURL) else { continue }
            for e in f.episodes.prefix(200) where !e.title.isEmpty {
                let addr = PodcastService.episodeAddress(feed: show.feedURL, id: e.id), when = e.published.map { Date(timeIntervalSince1970: $0) }
                if let t = transcript(show.feedURL, e.id), !t.blocks.isEmpty {
                    // Passages keep the second they start at: the part label says where in the episode.
                    var parts: [RetrievalDoc.Part] = [], cur: [String] = [], start = 0.0, n = 0
                    for b in t.blocks {
                        if cur.isEmpty { start = b.start }
                        cur.append(b.text); n += b.text.count
                        if n > 1100 { parts.append(.init(label: "at \(Int(start / 60)) min", text: cur.joined(separator: " "))); cur = []; n = 0 }
                    }
                    if !cur.isEmpty { parts.append(.init(label: "at \(Int(start / 60)) min", text: cur.joined(separator: " "))) }
                    out.append(SavedCorpus.Document(id: "podcast-tr:" + addr, title: show.title + ": " + e.title, address: addr, text: "", kind: "transcript", source: "Podcasts", date: when, parts: parts))
                } else {
                    out.append(SavedCorpus.Document(id: "podcast:" + addr, title: show.title + ": " + e.title, address: addr, text: e.notes, kind: "podcast", source: "Podcasts", date: when))
                }
            }
        }
        return out
    }

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

    /// Mail rows (sender, subject, the preview) as documents; the body is added when a message is opened. Not persisted.
    public static func mail(_ rows: [MailSummary]) -> [SavedCorpus.Document] {
        rows.filter { !$0.id.isEmpty }.map { r in
            SavedCorpus.Document(id: "mail:" + r.id, title: r.subject.isEmpty ? "(no subject)" : r.subject, address: "mail:" + r.id,
                                 text: "From \(r.from). \(r.snippet)", kind: "mail", source: "Mail", date: PaperStory.date(r.at))
        }
    }
    public static func mailMessage(_ d: MailDetail) -> SavedCorpus.Document {
        let r = d.summary
        return SavedCorpus.Document(id: "mail:" + r.id, title: r.subject.isEmpty ? "(no subject)" : r.subject, address: "mail:" + r.id,
                                    text: "From \(r.from). \(d.text.isEmpty ? r.snippet : d.text)", kind: "mail", source: "Mail", date: PaperStory.date(r.at))
    }
    /// A folder's file names (never their contents): searchable by name, openable by path.
    public static func fileNames(_ entries: [FileEntry]) -> [SavedCorpus.Document] {
        entries.filter { !$0.isDirectory }.map { e in
            SavedCorpus.Document(id: "file:" + e.path, title: e.name, address: "file:" + e.path, text: e.path, kind: "file", source: "Files", date: e.modified)
        }
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
            let sheet = await MainActor.run { RunSheetStore.shared.sheet }
            let r = RetrievalSources.runSheet(sheet)
            hold(group: "runsheet", r.docs); onAirHint = r.onAirTitle
            let ahead = r.docs.filter { $0.source == "Up next" }.prefix(4).map(\.title)
            onAirBrief = (r.onAirTitle.isEmpty ? "Nothing is on air." : "On air now: \(r.onAirTitle).") + (ahead.isEmpty ? "" : " Up next: " + ahead.joined(separator: "; ") + ".")
        }
    }
}

public extension PodcastService {
    /// Documents for retrieval: the library's shows, the episodes of their cached feeds, and the transcripts the phone has fetched.
    func retrievalDocuments() -> [SavedCorpus.Document] {
        RetrievalSources.podcasts(shows: subscriptions(), feed: { self.cachedFeed($0) }, transcript: { self.cachedTranscript(feed: $0, id: $1) })
    }
}
