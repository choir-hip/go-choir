import Foundation

// The one playback queue (lane autoradio-queue). Everything the radio plays, a paper story, a web page, an imported book, remote audio,
// goes through `PlayQueue.shared`; it is the only thing that decides what plays next. The paper's list and the DJ's stretch stay the
// default source: when the queue has nothing of its own behind the item that just ended, the existing behaviour carries on.
// The player itself is behind `PlaybackEngine`, so the queue is tested with a fake one; the real adapters are `ChoirApp` (stories and
// spoken text, through the existing `ProgrammePlayer`) and `RemoteAudioPlayer` (AVPlayer).

// MARK: The shape the other lanes code against (do not rename; add only)

public enum PlayItem: Equatable, Sendable {
    case story(route: String, title: String)                      // a paper story, read aloud
    case page(url: URL, title: String)                            // a web page, text extracted and read aloud
    case document(id: String, title: String, at: Int)             // an imported epub or pdf, read aloud from a position
    case audio(url: URL, title: String, show: String, start: Double?, end: Double?, episodeId: String?)  // remote audio; start/end make it a clip; "keep listening" clears `end`
}

extension PlayItem {
    public var title: String {
        switch self {
        case .story(_, let t), .page(_, let t), .document(_, let t, _), .audio(_, let t, _, _, _, _): return t
        }
    }
    /// What the Up next list calls it.
    public var kind: String {
        switch self {
        case .story: return "story"
        case .page: return "page"
        case .document: return "book"
        case .audio(_, _, _, _, let end, _): return end != nil ? "clip" : "audio"
        }
    }
    /// A title always: the real title, else the address.
    public var displayTitle: String {
        let t = title.trimmingCharacters(in: .whitespacesAndNewlines)
        if !t.isEmpty { return t }
        switch self {
        case .story(let r, _): return r
        case .page(let u, _), .audio(let u, _, _, _, _, _): return u.host ?? u.absoluteString
        case .document(let id, _, _): return id
        }
    }
    /// The same thing to listen to, whatever its title: a story's route, a page's address, a book's id, an audio file's address.
    public func isSame(as o: PlayItem) -> Bool {
        switch (self, o) {
        case (.story(let a, _), .story(let b, _)): return a == b
        case (.page(let a, _), .page(let b, _)): return a == b
        case (.document(let a, _, _), .document(let b, _, _)): return a == b
        case (.audio(let a, _, _, _, _, _), .audio(let b, _, _, _, _, _)): return a == b
        default: return false
        }
    }
    /// The item as the desk accepts it (docs/reference/AUTORADIO-TURNS.md, "The queue"): strings clamped to their byte bounds; nil when
    /// an address or id is over its bound and cannot be clamped (the entry is then left out of what the desk is shown).
    func forWire() -> PlayItem? {
        func clamp(_ s: String, _ n: Int) -> String {
            var out = "", used = 0
            for ch in s { let c = String(ch).utf8.count; if used + c > n { break }; out.append(ch); used += c }
            return out
        }
        switch self {
        case .story(let r, let t): return r.utf8.count <= 256 ? .story(route: r, title: clamp(t, 300)) : nil
        case .page(let u, let t): return u.absoluteString.utf8.count <= 1024 ? .page(url: u, title: clamp(t, 300)) : nil
        case .document(let id, let t, let at): return id.utf8.count <= 128 ? .document(id: id, title: clamp(t, 300), at: at) : nil
        case .audio(let u, let t, let show, let s, let e, let ep):
            guard u.absoluteString.utf8.count <= 1024 else { return nil }
            return .audio(url: u, title: clamp(t, 300), show: clamp(show, 200), start: s, end: e, episodeId: ep.flatMap { $0.utf8.count <= 128 ? $0 : nil })
        }
    }
    public var isRemoteAudio: Bool { if case .audio = self { return true }; return false }
    /// A clip: remote audio with an end.
    public var isClip: Bool { if case .audio(_, _, _, _, let end, _) = self { return end != nil }; return false }
}


