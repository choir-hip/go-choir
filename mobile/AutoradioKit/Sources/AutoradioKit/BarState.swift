import Foundation

// The radio view as data, and the native FACTS the bar is made from. The bar's own page model (its state, words, timed windows, progress, answer card and the run
// sheet overlay) is shared code now: AutoradioLab/bar.ts, from the facts this file keeps (`BarTracker.raw`) and the module hands over (`barFacts`).
// What stays here, and why: the engine's events (holding line, revision, error) are only seen by native code, so they are kept raw with the device's time; the audio's
// own position in the block (`clockProgress`) is the player's.
//
// Ways this can fail (the radio view's; the bar's list is in bar.test.ts):
//  1. Pause/resume must not count paused time: elapsed time accumulates only while running at normal level (not paused, not yielded to "pause").
//  2. The sentence estimate runs past the block (audio longer than the guess): progress clamps below 1, the last sentence stays lit until the block changes.
//  3. A block that is a clip (no spoken text) has no sentences: current is its summary, one "sentence".
//  5. A block index past the end (after a revision shortens it) clamps, not traps.
//  6. The words from a former turn show in a new one: the holding line clears on the phase to "candidate".

/// Everything the expanded player and the bar's extras need, in one source.
public struct RadioInfo: Codable, Sendable, Equatable {
    public var title = ""
    public var playing = false
    /// play | dip | pause: what the yield policy is doing to the programme right now.
    public var level = "play"
    /// The speed as a number, for the slider (0.5 to 3).
    public var speedValue = 1.0
    public var progress = 0.0   // 0...1 along the whole programme: (block index + the audio's fraction of the block) / block count
    public var sentenceIndex = 0, sentences: [String] = []   // the sentence being said and the block's sentences, for `ReaderFeed` and the `radioFacts` op
    /// 0...1, the smoothed microphone level while the mic is open (the mark breathes with it); 0 when nothing is heard.
    public var micLevel = 0.0
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
    public static let revisedWindow = 6.0, wordsPerSecond = 2.6

    private let lock = NSLock()
    private var holdingLine = ""
    private var revisedAt: Date?
    private var revisedSummary = ""
    private var errorAt: Date?
    private var errorStage = ""
    private var errorMessage = ""
    private var clock = BlockClock()
    /// Sign-out: the held words (a message, a revision summary, the holding line) are the account's.
    public func wipe() {
        lock.lock(); holdingLine = ""; revisedAt = nil; revisedSummary = ""; errorAt = nil; errorStage = ""; errorMessage = ""; lock.unlock()
    }
    public private(set) var speed = 1.0
    private var level = 0.0
    /// The audio's own position in the block (set by the runtime). Without one the estimate from play time is used.
    public var positionSource: (@Sendable () -> RenderPosition?)?
    /// Told when the listener picks a speed, so the audio can apply it.
    public var speedSink: (@Sendable (Double) -> Void)?
    /// Others that follow the speed (remote audio).
    public var speedObservers: [@Sendable (Double) -> Void] = []

    public init() {}

    // MARK: Microphone level
    // Ways this can fail: (1) silence (-120 dB, or all-zero virtual frames) must read as 0, not as a negative level; (2) one loud
    // frame must not stick: the level falls away over a few frames; (3) a flood of frames must not mean a flood of redraws, so the
    // caller is told only when the value moved by more than a few percent.
    public static func levelFor(db: Double) -> Double { min(1, max(0, (db + 55) / 35)) }

    /// Feeds one frame's level in dB. Returns true when the smoothed level moved enough to be worth a redraw.
    public func feedMic(db: Double) -> Bool {
        lock.lock(); defer { lock.unlock() }
        let t = Self.levelFor(db: db)
        let next = t > level ? t : level * 0.85 + t * 0.15
        let moved = abs(next - level) > 0.04 || (next < 0.01 && level != 0)
        level = next < 0.01 ? 0 : next
        return moved
    }
    public var micLevel: Double { lock.lock(); defer { lock.unlock() }; return level }

