import Foundation

// One story of the paper's front list (`GET /api/platform/publications/front`), as the RADIO reads it (StretchCandidates, the published times).
// The paper page no longer uses this: its data, ranking and layout are TypeScript (AutoradioLab/paper.ts, paper-model.ts, paper-layout.ts).

public struct PaperEntity: Sendable, Equatable { public var id: String; public var label: String
    public init(id: String, label: String) { self.id = id; self.label = label } }

public struct PaperStory: Sendable, Equatable {
    public var route: String
    public var title: String
    public var dek: String
    public var section: String
    public var words: Int
    public var importance: Int          // 1...10, 5 when the list does not say
    public var sources: Int
    public var versions: Int
    public var updated: Date?
    public var kind: String
    public var author: String
    public var entities: [PaperEntity]
    /// Every section the story belongs to (the front list's `sections`, house ids such as "world", "politics"); `section` is the primary one.
    /// The server's own `?section=` filter uses this list, so a story is in a category when the category is in it. Empty from an older server.
    public var sections: [String]

    public init(route: String, title: String, dek: String = "", section: String = "", words: Int = 0, importance: Int = 5, sources: Int = 0,
                versions: Int = 1, updated: Date? = nil, kind: String = "", author: String = "", entities: [PaperEntity] = [], sections: [String] = []) {
        self.route = route; self.title = title; self.dek = dek; self.section = section; self.words = words; self.importance = importance
        self.sources = sources; self.versions = versions; self.updated = updated; self.kind = kind; self.author = author; self.entities = entities
        self.sections = sections
    }


    /// Importance, lifted by depth (sources) and decayed by age, in the manner of the web's ranking
    /// (`L/autopaper.ts`: importance mode, depth factor 0.75 for thin sourcing, recency). `now` is the edition's time.
    public func weight(now: Date) -> Double {
        let imp = Double(min(max(importance, 1), 10)) / 10
        let depth = sources <= 2 ? 0.75 : 1 + 0.1 * min(log2(Double(sources) / 3), 3)
        var recency = 1.0
        if let u = updated { recency = pow(0.5, max(0, now.timeIntervalSince(u)) / (24 * 3600)) }
        let w = imp * depth * (0.6 + 0.4 * recency)
        return w.isFinite ? w : 0
    }

    // MARK: parsing the front list

    /// Parses one story object of the front list. Unknown or missing keys are tolerated; a story with no title or route is dropped.
    public static func parse(_ s: [String: Any]) -> PaperStory? {
        guard let title = s["title"] as? String, !title.isEmpty,
              let route = (s["route"] as? [String: Any])?["bare"] as? String ?? s["route"] as? String, !route.isEmpty else { return nil }
        func int(_ k: String) -> Int? { (s[k] as? Int) ?? (s[k] as? Double).map(Int.init) ?? (s[k] as? String).flatMap(Int.init) }
        var ents: [PaperEntity] = []
        for e in (s["entities"] as? [[String: Any]]) ?? [] {
            if let id = e["id"] as? String, let l = e["label"] as? String, !l.isEmpty { ents.append(PaperEntity(id: id, label: l)) }
        }
        return PaperStory(route: route, title: title, dek: tidy(s["dek"] as? String ?? ""), section: s["section"] as? String ?? "",
                          words: int("words") ?? 0, importance: int("importance") ?? 5, sources: int("source_count") ?? 0,
                          versions: max(1, int("version_count") ?? 1), updated: Self.date(s["updated_at"] as? String),
                          kind: s["kind"] as? String ?? "", author: s["author"] as? String ?? "", entities: ents,
                          sections: ((s["sections"] as? [Any]) ?? []).compactMap { $0 as? String }.filter { !$0.isEmpty })
    }

    /// The real dek is cut from sentences joined with a space before each stop ("Abu Dhabi . The Kremlin", "earlier: ."); say it the way it is read.
    static func tidy(_ s: String) -> String {
        var t = s.replacingOccurrences(of: "\\s+([.,;:!?])", with: "$1", options: .regularExpression)
        t = t.replacingOccurrences(of: "([:;,])\\.", with: "$1", options: .regularExpression)
        return t.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    public static func parseList(_ object: [String: Any]) -> [PaperStory] {
        ((object["stories"] as? [[String: Any]]) ?? []).compactMap(parse)
    }

    static func date(_ s: String?) -> Date? {
        guard let s, !s.isEmpty else { return nil }
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        if let d = f.date(from: s) { return d }
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f.date(from: s)
    }
}

