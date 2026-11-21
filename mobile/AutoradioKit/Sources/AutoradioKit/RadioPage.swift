import Foundation

// What native code still holds of the Autoradio page. The page itself (status line, the card on air, up next, heard, the greeting, "built on") is shared code now: AutoradioLab/radio-page.ts over
// radio-model.ts, tests radio-page.test.ts and radio.test.ts. Native hands it FACTS (`RadioPageFacts`, the source `radioPageFacts`): the run sheet as the store holds it, the player's state, the
// desk's flags, the story versions the phone holds and the take on air. Left here until the queue and its posted ops move (the next slices): the gestures (they change the sheet and so
// post ops), `RadioPage.length`, `openAddress` and `builtOn` (the takes pages still use them), `PublishedTimes`.

/// What the Autoradio page reads that only native code knows (interface/radio.json `RadioPageFacts`). Times are seconds since 1970.
public struct RadioPageFacts: Codable, Sendable, Equatable {
    public var at = 0.0
    /// The run sheet as the store keeps it (wire JSON), applied op ids left out.
    public var sheet = ""
    public var playing = false
    public var sentences: [String] = []
    public var sentenceIndex = 0
    public var progress = 0.0
    public var remoteAudio = false
    public var remoteElapsed = 0.0
    public var remoteDuration = 0.0
    public var wants = false
    public var message = ""
    public var notice = ""
    public var writing = false
    public var dry = false
    public var airTitle = "", airRoute = "", publishedAt = 0.0   // the headline being read and the story it is: the page names an on-air paper page that carries no title by it, only when the routes match
    public var versionRoutes: [String] = []
    public var versionNumbers: [Int] = []
    public var stale: [String] = []
    public var takeTranscript = ""
    public var takeRecord = ""
    public init() {}
}

/// A story an entry was built on: the title and the page that opens it (its Texture, which shows its own sources).
public struct RadioBuiltOn: Codable, Sendable, Equatable {
    public var n = 0
    public var title = ""
    public var address = ""
    /// The cite's role when it is not the subject ("quoted", "mentioned"); "" for a subject.
    public var role = ""
    /// "cited v2, now v3" when the story has a newer version than the one cited (the reader opens the current version, not a pinned one).
    public var note = ""
    public init() {}
}

public enum RadioPage {
    public static func length(_ seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 1 else { return "" }
        if seconds < 90 { return "\(Int(seconds.rounded())) sec" }
        return "\(Int((seconds / 60).rounded())) min"
    }


    /// The page an entry's title opens: a story, a book, a file.
    static func openAddress(_ e: RunSheetEntry) -> String {
        if let r = e.paperRoute { return "pub:" + r }
        // A desk story (kind story) names the story it tells in `source` (the first pub: cite): its title opens that story.
        if let s = e.source?.ref, s.hasPrefix("pub:"), e.kind == .story { return "pub:" + (s.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? "") }
        if let b = e.book { return ShelfAddress.address(b.id) }
        return e.filesAddress ?? ""
    }

    /// The stories an entry was built on: its cites and its source that name a story's page (`pub:<route>[@version]` or `texture:<id>`), once each, in
    /// order. The title is the label the server gave (it defaults to the story's own title). An opening, an answer or a clip with no story has none.
    public static func builtOn(_ e: RunSheetEntry, versions: [String: Int] = [:]) -> [RadioBuiltOn] {
        var seen = Set<String>(), out: [RadioBuiltOn] = []
        func add(_ ref: String, _ label: String, _ role: RunSheetRole? = nil) {
            guard ref.hasPrefix("pub:") || ref.hasPrefix("texture:") else { return }
            let address = ref.hasPrefix("pub:") ? "pub:" + (ref.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? "") : ref
            guard address.count > 4, seen.insert(address).inserted else { return }
            var b = RadioBuiltOn()
            b.n = out.count; b.address = address
            if let role, role != .subject { b.role = role.rawValue }
            if ref.hasPrefix("pub:"), let at = ref.firstIndex(of: "@"), ref[ref.index(after: at)...].hasPrefix("v"), let cited = Int(ref[ref.index(at, offsetBy: 2)...]),
               let now = versions[String(address.dropFirst(4))], now != cited { b.note = "cited v\(cited), now v\(now)" }
            let t = label.trimmingCharacters(in: .whitespacesAndNewlines)
            b.title = t.isEmpty ? String(address.drop(while: { $0 != ":" }).dropFirst()) : t
            out.append(b)
        }
        for c in e.cites ?? [] { add(c.ref, c.label, c.role) }
        if let s = e.source { add(s.ref, s.label) }
        return out
    }
}

