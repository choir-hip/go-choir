import Foundation

// Edition setup: the reader picks the topics of their edition on a screen of chips, or says them. It replaces the first-run
// spoken question-and-answer script (owner QA round 1, 2026-11-15: "I really don't like the Q&A script"; topics "should be done
// graphically"; "you should be able to talk to it"; no negative questions; "people are going to want more than two topics").
//
// Where the suggestions come from. The paper's sections are the house list, `internal/sectionlist/house.json` (the web's
// AUTOPAPER_SECTIONS is the same ten, and the front page carries the live list as `sections`): `EditionTopics.house` copies
// their labels. The finer topics are the subjects those ten sections' own questions name (house.json, field `question`), so a
// chip is something the paper actually sorts stories by or writes about. The live sections of today's paper are merged in by
// `suggestions(liveSections:)`, so a section the house list adds later shows up without an app release.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in EditionSetupTests):
//  E1 The same topic twice: by tap, by typing, by speech, in another case, with a plural, or as a synonym ("tech"): one entry.
//  E2 Speech names a topic that is not a suggestion: it is added as the reader's own topic, as said, not dropped.
//  E3 A very long typed topic: cut at a word boundary at `maxTopicLength`; empty or punctuation-only input adds nothing.
//  E4 Nothing selected: valid. It is the general edition, and the edition says so. There is no minimum and no maximum count.
//  E5 A question or a long sentence spoken at the screen is not a topic: no edit, and a short hint.
//  E6 Saving needs no network: the selection is a local file written after every change; a failed write loses nothing in memory.
//  E7 Coming back to the step shows the saved selection; a corrupt or empty file starts empty and never throws.
//  E8 "Not X" or "no X" said aloud is not an exclusion list: it only takes X off the selection if it is there.
//  T1 A subtopic under several parents is one topic: picking it anywhere lights it everywhere; the saved list holds it once.
//  T2 Every subtopic has a parent, no label is a category and a subtopic at once (no cycles), and speech finds the one "AI".
//  T3 Picking a category does not pick its subtopics, and picking a subtopic does not pick its category; a category with only some
//     subtopics picked is "partial". Picking changes no row, only expanding does.
//  E9 Chips of the reader's own topics stay on screen after they are deselected until the step is left, so a mis-tap is undone.

/// What the edition setup offers and how a word becomes a topic.
public enum EditionTopics {
    public static let maxTopicLength = 60

    /// The house sections (id, label): `internal/sectionlist/house.json`.
    public static let house: [(id: String, label: String)] = [
        ("world", "World"), ("us", "U.S."), ("politics", "Politics"), ("business", "Business"), ("tech_ai", "Technology"),
        ("science", "Science"), ("health", "Health"), ("climate", "Climate"), ("culture", "Culture"), ("sports", "Sports"),
    ]

    /// The graph: each house section with the subtopics its own `question` (house.json) covers. A subtopic may sit under several
    /// sections; it is still ONE topic (one label, one selection state). Where each edge comes from:
    ///  World      "events, governments, conflict or society outside the United States": Conflict, Elections (governments), Trade ("relations
    ///             among several countries"), Security (owner's example: Security under Technology and World).
    ///  U.S.       "its people, states, courts, institutions, communities or domestic affairs": Courts, Elections (states, institutions).
    ///  Politics   "elections, parties, legislatures, government policy, officials or political conflict": Elections, Conflict (political
    ///             conflict), Public health (owner's example: government policy).
    ///  Business   "markets, companies, banks, trade, jobs, prices or finance": Markets, Banks, Trade, Jobs, Prices; AI and Energy (owner's
    ///             examples: companies and the economy around them).
    ///  Technology "computing, the internet, AI, software, devices, security": AI, Software, Security.
    ///  Science    "research findings, discoveries, space, physics, biology": Space, Physics, Biology.
    ///  Health     "medicine, disease, public health, health care, drugs": Medicine, Public health.
    ///  Climate    "climate change, energy transition, emissions, extreme weather, nature or conservation": Energy, Extreme weather, Nature.
    ///  Culture    "arts, film, music, books, media, food, religion, ideas": Film, Music, Books, Food, Religion, Media, Ideas.
    ///  Sports     "a game, a match, a team, an athlete, a league or a competition": no subtopics.
    public static let tree: [(label: String, children: [String])] = [
        ("World", ["Conflict", "Elections", "Trade", "Security"]),
        ("U.S.", ["Courts", "Elections"]),
        ("Politics", ["Elections", "Conflict", "Public health"]),
        ("Business", ["Markets", "Banks", "Trade", "Jobs", "Prices", "AI", "Energy"]),
        ("Technology", ["AI", "Software", "Security"]),
        ("Science", ["Space", "Physics", "Biology"]),
        ("Health", ["Medicine", "Public health"]),
        ("Climate", ["Energy", "Extreme weather", "Nature"]),
        ("Culture", ["Film", "Music", "Books", "Food", "Religion", "Media", "Ideas"]),
        ("Sports", []),
    ]