/// The JSON the desk reads and writes: `{"kind":"story","route":"...","title":"..."}`, `{"kind":"page","url":...}`,
/// `{"kind":"document","id":...,"at":n}`, `{"kind":"audio","url":...,"show":...,"start":s,"end":s,"episode_id":...}`.
extension PlayItem: Codable {
    enum Keys: String, CodingKey { case kind, route, title, url, id, at, show, start, end, episodeId = "episode_id" }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: Keys.self)
        let title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
        switch try c.decode(String.self, forKey: .kind) {
        case "story": self = .story(route: try c.decode(String.self, forKey: .route), title: title)
        case "page": self = .page(url: try c.decode(URL.self, forKey: .url), title: title)
        case "document": self = .document(id: try c.decode(String.self, forKey: .id), title: title, at: try c.decodeIfPresent(Int.self, forKey: .at) ?? 0)
        case "audio":
            self = .audio(url: try c.decode(URL.self, forKey: .url), title: title, show: try c.decodeIfPresent(String.self, forKey: .show) ?? "",
                          start: try c.decodeIfPresent(Double.self, forKey: .start), end: try c.decodeIfPresent(Double.self, forKey: .end),
                          episodeId: try c.decodeIfPresent(String.self, forKey: .episodeId))
        default: throw DecodingError.dataCorruptedError(forKey: .kind, in: c, debugDescription: "unknown item kind")
        }
    }
    public func encode(to e: Encoder) throws {
        var c = e.container(keyedBy: Keys.self)
        switch self {
        case .story(let r, let t): try c.encode("story", forKey: .kind); try c.encode(r, forKey: .route); try c.encode(t, forKey: .title)
        case .page(let u, let t): try c.encode("page", forKey: .kind); try c.encode(u, forKey: .url); try c.encode(t, forKey: .title)
        case .document(let id, let t, let at): try c.encode("document", forKey: .kind); try c.encode(id, forKey: .id); try c.encode(t, forKey: .title); try c.encode(at, forKey: .at)
        case .audio(let u, let t, let show, let s, let en, let ep):
            try c.encode("audio", forKey: .kind); try c.encode(u, forKey: .url); try c.encode(t, forKey: .title); try c.encode(show, forKey: .show)
            try c.encodeIfPresent(s, forKey: .start); try c.encodeIfPresent(en, forKey: .end); try c.encodeIfPresent(ep, forKey: .episodeId)
        }
    }
}

// MARK: The queue as data: entries, operations, snapshot

/// Who changed the queue. The wire words are listener | dj | programme.
public enum QueueActor: String, Codable, Sendable { case reader = "listener", dj, app = "programme" }

/// One place in the queue. The id is stable for as long as the item is in the queue (the desk names items by it) and is never reused.
public struct QueueEntry: Codable, Equatable, Sendable {
    public var id: String
    public var item: PlayItem
    public var addedBy: QueueActor
    /// A quiet caption for Up next when someone other than the reader changed it: the desk's own reason ("you asked for the long read next").
    public var note: String
    enum CodingKeys: String, CodingKey { case id, item, addedBy = "added_by", note }
    public init(id: String, item: PlayItem, addedBy: QueueActor, note: String = "") { self.id = id; self.item = item; self.addedBy = addedBy; self.note = note }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id); item = try c.decode(PlayItem.self, forKey: .item)
        addedBy = try c.decodeIfPresent(QueueActor.self, forKey: .addedBy) ?? .app; note = try c.decodeIfPresent(String.self, forKey: .note) ?? ""
    }
}

/// The one way to change the queue. JSON: `{"op":"play_next","item":{...},"base_version":7,"reason":"..."}`, `{"op":"remove","id":"q3"}`,
/// `{"op":"move","id":"q3","to_index":0}`, `{"op":"skip","id":"<current id>"}`, `{"op":"previous"}`, `{"op":"clear"}`.
public struct QueueOp: Equatable, Codable, Sendable {
    public enum Kind: String, Codable, Sendable { case playNow = "play_now", playNext = "play_next", enqueue, remove, move, skip, previous, clear }
    public var kind: Kind
    public var item: PlayItem?
    public var id: String?
    public var toIndex: Int?
    public var reason: String
    public var baseVersion: Int?
    enum CodingKeys: String, CodingKey { case kind = "op", item, id, toIndex = "to_index", reason, baseVersion = "base_version" }

