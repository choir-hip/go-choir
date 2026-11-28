import Foundation

// What only native can report about the bar, and nothing else. What plays (the title, playing or paused, the progress, the sentence) is shared code's, from the run sheet, the deck's state and
// the place native reports in it (AutoradioLab/radio-follow.ts `runOf`), on both phones. Every field kept here is a FACT of the voice or the audio, with the reason beside it. The bar's own page
// model (its state, words, timed windows, answer card) is AutoradioLab/bar.ts, from the facts below (`BarTracker.raw`) and the module's `barFacts`.
//
// Ways this can fail (the bar's list is in bar.test.ts):
//  6. The words from a former turn show in a new one: the holding line clears on the phase to "candidate".
//  7. A sign-out leaves the old account's words (a holding line, a revision, an error) for the next account: `wipe` clears them.

/// What the radio view reports, all of it the voice's or the audio engine's to say (the title and whether it plays are shared code's now).
public struct RadioInfo: Codable, Sendable, Equatable {
    /// play | dip | pause: what the yield policy is doing to the programme right now. Native only: the policy answers the microphone (the voice).
    public var level = "play"
    /// The speed as a number, for the slider (0.5 to 3). Native only: the audio engine applies it (time-pitch) and the saved value is read at launch.
    public var speedValue = 1.0
    /// 0...1, the smoothed microphone level while the mic is open (the mark breathes with it); 0 when nothing is heard. Native only: it is made from the microphone's frames.
    public var micLevel = 0.0
    public init() {}
}

public final class BarTracker: @unchecked Sendable {
    public static let shared = BarTracker()
    public static let revisedWindow = 6.0, wordsPerSecond = 2.6

    private let lock = NSLock()
    // The engine's events, kept raw with the device's time. Native only: only native code sees the engine's events (holding line, revision, error), and the time is the device's.
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
    /// The radio view: the yield level (the snapshot's), the speed and the microphone's level. Nothing here says what plays.
    public func radioInfo(level yieldLevel: String) -> RadioInfo {
        lock.lock(); defer { lock.unlock() }
        var r = RadioInfo()
        r.level = yieldLevel; r.speedValue = speed
        r.micLevel = (level * 100).rounded() / 100
        return r
    }
}