    /// The top level: the tree, then any live section the house list does not have (no subtopics).
    public static func categories(liveSections: [String] = []) -> [(label: String, children: [String])] {
        var out = tree
        for id in liveSections {
            let l = label(forSectionID: id)
            if !l.isEmpty, !out.contains(where: { key($0.label) == key(l) }), !out.contains(where: { $0.children.contains { key($0) == key(l) } }) { out.append((l, [])) }
        }
        return out
    }

    /// Every subtopic once, in first-seen order.
    public static var finer: [String] {
        var seen: [String] = []
        for c in tree { for k in c.children where !seen.contains(k) { seen.append(k) } }
        return seen
    }

    /// The categories a subtopic sits under (one or more).
    public static func parents(of child: String) -> [String] {
        tree.filter { $0.children.contains { key($0) == key(child) } }.map(\.label)
    }

    /// Other ways of saying a suggestion (keys are `key(_:)` of the phrase).
    static let aliases: [String: String] = [
        "tech": "Technology", "technology": "Technology", "artificialintelligence": "AI", "machinelearning": "AI",
        "economy": "Business", "economics": "Business", "usnews": "U.S.", "america": "U.S.", "unitedstates": "U.S.",
        "war": "Conflict", "wars": "Conflict", "election": "Elections", "movies": "Film", "movie": "Film", "films": "Film",
        "cybersecurity": "Security", "weather": "Extreme weather", "environment": "Climate", "sport": "Sports",
        "world news": "World", "worldnews": "World", "arts": "Culture", "art": "Culture",
    ]

    /// The label a house section id carries; an unknown id is made readable ("tech_ai" style ids become words).
    public static func label(forSectionID id: String) -> String {
        if let h = house.first(where: { $0.id == id }) { return h.label }
        let words = id.replacingOccurrences(of: "_", with: " ").replacingOccurrences(of: "-", with: " ").trimmingCharacters(in: .whitespaces)
        return words.isEmpty ? "" : words.prefix(1).uppercased() + words.dropFirst()
    }

    /// The chips to offer: the house sections, any live section not among them, then the finer topics. No duplicates.
    public static func suggestions(liveSections: [String] = []) -> [String] {
        var out: [String] = []
        func add(_ l: String) { if !l.isEmpty, !out.contains(where: { key($0) == key(l) }) { out.append(l) } }
        categories(liveSections: liveSections).forEach { add($0.label) }
        finer.forEach(add)
        return out
    }

    /// Letters and digits only, lower case: "U.S." and "us" are one topic, "Public  Health" and "public health" too.
    static func key(_ s: String) -> String { String(s.lowercased().filter { $0.isLetter || $0.isNumber }) }

    /// Typed or spoken words as one tidy topic, or nil when there is nothing in them (E3).
    public static func tidy(_ raw: String) -> String? {
        let scalars = raw.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }
        var s = String(String.UnicodeScalarView(scalars)).split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
        s = s.trimmingCharacters(in: CharacterSet(charactersIn: " .,;:!?\"'()-"))
        if s.hasSuffix(".") == false, raw.lowercased().contains("u.s."), s.lowercased() == "u.s" { s = "U.S." }
        guard s.contains(where: { $0.isLetter || $0.isNumber }) else { return nil }
        if s.count > maxTopicLength {
            let cut = String(s.prefix(maxTopicLength))
            if let sp = cut.lastIndex(of: " "), cut.distance(from: cut.startIndex, to: sp) >= maxTopicLength / 2 { s = String(cut[..<sp]) } else { s = cut }
            s = s.trimmingCharacters(in: CharacterSet(charactersIn: " .,;:!?\"'()-"))
        }
        if s.isEmpty { return nil }
        return s.prefix(1).uppercased() + s.dropFirst()
    }

    /// The topic a phrase names: a suggestion when it matches one (any case, plural, synonym), else the reader's own words (E1, E2).
    public static func resolve(_ raw: String, among suggestions: [String]) -> String? {
        guard let t = tidy(raw) else { return nil }
        let k = key(t)
        if let s = suggestions.first(where: { key($0) == k }) { return s }
        if k.hasSuffix("s"), let s = suggestions.first(where: { key($0) == String(k.dropLast()) }) { return s }
        if let s = suggestions.first(where: { key($0) + "s" == k }) { return s }
        if let a = aliases[k] ?? aliases[t.lowercased()], let s = suggestions.first(where: { key($0) == key(a) }) { return s }
        return t
    }
}

