import Foundation

// The radio view as data, and the native FACTS the bar is made from. The bar's own page model (its state, words, timed windows, progress, answer card and the run
// sheet overlay) is shared code now: AutoradioLab/bar.ts, from the facts this file keeps (`BarTracker.raw`) and the module hands over (`barFacts`).
// What stays here, and why: the engine's events (holding line, revision, error) are only seen by native code, so they are kept raw with the device's time; the audio's
// own position in the block (`blockFraction`) is the player's.
//
// Ways this can fail (the radio view's; the bar's list is in bar.test.ts):
//  5. A block index past the end (after a revision shortens it) clamps, not traps.
//  6. The words from a former turn show in a new one: the holding line clears on the phase to "candidate".

/// What the expanded player and the mark read of the radio (interface/bar.json `RadioInfo`): the title on air, whether it plays, what the yield policy is doing to it, the speed as a number for the slider
/// and the smoothed microphone level the mark breathes with. Nothing else is read across the boundary: the sentences, progress and the follow-along are shared code's (AutoradioLab/radio-follow.ts).
public struct RadioInfo: Codable, Sendable, Equatable {
    public var title = ""
    public var playing = false
    /// play | dip | pause: what the yield policy is doing to the programme right now.
    public var level = "play"
    /// The speed as a number, for the slider (0.5 to 3).
    public var speedValue = 1.0
    /// 0...1, the smoothed microphone level while the mic is open (the mark breathes with it); 0 when nothing is heard.
    public var micLevel = 0.0
    public init() {}
}

public final class BarTracker: @unchecked Sendable {
    public static let shared = BarTracker()
    public static let revisedWindow = 6.0
    /// The speaking rate the player's own estimates use (ChoirPlayback).
    public static let wordsPerSecond = 2.6

    private let lock = NSLock()
    private var holdingLine = ""
    private var revisedAt: Date?
    private var revisedSummary = ""
    private var errorAt: Date?
    private var errorStage = ""
    private var errorMessage = ""
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
    /// The audio's place in the block now, 0...1: the audio's own position (set by the runtime), for this block; a block with no audio yet (loading, just skipped) is at 0.
    public func blockFraction(_ s: LabSnapshot) -> Double {
        lock.lock(); let src = positionSource; lock.unlock()
        guard let src, let p = src(), p.index == s.blockIndex else { return 0 }
        return min(0.99, max(0, p.fraction))
    }

    /// The block on air as the document holds it, for shared code to make the sentences from (the document is native's): its spoken words (the summary of a clip), its number clamped into the programme, and how many there are.
    public static func blockOnAir(_ s: LabSnapshot, document d: ProgrammeDocument) -> (spoken: String, index: Int, count: Int) {
        let n = d.blocks.count
        guard n > 0 else { return ("", 0, 0) }
        let idx = min(max(s.blockIndex, 0), n - 1), b = d.blocks[idx]
        return (b.spokenText ?? b.summary, idx, n)
    }

    public func radioInfo(player p: ChoirPlayerView, snapshot s: LabSnapshot, document d: ProgrammeDocument) -> RadioInfo {
        lock.lock(); defer { lock.unlock() }
        var r = RadioInfo()
        r.title = d.title
        // The entry on air is one segment: its title heads the player.
        let n = d.blocks.count
        if n > 0 {
            let idx = min(max(s.blockIndex, 0), n - 1)
            if d.segmentIndex(containingBlock: idx) != nil { r.title = ProgrammeNowPlaying.describe(document: d, blockIndex: idx).title }
        }
        r.playing = p.playing; r.level = s.level; r.speedValue = speed
        r.micLevel = (level * 100).rounded() / 100
        return r
    }
}
