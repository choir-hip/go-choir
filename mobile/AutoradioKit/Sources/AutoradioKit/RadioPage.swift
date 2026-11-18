import Foundation

// The Autoradio page (docs/design/autoradio-page-2026-11-17.md): the run sheet made readable. This file is the whole of what the page
// knows: a pure view model from the run sheet and the player's state to the words and rows the page draws, and the gestures the page
// sends. The page only reads the run sheet and sends the ops the bar and the old sheet sent; it owns no state of the radio.
//
// Ways this can fail (written before the code; each is a test in RadioPageTests):
//  1  the status line is wrong for a state (On air, Paused, Ready, Listening, Writing the next stretch, Can't speak: <reason>, offline)
//  2  the listener speaks: the on-air card must dim and the line say Listening, whatever else is true
//  3  the player could not speak while the sheet still says something is on air: the reason is on the line and the card, with one action
//  4  an answer is on air: the page must say which entry it will go back to (the last one an answer cut), never another
//  5  the cold start (nothing played yet) must greet with the real counts and minutes, and only then (not after something was heard)
//  6  a transition is the station talking between two entries: it is not a row in Up next or Heard, and counts as no entry
//  7  Up next must hold every entry ahead in order, with who placed each, the length and the first five split from the rest
//  8  "Why this" exists only when the entry carries a forecast; an entry with none has no control and no empty text
//  9  the sentence being spoken is the one at the player's index; an index past the end shows no sentence, not a crash
// 10  podcasts and clips show no sentence (no transcript on the phone): the progress line alone
//  11 (removed by the owner, 2026-11-17: the Say something row)
// 12  Heard lists the last entries that ended, newest first, with one word each; a failed entry is not called finished
// 13  a reorder is one `move` op (after the entry that now precedes it, `next` at the head, `end` at the foot); a swipe is one `remove`;
//     a tap is the same op "play now" sends (`move` at now); replay from Heard is one `place` at now of a fresh copy
// 14  an op for an entry that is no longer ahead changes nothing (the reducer drops it); the page does not mint a second way
// 15  a time of zero or unknown is no length, not "0 sec"

public struct RadioPageRow: Codable, Sendable, Equatable {
    public var n = 0
    public var id = ""
    public var kind = ""
    public var title = ""
    /// "45 sec", "3 min"; "" when the entry has no estimate.
    public var length = ""
    /// you | desk | phone
    public var by = ""
    /// The small mark that says who placed it: "You", "Desk", "Phone".
    public var byWord = ""
    /// One quiet caption ("Written for other neighbours"); "" when there is none.
    public var note = ""
    public var builtOn: [RadioBuiltOn] = []
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

public struct RadioHeardRow: Codable, Sendable, Equatable {
    public var n = 0
    public var id = ""
    public var kind = ""
    public var title = ""
    /// finished | skipped | cut | left | failed
    public var outcome = ""
    /// A take's publisher handle ("voice-3fa2c1"), so the row can mute it; "" for every other entry.
    public var publisher = ""
    public var builtOn: [RadioBuiltOn] = []
    public init() {}
}

public struct RadioPageInfo: Codable, Sendable, Equatable {
    /// cold | ready | writing | onair | paused | listening | error | offline
    public var state = "ready"
    public var statusLine = ""
    public var dim = false
    public var playing = false
    public var dateLine = ""

    // the on-air card
    public var onAir = false
    public var kindLabel = ""
    public var title = ""
    public var source = ""
    public var published = ""
    public var sentence = ""
    /// 1-based; 0 when no sentence is shown.
    public var sentenceNumber = 0
    public var sentenceCount = 0
    public var elapsed = ""
    public var remaining = ""
    public var fraction = 0.0
    /// "Then back to: <title>" while an answer is on air; "" otherwise.
    public var thenBack = ""
    /// The page the title opens (a story, a book); "" when the entry has none.
    public var openAddress = ""
    public var why = ""
    public var forecast = ""
    public var canWhy = false
    /// The reason the radio cannot speak, and the one action offered ("Try again"); "" when there is none.
    public var cannotSpeak = ""
    public var action = ""
    /// What the listener said, as it was recognised ("" before words come).
    public var heardWords = ""
    public var greeting = ""
    /// The stories the entry on air was built on ("Built on"); empty when it has none (no heading then).
    public var builtOn: [RadioBuiltOn] = []
    /// A Play button is offered only when pressing it does something: not while the desk is writing and the radio will start by itself.
    public var canPlay = true
    /// A take on air: who made it, what they said (the on-device transcript, shown as text), and their record; "" for every other entry.
    public var takePublisher = ""
    public var takeTranscript = ""
    public var takeRecord = ""