/// What the reader has chosen: any number of topics, in the order chosen. Zero is the general edition (E4).
public struct EditionSelection: Codable, Equatable, Sendable {
    public private(set) var topics: [String] = []
    public init(topics: [String] = []) { for t in topics where !contains(t) { self.topics.append(t) } }

    public func contains(_ t: String) -> Bool { topics.contains { EditionTopics.key($0) == EditionTopics.key(t) } }

    /// Add by label or words. Returns the topic when it was newly added, nil when it was empty or already there.
    @discardableResult
    public mutating func add(_ raw: String, among suggestions: [String]) -> String? {
        guard let t = EditionTopics.resolve(raw, among: suggestions), !contains(t) else { return nil }
        topics.append(t); return t
    }

    /// Take a topic off. Returns it when it was there.
    @discardableResult
    public mutating func remove(_ raw: String, among suggestions: [String]) -> String? {
        guard let t = EditionTopics.resolve(raw, among: suggestions), let i = topics.firstIndex(where: { EditionTopics.key($0) == EditionTopics.key(t) }) else { return nil }
        return topics.remove(at: i)
    }

    /// A tap: selects it, or deselects it when it was selected.
    public mutating func toggle(_ label: String, among suggestions: [String]) {
        if contains(label) { _ = remove(label, among: suggestions) } else { _ = add(label, among: suggestions) }
    }

    /// Typed words: commas and semicolons separate topics ("space, chess"); a topic may itself contain "and" ("Arts and crafts").
    @discardableResult
    public mutating func addTyped(_ text: String, among suggestions: [String]) -> [String] {
        var added: [String] = []
        for part in text.split(whereSeparator: { $0 == "," || $0 == ";" || $0 == "\n" }) {
            if let t = add(String(part), among: suggestions) { added.append(t) }
        }
        return added
    }
}

/// What a spoken sentence asked for.
public struct SpokenEdit: Equatable, Sendable {
    public var adds: [String] = []
    public var removes: [String] = []
    /// "continue", "done": the reader is finished choosing.
    public var finish = false
    public var isEmpty: Bool { adds.isEmpty && removes.isEmpty && !finish }
}

public enum SpokenTopics {
    static let fillers = ["please", "can you", "could you", "would you", "um", "uh", "okay", "ok", "so", "and", "also", "yeah", "yes", "well", "hey", "now", "then", "the", "a", "an", "my"]
    static let addVerbs = ["i would like to hear about", "i'd like to hear about", "i want to hear about", "i want to know about", "tell me about", "give me", "i'm interested in",
                           "i am interested in", "i care about", "i'd like", "i would like", "i want", "i like", "i love", "add", "select", "include", "pick", "choose", "follow", "put in",
                           "more", "mostly", "mainly", "just", "some"]
    static let removeVerbs = ["take off", "take out", "remove", "drop", "delete", "deselect", "unselect", "unpick", "untick"]
    static let finishPhrases: Set<String> = ["continue", "done", "i'm done", "i am done", "all done", "that's all", "that is all", "that's it", "that is it", "next", "go on",
                                             "finished", "i'm finished", "that's enough", "that is enough", "that's everything"]
    static let questionLead = ["what", "how", "why", "who", "does", "do", "is", "are", "can", "will", "could", "should", "which", "where", "when"]
    static let nothing: Set<String> = ["everything", "anything", "whatever", "stuff", "something", "it", "that", "this", "more", "some", "all", "no", "nope", "nah",
                                       "none", "nothing", "the", "a", "an", "too", "as well", "please", "thanks", "thank you"]
    static let negativeLead = ["not ", "no ", "without ", "never "]
    static let maxItemWords = 6

