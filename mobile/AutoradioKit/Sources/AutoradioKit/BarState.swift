import Foundation

// The bar's state machine and the radio view as data (plan section 1). Lane 1 owns this file.
//   idle | playing | paused | listening | talking | heard | holding | revised | error
// "typing" is not here: it is the focus of the bar's field, which only the contract knows.
//
// Ways this can fail (written before the code):
//  1. A turn leaves no state behind and the bar sticks on "holding" or "talking": every state but the three timed ones
//     (error, revised) is a pure function of the engine phase, so a phase back to "playing" always returns the bar.
//  2. "Revised" or "error" never goes away because nothing asks again: the tracker answers from a clock (`now`), the
//     module schedules one more read for the moment each window ends (`expiry`).
//  3. The same error text arrives twice: the window restarts, it does not stack.
//  4. Nothing is loaded (empty document): never "paused" (a play button with nothing behind it); that is "idle".
//  5. Paused versus idle: a loaded document that is not playing is "paused"; a document with a block index past its end
//     (after a revision shortens it) must clamp, not trap.
//  6. The sentence estimate runs past the block (audio longer than the guess): progress clamps below 1, the last
//     sentence stays lit until the block changes.
//  7. Pause/resume must not count paused time: elapsed time accumulates only while running at normal level (not paused,
//     not yielded to "pause").
//  8. A block that is a clip (no spoken text) has no sentences: current is its summary, one "sentence".
//  9. Thousands of blocks: the item list is capped, the current block is always inside the window.
// 10. The words from a former turn show in a new one: the heard text clears on the phase to "candidate".

public struct BarInfo: Codable, Sendable, Equatable {
    public var state = "idle"
    /// What the centre shows for every state but idle (idle shows the aim, which the contract builds from the page).
    public var text = ""
    public var playing = false
    public var talking = false
    public var signedIn = false
    /// 0...1 along the current programme; the hairline rule on the bar's lower edge.
    public var progress = 0.0
    public init() {}
}

public struct RadioItem: Codable, Sendable, Equatable {
    public var i = 0
    public var text = ""
    /// speech | clip | you
    public var kind = "speech"
    public var current = false
    public var done = false
    public init() {}
}

/// Everything the expanded player and the bar's extras need, in one source.
public struct RadioInfo: Codable, Sendable, Equatable {
    public var title = ""
    public var source = ""
    public var position = ""
    public var index = 0
    public var count = 0
    public var playing = false
    public var paused = false
    /// play | dip | pause: what the yield policy is doing to the programme right now.
    public var level = "play"
    public var speed = "1"
    public var speedChoices: [String] = ["0.8", "1", "1.25", "1.5"]
    /// button | open | hybrid
    public var mode = ""
    public var modeLabel = ""
    public var micOpen = false
    public var heard = ""
    public var holding = ""
    public var revised = ""
    public var progress = 0.0
    public var blockProgress = 0.0
    public var sentenceIndex = 0
    public var sentences: [String] = []
    public var next = ""
    public var items: [RadioItem] = []
    public init() {}
}

/// Elapsed play time inside the current block, for the sentence estimate. Pure: callers pass `now`.
public struct BlockClock: Sendable, Equatable {
    var index = -1
    var elapsed = 0.0
    var since: Date?
    public init() {}

    public mutating func update(index i: Int, running: Bool, now: Date) {
        if i != index { index = i; elapsed = 0; since = running ? now : nil; return }
        if running, since == nil { since = now }
        if !running, let s = since { elapsed += now.timeIntervalSince(s); since = nil }
    }
    public func seconds(now: Date) -> Double { elapsed + (since.map { now.timeIntervalSince($0) } ?? 0) }
}

public final class BarTracker: @unchecked Sendable {
    public static let shared = BarTracker()
    public static let errorWindow = 4.0, revisedWindow = 6.0, wordsPerSecond = 2.6

