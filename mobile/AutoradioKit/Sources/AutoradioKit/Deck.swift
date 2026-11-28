import Foundation

// The deck (station X4): the small native audio surface UNDER shared code. Shared code (TypeScript, the same on both phones) decides what plays
// next, what counts as heard and what is posted; the deck plays what it is handed, applies a short list of named rules that cannot wait for
// shared code, keeps ONE state for why it is silent, and keeps a journal of what started and ended with the device's time, the place reached
// and whether sound was rendered. The names and shapes are docs/reference/exact/deck.md, mobile/exact-package/interface/deck.json and
// mobile/AutoradioLab/deck.ts; Kotlin implements the same.
//
// The sound is behind `PlaybackEngine` (the Swift engine that already exists, unchanged). Today the Swift run sheet player still decides, and it
// reports to the deck what it started and how it ended (`began`, `soundBegan`, `ended`), so the journal is the evidence from the first day;
// when the decisions move to shared code, shared code commands the deck (`play`, `enqueue`, `act`) and the player is deleted.
//
// The rules the deck applies by itself (design, "The deck applies named rules"):
//  1. An item with `requiresPreviousHeard` plays only if the item before it ended finished with sound rendered; otherwise it is dropped from
//     the held list (no journal line; the state's `held` shows it gone).
//  2. When the audio session cannot be taken the item stays on air with no outcome and is tried again at 1, 2, 4, 8, 15 s (the last repeats).
//  3. A route loss or the listener's pause is not undone by a timer; the system's interruption ending is.
//  4. Two items in a row that made no sound stop the deck (state `stopped`); a dead engine must not burn the queue.
//  5. The deck plays on through the held items with shared code not running.
// Barge-in (the voice lowering the deck) and one-voice-at-a-time stay in SessionCoordinator and the engine: they are native to native.
//
// Ways this can fail, and what is done (each has a test in DeckTests):
//  - sound that was not made journalled as heard: `rendered` is true only after `soundBegan`, or when the host says played; a finish with none is "failed".
//  - a kill leaves an item open: the next launch closes it first, as failed ("interrupted by a restart"), before anything is restored.
//  - journal numbers reused after a relaunch: the number is stored with the lines and acknowledged lines leave but the number never falls.
//  - the journal grows without bound: it keeps the newest 2000 lines.
//  - two flags that disagree about why it is silent: one state, set only by the methods below.
//  - a late end from an item that was replaced closes the new one: an end names the item it is for and is ignored when it is not the current one.

/// What plays the sound. Spoken things go through the programme player; remote audio through AVPlayer.
@MainActor public protocol PlaybackEngine: AnyObject {
    /// Speak `paragraphs` as the document `documentID` (one segment named by what it is: `kind` is opening, transition, answer, result, mail; "" a story), replacing what is playing. False: nothing started.
    /// `source` is the item's `paragraphs` string exactly as the deck received it: the place reported while it speaks is a range into it.
    func speak(documentID: String, entryID: String, title: String, kind: String, paragraphs: [String], source: String) async -> Bool
    /// The spoken entry from the block that did not play (the engine was lost mid-entry and is back).
    func resumeSpoken()
    /// The spoken entry from its start.
    func restartSpoken()
    /// Where the spoken entry is: the server block being spoken, the last server block it has, and the seconds into it.
    func spokenReport() -> (block: Int, final: Int, seconds: Double)
    /// Where the voice is in the text the deck was handed (the unit being spoken, as a range into it), read once: voice, because only the player knows what it is saying and how far. Empty when it cannot be told.
    func spokenPlace() -> DeckPlace
    /// The block of the spoken entry and the seconds into it.
    func spokenPosition() -> (block: Int, seconds: Double)
    /// Silence the spoken entry (something else is starting, or it was skipped).
    func stopSpoken()
    /// Why the last spoken block could not be made (nil: it was).
    func speakFailure() -> String?
    /// What of the spoken entry is still to be said, from the start of the sentence it is in; nil when unknown.
    func spokenRemainder() -> String?
    /// The spoken entry says no more than the block it is on (the rest is replaced): whole sentences, no word cut.
    func endSpokenAfterCurrentBlock()
    /// Play remote audio from `start` (seconds) and stop at `end` when there is one.
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool
    func pauseRemote()
    func resumeRemote()
    /// Silence remote audio.
    func stopRemote()
    func seekRemote(by seconds: Double)
    func seekRemote(to seconds: Double)
    /// The playing clip no longer ends: the episode carries on from where it is, with no gap or restart.
    func clearRemoteEnd()
    func remotePosition() -> (elapsed: Double, duration: Double?)?
    func setRemoteRate(_ r: Double)
    func setRemoteLevel(_ l: PlayLevel)
    /// The length of the audio at `url`, read without playing it; nil when unknown within `deadline` seconds.
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double?
    /// Pause the spoken entry (the listener's pause, or the system's).
    func pauseSpoken()
    /// Sound is reaching the output for what is playing now (spoken: rendered seconds past zero; remote: the position is moving). The deck's
    /// evidence that an item was heard; a deck that cannot tell says false and nothing is ever journalled as rendered by it.
    func rendering() -> Bool
}
extension PlaybackEngine {
    public func pauseSpoken() {}
    public func rendering() -> Bool { false }
    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { nil }
    public func restartSpoken() {}
    public func resumeSpoken() {}
    public func spokenReport() -> (block: Int, final: Int, seconds: Double) { (0, 0, 0) }
    public func spokenPosition() -> (block: Int, seconds: Double) { (0, 0) }
    public func spokenPlace() -> DeckPlace { DeckPlace() }
    public func stopSpoken() {}
    public func endSpokenAfterCurrentBlock() {}
}