    // up next
    public var ahead: [RadioPageRow] = []
    /// Entries beyond the first five.
    public var aheadMore = 0
    public var writing = false
    /// The line under Up next when nothing is ahead; "" when something is.
    public var emptyNote = ""
    /// The desk could not be reached ("" when all is well).
    public var notice = ""

    public var heard: [RadioHeardRow] = []
    public init() {}
}

/// Everything the page reads, gathered by the caller from the run sheet and the player. Nothing here is read from a clock or a file.
public struct RadioPageInput: Sendable {
    public var sheet: RunSheet
    /// `BarInfo.state`: idle | playing | paused | listening | talking | heard | holding | revised | error
    public var barState = "idle"
    /// The words the listener said, as recognised ("" before any).
    public var heardWords = ""
    public var playing = false
    public var sentences: [String] = []
    public var sentenceIndex = 0
    /// 0...1 through the entry on air.
    public var progress = 0.0
    /// Remote audio's own labels ("1:12", "-3:40"); empty when the entry is spoken.
    public var remoteAudio = false
    public var elapsedLabel = ""
    public var remainingLabel = ""
    /// The player's message ("Can't speak: ...", "Could not play ..."); "" when all is well.
    public var message = ""
    public var notice = ""
    public var writing = false
    public var dry = false
    /// The radio is wanted (it was started and not stopped): what is written next starts by itself.
    public var wants = false
    /// When the entry on air was published ("2 h ago"); "" when not known.
    public var published = ""
    /// Ahead entries whose neighbours changed since they were written.
    public var stale: Set<String> = []
    /// The current version of stories the phone holds, by route (from the front list): a cite of another version is noted on its row.
    public var versions: [String: Int] = [:]
    /// The takes the phone holds, by id (for the transcript and record of a take on air).
    public var takeLookup: @Sendable (String) -> Take? = { _ in nil }
    public var now = Date()
    public var timeZone = TimeZone.current
    public init(sheet: RunSheet) { self.sheet = sheet }
}

public enum RadioPage {
    public static let nextShown = 5
    public static let heardShown = 5

    // MARK: Words

    /// Opening, Story, Podcast, Book, Answer, Transition (and the rest the sheet can hold).
    public static func kindLabel(_ e: RunSheetEntry) -> String {
        switch e.kind {
        case .opening: return "Opening"
        case .transition: return "Transition"
        case .story: return "Story"
        case .clip, .episode: return "Podcast"
        case .take: return "Take"
        case .page: return e.paperRoute != nil ? "Story" : "Page"
        case .document: return "Book"
        case .mail: return "Mail"
        case .result: return "Result"
        case .answer: return "Answer"
        }
    }

    public static func length(_ seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 1 else { return "" }
        if seconds < 90 { return "\(Int(seconds.rounded())) sec" }
        return "\(Int((seconds / 60).rounded())) min"
    }

    private static let small = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen",
                                "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty"]
    private static let tens = [25: "twenty-five", 30: "thirty", 35: "thirty-five", 40: "forty", 45: "forty-five", 50: "fifty", 55: "fifty-five", 60: "sixty"]
    static func word(_ n: Int) -> String { n >= 0 && n < small.count ? small[n] : (tens[n] ?? String(n)) }

    public static func outcomeWord(_ e: RunSheetEntry) -> String {
        switch e.ended?.outcome ?? .finished {
        case .finished: return "finished"
        case .skipped: return "skipped"
        case .cut: return "cut"
        case .left: return "left"
        case .failed: return "failed"
        }
    }

