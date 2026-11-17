import Foundation

// The bodies of the front page's and the edition's stories, fetched in the background and kept on the phone, so "tell me about the Senate"
// finds every story about the Senate and not only the headlines (follow-up to Retrieval.swift).
//
// HOW THE FETCH WORKS TODAY: one request per story, `GET /api/platform/publications/resolve?route=` (public), the same route the reader uses
// when a story is opened (TextureReaderSource.fetch). The server has NO list route that returns bodies (front/list carry title, dek and date;
// `publications/export` and `resolve` are per route), so a sweep is one request per story. They go one after another, never in parallel.
//
// BOUNDS (stated, tested): at most `maxStories` (100, the newest by date) and `maxBytes` (4,000,000 bytes of body text in all, about 600 stories
// of 6 KB, so the story cap is the one that binds; the byte cap guards a feed of very long pieces). Over either, the oldest held story goes first.
// NETWORK POLICY: the app has no Wi-Fi-only rule for stories (the reader prefetches on any network; only model downloads ask first), and a story is
// about 6 KB, so a sweep is at most about 600 KB; it runs on any network. It never runs while the radio needs the network for audio: before
// every request `audioBusy` is asked and a busy radio ends the sweep (the next sweep, at the next front load, continues: nothing held is asked
// again). RESUMABLE: bodies are written to `story-bodies.json` after each landing batch and read at launch; a story whose date and title are
// unchanged is not fetched again; a failed story is left for the next sweep (and not retried within this one).
//
// WAYS THIS CAN FAIL (tests in StoryBodiesTests, with a fake fetcher):
//  1. A story already held and unchanged: not fetched again.   2. A fetch that fails or returns no text: skipped, the sweep goes on.
//  3. Radio busy: the sweep stops before the next request; what landed stays.   4. More stories than the cap: only the newest `maxStories` are asked.
//  5. A body over the byte budget: the oldest bodies are dropped, the total never exceeds `maxBytes`.   6. Two sweeps at once: one runs, the
//  second returns at once.   7. A bad or missing cache file: empty, no crash.   8. A cancelled sweep: stops at the next story.

public final class StoryBodies: @unchecked Sendable {
    public static let shared = StoryBodies(file: defaultFile())
    public static let maxStories = 100
    public static let maxBytes = 4_000_000

    public struct Body: Codable, Equatable, Sendable { public var route: String, title: String, date: Date?, text: String }
    public struct Story: Sendable { public var route: String, title: String, date: Date?
        public init(route: String, title: String, date: Date?) { self.route = route; self.title = title; self.date = date } }

    /// route -> the story's plain text (nil: nothing to hold). Default: the reader's own `resolve` route.
    public var fetcher: @Sendable (String) async throws -> String?
    /// True while the radio needs the network for audio: a sweep does not start a request then.
    public var audioBusy: @Sendable () async -> Bool = { false }
    /// Called after a batch of bodies landed (the corpus re-reads its sources).
    public var onLand: (@Sendable () -> Void)?

    private let file: URL?
    private let lock = NSLock()
    private var bodies: [String: Body] = [:]
    private var sweeping = false
    private var failed = Set<String>()

    public init(file: URL?, fetcher: (@Sendable (String) async throws -> String?)? = nil) {
        self.file = file
        self.fetcher = fetcher ?? StoryBodies.resolveFetcher
        if let file, let d = try? Data(contentsOf: file), let list = try? JSONDecoder().decode([Body].self, from: d) { for b in list { bodies[b.route] = b } }
    }
    static func defaultFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("story-bodies.json")
    }

    /// The reader's route: resolve, then the readable text (headings and paragraphs, no code or embeds).
    static let resolveFetcher: @Sendable (String) async throws -> String? = { route in
        let a = TextureAddress.parse("pub:" + route)
        let r = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": a.route], authenticated: false,
                                                                route: "/api/platform/publications/resolve")
        let d = TextureDocParser.parseBundle(try r.object(), address: "pub:" + route)
        let text = d.blocks.filter { !["code", "rule", "embed"].contains($0.kind) }.map(\.text).filter { !$0.isEmpty }.joined(separator: "\n")
        return text.isEmpty ? nil : text
    }

    public var count: Int { lock.lock(); defer { lock.unlock() }; return bodies.count }
    public var totalBytes: Int { lock.lock(); defer { lock.unlock() }; return bodies.values.reduce(0) { $0 + $1.text.utf8.count } }

    /// Fetch what is missing, newest first, within the caps. Returns how many bodies landed.
    @discardableResult
    public func sweep(_ stories: [Story]) async -> Int {
        lock.lock(); if sweeping { lock.unlock(); return 0 }; sweeping = true; lock.unlock()
        defer { lock.lock(); sweeping = false; lock.unlock() }
        let newest = stories.filter { !$0.route.isEmpty }.sorted { ($0.date ?? .distantPast) > ($1.date ?? .distantPast) }.prefix(Self.maxStories)
        var landed = 0
        for s in newest {
            if Task.isCancelled { break }
            lock.lock(); let held = bodies[s.route], skip = failed.contains(s.route); lock.unlock()
            if let h = held, h.title == s.title, h.date == s.date { continue }
            if skip { continue }
            if await audioBusy() { break }
            do {
                guard let text = try await fetcher(s.route), !text.isEmpty else { markFailed(s.route); continue }
                lock.lock(); bodies[s.route] = Body(route: s.route, title: s.title, date: s.date, text: text); trimLocked(); lock.unlock()
                landed += 1
                onLand?()
            } catch is CancellationError { break } catch { markFailed(s.route) }
        }
        if landed > 0 { persist() }
        return landed
    }
    private func markFailed(_ r: String) { lock.lock(); failed.insert(r); lock.unlock() }
    private func trimLocked() {
        // Oldest first out, until both caps hold.
        var total = bodies.values.reduce(0) { $0 + $1.text.utf8.count }
        let order = bodies.values.sorted { ($0.date ?? .distantPast) < ($1.date ?? .distantPast) }
        var i = 0
        while (bodies.count > Self.maxStories || total > Self.maxBytes), i < order.count {
            total -= order[i].text.utf8.count; bodies[order[i].route] = nil; i += 1
        }
    }
    private func persist() {
        lock.lock(); let list = Array(bodies.values); lock.unlock()
        guard let file, let d = try? JSONEncoder().encode(list) else { return }
        try? d.write(to: file, options: .atomic)
    }

    /// The held bodies as corpus documents (id `body:<route>`, address `pub:<route>`; the index keeps one item per address).
    public func documents() -> [SavedCorpus.Document] {
        lock.lock(); defer { lock.unlock() }
        return bodies.values.map { SavedCorpus.Document(id: "body:" + $0.route, title: $0.title, address: "pub:" + $0.route, text: $0.text, kind: "story", source: "Autopaper", date: $0.date) }
    }
}
