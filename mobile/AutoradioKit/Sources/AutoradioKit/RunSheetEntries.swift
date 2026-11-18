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
    /// True when the entry's words are read from the phone's own store by its address (a message, a book, a file), so the entry carries none: the
    /// rest of a cut passage is then held by the phone (`HeldRest`) rather than made into a story entry with text.
    var wordsComeFromThePhone: Bool { kind == .mail || kind == .document || (source.map { $0.ref.hasPrefix("mail:") || $0.ref.hasPrefix("doc:") || $0.ref.hasPrefix("file:") } ?? false) }
    /// Addresses too long for the server's bound (2048 bytes), kept on the phone by entry id: the entry names the audio by its ref and leaves its url out.
    nonisolated(unsafe) private static var longURLs: [String: URL] = [:]
    private static let longLock = NSLock()
    static func longURL(_ id: String) -> URL? { longLock.lock(); defer { longLock.unlock() }; return longURLs[id] }

    public static func audio(_ a: RunSheetAudio, clipID: String? = nil, by: RunSheetBy = .listener) -> RunSheetEntry {
        let t = cleanTitle(a.title)
        let id = mintId(by == .listener ? "l-" : "p-")
        let long = a.url.absoluteString.utf8.count > RunSheetBounds.maxURLBytes
        if long { longLock.lock(); longURLs[id] = a.url; longLock.unlock() }
        let url: String? = long ? nil : a.url.absoluteString
        if a.end != nil || clipID != nil {
            return RunSheetEntry(id: id, kind: .clip, placedBy: by, title: t,
                                 source: RunSheetSource(ref: "clip:" + (clipID ?? a.episodeId ?? "x"), label: cleanTitle(a.show), url: url, start: a.start, end: a.end, clipId: clipID))
        }
        return RunSheetEntry(id: id, kind: .episode, placedBy: by, title: t,
                             source: RunSheetSource(ref: "episode:" + (a.episodeId ?? (long ? "long" : a.url.absoluteString)), label: cleanTitle(a.show), url: url, start: a.start))
    }

    /// An audio file in Files. The entry names the file by its Files path (`file:<path>`), never by where the phone keeps a copy: that is a device
    /// path and goes nowhere. The phone finds its own copy again from the path (`RunSheetPlayer.fileResolver`).
    public static func filesAudio(address: String, title: String, by: RunSheetBy = .listener) -> RunSheetEntry {
        RunSheetEntry(id: mintId(by == .listener ? "l-" : "p-"), kind: .episode, placedBy: by, title: cleanTitle(title), source: RunSheetSource(ref: address, label: "Files"))
    }
    /// The Files address of an audio file this entry plays.
    public var filesAddress: String? {
        guard kind == .episode || kind == .clip, let r = source?.ref, r.hasPrefix("file:") else { return nil }
        return r
    }

    /// True when a ref the entry names is over its bound: a ref cannot be cut, so such an entry is not placed (the store says so).
    public var refTooLong: Bool { (source?.ref.utf8.count ?? 0) > RunSheetBounds.maxURLBytes }

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
        guard kind == .clip || kind == .episode, let s = source, let u = s.url.flatMap(URL.init(string:)) ?? Self.longURL(id) else { return nil }
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
        case .take: return "take"
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

// MARK: Bounds (the server's, internal/runsheet): the phone builds nothing the server would drop as invalid.

extension RunSheetEntry {
    private static func cut(_ s: String, _ n: Int) -> String { StretchCandidates.cut(s, bytes: n) }
    private static func oneLine(_ s: String) -> String { StretchCandidates.oneLine(s) }

