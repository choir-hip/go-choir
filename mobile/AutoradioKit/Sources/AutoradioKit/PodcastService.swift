import Foundation

// The podcast app's state and pages. Where each thing lives (decided from the server's code, 2026-11-15):
//  - Show search and the suggestions: Apple's public directory, asked from the phone (the server's /api/podcast/search asks the same
//    API, and needs the computer awake; the phone has no need of it).
//  - Episodes: the publisher's RSS feed, fetched and parsed on the phone (the server keeps only raw feed XML), cached on disk.
//  - Subscriptions: kept on the phone (Application Support JSON). The phone's key is a full-access key, and /api/podcast/subscriptions
//    (GET, POST) is reachable with it like /api/email/*, so a subscribe is also pushed to the computer and the computer's list is
//    pulled in. The server has NO unsubscribe route: an unsubscribe is local; the feed stays in the computer's list, and the phone
//    remembers it was removed so a pull never brings it back.
//  - Resume positions: kept on the phone, read from the queue's engine (`remotePosition`) while an episode plays.
//  - Playback: only through the shared queue, by the hooks below (the adapter file in the app module sets them).
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. Subscribing twice, unsubscribing a show that is not subscribed, a feed written two ways (case, trailing slash): one subscription.
//  2. A feed that fails to load: a plain sentence and a retry, never an empty page; a cached copy is shown when there is one.
//  3. A clip opens an episode whose feed is not loaded yet, or whose episode has left the feed: the page still offers play from the clip's
//     own enclosure URL, with the title the clip carried.
//  4. A play position reported for an episode that is not a podcast id, or for no duration: ignored / kept without a duration.
//  5. Unplayed: no position at all. Started: a position. Finished: within 20 s of the end, or reported finished.
//  6. A search answered late for an older query: dropped. Directory failing: a sentence; subscriptions still work.
//  7. The server refusing or not answering a sync: the phone's list is unchanged and a note says it is not synced.
//  8. The library file missing, or damaged: an empty library, never a crash.

public struct PodcastPosition: Codable, Equatable, Sendable {
    public var seconds: Double
    public var duration: Double?
    public var finished: Bool
    public var at: Double
}

public struct PodcastSyncHooks: Sendable {
    public var signedIn: @Sendable () -> Bool
    public var push: @Sendable (PodcastShow) async -> Bool
    public var pull: @Sendable () async -> [PodcastShow]?
    public init(signedIn: @escaping @Sendable () -> Bool, push: @escaping @Sendable (PodcastShow) async -> Bool, pull: @escaping @Sendable () async -> [PodcastShow]?) {
        self.signedIn = signedIn; self.push = push; self.pull = pull
    }
}

public struct PodcastPlayRequest: Equatable, Sendable {
    public var url: URL, title: String, show: String
    public var start: Double?, end: Double?
    public var episodeId: String
}

/// What the queue is playing, as the podcast pages need it.
public struct PodcastNow: Equatable, Sendable {
    public var episodeId: String
    public var isClip: Bool
    public var start: Double?
    public var elapsed: Double?
    public var duration: Double?
    public init(episodeId: String, isClip: Bool, start: Double?, elapsed: Double?, duration: Double?) {
        self.episodeId = episodeId; self.isClip = isClip; self.start = start; self.elapsed = elapsed; self.duration = duration
    }
}

/// The platform's shared podcast routes (provisional shapes, owner direction 2026-11-15): the server transcribes the shows readers follow and
/// recommends what other readers follow. A 404 or any failure is "not available yet" and is silent.
public struct PodcastPlatformHooks: Sendable {
    public var report: @Sendable ([PodcastShow], PodcastNowPlaying?) async -> Void
    public var popular: @Sendable () async -> [PodcastShow]?
    /// The shared transcript of an episode; nil when the platform has none (404, signed out, offline).
    public var transcript: @Sendable (String, String) async -> PodcastTranscript?
    public init(report: @escaping @Sendable ([PodcastShow], PodcastNowPlaying?) async -> Void, popular: @escaping @Sendable () async -> [PodcastShow]?,
                transcript: @escaping @Sendable (String, String) async -> PodcastTranscript? = { _, _ in nil }) {
        self.report = report; self.popular = popular; self.transcript = transcript
    }
}
public struct PodcastTranscript: Equatable, Sendable {
    public struct Block: Equatable, Sendable { public var start: Double; public var end: Double; public var text: String
        public init(start: Double, end: Double, text: String) { self.start = start; self.end = end; self.text = text } }
    public var durationSeconds: Double?
    public var source: String
    public var blocks: [Block]
    public init(durationSeconds: Double?, source: String, blocks: [Block]) { self.durationSeconds = durationSeconds; self.source = source; self.blocks = blocks }
}
public struct PodcastNowPlaying: Equatable, Sendable { public var feedURL: String, episodeGuid: String, audioURL: String }

public typealias PodcastFetch = @Sendable (URL) async throws -> Data

private struct LibraryFile: Codable {
    struct Sub: Codable, Equatable { var show: PodcastShow; var at: Double }
    var subscriptions: [Sub] = []
    var removed: [String] = []
    var synced: [String] = []
    var positions: [String: PodcastPosition] = [:]
}

public final class PodcastService: @unchecked Sendable {
    public static let shared = PodcastService()
    public static let staleAfter = 15.0 * 60
    public static let finishedWithin = 20.0
    public static let searchPause = 0.35

    private let lock = NSLock()
    private var dir: URL?
    private var lib = LibraryFile()
    private var feeds: [String: PodcastFeed] = [:]
    private var known: [String: PodcastShow] = [:]
    private var inflight = Set<String>()
    private var failed: [String: (message: String, at: Double)] = [:]
    private var hints: [String: Hint] = [:]
    private var fetchData: PodcastFetch
    private let clock: @Sendable () -> Double

    struct Hint { var enclosure: String; var start: Double?; var title: String; var show: String }

