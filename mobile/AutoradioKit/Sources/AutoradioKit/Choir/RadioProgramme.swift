import Foundation

// The programme: a stretch of segments instead of one article (station P1).
// Design: docs/design/autoradio-dj-programme-2026-11-11.md sections 1.3, 2.2 and 6; vocabulary (blocks, last_block):
// docs/design/autoradio-learning-engine-2026-11-14.md 12.3. The failure list is in mobile/README.md ("Programme player (station P1)");
// the tests are ProgrammePlayerTests.swift and carry its numbers.
//
// What the phone does with a stretch: read its lines (StretchDecoder), make each segment into playback blocks (SegmentPrep), append
// them to the document the player is playing (ProgrammeDocument.append, ProgrammePlayer.appendSegment), and tell observers where
// playback is (SegmentTracker, in its own file). The server validates; the phone drops only what it cannot speak.

// MARK: Segments

/// One thing a segment names, as the server sent it. `label` is what the screen may show; a ref is never shown.
public struct ProgrammeCite: Codable, Sendable, Equatable {
    public var ref: String
    public var role: String
    public var label: String
    public init(ref: String, role: String = "", label: String = "") { self.ref = ref; self.role = role; self.label = label }
}

/// The clip a `podcast_clip` segment plays (the stream's `clip`): the id is looked up in the set the phone fetched.
public struct ProgrammeClipRef: Codable, Sendable, Equatable {
    public var clipID: String
    public var storyRef: String
    public var seconds: Double
    public init(clipID: String, storyRef: String = "", seconds: Double = 0) { self.clipID = clipID; self.storyRef = storyRef; self.seconds = seconds }
}

/// A range of a ProgrammeDocument's blocks that the DJ wrote as one segment.
public struct ProgrammeSegment: Codable, Sendable, Equatable {
    public var id: String
    public var stretchID: String
    public var kind: String
    public var title: String
    public var cites: [ProgrammeCite]
    /// The playback blocks this segment owns: `blocks[firstBlock ..< firstBlock + blockCount]`.
    public var firstBlock: Int
    public var blockCount: Int
    /// The server's count of the segment's blocks (`block_count`, or the phone's own split when none was sent): the count that is reported.
    public var serverBlockCount: Int
    /// The server sent a count that the phone's split of the text does not match.
    public var blockCountMismatch: Bool
    /// For each playback block, the index of the server block it was made from (the `last_block` to report), inside `serverBlockCount`.
    public var unitServerBlocks: [Int]
    /// A `podcast_clip` segment: the lead-in blocks, then one clip block.
    public var clip: ProgrammeClipRef?

    public init(id: String, stretchID: String, kind: String, title: String, cites: [ProgrammeCite], firstBlock: Int, blockCount: Int,
                serverBlockCount: Int, blockCountMismatch: Bool, unitServerBlocks: [Int], clip: ProgrammeClipRef? = nil) {
        self.clip = clip
        self.id = id; self.stretchID = stretchID; self.kind = kind; self.title = title; self.cites = cites; self.firstBlock = firstBlock
        self.blockCount = blockCount; self.serverBlockCount = serverBlockCount; self.blockCountMismatch = blockCountMismatch
        self.unitServerBlocks = unitServerBlocks
    }

    /// The labels of the cites that have one, each once, in order: the chips on screen.
    public var sources: [String] {
        var out: [String] = []
        for c in cites where !c.label.isEmpty && !out.contains(c.label) { out.append(c.label) }
        return out
    }
    public var kindLabel: String { SegmentKinds.label(kind) }
}

public enum SegmentKinds {
    static let labels: [String: String] = [
        "story": "Story", "headlines": "Headlines", "update": "Update", "mail": "Mail", "podcast_note": "Podcast",
        "answer": "Answer", "result": "Result", "bridge": "Bridge", "ident": "Station break", "podcast_clip": "Podcast clip",
        "opening": "Opening", "transition": "Transition", "take": "Take",
    ]
    public static func isKnown(_ kind: String) -> Bool { labels[kind] != nil }
    public static func label(_ kind: String) -> String { labels[kind] ?? "" }
}

