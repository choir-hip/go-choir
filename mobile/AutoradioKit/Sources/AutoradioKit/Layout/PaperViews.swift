import Foundation

// The paper's front has views (owner QA round 1, 2026-11-15: "the global front page and my edition, and then all the different vertical
// categories; and in my edition there should be an option to edit my edition"): Front page, My edition, then one view per category.
// This file is the data side: which categories there are, which story is in which, and what My edition is built from. The layout of a view
// is the engine's (`PaperComposer.front`); the switcher and the Edit control are the contract's (`paper.contract`).
//
// WHAT A FRONT-PAGE STORY CARRIES (real answer of `GET /api/platform/publications/front`, read 2026-11-15; `PaperStory.parse`): publication_id,
// title, dek (a cut of the opening), updated_at, version_count, route.bare, importance (1...10), `section` (the primary house section id),
// `sections` (ALL the house sections the story is in, 1 to 4 ids), source_count, words, entities ([{id, label}], named people, places and
// bodies; 0 to 7, none on about a third of stories). The response also carries `sections` ([{id, label}] the live house list, in house order)
// and `next_cursor`. Nothing carries topic labels finer than the ten sections; the finer topics (AI, Space) are matched against words.
// The route takes `?section=<id>&limit=<=100&cursor=`: a story is listed when the section is among its `sections`, newest first; a category
// view asks it, because the front's newest 100 stories hold only a handful of Sports or Health.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in PaperViewsTests):
//  V1 No picks: My edition is the general front page with a friendly line and a way to choose; never empty, never an error.
//  V2 Picks that match nothing today: the same, with a line that says so.
//  V3 A pick that is the reader's odd phrase ("chess openings", "u.s. politics", a single letter, only punctuation): never throws, never
//     matches everything; a one-letter topic matches nothing.
//  V4 The front list fails to load or is empty: the views still list, the answer says why (status), My edition falls back to it too.
//  V5 A story in two categories: it is in both views, once in each; one story is never listed twice in a view or in My edition.
//  V6 A category with one story: a page of one, the stack of one (a swipe stops at both ends).
//  V7 An unknown or stale view id: treated as a category with that name, never a crash and never the front.
//  V8 Several picks matching one story: the story comes before one that matches fewer; equal counts keep the front page's own order.
//  V9 The same pick said in another case, plural or synonym ("election", "Tech"): the same pick.
//  V10 The switcher lists categories in the live order and never changes when a view is selected (counts come from the front list, not the view).

public struct LiveSection: Sendable, Equatable {
    public var id: String
    public var label: String
    public init(id: String, label: String) { self.id = id; self.label = label }
}

public enum PaperViews {
    public static let front = "front"
    public static let mine = "mine"
    /// The front page view shows the newest this many stories, as the page always did; My edition reads the whole list it was given.
    public static let frontLimit = 60
    /// What a front or category request asks for (the route's own cap).
    public static let listLimit = 100

    /// The live house sections in a front answer (`sections: [{id, label}]`), in the server's order; none when an older server sends none.
    public static func parseLiveSections(_ object: [String: Any]) -> [LiveSection] {
        var seen = Set<String>(), out: [LiveSection] = []
        for e in (object["sections"] as? [[String: Any]]) ?? [] {
            guard let id = e["id"] as? String, !id.isEmpty, seen.insert(id).inserted else { continue }
            let l = (e["label"] as? String) ?? ""
            out.append(LiveSection(id: id, label: l.isEmpty ? display(id) : l))
        }
        return out
    }

    /// A section name a reader would say: the house label for a house id, a readable form of any other id ("tech_ai" style), a name that is
    /// already words as it is.
    public static func display(_ raw: String) -> String {
        let t = raw.trimmingCharacters(in: .whitespaces)
        if t.isEmpty { return "" }
        if let h = EditionTopics.house.first(where: { $0.id == t }) { return h.label }
        if t.contains(" ") || t.contains(where: { $0.isUppercase }) { return t }
        return EditionTopics.label(forSectionID: t)
    }

    static func key(_ s: String) -> String { EditionTopics.key(s) }

    /// Whether a story is in a category: the category is its primary section or one of its sections. Ids and labels are compared by
    /// their letters and digits, so "tech_ai", "tech-ai" and a fixture's "Tech and AI"-style labels are each found by themselves.
    public static func belongs(_ s: PaperStory, to sec: PaperSection) -> Bool {
        let keys = Set([key(sec.id), key(sec.label)].filter { !$0.isEmpty })
        return ([s.section] + s.sections).contains { keys.contains(key($0)) }
    }

