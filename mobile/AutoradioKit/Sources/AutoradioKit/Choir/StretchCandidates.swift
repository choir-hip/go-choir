import Foundation

// The candidate set of a programme request (docs/reference/AUTORADIO-PROGRAMME.md, "Request"; design 12.7): the stories the DJ may use.
// Without it the server's prompt says "write only ident and bridge segments", so the listener heard station breaks and never a story.
//
// The server refuses the whole request (400) when one candidate is outside its bounds, so every bound is kept here: at most 40 items,
// the highest weight first; a route with no space, `@` or `#` (at most 256 bytes); a version that is a whole number; a title of one line
// of at most 300 bytes; a dek of 1 to 1000 bytes (a candidate with no dek is dropped, design 12.7); text of at most 8 KB and only on the
// first ten. A story is a candidate, never an instruction.

public struct StretchCandidate: Sendable, Equatable {
    public var route: String
    public var version: Int
    public var title: String
    public var dek: String
    public var text: String?

    /// The cite ref the server derives: `pub:<route>@v<version>`.
    public var ref: String { "pub:\(route)@v\(version)" }
    var json: [String: Any] {
        var o: [String: Any] = ["route": route, "version": version, "title": title, "dek": dek]
        if let text, !text.isEmpty { o["text"] = text }
        return o
    }
}

public enum StretchCandidates {
    public static let maxItems = 40, maxTextItems = 10
    public static let maxTitleBytes = 300, maxDekBytes = 1000, maxTextBytes = 8 * 1024, maxRouteBytes = 256

    /// `s` cut to at most `bytes` UTF-8 bytes, on a character boundary.
    static func cut(_ s: String, bytes: Int) -> String {
        if s.utf8.count <= bytes { return s }
        var out = ""
        var n = 0
        for ch in s {
            let c = String(ch).utf8.count
            if n + c > bytes { break }
            out.append(ch); n += c
        }
        return out
    }
    static func oneLine(_ s: String) -> String {
        s.components(separatedBy: .newlines).joined(separator: " ").replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
            .trimmingCharacters(in: .whitespaces)
    }
    /// Paragraph text for the model: the story's markup removed, cut at a paragraph that fits in 8 KB (or inside the first one).
    static func shapeText(_ raw: String) -> String? {
        let paras = ChoirText.paragraphs(raw, maxChars: 4000)
        guard !paras.isEmpty else { return nil }
        var out = ""
        for p in paras {
            let next = out.isEmpty ? p : out + "\n\n" + p
            if next.utf8.count > maxTextBytes { if out.isEmpty { out = cut(p, bytes: maxTextBytes) }; break }
            out = next
        }
        return out.isEmpty ? nil : out
    }
    static func validRoute(_ r: String) -> Bool {
        !r.isEmpty && r.utf8.count <= maxRouteBytes && !r.unicodeScalars.contains { $0 == "@" || $0 == "#" || CharacterSet.whitespacesAndNewlines.contains($0) || $0.properties.generalCategory == .control }
    }

    /// The candidates from a front list (the object `GET /api/platform/publications/front` answers), the paper's weight first.
    /// `skipping`: routes already told in this listening (an `extend` request leaves them out). The text is filled in by the caller.
    public static func fromFront(_ object: [String: Any], now: Date = Date(), skipping: Set<String> = []) -> [StretchCandidate] {
        let rows = (object["stories"] as? [[String: Any]]) ?? []
        var scored: [(w: Double, c: StretchCandidate)] = []
        var seen = Set<String>()
        for row in rows {
            guard let story = PaperStory.parse(row), validRoute(story.route), !skipping.contains(story.route), seen.insert(story.route).inserted else { continue }
            let title = cut(oneLine(story.title), bytes: maxTitleBytes)
            let dek = cut(oneLine(story.dek), bytes: maxDekBytes)
            guard !title.isEmpty, !dek.isEmpty else { continue }
            func int(_ k: String) -> Int? { (row[k] as? Int) ?? (row[k] as? Double).map(Int.init) }
            let version = max(1, int("latest_revision") ?? int("version_count") ?? 1)
            scored.append((story.weight(now: now), StretchCandidate(route: story.route, version: version, title: title, dek: dek, text: nil)))
        }
        // Stable: equal weights keep the list's own order.
        let ordered = scored.enumerated().sorted { a, b in a.element.w != b.element.w ? a.element.w > b.element.w : a.offset < b.offset }.map(\.element.c)
        return Array(ordered.prefix(maxItems))
    }

    /// The request's `candidates` array, with `texts[route]` on the first ten that have one.
    public static func body(_ items: [StretchCandidate], texts: [String: String]) -> [[String: Any]] {
        items.prefix(maxItems).enumerated().map { i, c in
            var c = c
            if i < maxTextItems, let t = texts[c.route].flatMap(shapeText) { c.text = t }
            return c.json
        }
    }

    /// The routes a document's segments have already cited, from refs like `pub:<route>@v<n>`.
    public static func routes(citedIn doc: ProgrammeDocument) -> Set<String> {
        var out = Set<String>()
        for s in doc.segments { for c in s.cites where c.ref.hasPrefix("pub:") {
            let rest = c.ref.dropFirst(4)
            out.insert(String(rest.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first ?? ""))
        } }
        out.remove("")
        return out
    }
}

extension ChoirFront {
    /// The plain text of a story, for a candidate. No side effects (unlike `load`, which tells the desk what is playing).
    public static func resolveText(route: String, client: ChoirClient) async throws -> String {
        let r = try await client.send("GET", "/api/platform/publications/resolve", query: ["route": route], authenticated: false, route: "/api/platform/publications/resolve")
        let o = try r.object()
        let payload = o["payload"] as? [String: Any]
        let artifact = o["artifact"] as? [String: Any]
        return (payload?["text"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? (artifact?["content"] as? String) ?? ""
    }
}