public enum RadioPageGesture: Equatable, Sendable {
    /// Drag a row to a new place: `before` is the entry it now sits above (nil: the end of the list).
    case reorder(id: String, before: String?)
    /// Swipe a row away.
    case remove(id: String)
    /// Tap a row: play it now (RunSheetPlayer.playUpcoming: a `move` at now).
    case playNow(id: String)
    /// Tap a row of Heard: play it again (a fresh copy placed at now).
    case replay(id: String)
}

extension RunSheetPlayer {
    /// The op a reorder sends: a `move` after the entry that will precede it, `next` when it goes first, `end` when it goes last.
    /// Nil when nothing would change (it is not ahead, or it already sits there).
    func reorderOp(id: String, before: String?) -> RunSheetOp? {
        let ahead = store.sheet.ahead()
        guard let from = ahead.firstIndex(where: { $0.id == id }) else { return nil }
        let rest = ahead.filter { $0.id != id }
        let at: Int
        if let b = before { guard let k = rest.firstIndex(where: { $0.id == b }) else { return nil }; at = k } else { at = rest.count }
        if at == from { return nil }
        if at == 0 { return RunSheetOp(opId: store.mintOpId(), by: .listener, op: .move, at: .next, id: id) }
        if at == rest.count { return RunSheetOp(opId: store.mintOpId(), by: .listener, op: .move, at: .end, id: id) }
        return RunSheetOp(opId: store.mintOpId(), by: .listener, op: .move, at: .after, after: rest[at - 1].id, id: id)
    }

    /// Perform one gesture. Returns the op's result (nil when the gesture changes nothing).
    @discardableResult
    public func perform(_ g: RadioPageGesture) -> RunSheetResult? {
        switch g {
        case .reorder(let id, let before):
            guard let op = reorderOp(id: id, before: before) else { return nil }
            return store.apply(op)
        case .remove(let id):
            guard store.sheet.ahead().contains(where: { $0.id == id }) else { return nil }
            return store.remove(id: id)
        case .playNow(let id):
            guard let i = store.sheet.ahead().firstIndex(where: { $0.id == id }) else { return nil }
            playUpcoming(at: i)
            return nil
        case .replay(let id):
            guard let old = store.sheet.entry(id), old.state.isTail else { return nil }
            var again = old
            again.id = RunSheetEntry.mintId(old.placedBy == .listener ? "l-" : "p-")
            again.state = .ahead; again.ended = nil; again.placedBy = .listener
            playNow(again)
            return nil
        }
    }
}

/// When each story of the front list was last updated, by route, so the page can say when the story on air was published. Filled by
/// the radio (RadioProgramme.candidates) whenever it reads the front list; read by the Autoradio page. Nothing is asked from the network here.
public final class PublishedTimes: @unchecked Sendable {
    public static let shared = PublishedTimes()
    private let lock = NSLock()
    private var at: [String: Date] = [:]
    private var ver: [String: Int] = [:]
    public init() {}
    public func record(_ stories: [PaperStory]) {
        lock.lock(); defer { lock.unlock() }
        for s in stories { if let d = s.updated { at[s.route] = d }; ver[s.route] = s.versions }
    }
    public func versions() -> [String: Int] { lock.lock(); defer { lock.unlock() }; return ver }
    /// When the story was last updated, seconds since 1970; 0 when the story is not in the list. The words ("2 h ago") are the page's (radio-page.ts).
    public func updated(route: String) -> Double {
        lock.lock(); let d = at[route]; lock.unlock()
        return d?.timeIntervalSince1970 ?? 0
    }
}
