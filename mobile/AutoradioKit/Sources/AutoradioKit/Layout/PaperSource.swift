import Foundation

// Where the front page's stories come from: the public front list the Choir client already calls
// (`GET /api/platform/publications/front`, no sign-in needed), with the last good answer kept on the phone so an
// offline start still shows a page. In demo mode the stub session answers `front` from the fixtures; the three demo
// editions (big, even, thin) are fixtures `front-big`, `front-even`, `front-thin` read directly.
//
// Ways this can fail (written first): no network or a timeout -> the cached list with status "offline" (or "error" with
// no cache); a body that is not the expected object -> error; a cache file that is corrupt -> ignored; an unknown demo
// edition -> the plain `front` fixture; section filtering happens on the client so the section bar always lists all sections.

public final class PaperSource: @unchecked Sendable {
    public struct Result: Sendable { public var stories: [PaperStory]; public var status: String; public var message: String; public var cachedAt: Date? }
    private let cacheFile: URL?
    public init(cacheFile: URL?) { self.cacheFile = cacheFile }

    public func demo(edition: String) -> Result {
        let name = ["big", "even", "thin"].contains(edition) ? "front-\(edition)" : "front"
        guard let d = Demo.fixture(name), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else {
            return Result(stories: [], status: "error", message: "The demo front page is missing.", cachedAt: nil)
        }
        return Result(stories: PaperStory.parseList(o), status: "ready", message: "", cachedAt: nil)
    }

    public func live(client: ChoirClient) async -> Result {
        do {
            let r = try await client.send("GET", "/api/platform/publications/front", query: ["limit": "60"], authenticated: false)
            let o = try r.object()
            let stories = PaperStory.parseList(o)
            if !stories.isEmpty, let f = cacheFile {
                try? FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
                try? r.data.write(to: f, options: .atomic)
            }
            return Result(stories: stories, status: "ready", message: "", cachedAt: nil)
        } catch {
            if let f = cacheFile, let d = try? Data(contentsOf: f), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
                let at = (try? FileManager.default.attributesOfItem(atPath: f.path)[.modificationDate]) as? Date
                let stories = PaperStory.parseList(o)
                if !stories.isEmpty { return Result(stories: stories, status: "offline", message: "Offline. Showing the last front page.", cachedAt: at) }
            }
            return Result(stories: [], status: "error", message: "Could not reach choir.news. Pull to try again.", cachedAt: nil)
        }
    }
}
