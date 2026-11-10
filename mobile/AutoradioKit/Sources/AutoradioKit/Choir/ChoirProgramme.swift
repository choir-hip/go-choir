import Foundation

// MARK: Text to spoken blocks

public enum ChoirText {
    /// Plain spoken paragraphs from a story or revision body (markdown-ish text).
    public static func paragraphs(_ text: String, maxChars: Int = 600) -> [String] {
        var out: [String] = []
        for raw in text.replacingOccurrences(of: "\r\n", with: "\n").components(separatedBy: "\n\n") {
            let p = strip(raw)
            if p.isEmpty { continue }
            out.append(contentsOf: split(p, maxChars: maxChars))
        }
        return out
    }

    /// Title first (unless the body already starts with it), then the paragraphs.
    public static func blocks(title: String, text: String, maxBlocks: Int = 80) -> [String] {
        var ps = paragraphs(text)
        let t = strip(title)
        if !t.isEmpty, ps.first?.lowercased() != t.lowercased() { ps.insert(t.hasSuffix(".") ? t : t + ".", at: 0) }
        return Array(ps.prefix(maxBlocks))
    }

    static func strip(_ s: String) -> String {
        var t = s
        // links [text](url) -> text; citation marks [12]; heading, list, quote, emphasis marks
        t = t.replacingOccurrences(of: "\\[([^\\]]*)\\]\\([^)]*\\)", with: "$1", options: .regularExpression)
        t = t.replacingOccurrences(of: "\\[\\d+\\]", with: "", options: .regularExpression)
        t = t.components(separatedBy: "\n").map { line in
            line.replacingOccurrences(of: "^\\s*(#{1,6}\\s+|>\\s*|[-*+]\\s+|\\d+\\.\\s+)", with: "", options: .regularExpression)
        }.joined(separator: " ")
        t = t.replacingOccurrences(of: "[*_`]", with: "", options: .regularExpression)
        t = t.replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
        t = t.replacingOccurrences(of: " ([.,;:!?])", with: "$1", options: .regularExpression)
        return t.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func split(_ p: String, maxChars: Int) -> [String] {
        if p.count <= maxChars { return [p] }
        var out: [String] = [], cur = ""
        var sentences: [String] = []
        p.enumerateSubstrings(in: p.startIndex..<p.endIndex, options: .bySentences) { s, _, _, _ in if let s { sentences.append(s) } }
        for s in sentences {
            if !cur.isEmpty, cur.count + s.count > maxChars { out.append(cur.trimmingCharacters(in: .whitespaces)); cur = "" }
            cur += s
        }
        if !cur.trimmingCharacters(in: .whitespaces).isEmpty { out.append(cur.trimmingCharacters(in: .whitespaces)) }
        return out
    }
}

// MARK: Front page

public struct ChoirStory: Codable, Sendable, Equatable {
    public var title: String
    public var dek: String
    public var route: String
    public var section: String
    public var words: Int
    public var updated: String
}

public enum ChoirFront {
    public static func stories(client: ChoirClient, limit: Int = 20, section: String = "") async throws -> [ChoirStory] {
        var q = ["limit": String(limit)]
        if !section.isEmpty { q["section"] = section }
        let r = try await client.send("GET", "/api/platform/publications/front", query: q, authenticated: false)
        guard let list = try r.object()["stories"] as? [[String: Any]] else { throw ChoirError.decode("front page has no stories") }
        return list.compactMap { s in
            guard let title = s["title"] as? String, let route = (s["route"] as? [String: Any])?["bare"] as? String else { return nil }
            return ChoirStory(title: title, dek: s["dek"] as? String ?? "", route: route, section: s["section"] as? String ?? "",
                              words: s["words"] as? Int ?? 0, updated: s["updated_at"] as? String ?? "")
        }
    }

    /// Resolve one story into a playable document and remember it for the desk.
    public static func load(route: String, client: ChoirClient, maxBlocks: Int = 80, env: ChoirEnvironment = .shared) async throws -> ProgrammeDocument {
        let r = try await client.send("GET", "/api/platform/publications/resolve", query: ["route": route], authenticated: false, route: "/api/platform/publications/resolve")
        let o = try r.object()
        let payload = o["payload"] as? [String: Any]
        let artifact = o["artifact"] as? [String: Any]
        let version = o["version"] as? [String: Any]
        let text = (payload?["text"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? (artifact?["content"] as? String) ?? ""
        let title = ((o["publication"] as? [String: Any])?["title"] as? String) ?? ""
        let blocks = ChoirText.blocks(title: title, text: text, maxBlocks: maxBlocks)
        if ChoirText.paragraphs(text).isEmpty { throw ChoirError.empty("the story \"\(title)\"") }
        let versionID = (version?["id"] as? String) ?? ""
        let manifest = (version?["manifest_hash"] as? String) ?? (payload?["manifest_hash"] as? String) ?? ""
        let id = "choir:" + (versionID.isEmpty ? route : versionID)
        env.setPlaying(PlayingProgramme(programmeID: id, title: title, route: route, publicationVersionID: versionID, manifestHash: manifest, blocks: blocks))
        return ProgrammeDocument(id: id, title: title, blocks: blocks.map { .speech(text: $0) })
    }
}

/// The public front page as a programme: the picked story (param `route`), or the first story with text.
public final class ChoirFrontProgramme: Programme, @unchecked Sendable {
    public static let stageID = "choir-front"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Choir front page", summary: "Reads a story from the public paper, block by block.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL),
                                 .text("route", "Story route", "", note: "empty plays the newest story with text"),
                                 .text("section", "Section", "", note: "e.g. world, tech_ai; empty for all"),
                                 .number("timeoutSeconds", "Timeout (s)", 15, 2, 60),
                                 .number("maxBlocks", "Most blocks", 80, 5, 400)],
                        needsNetwork: true)
    }
    let baseURL: String, route: String, section: String, timeout: Double, maxBlocks: Int
    public init(_ p: StageParams) {
        baseURL = p.string("baseURL", ChoirEnvironment.defaultBaseURL); route = p.string("route"); section = p.string("section")
        timeout = p.double("timeoutSeconds", 15); maxBlocks = p.int("maxBlocks", 80)
    }
    public func load() async throws -> ProgrammeDocument {
        let client = ChoirEnvironment.shared.client(baseURL: baseURL, timeout: timeout)
        if !route.isEmpty { return try await ChoirFront.load(route: route, client: client, maxBlocks: maxBlocks) }
        let list = try await ChoirFront.stories(client: client, limit: 12, section: section)
        if list.isEmpty { throw ChoirError.empty("the front page") }
        var lastError: Error = ChoirError.empty("the front page")
        for s in list {
            do { return try await ChoirFront.load(route: s.route, client: client, maxBlocks: maxBlocks) }
            catch is CancellationError { throw CancellationError() }
            catch { lastError = error }   // unpublished since listed, or no text: try the next
        }
        throw lastError
    }
}
