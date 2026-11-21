import Foundation

// A story to keep: its route, title and date. The bodies are fetched and kept by ContentSync into the ContentStore (one store for every kind).
public enum StoryBodies {
    public struct Story: Sendable { public var route: String, title: String, date: Date?
        public init(route: String, title: String, date: Date?) { self.route = route; self.title = title; self.date = date } }

    /// The story's readable text for the phone's own search: `GET /api/platform/publications/resolve?route=` (public), its Markdown (`artifact.content`, else `payload.text`) without code fences, rules or markers.
    static let resolveFetcher: @Sendable (String) async throws -> String? = { route in
        let a = TextureAddress.parse("pub:" + route)
        let o = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": a.route], authenticated: false,
                                                                route: "/api/platform/publications/resolve").object()
        let md = ((o["artifact"] as? [String: Any])?["content"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? ((o["payload"] as? [String: Any])?["text"] as? String) ?? ""
        let text = plain(md)
        return text.isEmpty ? nil : text
    }
    static func plain(_ markdown: String) -> String {
        var fenced = false, out: [String] = []
        for raw in markdown.components(separatedBy: "\n") {
            let l = raw.trimmingCharacters(in: .whitespaces)
            if l.hasPrefix("```") { fenced.toggle(); continue }
            if fenced || l.isEmpty || l.allSatisfy({ "-*_ ".contains($0) }) { continue }
            let t = l.drop(while: { "#>-* ".contains($0) }).replacingOccurrences(of: "**", with: "")
            if !t.isEmpty { out.append(String(t)) }
        }
        return out.joined(separator: "\n")
    }
}