    /// This entry with every string cut at its bound, at a character boundary, every list and number inside its range. A title is cut, never
    /// refused; an id that is too long to cut (an id names a thing) is replaced by a fresh one of the same placer.
    public func bounded() -> RunSheetEntry {
        let B = RunSheetBounds.self
        var e = self
        e.title = Self.cut(Self.oneLine(e.title), B.maxTitleBytes)
        if let t = e.text { let c = Self.cut(t, B.maxTextBytes); e.text = c.isEmpty ? nil : c }
        e.estSeconds = min(max(e.estSeconds.isFinite ? e.estSeconds : 0, 0), B.maxEstSeconds)
        e.revision = max(e.revision, 0)
        if let a = e.airAt { e.airAt = Self.cut(a, B.maxTimeBytes) }
        if let o = e.origin { e.origin = Self.cut(o, B.maxShortBytes) }
        if var s = e.source {
            // A ref names the thing (a route, an address): it is never cut (the store refuses an entry whose ref is over its bound). A url is only
            // the way to fetch it: over the bound it is left out and the ref stays.
            s.label = Self.cut(Self.oneLine(s.label), B.maxRefBytes)
            if let u = s.url, u.utf8.count > B.maxURLBytes { s.url = nil }
            s.clipId = s.clipId.map { Self.cut($0, B.maxRefBytes) }
            if let v = s.start, v < 0 { s.start = 0 }
            if let v = s.end, v < 0 { s.end = 0 }
            e.source = s
        }
        if let cs = e.cites {
            let kept = cs.filter { $0.ref.utf8.count <= B.maxURLBytes }.prefix(B.maxCites).map { c -> RunSheetCite in var c = c; c.label = Self.cut(Self.oneLine(c.label), B.maxRefBytes); return c }
            e.cites = kept.isEmpty ? nil : Array(kept)
        }
        if let b = e.between, b.after.utf8.count > B.maxIDBytes || b.before.utf8.count > B.maxIDBytes { e.between = nil }       // written for neighbours it cannot name
        if var f = e.forecast {
            f.pHeard = f.pHeard.map { min(max($0.isFinite ? $0 : 0, 0), 1) }; f.causedBy = Self.cut(f.causedBy, B.maxShortBytes); f.claimId = Self.cut(f.claimId, B.maxShortBytes)
            e.forecast = f
        }
        if let en = e.ended { e.ended = en.bounded() }
        if !RunSheet.validId(e.id) { e.id = Self.mintId(e.placedBy == .listener ? "l-" : (e.placedBy == .desk ? "d-" : "p-")) }
        // The 32 KB bound is of the carried form (a quote costs 4, `<` 6): words are taken from the end of the text until the entry fits.
        while RunSheet.entryJSONSize(e) > B.maxEntryJSON, let t = e.text, t.count > 1 { e.text = Self.cut(String(t.prefix(t.count * 3 / 4)), B.maxTextBytes) }
        return e
    }
}

extension RunSheetEnded {
    func bounded() -> RunSheetEnded {
        var e = self
        e.at = StretchCandidates.cut(e.at, bytes: RunSheetBounds.maxTimeBytes); e.lastBlock = max(e.lastBlock, 0)
        return e
    }
}

extension RunSheetOp {
    /// This op with every field cut or clamped to the server's bounds (its entry too), so the server never drops it as invalid.
    public func bounded() -> RunSheetOp {
        let B = RunSheetBounds.self
        var o = self
        o.entry = o.op == .place ? o.entry?.bounded() : nil           // an op carries nothing it does not use
        if o.op != .rewrite { o.text = nil } else if let t = o.text { o.text = StretchCandidates.cut(t, bytes: B.maxTextBytes) }
        o.ended = o.ended?.bounded()
        if let r = o.reason { o.reason = StretchCandidates.cut(r, bytes: B.maxReasonBytes) }
        if let a = o.after, a.utf8.count > B.maxIDBytes { o.after = nil; if o.at == .after { o.at = .next } }
        if let g = o.origin { o.origin = StretchCandidates.cut(g, bytes: B.maxShortBytes) }
        if let p = o.positionSeconds { o.positionSeconds = min(max(p.isFinite ? p : 0, 0), B.maxEstSeconds) }
        return o
    }
}