public struct IncomingBetween: Sendable, Equatable {
    public var after: String, before: String
    public init(after: String, before: String) { self.after = after; self.before = before }
}

/// A segment as it arrived: text not yet made into blocks.
public struct IncomingSegment: Sendable, Equatable {
    public var stretchID: String
    public var segmentID: String
    public var kind: String
    public var title: String
    public var text: String
    public var cites: [ProgrammeCite]
    /// The server's `block_count`, when sent.
    public var blockCount: Int?
    /// A `podcast_clip` segment's clip.
    public var clip: ProgrammeClipRef?
    /// A `transition`: the `segment_id`s of the two segments it joins.
    public var between: IncomingBetween?
    /// A `take` segment's take id, and the forecast the line carried (a take line without one is dropped by the server).
    public var takeID: String?
    public var forecast: RunSheetForecast?
    /// The server's entry id for this segment (`d-<segment_id>`) and its forecast's claim id, from the `segment` event (settlement slice, not
    /// deployed on every server: nil from an older one). The claim id is the key the server settles by.
    public var entryID: String?
    public var claimID: String?
    public init(stretchID: String, segmentID: String, kind: String, title: String, text: String, cites: [ProgrammeCite], blockCount: Int?, clip: ProgrammeClipRef? = nil,
                between: IncomingBetween? = nil, takeID: String? = nil, forecast: RunSheetForecast? = nil, entryID: String? = nil, claimID: String? = nil) {
        self.entryID = entryID; self.claimID = claimID
        self.clip = clip; self.between = between; self.takeID = takeID; self.forecast = forecast
        self.stretchID = stretchID; self.segmentID = segmentID; self.kind = kind; self.title = title; self.text = text
        self.cites = cites; self.blockCount = blockCount
    }
}

/// A segment ready to append: its range is relative (`firstBlock` is 0) until the document numbers it.
public struct PreparedSegment: Sendable, Equatable {
    public var segment: ProgrammeSegment
    public var blocks: [ProgrammeBlock]
}

/// The server's `SplitBlocks` (learning engine design 12.3), mirrored: line endings to LF (a lone CR is one too), split on one or more
/// blank lines (a line of only whitespace is blank), trim each part, drop empty parts.
/// It works on Unicode scalars, as Go's strings do, not on Swift's characters: a space with a combining mark after it is one character
/// to Swift (and "whitespace"), but to Go it is a space and a mark, and the line is not blank. The outputs in blocks.json and
/// blocks_unicode.json are what the Go function returned.
public enum SplitBlocks {
    /// Go's `unicode.IsSpace` (what `strings.TrimSpace` trims).
    static func isSpace(_ u: Unicode.Scalar) -> Bool {
        switch u.value {
        case 0x09...0x0D, 0x20, 0x85, 0xA0, 0x1680, 0x2000...0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000: return true
        default: return false
        }
    }
    static func trimmed(_ s: [Unicode.Scalar]) -> [Unicode.Scalar] {
        var a = s[...]
        while let f = a.first, isSpace(f) { a = a.dropFirst() }
        while let l = a.last, isSpace(l) { a = a.dropLast() }
        return Array(a)
    }
    public static func split(_ text: String) -> [String] {
        // line endings to LF: CRLF and a lone CR
        var lf: [Unicode.Scalar] = []
        var prevCR = false
        for u in text.unicodeScalars {
            if u == "\n" { if !prevCR { lf.append("\n") }; prevCR = false }
            else if u == "\r" { lf.append("\n"); prevCR = true }
            else { lf.append(u); prevCR = false }
        }
        var out: [String] = [], cur: [[Unicode.Scalar]] = []
        func flush() {
            var joined: [Unicode.Scalar] = []
            for (i, l) in cur.enumerated() { if i > 0 { joined.append("\n") }; joined += l }
            let part = trimmed(joined)
            if !part.isEmpty { var v = String.UnicodeScalarView(); v.append(contentsOf: part); out.append(String(v)) }
            cur = []
        }
        var line: [Unicode.Scalar] = []
        func endLine() {
            if line.allSatisfy(isSpace) { flush() } else { cur.append(line) }
            line = []
        }
        for u in lf { if u == "\n" { endLine() } else { line.append(u) } }
        endLine()
        flush()
        return out
    }
}