    // Hooks the host sets.
    public var onChange: (@Sendable () -> Void)?
    public var opener: (@Sendable (String, String) -> Void)?
    public var playHook: (@Sendable (PodcastPlayRequest, Bool) -> Void)?
    public var nowHook: (@Sendable () -> PodcastNow?)?
    public var keepHook: (@Sendable () -> Void)?
    public var syncHooks: PodcastSyncHooks?
    public var platform: PodcastPlatformHooks?
    private var reportedThisLaunch = false
    private var transcripts: [String: PodcastTranscript] = [:]
    private var transcriptAsked: [String: Double] = [:]
    private var playedKeys = Set<String>()

    // Search and suggestions.
    private var searchQuery = ""
    private var searchState: SearchState = .idle
    private var searchTask: Task<Void, Never>?
    private var suggestions: [PodcastShow] = []
    private var suggestionsAt = 0.0
    private var suggesting = false
    private var syncAt = 0.0
    private var syncing = false
    private var syncNote = ""

    public enum SearchState: Equatable, Sendable { case idle, searching, done([PodcastShow]), failed }

    public init(directory: URL? = nil, fetch: @escaping PodcastFetch = PodcastService.liveFetch, clock: @escaping @Sendable () -> Double = { Date().timeIntervalSince1970 }) {
        self.dir = directory; self.fetchData = fetch; self.clock = clock
        loadLibrary()
    }

    /// The host calls this once with Application Support.
    public func configure(directory: URL) {
        lock.lock(); dir = directory; lock.unlock()
        loadLibrary()
    }

    public static let liveFetch: PodcastFetch = { url in
        var r = URLRequest(url: url, timeoutInterval: 30)
        r.setValue("Autoradio", forHTTPHeaderField: "User-Agent")
        r.setValue("application/rss+xml, application/xml, text/xml, application/json, */*", forHTTPHeaderField: "Accept")
        let (d, resp) = try await URLSession.shared.data(for: r)
        if let h = resp as? HTTPURLResponse, !(200..<300).contains(h.statusCode) { throw ChoirError.server(status: h.statusCode, message: "") }
        if d.count > 25_000_000 { throw PodcastError.tooBig }
        return d
    }

    // MARK: Sign-out

    /// Sign-out: the library (subscriptions, positions, removed and synced sets), every cached feed and transcript, and the files
    /// (`podcasts-library.json`, `podcast-feeds/`). A sync or feed load in flight finds the account generation changed and keeps nothing.
    public func wipe() {
        lock.lock()
        lib = LibraryFile(); feeds = [:]; known = [:]; inflight = []; failed = [:]; hints = [:]
        transcripts = [:]; transcriptAsked = [:]; playedKeys = []
        searchTask?.cancel(); searchTask = nil; searchQuery = ""; searchState = .idle
        suggestions = []; suggestionsAt = 0; suggesting = false
        syncAt = 0; syncing = false; syncNote = ""; reportedThisLaunch = false
        let d = dir
        lock.unlock()
        if let d {
            try? FileManager.default.removeItem(at: d.appendingPathComponent("podcasts-library.json"))
            try? FileManager.default.removeItem(at: d.appendingPathComponent("podcast-feeds"))
        }
        changed()
    }

    // MARK: Library file

    private var libraryFile: URL? { dir?.appendingPathComponent("podcasts-library.json") }
    private func loadLibrary() {
        lock.lock(); defer { lock.unlock() }
        guard let f = libraryFile, let d = try? Data(contentsOf: f), let l = try? JSONDecoder().decode(LibraryFile.self, from: d) else { return }
        lib = l
        for s in l.subscriptions { known[PodcastKey.feed(s.show.feedURL)] = s.show }
    }
    private func saveLocked() {
        guard let f = libraryFile, let d = try? JSONEncoder().encode(lib) else { return }
        try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? d.write(to: f, options: .atomic)
    }
    private func feedFile(_ norm: String) -> URL? {
        var h: UInt64 = 0xcbf29ce484222325
        for b in norm.utf8 { h = (h ^ UInt64(b)) &* 0x100000001b3 }
        return dir?.appendingPathComponent("podcast-feeds").appendingPathComponent(String(h, radix: 16) + ".json")
    }
    private func changed() { onChange?() }

    // MARK: Subscriptions

    public func isSubscribed(_ feedURL: String) -> Bool {
        let n = PodcastKey.feed(feedURL); lock.lock(); defer { lock.unlock() }
        return lib.subscriptions.contains { PodcastKey.feed($0.show.feedURL) == n }
    }
    func cachedTranscript(feed: String, id: String) -> PodcastTranscript? {
        let key = PodcastKey.episode(feed: PodcastKey.feed(feed), id: id)
        lock.lock(); defer { lock.unlock() }; return transcripts[key]
    }
    public func subscriptions() -> [PodcastShow] { lock.lock(); defer { lock.unlock() }; return lib.subscriptions.map(\.show) }

    @discardableResult
    public func subscribe(_ show: PodcastShow) -> Bool {
        let n = PodcastKey.feed(show.feedURL)
        lock.lock()
        if lib.subscriptions.contains(where: { PodcastKey.feed($0.show.feedURL) == n }) { lock.unlock(); return false }
        var s = show
        if let k = known[n] { if s.title.isEmpty { s.title = k.title }; if s.artworkURL.isEmpty { s.artworkURL = k.artworkURL }; if s.author.isEmpty { s.author = k.author } }
        lib.subscriptions.append(.init(show: s, at: clock()))
        lib.removed.removeAll { PodcastKey.feed($0) == n }
        known[n] = s
        saveLocked(); lock.unlock()
        changed()
        if let hooks = syncHooks, hooks.signedIn() {
            let gen = AccountWipe.generation
            Task { [weak self] in
                if await hooks.push(s), gen == AccountWipe.generation { self?.markSynced(n) }
            }
        }
        reportFeeds()
        refreshIfNeeded(s.feedURL, force: true)
        return true
    }

    /// Subscribe by feed address alone (a show the page was opened for), with what the directory or feed has told us about it.
    @discardableResult
    public func subscribe(feedURL: String) -> Bool {
        subscribe(knownShow(feedURL) ?? PodcastShow(feedURL: feedURL, title: cachedFeed(feedURL)?.show.title ?? feedURL))
    }

