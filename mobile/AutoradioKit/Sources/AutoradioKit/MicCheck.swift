import Foundation

/// The first-run microphone check: what counts as "heard", the timeout, and the level the meter shows. Pure, so it is tested without
/// audio. The check passes on the first transcript OR on a sustained input level above the noise floor (a tester may say nothing the
/// recogniser knows). No phrase is matched.
public struct MicCheck: Sendable {
    /// Seconds before the check gives up.
    public static let giveUpSeconds: Double = 20
    /// Frame level (dBFS RMS) above which a frame counts as sound. Room tone on a phone is around -55 to -50.
    public static let floorDb: Double = -42
    /// Seconds of sound above the floor, summed, before the check passes.
    public static let sustainSeconds: Double = 0.6
    /// The meter spans this range of dBFS.
    public static let meterLowDb: Double = -60, meterHighDb: Double = -20

    public enum Verdict: Equatable, Sendable { case listening, heard, silent }

    public private(set) var level: Double = 0
    public private(set) var elapsed: Double = 0
    private var loud: Double = 0
    private var transcriptHeard = false

    public init() {}

    /// One mic frame: `db` its RMS level, `seconds` its length.
    public mutating func feed(db: Double, seconds: Double) {
        let target = min(1, max(0, (db - Self.meterLowDb) / (Self.meterHighDb - Self.meterLowDb)))
        // Quick to rise, slow to fall, so the bar is readable.
        level = target > level ? level + (target - level) * 0.6 : level + (target - level) * 0.15
        if db >= Self.floorDb { loud += seconds } else { loud = max(0, loud - seconds * 0.5) }   // a pause does not erase the speech before it at once
    }

    /// The engine's transcript so far (any non-empty text counts).
    public mutating func hear(transcript: String) {
        if !transcript.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { transcriptHeard = true }
    }

    /// Wall time passing (the polling loop's clock; frames do not advance it, so a silent microphone still times out).
    public mutating func tick(seconds: Double) { elapsed += seconds }

    public var verdict: Verdict {
        if transcriptHeard || loud >= Self.sustainSeconds { return .heard }
        return elapsed >= Self.giveUpSeconds ? .silent : .listening
    }
}