    public static func dateLine(_ now: Date, timeZone: TimeZone) -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "EEE d MMM"
        f.timeZone = timeZone
        return f.string(from: now)
    }

    /// The app's one day-part word (PhoneOpening.hello): morning from 5, afternoon from 12, evening from 18, plain "Hello" after midnight.
    static func daypart(_ now: Date, timeZone: TimeZone) -> String {
        var c = Calendar(identifier: .gregorian); c.timeZone = timeZone
        return PhoneOpening.hello(now, calendar: c)
    }

    /// "Good evening. Six things are ready, about eighteen minutes. Press play and I'll start with the opening."
    public static func greeting(ahead: [RunSheetEntry], now: Date, timeZone: TimeZone) -> String {
        let said = ahead.filter { $0.kind != .transition }
        let hello = daypart(now, timeZone: timeZone)
        guard let first = said.first else { return "" }
        let n = said.count
        let minutes = max(1, Int((said.reduce(0) { $0 + $1.estSeconds } / 60).rounded()))
        let things = n == 1 ? "One thing is ready" : "\(word(n).prefix(1).uppercased() + word(n).dropFirst()) things are ready"
        let about = minutes == 1 ? "about a minute" : "about \(word(minutes)) minutes"
        let start = first.kind == .opening ? "the opening" : first.displayTitle
        return "\(hello). \(things), \(about). Press play and I\u{2019}ll start with \(start)."
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

    /// A sentence from what an entry carries when it carries no stated cause.
    static func builtWhy(_ e: RunSheetEntry) -> String {
        let subjects = (e.cites ?? []).filter { $0.role == .subject }.map(\.label).filter { !$0.isEmpty }
        let drawn = subjects.isEmpty ? "" : " It draws on \(subjects.prefix(3).joined(separator: ", "))."
        switch e.placedBy {
        case .listener: return "You added this" + (e.kind == .answer ? " by asking." : ".")
        case .phone:
            if e.kind == .opening { return "The station opens each listening, from the front page it holds." }
            if e.kind == .answer { return "You asked, and this is the answer." }
            return "The radio put this here." + drawn
        case .desk:
            if e.kind == .transition { return "The station speaks between two entries." }
            return "The desk wrote this for you." + drawn
        }
    }

    /// The entry an answer will hand back to: the latest one an answer cut (the player puts its rest back after the answers).
    static func held(in sheet: RunSheet) -> RunSheetEntry? {
        sheet.entries.last { $0.state.isTail && $0.ended?.outcome == .cut && $0.kind != .answer && $0.kind != .transition }
    }

    /// Nothing played yet: nothing heard, or what was heard last ended more than half an hour ago (an earlier listening). A heard entry with no
    /// time on it counts as lately heard.
    public static let coldAfter: TimeInterval = 30 * 60
    static func isCold(tail: [RunSheetEntry], now: Date) -> Bool {
        guard let last = tail.last else { return true }
        guard let at = last.ended?.at, !at.isEmpty, let d = ISO8601DateFormatter().date(from: at) else { return false }
        return now.timeIntervalSince(d) > coldAfter
    }

    // MARK: The model

    public static func make(_ i: RadioPageInput) -> RadioPageInfo {
        var o = RadioPageInfo()
        let air = i.sheet.onAirEntry
        let ahead = i.sheet.ahead().filter { $0.kind != .transition }
        let tail = i.sheet.entries.filter { $0.state.isTail && $0.kind != .transition }
        o.dateLine = dateLine(i.now, timeZone: i.timeZone)
        o.playing = air != nil && i.playing
        o.notice = i.notice
        o.writing = i.writing

        // the on-air card
        if let e = air {
            o.onAir = true
            o.kindLabel = kindLabel(e)
            o.title = e.displayTitle
            o.source = e.audioSpec?.show.isEmpty == false ? e.audioSpec!.show : (e.source?.label ?? "")
            o.published = i.published
            o.openAddress = openAddress(e)
            o.builtOn = builtOn(e, versions: i.versions)
            if e.kind == .take {
                o.takePublisher = e.source?.label ?? ""
                if let id = e.takeID, let t = i.takeLookup(id) { o.takeTranscript = t.transcript; o.takeRecord = (t.record ?? TakeRecordSummary()).words }
            }
            if !i.remoteAudio, i.sentences.indices.contains(i.sentenceIndex) {
                o.sentence = i.sentences[i.sentenceIndex]; o.sentenceNumber = i.sentenceIndex + 1; o.sentenceCount = i.sentences.count
            }
            o.fraction = min(1, max(0, i.progress.isFinite ? i.progress : 0))
            if i.remoteAudio {
                o.elapsed = i.elapsedLabel; o.remaining = i.remainingLabel
            } else if e.estSeconds >= 1 {
                let done = e.estSeconds * o.fraction
                o.elapsed = QueueClock.label(done); o.remaining = "-" + QueueClock.label(max(0, e.estSeconds - done))
            }
            if e.kind == .answer, let h = held(in: i.sheet) { o.thenBack = "Then back to: \(h.displayTitle)" }
            // Why this: the desk's own cause and forecast when the entry carries them (the programme route sends neither today), else a sentence
            // built from what the entry does carry: who placed it, what it draws on, the entry it leads into.
            if let f = e.forecast {
                let cause = f.causedBy.trimmingCharacters(in: .whitespacesAndNewlines)
                if !cause.isEmpty { o.why = cause.prefix(1).uppercased() + cause.dropFirst() + (cause.hasSuffix(".") ? "" : ".") }
                if let p = f.pHeard, p >= 0, p <= 1 { o.forecast = "The desk forecast a \(Int((p * 100).rounded())) in 100 chance that you hear it through." }
            }
            if o.why.isEmpty { o.why = builtWhy(e) }
            o.canWhy = !o.why.isEmpty
        }

        // up next
        let rows: [RadioPageRow] = ahead.enumerated().map { k, e in
            var r = RadioPageRow()
            r.n = k; r.id = e.id; r.kind = kindLabel(e); r.title = e.displayTitle; r.length = length(e.estSeconds)
            r.by = e.placedBy.rawValue; r.byWord = e.placedBy == .listener ? "You" : (e.placedBy == .desk ? "Desk" : "Phone")
            if i.stale.contains(e.id) { r.note = "Written for other neighbours" }
            r.builtOn = builtOn(e, versions: i.versions)
            return r
        }
        o.ahead = rows
        o.aheadMore = max(0, rows.count - nextShown)
        if rows.isEmpty {
            o.emptyNote = i.writing ? "The desk is writing what comes next."
                : (i.notice != "" ? "Nothing is coming up yet." : (i.dry ? "Nothing is coming up. Open the mark below to find a story, a page or a podcast, and add it." : "The desk will write more shortly."))
        }

        // An entry that never made a sound (failed) was not heard: it is not listed.
        let sounded = tail.filter { $0.ended?.outcome != .failed }
        o.heard = sounded.reversed().prefix(heardShown).enumerated().map { k, e in
            var r = RadioHeardRow()
            r.n = k; r.id = e.id; r.kind = kindLabel(e); r.title = e.displayTitle; r.outcome = outcomeWord(e); r.publisher = e.kind == .take ? (e.source?.label ?? "") : ""; r.builtOn = builtOn(e, versions: i.versions)
            return r
        }


        // the status line, one state at a time; the first that is true wins
        // (the greeting is not offered while the radio cannot speak: it would say "Press play" over the reason)
        let listening = ["listening", "talking", "heard"].contains(i.barState)
        let failure = i.playing ? "" : i.message.trimmingCharacters(in: .whitespacesAndNewlines)
        if air == nil, failure.isEmpty, !ahead.isEmpty { o.greeting = greeting(ahead: ahead, now: i.now, timeZone: i.timeZone) }
        if listening {
            o.state = "listening"; o.statusLine = "Listening"; o.dim = true
            let w = i.heardWords.trimmingCharacters(in: .whitespacesAndNewlines)
            o.heardWords = w == "Listening\u{2026}" ? "" : w
        } else if !failure.isEmpty {
            var why = failure
            for p in ["Can\u{2019}t speak: ", "Can't speak: "] where why.hasPrefix(p) { why = String(why.dropFirst(p.count)) }
            o.state = "error"; o.cannotSpeak = why; o.statusLine = "Can\u{2019}t speak: \(why)"; o.action = "Try again"
        } else if !i.notice.isEmpty {
            o.state = "offline"; o.statusLine = "Offline: the desk cannot write ahead"
        } else if air != nil {
            o.state = i.playing ? "onair" : "paused"; o.statusLine = i.playing ? "On air" : "Paused"
        } else if i.writing {
            o.state = "writing"
            o.statusLine = i.wants ? "Writing the next stretch. It will start by itself." : "Writing the next stretch"
            o.canPlay = !i.wants
        } else if !o.greeting.isEmpty && isCold(tail: tail, now: i.now) {
            o.state = "cold"; o.statusLine = "Ready"
        } else {
            o.state = "ready"; o.statusLine = "Ready"
        }
        return o
    }
}

// MARK: The gestures the page sends

/// What a hand on the page means, as the one op each sends (the same ops the bar and the old sheet sent; the page adds no way of its own).
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
/// PaperSource whenever it reads the front list; read by the Autoradio page. Nothing is asked from the network here.
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
    /// "2 h ago"; "" when the story is not in the list.
    public func words(route: String, now: Date = Date()) -> String {
        lock.lock(); let d = at[route]; lock.unlock()
        return PaperText.relative(d, now: now)
    }
}