    static func norm(_ s: String) -> String {
        let t = s.lowercased().replacingOccurrences(of: "\u{2019}", with: "'")
        // keep "u.s." intact as "us"
        let u = t.replacingOccurrences(of: "u.s.", with: "us")
        let kept = u.map { $0.isLetter || $0.isNumber || $0 == " " || $0 == "'" || $0 == "?" || $0 == "," || $0 == ";" || $0 == "&" ? $0 : " " }
        return String(kept).split(separator: " ").joined(separator: " ")
    }

    /// The first prefix of `list` that starts `s` as a whole word, and what is left.
    static func strip(_ s: String, _ list: [String]) -> (String, Bool) {
        for l in list.sorted(by: { $0.count > $1.count }) where s == l || s.hasPrefix(l + " ") || s.hasPrefix(l + ",") {
            return (String(s.dropFirst(l.count)).trimmingCharacters(in: CharacterSet(charactersIn: " ,")), true)
        }
        return (s, false)
    }

    static func stripFillers(_ s: String) -> String {
        var p = s, again = true
        while again { (p, again) = strip(p, fillers) }
        return p
    }

    static func split(_ s: String) -> [String] {
        var t = s
        for sep in [" as well as ", " and also ", " plus ", " & ", ";", " and "] { t = t.replacingOccurrences(of: sep, with: ",") }
        return t.split(separator: ",").map { String($0).trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
    }

    /// "add space and chess" -> adds [space, chess]. "drop chess" -> removes. "space" alone adds. A question adds nothing (E5).
    public static func parse(_ words: String) -> SpokenEdit {
        var out = SpokenEdit()
        let n0 = norm(words)
        var n = stripFillers(n0)
        if n.isEmpty { return out }
        if finishPhrases.contains(n.replacingOccurrences(of: "?", with: "")) { out.finish = true; return out }
        let toks = n.split(separator: " ").map(String.init)
        let (rest, removing) = strip(n, removeVerbs)
        let (afterAdd, adding) = removing ? (rest, false) : strip(n, addVerbs)
        if !removing, !adding, words.contains("?") || (toks.count >= 2 && questionLead.contains(toks[0])) { return out }
        n = removing ? rest : (adding ? afterAdd : n)
        n = stripFillers(n)
        for var item in split(n) {
            item = stripFillers(item)
            var (a, _) = strip(item, addVerbs); item = stripFillers(a)
            let (r, rem) = strip(item, removeVerbs)
            var isRemove = removing || rem
            if rem { item = r }
            for neg in negativeLead where item.hasPrefix(neg) { item = String(item.dropFirst(neg.count)); isRemove = true; break }
            for tail in [" too", " as well", " please"] where item.hasSuffix(tail) { item = String(item.dropLast(tail.count)) }
            (a, _) = strip(item, addVerbs); item = a.trimmingCharacters(in: CharacterSet(charactersIn: " ?"))
            if item.isEmpty || nothing.contains(item) { continue }
            if item.split(separator: " ").count > maxItemWords { continue }
            if isRemove { if !out.removes.contains(item) { out.removes.append(item) } } else if !out.adds.contains(item) { out.adds.append(item) }
        }
        return out
    }
}

/// What a spoken sentence did, and the one short line the radio says back.
public struct SpokenOutcome: Equatable, Sendable {
    public var added: [String] = []
    public var removed: [String] = []
    public var finish = false
    public var reply: String
}

extension EditionSelection {
    /// Speech applied to the selection (the same path the chips use, so a spoken topic and a tapped one are one thing).
    public mutating func apply(spoken words: String, among suggestions: [String]) -> SpokenOutcome {
        let e = SpokenTopics.parse(words)
        var o = SpokenOutcome(reply: "")
        if e.finish { o.finish = true; o.reply = "Okay."; return o }
        for a in e.adds { if let t = add(a, among: suggestions) { o.added.append(t) } }
        for r in e.removes { if let t = remove(r, among: suggestions) { o.removed.append(t) } }
        var parts: [String] = []
        if !o.added.isEmpty { parts.append("Added " + EditionSetupText.joined(o.added) + ".") }
        if !o.removed.isEmpty { parts.append("Took off " + EditionSetupText.joined(o.removed) + ".") }
        if parts.isEmpty { parts.append(e.adds.isEmpty && e.removes.isEmpty ? "Tell me a topic to add, like space or chess." : "You have that already.") }
        o.reply = parts.joined(separator: " ")
        return o
    }
}

public enum EditionSetupText {
    /// The two or three plain sentences that explain this step, said once at the top of the screen.
    public static let explainer = "Your edition is the paper arranged around what you care about. Tap a topic to pick it, open one to see what is inside, or tell me. You can change your edition at any time."
    public static let voiceHint = "Say, for example, add space and chess."
    public static let emptyLine = "Nothing picked yet. You will get the general edition, and you can add topics whenever you like."
    public static let spokenOpen = "Tap the topics you want, or tell me. Say, add space and chess."