public enum SegmentPrep {
    /// Longest title, in bytes (the server's own bound, 12.7).
    static let maxTitleBytes = 200
    static let stripped: Set<UInt32> = Set((0x202A...0x202E).map { UInt32($0) } + (0x2066...0x2069).map { UInt32($0) } + [0x200E, 0x200F, 0x061C, 0x200B, 0xFEFF])

    /// One clean line: controls and newlines become spaces, runs of space collapse, capped in bytes at a character boundary.
    static func oneLine(_ s: String, maxBytes: Int = maxTitleBytes) -> String {
        // Controls (and newlines) become spaces; format characters (zero-width, bidi overrides) are dropped.
        var scalars = String.UnicodeScalarView()
        for u in s.unicodeScalars {
            switch u.properties.generalCategory {
            case .control: scalars.append(" ")
            default:
                // Only what changes how text is displayed or hides it: bidi controls, the zero-width space, the byte-order mark. Joiners
                // (ZWJ, ZWNJ) are part of the text (emoji sequences, Persian, Indic) and stay.
                if Self.stripped.contains(u.value) { continue }
                scalars.append(u)
            }
        }
        var t = String(scalars)
        t = t.replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression).trimmingCharacters(in: .whitespaces)
        while t.utf8.count > maxBytes { t.removeLast() }
        return t.trimmingCharacters(in: .whitespaces)
    }
    /// A stranger's longer text: bidi and zero-width characters dropped, controls other than a newline become spaces; the lines stay.
    static func stripUnsafe(_ s: String) -> String {
        var scalars = String.UnicodeScalarView()
        for u in s.unicodeScalars {
            if u == "\n" { scalars.append(u); continue }
            if u.properties.generalCategory == .control { scalars.append(" "); continue }
            if Self.stripped.contains(u.value) { continue }
            scalars.append(u)
        }
        return String(scalars)
    }
    /// `pub:harbour-1`, `mail:abc`: a ref, which is never shown to the listener. Known limit (review round 1, left as is): a real one-word
    /// title with a colon and no space, such as "Note:important", reads as a ref and becomes the kind's plain label.
    static func looksLikeRef(_ s: String) -> Bool {
        s.range(of: "^[a-z][a-z_]{1,11}:\\S+$", options: .regularExpression) != nil
    }
    public static func title(_ raw: String, kind: String) -> String {
        let t = oneLine(raw)
        // A bridge has no name of its own to show: the programme's, never the word "Bridge".
        return t.isEmpty || looksLikeRef(t) ? (kind == "bridge" || kind == "transition" ? "Autoradio" : SegmentKinds.label(kind)) : t
    }
    static func label(_ raw: String) -> String {
        let t = oneLine(raw)
        return looksLikeRef(t) ? "" : t
    }

    /// The segment and its playback blocks; nil when there is nothing to say.
    public static func prepare(_ s: IncomingSegment) -> PreparedSegment? {
        let server = SplitBlocks.split(s.text)
        guard !server.isEmpty else { return nil }
        var units: [ProgrammeBlock] = [], map: [Int] = []
        let reported = (s.blockCount ?? 0) > 0 ? s.blockCount! : server.count
        for (i, b) in server.enumerated() {
            // A single newline inside a paragraph is not a pause; a long paragraph is spoken as several blocks of the one server block.
            let flat = b.components(separatedBy: .newlines).map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }.joined(separator: " ")
            for part in ChoirText.split(flat, maxChars: ChoirText.speechChars) where !part.isEmpty {
                units.append(.speech(text: part)); map.append(min(i, reported - 1))
            }
        }
        guard !units.isEmpty else { return nil }
        if s.kind == "podcast_clip" {
            guard let c = s.clip, !c.clipID.isEmpty else { return nil }          // a clip line with no clip is not a segment
            units.append(.podcastClip(clipID: c.clipID)); map.append(max(0, reported - 1))
        }
        var cites: [ProgrammeCite] = []
        for c in s.cites {
            let l = label(c.label)
            if let k = cites.firstIndex(where: { $0.ref == c.ref }) { if cites[k].label.isEmpty { cites[k].label = l }; continue }
            cites.append(ProgrammeCite(ref: c.ref, role: c.role, label: l))
        }
        let seg = ProgrammeSegment(id: s.segmentID, stretchID: s.stretchID, kind: s.kind, title: title(s.title, kind: s.kind), cites: cites,
                                   firstBlock: 0, blockCount: units.count, serverBlockCount: reported,
                                   blockCountMismatch: (s.blockCount ?? 0) > 0 && s.blockCount! != server.count, unitServerBlocks: map, clip: s.clip)
        return PreparedSegment(segment: seg, blocks: units)
    }
}

