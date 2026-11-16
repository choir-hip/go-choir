import Foundation

// What each thing the listener can play is, as an entry on the run sheet (slice B). There is one list of what is heard; these are the
// entries the phone and the listener place on it. The desk's entries come through `RunSheetFeed`.
//
//   a paper story read in full      kind page      source.ref pub:<route>
//   a web page                      kind page      source.ref url:<address>
//   a page the listener heard read  kind page|mail source.ref <the page's own address>; the words are held by the player (the entry has none)
//   a book or a PDF                 kind document  source.ref doc:<id>, source.start the block it is read from
//   a podcast clip                  kind clip      source.ref clip:<id>, source.url the audio, source.start and end its range
//   a podcast episode               kind episode   source.ref episode:<podcast key>, source.url the audio, source.start where it begins
//
// `doc:` is not in the frozen interface's list of refs; the reducer does not read refs, and the server's list is the thing to extend when
// it validates them.

/// The audio an entry plays: what the old queue called `PlayItem.audio`.
public struct RunSheetAudio: Equatable, Sendable {
    public var url: URL
    public var title: String
    public var show: String
    public var start: Double?
    public var end: Double?
    public var episodeId: String?
    public var isClip: Bool { end != nil }
    public init(url: URL, title: String, show: String, start: Double? = nil, end: Double? = nil, episodeId: String? = nil) {
        self.url = url; self.title = title; self.show = show; self.start = start; self.end = end; self.episodeId = episodeId
    }
}

extension RunSheetEntry {
    /// `p-` (phone), `l-` (listener) or `d-` plus eight hex characters: placer-minted, never reused.
    public static func mintId(_ prefix: String) -> String { prefix + UUID().uuidString.replacingOccurrences(of: "-", with: "").prefix(8).lowercased() }

    /// One clean title: one line, at most 300 bytes at a character boundary.
    public static func cleanTitle(_ s: String) -> String { StretchCandidates.cut(StretchCandidates.oneLine(s), bytes: RunSheetBounds.maxTitleBytes) }

    public static func paperStory(route: String, title: String, by: RunSheetBy = .listener) -> RunSheetEntry {
        RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .page, placedBy: by, title: cleanTitle(title), source: RunSheetSource(ref: "pub:" + route, label: cleanTitle(title)))
    }
    public static func webPage(url: URL, title: String, by: RunSheetBy = .listener) -> RunSheetEntry {
        let t = cleanTitle(title.isEmpty ? (url.host ?? url.absoluteString) : title)
        return RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .page, placedBy: by, title: t, source: RunSheetSource(ref: "url:" + url.absoluteString, label: url.host ?? t, url: url.absoluteString))
    }
    public static func book(id: String, title: String, at: Int, by: RunSheetBy = .listener) -> RunSheetEntry {
        RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .document, placedBy: by, title: cleanTitle(title), source: RunSheetSource(ref: "doc:" + id, label: cleanTitle(title), start: Double(at)))
    }
    /// A page's words, read aloud on the listener's asking (Listen on a page, "Read it to me" on a message): the address says what it is.
    public static func readAloud(address: String, title: String, by: RunSheetBy = .listener) -> RunSheetEntry {
        let kind: RunSheetKind = address.hasPrefix("mail:") ? .mail : .page
        return RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: kind, placedBy: by, title: cleanTitle(title), source: RunSheetSource(ref: address, label: cleanTitle(title)))
    }
    public static func audio(_ a: RunSheetAudio, clipID: String? = nil, by: RunSheetBy = .listener) -> RunSheetEntry {
        let t = cleanTitle(a.title)
        if a.end != nil || clipID != nil {
            return RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .clip, placedBy: by, title: t,
                                 source: RunSheetSource(ref: "clip:" + (clipID ?? a.episodeId ?? "x"), label: cleanTitle(a.show), url: a.url.absoluteString, start: a.start, end: a.end, clipId: clipID))
        }
        return RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .episode, placedBy: by, title: t,
                             source: RunSheetSource(ref: "episode:" + (a.episodeId ?? a.url.absoluteString), label: cleanTitle(a.show), url: a.url.absoluteString, start: a.start))
    }

    /// The route of a paper story this entry reads in full.
    public var paperRoute: String? {
        guard kind == .page, let r = source?.ref, r.hasPrefix("pub:") else { return nil }
        return String(r.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first ?? "")
    }
    public var webURL: URL? {
        guard kind == .page, let r = source?.ref, r.hasPrefix("url:") else { return nil }
        return URL(string: String(r.dropFirst(4)))
    }
    public var book: (id: String, at: Int)? {
        guard kind == .document, let r = source?.ref, r.hasPrefix("doc:") else { return nil }
        return (String(r.dropFirst(4)), Int(source?.start ?? 0))
    }
    public var audioSpec: RunSheetAudio? {
        guard kind == .clip || kind == .episode, let s = source, let u = s.url.flatMap(URL.init(string:)) else { return nil }
        var ep: String?
        if kind == .episode, s.ref.hasPrefix("episode:") { ep = String(s.ref.dropFirst(8)) }
        return RunSheetAudio(url: u, title: title, show: s.label, start: s.start, end: s.end, episodeId: ep)
    }
    /// What the Up next row calls it.
    public var kindWord: String {
        switch kind {
        case .opening: return "opening"
        case .transition: return "transition"
        case .story: return "story"
        case .clip: return "clip"
        case .episode: return "audio"
        case .page: return paperRoute != nil ? "story" : "page"
        case .document: return "book"
        case .mail: return "mail"
        case .result: return "result"
        case .answer: return "answer"
        }
    }
    /// A title always: the real title, else what the entry points at.
    public var displayTitle: String {
        let t = title.trimmingCharacters(in: .whitespacesAndNewlines)
        if !t.isEmpty { return t }
        if let h = webURL?.host { return h }
        if let l = source?.label, !l.isEmpty { return l }
        return kindWord.capitalized
    }
    /// Who put it there, as a sentence.
    public var byWords: String {
        switch placedBy {
        case .listener: return "Added by you"
        case .desk: return "From the desk"
        case .phone: return "From the phone"
        }
    }
    /// A rough time to speak it, from the words (2.6 a second).
    public static func estimateSeconds(words text: String) -> Double { max(1, (Double(text.split(whereSeparator: { $0.isWhitespace }).count) / BarTracker.wordsPerSecond).rounded()) }
}
