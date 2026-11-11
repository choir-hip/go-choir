import Foundation

// Preferences: "What I've told the paper" (plan section 1, "What the swipes mean" and "Where the learning lives").
//
// The scored acts (a swipe right on a story, an explicit "less of this") are kept as a list the listener can read,
// edit and delete; the weights the ranking uses are derived from that list on every read, never stored apart from
// it. In the plan the list is a Texture beside the listener's edition, one line per act in plain words:
//   "More like this: Russia-Ukraine talks (Ukraine, Russia, Abu Dhabi talks) · 11 Nov 00:44".
// DEMO PATH (built): the acts live in a local file, `what-i-told-the-paper.json` in the app's support directory, and the
// document text is `lines()`. REAL PATH (named, not built): the same lines as the revisions of a private Texture,
// "What I've told the paper", written through the Texture desk (`PUT` of a revision on the listener's computer, the
// reader lane's Texture client); the file is then only a cache, and `lines()` is what the revision holds. No route for
// that exists in the app yet; nothing here pretends it does.
//
// The shading (plan, "Effect"): a story sharing a central entity with "more" acts gets a lift, one sharing with "less"
// acts a dip, bounded at +-25% (`maxLift`, as the web's section lift) and decaying with age (half-life 14 days). The
// lift moves a story at most `placesPerLift` places in the order it already has (rank 0 is the paper's own order). The
// shared paper's ranking is never changed by one listener; this is applied on the phone only.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. A story with no entities and no section: nothing to learn from. record() refuses it (returns nil) instead of
//     writing an act that matches everything or nothing.
//  2. The same entity spelled two ways ("Harbor Authority", " harbor  authority "): both are normalised (lower case,
//     trimmed, runs of spaces collapsed) before comparing.
//  3. A double fire (two swipe events for one gesture): the same kind on the same route within 2 s is one act.
//  4. The file is corrupt or empty: the preferences start empty and the file is rewritten at the next change;
//     they never throw. An unknown gesture meaning in the file falls back to the default.
//  5. Many acts on one topic pile up without limit: the lift saturates (tanh), so ten acts do not move a story
//     more than `placesPerLift` places, and a "less" and a "more" on the same entity cancel.
//  6. The clock moves backwards (an act dated in the future): age is clamped at zero, never a weight above one.
//  7. A tie in the shaded order: the sort is by (shaded key, original index), so equal stories keep the paper's order and
//     the result is a permutation of the input (nothing lost, nothing doubled).
//  8. Two threads (the module's handlers and the main thread): one lock covers the acts and the file write.
//  9. "Forget" of a route with no acts, "reset" of nothing: no event, no error; the result says how many were removed.

public struct ToldAct: Codable, Equatable, Sendable {
    public var at: Double
    public var kind: String            // "more" | "less"
    public var route: String
    public var title: String
    public var entities: [String]
    public var topic: String
    public init(at: Double, kind: String, route: String, title: String, entities: [String], topic: String) {
        self.at = at; self.kind = kind; self.route = route; self.title = title; self.entities = entities; self.topic = topic
    }
}

/// What the gestures mean, as data (plan: "Gesture meanings are a setting"). A change applies to the next gesture.
public struct GestureMeanings: Codable, Equatable, Sendable {
    /// Swipe left on a story: "close" (neutral, the default), "close-less" (close and less of this), "less-in-stack"
    /// (neutral on open pages, less of this in the story stack only).
    public var left: String
    /// Swipe right on a story: "more" (the default) or "nothing".
    public var right: String
    public static let lefts = ["close", "close-less", "less-in-stack"]
    public static let rights = ["more", "nothing"]
    public init(left: String = "close", right: String = "more") {
        self.left = Self.lefts.contains(left) ? left : "close"
        self.right = Self.rights.contains(right) ? right : "more"
    }
    /// Whether a swipe left on a story means "less of this" on this surface ("pile" or "stack").
    public func leftIsLess(surface: String) -> Bool {
        left == "close-less" || (left == "less-in-stack" && surface == "stack")
    }
}