extension ProgrammeDocument {
    /// Add a segment at the end. False (nothing changed) when a segment with that id is already here.
    @discardableResult
    public mutating func append(_ p: PreparedSegment) -> Bool {
        guard !segments.contains(where: { $0.id == p.segment.id }) else { return false }
        var s = p.segment
        s.firstBlock = blocks.count
        blocks += p.blocks
        segments.append(s)
        return true
    }
    /// The index of the segment that owns block `i`; nil for a block outside every segment (a single article has none).
    public func segmentIndex(containingBlock i: Int) -> Int? {
        segments.firstIndex { i >= $0.firstBlock && i < $0.firstBlock + $0.blockCount }
    }
}

/// What Now Playing, the lock screen and the player's header show for a block.
public enum ProgrammeNowPlaying {
    public struct Info: Equatable, Sendable { public var title: String, artist: String, kind: String, sources: [String] }
    public static func describe(document d: ProgrammeDocument, blockIndex i: Int) -> Info {
        if let si = d.segmentIndex(containingBlock: i) {
            let s = d.segments[si]
            return Info(title: s.title, artist: s.kindLabel.isEmpty ? "Autoradio" : "Autoradio · \(s.kindLabel)", kind: s.kindLabel, sources: s.sources)
        }
        return Info(title: d.blocks.indices.contains(i) ? d.blocks[i].summary : "", artist: d.title.isEmpty ? "Autoradio" : d.title, kind: "", sources: [])
    }
}

// MARK: The stream

public enum StretchEvent: Sendable, Equatable {
    case segment(IncomingSegment)
    case done
    case error(String)
}

/// Reads a stretch as it arrives: server-sent events (`event:` and `data:` lines, the programme route) or one JSON object per line (the
/// fixtures). Chunks may end anywhere. Line endings are LF, CRLF or a lone CR. Anything it cannot use is counted and skipped.
public struct StretchDecoder {
    public private(set) var ignoredCount = 0
    public private(set) var droppedCount = 0
    private var line = ""
    private var afterCR = false
    private var sse = SSEParser()
    private var stretchID = "local"
    private var seen = 0
    public init() {}