    /// The switcher's categories. With the live list: those, in its order. Without it (an older server, the demo fixtures, an offline start
    /// with an old cache): the sections the stories name, house order first. Counts are of `stories` (the front list), never of the open view,
    /// so selecting a view cannot change the switcher (V10).
    public static func sections(live: [LiveSection], stories: [PaperStory]) -> [PaperSection] {
        var out: [PaperSection] = []
        if !live.isEmpty {
            out = live.map { PaperSection(id: $0.id, label: $0.label, count: 0, current: false) }
        } else {
            var raws: [String] = []
            for s in stories where !s.isOpinion {
                let r = s.section.trimmingCharacters(in: .whitespaces)
                if !r.isEmpty, !raws.contains(where: { key($0) == key(r) }) { raws.append(r) }
            }
            let houseOrder = EditionTopics.house.map(\.id)
            // House sections in house order, then any other by name: the same whatever order the stories arrive in.
            let ranked = raws.sorted { a, b in
                let ia = houseOrder.firstIndex { key($0) == key(a) } ?? Int.max
                let ib = houseOrder.firstIndex { key($0) == key(b) } ?? Int.max
                return ia != ib ? ia < ib : key(a) < key(b)
            }
            out = ranked.map { PaperSection(id: PaperText.slug($0), label: display($0), count: 0, current: false) }
        }
        for i in out.indices { out[i].count = stories.filter { belongs($0, to: out[i]) }.count }
        return out
    }

    /// The category a view id names: by id, else by letters and digits of id or label. Nil for "front", "mine" and a name the list lacks.
    public static func resolve(_ view: String, in sections: [PaperSection]) -> PaperSection? {
        if let e = sections.first(where: { $0.id == view }) { return e }
        let k = key(view)
        return k.isEmpty ? nil : sections.first { key($0.id) == k || key($0.label) == k }
    }

    public static func isCategory(_ view: String) -> Bool { !view.isEmpty && view != front && view != mine }
}

/// My edition: the paper arranged around what the reader picked. Honest about what it is: the front list is the only source, the picks are
/// matched against what each story carries (its sections, its named entities, its words), and nothing is guessed beyond a small table of
/// other ways of saying a topic. A server-side edition would score stories by their meaning; this does not.
public enum MyEdition {
    public enum State: String, Sendable { case ready, noPicks = "nopicks", noMatch = "nomatch" }

    public struct Result: Sendable, Equatable {
        public var state: State
        /// The stories that match at least one pick, in the order given.
        public var stories: [PaperStory]
        /// route -> how many picks the story matches (the layout puts more first).
        public var priority: [String: Int]
        /// "Technology, AI, Space": every pick, as the reader chose them (more than five become "and N more").
        public var from: String
    }

    /// Other ways a story says a finer topic, keyed by the topic's letters and digits. Small on purpose: each entry is a plain synonym or
    /// an everyday instance (a "vaccine" is Medicine), not a guess about meaning. A topic not here matches by its own words only.
    static let terms: [String: [String]] = [
        "ai": ["artificial intelligence", "machine learning", "large language model", "chatbot"],
        "software": ["developers", "open source"],
        "security": ["cybersecurity", "cyber security", "cyberattack", "data breach", "hackers"],
        "conflict": ["war", "ceasefire", "airstrike", "fighting"],
        "elections": ["election", "voters", "ballot", "polls"],
        "courts": ["court", "judge", "supreme court", "lawsuit"],
        "trade": ["tariffs", "exports", "imports", "trade deal"],
        "markets": ["stocks", "stock market", "wall street", "investors", "shares"],
        "banks": ["bank", "banking", "central bank", "lender"],
        "jobs": ["employment", "unemployment", "layoffs", "hiring"],
        "prices": ["inflation", "cost of living"],
        "energy": ["oil", "electricity", "renewables", "solar", "nuclear"],
        "space": ["nasa", "spacex", "rocket", "satellite", "astronaut", "astronomy"],
        "physics": ["quantum", "particle"],
        "biology": ["genome", "dna", "species"],
        "medicine": ["drug", "vaccine", "clinical trial", "hospital", "cancer"],
        "publichealth": ["epidemic", "outbreak", "vaccination", "pandemic"],
        "extremeweather": ["hurricane", "flood", "wildfire", "heatwave", "drought"],
        "nature": ["wildlife", "biodiversity", "conservation"],
        "film": ["movie", "cinema", "box office"],
        "music": ["album", "concert", "singer"],
        "books": ["novel", "author", "publisher", "literature"],
        "food": ["restaurant", "recipe", "chef"],
        "religion": ["church", "pope", "mosque", "synagogue"],
        "media": ["newspaper", "broadcaster", "journalism", "streaming"],
    ]

    static let stopwords: Set<String> = ["the", "and", "for", "with", "from", "about", "into", "over", "news", "new"]

    /// Lower case, accents folded, letters and digits only, words joined by single spaces, a space at both ends so " word " finds a whole word.
    static func padded(_ s: String) -> String {
        let folded = s.folding(options: [.diacriticInsensitive, .caseInsensitive], locale: Locale(identifier: "en_US_POSIX"))
        let mapped = folded.lowercased().map { $0.isLetter || $0.isNumber ? $0 : " " }
        return " " + String(mapped).split(separator: " ").joined(separator: " ") + " "
    }

    /// The phrase as a whole-word run in the text, in the phrase's own form or with a plural s or es on its last word (and the singular of one).
    static func has(_ hay: String, phrase: String) -> Bool {
        let p = padded(phrase).trimmingCharacters(in: .whitespaces)
        guard p.count >= 2 else { return false }
        var forms = [p, p + "s", p + "es"]
        if p.hasSuffix("s"), p.count > 3 { forms.append(String(p.dropLast())) }
        return forms.contains { hay.contains(" " + $0 + " ") }
    }