/// What the learning needs to know of a story. `entities` are the central entities the server will send
/// (a server gap today); until then StoryTopics derives a weak set.
public struct StoryFacts: Equatable, Sendable {
    public var route: String
    public var title: String
    public var dek: String
    public var section: String
    public var entities: [String]
    public init(route: String, title: String, dek: String = "", section: String = "", entities: [String] = []) {
        self.route = route; self.title = title; self.dek = dek; self.section = section; self.entities = entities
    }
    public var address: String { "pub:" + route }
    public var topic: String { section }
}

public final class Preferences: @unchecked Sendable {
    public static let maxLift = 0.25
    public static let halfLifeDays = 14.0
    public static let placesPerLift = 3.0
    static let doubleFireSeconds = 2.0

    private let lock = NSLock()
    private var acts: [ToldAct] = []
    private var meanings = GestureMeanings()
    private let file: URL?
    private let now: @Sendable () -> Date

    private struct Saved: Codable { var acts: [ToldAct]; var left: String?; var right: String? }

    /// `file` nil keeps everything in memory (tests).
    public init(file: URL?, now: @escaping @Sendable () -> Date = { Date() }) {
        self.file = file; self.now = now
        if let file, let d = try? Data(contentsOf: file), let s = try? JSONDecoder().decode(Saved.self, from: d) {
            acts = s.acts.filter { ($0.kind == "more" || $0.kind == "less") && !$0.route.isEmpty }
            meanings = GestureMeanings(left: s.left ?? "close", right: s.right ?? "more")
        }
    }

