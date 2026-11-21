import Foundation

// Podcast clips (lane autoradio-dj-clips). The server lane's shapes are fixed in docs/design/podcast-clips-2026-11-15.md section 4:
//   GET /api/platform/podcast-clips  (public; the clips of every story updated in the last 72 hours, one request)
//   a candidate may carry `clips` (at most 3, copied from that answer); a `podcast_clip` segment names one by `clip_id`.
// The phone fetches the set ONCE per front refresh (never a request per story), keeps it for the session, attaches a story's clips to its
// candidate in the one programme request, and looks the id up here for the audio URL and range when the segment plays. A route that
// answers 404, an error or an empty list leaves the set empty and the radio behaves exactly as it did.

/// One clip the server cut from a podcast episode for a story.
public struct PodcastClip: Sendable, Equatable {
    public var clipID: String
    public var route: String            // the story's bare route, without a leading slash
    public var version: Int
    public var storyTitle: String
    public var show: String
    public var episodeTitle: String
    public var episodeGUID: String
    public var feedURL: URL?
    public var published: String
    public var audioURL: URL
    public var duration: Double
    public var start: Double
    public var end: Double
    public var quote: String
    public var support: Double
    /// The audio signature the phone finds the clip by (docs/reference/PODCAST-CLIP-FINGERPRINT.md); nil on an old record.
    public var fingerprint: ClipFingerprintRecord? = nil

    /// The clip a shared page handed over: the server's row as JSON. nil when it is not one.
    public static func parse(record: String) -> PodcastClip? {
        (try? JSONSerialization.jsonObject(with: Data(record.utf8))).flatMap { $0 as? [String: Any] }.flatMap { parse($0) }
    }

    /// The phone can find this clip in a served file: it has both fingerprints and the file is seekable by bitrate. A clip that is not
    /// relocatable is never played as a clip (its row offers the episode).
    public var isRelocatable: Bool { fingerprint?.isUsable ?? false }

    public var seconds: Double { end - start }
    /// The same episode from its start: no clip times (a file whose ads moved the range).
    public var episodeEntry: RunSheetEntry {
        .audio(RunSheetAudio(url: audioURL, title: episodeTitle.isEmpty ? show : episodeTitle, show: show, start: nil, end: nil, episodeId: episodeGUID.isEmpty ? nil : episodeGUID))
    }
    /// How far the file's length may differ from the transcribed file's before the range is distrusted.
    public static let durationTolerance = 3.0

    static func bare(_ route: String) -> String { route.hasPrefix("/") ? String(route.dropFirst()) : route }

    /// nil when the row is unusable (no id, no http(s) audio, an empty or negative range).
    public static func parse(_ o: [String: Any]) -> PodcastClip? {
        guard let id = o["clip_id"] as? String, !id.isEmpty,
              let a = (o["audio_url"] as? String).flatMap(URL.init(string:)), let scheme = a.scheme?.lowercased(), scheme == "http" || scheme == "https", a.host != nil,
              let start = (o["start"] as? NSNumber)?.doubleValue, let end = (o["end"] as? NSNumber)?.doubleValue,
              start.isFinite, end.isFinite, start >= 0, end > start else { return nil }
        let story = (o["story"] as? [String: Any]) ?? [:]
        func num(_ v: Any?) -> Double { (v as? NSNumber)?.doubleValue ?? 0 }
        let route = bare((story["route"] as? String) ?? "")
        return PodcastClip(clipID: id, route: route, version: Int(exactly: num(story["version"]).rounded(.towardZero)) ?? 0, storyTitle: (story["title"] as? String) ?? "",
                           show: (o["show"] as? String) ?? "", episodeTitle: (o["episode_title"] as? String) ?? "",
                           episodeGUID: (o["episode_guid"] as? String) ?? "", feedURL: (o["feed_url"] as? String).flatMap(URL.init(string:)),
                           published: (o["published"] as? String) ?? "", audioURL: a, duration: num(o["duration_seconds"]), start: start, end: end,
                           quote: (o["quote"] as? String) ?? "", support: num(o["support"]),
                           fingerprint: (o["fingerprint"] as? [String: Any]).flatMap(ClipFingerprintRecord.parse))
    }

