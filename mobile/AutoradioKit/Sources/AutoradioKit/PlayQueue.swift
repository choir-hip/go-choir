import Foundation

// The one playback queue (lane autoradio-queue). Everything the radio plays, a paper story, a web page, an imported book, remote audio,
// goes through `PlayQueue.shared`; it is the only thing that decides what plays next. The paper's list and the DJ's stretch stay the
// default source: when the queue has nothing of its own behind the item that just ended, the existing behaviour carries on.
// The player itself is behind `PlaybackEngine`, so the queue is tested with a fake one; the real adapters are `ChoirApp` (stories and
// spoken text, through the existing `ProgrammePlayer`) and `RemoteAudioPlayer` (AVPlayer).

// MARK: The shape the other lanes code against (do not rename; add only)

public enum PlayItem: Equatable, Codable, Sendable {
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
    public var isRemoteAudio: Bool { if case .audio = self { return true }; return false }
    /// A clip: remote audio with an end.
    public var isClip: Bool { if case .audio(_, _, _, _, let end, _) = self { return end != nil }; return false }
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
}

// MARK: A URL in the prompt bar

public enum BarURL {
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
        guard let tld = labels.last, tld.count >= 2, tld.allSatisfy({ $0.isLetter }) else { return nil }
        guard labels.allSatisfy({ $0.allSatisfy { $0.isLetter || $0.isNumber || $0 == "-" } }) else { return nil }
        return URL(string: "https://" + t)
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

// MARK: The queue

@MainActor public final class PlayQueue {
    public static let shared = PlayQueue()

    public private(set) var current: PlayItem?
    public private(set) var upcoming: [PlayItem] = []
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

    // MARK: Putting things in the queue

    /// Play this now. What was playing is replaced (it is not put back); what was queued stays queued.
    public func playNow(_ item: PlayItem) { startTask = Task { await self.start(item) } }
    /// After the one playing, before anything else queued.
    public func playNext(_ item: PlayItem) { upcoming.insert(item, at: 0); changed() }
    /// At the end.
    public func enqueue(_ item: PlayItem) { upcoming.append(item); changed() }
    /// Take an item out of "Up next".
    public func remove(at i: Int) { guard upcoming.indices.contains(i) else { return }; upcoming.remove(at: i); changed() }
    /// Tap an item in "Up next": play it now; the ones before it are dropped, like a playlist jump.
    public func playUpcoming(at i: Int) {
        guard upcoming.indices.contains(i) else { return }
        let item = upcoming[i]
        upcoming.removeSubrange(0...i)
        playNow(item)
    }
    public func clearUpcoming() { upcoming.removeAll(); changed() }

    /// Wait for the start in flight (tests, and the host that wants to know it began).
    public func settle() async { await startTask?.value }

    /// Something began playing outside the queue (the paper's list moved to its next story): the queue shows it, and does not drive it.
    public func noteStarted(_ item: PlayItem?) { current = item; paused = false; documentBlock = 0; restoredIdle = false; changed() }

    // MARK: Starting

    @discardableResult
    func start(_ item: PlayItem) async -> Bool {
        generation += 1
        let my = generation
        current = item; paused = false; message = ""; restoredIdle = false
        documentBlock = { if case .document(_, _, let at) = item { return at }; return 0 }()
        let resume = resumeSeconds; resumeSeconds = nil
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
        if !ok { await advance(); return false }       // what cannot play is skipped
        changed()
        return true
    }

    /// The current item is over: the next queued one plays; with none, `defaultSource` (not for a paper story: the paper's list is its own default).
    @discardableResult
    func advance(useDefault: Bool = true) async -> Bool {
        if !upcoming.isEmpty {
            let n = upcoming.removeFirst()
            return await start(n)
        }
        current = nil; paused = false; changed()
        if useDefault, let d = defaultSource { return await d() }
        return false
    }

    /// Skip to what is queued next (the lock screen's and the player's next while the queue has something).
    public func skip() { if !upcoming.isEmpty { startTask = Task { await self.advance() } } }
    public var hasUpcoming: Bool { !upcoming.isEmpty }

    // MARK: Ends, reported by the engines

    /// A paper story ran out of blocks. True: the queue started its own next item (the paper's list must not also advance).
    public func storyEnded(route: String) async -> Bool {
        guard case .story(let r, _)? = current, r == route, !upcoming.isEmpty else { return false }
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
        if restoredIdle { restoredIdle = false; startTask = Task { await self.start(c) }; return }   // a queue from the last launch: first play
        guard c.isRemoteAudio else { return }
        paused = false; engine?.resumeRemote(); changed()
    }
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
        save(); changed()
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

    struct Saved: Codable { var current: PlayItem?; var upcoming: [PlayItem]; var seconds: Double?; var block: Int }

    func changed() { save(); onChange?() }

    public func save() {
        guard let storage else { return }
        let elapsed = current?.isRemoteAudio == true ? engine?.remotePosition()?.elapsed ?? resumeSeconds : nil
        let s = Saved(current: current, upcoming: upcoming, seconds: elapsed, block: documentBlock)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: storage, options: .atomic) }
    }

    /// The queue and where it was, from the last launch. Nothing plays; the first play resumes the current item.
    public func restore() {
        guard let storage, let d = try? Data(contentsOf: storage), let s = try? JSONDecoder().decode(Saved.self, from: d) else { return }
        current = s.current; upcoming = s.upcoming; resumeSeconds = s.seconds; restoredIdle = s.current != nil; paused = s.current != nil
        documentBlock = s.block
        if case .document(let id, let title, _)? = current { current = .document(id: id, title: title, at: s.block) }
        onChange?()
    }
}