    private func save() {
        guard let file else { return }
        let s = Saved(acts: acts, left: meanings.left, right: meanings.right)
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: file, options: .atomic) }
    }

    // MARK: Gestures

    public var gestures: GestureMeanings { lock.lock(); defer { lock.unlock() }; return meanings }

    /// side "left" or "right"; returns false for an unknown side or meaning (nothing changes).
    @discardableResult
    public func setGesture(side: String, meaning: String) -> Bool {
        lock.lock(); defer { lock.unlock() }
        switch side {
        case "left" where GestureMeanings.lefts.contains(meaning): meanings.left = meaning
        case "right" where GestureMeanings.rights.contains(meaning): meanings.right = meaning
        default: return false
        }
        save(); return true
    }

    // MARK: Acts

    public var all: [ToldAct] { lock.lock(); defer { lock.unlock() }; return acts }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return acts.count }

    static func norm(_ s: String) -> String {
        s.lowercased().split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
    }

    /// The subjects an act learns from: the story's entities, else its section. Empty when there is nothing.
    static func subjects(_ f: StoryFacts) -> [String] {
        let e = f.entities.map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
        if !e.isEmpty { return e }
        let t = f.section.trimmingCharacters(in: .whitespacesAndNewlines)
        return t.isEmpty ? [] : [t]
    }

    /// The name the acknowledgement uses: the first central entity, else the section.
    public static func primary(_ f: StoryFacts) -> String { subjects(f).first ?? "" }

    /// "More on Harbor Authority" / "Less of Harbor Authority".
    public static func note(kind: String, _ f: StoryFacts) -> String {
        let p = primary(f)
        if p.isEmpty { return kind == "more" ? "More like this" : "Less like this" }
        return kind == "more" ? "More on \(p)" : "Less of \(p)"
    }

    @discardableResult
    public func record(kind: String, _ f: StoryFacts) -> ToldAct? {
        guard kind == "more" || kind == "less" else { return nil }
        let subj = Self.subjects(f)
        guard !subj.isEmpty, !f.route.isEmpty else { return nil }
        lock.lock(); defer { lock.unlock() }
        let t = now().timeIntervalSince1970
        if let last = acts.last(where: { $0.route == f.route && $0.kind == kind }), abs(t - last.at) < Self.doubleFireSeconds { return nil }
        let a = ToldAct(at: t, kind: kind, route: f.route, title: f.title, entities: subj, topic: f.topic)
        acts.append(a); save(); return a
    }

    /// Forget the acts on one story. Returns how many were removed.
    @discardableResult
    public func forget(route: String) -> Int {
        lock.lock(); defer { lock.unlock() }
        let n = acts.count
        acts.removeAll { $0.route == route }
        if acts.count != n { save() }
        return n - acts.count
    }

    /// Remove the most recent act (the stack's Undo takes back its own act). Returns it.
    @discardableResult
    public func removeLast(route: String, kind: String) -> ToldAct? {
        lock.lock(); defer { lock.unlock() }
        guard let i = acts.lastIndex(where: { $0.route == route && $0.kind == kind }) else { return nil }
        let a = acts.remove(at: i); save(); return a
    }

    @discardableResult
    public func reset() -> Int {
        lock.lock(); defer { lock.unlock() }
        let n = acts.count
        acts = []
        if n > 0 { save() }
        return n
    }

    // MARK: Effect

    private func weight(_ a: ToldAct, at t: Double) -> Double {
        let age = max(0, t - a.at)
        return (a.kind == "more" ? 1 : -1) * pow(0.5, age / (Self.halfLifeDays * 86400))
    }

    /// The story's lift, in -maxLift...+maxLift.
    public func lift(for f: StoryFacts) -> Double {
        lock.lock(); defer { lock.unlock() }
        return liftLocked(f)
    }

    private func liftLocked(_ f: StoryFacts) -> Double {
        let mine = Set(Self.subjects(f).map(Self.norm))
        guard !mine.isEmpty, !acts.isEmpty else { return 0 }
        let t = now().timeIntervalSince1970
        var s = 0.0
        for a in acts {
            let theirs = Set(a.entities.map(Self.norm))
            let shared = mine.intersection(theirs).count
            var m = Double(min(shared, 2))
            if m == 0, !f.topic.isEmpty, Self.norm(f.topic) == Self.norm(a.topic) { m = 0.35 }
            s += weight(a, at: t) * m
        }
        return Self.maxLift * tanh(s / 1.5)
    }

    /// The paper's order, shaded: a permutation of `stories`, each with its lift. Stable for equal keys.
    public func order(_ stories: [StoryFacts]) -> [(story: StoryFacts, lift: Double)] {
        lock.lock(); defer { lock.unlock() }
        let lifted = stories.enumerated().map { (i, s) -> (Int, StoryFacts, Double, Double) in
            let l = liftLocked(s)
            return (i, s, l, Double(i) - (l / Self.maxLift) * Self.placesPerLift)
        }
        return lifted.sorted { $0.3 != $1.3 ? $0.3 < $1.3 : $0.0 < $1.0 }.map { (story: $0.1, lift: $0.2) }
    }

    /// "Why this story": the part after "Ranked by the paper." that says what the listener's acts did.
    public func why(for f: StoryFacts) -> String {
        lock.lock(); defer { lock.unlock() }
        let mine = Self.subjects(f)
        var parts: [String] = []
        for e in mine {
            let n = Self.norm(e)
            let more = acts.filter { $0.kind == "more" && $0.entities.contains { Self.norm($0) == n } }.count
            let less = acts.filter { $0.kind == "less" && $0.entities.contains { Self.norm($0) == n } }.count
            if more > 0 { parts.append("you asked for more on \(e) (\(more == 1 ? "once" : "\(more) times"))") }
            if less > 0 { parts.append("you asked for less of \(e) (\(less == 1 ? "once" : "\(less) times"))") }
        }
        return parts.isEmpty ? "Ranked by the paper. Nothing you told it changes this." : "Ranked by the paper. Shaded for you: " + parts.joined(separator: "; ") + "."
    }

    // MARK: The document

    /// One line per act, newest first, in plain words.
    public func lines(timeZone: TimeZone = .current) -> [String] {
        lock.lock(); defer { lock.unlock() }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX"); f.timeZone = timeZone; f.dateFormat = "d MMM HH:mm"
        return acts.reversed().map { a in
            let who = a.entities.joined(separator: ", ")
            return "\(a.kind == "more" ? "More like this" : "Less of this"): \(a.title) (\(who)) · \(f.string(from: Date(timeIntervalSince1970: a.at)))"
        }
    }
}

// MARK: Entities for a story

/// Where a story's entities come from. The server will send `entities: [{id, label}]` (central only) on front-list stories
/// (a server gap, plan section 1). Until it does: a fixture map in demo mode (`fixtures/demo/entities.json`, invented),
/// else a weak derivation from the title (runs of capitalised words after the first word, then long words), and the
/// section as the topic.
public enum StoryTopics {
    static let stop: Set<String> = ["about", "after", "again", "their", "there", "these", "those", "which", "while", "where", "other",
                                    "could", "would", "should", "being", "before", "during", "within", "without", "against", "among"]