    /// What the run sheet plays: the episode from `start`, stopping at `end` (Keep listening clears the end).
    public var clipEntry: RunSheetEntry {
        .audio(RunSheetAudio(url: audioURL, title: episodeTitle.isEmpty ? show : episodeTitle, show: show, start: start, end: end, episodeId: episodeGUID.isEmpty ? nil : episodeGUID), clipID: clipID)
    }

    /// "56 s", "1 min 5 s": the length the reader is told.
    public var lengthLabel: String {
        let s = Int(seconds.rounded())
        return s < 60 ? "\(s) s" : (s % 60 == 0 ? "\(s / 60) min" : "\(s / 60) min \(s % 60) s")
    }
}

/// The record's `fingerprint` object (all additive and optional on the wire; the whole object is dropped when it does not decode).
public struct ClipFingerprintRecord: Sendable, Equatable {
    public var version: Int
    public var head: [UInt8], tail: [UInt8]             // packed fingerprints
    public var headAt: Double, tailAt: Double           // where head and tail begin in the fingerprinted file
    public var start: Double, end: Double               // the clip's bounds in the fingerprinted file
    public var fileBytes: Int64, fileSeconds: Double
    public var firstFrameOffset: Int
    public var bitrateKbps: Int
    public var seekableByBitrate: Bool

    /// A head or tail is always 120 frames (12 s).
    public static let frames = 120
    public var isUsable: Bool {
        guard version == ClipFingerprint.version, seekableByBitrate, bitrateKbps > 0, end > start,
              let h = ClipFingerprint.unpack(head), let t = ClipFingerprint.unpack(tail) else { return false }
        return h.count == Self.frames * ClipFingerprint.bands && t.count == Self.frames * ClipFingerprint.bands      // 12 s each; a shorter clip has none
    }
    /// The byte of a file, by its constant bitrate: first audio frame offset plus seconds at bitrate (spec section 3).
    public func byte(at seconds: Double) -> Int { firstFrameOffset + Int(min(9e15, (max(0, seconds) * Double(bitrateKbps) * 125).rounded())) }

    static func parse(_ o: [String: Any]) -> ClipFingerprintRecord? {
        func num(_ k: String) -> Double? { (o[k] as? NSNumber).map(\.doubleValue).flatMap { $0.isFinite ? $0 : nil } }
        func bytes(_ k: String) -> [UInt8]? { (o[k] as? String).flatMap { Data(base64Encoded: $0) }.map { [UInt8]($0) } }
        guard let v = num("version"), let head = bytes("head"), let tail = bytes("tail"), let ha = num("head_at"), let ta = num("tail_at"),
              let s = num("start"), let e = num("end") else { return nil }
        // A finite number outside the Int range would trap in Int(Double): such a record is no fingerprint.
        func whole(_ x: Double) -> Int? { x.rounded(.towardZero) == x && x >= -9e15 && x <= 9e15 ? Int(x) : nil }
        guard let version = whole(v), let ffo = whole(num("first_frame_offset") ?? 0), let kbps = whole(num("bitrate_kbps") ?? 0),
              let fb = whole(num("file_bytes") ?? 0) else { return nil }
        return ClipFingerprintRecord(version: version, head: head, tail: tail, headAt: ha, tailAt: ta, start: s, end: e,
                                     fileBytes: Int64(fb), fileSeconds: num("file_seconds") ?? 0,
                                     firstFrameOffset: ffo, bitrateKbps: kbps,
                                     seekableByBitrate: (o["seekable_by_bitrate"] as? Bool) ?? false)
    }
}

/// The set the phone fetched, for the session.
public final class ClipStore: @unchecked Sendable {
    public static let shared = ClipStore()
    private let lock = NSLock()
    private var byID: [String: PodcastClip] = [:]
    private var byRoute: [String: [PodcastClip]] = [:]
    private var fetchedAt: Date?
    private var stale = true
    private var lastFailed = false
    private var mismatched = Set<String>()
    public init() {}
    /// This clip's audio is not the file that was transcribed (its length differs): it is never played as a clip.
    public func markMismatch(_ id: String) { lock.lock(); mismatched.insert(id); lock.unlock() }
    public func isMismatch(_ id: String) -> Bool { lock.lock(); defer { lock.unlock() }; return mismatched.contains(id) }

