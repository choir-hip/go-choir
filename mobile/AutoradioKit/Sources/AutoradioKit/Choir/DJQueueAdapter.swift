import Foundation

// The DJ and the other lanes (lane autoradio-dj-clips): the seam to the podcast lane, and the DJ's coming segments for "Up next".
// Queue changes by the DJ travel only on POST /api/autoradio/turns (the talk lane's path); the programme stream carries none.

/// The podcast lane's `PodcastEpisodes.open(episodeId:feedURL:enclosureURL:at:)`, until it exists: set by the host. "Open episode".
public enum PodcastBridge {
    @MainActor public static var open: ((_ episodeId: String?, _ feedURL: URL?, _ enclosureURL: URL, _ at: Double) -> Void)?
    @MainActor public static func openEpisode(of clip: PodcastClip) {
        open?(clip.episodeGUID.isEmpty ? nil : clip.episodeGUID, clip.feedURL, clip.audioURL, clip.start)
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
}
public enum HearIt {
    public static func rows(route: String, store: ClipStore = .shared) -> [HearItRow] {
        store.clips(forRoute: route).map { HearItRow(clipID: $0.clipID, show: $0.show, episode: $0.episodeTitle, length: $0.lengthLabel, quote: $0.quote, episodeOnly: store.isMismatch($0.clipID)) }
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