    public init(_ kind: Kind, item: PlayItem? = nil, id: String? = nil, toIndex: Int? = nil, reason: String = "", baseVersion: Int? = nil) {
        self.kind = kind; self.item = item; self.id = id; self.toIndex = toIndex; self.reason = reason; self.baseVersion = baseVersion
    }
    public static func playNow(_ i: PlayItem, reason: String = "") -> QueueOp { QueueOp(.playNow, item: i, reason: reason) }
    public static func playNext(_ i: PlayItem, reason: String = "") -> QueueOp { QueueOp(.playNext, item: i, reason: reason) }
    public static func enqueue(_ i: PlayItem, reason: String = "") -> QueueOp { QueueOp(.enqueue, item: i, reason: reason) }
    public static func remove(id: String, reason: String = "") -> QueueOp { QueueOp(.remove, id: id, reason: reason) }
    public static func move(id: String, toIndex: Int, reason: String = "") -> QueueOp { QueueOp(.move, id: id, toIndex: toIndex, reason: reason) }
    public static func skip(id: String? = nil, reason: String = "") -> QueueOp { QueueOp(.skip, id: id, reason: reason) }
    public static var previous: QueueOp { QueueOp(.previous) }
    public static var clear: QueueOp { QueueOp(.clear) }

    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        kind = try c.decode(Kind.self, forKey: .kind)
        item = try c.decodeIfPresent(PlayItem.self, forKey: .item); id = try c.decodeIfPresent(String.self, forKey: .id)
        toIndex = try c.decodeIfPresent(Int.self, forKey: .toIndex); reason = try c.decodeIfPresent(String.self, forKey: .reason) ?? ""
        baseVersion = try c.decodeIfPresent(Int.self, forKey: .baseVersion)
        // an op that lacks what it needs is not an op
        switch kind {
        case .playNow, .playNext, .enqueue: if item == nil { throw DecodingError.dataCorruptedError(forKey: .item, in: c, debugDescription: "item required") }
        case .remove, .move: if id == nil { throw DecodingError.dataCorruptedError(forKey: .id, in: c, debugDescription: "id required") }
            if kind == .move, toIndex == nil { throw DecodingError.dataCorruptedError(forKey: .toIndex, in: c, debugDescription: "to_index required") }
        default: break
        }
    }
    /// A short word for the log.
    var word: String { kind.rawValue }
}

/// Who changed the queue, when, and how: kept (the last 100) so the reader can see why the order changed.
public struct QueueLogEntry: Codable, Equatable, Sendable {
    public var at: Date
    public var actor: QueueActor
    public var op: QueueOp
    public var version: Int
}

/// The queue as the desk reads it (the `queue` field of a turn request): `{version, current:{id,item,position_seconds}|null, upcoming:[{id,item,added_by}]}`.
public struct QueueSnapshot: Codable, Equatable, Sendable {
    public struct Current: Codable, Equatable, Sendable {
        public var id: String
        public var item: PlayItem
        public var positionSeconds: Double
        enum CodingKeys: String, CodingKey { case id, item, positionSeconds = "position_seconds" }
    }
    public struct Upcoming: Codable, Equatable, Sendable {
        public var id: String
        public var item: PlayItem
        public var addedBy: QueueActor
        enum CodingKeys: String, CodingKey { case id, item, addedBy = "added_by" }
    }
    public var version: Int
    public var current: Current?
    public var upcoming: [Upcoming]
    enum CodingKeys: String, CodingKey { case version, current, upcoming }
    public func encode(to e: Encoder) throws {
        var c = e.container(keyedBy: CodingKeys.self)
        try c.encode(version, forKey: .version)
        if let current { try c.encode(current, forKey: .current) } else { try c.encodeNil(forKey: .current) }
        try c.encode(upcoming, forKey: .upcoming)
    }
}

extension QueueOp {
    /// Ops as the desk returns them: an array, or an object with `ops` or `queue_ops`. Ops that do not parse are dropped one by one.
    public static func decode(_ data: Data) -> [QueueOp] {
        struct Lossy: Decodable { var op: QueueOp?; init(from d: Decoder) throws { op = try? QueueOp(from: d) } }
        struct Wrapper: Decodable { var ops: [Lossy]?; var queue_ops: [Lossy]? }
        if let w = try? JSONDecoder().decode(Wrapper.self, from: data), let l = w.ops ?? w.queue_ops { return l.compactMap(\.op) }
        if let a = try? JSONDecoder().decode([Lossy].self, from: data) { return a.compactMap(\.op) }
        return []
    }
}

// MARK: The engine behind the queue

/// What the queue drives. Spoken things (a story, a page, a book) go through the existing programme player; remote audio through AVPlayer.
/// Every `start` returns false when nothing could play (the queue then says so and moves on).
@MainActor public protocol PlaybackEngine: AnyObject {
    func playStory(route: String, title: String) async -> Bool
    /// Read `paragraphs` aloud as the document `text:<address>`, replacing what is playing.
    func speak(address: String, title: String, paragraphs: [String]) async -> Bool
    /// Play remote audio from `start` (seconds) and stop at `end` when there is one.
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool
    func pauseRemote()
    func resumeRemote()
    /// Silence remote audio (something else is starting). The queue calls it before every non-audio item.
    func stopRemote()
    func seekRemote(by seconds: Double)
    func seekRemote(to seconds: Double)
    /// The playing clip no longer ends: the episode carries on from where it is, with no gap or restart.
    func clearRemoteEnd()
    func remotePosition() -> (elapsed: Double, duration: Double?)?
    func setRemoteRate(_ r: Double)
    func setRemoteLevel(_ l: PlayLevel)
    /// The length of the audio at `url`, read without playing it (nothing is paused or started); nil when unknown within `deadline` seconds.
    /// A clip's range is valid only for the file that was transcribed (publishers that stitch in ads serve another file each time).
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double?
}
extension PlaybackEngine {
    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { nil }
}

