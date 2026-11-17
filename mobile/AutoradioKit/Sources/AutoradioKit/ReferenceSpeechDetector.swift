import Foundation

/// Hears the listener speaking over the radio without echo cancellation: the microphone is compared, band by band, with what the engine is
/// itself playing (the reference tap on the mixer), so the radio's own voice from the speaker does not count as the listener's.
///
/// Four speech bands (about 350, 700, 1400 and 2800 Hz) at 20 ms hops. For each band the mic energy is compared with
///     margin * (echo gain * the loudest reference energy of the last 200 ms + the band's noise floor).
/// The echo gain is the upper envelope of mic over reference while no speech is detected: it rises slowly (at most 1 % a hop) and falls slowly
/// (0.2 % a hop), so a talker cannot teach it that they are echo within the time it takes to detect them. It starts high (an echo of -6 dB), so
/// a cold start errs towards not pausing. Speech is two bands over their threshold for four hops (80 ms). The decision is not a model of speech;
/// it is "something in the room is louder than the radio's own sound can explain".
///
/// What it misses (accepted): a talker quieter than the radio's own echo in the bands that matter (about 3 dB under it, measured in the tests).
/// In that case the listener's words are still heard by the turn policy, which pauses when the words arrive.
public struct ReferenceSpeechDetector: Sendable {
    public struct Event: Sendable, Equatable {
        /// The largest per-band ratio of mic energy to what the radio's own sound explains.
        public var ratio: Double
        /// How long the speech had been going on when it was decided.
        public var ms: Int
    }

    public static let hop = 320                       // 20 ms at 16 kHz
    static let bandCentres: [Double] = [350, 700, 1400, 2800]
    static let margin = 3.0                           // 4.8 dB over what the radio explains
    static let bandsNeeded = 2
    static let hopsNeeded = 4
    static let quietHopsToRearm = 25
    static let floor = 1e-8

    private struct Biquad: Sendable {
        var b0 = 0.0, b1 = 0.0, b2 = 0.0, a1 = 0.0, a2 = 0.0, z1 = 0.0, z2 = 0.0
        init(centre: Double, rate: Double, q: Double = 1.4) {
            let w = 2 * Double.pi * centre / rate, alpha = sin(w) / (2 * q), a0 = 1 + alpha
            b0 = alpha / a0; b1 = 0; b2 = -alpha / a0; a1 = -2 * cos(w) / a0; a2 = (1 - alpha) / a0
        }
        mutating func run(_ x: Double) -> Double {
            let y = b0 * x + z1
            z1 = b1 * x - a1 * y + z2
            z2 = b2 * x - a2 * y
            return y
        }
    }
    private struct Bank: Sendable {
        var filters = ReferenceSpeechDetector.bandCentres.map { Biquad(centre: $0, rate: busSampleRate) }
        var pending: [Float] = []
        /// Feeds samples; returns the band energies of each completed hop.
        mutating func hops(_ samples: [Float]) -> [[Double]] {
            pending += samples
            var out: [[Double]] = []
            while pending.count >= ReferenceSpeechDetector.hop {
                var e = [Double](repeating: 0, count: filters.count)
                for s in pending.prefix(ReferenceSpeechDetector.hop) {
                    for b in filters.indices { let y = filters[b].run(Double(s)); e[b] += y * y }
                }
                out.append(e.map { $0 / Double(ReferenceSpeechDetector.hop) })
                pending.removeFirst(ReferenceSpeechDetector.hop)
            }
            return out
        }
    }

    private var micBank = Bank(), refBank = Bank()
    private var refHistory: [[Double]] = []            // the last 10 hops of the reference's band energies
    private var gain = [Double](repeating: 0.25, count: ReferenceSpeechDetector.bandCentres.count)       // echo -6 dB to start
    private var noise = [Double](repeating: 1e-6, count: ReferenceSpeechDetector.bandCentres.count)
    private var run = 0, quiet = 0
    private var fired = false
    public init() {}

    public mutating func feedReference(_ samples: [Float]) {
        for e in refBank.hops(samples) { refHistory.append(e); if refHistory.count > 10 { refHistory.removeFirst() } }
    }

    /// Feeds microphone samples; the event comes at the hop where the speech was decided (once per stretch of speech).
    public mutating func feedMic(_ samples: [Float]) -> Event? {
        var event: Event?
        for m in micBank.hops(samples) {
            let n = m.count
            var over = 0
            var worst = 0.0
            var explained = [Double](repeating: 0, count: n)
            for b in 0..<n {
                let rmax = refHistory.map { $0[b] }.max() ?? 0
                explained[b] = gain[b] * rmax + noise[b]
                let ratio = m[b] / max(explained[b], Self.floor)
                if m[b] > Self.floor, ratio > Self.margin { over += 1 }
                worst = max(worst, ratio)
            }
            let speech = over >= Self.bandsNeeded
            if speech {
                run += 1; quiet = 0
                if run >= Self.hopsNeeded, !fired { fired = true; event = Event(ratio: worst, ms: run * 20) }
            } else {
                quiet += 1
                if quiet >= Self.quietHopsToRearm { run = 0; fired = false }
                else if !fired { run = 0 }
                // What the radio's own sound explains is learned only while nothing is being said.
                if !fired {
                    for b in 0..<n {
                        let rmax = refHistory.map { $0[b] }.max() ?? 0
                        if rmax > noise[b] * 20 {                      // only where the radio's sound is well above the room's noise
                            let ratio = max(m[b] - noise[b], 0) / rmax
                            gain[b] = max(gain[b] * 0.998, min(ratio, gain[b] * 1.01))
                        }
                        noise[b] = min(noise[b] * 1.001, max(m[b], 1e-9))
                    }
                }
            }
        }
        return event
    }

    var debugGain: [Double] { gain }
    var debugNoise: [Double] { noise }
}