public extension PlaybackEngine { func speakFailure() -> String? { nil }; func spokenRemainder() -> String? { nil } }

public enum DeckWhy: String, Codable, Sendable {
    case playing, pausedListener = "paused_listener", pausedSystem = "paused_system", heldSession = "held_session", heldEngine = "held_engine", stopped, empty
}

public struct DeckItem: Equatable, Sendable {
    public var id: String
    public var kind: String                 // "speech" or "audio"
    public var title = "", show = "", document = ""
    /// The text in one string, a blank line between paragraphs.
    public var paragraphs = ""
    public var url = ""
    public var start = 0.0, end = 0.0
    public var block = 0
    public var seconds = 0.0
    public var requiresPreviousHeard = false
    /// What a speech item is for the voice's segment naming: opening, transition, answer, result, mail; "" is a story.
    public var segment = ""
    public init(id: String, kind: String) { self.id = id; self.kind = kind }
    var paragraphList: [String] { paragraphs.components(separatedBy: "\n\n").map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty } }
    var documentID: String { document.isEmpty ? "deck:" + id : document }
}

/// One line of the journal (the wire shape of `DeckEvent`).
public struct DeckLine: Codable, Equatable, Sendable {
    public var seq: Int
    public var type: String                 // started, sounding, ended, remote, system
    public var item: String
    public var at: String                   // RFC 3339, the device's clock
    public var block: Int
    public var seconds: Double
    public var final: Int
    public var rendered: Bool
    public var end: String                  // finished, skipped, replaced, stopped, failed
    public var command: String
    public var reason: String
}

extension DeckLine {
    /// What the run sheet is told of an `ended` line: the twin of deck-ops.ts (the fixtures in mobile/fixtures/deck hold both to one answer).
    /// Heard only with sound; a finish with none is "could not play".
    var airedEnd: (state: RunSheetState, outcome: RunSheetOutcome, lastBlock: Int, seconds: Double, reason: String?)? {
        guard type == "ended" else { return nil }
        switch end {
        case "finished": return rendered ? (.heard, .finished, final, seconds, nil) : (.skipped, .failed, 0, 0, "ended with no sound rendered")
        case "skipped": return (.skipped, .skipped, block, seconds, nil)
        case "replaced": return (.skipped, .cut, block, seconds, nil)
        case "stopped": return rendered ? (.heard, .left, block, seconds, nil) : (.skipped, .failed, 0, 0, "stopped before it made a sound")
        default: return (.skipped, .failed, block, seconds, reason.isEmpty ? "could not play" : reason)
        }
    }
}