    @discardableResult
    public func unsubscribe(_ feedURL: String) -> Bool {
        let n = PodcastKey.feed(feedURL)
        lock.lock()
        let before = lib.subscriptions.count
        lib.subscriptions.removeAll { PodcastKey.feed($0.show.feedURL) == n }
        let did = lib.subscriptions.count != before
        if did, !lib.removed.contains(where: { PodcastKey.feed($0) == n }) { lib.removed.append(feedURL) }
        if did { saveLocked() }
        lock.unlock()
        if did { changed(); reportFeeds() }
        return did
    }

    /// Tell the platform which feeds this reader follows (and what is playing now), so the server can transcribe them first.
    public func reportFeeds(nowPlaying: PodcastNowPlaying? = nil) {
        guard let hooks = platform else { return }
        let feeds = subscriptions()
        Task { await hooks.report(feeds, nowPlaying) }
    }

    private func markSynced(_ n: String) {
        lock.lock(); if !lib.synced.contains(n) { lib.synced.append(n); saveLocked() }; lock.unlock()
        changed()
    }

    // MARK: Positions

    public func position(_ key: String) -> PodcastPosition? { lock.lock(); defer { lock.unlock() }; return lib.positions[key] }

    /// The playhead of an episode, from the queue's engine. `episodeId` is the queue's id for it (`PodcastKey.episode`).
    public func report(episodeId: String, seconds: Double, duration: Double?, finished: Bool = false) {
        guard PodcastKey.parseEpisode(episodeId) != nil, seconds.isFinite, seconds >= 0 else { return }
        lock.lock()
        let done = finished || (duration.map { $0 > 0 && seconds >= $0 - Self.finishedWithin } ?? false)
        lib.positions[episodeId] = PodcastPosition(seconds: seconds, duration: duration ?? lib.positions[episodeId]?.duration, finished: done, at: clock())
        saveLocked(); lock.unlock()
    }

    /// Read the queue's playhead now and keep it.
    public func sample() {
        if let n = nowHook?(), let e = n.elapsed, e > 0 { report(episodeId: n.episodeId, seconds: e, duration: n.duration) }
    }

    /// Where "Resume" starts: nil when never started or finished.
    public func resumeSeconds(_ key: String) -> Double? {
        guard let p = position(key), !p.finished, p.seconds >= 5 else { return nil }
        return p.seconds
    }

    // MARK: Feeds

    public func cachedFeed(_ feedURL: String) -> PodcastFeed? {
        let n = PodcastKey.feed(feedURL)
        lock.lock()
        if let f = feeds[n] { lock.unlock(); return f }
        lock.unlock()
        guard let file = feedFile(n), let d = try? Data(contentsOf: file), let f = try? JSONDecoder().decode(PodcastFeed.self, from: d) else { return nil }
        lock.lock(); feeds[n] = f; lock.unlock()
        return f
    }

    /// Ask for the feed in the background when there is none, it is stale, or `force`. One ask per feed at a time; a failure waits a minute.
    public func refreshIfNeeded(_ feedURL: String, force: Bool = false) {
        let n = PodcastKey.feed(feedURL), now = clock()
        let have = cachedFeed(feedURL)
        lock.lock()
        let fresh = have.map { now - $0.fetchedAt < Self.staleAfter } ?? false
        let cooling = failed[n].map { now - $0.at < 60 } ?? false
        if inflight.contains(n) || (!force && (fresh || cooling)) { lock.unlock(); return }
        inflight.insert(n); lock.unlock()
        changed()
        Task { [weak self] in await self?.load(feedURL, norm: n) }
    }

    public func load(_ feedURL: String, norm n: String) async {
        var candidates = [PodcastText.https(feedURL)]
        if candidates[0] != feedURL { candidates.append(feedURL) }
        let gen = AccountWipe.generation
        var lastError = "The feed could not be reached."
        var result: PodcastFeed?
        for c in candidates {
            guard let u = URL(string: c), u.scheme != nil else { lastError = PodcastError.badURL.description; continue }
            do {
                let data = try await fetchData(u)
                result = try RSSFeedParser.parse(data, feedURL: feedURL, fetchedAt: clock())
                break
            } catch let e as PodcastError { lastError = e.description }
            catch is CancellationError { break }
            catch { lastError = "The feed could not be reached. Check your connection." }
        }
        lock.lock()
        if gen != AccountWipe.generation { lock.unlock(); return }          // signed out while the feed was fetched: it is not kept
        inflight.remove(n)
        if var f = result {
            // Keep what the subscription knew (a title from the directory beats a feed with none).
            if let k = known[n] { if f.show.author.isEmpty { f.show.author = k.author }; if f.show.artworkURL.isEmpty { f.show.artworkURL = k.artworkURL } }
            feeds[n] = f; failed[n] = nil
            known[n] = f.show
            if let i = lib.subscriptions.firstIndex(where: { PodcastKey.feed($0.show.feedURL) == n }) { lib.subscriptions[i].show = f.show; saveLocked() }
            if let file = feedFile(n), let d = try? JSONEncoder().encode(f) {
                try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
                try? d.write(to: file, options: .atomic)
            }
        } else { failed[n] = (lastError, clock()) }
        lock.unlock()
        changed()
    }

    func failure(_ feedURL: String) -> String? { lock.lock(); defer { lock.unlock() }; return failed[PodcastKey.feed(feedURL)]?.message }
    func loading(_ feedURL: String) -> Bool { lock.lock(); defer { lock.unlock() }; return inflight.contains(PodcastKey.feed(feedURL)) }
    func remember(_ s: PodcastShow) { lock.lock(); if known[PodcastKey.feed(s.feedURL)] == nil { known[PodcastKey.feed(s.feedURL)] = s }; lock.unlock() }
    public func knownShow(_ feedURL: String) -> PodcastShow? { lock.lock(); defer { lock.unlock() }; return known[PodcastKey.feed(feedURL)] }