    public func replace(_ clips: [PodcastClip], at: Date = Date(), failed: Bool = false) {
        lock.lock(); defer { lock.unlock() }
        byID = [:]; byRoute = [:]
        for c in clips where byID[c.clipID] == nil { byID[c.clipID] = c; byRoute[c.route, default: []].append(c) }
        fetchedAt = at; stale = false; lastFailed = failed
    }
    public func clip(_ id: String) -> PodcastClip? { lock.lock(); defer { lock.unlock() }; return byID[id] }
    public func clips(forRoute route: String) -> [PodcastClip] { lock.lock(); defer { lock.unlock() }; return byRoute[PodcastClip.bare(route)] ?? [] }
    /// Add clips without dropping the others (a show's own fetch): a clip held already is kept once.
    public func merge(_ clips: [PodcastClip]) {
        lock.lock(); defer { lock.unlock() }
        for c in clips where byID[c.clipID] == nil { byID[c.clipID] = c; byRoute[c.route, default: []].append(c) }
    }
    public var all: [PodcastClip] { lock.lock(); defer { lock.unlock() }; return byID.values.sorted { $0.clipID < $1.clipID } }
    /// The front was refreshed: the next stretch asks again.
    public func markStale() { lock.lock(); stale = true; lock.unlock() }
    /// Fresh enough not to ask again: fetched, not marked stale, and (after a failure) not older than `failedAge`.
    func isFresh(now: Date = Date(), maxAge: TimeInterval = 900, failedAge: TimeInterval = 120) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard let t = fetchedAt, !stale else { return false }
        return now.timeIntervalSince(t) < (lastFailed ? failedAge : maxAge)
    }
}

// MARK: What the native side still needs of the podcast app's names
// The podcast pages, feeds, subscriptions and positions are shared code now (AutoradioLab/podcast.ts). Native code keeps only what the clip store,
// the locator and the "Open episode" button of the playing bar need: a feed written one way, the key a podcast play carries as its episode id, and
// the address of an episode's page. These must agree with podcast-model.ts (`feedKey`, `episodeKey`, `episodeAddress`); PodcastKeyTests pins them.

public enum PodcastKey {
    /// A feed address as one identity: trimmed, scheme and host lower-cased, no fragment, no trailing slash.
    public static func feed(_ s: String) -> String {
        var t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        if let u = URL(string: t), var c = URLComponents(url: u, resolvingAgainstBaseURL: false) {
            c.scheme = c.scheme?.lowercased(); c.host = c.host?.lowercased(); c.fragment = nil
            t = c.string ?? t
        }
        while t.hasSuffix("/") { t.removeLast() }
        return t
    }
    /// One episode as one string: the feed and the guid. The deck's episode id for a podcast play, and the key of its resume position.
    public static func episode(feed: String, id: String) -> String { "pod:" + PathSegment.encode(feed) + "/" + PathSegment.encode(id) }
    public static func parseEpisode(_ key: String) -> (feed: String, id: String)? {
        guard key.hasPrefix("pod:") else { return nil }
        let parts = key.dropFirst(4).split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
        guard parts.count == 2, let f = String(parts[0]).removingPercentEncoding, let i = String(parts[1]).removingPercentEncoding, !f.isEmpty, !i.isEmpty else { return nil }
        return (f, i)
    }
    /// The page of an episode a clip or the playing bar opens, with the clip's audio and start in the address (the page then plays from the clip's own
    /// audio when the feed no longer has the episode).
    public static func episodeAddress(feed: String, id: String, enclosure: String, at: Double) -> String {
        var a = "podcast:episode/" + PathSegment.encode(feed) + "/" + PathSegment.encode(id)
        var q: [String] = []
        if !enclosure.isEmpty { q.append("audio=" + PathSegment.encode(enclosure)) }
        if at > 0 { q.append("at=" + (at == at.rounded() ? String(Int(at)) : String(at))) }
        if !q.isEmpty { a += "?" + q.joined(separator: "&") }
        return a
    }
    /// The address when it is an http or https URL with a host, else "".
    public static func webURL(_ s: String) -> String {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let u = URL(string: t), let sc = u.scheme?.lowercased(), sc == "http" || sc == "https", u.host?.isEmpty == false else { return "" }
        return t
    }
}