/// The journal: ordered, durable, acknowledged by shared code once the server has the ops. A file of its own that survives a kill and is
/// readable after the first unlock, so it can be written with the screen locked.
@MainActor public final class DeckJournal {
    public static let maxLines = 2000
    public private(set) var lines: [DeckLine] = []
    public private(set) var seq = 0
    public private(set) var acked = 0
    /// How many unacknowledged lines the cap has dropped, ever (each drop writes a `system` line that says how many).
    public private(set) var dropped = 0
    /// Saves that failed (a full disk, a protected file). The whole journal is written again at the next event.
    public private(set) var saveFailures = 0
    public var log: (String) -> Void = { _ in }
    public var now: () -> Date = { Date() }
    private let storage: URL?
    /// False while the file is there and could not be read (a background launch before the first unlock): the lines made meanwhile are held in
    /// memory, nothing is saved over the file, and they are merged after it once it reads. An ABSENT file is a fresh journal.
    public private(set) var isLoaded = true
    private var floor = 0           // the numbers known before the file could be read: lines held meanwhile are numbered after the larger of this and the file's

    public init(storage: URL? = nil) {
        self.storage = storage
        guard storage != nil else { return }
        switch read() {
        case .absent: break
        case .ok(let s): lines = s.lines; seq = s.seq; acked = s.acked
        case .unreadable: isLoaded = false
        case .corrupt: setAside()
        }
        // The numbers have a file of their own: a lost or unreadable lines file never makes them start again.
        if let n = readNumbers() { seq = max(seq, n.seq); acked = max(acked, n.acked) }
        floor = seq; lines.removeAll { $0.seq <= acked }
        if aside { append("system", item: "", command: "journal-corrupt", reason: "the journal file could not be decoded and was moved aside") }
    }