    // MARK: Events

    public func observe(_ e: LabEvent, now: Date = Date()) {
        lock.lock(); defer { lock.unlock() }
        switch e {
        case .holdingSaid(let l): holdingLine = l
        case .phase(_, let to): if to == "candidate" || to == "addressed" { holdingLine = "" }
        case .revisionArrived(_, _, _, let s): revisedAt = now; revisedSummary = s
        case .error(let stage, let m) where ["system1", "desk", "programme", "voice"].contains(stage):
            errorAt = now; errorStage = stage; errorMessage = m
        default: break
        }
    }

    /// A plain line from the app itself (not an engine event): it holds the bar for the error window like any other.
    public func say(_ line: String, now: Date = Date()) {
        lock.lock(); errorAt = now; errorStage = ""; errorMessage = line; lock.unlock()
    }

    public func setSpeed(_ s: Double) {
        lock.lock(); speed = PlaybackSpeed.snap(s); let v = speed; let sink = speedSink; lock.unlock()
        sink?(v)
        for o in speedObservers { o(v) }
    }

    /// When the radio view's revision line next ends, so the host can ask once more then (nil: nothing is timed). The bar's own timed states are shared code's (bar.ts).
    public func expiry(now: Date = Date()) -> TimeInterval? {
        lock.lock(); defer { lock.unlock() }
        guard let t = revisedAt else { return nil }
        let r = Self.revisedWindow - now.timeIntervalSince(t)
        return r > 0 ? r : nil
    }

    // MARK: Views

    /// What the engine told the bar, as it was told (times in seconds since 1970, 0 for none). bar.ts decides what the bar says.
    public struct Raw: Sendable {
        public var holding = "", revisedAt = 0.0, errorAt = 0.0, errorStage = "", errorMessage = ""
    }
    public func raw() -> Raw {
        lock.lock(); defer { lock.unlock() }
        return Raw(holding: holdingLine, revisedAt: revisedAt?.timeIntervalSince1970 ?? 0, errorAt: errorAt?.timeIntervalSince1970 ?? 0, errorStage: errorStage, errorMessage: errorMessage)
    }
    /// The audio's place in the block now, 0...1 (the audio's own position; an estimate from play time only without one).
    public func blockFraction(_ s: LabSnapshot, now: Date = Date()) -> Double {
        lock.lock(); defer { lock.unlock() }
        return clockProgress(s, doc: nil, now: now)
    }

    func clockProgress(_ s: LabSnapshot, doc: ProgrammeDocument?, now: Date) -> Double {
        // The real position: the audio that was rendered, for this block. A block with no audio yet (loading, just skipped) is at 0.
        if let src = positionSource {
            guard let p = src(), p.index == s.blockIndex else { return 0 }
            return min(0.99, max(0, p.fraction))
        }
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

    public func radioInfo(player p: ChoirPlayerView, snapshot s: LabSnapshot, document d: ProgrammeDocument, now: Date = Date()) -> RadioInfo {
        lock.lock(); defer { lock.unlock() }
        var r = RadioInfo()
        let bp = clockProgress(s, doc: d, now: now)
        let n = d.blocks.count
        let idx = n == 0 ? 0 : min(max(s.blockIndex, 0), n - 1)
        r.title = d.title
        // The entry on air is one segment: its title heads the player.
        if n > 0, d.segmentIndex(containingBlock: idx) != nil { r.title = ProgrammeNowPlaying.describe(document: d, blockIndex: idx).title }
        r.playing = p.playing
        r.level = s.level; r.speedValue = speed
        r.micLevel = (level * 100).rounded() / 100
        r.progress = n == 0 ? 0 : min(1, (Double(idx) + bp) / Double(n))
        if n > 0 {
            let b = d.blocks[idx]
            let text = b.spokenText ?? b.summary
            r.sentences = Self.sentences(text)
            r.sentenceIndex = Self.sentenceIndex(r.sentences, progress: bp)
        }
        return r
    }
}