    private let lock = NSLock()
    private var holdingLine = ""
    private var revisedAt: Date?
    private var revisedSummary = ""
    private var errorAt: Date?
    private var errorText = ""
    private var lastMessage = ""
    private var clock = BlockClock()
    public private(set) var speed = 1.0

    public init() {}

    // MARK: Events

    public func observe(_ e: LabEvent, now: Date = Date()) {
        lock.lock(); defer { lock.unlock() }
        switch e {
        case .holdingSaid(let l): holdingLine = l
        case .phase(_, let to): if to == "candidate" || to == "addressed" { holdingLine = "" }
        case .revisionArrived(_, _, _, let s): revisedAt = now; revisedSummary = s
        case .error(let stage, let m) where ["system1", "desk", "programme", "voice"].contains(stage):
            errorAt = now; errorText = Self.plain(stage: stage, message: m)
        default: break
        }
    }

    static func plain(stage: String, message m: String) -> String {
        if m.contains("not signed in") { return "Sign in to talk to your computer." }
        if m.contains("429") { return "Your computer is busy right now. Try again in a moment." }
        if m.contains("network") || m.contains("timed out") { return "Could not reach choir.news. Check your connection." }
        switch stage {
        case "system1": return "Your computer was slow to understand that; carrying on."
        case "desk": return "Your computer could not act on that. Try asking again."
        case "voice": return "The voice stopped. Tap play to carry on."
        default: return "Something went wrong with the radio."
        }
    }

    public func setSpeed(_ s: Double) { lock.lock(); speed = min(max(s, 0.5), 2.0); lock.unlock() }

    /// When the bar's timed states next end, so the host can ask once more then (nil: nothing is timed).
    public func expiry(now: Date = Date()) -> TimeInterval? {
        lock.lock(); defer { lock.unlock() }
        var out: [TimeInterval] = []
        if let t = errorAt { let r = Self.errorWindow - now.timeIntervalSince(t); if r > 0 { out.append(r) } }
        if let t = revisedAt { let r = Self.revisedWindow - now.timeIntervalSince(t); if r > 0 { out.append(r) } }
        return out.min()
    }

    // MARK: Views

    public func barInfo(player p: ChoirPlayerView, snapshot s: LabSnapshot, now: Date = Date()) -> BarInfo {
        lock.lock(); defer { lock.unlock() }
        var b = BarInfo()
        b.signedIn = p.signedIn; b.playing = p.playing
        if s.blockCount > 0 { b.progress = min(1, (Double(s.blockIndex) + clockProgress(s, doc: nil, now: now)) / Double(s.blockCount)) }
        // A message from the listener path (sign-in, the programme) is new once; it holds the bar for the error window.
        if !p.message.isEmpty, p.message != lastMessage, p.message != "Pick a story below." {
            lastMessage = p.message; errorAt = now; errorText = p.message
        }
        if let t = errorAt, now.timeIntervalSince(t) < Self.errorWindow { b.state = "error"; b.text = errorText; return b }
        switch s.phase {
        case "candidate": b.state = "listening"; b.text = s.transcript.isEmpty ? "Listening…" : s.transcript; return b
        case "addressed", "capturing": b.state = "talking"; b.talking = true; b.text = s.transcript.isEmpty ? "Listening…" : s.transcript; return b
        case "ended": b.state = "heard"; b.text = s.transcript.isEmpty ? "Heard you" : s.transcript; return b
        case "acknowledging", "holding": b.state = "holding"; b.text = holdingLine.isEmpty ? "On it…" : holdingLine; return b
        case "revised": b.state = "revised"; b.text = "Revised · Show"; return b
        default: break
        }
        if let t = revisedAt, now.timeIntervalSince(t) < Self.revisedWindow { b.state = "revised"; b.text = "Revised · Show"; return b }
        if p.playing { b.state = "playing"; b.text = p.title }
        else if !p.title.isEmpty, s.blockCount > 0 { b.state = "paused"; b.text = p.title }
        return b
    }