    public mutating func feed(_ chunk: String) -> [StretchEvent] {
        var out: [StretchEvent] = []
        for u in chunk.unicodeScalars {
            if u == "\n" {
                if afterCR { afterCR = false; continue }                  // the LF of a CRLF whose CR already ended the line
                out += endLine()
            } else if u == "\r" {
                out += endLine(); afterCR = true
            } else {
                afterCR = false; line.unicodeScalars.append(u)
            }
        }
        return out
    }
    /// The stream ended: a last line with no line ending still counts.
    public mutating func finish() -> [StretchEvent] {
        afterCR = false
        var out = line.isEmpty ? [] : endLine()
        out += flushPending()
        return out
    }

    /// An `event:` line that no `data:` line followed (a bare `event: done`): it is dispatched when the next line that is not a comment or
    /// its data arrives, or when the stream ends. (AsyncLineSequence drops the blank line, so the blank line cannot be waited for.)
    private var pendingEvent: String?
    private mutating func flushPending() -> [StretchEvent] {
        guard let p = pendingEvent else { return [] }
        pendingEvent = nil
        return dispatch(p, [:])
    }

    private mutating func endLine() -> [StretchEvent] {
        let l = line; line = ""
        if l.hasPrefix(":") { return [] }                                    // a comment says nothing about the event in progress
        var out: [StretchEvent] = []
        if !l.hasPrefix("data:") { out = flushPending() }
        let rest = process(l)
        if l.hasPrefix("event:") { pendingEvent = l.dropFirst(6).trimmingCharacters(in: .whitespaces) }
        else if l.hasPrefix("data:") { pendingEvent = nil }
        return out + rest
    }

    private mutating func process(_ l: String) -> [StretchEvent] {
        if l.trimmingCharacters(in: .whitespaces).isEmpty { _ = sse.push(""); return [] }
        if l.hasPrefix("{") {
            guard let o = Self.object(l) else { ignoredCount += 1; return [] }
            return dispatch((o["type"] as? String) ?? (o["event"] as? String) ?? "segment", o)
        }
        if let e = sse.push(l) {
            guard let o = Self.object(e.data) else { ignoredCount += 1; return [] }
            return dispatch(e.name == "message" ? "segment" : e.name, o)
        }
        if l.hasPrefix(":") || l.hasPrefix("event:") || l.hasPrefix("id:") || l.hasPrefix("retry:") { return [] }
        ignoredCount += 1                                                    // not JSON, not SSE
        return []
    }

    /// A JSON number that is a whole number: not a bool, not a fraction, not a string.
    static func wholeNumber(_ v: Any?) -> Int? {
        guard let n = v as? NSNumber, CFGetTypeID(n) != CFBooleanGetTypeID() else { return nil }
        let d = n.doubleValue
        return d.isFinite && d == d.rounded() && abs(d) < 1e9 ? Int(d) : nil
    }
    private static func object(_ s: String) -> [String: Any]? {
        (try? JSONSerialization.jsonObject(with: Data(s.utf8))) as? [String: Any]
    }

