import Foundation

// MARK: Text to spoken blocks

public enum ChoirText {
    /// Plain spoken paragraphs from a story or revision body (markdown-ish text).
    /// The longest a spoken block is: about two sentences, so the first sound of a story comes after one short block is made and the rest
    /// is made while it plays. Reading pages keep the longer paragraphs.
    public static let speechChars = 280

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
    public static func blocks(title: String, text: String, maxBlocks: Int = 200) -> [String] {
        var ps = paragraphs(text, maxChars: speechChars)
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
    public static func load(route: String, client: ChoirClient, maxBlocks: Int = 200, env: ChoirEnvironment = .shared) async throws -> ProgrammeDocument {
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