    public static func applicationSupportFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("deck-journal.json")
    }
    /// The small file beside the journal that holds `seq` and `acked`.
    static func seqFile(for storage: URL) -> URL { storage.deletingPathExtension().appendingPathExtension("seq.json") }

    struct Saved: Codable { var seq: Int; var acked: Int; var lines: [DeckLine] }
    struct Numbers: Codable { var seq: Int; var acked: Int }
    private enum Read { case absent, ok(Saved), unreadable, corrupt }
    private var aside = false, bootPending = false

    private func read() -> Read {
        guard let storage else { return .absent }
        guard FileManager.default.fileExists(atPath: storage.path) else { return .absent }
        guard let d = try? Data(contentsOf: storage) else { return .unreadable }          // cannot be read now (protected data): kept, tried again
        guard let s = try? JSONDecoder().decode(Saved.self, from: d) else { return .corrupt }   // read, and still no journal: it would wedge journalling for good
        return .ok(s)
    }
    /// A file that reads and cannot be decoded is moved aside (`.bad`), never deleted, and the numbers file carries on.
    private func setAside() {
        guard let storage else { return }
        let bad = storage.deletingPathExtension().appendingPathExtension("bad")
        try? FileManager.default.removeItem(at: bad); try? FileManager.default.moveItem(at: storage, to: bad); aside = true
    }
    private func readNumbers() -> Numbers? {
        guard let storage, let d = try? Data(contentsOf: Self.seqFile(for: storage)) else { return nil }
        return try? JSONDecoder().decode(Numbers.self, from: d)
    }

    /// Try the unreadable file again. When it reads, the lines made meanwhile follow it, numbered after it.
    private func ensureLoaded() {
        guard !isLoaded else { return }
        let held = lines
        switch read() {
        case .unreadable: return
        case .absent: lines = []; seq = floor
        case .corrupt: setAside(); lines = []; seq = floor
        case .ok(let s): lines = s.lines; seq = max(floor, s.seq); acked = max(acked, s.acked)
        }
        lines.removeAll { $0.seq <= acked }
        seq = max(seq, lines.last?.seq ?? 0, acked)
        for var l in held { seq += 1; l.seq = seq; lines.append(l) }
        isLoaded = true
        if aside { aside = false; append("system", item: "", command: "journal-corrupt", reason: "the journal file could not be decoded and was moved aside") }
        if bootPending { bootPending = false; closeOpenAtBoot() }          // the launch found the file unreadable: the item a kill left open is closed now
    }

    static func stamp(_ d: Date) -> String {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime]
        return f.string(from: d)
    }

    @discardableResult
    func append(_ type: String, item: String, block: Int = 0, seconds: Double = 0, final: Int = 0, rendered: Bool = false, end: String = "",
                command: String = "", reason: String = "") -> DeckLine {
        ensureLoaded()
        seq += 1
        let l = DeckLine(seq: seq, type: type, item: item, at: Self.stamp(now()), block: block, seconds: seconds, final: final, rendered: rendered, end: end, command: command, reason: reason)
        lines.append(l)
        if lines.count > Self.maxLines {
            // Every line here is unacknowledged (an acknowledged one has already gone): the cap is hit by unacknowledged lines alone. The oldest go,
            // and a `system` line says how many, so shared code sees the gap (never a silent drop).
            // It cuts a tenth at once, so the marker is one line and not one per event.
            let cut = lines.count - Self.maxLines + Self.maxLines / 10
            lines.removeFirst(cut); dropped += cut
            seq += 1
            lines.append(DeckLine(seq: seq, type: "system", item: "", at: Self.stamp(now()), block: 0, seconds: 0, final: 0, rendered: false, end: "", command: "dropped", reason: "dropped \(cut) unacknowledged lines"))
        }
        save()
        return l
    }

    /// Shared code (or the player) has what is up to `n`: those lines go. The number never falls.
    public func ack(through n: Int) {
        ensureLoaded()
        guard n > acked else { return }
        acked = min(n, seq)
        lines.removeAll { $0.seq <= acked }
        save()
    }

    /// The item started and not yet ended, if any.
    public var openItem: String? {
        var open: String?
        for l in lines {
            if l.type == "started" { open = l.item } else if l.type == "ended", l.item == open { open = nil }
        }
        return open
    }

    /// A launch finds an item open (the app was killed): it is closed as failed before anything else reads the journal. A restart is no verdict,
    /// however much sound there was.
    @discardableResult
    public func closeOpenAtBoot() -> DeckLine? {
        ensureLoaded()
        if !isLoaded { bootPending = true; return nil }
        guard let item = openItem else { return nil }
        let last = lines.last { $0.item == item && ($0.type == "sounding" || $0.type == "started") }
        let sounded = lines.contains { $0.item == item && $0.type == "sounding" }
        return append("ended", item: item, block: last?.block ?? 0, seconds: last?.seconds ?? 0, final: last?.final ?? 0, rendered: sounded, end: "failed", reason: "interrupted by a restart")
    }

    public func wipe() {
        lines = []; seq = 0; acked = 0; dropped = 0; isLoaded = true
        if let storage { try? FileManager.default.removeItem(at: storage); try? FileManager.default.removeItem(at: Self.seqFile(for: storage))
            try? FileManager.default.removeItem(at: storage.deletingPathExtension().appendingPathExtension("bad")) }   // the set-aside journal holds the account's lines too
    }

    /// Nothing is written while the file could not be read (it would destroy what it holds). A failed write is counted and logged with counts only;
    /// the next event writes everything held again.
    private func save() {
        guard let storage, isLoaded else { return }
        do {
            let d = try JSONEncoder().encode(Saved(seq: seq, acked: acked, lines: lines)), n = try JSONEncoder().encode(Numbers(seq: seq, acked: acked))
            #if os(iOS)
            try n.write(to: Self.seqFile(for: storage), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
            try d.write(to: storage, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
            #else
            try n.write(to: Self.seqFile(for: storage), options: .atomic)
            try d.write(to: storage, options: .atomic)
            #endif
        } catch {
            saveFailures += 1
            log("deck journal: save failed (\(saveFailures) failures, \(lines.count) lines held)")
        }
    }
}

/// Where the voice is in the entry's text (interface/deck.json `DeckPlace`; shared code makes the sentence from it: AutoradioLab/radio-follow.ts). `entry` is the id the deck was given for the item (empty: no place);
/// `digest` is `DeckDigest.of` the item's `paragraphs` as received; `start` and `length` are the unit being spoken, in UTF-16 code units of that string; `seconds` and `total` are media seconds in the unit (0 when not known yet).
public struct DeckPlace: Codable, Equatable, Sendable {
    public var entry = "", digest = ""
    public var start = 0, length = 0
    public var seconds = 0.0, total = 0.0
    public init() {}
    public init(entry: String, digest: String, start: Int, length: Int, seconds: Double, total: Double) {
        self.entry = entry; self.digest = digest; self.start = start; self.length = length; self.seconds = seconds; self.total = total
    }
}

/// A unit's range in the text it was cut from, in UTF-16 code units.
public struct UnitRange: Codable, Sendable, Equatable {
    public var start: Int, length: Int
    public init(start: Int, length: Int) { self.start = start; self.length = length }
}

/// The digest of an entry's text, and where each spoken unit lies in it. Voice: the cut is the voice engine's own bound, so only native knows where each unit begins.
public enum DeckDigest {
    /// The length in UTF-16 code units, a colon, and the 32-bit FNV-1a of those code units in eight hex digits (AutoradioLab/radio-follow.ts `digestOf` and Android's `DeckDigest` make the same string).
    public static func of(_ s: String) -> String {
        var h: UInt32 = 0x811c9dc5, n = 0
        for u in s.utf16 { h = (h ^ UInt32(u)) &* 0x01000193; n += 1 }
        let hex = String(h, radix: 16)
        return "\(n):" + String(repeating: "0", count: 8 - hex.count) + hex
    }
    /// Each unit's range in `source`, in order. The units are `source` cut and trimmed, with a run of white space (a newline joined to a space) standing for the run it came from, so a unit is matched
    /// character by character, white space to white space. Nil when any unit cannot be found that way: no place is reported rather than a wrong one.
    public static func ranges(of units: [String], in source: String) -> [UnitRange]? {
        let src = Array(source.unicodeScalars)
        var offset = [Int](); offset.reserveCapacity(src.count + 1)
        var o = 0
        for u in src { offset.append(o); o += u.utf16.count }
        offset.append(o)
        func blank(_ u: Unicode.Scalar) -> Bool { u.properties.isWhitespace }
        var i = 0, out: [UnitRange] = []
        for unit in units {
            let us = Array(unit.unicodeScalars)
            guard !us.isEmpty else { return nil }
            while i < src.count, blank(src[i]) { i += 1 }
            let start = i
            var j = 0
            while j < us.count {
                if blank(us[j]) {
                    while j < us.count, blank(us[j]) { j += 1 }
                    guard i < src.count, blank(src[i]) else { return nil }
                    while i < src.count, blank(src[i]) { i += 1 }
                } else {
                    while i < src.count, blank(src[i]) { i += 1 }
                    guard i < src.count, src[i] == us[j] else { return nil }
                    i += 1; j += 1
                }
            }
            out.append(UnitRange(start: offset[start], length: offset[i] - offset[start]))
        }
        return out
    }
}

/// What `deckState` answers.
public struct DeckSnapshot: Codable, Equatable, Sendable {
    public var why: DeckWhy
    public var item: String
    public var block: Int
    public var seconds: Double
    public var duration: Double
    public var place: DeckPlace
    public var held: [String]
    public var seq: Int
    public var acked: Int
    public var journal: [DeckLine]
}

@MainActor public final class Deck {
    public static let shared = Deck(journal: DeckJournal(storage: DeckJournal.applicationSupportFile()))
    /// Every entry the run sheet can have ahead (its own bound, 100): shared code hands over each ready entry whose sound is in the entry, so the deck plays on through all of them with shared code not running. Memory is bounded by that
    /// bound times an entry's own bounds (a text or an address, the server's entry limits): a few hundred kilobytes at most.
    public static let heldMax = 100

    public let journal: DeckJournal
    public var engine: PlaybackEngine?
    /// Called before every start the deck makes itself: the host claims the audio session and readies the engine. nil: ready; else why not.
    public var sessionReady: ((String) async -> String?)?
    public var onChange: (() -> Void)?
    /// Seconds between tries when the session cannot be taken (the last repeats).
    public var retryDelays: [TimeInterval] = [1, 2, 4, 8, 15]

    public private(set) var current: DeckItem?
    public private(set) var held: [DeckItem] = []
    /// The one state of why the deck is silent. `playing` when an item is on and nothing holds it; `empty` with nothing.
    public private(set) var why: DeckWhy = .empty
    /// The current item was started by `play` or by playing on (shared code commands the deck), not by the Swift player.
    public private(set) var commanded = false
    private var sounded = false
    private var silentRun = 0
    private var lastHeard = false           // the item before ended finished with sound
    private var attempts = 0
    private var retryTask: Task<Void, Never>?
    private var generation = 0

    public init(journal: DeckJournal) { self.journal = journal }

    /// Sign-out: the item on air, the held items and the reason go with the journal, the engine stops: a late end writes nothing for the next account.
    public func wipe() {
        retryTask?.cancel(); retryTask = nil; generation += 1
        let wasPlaying = current != nil
        current = nil; held = []; why = .empty; sounded = false; commanded = false; silentRun = 0; lastHeard = false; attempts = 0
        if wasPlaying { stopEngine() }
        journal.wipe()
        changed()
    }

    /// At launch, before the run sheet is restored: an item the kill left open is closed in the journal.
    @discardableResult public func boot() -> DeckLine? { journal.closeOpenAtBoot() }

    // MARK: What the Swift player reports (it still decides)

    /// The player started `id` (sound is expected, none yet).
    public func began(id: String, kind: String = "speech", block: Int = 0, seconds: Double = 0) {
        if current?.id == id { return }
        if current != nil { ended(.replaced) }
        var i = DeckItem(id: id, kind: kind); i.block = block; i.seconds = seconds
        current = i; sounded = false; commanded = false; why = .playing; attempts = 0
        journal.append("started", item: id, block: block, seconds: seconds)
        changed()
    }

    /// Sound reached the output for the current item. Written once per item.
    public func soundBegan(block: Int = 0, seconds: Double = 0, final: Int = 0) {
        guard let c = current, !sounded else { return }
        sounded = true
        journal.append("sounding", item: c.id, block: block, seconds: seconds, final: final, rendered: true)
    }

    public enum How: String { case finished, skipped, replaced, stopped, failed }

    /// The current item left the air. `played`: the host says a block made sound (a finish counts as sound even if `soundBegan` was never seen).
    @discardableResult
    public func ended(_ how: How, played: Bool? = nil, block: Int = 0, seconds: Double = 0, final: Int = 0, reason: String = "", id: String? = nil) -> DeckLine? {
        guard let c = current, id == nil || id == c.id else { return nil }
        if played == true { sounded = true }
        let rendered = sounded
        let how = (how == .finished && !rendered) ? How.failed : how
        let line = journal.append("ended", item: c.id, block: block, seconds: seconds, final: final, rendered: rendered, end: how.rawValue,
                                  reason: how == .failed && reason.isEmpty ? "ended with no sound rendered" : reason)
        current = nil; sounded = false; commanded = false; retryTask?.cancel(); retryTask = nil; attempts = 0; generation += 1
        lastHeard = how == .finished && rendered
        silentRun = rendered ? 0 : silentRun + 1
        if why != .stopped { why = .empty }
        changed()
        return line
    }

    /// A transport or system word worth keeping in the journal (the lock screen's pause, an interruption beginning).
    /// A remote line carries where the item was when the command came (block and seconds), so shared code reports the cut where it happened even if the deck has moved on.
    public func note(remote command: String) {
        let p = current != nil ? position() : (block: 0, seconds: 0.0)
        journal.append("remote", item: current?.id ?? "", block: p.block, seconds: p.seconds, command: command)
    }
    public func note(system command: String) { journal.append("system", item: current?.id ?? "", command: command) }

    // MARK: Commands (shared code)

    /// Play this item now, cutting what is playing.
    @discardableResult
    public func play(_ item: DeckItem) async -> Bool {
        if current != nil { ended(.replaced, block: engine?.spokenPosition().block ?? 0, seconds: position().seconds, final: engine?.spokenReport().final ?? 0) }
        silentRun = 0
        return await start(item, commanded: true)
    }

    /// Hold `item` after the playing one at place `at` (nil: the end). Items from `at` on are dropped first; an id already held is removed first;
    /// a place beyond the limit is refused.
    public func enqueue(_ item: DeckItem, at: Int? = nil) {
        held.removeAll { $0.id == item.id }
        if let at, at >= 0, at < held.count { held.removeSubrange(at...) }
        guard held.count < Self.heldMax else { changed(); return }
        held.append(item)
        if current == nil, why != .stopped { Task { @MainActor in await self.playOn() } }
        changed()
    }

    public func trim(keeping n: Int) { if held.count > max(0, n) { held.removeSubrange(max(0, n)...); changed() } }

    public func act(_ op: String, value: Double = 0) async {
        switch op {
        case "pause": listenerPaused()
        case "resume": await resumed()
        case "stop":
            if current != nil { ended(.stopped, block: engine?.spokenPosition().block ?? 0, seconds: position().seconds) }
            held = []; stopEngine(); why = .stopped; changed()
        case "skip":
            if current != nil { ended(.skipped, block: engine?.spokenPosition().block ?? 0, seconds: position().seconds, final: engine?.spokenReport().final ?? 0); stopEngine() }
            await playOn()
        case "seekTo": engine?.seekRemote(to: value)
        case "seekBy": engine?.seekRemote(by: value)
        case "trim": trim(keeping: Int(value))
        case "clearEnd": engine?.clearRemoteEnd()          // "Keep listening" to a clip: it no longer ends, the episode goes on from where it is
        default: break
        }
    }

    /// The lock screen's and the headset's next: acts at once on the held list (the deck skips and plays on), and the journal says so first, so shared code, when it runs, records one skip of the entry that was on
    /// air (where it was cut) and finds the next already playing. With shared code not running this is all that happens, and the radio goes on.
    public func remoteNext() async {
        note(remote: "next")
        guard current != nil else { return }
        await act("skip")
    }
    /// The lock screen's previous: audio goes back to its start and a spoken entry that has gone on (past its first block, or 3 s) starts again, at once. A spoken entry just begun means the entry before it,
    /// which only shared code knows (the sheet's heard list): the line is journalled and shared code does it when it runs.
    public func remotePrevious() {
        note(remote: "previous")
        guard let c = current else { return }
        let p = position()
        if c.kind == "audio" { engine?.seekRemote(to: c.start) } else if p.block > 0 || p.seconds >= 3 { engine?.restartSpoken() }
    }

    public func ack(through n: Int) { journal.ack(through: n); changed() }

    public func listenerPaused() {
        guard current != nil else { return }
        engine?.pauseRemote(); engine?.pauseSpoken(); why = .pausedListener; note(remote: "pause"); changed()
    }
    /// The system took the audio (an interruption began, a route was lost): not undone by a timer.
    public func systemPaused(routeLost: Bool = false) {
        guard current != nil else { return }
        why = .pausedSystem; note(system: routeLost ? "route-lost" : "interruption-began"); changed()
    }
    /// The listener pressed play, or the system said to resume.
    public func resumed() async {
        guard let c = current else { await playOn(); return }
        why = .playing; note(remote: "play")
        if c.kind == "audio" { engine?.resumeRemote() } else { engine?.resumeSpoken() }
        changed()
    }
    /// The system's interruption ended: resumes only when the system paused it, never over the listener's pause.
    public func systemMayResume() async {
        guard why == .pausedSystem || why == .heldSession || why == .heldEngine else { return }
        note(system: "interruption-ended")
        if why == .heldSession, let c = current { retryTask?.cancel(); retryTask = nil; await begin(c) } else { await resumed() }
    }

    // MARK: What the engine reports (commanded items)

    /// The spoken document `id` ran out of blocks.
    public func documentEnded(id: String, anyPlayed: Bool) {
        guard let c = current, commanded, c.kind == "speech", c.documentID == id else { return }
        finish(anyPlayed: anyPlayed)
    }
    /// Remote audio reached its end.
    public func remoteEnded() {
        guard let c = current, commanded, c.kind == "audio" else { return }
        let r = engine?.remotePosition()
        engine?.stopRemote()
        finish(anyPlayed: true, seconds: r?.elapsed ?? 0)
    }
    /// The engine was lost in the middle of the item: held, and tried again when the session is back.
    public func engineLost() {
        guard current != nil, commanded else { return }
        why = .heldEngine; note(system: "engine-lost"); changed()
    }

    private func finish(anyPlayed: Bool, seconds: Double = 0) {
        senseSound()
        let rep = engine?.spokenReport() ?? (block: 0, final: 0, seconds: 0)
        ended(anyPlayed ? .finished : .failed, played: anyPlayed, block: rep.final, seconds: seconds > 0 ? seconds : rep.seconds, final: rep.final,
              reason: anyPlayed ? "" : "its document ended with no block heard")
        Task { @MainActor in await self.playOn() }
    }

    /// Play on: the first held item the rules allow.
    private func playOn() async {
        guard current == nil else { return }
        if silentRun >= 2 { why = .stopped; changed(); return }
        while let next = held.first {
            held.removeFirst()
            if next.requiresPreviousHeard && !lastHeard { continue }       // rule 1: dropped, no line
            _ = await start(next, commanded: true)
            return
        }
        if why != .stopped { note(system: "held-dry") }                 // nothing held to play on (shared code was not there to refill): journalled, so it is known the radio ran dry
        why = .empty; changed()
    }

    private func start(_ item: DeckItem, commanded cmd: Bool) async -> Bool {
        generation += 1
        current = item; sounded = false; commanded = cmd; why = .playing; attempts = 0
        journal.append("started", item: item.id, block: item.block, seconds: item.seconds)
        changed()
        return await begin(item)
    }

    private func begin(_ item: DeckItem) async -> Bool {
        let my = generation
        if let notReady = await sessionReady?(item.kind) {
            guard my == generation, current?.id == item.id else { return false }
            hold(item, why: notReady)
            return false
        }
        guard my == generation, current?.id == item.id, let engine else { return false }
        if why == .heldSession { note(system: "session-back") }
        why = .playing
        var ok = false
        if item.kind == "audio", let url = URL(string: item.url) {
            ok = await engine.playRemote(url: url, title: item.title, show: item.show, start: item.seconds > 0 ? item.seconds : (item.start > 0 ? item.start : nil), end: item.end > 0 ? item.end : nil)
        } else if item.kind == "speech" {
            ok = await engine.speak(documentID: item.documentID, entryID: item.id, title: item.title, kind: item.segment, paragraphs: item.paragraphList, source: item.paragraphs)
        }
        guard my == generation, current?.id == item.id else { if ok, current == nil { stopEngine() }; return ok }       // wiped while it started: no sound for the next account
        if ok, item.kind == "audio" { soundBegan(block: 0, seconds: item.start) }        // the player took the audio and started it: the journal's `sounding` line (speech: `senseSound`, from what the engine renders)
        if !ok {
            ended(.failed, reason: "the engine could not start it")
            await playOn()
        }
        return ok
    }

    /// The session cannot be taken: the item stays on air, held, no outcome; tried again at 1, 2, 4, 8, 15 s.
    private func hold(_ item: DeckItem, why reason: String) {
        if why != .heldSession { note(system: "session-held") }
        why = .heldSession
        attempts += 1
        let wait = retryDelays[min(attempts - 1, retryDelays.count - 1)]
        changed()
        retryTask?.cancel()
        retryTask = Task { @MainActor [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(wait * 1_000_000_000))
            guard let self, !Task.isCancelled, self.why == .heldSession, self.current?.id == item.id else { return }
            _ = await self.begin(item)
        }
    }

    private func stopEngine() { engine?.stopRemote(); engine?.stopSpoken() }

    private func position() -> (block: Int, seconds: Double) {
        if current?.kind == "audio" { return (0, engine?.remotePosition()?.elapsed ?? 0) }
        let r = engine?.spokenReport() ?? (block: 0, final: 0, seconds: 0)
        return (r.block, r.seconds)
    }

    // MARK: What shared code reads

    public func snapshot() -> DeckSnapshot {
        senseSound()
        let p = position()
        let dur = current?.kind == "audio" ? (engine?.remotePosition()?.duration ?? 0) : 0
        var place = current?.kind == "speech" ? engine?.spokenPlace() ?? DeckPlace() : DeckPlace()
        if place.entry != current?.id { place = DeckPlace() }       // the place is for the item on air, or nothing
        return DeckSnapshot(why: why, item: current?.id ?? "", block: p.block, seconds: current == nil ? 0 : p.seconds, duration: dur, place: place,
                            held: held.map(\.id), seq: journal.seq, acked: journal.acked, journal: journal.lines)
    }

    /// Spoken sound reached the output: the engine renders audio for the item on air (it is asked whenever shared code reads the state, about once a second, and before an end is written), so the journal gets its `sounding` line.
    /// Nothing is written for an item the engine has not rendered, so a silent item is never journalled as heard.
    private func senseSound() {
        guard let c = current, c.kind == "speech", !sounded, why == .playing, engine?.rendering() == true else { return }
        let r = engine?.spokenReport() ?? (block: 0, final: 0, seconds: 0)
        soundBegan(block: r.block, seconds: r.seconds, final: r.final)
    }

    private func changed() { onChange?() }
}