    private mutating func dispatch(_ name: String, _ o: [String: Any]) -> [StretchEvent] {
        switch name {
        case "segment":
            let n = seen; seen += 1
            let kind = (o["kind"] as? String) ?? ""
            let text = (o["text"] as? String) ?? ""
            guard SegmentKinds.isKnown(kind), !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { droppedCount += 1; return [] }
            var clip: ProgrammeClipRef?
            if let c = o["clip"] as? [String: Any], let id = c["clip_id"] as? String, !id.isEmpty {
                clip = ProgrammeClipRef(clipID: id, storyRef: c["story_ref"] as? String ?? "", seconds: (c["seconds"] as? NSNumber)?.doubleValue ?? 0)
            }
            if kind == "podcast_clip", clip == nil { droppedCount += 1; return [] }
            // ASSUMED wire: `take` is the id, or an object with `take_id`; the forecast is `forecast{p_heard,caused_by,claim_id}` or the line's own flat fields.
            var takeID: String?
            if let t = o["take"] as? String { takeID = t } else if let t = o["take"] as? [String: Any] { takeID = t["take_id"] as? String }
            if kind == "take", takeID.map({ Take.validID($0) }) != true { droppedCount += 1; return [] }
            var forecast: RunSheetForecast?
            let fo = (o["forecast"] as? [String: Any]) ?? o
            if let p = (fo["p_heard"] as? NSNumber)?.doubleValue, p.isFinite, p >= 0, p <= 1 {
                forecast = RunSheetForecast(pHeard: p, causedBy: (fo["caused_by"] as? String) ?? "", claimId: (fo["claim_id"] as? String) ?? (o["claim_id"] as? String) ?? "")
            }
            // ASSUMED wire (learn-settle 64e1b34c2): the event carries `entry_id` and, for a content segment, `claim_id`, flat on the event.
            let entryID = (o["entry_id"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            let claimID = ((o["claim_id"] as? String) ?? ((o["forecast"] as? [String: Any])?["claim_id"] as? String)).flatMap { $0.isEmpty ? nil : $0 }
            if kind == "take", forecast == nil { droppedCount += 1; return [] }
            let st = (o["stretch_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? stretchID
            let id = (o["segment_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "\(st)/\(n)"
            var cites: [ProgrammeCite] = []
            for c in (o["cites"] as? [Any]) ?? [] {
                if let r = c as? String, !r.isEmpty { cites.append(ProgrammeCite(ref: r)) }
                else if let d = c as? [String: Any], let r = d["ref"] as? String, !r.isEmpty {
                    cites.append(ProgrammeCite(ref: r, role: d["role"] as? String ?? "", label: d["label"] as? String ?? ""))
                }
            }
            var between: IncomingBetween?
            if let b = o["between"] as? [String: Any], let a = (b["after_segment"] ?? b["after"]) as? String, let z = (b["before_segment"] ?? b["before"]) as? String, !a.isEmpty, !z.isEmpty {
                between = IncomingBetween(after: a, before: z)
            }
            return [.segment(IncomingSegment(stretchID: st, segmentID: id, kind: kind, title: (o["title"] as? String) ?? "", text: text, cites: cites,
                                             blockCount: Self.wholeNumber(o["block_count"]), clip: kind == "podcast_clip" ? clip : nil, between: between,
                                             takeID: kind == "take" ? takeID : nil, forecast: forecast, entryID: entryID, claimID: claimID))]
        case "stretch":
            if let id = o["stretch_id"] as? String, !id.isEmpty { stretchID = id }
            return []
        case "done": return [.done]
        case "error":
            let reason = o["reason"] as? String ?? "", spoken = o["spoken"] as? String ?? ""
            return [.error(reason.isEmpty ? (spoken.isEmpty ? "the stretch failed" : spoken) : (spoken.isEmpty ? reason : "\(reason): \(spoken)"))]
        default:
            ignoredCount += 1                                                // handoff (reserved) and anything unknown
            return []
        }
    }
}

// MARK: Where a stretch comes from

public protocol StretchSource: Sendable {
    /// Throws before any event when there is no stretch to be had (a missing route, no sign-in, no connection, no story to tell).
    func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error>
    /// As `events(cause:)`, leaving the stories at these routes out (an `extend` request: they were already told).
    func events(cause: String, excludingRoutes: Set<String>) async throws -> AsyncThrowingStream<StretchEvent, Error>
    func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?) async throws -> AsyncThrowingStream<StretchEvent, Error>
    /// As above, with the titles of the entries ahead on the run sheet (`ahead_titles`, at most 20), so the desk does not repeat them.
    func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?, aheadTitles: [String]) async throws -> AsyncThrowingStream<StretchEvent, Error>
}
extension StretchSource {
    public func events(cause: String, excludingRoutes: Set<String>) async throws -> AsyncThrowingStream<StretchEvent, Error> { try await events(cause: cause) }
    /// As `events(cause:excludingRoutes:)`, with the candidates limited to these routes in this order (Play hands the list to the DJ).
    /// A source that does not know the scope ignores it.
    public func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        try await events(cause: cause, excludingRoutes: excludingRoutes)
    }
    public func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?, aheadTitles: [String]) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        try await events(cause: cause, excludingRoutes: excludingRoutes, onlyRoutes: onlyRoutes)
    }
}

/// `POST /api/autoradio/programme` on the listener's computer (design 2.2). Until station L1 is deployed it answers 404 or 503, which is
/// "no stretch": the front page plays instead.
public struct RouteStretchSource: StretchSource {
    let baseURL: String
    public init(baseURL: String) { self.baseURL = baseURL }
    /// How long the texts of the first ten stories may take to arrive (plain GETs, not model calls); a story whose text is late goes without it.
    static let textDeadline: TimeInterval = 3

