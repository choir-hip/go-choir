import Foundation

// A story to keep: its route, title and date. The bodies are fetched and kept by ContentSync into the ContentStore (one store for every kind).
public enum StoryBodies {
    public struct Story: Sendable { public var route: String, title: String, date: Date?
        public init(route: String, title: String, date: Date?) { self.route = route; self.title = title; self.date = date } }

    /// The reader's route: `GET /api/platform/publications/resolve?route=` (public), then the readable text (headings and paragraphs, no code or embeds).
    static let resolveFetcher: @Sendable (String) async throws -> String? = { route in
        let a = TextureAddress.parse("pub:" + route)
        let r = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": a.route], authenticated: false,
                                                                route: "/api/platform/publications/resolve")
        let d = TextureDocParser.parseBundle(try r.object(), address: "pub:" + route)
        let text = d.blocks.filter { !["code", "rule", "embed"].contains($0.kind) }.map(\.text).filter { !$0.isEmpty }.joined(separator: "\n")
        return text.isEmpty ? nil : text
    }
}
