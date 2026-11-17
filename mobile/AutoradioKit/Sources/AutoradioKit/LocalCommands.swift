import Foundation

// Commands the phone can carry out by itself from what it holds, with no model: "read my book", "play the latest <show>", "what is in my mail".
// (Owner, 2026-11-17: tapping the page's suggestion "Read my book" got "I can't read your book ... the page is empty". A command is not a question.)
// They are recognised after the playback commands (SpokenCommand) and "play that" (RetrievedCommand), and resolved against the ContentStore, the
// Reader's shelf and the podcast library: the same rows retrieval reads.
//
// WAYS THIS CAN FAIL (tests in LocalCommandsTests):
//  1. A question that merely contains the words ("who wrote the book I should read", "what is the latest news") is not a command: the WHOLE utterance
//     must be the command (verb, filler words, a reference), as for SpokenCommand.
//  2. "read my book" with one book: that book, from the block the listener reached; with several: the one the words name, else the one being read
//     (progress above zero, the most recently added of those), else the newest; with none: not a command (the desk says so).
//  3. "play the latest <show>": the show is named by words of its title (one show only; two shows tying is not a command); its newest playable episode.
//  4. Mail: counts and the newest message come from the stored inbox; an empty store says so plainly and claims nothing.
//  5. A name that matches nothing in the library ("play the latest zebra show"): not a command.

public enum LocalCommand: Equatable, Sendable {
    case readBook(id: String, title: String, block: Int)
    case playEpisode(address: String, title: String)
    case readMail(address: String, unread: Int, total: Int, from: String, subject: String)

    public var name: String {
        switch self { case .readBook: return "read-book"; case .playEpisode: return "play-episode"; case .readMail: return "read-mail" }
    }
    /// The line the conversation shows.
    public var line: String {
        switch self {
        case .readBook(_, let t, let b): return b > 0 ? "Okay, reading \(String(t.prefix(60))) from where you left off." : "Okay, reading \(String(t.prefix(60)))."
        case .playEpisode(_, let t): return "Okay, playing \(String(t.prefix(70)))."
        case .readMail(let a, let unread, let total, let from, let subject):
            if a.isEmpty { return "I have no mail saved on this phone yet." }
            let head = unread > 0 ? "You have \(unread) unread of \(total) in your inbox." : "Nothing unread; \(total) in your inbox."
            return head + " Reading the newest, from \(from.isEmpty ? "an unknown sender" : String(from.prefix(40))): \(String(subject.prefix(70)))."
        }
    }
}

public struct LocalCatalog: Sendable {
    public var books: @Sendable () -> [ShelfDoc]
    public var shows: @Sendable () -> [PodcastShow]
    public var feed: @Sendable (String) -> PodcastFeed?
    public var inbox: @Sendable () -> [MailSummary]
    public init(books: @escaping @Sendable () -> [ShelfDoc] = { [] }, shows: @escaping @Sendable () -> [PodcastShow] = { [] },
                feed: @escaping @Sendable (String) -> PodcastFeed? = { _ in nil }, inbox: @escaping @Sendable () -> [MailSummary] = { [] }) {
        self.books = books; self.shows = shows; self.feed = feed; self.inbox = inbox
    }
    /// The real stores.
    public static func live() -> LocalCatalog {
        LocalCatalog(books: { ReaderShelf.shared.list() }, shows: { PodcastService.shared.subscriptions() }, feed: { PodcastService.shared.cachedFeed($0) },
                     inbox: { ContentStore.shared.mailSummaries(folder: "inbox") })
    }
}