    // MARK: Directory search and suggestions

    public func searchStatus(for raw: String) -> SearchState {
        let q = CorpusSearch.normalize(raw)
        lock.lock(); defer { lock.unlock() }
        if q.isEmpty { searchTask?.cancel(); searchTask = nil; searchQuery = ""; searchState = .idle; return .idle }
        if q == searchQuery { return searchState }
        searchTask?.cancel()
        searchQuery = q; searchState = .searching
        searchTask = Task { [weak self] in await self?.runSearch(q) }
        return .searching
    }

    private func runSearch(_ q: String) async {
        do { try await Task.sleep(nanoseconds: UInt64(Self.searchPause * 1_000_000_000)) } catch { return }
        let result: SearchState
        do {
            guard let u = PodcastDirectory.searchURL(q) else { throw PodcastError.badURL }
            let shows = try PodcastDirectory.decodeSearch(try await fetchData(u))
            result = .done(shows)
        } catch is CancellationError { return } catch { result = .failed }
        lock.lock()
        let fresh = q == searchQuery && !Task.isCancelled
        if fresh { searchState = result; if case .done(let s) = result { for x in s { let n = PodcastKey.feed(x.feedURL); if known[n] == nil { known[n] = x } } } }
        lock.unlock()
        if fresh { changed() }
    }

    /// Top news shows of the directory's chart, asked once per ten minutes. None when the chart or lookup fails: nothing is made up.
    public func loadSuggestionsIfNeeded() {
        lock.lock()
        let now = clock()
        if suggesting || (suggestionsAt > 0 && now - suggestionsAt < 600) { lock.unlock(); return }
        suggesting = true; lock.unlock()
        Task { [weak self] in
            guard let self else { return }
            var out: [PodcastShow] = []
            if let pop = await self.platform?.popular(), !pop.isEmpty { out = pop }
            if out.isEmpty { do {
                let chart = try PodcastDirectory.decodeChartNews(try await self.fetchData(PodcastDirectory.topURL))
                if !chart.isEmpty, let u = PodcastDirectory.lookupURL(ids: chart.map(\.id)) {
                    let found = try PodcastDirectory.decodeSearch(try await self.fetchData(u))
                    out = found
                }
            } catch {} }
            self.lock.lock(); self.suggestions = out; self.suggestionsAt = self.clock(); self.suggesting = false
            for x in out { let n = PodcastKey.feed(x.feedURL); if self.known[n] == nil { self.known[n] = x } }
            self.lock.unlock()
            self.changed()
        }
    }

    // MARK: Sync with the computer

    public func syncIfDue() {
        guard let hooks = syncHooks, hooks.signedIn() else { return }
        lock.lock()
        let now = clock()
        if syncing || (syncAt > 0 && now - syncAt < 600) { lock.unlock(); return }
        syncing = true; syncAt = now; let startEpoch = AccountWipe.generation; lock.unlock()
        Task { [weak self] in
            guard let self else { return }
            var note = ""
            if let list = await hooks.pull() {
                self.lock.lock()
                if AccountWipe.generation != startEpoch { self.lock.unlock(); return }          // signed out meanwhile: this is not the account that asked
                let removed = Set(self.lib.removed.map(PodcastKey.feed))
                var added: [PodcastShow] = []
                for s in list {
                    let n = PodcastKey.feed(s.feedURL)
                    guard !removed.contains(n), !self.lib.subscriptions.contains(where: { PodcastKey.feed($0.show.feedURL) == n }) else { continue }
                    self.lib.subscriptions.append(.init(show: s, at: self.clock())); self.known[n] = s
                    if !self.lib.synced.contains(n) { self.lib.synced.append(n) }
                    added.append(s)
                }
                let onServer = Set(list.map { PodcastKey.feed($0.feedURL) })
                let toPush = self.lib.subscriptions.map(\.show).filter { !onServer.contains(PodcastKey.feed($0.feedURL)) }
                if !added.isEmpty { self.saveLocked() }
                self.lock.unlock()
                var failedPush = 0
                for s in toPush {
                    self.lock.lock(); let same = AccountWipe.generation == startEpoch; self.lock.unlock()
                    if !same { return }
                    if await hooks.push(s) { self.markSynced(PodcastKey.feed(s.feedURL)) } else { failedPush += 1 } }
                note = failedPush == 0 ? "In step with your computer's podcasts." : "Some shows could not be sent to your computer yet."
                for s in added { self.refreshIfNeeded(s.feedURL) }
            } else { note = "Not in step with your computer's podcasts yet: it did not answer. Subscriptions here are kept on this phone." }
            self.lock.lock(); self.syncing = false; self.syncNote = note; self.lock.unlock()
            self.changed()
        }
    }

    // MARK: Transcripts (the platform's shared ones)

    /// Ask for the transcript once per two minutes while there is none.
    func transcriptIfNeeded(feed: String, id: String) {
        guard let hooks = platform else { return }
        let key = PodcastKey.episode(feed: PodcastKey.feed(feed), id: id), now = clock()
        lock.lock()
        if transcripts[key] != nil || (transcriptAsked[key].map { now - $0 < 120 } ?? false) { lock.unlock(); return }
        transcriptAsked[key] = now; lock.unlock()
        Task { [weak self] in
            guard let t = await hooks.transcript(feed, id), let self else { return }
            self.lock.lock(); self.transcripts[key] = t; self.lock.unlock()
            self.changed()
        }
    }

    // MARK: Clip to episode

    func setHint(feed: String, id: String, enclosure: String, start: Double?, title: String, show: String) {
        lock.lock(); hints[PodcastKey.episode(feed: PodcastKey.feed(feed), id: id)] = Hint(enclosure: enclosure, start: start, title: title, show: show); lock.unlock()
    }
    func hint(_ key: String) -> Hint? { lock.lock(); defer { lock.unlock() }; return hints[key] }

    // MARK: Addresses

