import Foundation

// The programme: a stretch of segments instead of one article (station P1).
// Design: docs/design/autoradio-dj-programme-2026-11-11.md sections 1.3, 2.2 and 6; vocabulary (blocks, last_block):
// docs/design/autoradio-learning-engine-2026-11-14.md 12.3. The failure list is in mobile/README.md ("Programme player (station P1)");
// the tests are ProgrammePlayerTests.swift and carry its numbers.
//
// What stays native: the voice's own speech blocks for one spoken entry (`ChoirApp.speakSegment`: text to blocks the deck's programme player speaks, at the
// speech size the voice engine needs). The stretch's lines, the run sheet's entries, clips, takes and forecasts are shared code's (AutoradioLab/programme-model.ts); native decodes none.

// MARK: Segments

/// One thing a segment names, as the server sent it. `label` is what the screen may show; a ref is never shown.
public struct ProgrammeCite: Codable, Sendable, Equatable {
    public var ref: String
    public var role: String
    public var label: String
    public init(ref: String, role: String = "", label: String = "") { self.ref = ref; self.role = role; self.label = label }
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
    /// For each playback block, the index of the server block it was made from (the `last_block` to report), inside `serverBlockCount`.
    public var unitServerBlocks: [Int]
    /// Each playback block's range in the text the segment was cut from, and that text's `DeckDigest`: the place the deck reports. Nil when no source text was given or a unit could not be found in it.
    public var unitRanges: [UnitRange]?
    public var sourceDigest: String?

    public init(id: String, stretchID: String, kind: String, title: String, cites: [ProgrammeCite], firstBlock: Int, blockCount: Int,
                serverBlockCount: Int, unitServerBlocks: [Int], unitRanges: [UnitRange]? = nil, sourceDigest: String? = nil) {
        self.id = id; self.stretchID = stretchID; self.kind = kind; self.title = title; self.cites = cites; self.firstBlock = firstBlock
        self.blockCount = blockCount; self.serverBlockCount = serverBlockCount
        self.unitServerBlocks = unitServerBlocks; self.unitRanges = unitRanges; self.sourceDigest = sourceDigest
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
        "answer": "Answer", "result": "Result", "bridge": "Bridge", "ident": "Station break",
        "opening": "Opening", "transition": "Transition", "take": "Take",
    ]
    public static func label(_ kind: String) -> String { labels[kind] ?? "" }
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
    /// The deck item's `paragraphs` as received, when the segment is the deck's: the units' ranges are into it.
    public var source: String?
    public init(stretchID: String, segmentID: String, kind: String, title: String, text: String, cites: [ProgrammeCite], blockCount: Int?, source: String? = nil) {
        self.stretchID = stretchID; self.segmentID = segmentID; self.kind = kind; self.title = title; self.text = text
        self.cites = cites; self.blockCount = blockCount; self.source = source
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
        var cites: [ProgrammeCite] = []
        for c in s.cites {
            let l = label(c.label)
            if let k = cites.firstIndex(where: { $0.ref == c.ref }) { if cites[k].label.isEmpty { cites[k].label = l }; continue }
            cites.append(ProgrammeCite(ref: c.ref, role: c.role, label: l))
        }
        let seg = ProgrammeSegment(id: s.segmentID, stretchID: s.stretchID, kind: s.kind, title: title(s.title, kind: s.kind), cites: cites,
                                   firstBlock: 0, blockCount: units.count, serverBlockCount: reported, unitServerBlocks: map,
                                   unitRanges: s.source.flatMap { DeckDigest.ranges(of: units.map { $0.spokenText ?? "" }, in: $0) }, sourceDigest: s.source.map(DeckDigest.of))
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

// MARK: The stage

/// The default programme stage's name. The run sheet decides what plays, so this stage reads nothing: the player is started by the
/// phone's opening and by the desk's entries (shared code: radio-bridge.ts, radio-writer.ts). Kept so a configuration that names it still resolves,
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