    public static func joined(_ xs: [String]) -> String {
        switch xs.count {
        case 0: return ""
        case 1: return xs[0]
        case 2: return xs[0] + " and " + xs[1]
        default: return xs.dropLast().joined(separator: ", ") + " and " + xs.last!
        }
    }

    public static func selectionLine(_ topics: [String]) -> String {
        topics.isEmpty ? emptyLine : (topics.count == 1 ? "Your edition follows: " : "Your edition follows \(topics.count) topics: ") + joined(topics) + "."
    }
}

// MARK: The session: one selection, saved after every change, shared by the screen and the voice stages

/// One line of the tree on screen. `kind`: category | sub | yoursHead | yours.
public struct TopicRow: Equatable, Sendable {
    public var key: String
    public var label: String
    public var kind: String
    public var selected: Bool
    /// A category that is not itself picked but has some of its subtopics picked.
    public var partial: Bool
    public var expandable: Bool
    public var expanded: Bool
    /// Quiet text beside the chip: "2 picked" on a category, "also in Business" on a shared subtopic.
    public var note: String
}

public enum TopicTree {
    /// The rows to show. Collapsed: one per category. A subtopic appears under every parent that is expanded, with the same selection
    /// state everywhere. The reader's own topics come last under "Yours". Picking never changes which rows exist; only `expanded` does.
    public static func rows(liveSections: [String] = [], expanded: Set<String>, selection: EditionSelection, own: [String]) -> [TopicRow] {
        var out: [TopicRow] = []
        for c in EditionTopics.categories(liveSections: liveSections) {
            let picked = c.children.filter { selection.contains($0) }.count
            let isSel = selection.contains(c.label)
            let open = expanded.contains(EditionTopics.key(c.label)) && !c.children.isEmpty
            let note = picked > 0 ? "\(picked) picked" : ""
            out.append(TopicRow(key: "c/" + c.label, label: c.label, kind: "category", selected: isSel, partial: !isSel && picked > 0,
                                expandable: !c.children.isEmpty, expanded: open, note: note))
            if open {
                for k in c.children {
                    let others = EditionTopics.parents(of: k).filter { EditionTopics.key($0) != EditionTopics.key(c.label) }
                    out.append(TopicRow(key: "s/\(c.label)/\(k)", label: k, kind: "sub", selected: selection.contains(k), partial: false, expandable: false,
                                        expanded: false, note: others.isEmpty ? "" : "also in " + EditionSetupText.joined(others)))
                }
            }
        }
        if !own.isEmpty {
            out.append(TopicRow(key: "yours", label: "Yours", kind: "yoursHead", selected: false, partial: false, expandable: false, expanded: false, note: ""))
            for o in own { out.append(TopicRow(key: "y/" + o, label: o, kind: "yours", selected: selection.contains(o), partial: false, expandable: false, expanded: false, note: "")) }
        }
        return out
    }
}

public final class EditionSetupSession: @unchecked Sendable {
    nonisolated(unsafe) public static var shared: EditionSetupSession?

    private let lock = NSLock()
    private var selection: EditionSelection
    private var suggestions: [String]
    /// The reader's own topics seen this visit, kept as chips after a deselect (E9).
    private var ownSeen: [String] = []
    private let file: URL?
    public var onChange: (@Sendable () -> Void)?
    /// Called when the reader says "continue" or "done".
    public var onFinish: (@Sendable () -> Void)?

    private var liveSections: [String] = []
    private var expanded: Set<String> = []