    public static func demoMap() -> [String: [String]] {
        guard let d = Demo.fixture("entities"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: [String]] else { return [:] }
        return o
    }

    public static func entities(route: String, title: String, section: String, given: [String] = [], map: [String: [String]] = [:]) -> [String] {
        let g = given.map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        if !g.isEmpty { return g }
        if let m = map[route], !m.isEmpty { return m }
        return derive(title: title)
    }

    /// Capitalised runs after the first word ("The council of Port Hale" -> "Port Hale"), else up to two long words.
    static func derive(title: String) -> [String] {
        let words = title.split(whereSeparator: { $0.isWhitespace }).map { $0.trimmingCharacters(in: CharacterSet.punctuationCharacters) }.filter { !$0.isEmpty }
        var out: [String] = [], run: [String] = []
        for (i, w) in words.enumerated() {
            if i > 0, let c = w.first, c.isUppercase { run.append(w) } else { if !run.isEmpty { out.append(run.joined(separator: " ")); run = [] } }
        }
        if !run.isEmpty { out.append(run.joined(separator: " ")) }
        if out.isEmpty {
            out = words.filter { $0.count >= 6 && !stop.contains($0.lowercased()) }.prefix(2).map { $0.capitalized }
        }
        return Array(out.prefix(3))
    }

    public static func facts(route: String, title: String, dek: String, section: String, given: [String] = [], map: [String: [String]] = [:]) -> StoryFacts {
        StoryFacts(route: route, title: title, dek: dek, section: section, entities: entities(route: route, title: title, section: section, given: given, map: map))
    }
}

// MARK: The story stack's session

/// The story-stack mode (plan section 3): a stack of stories as cards. Decided cards keep their place at the front of the
/// order (so the card parked off-screen and the Undo are stable); the rest are in the shaded order and are re-ranked as
/// the listener tells the paper things (a "more" on a harbor story brings the next harbor story forward).
///
/// Ways this can fail: (1) a decided story is no longer in the list (a refresh): it is dropped from the front, the
/// index follows; (2) deciding the same route twice: the second is ignored; (3) undo with nothing decided: nil;
/// (4) an empty list: the index is 0 and there is no top card.
public final class StackSession: @unchecked Sendable {
    private let lock = NSLock()
    private var decisions: [(route: String, side: String)] = []
    public init() {}

    public struct Row: Equatable, Sendable {
        public var story: StoryFacts
        public var lift: Double
        public var n: Int
        public var dir: String           // "", "left", "right"
    }

    public func decide(route: String, side: String) {
        lock.lock(); defer { lock.unlock() }
        guard side == "left" || side == "right", !decisions.contains(where: { $0.route == route }) else { return }
        decisions.append((route, side))
    }
    @discardableResult
    public func undo() -> (route: String, side: String)? { lock.lock(); defer { lock.unlock() }; return decisions.popLast() }
    public func reset() { lock.lock(); decisions = []; lock.unlock() }
    public var setAside: Int { lock.lock(); defer { lock.unlock() }; return decisions.filter { $0.side == "left" }.count }

    /// `shaded` is the preferences' order of the whole list.
    public func rows(shaded: [(story: StoryFacts, lift: Double)]) -> [Row] {
        lock.lock(); let ds = decisions; lock.unlock()
        var rows: [Row] = []
        var used = Set<String>()
        for d in ds {
            if let s = shaded.first(where: { $0.story.route == d.route }) {
                rows.append(Row(story: s.story, lift: s.lift, n: rows.count, dir: d.side)); used.insert(d.route)
            }
        }
        for s in shaded where !used.contains(s.story.route) {
            rows.append(Row(story: s.story, lift: s.lift, n: rows.count, dir: ""))
        }
        return rows
    }
    public func index(shaded: [(story: StoryFacts, lift: Double)]) -> Int {
        lock.lock(); defer { lock.unlock() }
        return decisions.filter { d in shaded.contains { $0.story.route == d.route } }.count
    }
}
