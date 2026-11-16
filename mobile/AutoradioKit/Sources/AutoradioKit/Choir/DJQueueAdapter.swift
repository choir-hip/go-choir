import Foundation

// The DJ and the other lanes (lane autoradio-dj-clips): the seam to the podcast lane, and the DJ's coming segments for "Up next".
// Queue changes by the DJ travel only on POST /api/autoradio/turns (the talk lane's path); the programme stream carries none.

/// The podcast lane's `PodcastEpisodes.open(episodeId:feedURL:enclosureURL:at:title:show:)`: set by the host (`AutoradioModule+Podcasts.swift`). "Open episode".
public enum PodcastBridge {
    @MainActor public static var open: ((_ episodeId: String?, _ feedURL: URL?, _ enclosureURL: URL, _ at: Double) -> Void)?
    @MainActor public static func openEpisode(of clip: PodcastClip) {
        open?(clip.episodeGUID.isEmpty ? nil : clip.episodeGUID, clip.feedURL, clip.audioURL, clip.start)
    }

    public struct Target: Equatable { public var episodeId: String?; public var feedURL: URL; public var enclosureURL: URL; public var at: Double }

    /// The episode a playing audio item belongs to. A podcast-lane play carries the feed and the guid in its `episodeId`; a clip carries the bare
    /// guid (or none), so its feed comes from the clip set the phone holds, matched by guid and audio address. Nil when the phone cannot place it.
    public static func target(of item: PlayItem?, store: ClipStore = .shared) -> Target? {
        guard case .audio(let url, _, _, let start, _, let ep)? = item else { return nil }
        if let ep, let k = PodcastKey.parseEpisode(ep), let feed = URL(string: k.feed) {
            return Target(episodeId: k.id, feedURL: feed, enclosureURL: url, at: start ?? 0)
        }
        guard let c = store.all.first(where: { $0.audioURL == url && ($0.episodeGUID.isEmpty || ep == nil || $0.episodeGUID == ep) }), let feed = c.feedURL else { return nil }
        return Target(episodeId: c.episodeGUID.isEmpty ? nil : c.episodeGUID, feedURL: feed, enclosureURL: url, at: start ?? c.start)
    }

    /// "Open episode" from a playing item. False (and nothing happens) when the item has no episode page the phone can place.
    @MainActor @discardableResult
    public static func openEpisode(of item: PlayItem?, store: ClipStore = .shared) -> Bool {
        guard let open, let t = target(of: item, store: store) else { return false }
        open(t.episodeId, t.feedURL, t.enclosureURL, t.at)
        return true
    }
}

/// The DJ's own coming segments, by title, for "Up next" (the queue shows what the DJ will play next beside what the reader queued).
public enum DJUpcoming {
    public static func titles(in doc: ProgrammeDocument, blockIndex: Int, limit: Int = 3) -> [String] {
        guard doc.id.hasPrefix("radio:"), let si = doc.segmentIndex(containingBlock: blockIndex) else { return [] }
        return doc.segments.dropFirst(si + 1).prefix(limit).map(\.title)
    }
}

/// "Hear it" on a story's page: one row per clip the bulk set holds for the route (show, episode, length, the short quote), with Play and
/// Add to queue. The reader page's host gives the rows to the contract and calls `play` / `enqueue` from its buttons.
public struct HearItRow: Codable, Sendable, Equatable {
    public var clipID, show, episode, length, quote: String
    /// The file is not the one transcribed: the row offers the episode only, no clip.
    public var episodeOnly: Bool = false
    /// The story the clip was cut for (a show page lists clips by the story they were found for).
    public var story: String = ""
}
public enum HearIt {
    public static func rows(route: String, store: ClipStore = .shared) -> [HearItRow] {
        store.clips(forRoute: route).map { row($0, store) }
    }
    static func row(_ c: PodcastClip, _ store: ClipStore) -> HearItRow {
        HearItRow(clipID: c.clipID, show: c.show, episode: c.episodeTitle, length: c.lengthLabel, quote: c.quote, episodeOnly: store.isMismatch(c.clipID), story: c.storyTitle)
    }
    /// The clips of the story at a page address (`pub:<route>`, with or without a version); at most three; none for any other page.
    public static func rows(address: String, store: ClipStore = .shared) -> [HearItRow] {
        let t = TextureAddress.parse(address)
        guard t.kind == .publication else { return [] }
        var found = store.clips(forRoute: t.route)
        if found.isEmpty { found = store.clips(forRoute: TextureAddress.splitRoute(t.route).bare) }
        return found.prefix(3).map { row($0, store) }
    }
    /// The clips found for a show, quote first (a show page's "Clips"); at most ten.
    public static func rows(feed: String, store: ClipStore = .shared) -> [HearItRow] {
        store.clips(forFeed: feed).prefix(10).map { row($0, store) }
    }
    /// Play: the clip when the audio is the transcribed file; else the episode from its start (the row then says "Open episode").
    @MainActor public static func play(_ clipID: String, store: ClipStore = .shared, queue: PlayQueue = .shared, runner: DJClipRunner = .shared) async {
        guard let c = store.clip(clipID) else { return }
        if store.isMismatch(clipID) { queue.playNow(c.episodeItem); return }
        queue.playNow(await runner.preflight(clipID) ? c.playItem : c.episodeItem)
    }
    @MainActor public static func enqueue(_ clipID: String, store: ClipStore = .shared, queue: PlayQueue = .shared) {
        if let c = store.clip(clipID) { queue.enqueue(c.playItem) }
    }
}

/// What a clip row's Play, Queue and Open episode do. Returns the plain note the page shows ("" when nothing needs saying).
public enum ClipActions {
    public static let gone = "That clip is not available any more."
    @MainActor public static func run(op: String, clipID: String, store: ClipStore = .shared, queue: PlayQueue = .shared) async -> String {
        guard let c = store.clip(clipID) else { return gone }
        switch op {
        case "play": await HearIt.play(clipID, store: store, queue: queue); return ""
        case "queue": HearIt.enqueue(clipID, store: store, queue: queue); return "Added to the queue."
        case "open": PodcastBridge.openEpisode(of: c); return ""
        default: return ""
        }
    }
}

/// The wire shapes of the clip rows (interface/joins.json).
public struct ClipRowsView: Codable, Sendable, Equatable { public var rows: [HearItRow]; public init(rows: [HearItRow] = []) { self.rows = rows } }
public struct ClipAckView: Codable, Sendable, Equatable { public var status = "ok", message = ""; public init(message: String = "") { self.message = message } }
