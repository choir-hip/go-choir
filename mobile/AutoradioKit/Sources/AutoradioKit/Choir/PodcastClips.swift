import Foundation

// Podcast clips (lane autoradio-dj-clips). The server lane's shapes are fixed in docs/design/podcast-clips-2026-11-15.md section 4:
//   GET /api/platform/podcast-clips  (public; the clips of every story updated in the last 72 hours, one request)
//   a candidate may carry `clips` (at most 3, copied from that answer); a `podcast_clip` segment names one by `clip_id`.
// Shared code fetches the set (AutoradioLab/clips.ts, radio-writer.ts) and hands the native player a clip's record when it plays
// (`ClipStore.put`); this file keeps the record, its parse, and the lookup by id for the audio URL and range.

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
    /// The id the podcast lane keys the episode by (its feed and guid, or its audio address when it has none): what the queue's "Open episode" and the resume position read (shared code: podcast-model.ts `episodeKey`). nil without a feed.
    public var episodeKey: String? { feedURL.map { PodcastKey.episode(feed: $0.absoluteString, id: episodeGUID.isEmpty ? PodcastKey.webURL(audioURL.absoluteString) : episodeGUID) } }
    /// The same episode from its start: no clip times (a file whose ads moved the range).
    public var episodeEntry: RunSheetEntry {
        .audio(RunSheetAudio(url: audioURL, title: episodeTitle.isEmpty ? show : episodeTitle, show: show, start: nil, end: nil, episodeId: episodeKey))
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
        .audio(RunSheetAudio(url: audioURL, title: episodeTitle.isEmpty ? show : episodeTitle, show: show, start: start, end: end, episodeId: episodeKey), clipID: clipID)
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
    private var mismatched = Set<String>()
    public init() {}
    /// This clip's audio is not the file that was transcribed (its length differs): it is never played as a clip.
    public func markMismatch(_ id: String) { lock.lock(); mismatched.insert(id); lock.unlock() }
    public func isMismatch(_ id: String) -> Bool { lock.lock(); defer { lock.unlock() }; return mismatched.contains(id) }

    public func replace(_ clips: [PodcastClip]) {
        lock.lock(); defer { lock.unlock() }
        byID = [:]; byRoute = [:]
        for c in clips where byID[c.clipID] == nil { byID[c.clipID] = c; byRoute[c.route, default: []].append(c) }
    }
    public func clip(_ id: String) -> PodcastClip? { lock.lock(); defer { lock.unlock() }; return byID[id] }
    public func clips(forRoute route: String) -> [PodcastClip] { lock.lock(); defer { lock.unlock() }; return byRoute[PodcastClip.bare(route)] ?? [] }
    /// Add clips without dropping the others (a show's own fetch): a clip held already is kept once.
    public func merge(_ clips: [PodcastClip]) {
        lock.lock(); defer { lock.unlock() }
        for c in clips where byID[c.clipID] == nil { byID[c.clipID] = c; byRoute[c.route, default: []].append(c) }
    }
    /// The page handed this clip's own record (the newest the phone has of it): it REPLACES the row held for the id, so a record the radio's fetch kept without a usable fingerprint
    /// plays as the clip the row offered, and one held once stays held once.
    public func put(_ clip: PodcastClip) {
        lock.lock(); defer { lock.unlock() }
        if let old = byID[clip.clipID] { byRoute[old.route]?.removeAll { $0.clipID == clip.clipID } }
        byID[clip.clipID] = clip; byRoute[clip.route, default: []].append(clip)
    }
    public var all: [PodcastClip] { lock.lock(); defer { lock.unlock() }; return byID.values.sorted { $0.clipID < $1.clipID } }
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