    public func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { try await events(cause: cause, excludingRoutes: [], onlyRoutes: nil) }
    public func events(cause: String, excludingRoutes: Set<String>) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        try await events(cause: cause, excludingRoutes: excludingRoutes, onlyRoutes: nil)
    }

    /// The candidates: the front list in the paper's order, and the text of the first ten. Throws `empty` when there is no story to tell: a
    /// request with no candidates is answered with station breaks only, so it is not made.
    func candidates(client: ChoirClient, skipping: Set<String>, onlyRoutes: [String]? = nil, clips: ClipStore = .shared) async throws -> [[String: Any]] {
        let r = try await client.send("GET", "/api/platform/publications/front", query: ["limit": "60"], authenticated: false)
        let front = try r.object()
        var items = StretchCandidates.fromFront(front, skipping: skipping, onlyRoutes: onlyRoutes)
        // The list handed over is told out (or none of it is in the front): the radio goes on with the front, it does not stop.
        if items.isEmpty, onlyRoutes != nil { items = StretchCandidates.fromFront(front, skipping: skipping) }
        // Every story on the front was told already: the desk is still asked (it may have an update, or a different angle) with the whole list.
        if items.isEmpty, !skipping.isEmpty { items = StretchCandidates.fromFront(front, onlyRoutes: onlyRoutes) }
        if items.isEmpty { throw ChoirError.empty("no story to tell") }
        enum Got: Sendable { case text(String, String), none, late, clips }
        let routes = items.prefix(StretchCandidates.maxTextItems).map(\.route)
        var texts: [String: String] = [:]
        let deadline = Self.textDeadline
        await withTaskGroup(of: Got.self) { g in
            for route in routes {
                g.addTask {
                    guard let t = try? await ChoirFront.resolveText(route: route, client: client), !t.isEmpty else { return .none }
                    return .text(route, t)
                }
            }
            // The clips: ONE bulk request for the whole list (cached for the session), never one per story.
            g.addTask { _ = await PodcastClips.ensure(client: client, store: clips); return .clips }
            g.addTask { try? await Task.sleep(nanoseconds: UInt64(deadline * 1_000_000_000)); return .late }
            var pending = routes.count + 1
            loop: for await got in g {
                switch got {
                case .text(let route, let t): texts[route] = t; pending -= 1
                case .none, .clips: pending -= 1
                case .late: break loop
                }
                if pending <= 0 { break loop }
            }
            g.cancelAll()
        }
        let stories = StretchCandidates.body(items, texts: texts, clips: clips.candidateClips(forRoutes: items.map(\.route)))
        // The takes: ONE list request (signed-in only; a failure or an empty list leaves the request as it was). Their audio address stays here.
        let takes = await Self.takeCandidates(client: client)
        return TakeCandidates.merged(stories: stories, takes: takes)
    }

