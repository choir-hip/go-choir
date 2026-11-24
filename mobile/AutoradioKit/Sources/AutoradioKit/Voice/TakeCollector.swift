import Foundation

/// Decides when one spoken sentence is over. Pure: no audio, no clock; the frames carry their own length. (Voice enrolment, EnrolCapture.)
public struct TakeCollector: Sendable {
    public static let minSpeechSeconds = 1.5, endSilenceSeconds = 0.7, maxSeconds = 8.0
    /// Frame level (dBFS RMS) above which a frame counts as speech (the first-run microphone check uses the same floor).
    public static let floorDb = MicCheck.floorDb

    public private(set) var samples: [Float] = []
    public private(set) var speech = 0.0, total = 0.0, silence = 0.0, level = 0.0
    public enum End: Equatable, Sendable { case heard, timedOut }
    public private(set) var ended: End?
    public init() {}

    public mutating func feed(_ f: AudioFrame) {
        guard ended == nil, f.sampleRate == busSampleRate else { return }
        let db = f.rmsDb, s = f.duration
        let target = min(1, max(0, (db - MicCheck.meterLowDb) / (MicCheck.meterHighDb - MicCheck.meterLowDb)))
        level = target > level ? level + (target - level) * 0.6 : level + (target - level) * 0.15
        total += s
        if db >= Self.floorDb { speech += s; silence = 0 } else if speech > 0 { silence += s }
        if speech > 0 { samples.append(contentsOf: f.samples) }                      // the lead-in silence is not kept
        if speech >= Self.minSpeechSeconds && silence >= Self.endSilenceSeconds { ended = .heard }
        else if total >= Self.maxSeconds { ended = .timedOut }
    }
    /// A take ended with enough speech to learn from: the profile ignores anything under `SpeakerProfile.minimumLearnMs`, so the page does not accept less.
    public var usable: Bool { ended != nil && samples.count >= SpeakerProfile.minimumLearnMs * 16 }
}