// MARK: A URL in the prompt bar

public enum BarURL {
    /// A book or a PDF typed into the bar goes to the Reader's import, not to a web page. The reader lane sets this to `ReaderShelf.importFrom(url:)`;
    /// until it is set the address opens as a page.
    nonisolated(unsafe) public static var importDocument: (@Sendable (URL) -> Void)?
    public static func isDocument(_ url: URL) -> Bool { ["pdf", "epub"].contains(url.pathExtension.lowercased()) }
    /// The page a submitted bar text means, or nil when it is an ordinary sentence. http and https; or one bare word with a dot, a
    /// known-looking host ("choir.news", "example.com/path"). A question ("what is choir.news?") has spaces, so it is not a URL.
    public static func detect(_ text: String) -> URL? {
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty, t.rangeOfCharacter(from: .whitespacesAndNewlines) == nil, !t.contains("@") || t.lowercased().hasPrefix("http") else { return nil }
        let lower = t.lowercased()
        if lower.hasPrefix("http://") || lower.hasPrefix("https://") {
            guard let u = URL(string: t), let h = u.host, !h.isEmpty else { return nil }
            return u
        }
        if t.contains("://") || t.hasSuffix("?") || t.hasSuffix("!") || t.hasSuffix(",") { return nil }
        let host = t.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? t
        let hostOnly = host.split(separator: ":").first.map(String.init) ?? host
        let labels = hostOnly.split(separator: ".", omittingEmptySubsequences: false)
        guard labels.count >= 2, labels.allSatisfy({ !$0.isEmpty }) else { return nil }
        guard let tld = labels.last, tld.count >= 2, tld.allSatisfy({ $0.isLetter && $0.isLowercase }) else { return nil }
        guard labels.allSatisfy({ $0.allSatisfy { $0.isLetter || $0.isNumber || $0 == "-" } }) else { return nil }
        return URL(string: "https://" + t)
    }
}

/// A .pdf or .epub typed into the bar: import it into the Reader, then open its page; if it fails say why in the bar's own line.
public enum BarDocumentImport {
    public static func name(of url: URL) -> String { url.lastPathComponent.removingPercentEncoding ?? url.lastPathComponent }
    public static func failureLine(_ error: Error, url: URL) -> String {
        let why = (error as? ShelfError)?.description ?? ShelfError.unreadable("").description
        return "\(name(of: url)) was not added. \(why)"
    }
    /// `importer` returns the new document's id. Nothing opens on a failure.
    public static func run(url: URL, importer: (URL) async throws -> String, open: @MainActor (String) -> Void, say: (String) -> Void) async {
        do { let id = try await importer(url); await open(ShelfAddress.address(id)) }
        catch { say(failureLine(error, url: url)) }
    }
}

// MARK: Reading a page

public enum PageText {
    /// A web page as paragraphs to read: its main content (an article, a main, else the body), with the page title first.
    public static func paragraphs(html: String, fallbackTitle: String) -> (title: String, paragraphs: [String]) {
        let title = firstMatch(html, "<title[^>]*>(.*?)</title>").map { MailText.decodeEntities($0).trimmingCharacters(in: .whitespacesAndNewlines) } ?? ""
        var body = html
        for tag in ["article", "main"] {
            if let inner = firstMatch(body, "<\(tag)\\b[^>]*>(.*)</\(tag)\\s*>"), inner.count > 400 { body = inner; break }
        }
        for junk in ["nav", "header", "footer", "aside", "form"] {
            body = body.replacingOccurrences(of: "<\(junk)\\b.*?</\(junk)\\s*>", with: "", options: [.regularExpression, .caseInsensitive])
        }
        let text = MailText.htmlToText(body)
        let t = (title.isEmpty ? fallbackTitle : title)
        let composed = ListenText.compose(title: t, body: text)
        return (t, ChoirText.paragraphs(composed, maxChars: ChoirText.speechChars))
    }

    static func firstMatch(_ s: String, _ pattern: String) -> String? {
        guard let re = try? NSRegularExpression(pattern: pattern, options: [.caseInsensitive, .dotMatchesLineSeparators]),
              let m = re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)), m.numberOfRanges > 1, let r = Range(m.range(at: 1), in: s) else { return nil }
        return String(s[r])
    }

    /// Fetch and extract. nil when the page cannot be fetched or has nothing to read.
    public static func fetch(_ url: URL, title: String) async -> (title: String, paragraphs: [String])? {
        var req = URLRequest(url: url, timeoutInterval: 20)
        req.setValue("text/html,application/xhtml+xml", forHTTPHeaderField: "Accept")
        guard let (data, resp) = try? await URLSession.shared.data(for: req),
              (resp as? HTTPURLResponse).map({ (200..<300).contains($0.statusCode) }) ?? true else { return nil }
        let html = String(decoding: data.prefix(3_000_000), as: UTF8.self)
        let r = paragraphs(html: html, fallbackTitle: title)
        return r.paragraphs.count > (r.title.isEmpty ? 0 : 1) ? r : nil
    }
}