    public init(suggestions: [String], file: URL? = nil, liveSections: [String] = []) {
        self.suggestions = suggestions; self.file = file; self.liveSections = liveSections
        self.selection = Self.load(file) ?? EditionSelection()
        ownSeen = selection.topics.filter { t in !suggestions.contains { EditionTopics.key($0) == EditionTopics.key(t) } }
    }

    /// The saved selection, or nil when there is none or the file is unreadable (E7).
    public static func load(_ file: URL?) -> EditionSelection? {
        guard let file, let d = try? Data(contentsOf: file), let s = try? JSONDecoder().decode(EditionSelection.self, from: d) else { return nil }
        return EditionSelection(topics: s.topics.compactMap { EditionTopics.tidy($0) })
    }

    private func save() {
        guard let file else { return }
        lock.lock(); let s = selection; lock.unlock()
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: file, options: .atomic) }     // local only: no network (E6)
    }

    public var topics: [String] { lock.lock(); defer { lock.unlock() }; return selection.topics }
    /// Sign-out: the selection, the topics seen and the saved file.
    public func wipe() {
        lock.lock(); selection = EditionSelection(); ownSeen = []; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    public var hasSaved: Bool { guard let f = file else { return false }; return FileManager.default.fileExists(atPath: f.path) }

    public func setLiveSections(_ live: [String]) {
        lock.lock(); liveSections = live; suggestions = EditionTopics.suggestions(liveSections: live); lock.unlock(); onChange?()
    }

    /// Open or close a category (the disclosure control). Not a selection.
    public func toggleExpanded(_ label: String) {
        lock.lock(); let k = EditionTopics.key(label); if expanded.contains(k) { expanded.remove(k) } else { expanded.insert(k) }; lock.unlock()
        onChange?()
    }

    private func noteOwn(_ t: String) {
        if !suggestions.contains(where: { EditionTopics.key($0) == EditionTopics.key(t) }), !ownSeen.contains(where: { EditionTopics.key($0) == EditionTopics.key(t) }) { ownSeen.append(t) }
    }

    /// The tree as rows (see `TopicTree.rows`).
    public func treeRows() -> [TopicRow] {
        lock.lock(); defer { lock.unlock() }
        return TopicTree.rows(liveSections: liveSections, expanded: expanded, selection: selection, own: ownSeen)
    }

    public func toggle(_ label: String) {
        lock.lock(); selection.toggle(label, among: suggestions); if selection.contains(label) { noteOwn(EditionTopics.resolve(label, among: suggestions) ?? label) }; lock.unlock()
        save(); onChange?()
    }

    /// Typed topics. Returns what was added.
    @discardableResult
    public func addTyped(_ text: String) -> [String] {
        lock.lock(); let a = selection.addTyped(text, among: suggestions); a.forEach(noteOwn); lock.unlock()
        save(); onChange?(); return a
    }

    /// The reader spoke. Returns the one line to say back (nil when there is nothing to say).
    public func hear(_ words: String) -> String? {
        lock.lock(); let o = selection.apply(spoken: words, among: suggestions); o.added.forEach(noteOwn); lock.unlock()
        save(); onChange?()
        if o.finish { onFinish?() }
        return o.reply.isEmpty ? nil : o.reply
    }

    public func clear() { lock.lock(); selection = EditionSelection(); ownSeen = []; lock.unlock(); save(); onChange?() }
}

// MARK: Stages: voice on the existing engine

/// The programme the engine plays while the reader talks to the setup: one line that says what to do.
public final class EditionSetupProgramme: Programme, @unchecked Sendable {
    public static let stageID = "edition-setup"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Edition setup (first run)", summary: "One spoken line; the reader's words add and remove topics.")
    }
    public init(_ p: StageParams) {}
    public func load() async throws -> ProgrammeDocument {
        guard EditionSetupSession.shared != nil else { throw LabError.unavailable("no edition setup session") }
        return ProgrammeDocument(id: "edition-setup", title: "Your edition", blocks: [.speech(text: EditionSetupText.spokenOpen)])
    }
}

/// System 2 for the setup: a turn adds or removes topics, locally, and the answer is one short line.
public final class EditionSetupDesk: Desk, @unchecked Sendable {
    public static let stageID = "edition-setup-desk"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Edition setup desk (first run)", summary: "Adds and removes edition topics from what the reader says; local, no network.")
    }
    public init(_ p: StageParams) {}
    public func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        AsyncThrowingStream { c in
            if let s = EditionSetupSession.shared, let line = s.hear(turn.listenerWords) {
                c.yield(Revision(id: "edition-setup-\(turn.turnID)", fromBlock: turn.blockIndex + 1, blocks: [.speech(text: line)], keepRest: false, isFinal: true))
            }
            c.finish()
        }
    }
}