    func clockProgress(_ s: LabSnapshot, doc: ProgrammeDocument?, now: Date) -> Double {
        let running = s.running && !s.paused && s.level == "play" && s.phase == "playing"
        clock.update(index: s.blockIndex, running: running, now: now)
        let words = max(1, (doc.flatMap { $0.blocks.indices.contains(s.blockIndex) ? $0.blocks[s.blockIndex].spokenText : nil } ?? s.blockText)
            .split(separator: " ").count)
        let duration = max(2.0, Double(words) / Self.wordsPerSecond / speed)
        return min(0.99, clock.seconds(now: now) / duration)
    }

    public static func sentences(_ t: String) -> [String] {
        var out: [String] = []
        t.enumerateSubstrings(in: t.startIndex..<t.endIndex, options: .bySentences) { s, _, _, _ in
            if let s = s?.trimmingCharacters(in: .whitespacesAndNewlines), !s.isEmpty { out.append(s) }
        }
        return out.isEmpty && !t.isEmpty ? [t] : out
    }

    public static func sentenceIndex(_ sentences: [String], progress: Double) -> Int {
        let total = sentences.reduce(0) { $0 + $1.count }
        guard total > 0, sentences.count > 1 else { return 0 }
        var acc = 0
        for (i, s) in sentences.enumerated() {
            acc += s.count
            if Double(acc) / Double(total) > progress { return i }
        }
        return sentences.count - 1
    }

    public func radioInfo(player p: ChoirPlayerView, snapshot s: LabSnapshot, document d: ProgrammeDocument, route: String, now: Date = Date()) -> RadioInfo {
        lock.lock(); defer { lock.unlock() }
        var r = RadioInfo()
        let bp = clockProgress(s, doc: d, now: now)
        let n = d.blocks.count
        let idx = n == 0 ? 0 : min(max(s.blockIndex, 0), n - 1)
        r.title = d.title; r.source = route
        r.index = idx; r.count = n
        r.position = n == 0 ? "" : "\(idx + 1) of \(n)"
        r.playing = p.playing; r.paused = n > 0 && !p.playing
        r.level = s.level; r.speed = Self.speedName(speed)
        r.mode = s.addressing
        switch s.addressing {
        case "button": r.modeLabel = "Hold the mic button to talk"
        case "open": r.modeLabel = "Just speak; the radio is listening"
        case "hybrid": r.modeLabel = "Hold to talk, or speak just after"
        default: r.modeLabel = ""
        }
        r.micOpen = s.micActive
        r.heard = s.phase == "playing" ? "" : s.transcript
        r.holding = holdingLine
        if let t = revisedAt, now.timeIntervalSince(t) < Self.revisedWindow { r.revised = revisedSummary }
        r.blockProgress = bp
        r.progress = n == 0 ? 0 : min(1, (Double(idx) + bp) / Double(n))
        if n > 0 {
            let b = d.blocks[idx]
            let text = b.spokenText ?? b.summary
            r.sentences = Self.sentences(text)
            r.sentenceIndex = Self.sentenceIndex(r.sentences, progress: bp)
            if idx + 1 < n { r.next = d.blocks[idx + 1].spokenText ?? d.blocks[idx + 1].summary }
            // A window of at most 40 items that always holds the current block.
            let lo = max(0, min(idx - 5, n - 40)), hi = min(n, lo + 40)
            for i in lo..<hi {
                var it = RadioItem()
                it.i = i; it.current = i == idx; it.done = i < idx
                switch d.blocks[i] {
                case .speech(let t): it.text = t; it.kind = "speech"
                case .listenerPassage(let t, _): it.text = t; it.kind = "you"
                case .clip: it.text = d.blocks[i].summary; it.kind = "clip"
                }
                r.items.append(it)
            }
        }
        return r
    }

    static func speedName(_ s: Double) -> String { s == s.rounded() ? String(Int(s)) : String(s) }
}

public enum BarState {
    /// Kept for callers that have no tracker of their own: uses the shared one.
    public static func info(player p: ChoirPlayerView, snapshot s: LabSnapshot) -> BarInfo {
        BarTracker.shared.barInfo(player: p, snapshot: s)
    }
}
