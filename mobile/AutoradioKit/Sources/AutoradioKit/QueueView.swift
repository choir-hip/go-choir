import Foundation

// What the screens read of the queue: the playing clip's "Keep listening", the scrubber, the page opened from the bar.

public struct QueueInfo: Codable, Sendable, Equatable {
    public var current = ""            // title of what the queue is playing ("" when it plays nothing of its own)
    public var currentKind = ""
    public var audio = false           // remote audio is the current item
    public var playing = false
    public var clip = false
    public var keepTitle = ""          // the episode a playing clip is cut from, for "Keep listening"
    public var canOpenEpisode = false  // the audio playing has an episode page the phone can open ("Open episode")
    public var elapsed = 0.0
    public var duration = 0.0          // 0 when not known
    public var elapsedLabel = ""
    public var remainingLabel = ""
    public var fraction = 0.0          // 0...1 for the scrubber
    public var message = ""
    public var notice = ""             // the desk could not be reached ("" when all is well): a quiet line, never a row
    public var dry = false             // the last write brought nothing new and none is under way: "nothing is coming" is true
    public var writing = false         // the desk is writing what comes next
    public var webUrl = ""             // the page the bar opened ("" none)
    public var webHost = ""
    public init() {}
}

public enum QueueClock {
    /// "3:07" or "1:02:09".
    public static func label(_ seconds: Double) -> String {
        let s = Int(max(0, seconds.isFinite ? seconds : 0))
        let h = s / 3600, m = (s % 3600) / 60, sec = s % 60
        return h > 0 ? String(format: "%d:%02d:%02d", h, m, sec) : String(format: "%d:%02d", m, sec)
    }
}

extension RunSheetPlayer {
    public func info(clips: ClipStore = .shared) -> QueueInfo {
        var q = QueueInfo()
        let air = store.onAir()
        q.current = air?.displayTitle ?? ""
        q.currentKind = air?.kindWord ?? ""
        q.audio = isRemote
        q.playing = q.audio && !paused
        q.clip = audio?.isClip ?? false
        q.keepTitle = keepListeningTitle ?? ""
        q.canOpenEpisode = PodcastBridge.target(of: audio, store: clips) != nil
        if let p = remotePosition() {
            q.elapsed = p.elapsed; q.duration = p.duration ?? 0
            q.elapsedLabel = QueueClock.label(p.elapsed)
            if let d = p.duration { q.remainingLabel = "-" + QueueClock.label(max(0, d - p.elapsed)); q.fraction = min(1, max(0, p.elapsed / d)) }
        }
        q.message = message
        q.notice = deskNotice ?? ""; q.writing = deskWriting; q.dry = deskDry && !deskWriting
        return q
    }

    /// The remote audio shown as the radio: the bar's line, the player's title and position, and its play button follow the audio.
    public func overlay(_ r: RadioInfo) -> RadioInfo {
        if isWaitingForSession, !isRemote {               // the entry is held for the audio session: nothing is playing, and Play tries again
            var o = r; o.playing = false; o.paused = true
            return o
        }
        if store.onAir() == nil, !isRemote {              // nothing on air: say so plainly; Play starts the opening or asks the desk
            var o = r
            o.title = ""; o.kind = ""; o.sources = []; o.source = ""; o.position = ""; o.playing = false; o.paused = false
            o.progress = 0; o.blockProgress = 0; o.sentences = []; o.sentenceIndex = 0; o.items = []; o.next = ""
            return o
        }
        guard isRemote, let a = audio else { return r }
        var o = r
        let i = info()
        o.title = a.title; o.kind = a.show.isEmpty ? i.currentKind.uppercased() : a.show; o.source = ""
        o.position = i.duration > 0 ? "\(i.elapsedLabel) of \(QueueClock.label(i.duration))" : i.elapsedLabel
        o.playing = i.playing; o.paused = !i.playing
        o.progress = i.fraction; o.blockProgress = i.fraction
        o.sentences = []; o.sentenceIndex = 0; o.items = []; o.next = ""; o.sources = []
        o.count = 1; o.index = 0
        return o
    }
}

/// The page the embedded web view is on right now (the view reports each load). "Read aloud" reads this, not the address the view was
/// first given, so a reader who followed a link reads the page in front of them.
public final class EmbeddedPage: @unchecked Sendable {
    public static let shared = EmbeddedPage()
    private let lock = NSLock()
    private var u: URL?, t = "", opened = ""
    public func opened(_ url: String) { lock.lock(); opened = url; u = URL(string: url); t = ""; lock.unlock() }
    public func loaded(url: URL?, title: String?) { lock.lock(); if let url { u = url }; t = title ?? ""; lock.unlock() }
    /// The page to read for `ref` (the address the contract holds): the page the view reached from it, else `ref` itself.
    public func current(for ref: String) -> (url: URL, title: String)? {
        lock.lock(); defer { lock.unlock() }
        if ref == opened, let u { return (u, t) }
        guard let r = URL(string: ref) else { return nil }
        return (r, "")
    }
}

/// The entry a screen's "Play" or "Add to queue" means. `kind` is "story" (ref a route), "page" (ref an address).
public enum QueueRequest {
    @MainActor public static func entry(kind: String, ref: String, title: String) -> RunSheetEntry? {
        switch kind {
        case "story":
            let r = ref.hasPrefix("pub:") ? String(ref.dropFirst(4)) : ref
            return r.isEmpty ? nil : .paperStory(route: r, title: title)
        case "page":
            guard case .inApp = SourceLink.route(ref), let c = EmbeddedPage.shared.current(for: ref) else { return nil }
            let t = title.isEmpty ? c.title : (c.title.isEmpty ? title : c.title)
            return .webPage(url: c.url, title: t.isEmpty ? (c.url.host ?? "") : t)
        default: return nil
        }
    }
}
