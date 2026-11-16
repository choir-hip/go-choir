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

    public var seconds: Double { end - start }

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
        return PodcastClip(clipID: id, route: route, version: Int(num(story["version"])), storyTitle: (story["title"] as? String) ?? "",
                           show: (o["show"] as? String) ?? "", episodeTitle: (o["episode_title"] as? String) ?? "",
                           episodeGUID: (o["episode_guid"] as? String) ?? "", feedURL: (o["feed_url"] as? String).flatMap(URL.init(string:)),
                           published: (o["published"] as? String) ?? "", audioURL: a, duration: num(o["duration_seconds"]), start: start, end: end,
                           quote: (o["quote"] as? String) ?? "", support: num(o["support"]))
    }

    /// What the queue plays: the episode from `start`, stopping at `end` (Keep listening clears the end).
    public var playItem: PlayItem {
        .audio(url: audioURL, title: episodeTitle.isEmpty ? show : episodeTitle, show: show, start: start, end: end, episodeId: episodeGUID.isEmpty ? nil : episodeGUID)
    }

    /// "56 s", "1 min 5 s": the length the reader is told.
    public var lengthLabel: String {
        let s = Int(seconds.rounded())
        return s < 60 ? "\(s) s" : (s % 60 == 0 ? "\(s / 60) min" : "\(s / 60) min \(s % 60) s")
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
    public init() {}

    public func replace(_ clips: [PodcastClip], at: Date = Date(), failed: Bool = false) {
        lock.lock(); defer { lock.unlock() }
        byID = [:]; byRoute = [:]
        for c in clips where byID[c.clipID] == nil { byID[c.clipID] = c; byRoute[c.route, default: []].append(c) }
        fetchedAt = at; stale = false; lastFailed = failed
    }
    public func clip(_ id: String) -> PodcastClip? { lock.lock(); defer { lock.unlock() }; return byID[id] }
    public func clips(forRoute route: String) -> [PodcastClip] { lock.lock(); defer { lock.unlock() }; return byRoute[PodcastClip.bare(route)] ?? [] }
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
        let show = StretchCandidates.cut(StretchCandidates.oneLine(c.show), bytes: 200), ep = StretchCandidates.cut(StretchCandidates.oneLine(c.episodeTitle), bytes: 200)
        guard !show.isEmpty, !ep.isEmpty else { return nil }
        return CandidateClip(clipID: c.clipID, show: show, episodeTitle: ep, published: c.published,
                             quote: StretchCandidates.cut(StretchCandidates.oneLine(c.quote), bytes: 400), seconds: c.seconds)
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