    /// One pick, resolved once: a category (matched by section) and/or the words to find.
    struct Pick {
        var label: String
        var section: PaperSection?
        var phrases: [String]
        var words: [String]            // a longer phrase also matches when all its main words are in the story
    }

    static func pick(_ raw: String, sections: [PaperSection]) -> Pick? {
        guard let label = EditionTopics.tidy(raw) else { return nil }
        let k = key(label)
        // A category: the house label (or an alias such as "tech") whose live section the paper has.
        var catLabel: String?
        if let h = EditionTopics.house.first(where: { key($0.label) == k || key($0.id) == k }) { catLabel = h.label }
        else if let a = EditionTopics.aliases[k] ?? EditionTopics.aliases[label.lowercased()], EditionTopics.house.contains(where: { $0.label == a }) { catLabel = a }
        if let cl = catLabel {
            let id = EditionTopics.house.first { $0.label == cl }?.id ?? ""
            let sec = sections.first { key($0.id) == key(id) } ?? sections.first { key($0.label) == key(cl) }
            // No such section in this paper (an old cache, a fixture): the name is found in the words instead.
            return Pick(label: cl, section: sec, phrases: sec == nil ? [cl] : [], words: [])
        }
        let main = padded(label).split(separator: " ").map(String.init).filter { $0.count >= 3 && !stopwords.contains($0) }
        let extra = terms[k] ?? terms[key(EditionTopics.resolve(label, among: EditionTopics.finer) ?? label)] ?? []
        // A stopword alone ("the") is not something a story is about.
        let phrases = ([label] + extra).filter { !stopwords.contains(padded($0).trimmingCharacters(in: .whitespaces)) }
        return Pick(label: label, section: nil, phrases: phrases, words: main.count >= 2 ? main : [])
    }

    static func matches(_ p: Pick, story: PaperStory, hay: String) -> Bool {
        if let sec = p.section, PaperViews.belongs(story, to: sec) { return true }
        if p.phrases.contains(where: { has(hay, phrase: $0) }) { return true }
        if p.words.count >= 2, p.words.allSatisfy({ hay.contains(" " + $0 + " ") || hay.contains(" " + $0 + "s ") }) { return true }
        return false
    }

    static func key(_ s: String) -> String { EditionTopics.key(s) }

    /// The reader's picks as one line: all of them in the order chosen, five at most before "and N more".
    public static func fromLine(_ labels: [String]) -> String {
        let shown = labels.prefix(5).joined(separator: ", ")
        return labels.count > 5 ? "\(shown) and \(labels.count - 5) more" : shown
    }

    /// The edition for these picks over these stories (the front list, deduplicated by route, in the order given).
    public static func build(stories: [PaperStory], picks raw: [String], sections: [PaperSection]) -> Result {
        var seenKeys = Set<String>(), labels: [String] = [], resolved: [Pick] = []
        for r in raw {
            guard let p = pick(r, sections: sections), seenKeys.insert(key(p.label)).inserted else { continue }
            labels.append(p.label); resolved.append(p)
        }
        if resolved.isEmpty { return Result(state: .noPicks, stories: [], priority: [:], from: "") }
        var seen = Set<String>(), out: [PaperStory] = [], prio: [String: Int] = [:]
        for s in stories where seen.insert(s.route).inserted {
            let hay = padded(([s.title, s.dek] + s.entities.map(\.label)).joined(separator: " . "))
            let n = resolved.filter { matches($0, story: s, hay: hay) }.count
            if n > 0 { out.append(s); prio[s.route] = n }
        }
        return Result(state: out.isEmpty ? .noMatch : .ready, stories: out, priority: prio, from: fromLine(labels))
    }
}

/// The list the paper is showing right now, for whatever reads "the list the story came from": the article stack (the contract reads the same
/// `routes` and `titles` off the answer) and the player's queue. One place, set whenever the paper answers; `view` says which view it is, and
/// a reader of it that must not change under a story already playing should copy it at the moment the story starts.
public final class PaperCurrentList: @unchecked Sendable {
    public struct Snapshot: Sendable, Equatable {
        public var view: String
        public var routes: [String]
        public var titles: [String]
        public static let empty = Snapshot(view: "", routes: [], titles: [])
        /// The route after (`step` 1) or before (-1) `route` in this list, or nil at either end or for a route not in it.
        public func neighbor(of route: String, step: Int) -> String? {
            guard let i = routes.firstIndex(of: route), routes.indices.contains(i + step) else { return nil }
            return routes[i + step]
        }
    }
    public static let shared = PaperCurrentList()
    private let lock = NSLock()
    private var current = Snapshot.empty
    public init() {}
    public var snapshot: Snapshot { lock.lock(); defer { lock.unlock() }; return current }
    public func set(view: String, routes: [String], titles: [String]) {
        lock.lock(); current = Snapshot(view: view, routes: routes, titles: titles); lock.unlock()
    }
}