/// Run on the main actor from any thread (the lock screen's commands arrive on whichever thread the system chooses).
func onMainThread<T>(_ f: @MainActor () -> T) -> T {
    if Thread.isMainThread { return MainActor.assumeIsolated(f) }
    return DispatchQueue.main.sync { MainActor.assumeIsolated(f) }
}

// MARK: The queue

@MainActor public final class PlayQueue {
    /// The titles of the DJ's coming segments, given by the host. Shown in Up next after the queued items; never part of the queue.
    public var programmeComing: (@MainActor () -> [String])?
    public static let shared = PlayQueue(storage: PlayQueue.applicationSupportFile())

    public private(set) var current: PlayItem?
    public private(set) var currentId: String?
    /// The queued entries, with their ids. `upcoming` is the same list as plain items.
    public private(set) var entries: [QueueEntry] = []
    public var upcoming: [PlayItem] { entries.map(\.item) }
    /// Increments on every change: the desk is told which queue it is editing.
    public private(set) var version = 0
    public private(set) var log: [QueueLogEntry] = []
    /// The listener paused (remote audio; spoken items are paused through the programme player's own transport).
    public private(set) var paused = false
    /// A plain-words line about the last thing that could not play ("" when nothing is wrong).
    public private(set) var message = ""
    /// The block of the current document or page being read (absolute: counts from the start of the document).
    public private(set) var documentBlock = 0
    /// Which start this is; the spoken document is `text:q<generation>`.
    public private(set) var generation = 0

    public var engine: PlaybackEngine?
    public var onChange: (() -> Void)?
    /// The reader lane's: the document and block being read, reported as reading moves.
    public var onDocumentPosition: ((String, Int) -> Void)?
    /// When the queue is empty behind an item that was not a paper story: the default source (the DJ's stretch) refills. True: it started.
    public var defaultSource: (() async -> Bool)?
    /// Fetches a page's text; replaceable for tests.
    public var pageText: (URL, String) async -> (title: String, paragraphs: [String])? = { await PageText.fetch($0, title: $1) }

    private var textSources: [String: @Sendable (String, Int) async -> [String]?] = [:]
    private var startTask: Task<Void, Never>?
    private var resumeSeconds: Double?          // restored position of the current audio item
    private var restoredIdle = false            // current came from the saved file and has not been started
    private var nextId = 0
    private let storage: URL?

    public init(storage: URL? = nil) { self.storage = storage }

