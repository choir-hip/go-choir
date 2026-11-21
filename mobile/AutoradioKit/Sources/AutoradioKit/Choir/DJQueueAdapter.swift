import Foundation

// The desk and the other lanes: the seam to the podcast lane, and the "Hear it" rows on a story's page.
// Plays go onto the run sheet (`RunSheetPlayer`): Play is `place now`, Add to queue is `place queued`.

/// The podcast lane's `PodcastEpisodes.open(episodeId:feedURL:enclosureURL:at:title:show:)`: set by the host (`AutoradioModule+Podcasts.swift`). "Open episode".
public enum PodcastBridge {
    @MainActor public static var open: ((_ episodeId: String?, _ feedURL: URL?, _ enclosureURL: URL, _ at: Double) -> Void)?
    @MainActor public static func openEpisode(of clip: PodcastClip) {
        open?(clip.episodeGUID.isEmpty ? nil : clip.episodeGUID, clip.feedURL, clip.audioURL, clip.start)
    }

    public struct Target: Equatable { public var episodeId: String?; public var feedURL: URL; public var enclosureURL: URL; public var at: Double }

    /// The episode a playing audio item belongs to. A podcast-lane play carries the feed and the guid in its `episodeId`; a clip carries the bare
    /// guid (or none), so its feed comes from the clip set the phone holds, matched by guid and audio address. Nil when the phone cannot place it.
    public static func target(of item: RunSheetAudio?, store: ClipStore = .shared) -> Target? {
        guard let item else { return nil }
        let url = item.url, start = item.start, ep = item.episodeId
        if let ep, let k = PodcastKey.parseEpisode(ep), let feed = URL(string: k.feed) {
            return Target(episodeId: k.id, feedURL: feed, enclosureURL: url, at: start ?? 0)
        }
        guard let c = store.all.first(where: { $0.audioURL == url && ($0.episodeGUID.isEmpty || ep == nil || $0.episodeGUID == ep) }), let feed = c.feedURL else { return nil }
        return Target(episodeId: c.episodeGUID.isEmpty ? nil : c.episodeGUID, feedURL: feed, enclosureURL: url, at: start ?? c.start)
    }

    /// "Open episode" from a playing item. False (and nothing happens) when the item has no episode page the phone can place.
    @MainActor @discardableResult
    public static func openEpisode(of item: RunSheetAudio?, store: ClipStore = .shared) -> Bool {
        guard let open, let t = target(of: item, store: store) else { return false }
        open(t.episodeId, t.feedURL, t.enclosureURL, t.at)
        return true
    }
}

/// Play and Add to queue of a clip. The rows of "Hear it" and a show's "Clips" are shared code (AutoradioLab/clips.ts).
public enum HearIt {
    /// Play: the clip when it can be found in the served file (the player locates it); else the episode from its start. (The shared row offers Play only for a usable fingerprint; the phone no longer marks a mismatch from any live path.)
    @MainActor public static func play(_ clipID: String, store: ClipStore = .shared, player: RunSheetPlayer = .shared) async {
        guard let c = store.clip(clipID) else { return }
        player.playNow(c.isRelocatable && !store.isMismatch(clipID) ? c.clipEntry : c.episodeEntry)
    }
    @MainActor public static func enqueue(_ clipID: String, store: ClipStore = .shared, player: RunSheetPlayer = .shared) {
        if let c = store.clip(clipID) { player.queue(c.isRelocatable && !store.isMismatch(clipID) ? c.clipEntry : c.episodeEntry) }
    }
}