public extension StageRegistry {
    static func registerEditionSetupStages(_ r: StageRegistry) {
        r.register(.programme, EditionSetupProgramme.self) { EditionSetupProgramme($0) }
        r.register(.desk, EditionSetupDesk.self) { EditionSetupDesk($0) }
    }
}

// MARK: The edition: written from the choices

public struct Edition: Codable, Equatable, Sendable {
    public struct Section: Codable, Equatable, Sendable { public var heading: String; public var lines: [String] }
    public var title: String
    public var intro: String
    public var writingLines: [String]
    public var sections: [Section]

    public static func parse(_ data: Data) -> Edition? { try? JSONDecoder().decode(Edition.self, from: data) }
    /// Heading, then its lines, section by section: what a plain page shows.
    public var lines: [String] { sections.flatMap { [$0.heading] + $0.lines } }
}

public enum EditionError: Error, CustomStringConvertible {
    case failed(String)
    public var description: String { switch self { case .failed(let m): return m } }
}

public protocol EditionWriter: Sendable {
    /// Writes the edition from the chosen topics (none: the general edition). `progress` receives what is happening, in plain words.
    func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition
}

/// The edition on this phone, from the choices alone: deterministic, no network (E6). The computer's desk is not asked.
public struct LocalEditionWriter: EditionWriter {
    public init() {}
    public static func edition(topics: [String]) -> Edition {
        let lines = topics.isEmpty ? ["The whole paper, until I say otherwise."] : topics
        return Edition(title: "My edition", intro: "This is your edition. Change anything by asking.", writingLines: ["Putting your edition together"],
                       sections: [.init(heading: "What I follow", lines: lines)])
    }
    public func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
        progress("Putting your edition together")
        return Self.edition(topics: topics)
    }
}

// MARK: Finishing: a deadline, never a dead end
//
// The first phone walk's interview ended with "Good. Writing your edition now." and then nothing, for ever. Cause (HEAD of
// lane/autoradio-qa): `AutoradioModule+Onboarding.swift:275` was the only caller of `writeEdition()` on the way to the end,
// inside a drive that no longer exists. On a phone the end of the interview came from `InterviewDesk.submit` (Interview.swift,
// `finish(limit:)` sets status `.done` and returns the closing line); nothing watched `.done`, so the step never became
// "writing". Had it been reached, `UnbuiltEditionWriter.write` threw "not connected in this build", with no deadline anywhere.
// Now Continue calls `EditionCompletion.run`, which always ends: the edition, a failure, or the deadline.
//
//  C1 The writer returns: `.written`.        C2 The writer throws: `.failed(plain words)`.
//  C3 The writer never returns: `.timedOut` at the deadline, and the writer's task is cancelled.
//  C4 Whatever happens, the reader's topics are untouched (they are saved before this runs).

public enum EditionOutcome: Equatable, Sendable {
    case written(Edition)
    case failed(String)
    case timedOut
}

public enum EditionCompletion {
    public static let deadlineSeconds = 20.0
    public static let failedLine = "I couldn't write your edition just now. Your topics are saved."
    public static let timedOutLine = "Writing your edition is taking too long. Your topics are saved."

    public static func run(topics: [String], writer: EditionWriter, deadline: Double = deadlineSeconds,
                           progress: @escaping @Sendable (String) -> Void) async -> EditionOutcome {
        await withTaskGroup(of: EditionOutcome.self) { group in
            group.addTask {
                do { return .written(try await writer.write(topics: topics, progress: progress)) }
                catch is CancellationError { return .timedOut }
                catch { return .failed(failedLine) }
            }
            group.addTask {
                try? await Task.sleep(nanoseconds: UInt64(max(deadline, 0) * 1_000_000_000))
                return .timedOut
            }
            let first = await group.next() ?? .timedOut
            group.cancelAll()
            return first
        }
    }

    /// The general edition: what "Continue anyway" writes. It needs nothing from the network and cannot fail.
    public static func general() -> Edition { LocalEditionWriter.edition(topics: []) }
}