    public static func applicationSupportFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("queue.json")
    }

    /// Text for `.document` (and any future kind): the reader lane registers `{ id, position in blocks }`; blocks start at `position`.
    public func registerTextSource(kind: String, _ source: @escaping @Sendable (String, Int) async -> [String]?) { textSources[kind] = source }

    /// `q<n>`: a counter that is saved with the queue, so an id is never used twice (the desk names items by id).
    private func makeId() -> String { nextId += 1; return "q\(nextId)" }
    private var appliedTurns: [String] = []

    // MARK: The one mutation

    /// Apply operations, in order, as `actor`. Every change bumps `version` and is logged. An unknown id is ignored (the desk may be a
    /// version behind). Anything that starts playing runs behind this call (`settle()` waits for it).
    @discardableResult
    public func apply(_ ops: [QueueOp], by actor: QueueActor) -> Int { run(ops, by: actor, server: false, batchBase: nil).applied }

    /// What the desk answered (the `queue_ops` event, parsed). The phone's rebase rule, in order: (1) id ops (move, remove, skip) apply if
    /// that entry is still there (skip: only if it is still the current one), whatever the version, else are dropped silently; (2) add ops
    /// always apply; (3) clear and previous apply only if the phone's version is still the op's `base_version`, else are dropped.
    /// A turn's ops apply once: a replayed turn_id changes nothing.
    @discardableResult
    public func apply(serverOps ops: [QueueOp], turnId: String, baseVersion: Int?) -> (applied: Int, dropped: Int) {
        if !turnId.isEmpty, !ops.isEmpty {
            if appliedTurns.contains(turnId) { return (0, ops.count) }
            appliedTurns.append(turnId); if appliedTurns.count > 100 { appliedTurns.removeFirst(appliedTurns.count - 100) }
        }
        return run(ops, by: .dj, server: true, batchBase: baseVersion)
    }
    /// The `queue_ops` event's data: `{"turn_id","base_version","replayed","ops":[...]}`.
    @discardableResult
    public func applyQueueOpsEvent(_ data: Data) -> (applied: Int, dropped: Int) {
        struct Head: Decodable { var turn_id: String?; var base_version: Int? }
        let h = (try? JSONDecoder().decode(Head.self, from: data)) ?? Head()
        return apply(serverOps: QueueOp.decode(data), turnId: h.turn_id ?? "", baseVersion: h.base_version)
    }

    private func run(_ ops: [QueueOp], by actor: QueueActor, server: Bool, batchBase: Int?) -> (applied: Int, dropped: Int) {
        let startVersion = version
        var applied = 0, dropped = 0
        for op in ops {
            var did = true
            if server {
                switch op.kind {
                case .move, .remove: if !entries.contains(where: { $0.id == op.id }) { did = false }
                case .skip: if current == nil || (op.id != nil && op.id != currentId) { did = false }
                case .clear, .previous: if startVersion != (op.baseVersion ?? batchBase) { did = false }
                default: break
                }
                if !did { dropped += 1; continue }
            }
            let caption = op.reason.isEmpty ? (actor == .dj ? "DJ added" : "") : op.reason
            // Queueing what is playing right now adds nothing: said quietly, not an error.
            if (op.kind == .playNext || op.kind == .enqueue), let it = op.item, let c = current, c.isSame(as: it) {
                message = "Already playing."; dropped += 1; continue
            }
            switch op.kind {
            case .playNow:
                guard let item = op.item else { did = false; break }
                startTask = Task { await self.start(item, by: actor) }
            case .playNext:
                guard let item = op.item else { did = false; break }
                entries.insert(QueueEntry(id: makeId(), item: item, addedBy: actor, note: caption), at: 0)
            case .enqueue:
                guard let item = op.item else { did = false; break }
                entries.append(QueueEntry(id: makeId(), item: item, addedBy: actor, note: caption))
            case .remove:
                if let i = entries.firstIndex(where: { $0.id == op.id }) { entries.remove(at: i) }
                else if op.id == currentId { startTask = Task { await self.advance() } }
                else { did = false }
            case .move:
                if let i = entries.firstIndex(where: { $0.id == op.id }) {
                    let e = entries.remove(at: i)
                    let t = min(max(op.toIndex ?? 0, 0), entries.count)
                    entries.insert(e, at: t)
                    if actor != .reader {
                        entries[t].note = op.reason.isEmpty ? (t < i ? "DJ moved this up" : "DJ moved this down") : op.reason
                    }
                } else { did = false }
            case .skip:
                if !entries.isEmpty || current != nil { startTask = Task { await self.advance() } } else { did = false }
            case .previous:
                if let c = current {
                    if c.isRemoteAudio { engine?.seekRemote(to: 0) } else { let id = currentId; startTask = Task { await self.start(c, by: actor, keepId: id) } }
                } else { did = false }
            case .clear:
                did = !entries.isEmpty; entries.removeAll()
            }
            if did {
                applied += 1; version += 1
                var logged = op; if logged.reason.isEmpty { logged.reason = "" }
                log.append(QueueLogEntry(at: Date(), actor: actor, op: logged, version: version))
            } else { dropped += 1 }
        }
        if log.count > 100 { log.removeFirst(log.count - 100) }
        if applied > 0 { changed() }
        return (applied, dropped)
    }

    /// The queue as the desk reads it.
    public func snapshot() -> QueueSnapshot {
        let cur = current.flatMap { $0.forWire() }.map { QueueSnapshot.Current(id: currentId ?? "", item: $0, positionSeconds: min(max(positionSeconds(), 0), 10_000_000)) }
        // An entry the desk cannot take (an address over its bound) is left out; `snapshotJSON` sends the first 50 of the rest.
        let up = entries.compactMap { e in e.item.forWire().map { QueueSnapshot.Upcoming(id: e.id, item: $0, addedBy: e.addedBy) } }
        return QueueSnapshot(version: version, current: cur, upcoming: up)
    }
    /// The server reads at most this many upcoming entries; more makes the desk say it cannot see the queue.
    public static let maxSnapshotUpcoming = 50
    public func snapshotJSON(maxUpcoming: Int = PlayQueue.maxSnapshotUpcoming) -> String {
        let e = JSONEncoder(); e.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        var snap = snapshot()
        if snap.upcoming.count > maxUpcoming { snap.upcoming = Array(snap.upcoming.prefix(maxUpcoming)) }
        return (try? e.encode(snap)).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
    }
    /// Apply what the desk answered (`{"queue_ops":[...]}` or an array), as the DJ.
    @discardableResult
    public func applyDeskAnswer(_ data: Data) -> Int { applyQueueOpsEvent(data).applied }

    private func positionSeconds() -> Double {
        if current?.isRemoteAudio == true { return engine?.remotePosition()?.elapsed ?? resumeSeconds ?? 0 }
        return Double(documentBlock)
    }

    // MARK: Wrappers

    /// Play this now. What was playing is replaced (it is not put back); what was queued stays queued.
    public func playNow(_ item: PlayItem) { apply([.playNow(item)], by: .reader) }
    /// After the one playing, before anything else queued.
    public func playNext(_ item: PlayItem) { apply([.playNext(item)], by: .reader) }
    /// At the end.
    public func enqueue(_ item: PlayItem) { apply([.enqueue(item)], by: .reader) }
    /// Take an item out of "Up next".
    public func remove(at i: Int) { guard entries.indices.contains(i) else { return }; apply([.remove(id: entries[i].id)], by: .reader) }
    /// Tap an item in "Up next": play it now; the ones before it are dropped, like a playlist jump.
    public func playUpcoming(at i: Int) {
        guard entries.indices.contains(i) else { return }
        let e = entries[i]
        entries.removeSubrange(0...i)
        startTask = Task { await self.start(e.item, by: .reader, keepId: e.id) }
        version += 1; changed()
    }
    public func clearUpcoming() { apply([.clear], by: .reader) }

    /// Wait for the start in flight (tests, and the host that wants to know it began).
    public func settle() async { await startTask?.value }

    /// Something began playing outside the queue (the paper's list moved to its next story): the queue shows it, and does not drive it.
    public func noteStarted(_ item: PlayItem?) {
        current = item; currentId = item == nil ? nil : makeId(); paused = false; documentBlock = 0; restoredIdle = false; version += 1; changed()
    }

    // MARK: Starting

    @discardableResult
    func start(_ item: PlayItem, by actor: QueueActor = .reader, keepId: String? = nil) async -> Bool {
        generation += 1
        let my = generation
        current = item; currentId = keepId ?? makeId(); paused = false; message = ""; restoredIdle = false
        documentBlock = { if case .document(_, _, let at) = item { return at }; return 0 }()
        let resume = resumeSeconds; resumeSeconds = nil
        version += 1
        changed()
        guard let engine else { return false }
        var ok = false
        switch item {
        case .story(let route, let title):
            engine.stopRemote()
            ok = await engine.playStory(route: route, title: title)
        case .page(let url, let title):
            engine.stopRemote()
            if let p = await pageText(url, title), my == generation {
                ok = await engine.speak(address: "q\(my)", title: p.title.isEmpty ? item.displayTitle : p.title, paragraphs: p.paragraphs)
            } else if my == generation { message = "Could not read \u{201C}\(item.displayTitle)\u{201D}." }
        case .document(let id, let title, let at):
            engine.stopRemote()
            if let src = textSources["document"], let blocks = await src(id, at), !blocks.isEmpty, my == generation {
                ok = await engine.speak(address: "q\(my)", title: title, paragraphs: blocks)
            } else if my == generation { message = "Could not open \u{201C}\(item.displayTitle)\u{201D} to read." }
        case .audio(let url, let title, let show, let start, let end, _):
            ok = await engine.playRemote(url: url, title: title, show: show, start: resume ?? start, end: end)
            if !ok, my == generation { message = "Could not play \u{201C}\(item.displayTitle)\u{201D}." }
        }
        guard my == generation else { return ok }       // a newer start took over
        if !ok { await advance(); return false }        // what cannot play is skipped
        changed()
        return true
    }

    /// The current item is over: the next queued one plays; with none, `defaultSource` (not for a paper story: the paper's list is its own default).
    @discardableResult
    func advance(useDefault: Bool = true) async -> Bool {
        if !entries.isEmpty {
            let n = entries.removeFirst()
            return await start(n.item, keepId: n.id)
        }
        current = nil; currentId = nil; paused = false; version += 1; changed()
        if useDefault, let d = defaultSource { return await d() }
        return false
    }

    /// Skip to what is queued next (the lock screen's and the player's next while the queue has something).
    public func skip() { if !entries.isEmpty || current?.isRemoteAudio == true { apply([.skip()], by: .reader) } }
    public var hasUpcoming: Bool { !entries.isEmpty }

    // MARK: Ends, reported by the engines

    /// The document that was playing ended (a story, a queued text, the DJ's stretch between segments): the queue's own next item plays.
    /// True: it started one.
    func advanceFromEnd(documentID: String, route: String?) async -> Bool {
        if documentID.hasPrefix("text:q") { return await spokenEnded(documentID: documentID) }
        if let route, await storyEnded(route: route) { return true }
        if documentID.hasPrefix("radio:"), !entries.isEmpty { return await advance(useDefault: false) }
        return false
    }

    /// A paper story ran out of blocks. True: the queue started its own next item (the paper's list must not also advance).
    public func storyEnded(route: String) async -> Bool {
        guard case .story(let r, _)? = current, r == route, !entries.isEmpty else { return false }
        return await advance(useDefault: false)
    }
    /// A spoken document (`text:q<n>`) ran out of blocks.
    public func spokenEnded(documentID: String) async -> Bool {
        guard documentID == "text:q\(generation)", current != nil, current?.isRemoteAudio == false else { return false }
        return await advance()
    }
    /// Remote audio reached its end, or the clip's end. `generation` is the start it belongs to.
    public func remoteEnded(generation g: Int) async {
        guard g == generation, current?.isRemoteAudio == true else { return }
        engine?.stopRemote()
        await advance()
    }

    // MARK: Transport for remote audio

    public func pause() { guard current?.isRemoteAudio == true else { return }; paused = true; engine?.pauseRemote(); save(); changed() }
    public func resume() {
        guard let c = current else { return }
        if restoredIdle { restoredIdle = false; let id = currentId; startTask = Task { await self.start(c, keepId: id) }; return }   // a queue from the last launch: first play
        guard c.isRemoteAudio else { return }
        paused = false; engine?.resumeRemote(); changed()
    }
    /// The saved queue has a current item that has not been started this launch.
    public var hasRestoredCurrent: Bool { restoredIdle && current != nil }
    public func seek(by seconds: Double) { guard current?.isRemoteAudio == true else { return }; engine?.seekRemote(by: seconds); changed() }
    public func seek(to seconds: Double) { guard current?.isRemoteAudio == true else { return }; engine?.seekRemote(to: seconds); changed() }
    public func setRate(_ r: Double) { engine?.setRemoteRate(PlaybackSpeed.clamp(r)) }
    /// The hands-free turn: the radio's own level (dip or pause for the reader's words, play after).
    public func setLevel(_ l: PlayLevel) { if current?.isRemoteAudio == true { engine?.setRemoteLevel(l) } }

    /// "Keep listening": the clip stops being a clip. The episode continues past it, from where it is.
    public func keepListening() {
        guard case .audio(let url, let title, let show, let start, .some, let ep)? = current else { return }
        current = .audio(url: url, title: title, show: show, start: start, end: nil, episodeId: ep)
        engine?.clearRemoteEnd()
        version += 1; save(); changed()
    }

    /// The current item's title, and for a clip the episode it is cut from (what "Keep listening" shows).
    public var keepListeningTitle: String? {
        guard case .audio(_, let title, let show, _, .some, _)? = current else { return nil }
        return show.isEmpty ? title : "\(title) (\(show))"
    }

    public func remotePosition() -> (elapsed: Double, duration: Double?)? { current?.isRemoteAudio == true ? engine?.remotePosition() : nil }

    /// Reading moved to block `i` of the spoken document (counted from the block the reading began at).
    public func spokenBlock(_ i: Int) {
        let base: Int = { if case .document(_, _, let at)? = current { return at }; return 0 }()
        guard documentBlock != base + i else { return }
        documentBlock = base + i
        if case .document(let id, _, _)? = current { onDocumentPosition?(id, documentBlock) }
    }

    // MARK: Saving

    struct Saved: Codable {
        var version: Int?; var current: PlayItem?; var currentId: String?; var entries: [QueueEntry]?; var upcoming: [PlayItem]?
        var seconds: Double?; var block: Int; var log: [QueueLogEntry]?; var nextId: Int?; var turns: [String]?
    }

    func changed() { save(); onChange?() }

    public func save() {
        guard let storage else { return }
        let elapsed = current?.isRemoteAudio == true ? engine?.remotePosition()?.elapsed ?? resumeSeconds : nil
        let s = Saved(version: version, current: current, currentId: currentId, entries: entries, upcoming: nil, seconds: elapsed, block: documentBlock, log: log, nextId: nextId, turns: appliedTurns)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: storage, options: .atomic) }
    }

    /// The queue and where it was, from the last launch. Nothing plays; the first play resumes the current item.
    public func restore() {
        guard let storage, let d = try? Data(contentsOf: storage), let s = try? JSONDecoder().decode(Saved.self, from: d) else { return }
        nextId = s.nextId ?? 0; appliedTurns = s.turns ?? []
        current = s.current; currentId = s.currentId ?? (s.current == nil ? nil : makeId())
        entries = s.entries ?? (s.upcoming ?? []).map { QueueEntry(id: makeId(), item: $0, addedBy: .app) }
        log = s.log ?? []; version = s.version ?? 0
        resumeSeconds = s.seconds; restoredIdle = s.current != nil; paused = s.current != nil
        documentBlock = s.block
        if case .document(let id, let title, _)? = current { current = .document(id: id, title: title, at: s.block) }
        onChange?()
    }
}