    public static let homeAddress = "podcasts:"
    public static func showAddress(_ feedURL: String) -> String { "podcast:show/" + PathSegment.encode(feedURL) }
    public static func episodeAddress(feed: String, id: String) -> String { "podcast:episode/" + PathSegment.encode(feed) + "/" + PathSegment.encode(id) }
    public static func parseShow(_ address: String) -> String? {
        let r = PageAddress.rest(of: address)
        guard address.hasPrefix("podcast:"), r.hasPrefix("show/") else { return nil }
        return String(r.dropFirst(5)).removingPercentEncoding.flatMap { $0.isEmpty ? nil : $0 }
    }
    public static func parseEpisode(_ address: String) -> (feed: String, id: String)? {
        let r = PageAddress.rest(of: address)
        guard address.hasPrefix("podcast:"), r.hasPrefix("episode/") else { return nil }
        let parts = r.dropFirst(8).split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
        guard parts.count == 2, let f = String(parts[0]).removingPercentEncoding, let i = String(parts[1]).removingPercentEncoding, !f.isEmpty, !i.isEmpty else { return nil }
        return (f, i)
    }

    // MARK: Views (what the pages read)

    public struct ShowRow: Codable, Equatable, Sendable {
        public var feedURL = "", address = "", title = "", author = "", artwork = "", detail = "", latest = "", latestStamp = ""
        public var subscribed = false, unplayed = false
        public var unplayedCount = 0
    }
    public struct HomeView: Codable, Equatable, Sendable {
        public var status = "ready", message = "", query = "", searchNote = "", syncNote = ""
        public var searching = false, hasQuery = false
        public var shows: [ShowRow] = [], results: [ShowRow] = [], suggestions: [ShowRow] = []
    }
    public struct EpisodeRow: Codable, Equatable, Sendable {
        public var key = "", address = "", title = "", stamp = "", duration = "", snippet = "", progress = ""
        public var unplayed = false, playable = false, current = false
    }
    public struct ShowView: Codable, Equatable, Sendable {
        public var status = "loading", message = "", feedURL = "", title = "", author = "", artwork = "", summary = "", unplayedLine = ""
        public var subscribed = false, refreshing = false
        public var episodes: [EpisodeRow] = []
    }
    public struct TranscriptRow: Codable, Equatable, Sendable {
        public var key = "", time = "", text = ""
        public var seconds = 0.0
        public var current = false, seekable = false
    }
    public struct EpisodeView: Codable, Equatable, Sendable {
        public var status = "loading", message = "", address = "", feedURL = "", showTitle = "", showAddress = "", title = "", stamp = "", duration = "", artwork = ""
        public var playingFrom = "", resumeLabel = "", note = "", transcriptState = "", transcriptNote = "", transcriptSource = ""
        public var transcript: [TranscriptRow] = []
        public var notes: [String] = []
        public var subscribed = false, playable = false, current = false, fromClip = false, hasResume = false
    }
    public struct Ack: Codable, Equatable, Sendable { public var status = "ok", message = "" }

    private func row(_ show: PodcastShow, subscribed: Bool) -> ShowRow {
        var r = ShowRow(feedURL: show.feedURL, address: Self.showAddress(show.feedURL), title: show.title, author: show.author,
                        artwork: PodcastText.https(show.artworkURL), subscribed: subscribed)
        if let f = cachedFeed(show.feedURL), let e = f.episodes.first {
            let n = unplayedCount(f)
            r.latest = e.title; r.latestStamp = PodcastText.relative(e.published, now: clock())
            r.unplayedCount = n; r.unplayed = n > 0
            r.detail = n > 0 ? "\(n) unplayed · \(e.title)" : e.title
        } else { r.detail = show.author }
        return r
    }
    private func unplayedCount(_ f: PodcastFeed) -> Int {
        f.episodes.filter { $0.playable && position(PodcastKey.episode(feed: f.show.feedURL, id: $0.id)) == nil }.count
    }

    public func home(query raw: String) -> HomeView {
        syncIfDue(); loadSuggestionsIfNeeded()
        lock.lock(); let first = !reportedThisLaunch; reportedThisLaunch = true; lock.unlock()
        if first { reportFeeds() }
        var v = HomeView()
        let q = CorpusSearch.normalize(raw)
        v.query = q; v.hasQuery = !q.isEmpty
        let subs = subscriptions()
        for s in subs { refreshIfNeeded(s.feedURL) }
        v.shows = subs.map { row($0, subscribed: true) }.enumerated().sorted { a, b in
            let ta = cachedFeed(a.element.feedURL)?.episodes.first?.published ?? 0, tb = cachedFeed(b.element.feedURL)?.episodes.first?.published ?? 0
            return ta != tb ? ta > tb : a.offset < b.offset
        }.map(\.element)
        v.syncNote = { lock.lock(); defer { lock.unlock() }; return syncNote }()
        if q.isEmpty {
            lock.lock(); let sug = suggestions; lock.unlock()
            v.suggestions = sug.filter { !isSubscribed($0.feedURL) }.map { row($0, subscribed: false) }
        } else {
            switch searchStatus(for: q) {
            case .idle: break
            case .searching: v.searching = true; v.searchNote = "Searching podcasts…"
            case .failed: v.searchNote = "Could not search podcasts right now. Check your connection and try again."
            case .done(let found):
                v.results = found.map { row($0, subscribed: isSubscribed($0.feedURL)) }
                if found.isEmpty { v.searchNote = "No podcasts found for that." }
            }
        }
        v.status = subs.isEmpty ? "empty" : "ready"
        v.message = subs.isEmpty ? "You have not subscribed to any podcasts yet. Search for a show above, subscribe, and its episodes will be here." : ""
        return v
    }

