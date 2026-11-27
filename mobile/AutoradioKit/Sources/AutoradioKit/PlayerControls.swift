import Foundation

// The player's controls: the speed range, the list a story came from, and what happens when a story ends.
// Failure list: mobile/README.md, "Player controls" (the numbers 1 to 18 are in the test names, PlayerControlsTests.swift).

// MARK: Speed

/// The reading speed. One range, one step, one place that clamps: the tracker, the player and the audio all call `clamp`.
/// Every voice is rendered to buffers and played through `LiveAudio.timePitch` (rate range 1/32 to 32, pitch kept), so a voice's own
/// rate is never used and the ceiling is the same for all of them: `maximum`.
public enum PlaybackSpeed {
    public static let minimum = 0.5
    public static let maximum = 3.0
    public static let step = 0.05

    /// Into the range; NaN and infinity are 1 (natural).
    public static func clamp(_ s: Double) -> Double { s.isFinite ? min(maximum, max(minimum, s)) : 1 }
    /// Clamped, then to the nearest step, so what the label shows is what is stored.
    public static func snap(_ s: Double) -> Double {
        let c = clamp(s)
        let n = (c / step).rounded() * step
        return clamp((n * 1000).rounded() / 1000)
    }
}

/// Where the chosen speed is kept between launches. A value that is not a number in range is ignored.
public struct SpeedStore: @unchecked Sendable {
    let defaults: UserDefaults
    let key: String
    public init(defaults: UserDefaults = .standard, key: String = "autoradio.playbackSpeed") { self.defaults = defaults; self.key = key }
    public static let standard = SpeedStore()

    public func load() -> Double? {
        guard let v = defaults.object(forKey: key) as? Double, v.isFinite, v >= PlaybackSpeed.minimum, v <= PlaybackSpeed.maximum else { return nil }
        return PlaybackSpeed.snap(v)
    }
    public func save(_ v: Double) { defaults.set(PlaybackSpeed.snap(v), forKey: key) }
}

// MARK: The list a story came from

public enum StoryCursor {
    /// How long into a story "previous" still means the story before it; later it means the start of this one (the usual player rule).
    public static let restartAfterSeconds = 3.0

    /// "Previous" restarts the story being played unless it has barely begun: the first block, and under `restartAfterSeconds` in.
    public static func previousRestartsCurrent(blockIndex: Int, secondsIntoBlock: Double) -> Bool {
        blockIndex > 0 || secondsIntoBlock >= restartAfterSeconds
    }
}