    /// The takes the request may offer: the public list minus the listener's own and the muted publishers', at most 3.
    static func takeCandidates(client: ChoirClient, own: TakeOwn = .shared, mutes: TakeMutes = .shared, store: TakeStore = .shared) async -> [Take] {
        guard client.hasKey, let all = try? await TakesClient(client: client).list() else { return [] }
        store.replace(all)
        let picked = TakeCandidates.select(all, own: own, mutes: mutes)
        print("AUTORADIO takes listed=\(all.count) offered=\(picked.count)")
        return picked
    }

    public func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        try await events(cause: cause, excludingRoutes: excludingRoutes, onlyRoutes: onlyRoutes, aheadTitles: [])
    }
    /// The request body. `edition_text` is what the phone knows about the listener: the notebook text when it holds it, else the topics as text.
    static func requestBody(cause: String, candidates: [[String: Any]], aheadTitles: [String], editionText: String?, now: Date) -> [String: Any] {
        let f = DateFormatter(); f.locale = Locale(identifier: "en_US_POSIX"); f.dateFormat = "yyyy-MM-dd'T'HH:mm:ssXXXXX"
        var body: [String: Any] = ["request_id": "radio-" + UUID().uuidString.prefix(12).lowercased(), "cause": cause,
                                   "local_time": f.string(from: now), "tz": TimeZone.current.identifier, "candidates": candidates]
        let titles = aheadTitles.prefix(20).map { StretchCandidates.cut($0, bytes: 300) }.filter { !$0.isEmpty }
        if !titles.isEmpty { body["ahead_titles"] = Array(titles) }
        if let t = editionText, !t.isEmpty { body["edition_text"] = t }
        return body
    }
    public func events(cause: String, excludingRoutes: Set<String>, onlyRoutes: [String]?, aheadTitles: [String]) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        let client = ChoirEnvironment.shared.client(baseURL: baseURL)
        let cands = try await candidates(client: client, skipping: excludingRoutes, onlyRoutes: onlyRoutes)
        var body = Self.requestBody(cause: cause, candidates: cands, aheadTitles: aheadTitles, editionText: EditionText.current(), now: Date())
        // `ahead_titles` is new on the route; a server that does not know it refuses the whole body (400) before any model call. That is a
        // correctness refusal, so the request is made once more without it. Nothing else changes between the two.
        let lines: AsyncLineSequence<URLSession.AsyncBytes>
        do { (_, lines) = try await client.postEventStream("/api/autoradio/programme", body, route: "/api/autoradio/programme") }
        catch ChoirError.server(let status, _) where status == 400 && body["ahead_titles"] != nil {
            body["ahead_titles"] = nil
            (_, lines) = try await client.postEventStream("/api/autoradio/programme", body, route: "/api/autoradio/programme")
        }
        return AsyncThrowingStream { c in
            let task = Task {
                var d = StretchDecoder()
                do {
                    for try await l in lines { try Task.checkCancellation(); for e in d.feed(l + "\n") { c.yield(e) } }
                    for e in d.finish() { c.yield(e) }
                    c.finish()
                } catch is CancellationError { c.finish() }
                catch let e as URLError where e.code == .cancelled { c.finish() }
                catch let e as URLError {
                    ChoirEnvironment.shared.log?.emit(.note("programme: the stretch stream dropped (\(e.code.rawValue))"))
                    c.finish()                                               // what arrived is kept
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
}

// MARK: The stage

/// The default programme stage's name. The run sheet decides what plays, so this stage reads nothing: the player is started by the
/// phone's opening and by the desk's entries (`RunSheetPlayer`, `RunSheetWriter`). Kept so a configuration that names it still resolves,
/// and so the setup interview can put it back.
public final class RadioProgramme: Programme, @unchecked Sendable {
    public static let stageID = "radio"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Autoradio run sheet", summary: "Plays the run sheet: the phone's opening, then what the desk writes ahead. Loads nothing by itself.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL)], needsNetwork: true)
    }
    public init(_ p: StageParams) {}
    public func load() async throws -> ProgrammeDocument { ProgrammeDocument(id: "none", title: "", blocks: []) }
}