    func episodeRow(_ e: PodcastEpisode, feed: String, now: PodcastNow?) -> EpisodeRow {
        let key = PodcastKey.episode(feed: feed, id: e.id)
        let pos = position(key)
        var progress = ""
        if let p = pos {
            if p.finished { progress = "Played" }
            else if let d = p.duration ?? e.duration, d > p.seconds { progress = PodcastText.durationLabel(d - p.seconds) + " left" }
            else if p.seconds >= 5 { progress = "At " + PodcastText.clock(p.seconds) }
        }
        let snippet = e.notes.split(separator: "\n").first.map(String.init) ?? ""
        return EpisodeRow(key: key, address: Self.episodeAddress(feed: feed, id: e.id), title: e.title, stamp: PodcastText.relative(e.published, now: clock()),
                          duration: PodcastText.durationLabel(e.duration), snippet: snippet.count > 140 ? String(snippet.prefix(139)) + "…" : snippet,
                          progress: progress, unplayed: pos == nil && e.playable, playable: e.playable, current: now?.episodeId == key)
    }

    /// A transcript's times are good for this file when its length is within 3 s of the transcribed one's.
    public static func durationsAgree(_ file: Double?, _ transcribed: Double?) -> Bool {
        guard let f = file, let t = transcribed else { return false }
        return abs(f - t) <= 3
    }

    public func show(address: String) -> ShowView {
        var v = ShowView()
        guard let feed = Self.parseShow(address) else { v.status = "error"; v.message = "This show's address is not one the app can open."; return v }
        sample()
        refreshIfNeeded(feed)
        let k = knownShow(feed)
        v.feedURL = feed; v.subscribed = isSubscribed(feed); v.refreshing = loading(feed)
        v.title = k?.title ?? ""; v.author = k?.author ?? ""; v.artwork = PodcastText.https(k?.artworkURL ?? ""); v.summary = k?.summary ?? ""
        if let f = cachedFeed(feed) {
            v.status = "ready"
            v.title = f.show.title; v.author = f.show.author.isEmpty ? v.author : f.show.author
            if !f.show.artworkURL.isEmpty { v.artwork = f.show.artworkURL }
            v.summary = f.show.summary.isEmpty ? v.summary : f.show.summary
            let now = nowHook?()
            v.episodes = f.episodes.map { episodeRow($0, feed: f.show.feedURL, now: now) }
            let n = unplayedCount(f)
            v.unplayedLine = n > 0 ? "\(n) unplayed" : ""
            if v.episodes.isEmpty { v.status = "empty"; v.message = "This feed has no episodes yet." }
        } else if let m = failure(feed) { v.status = "error"; v.message = m }
        else { v.status = "loading" }
        return v
    }

    public func episode(address: String, find: String = "") -> EpisodeView {
        var v = EpisodeView(address: address)
        guard let (feed, id) = Self.parseEpisode(address) else { v.status = "error"; v.message = "This episode's address is not one the app can open."; return v }
        sample()
        refreshIfNeeded(feed)
        let key = PodcastKey.episode(feed: PodcastKey.feed(feed), id: id)
        let hint = self.hint(key)
        let k = knownShow(feed)
        v.feedURL = feed; v.subscribed = isSubscribed(feed); v.showAddress = Self.showAddress(feed)
        v.showTitle = k?.title ?? hint?.show ?? ""; v.artwork = PodcastText.https(k?.artworkURL ?? "")
        let now = nowHook?()
        let ep = cachedFeed(feed)?.episodes.first { $0.id == id }
        if let f = cachedFeed(feed) { v.showTitle = f.show.title; if !f.show.artworkURL.isEmpty { v.artwork = f.show.artworkURL } }
        var resolved = ep
        if resolved == nil, let h = hint, !h.enclosure.isEmpty || cachedFeed(feed) != nil {
            resolved = PodcastEpisode(id: id, title: h.title.isEmpty ? "Episode" : h.title, enclosureURL: h.enclosure)
            if cachedFeed(feed) != nil { v.note = "This episode is no longer in the show's feed. It can still be played from the link the clip carried." }
        }
        guard let e = resolved else {
            if let m = failure(feed) { v.status = "error"; v.message = m } else { v.status = "loading" }
            return v
        }
        v.status = "ready"
        v.title = e.title; v.stamp = PodcastText.relative(e.published, now: clock()); v.duration = PodcastText.durationLabel(e.duration)
        if !(ep?.artworkURL.isEmpty ?? true) { v.artwork = ep!.artworkURL }
        v.notes = e.notes.components(separatedBy: "\n\n").map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
        v.playable = e.playable || (hint?.enclosure.isEmpty == false)
        if !v.playable { v.note = "This episode has no audio file in the feed, so it cannot be played here." }
        v.current = now?.episodeId == key
        if v.current, let n = now, n.isClip, let s = n.start { v.fromClip = true; v.playingFrom = "Playing from " + PodcastText.clock(s) }
        else if let h = hint, let s = h.start, s > 0, !v.current { v.playingFrom = "From the clip at " + PodcastText.clock(s) }
        if let r = resumeSeconds(key) { v.hasResume = true; v.resumeLabel = "Resume from " + PodcastText.clock(r) }
        // The shared transcript: shown when the platform has one; "on its way" only for an episode that was played here.
        transcriptIfNeeded(feed: feed, id: id)
        lock.lock(); let tr = transcripts[key]; let wasPlayed = playedKeys.contains(key); lock.unlock()
        if let t = tr {
            v.transcriptState = "ready"; v.transcriptSource = t.source
            // Tap-to-seek only when the file that plays is the file that was transcribed (within 3 s).
            let fileLength = (v.current ? now?.duration : nil) ?? e.duration
            let seek = Self.durationsAgree(fileLength, t.durationSeconds)
            if !seek { v.transcriptNote = "These times are for another recording of this episode, so tapping a line will not jump to it." }
            let playhead = v.current ? now?.elapsed : nil
            let q = find.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            var curIndex = -1
            if let p = playhead { curIndex = t.blocks.lastIndex { $0.start <= p } ?? -1 }
            for (i, b) in t.blocks.enumerated() where q.isEmpty || b.text.lowercased().contains(q) {
                v.transcript.append(TranscriptRow(key: String(i), time: PodcastText.clock(b.start), text: b.text, seconds: b.start, current: i == curIndex, seekable: seek && v.playable))
            }
            if v.transcript.isEmpty, !q.isEmpty { v.transcriptNote = "Nothing in the transcript matches that." }
        } else if wasPlayed || position(key) != nil { v.transcriptState = "pending"; v.transcriptNote = "Transcript on its way." }
        return v
    }