public enum LocalCommands {
    static let bookVerbs: Set<String> = ["read", "continue", "resume", "open", "start", "play"]
    static let bookFiller: Set<String> = ["my", "the", "a", "current", "to", "me", "please", "again", "now", "from", "where", "i", "left", "off", "stopped", "book", "books", "it", "aloud", "out", "loud", "next", "up"]
    static let showFiller: Set<String> = ["the", "latest", "newest", "new", "newer", "last", "most", "recent", "episode", "episodes", "of", "from", "podcast", "show", "my", "please", "a", "an", "to", "me", "radio"]
    static let mailWords: Set<String> = ["mail", "email", "emails", "e-mail", "messages", "message", "inbox"]
    static let mailFiller: Set<String> = ["what", "whats", "what's", "is", "in", "my", "the", "any", "new", "unread", "do", "i", "have", "check", "read", "me", "to", "please", "latest", "newest", "there", "anything", "show", "tell", "about", "got", "get", "is", "are", "latest", "recent", "aloud"]

    static func words(_ text: String) -> [String] {
        SpokenCommand.normalise(text).split(separator: " ").map { $0.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "?!.,")) }.filter { !$0.isEmpty }
    }
    static func titleTokens(_ s: String) -> Set<String> { Set(RetrievalText.tokens(s, keepStops: true)) }

    public static func match(_ text: String, catalog: LocalCatalog) -> LocalCommand? {
        let w = words(text)
        guard w.count >= 2 else { return nil }
        return mail(w, catalog) ?? book(w, catalog) ?? episode(w, catalog)
    }

    // MARK: mail
    static func mail(_ w: [String], _ c: LocalCatalog) -> LocalCommand? {
        guard w.contains(where: { mailWords.contains($0) }), w.allSatisfy({ mailWords.contains($0) || mailFiller.contains($0) }) else { return nil }
        let rows = c.inbox().sorted { ($0.at) > ($1.at) }
        guard let newest = rows.first else { return .readMail(address: "", unread: 0, total: 0, from: "", subject: "") }
        return .readMail(address: "mail:" + newest.id, unread: rows.filter(\.unread).count, total: rows.count, from: newest.from, subject: newest.subject)
    }

    // MARK: book
    static func book(_ w: [String], _ c: LocalCatalog) -> LocalCommand? {
        guard let verb = w.first, bookVerbs.contains(verb) else { return nil }
        let books = c.books()
        guard !books.isEmpty else { return nil }
        let rest = Array(w.dropFirst())
        let sayBook = rest.contains("book") || rest.contains("books")
        // The words that are not filler must all be words of one book's title.
        let rare = rest.filter { !bookFiller.contains($0) }
        var chosen: ShelfDoc?
        if rare.isEmpty {
            guard sayBook else { return nil }
            chosen = books.count == 1 ? books[0] : (books.filter { $0.position > 0 }.max { $0.added < $1.added } ?? books.max { $0.added < $1.added })
        } else {
            let hits = books.filter { b in let t = titleTokens(b.title); return rare.allSatisfy { t.contains(RetrievalText.stem($0)) || t.contains($0) } }
            guard hits.count == 1 else { return nil }
            chosen = hits[0]
        }
        guard let b = chosen else { return nil }
        return .readBook(id: b.id, title: b.title, block: b.position)
    }

    // MARK: podcast
    static func episode(_ w: [String], _ c: LocalCatalog) -> LocalCommand? {
        guard w.first == "play" else { return nil }
        let rare = w.dropFirst().filter { !showFiller.contains($0) }
        guard !rare.isEmpty else { return nil }
        let scored = c.shows().map { s -> (PodcastShow, Int) in
            let t = titleTokens(s.title); return (s, rare.allSatisfy { t.contains(RetrievalText.stem($0)) || t.contains($0) } ? t.count : -1)
        }.filter { $0.1 >= 0 }
        guard let best = scored.min(by: { $0.1 < $1.1 }), scored.filter({ $0.1 == best.1 }).count == 1 else { return nil }
        guard let feed = c.feed(best.0.feedURL), let ep = feed.episodes.filter(\.playable).max(by: { ($0.published ?? 0) < ($1.published ?? 0) }) else { return nil }
        return .playEpisode(address: PodcastService.episodeAddress(feed: best.0.feedURL, id: ep.id), title: best.0.title + ": " + ep.title)
    }
}
