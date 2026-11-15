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

    public init(id: String, stretchID: String, kind: String, title: String, cites: [ProgrammeCite], firstBlock: Int, blockCount: Int,
                serverBlockCount: Int, blockCountMismatch: Bool, unitServerBlocks: [Int]) {
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
        "answer": "Answer", "result": "Result", "bridge": "Bridge", "ident": "Station break",
    ]
    public static func isKnown(_ kind: String) -> Bool { labels[kind] != nil }
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
    public init(stretchID: String, segmentID: String, kind: String, title: String, text: String, cites: [ProgrammeCite], blockCount: Int?) {
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
public enum SplitBlocks {
    public static func split(_ text: String) -> [String] {
        let lf = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        var out: [String] = [], cur: [Substring] = []
        func flush() {
            let part = cur.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
            if !part.isEmpty { out.append(part) }
            cur = []
        }
        for line in lf.split(separator: "\n", omittingEmptySubsequences: false) {
            if line.allSatisfy({ $0.isWhitespace }) { flush() } else { cur.append(line) }
        }
        flush()
        return out
    }
}

public enum SegmentPrep {
    /// Longest title, in bytes (the server's own bound, 12.7).
    static let maxTitleBytes = 200

    /// One clean line: controls and newlines become spaces, runs of space collapse, capped in bytes at a character boundary.
    static func oneLine(_ s: String) -> String {
        var t = String(String.UnicodeScalarView(s.unicodeScalars.map { $0.properties.generalCategory == .control ? " " : $0 }))
        t = t.replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression).trimmingCharacters(in: .whitespaces)
        while t.utf8.count > maxTitleBytes { t.removeLast() }
        return t.trimmingCharacters(in: .whitespaces)
    }
    /// `pub:harbour-1`, `mail:abc`: a ref, which is never shown to the listener.
    static func looksLikeRef(_ s: String) -> Bool {
        s.range(of: "^[a-z][a-z_]{1,11}:\\S+$", options: .regularExpression) != nil
    }
    public static func title(_ raw: String, kind: String) -> String {
        let t = oneLine(raw)
        return t.isEmpty || looksLikeRef(t) ? SegmentKinds.label(kind) : t
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
            for part in ChoirText.split(flat, maxChars: 600) where !part.isEmpty {
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
                                   firstBlock: 0, blockCount: units.count, serverBlockCount: reported,
                                   blockCountMismatch: (s.blockCount ?? 0) > 0 && s.blockCount! != server.count, unitServerBlocks: map)
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
        return line.isEmpty ? [] : endLine()
    }

    private mutating func endLine() -> [StretchEvent] {
        let l = line; line = ""
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
            let st = (o["stretch_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? stretchID
            let id = (o["segment_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "\(st)/\(n)"
            var cites: [ProgrammeCite] = []
            for c in (o["cites"] as? [Any]) ?? [] {
                if let r = c as? String, !r.isEmpty { cites.append(ProgrammeCite(ref: r)) }
                else if let d = c as? [String: Any], let r = d["ref"] as? String, !r.isEmpty {
                    cites.append(ProgrammeCite(ref: r, role: d["role"] as? String ?? "", label: d["label"] as? String ?? ""))
                }
            }
            return [.segment(IncomingSegment(stretchID: st, segmentID: id, kind: kind, title: (o["title"] as? String) ?? "", text: text, cites: cites,
                                             blockCount: (o["block_count"] as? NSNumber)?.intValue))]
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
    /// Throws before any event when there is no stretch to be had (a missing route, no sign-in, no connection).
    func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error>
}

/// `POST /api/autoradio/programme` on the listener's computer (design 2.2). Until station L1 is deployed it answers 404 or 503, which is
/// "no stretch": the front page plays instead.
public struct RouteStretchSource: StretchSource {
    let baseURL: String
    public init(baseURL: String) { self.baseURL = baseURL }
    public func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> {
        let client = ChoirEnvironment.shared.client(baseURL: baseURL)
        let f = DateFormatter(); f.locale = Locale(identifier: "en_US_POSIX"); f.dateFormat = "yyyy-MM-dd'T'HH:mm:ssXXXXX"
        let body: [String: Any] = ["request_id": "radio-" + UUID().uuidString.prefix(12).lowercased(), "cause": cause,
                                   "local_time": f.string(from: Date()), "tz": TimeZone.current.identifier]
        let (_, lines) = try await client.postEventStream("/api/autoradio/programme", body, route: "/api/autoradio/programme")
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

public struct OpenedProgramme: Sendable {
    public var document: ProgrammeDocument
    /// True when no stretch was available and `document` is the front-page article.
    public var usedFallback: Bool
    /// Reads the rest of the stretch and appends each segment to the player until the stream ends or the task is cancelled. Nil for the article.
    public var follow: (@Sendable (ProgrammePlayer) async -> Void)?
}

/// The programme stage that replaces ChoirFrontProgramme as the listener's default: a stretch from the programme route when there is one,
/// else the front-page article, exactly as before. (The on-device cold-start ladder of design 2.1 is not part of P1.)
public final class RadioProgramme: Programme, @unchecked Sendable {
    public static let stageID = "radio"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Autoradio programme", summary: "Plays the stretch your computer writes, segment by segment; reads one story from the public paper when there is no stretch.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL)], needsNetwork: true)
    }
    let source: StretchSource
    let fallback: @Sendable () async throws -> ProgrammeDocument

    public init(_ p: StageParams) {
        let base = p.string("baseURL", ChoirEnvironment.defaultBaseURL)
        source = RouteStretchSource(baseURL: base)
        fallback = { try await ChoirFrontProgramme(StageParams()).load() }
    }
    public init(source: StretchSource, fallback: @escaping @Sendable () async throws -> ProgrammeDocument) {
        self.source = source; self.fallback = fallback
    }

    /// As a plain Programme stage: the document to start with. The rest of a stretch is lost; ChoirApp uses `open`.
    public func load() async throws -> ProgrammeDocument { try await open().document }

    /// The first speakable segment (or, with no stretch, the article), and what follows.
    public func open(cause: String = "open") async throws -> OpenedProgramme {
        let env = ChoirEnvironment.shared
        do {
            let stream = try await source.events(cause: cause)
            var it = stream.makeAsyncIterator()
            var doc: ProgrammeDocument?
            readFirst: while let e = try await it.next() {
                switch e {
                case .segment(let s):
                    guard let p = SegmentPrep.prepare(s) else { continue }
                    var d = ProgrammeDocument(id: "radio:" + s.stretchID, title: "Autoradio", blocks: [])
                    d.append(p); doc = d
                    break readFirst
                case .done: break readFirst
                case .error(let m): env.log?.emit(.note("programme: the stretch ended with an error: \(m)")); break readFirst
                }
            }
            if let doc {
                let box = IteratorBox(it)
                let id = doc.id
                return OpenedProgramme(document: doc, usedFallback: false, follow: { player in
                    while !Task.isCancelled, let e = try? await box.next() {
                        switch e {
                        case .segment(let s):
                            if let p = SegmentPrep.prepare(s) { _ = player.appendSegment(p, toDocument: id) }
                        case .done: return
                        case .error(let m): env.log?.emit(.note("programme: the stretch ended with an error: \(m)")); return
                        }
                    }
                })
            }
            env.log?.emit(.note("programme: the stretch had nothing to say; playing the front page"))
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            env.log?.emit(.note("programme: no stretch (\(error)); playing the front page"))
        }
        return OpenedProgramme(document: try await fallback(), usedFallback: true, follow: nil)
    }

    private final class IteratorBox: @unchecked Sendable {
        private var it: AsyncThrowingStream<StretchEvent, Error>.Iterator
        init(_ it: AsyncThrowingStream<StretchEvent, Error>.Iterator) { self.it = it }
        func next() async throws -> StretchEvent? { try await it.next() }
    }
}