    // MARK: Acts

    private func resolve(_ address: String) -> (request: PodcastPlayRequest, key: String, feed: String)? {
        guard let (feed, id) = Self.parseEpisode(address) else { return nil }
        let key = PodcastKey.episode(feed: PodcastKey.feed(feed), id: id)
        let ep = cachedFeed(feed)?.episodes.first { $0.id == id }
        let h = hint(key)
        let urlString = (ep?.enclosureURL.isEmpty == false ? ep?.enclosureURL : nil) ?? h?.enclosure ?? ""
        guard let url = URL(string: urlString), !urlString.isEmpty else { return nil }
        let show = cachedFeed(feed)?.show.title ?? knownShow(feed)?.title ?? h?.show ?? ""
        return (PodcastPlayRequest(url: url, title: ep?.title ?? h?.title ?? "Episode", show: show, start: nil, end: nil, episodeId: key), key, feed)
    }

    /// mode: "resume" (from where it stopped, else the start), "start" (from the beginning), "keep" (keep listening to the clip's episode).
    public func play(address: String, mode: String) -> Ack {
        guard var r = resolve(address) else { return Ack(status: "error", message: "This episode has no audio to play.") }
        guard let hook = playHook else { return Ack(status: "error", message: "Playback is not ready yet.") }
        sample()
        let now = nowHook?()
        switch mode {
        case "keep":
            if let n = now, n.episodeId == r.key, n.isClip { keepHook?(); return Ack(status: "ok", message: "Keeping on.") }
            let from = max(hint(r.key)?.start ?? 0, position(r.key)?.seconds ?? 0)
            r.request.start = from > 0 ? from : nil
        case "start": r.request.start = nil
        case _ where mode.hasPrefix("at:"):
            // From a transcript block: only from a second the transcript's own file can be trusted for (the page checked), never past 0.
            let at = Double(mode.dropFirst(3)) ?? 0
            r.request.start = at > 0 ? at : nil
        default: r.request.start = resumeSeconds(r.key)
        }
        hook(r.request, false)
        lock.lock(); playedKeys.insert(r.key); lock.unlock()
        if let (f, g) = PodcastKey.parseEpisode(r.key) { reportFeeds(nowPlaying: PodcastNowPlaying(feedURL: f, episodeGuid: g, audioURL: r.request.url.absoluteString)) }
        return Ack(status: "ok", message: "Playing.")
    }

    public func enqueue(address: String) -> Ack {
        guard let r = resolve(address) else { return Ack(status: "error", message: "This episode has no audio to queue.") }
        guard let hook = playHook else { return Ack(status: "error", message: "Playback is not ready yet.") }
        var req = r.request; req.start = resumeSeconds(r.key)
        hook(req, true)
        return Ack(status: "ok", message: "Added to the queue.")
    }

    // MARK: Search provider

    /// Matches among subscribed shows and their cached episodes (title, notes), then the directory's shows for the query.
    public func searchHits(_ raw: String) -> [SearchHit] {
        let q = CorpusSearch.normalize(raw).lowercased()
        guard !q.isEmpty else { return [] }
        var out: [SearchHit] = []
        var episodes: [SearchHit] = []
        for s in subscriptions() {
            if s.title.lowercased().contains(q) || s.author.lowercased().contains(q) {
                out.append(SearchHit(address: Self.showAddress(s.feedURL), title: s.title, kind: "podcast", detail: "Podcast · subscribed"))
            }
            guard let f = cachedFeed(s.feedURL) else { continue }
            for e in f.episodes where episodes.count < 6 {
                let inTitle = e.title.lowercased().contains(q)
                guard inTitle || e.notes.lowercased().contains(q) else { continue }
                episodes.append(SearchHit(address: Self.episodeAddress(feed: f.show.feedURL, id: e.id), title: e.title, kind: "podcast",
                                          detail: "Episode · " + f.show.title + (inTitle ? "" : " · in the notes")))
            }
        }
        out += episodes
        if case .done(let found) = searchStatus(for: raw) {
            for s in found.prefix(5) where !isSubscribed(s.feedURL) {
                out.append(SearchHit(address: Self.showAddress(s.feedURL), title: s.title, kind: "podcast", detail: "Podcast" + (s.author.isEmpty ? "" : " · " + s.author)))
            }
        }
        return out
    }
}

// MARK: The entry point a clip uses

public enum PodcastEpisodes {
    /// Open the episode a clip was cut from. `episodeId` is the feed's guid; `feedURL` the show's RSS address; `enclosureURL` the audio file
    /// the clip plays; `at` the second the clip starts. The page shows the show, the title, the notes, a Subscribe button for the show,
    /// "Playing from 12:40" and a Keep listening button. Returns the page's address.
    @discardableResult
    public static func open(episodeId: String, feedURL: String, enclosureURL: String, at: Double?, title: String = "", show: String = "",
                            service: PodcastService = .shared) -> String {
        service.setHint(feed: feedURL, id: episodeId, enclosure: PodcastText.webURL(enclosureURL), start: at, title: title, show: show)
        let address = PodcastService.episodeAddress(feed: feedURL, id: episodeId)
        service.opener?(address, title.isEmpty ? "Episode" : title)
        service.refreshIfNeeded(feedURL)
        return address
    }
}

// MARK: The server's subscription route (reachable with the phone's key; see the top of PodcastService.swift)

public extension ChoirClient {
    func podcastSubscriptions() async throws -> [PodcastShow] {
        let r = try await send("GET", "/api/podcast/subscriptions", query: ["limit": "200"], route: "/api/podcast/subscriptions")
        return try PodcastSyncCodec.decodeList(r.data)
    }
    func podcastSubscribe(_ show: PodcastShow) async throws {
        let body = try JSONSerialization.data(withJSONObject: ["feed_url": show.feedURL, "title": show.title, "author": show.author, "artwork_url": show.artworkURL])
        try await send("POST", "/api/podcast/subscriptions", body: body, route: "/api/podcast/subscriptions")
    }
}