public enum PodcastClips {
    public static let path = "/api/platform/podcast-clips"

    /// The one bulk request. Nothing thrown: a 404, a failure or a bad body is an empty set (`failed` says which).
    public static func fetch(client: ChoirClient) async -> (clips: [PodcastClip], failed: Bool) {
        do {
            let r = try await client.send("GET", path, query: ["limit": "200"], authenticated: false, route: path)
            let o = try r.object()
            return (((o["clips"] as? [[String: Any]]) ?? []).compactMap(PodcastClip.parse), false)
        } catch { return ([], true) }
    }

    /// One request per front refresh: skipped while the set is fresh.
    @discardableResult
    public static func ensure(client: ChoirClient, store: ClipStore = .shared) async -> Bool {
        if store.isFresh() { return false }
        let r = await fetch(client: client)
        store.replace(r.clips, failed: r.failed)
        return true
    }
}

// MARK: The clips on a candidate

public struct CandidateClip: Sendable, Equatable {
    public var clipID, show, episodeTitle, published, quote: String
    public var seconds: Double
    var json: [String: Any] {
        ["clip_id": clipID, "show": show, "episode_title": episodeTitle, "published": published, "seconds": (seconds * 10).rounded() / 10, "quote": quote]
    }

    /// The server's bounds (design 4.2): at most 3 per candidate, ids `clip-` and 12 to 64 hex digits and unique in the request, show and
    /// episode title one line of at most 200 bytes, quote at most 400 bytes, seconds 5 to 300. A clip outside them is left out, so the
    /// request is never refused for one.
    static func make(_ c: PodcastClip) -> CandidateClip? {
        guard c.clipID.range(of: "^clip-[0-9a-f]{12,64}$", options: .regularExpression) != nil, c.seconds >= 5, c.seconds <= 300 else { return nil }
        let show = SegmentPrep.oneLine(c.show, maxBytes: 200), ep = SegmentPrep.oneLine(c.episodeTitle, maxBytes: 200)
        guard !show.isEmpty, !ep.isEmpty else { return nil }
        // The date is RFC 3339 or the clip is left out: one bad value would make the server refuse the whole request.
        let published = c.published.trimmingCharacters(in: .whitespaces)
        guard Take.date(published) != nil else { return nil }
        return CandidateClip(clipID: c.clipID, show: show, episodeTitle: ep, published: published,
                             quote: SegmentPrep.oneLine(c.quote, maxBytes: 400), seconds: c.seconds)
    }
    public static let maxPerCandidate = 3
}

/// The one-line explanation of why the DJ would say a clip's lead-in, for tests and the trace.
extension ClipStore {
    /// `clips[route]` for the candidates, ids unique across the request, at most 3 each.
    func candidateClips(forRoutes routes: [String]) -> [String: [CandidateClip]] {
        var used = Set<String>()
        var out: [String: [CandidateClip]] = [:]
        for r in routes {
            var list: [CandidateClip] = []
            for c in clips(forRoute: r) where list.count < CandidateClip.maxPerCandidate && !used.contains(c.clipID) {
                guard let cc = CandidateClip.make(c) else { continue }
                used.insert(c.clipID); list.append(cc)
            }
            if !list.isEmpty { out[r] = list }
        }
        return out
    }
}

// MARK: A single waiting point

/// Resolved once; any number of waiters. Used where a result may come from two places (a stretch and a deadline) and the loser must not
/// hold the winner up (a task group would wait for it).
final class OneShot<T: Sendable>: @unchecked Sendable {
    private let lock = NSLock()
    private var value: T?
    private var waiters: [CheckedContinuation<T, Never>] = []
    func fire(_ v: T) {
        lock.lock()
        guard value == nil else { lock.unlock(); return }
        value = v; let w = waiters; waiters = []; lock.unlock()
        for c in w { c.resume(returning: v) }
    }
    func wait() async -> T {
        await withCheckedContinuation { c in
            lock.lock()
            if let v = value { lock.unlock(); c.resume(returning: v) } else { waiters.append(c); lock.unlock() }
        }
    }
}
