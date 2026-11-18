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
    /// Every section the story belongs to (the front list's `sections`, house ids such as "world", "politics"); `section` is the primary one.
    /// The server's own `?section=` filter uses this list, so a story is in a category when the category is in it. Empty from an older server.
    public var sections: [String]

    public init(route: String, title: String, dek: String = "", section: String = "", words: Int = 0, importance: Int = 5, sources: Int = 0,
                versions: Int = 1, updated: Date? = nil, kind: String = "", author: String = "", entities: [PaperEntity] = [], sections: [String] = []) {
        self.route = route; self.title = title; self.dek = dek; self.section = section; self.words = words; self.importance = importance
        self.sources = sources; self.versions = versions; self.updated = updated; self.kind = kind; self.author = author; self.entities = entities
        self.sections = sections
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
    public var sectionId: String         // the open category's id; "" on the front page and on My edition
    public var sectionLabel: String
    /// Which view this answer is for: "front", "mine" or a category id; and its name for the masthead.
    public var viewId: String
    public var viewLabel: String
    /// My edition: "ready" (arranged around the picks), "nopicks" (nothing picked yet) or "nomatch" (picks, but nothing in the paper matches today);
    /// "" in every other view. For the last two the bands are the general front page and `note` says why.
    public var mineState: String
    /// My edition: what it is built from, one line ("Technology, AI, Space"). "" when nothing is picked.
    public var mineFrom: String
    public var note: String
    public var count: Int
    /// The switcher's categories after "Front page" and "My edition": the live house sections, in house order.
    public var sections: [PaperSection]
    public var bands: [PaperBand]
    /// The stories of this page in reading order (band by band, item by item), each once. An open article is a card in the stack
    /// these make: a swipe goes to the next or previous route here. `titles[i]` is the title of `routes[i]`.
    public var routes: [String]
    public var titles: [String]

    /// The card a swipe from `route` lands on: `step` +1 is the next article, -1 the previous. Nil when `route` is not in the
    /// stack or the swipe runs past either end (the card springs back; the stack never wraps and never leaves for the front page).
    public func neighbor(of route: String, step: Int) -> Int? {
        guard let i = routes.firstIndex(of: route) else { return nil }
        let j = i + step
        return routes.indices.contains(j) ? j : nil
    }

    /// Flatten bands into the stack order. A route in two bands is listed once, at its first place.
    public static func stack(of bands: [PaperBand]) -> (routes: [String], titles: [String]) {
        var seen = Set<String>(), routes: [String] = [], titles: [String] = []
        for b in bands { for it in b.items where !it.route.isEmpty && seen.insert(it.route).inserted { routes.append(it.route); titles.append(it.title) } }
        return (routes, titles)
    }

    public static func empty(status: String = "empty", message: String = "") -> PaperFront {
        PaperFront(status: status, message: message, edition: "", layout: "", composition: "", dateLine: "", updated: "", sectionId: "", sectionLabel: "",
                   viewId: "", viewLabel: "", mineState: "", mineFrom: "", note: "", count: 0, sections: [], bands: [], routes: [], titles: [])
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
