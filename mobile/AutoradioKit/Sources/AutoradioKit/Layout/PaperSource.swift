import Foundation

// Where the front page's stories come from: the public front list the Choir client already calls
// (`GET /api/platform/publications/front`, no sign-in needed), with the last good answer kept on the phone so an
// offline start still shows a page. In demo mode the stub session answers `front` from the fixtures; the three demo
// editions (big, even, thin) are fixtures `front-big`, `front-even`, `front-thin` read directly.
//
// Ways this can fail (written first): no network or a timeout -> the cached list with status "offline" (or "error" with
// no cache); a body that is not the expected object -> error; a cache file that is corrupt -> ignored; an unknown demo
// edition -> the plain `front` fixture; a category view asks the route's own `?section=` list (a fuller list than the front's newest hundred); when that fails the front list is filtered instead.
// The newest answer of each list is kept for the session (`maxAge`) so switching views and coming back from another page ask nothing; a pull to refresh
// (`fresh`) asks again. One ask per list, never one per view change.

public final class PaperSource: @unchecked Sendable {
    public struct Result: Sendable {
        public var stories: [PaperStory]; public var status: String; public var message: String; public var cachedAt: Date?
        public var sections: [LiveSection] = []
        public init(stories: [PaperStory], status: String, message: String, cachedAt: Date?, sections: [LiveSection] = []) {
            self.stories = stories; self.status = status; self.message = message; self.cachedAt = cachedAt; self.sections = sections
        }
    }
    /// How long a good answer is reused within a session.
    public static let maxAge: TimeInterval = 120
    private let cacheFile: URL?
    private let lock = NSLock()
    private var kept: [String: (result: Result, at: Date)] = [:]
    private var lastTick = 0
    public init(cacheFile: URL?) { self.cacheFile = cacheFile }

    /// The reload counter the contract passes: true once for each new value (a pull to refresh), so the caller can ask afresh.
    public func isReload(_ tick: Int) -> Bool {
        lock.lock(); defer { lock.unlock() }
        let f = tick != lastTick; lastTick = tick; return f
    }

    public func demo(edition: String) -> Result {
        let name = ["big", "even", "thin"].contains(edition) ? "front-\(edition)" : "front"
        guard let d = Demo.fixture(name), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else {
            return Result(stories: [], status: "error", message: "The demo front page is missing.", cachedAt: nil)
        }
        return Result(stories: PaperStory.parseList(o), status: "ready", message: "", cachedAt: nil, sections: PaperViews.parseLiveSections(o))
    }

    /// The front list (`section` empty) or one category's list. A category that cannot be asked answers status "error" with no stories and
    /// writes no cache; the caller then filters the front list. Only the front list is written to the phone for an offline start.
    public func live(client: ChoirClient, section: String = "", fresh: Bool = false, now: Date = Date()) async -> Result {
        lock.lock()
        let hit = kept[section]
        lock.unlock()
        if !fresh, let h = hit, now.timeIntervalSince(h.at) < Self.maxAge { return h.result }
        do {
            var q = ["limit": String(PaperViews.listLimit)]
            if !section.isEmpty { q["section"] = section }
            let r = try await client.send("GET", "/api/platform/publications/front", query: q, authenticated: false)
            let o = try r.object()
            let stories = PaperStory.parseList(o)
            if section.isEmpty, !stories.isEmpty, let f = cacheFile {
                try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
                try? r.data.write(to: f, options: .atomic)
            }
            let res = Result(stories: stories, status: "ready", message: "", cachedAt: nil, sections: PaperViews.parseLiveSections(o))
            if !stories.isEmpty || !section.isEmpty { lock.lock(); kept[section] = (res, now); lock.unlock() }
            return res
        } catch {
            if section.isEmpty, let f = cacheFile, let d = try? Data(contentsOf: f), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
                let at = (try? FileManager.default.attributesOfItem(atPath: f.path)[.modificationDate]) as? Date
                let stories = PaperStory.parseList(o)
                if !stories.isEmpty {
                    return Result(stories: stories, status: "offline", message: "Offline. Showing the last front page.", cachedAt: at, sections: PaperViews.parseLiveSections(o))
                }
            }
            if section.isEmpty, let h = hit { return Result(stories: h.result.stories, status: "offline", message: "Offline. Showing the last front page.", cachedAt: h.at, sections: h.result.sections) }
            return Result(stories: [], status: "error", message: "Could not reach choir.news. Pull to try again.", cachedAt: nil)
        }
    }
}
