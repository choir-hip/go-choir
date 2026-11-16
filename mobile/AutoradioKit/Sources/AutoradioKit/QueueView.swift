import Foundation

// What the screens read of the queue: "Up next", the playing clip's "Keep listening", the scrubber, the page opened from the bar.

public struct QueueRow: Codable, Sendable, Equatable {
    public var i = 0
    public var title = ""
    public var kind = ""
    public var detail = ""
    public var id = ""
    public var note = ""       // a quiet caption when the DJ changed the order ("DJ moved this up")
    public var by = ""         // who put it there, as a sentence: "Added by you", "Added by the DJ", "From the station"
    public init() {}
}

public struct QueueInfo: Codable, Sendable, Equatable {
    public var current = ""            // title of what the queue is playing ("" when it plays nothing of its own)
    public var currentKind = ""
    public var audio = false           // remote audio is the current item
    public var playing = false
    public var clip = false
    public var keepTitle = ""          // the episode a playing clip is cut from, for "Keep listening"
    public var elapsed = 0.0
    public var duration = 0.0          // 0 when not known
    public var elapsedLabel = ""
    public var remainingLabel = ""
    public var fraction = 0.0          // 0...1 for the scrubber
    public var upcoming: [QueueRow] = []
    public var message = ""
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

extension PlayQueue {
    public func info() -> QueueInfo {
        var q = QueueInfo()
        q.current = current?.displayTitle ?? ""
        q.currentKind = current?.kind ?? ""
        q.audio = current?.isRemoteAudio ?? false
        q.playing = q.audio && !paused
        q.clip = current?.isClip ?? false
        q.keepTitle = keepListeningTitle ?? ""
        if let p = remotePosition() {
            q.elapsed = p.elapsed; q.duration = p.duration ?? 0
            q.elapsedLabel = QueueClock.label(p.elapsed)
            if let d = p.duration { q.remainingLabel = "-" + QueueClock.label(max(0, d - p.elapsed)); q.fraction = min(1, max(0, p.elapsed / d)) }
        }
        q.upcoming = entries.enumerated().map { i, e in
            var r = QueueRow(); r.i = i; r.title = e.item.displayTitle; r.kind = e.item.kind; r.id = e.id
            if case .audio(_, _, let show, _, _, _) = e.item { r.detail = show }
            r.note = e.note
            r.by = e.addedBy == .reader ? "Added by you" : (e.addedBy == .dj ? "Added by the DJ" : "From the station")
            return r
        }
        q.message = message
        return q
    }

    /// The remote audio shown as the radio: the bar's line, the player's title and position, and its play button follow the audio.
    public func overlay(_ r: RadioInfo) -> RadioInfo {
        guard case .audio(_, let title, let show, _, _, _)? = current else { return r }
        var o = r
        let i = info()
        o.title = title; o.kind = show.isEmpty ? i.currentKind.uppercased() : show; o.source = ""
        o.position = i.duration > 0 ? "\(i.elapsedLabel) of \(QueueClock.label(i.duration))" : i.elapsedLabel
        o.playing = i.playing; o.paused = !i.playing
        o.progress = i.fraction; o.blockProgress = i.fraction
        o.sentences = []; o.sentenceIndex = 0; o.items = []; o.next = ""; o.sources = []
        o.count = 1; o.index = 0
        return o
    }
    public func overlay(_ b: BarInfo) -> BarInfo {
        guard current?.isRemoteAudio == true, b.state == "idle" || b.state == "playing" || b.state == "paused" else { return b }
        var o = b
        let i = info()
        o.state = i.playing ? "playing" : "paused"; o.text = i.current; o.playing = i.playing; o.progress = i.fraction
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

/// The item a screen's "Play" or "Add to queue" means. `kind` is "story" (ref a route), "page" (ref an address).
public enum QueueRequest {
    @MainActor public static func item(kind: String, ref: String, title: String) -> PlayItem? {
        switch kind {
        case "story":
            let r = ref.hasPrefix("pub:") ? String(ref.dropFirst(4)) : ref
            return r.isEmpty ? nil : .story(route: r, title: title)
        case "page":
            guard case .inApp = SourceLink.route(ref), let c = EmbeddedPage.shared.current(for: ref) else { return nil }
            let t = title.isEmpty ? c.title : (c.title.isEmpty ? title : c.title)
            return .page(url: c.url, title: t.isEmpty ? (c.url.host ?? "") : t)
        default: return nil
        }
    }
}
