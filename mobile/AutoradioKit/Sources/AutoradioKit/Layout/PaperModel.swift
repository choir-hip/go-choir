import Foundation

// The front page as data. Input: stories from the paper's front list (`GET /api/platform/publications/front`, the route
// the Choir client already calls). Output: the placed bands the Contract draws (shapes declared in
// exact-package/interface/paper.json; these Codable types encode to them).

public struct PaperEntity: Codable, Sendable, Equatable { public var id: String; public var label: String
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

    public init(route: String, title: String, dek: String = "", section: String = "", words: Int = 0, importance: Int = 5, sources: Int = 0,
                versions: Int = 1, updated: Date? = nil, kind: String = "", author: String = "", entities: [PaperEntity] = []) {
        self.route = route; self.title = title; self.dek = dek; self.section = section; self.words = words; self.importance = importance
        self.sources = sources; self.versions = versions; self.updated = updated; self.kind = kind; self.author = author; self.entities = entities
    }

    public var isOpinion: Bool { kind.lowercased() == "opinion" || section.lowercased() == "opinion" }

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
        return PaperStory(route: route, title: title, dek: s["dek"] as? String ?? "", section: s["section"] as? String ?? "",
                          words: int("words") ?? 0, importance: int("importance") ?? 5, sources: int("source_count") ?? 0,
                          versions: max(1, int("version_count") ?? 1), updated: Self.date(s["updated_at"] as? String),
                          kind: s["kind"] as? String ?? "", author: s["author"] as? String ?? "", entities: ents)
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

// MARK: output (these encode to the shapes in interface/paper.json)

public struct PaperItem: Codable, Sendable, Equatable {
    public var route: String
    public var title: String
    public var dek: String
    public var section: String
    public var meta: String
    public var byline: String
    public var entityLabels: String      // "Ukraine · Russia", shown under the lead and in the cluster
    public var entityIds: String         // "ukraine,russia": what lane 4's "more of this" records
    public var size: Double
    public var weight: Double
    public var lineHeight: Double
    public var lines: Int
    public var showDek: Bool
    public var dekSize: Double
    public var dekLines: Int
    public var indent: Bool
    public var rank: Int
}

public struct PaperBand: Codable, Sendable, Equatable {
    public var id: String
    public var module: String            // lead | colead | pair | cluster | briefs | river | opinion
    public var label: String
    public var side: Bool                // items sit side by side (a pair that fits)
    public var rule: Bool                // a rule between items
    public var items: [PaperItem]
}

public struct PaperSection: Codable, Sendable, Equatable { public var id: String; public var label: String; public var count: Int; public var current: Bool }

public struct PaperFront: Codable, Sendable, Equatable {
    public var status: String            // ready | empty | offline | error
    public var message: String
    public var edition: String
    public var layout: String
    public var composition: String       // "lead+cluster+pair+briefs+river": the modules the rules chose
    public var dateLine: String
    public var updated: String
    public var sectionId: String
    public var sectionLabel: String
    public var count: Int
    public var sections: [PaperSection]
    public var bands: [PaperBand]

    public static func empty(status: String = "empty", message: String = "") -> PaperFront {
        PaperFront(status: status, message: message, edition: "", layout: "", composition: "", dateLine: "", updated: "", sectionId: "", sectionLabel: "",
                   count: 0, sections: [], bands: [])
    }
}

public enum PaperText {
    public static func slug(_ s: String) -> String {
        let t = s.lowercased().map { $0.isLetter || $0.isNumber ? String($0) : "-" }.joined()
        return t.split(separator: "-").joined(separator: "-")
    }
    public static func relative(_ d: Date?, now: Date) -> String {
        guard let d else { return "" }
        let s = Int(now.timeIntervalSince(d))
        if s < 90 { return "just now" }
        if s < 3600 { return "\(s / 60) min ago" }
        if s < 86400 { return "\(s / 3600) h ago" }
        return "\(s / 86400) d ago"
    }
    public static func dateLine(_ d: Date) -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "EEE d MMM"
        f.timeZone = TimeZone(identifier: "UTC")
        return f.string(from: d)
    }
}