public enum PodcastSyncCodec {
    /// The server's `{subscriptions:[{feed_url,title,author,artwork_url,...}]}`.
    public static func decodeList(_ data: Data) throws -> [PodcastShow] {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { throw ChoirError.decode("not a JSON object") }
        guard let list = o["subscriptions"] as? [[String: Any]] else { if o["subscriptions"] == nil || o["subscriptions"] is NSNull { return [] }; throw ChoirError.decode("subscriptions is not a list") }
        var out: [PodcastShow] = [], seen = Set<String>()
        for r in list {
            let feed = PodcastText.webURL((r["feed_url"] as? String) ?? "")
            guard !feed.isEmpty, seen.insert(PodcastKey.feed(feed)).inserted else { continue }
            var title = (r["title"] as? String) ?? ""
            if title.isEmpty || title.lowercased().hasPrefix("http") { title = ((r["content_item"] as? [String: Any])?["title"] as? String) ?? title }
            out.append(PodcastShow(feedURL: feed, title: title.isEmpty ? feed : title, author: (r["author"] as? String) ?? "",
                                   artworkURL: PodcastText.https(PodcastText.webURL((r["artwork_url"] as? String) ?? ""))))
        }
        return out
    }

    public static func hooks(environment: ChoirEnvironment = .shared) -> PodcastSyncHooks {
        PodcastSyncHooks(signedIn: { environment.client().hasKey },
                         push: { show in (try? await environment.client().podcastSubscribe(show)) != nil },
                         pull: { try? await environment.client().podcastSubscriptions() })
    }
}

// MARK: The platform's shared podcast routes

public enum PodcastPlatformCodec {
    public static let feedsPath = "/api/platform/podcast-feeds"
    public static let popularPath = "/api/platform/podcast-popular"
    public static let transcriptPath = "/api/platform/podcast-transcript"

    /// The wire's bounds: at most 50 feeds, http(s) URLs with no credentials of at most 2048 bytes, titles of at most 200 bytes.
    static func clean(_ feeds: [PodcastShow]) -> [[String: String]] {
        func ok(_ u: String) -> Bool { u.utf8.count <= 2048 && URL(string: u).map { $0.user == nil && $0.password == nil } == true && !PodcastText.webURL(u).isEmpty }
        func cut(_ s: String, _ n: Int) -> String { var t = s; while t.utf8.count > n { t.removeLast() }; return t }
        return feeds.filter { ok($0.feedURL) }.prefix(50).map { ["feed_url": $0.feedURL, "title": cut($0.title, 200)] }
    }

    /// `{episode:{duration_seconds,...},source,blocks:[{start,end,text}]}`; blocks without text are dropped.
    public static func decodeTranscript(_ data: Data) -> PodcastTranscript? {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any], let list = o["blocks"] as? [[String: Any]] else { return nil }
        let blocks = list.compactMap { b -> PodcastTranscript.Block? in
            guard let t = (b["text"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines), !t.isEmpty, let s = (b["start"] as? NSNumber)?.doubleValue, s >= 0 else { return nil }
            return PodcastTranscript.Block(start: s, end: (b["end"] as? NSNumber)?.doubleValue ?? s, text: t)
        }
        guard !blocks.isEmpty else { return nil }
        return PodcastTranscript(durationSeconds: ((o["episode"] as? [String: Any])?["duration_seconds"] as? NSNumber)?.doubleValue, source: (o["source"] as? String) ?? "", blocks: blocks)
    }

    public static func reportBody(_ feeds: [PodcastShow], nowPlaying: PodcastNowPlaying?) -> Data? {
        var o: [String: Any] = ["feeds": clean(feeds)]
        if let n = nowPlaying, !PodcastText.webURL(n.feedURL).isEmpty, !PodcastText.webURL(n.audioURL).isEmpty { o["now_playing"] = ["feed_url": n.feedURL, "episode_guid": n.episodeGuid, "audio_url": n.audioURL] }
        return try? JSONSerialization.data(withJSONObject: o)
    }

    /// Shows other readers follow: `{shows:[{feed_url,title,author,artwork_url}]}` (provisional; `feeds` or `results` are read too).
    public static func decodePopular(_ data: Data) -> [PodcastShow] {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { return [] }
        let list = (o["feeds"] ?? o["shows"]) as? [[String: Any]] ?? []
        var seen = Set<String>(), out: [PodcastShow] = []
        for r in list {
            let feed = PodcastText.webURL((r["feed_url"] as? String) ?? "")
            let title = ((r["title"] as? String) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            guard !feed.isEmpty, !title.isEmpty, seen.insert(PodcastKey.feed(feed)).inserted else { continue }
            out.append(PodcastShow(feedURL: feed, title: title, author: (r["author"] as? String) ?? "",
                                   artworkURL: PodcastText.https(PodcastText.webURL((r["artwork_url"] as? String) ?? ""))))
        }
        return out
    }

    public static func hooks(environment: ChoirEnvironment = .shared) -> PodcastPlatformHooks {
        PodcastPlatformHooks(
            report: { feeds, now in
                guard let body = reportBody(feeds, nowPlaying: now) else { return }
                let c = environment.client()
                _ = try? await c.send("POST", feedsPath, body: body, authenticated: c.hasKey, route: feedsPath, okStatuses: [404])
            },
            popular: {
                let c = environment.client()
                guard let r = try? await c.send("GET", popularPath, query: ["limit": "30"], authenticated: false, route: popularPath, okStatuses: [404]), r.status == 200 else { return nil }
                return decodePopular(r.data)
            },
            transcript: { feed, guid in
                let c = environment.client()
                guard c.hasKey, let r = try? await c.send("GET", transcriptPath, query: ["feed": feed, "guid": guid], route: transcriptPath, okStatuses: [404]), r.status == 200 else { return nil }
                return decodeTranscript(r.data)
            })
    }
}
