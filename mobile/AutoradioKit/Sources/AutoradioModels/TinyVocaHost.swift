// Host steps between the Core ML models: ports of tiny_voca_ane.vocoder
// (durations, expand, vowel_mask, f0_from_pitch, pulse_train, noise).
// Same float32 math as numpy where the result is rounded (durations), float64
// for the pulse train (its phase sum runs over the whole utterance).

import Accelerate
import Foundation

/// Constants of tiny_voca.config.Config that the host steps need.
public enum Config {
    public static let sampleRate = 24000
    public static let hop = 256
    public static let channels = 192
    public static let maxTokens = 512
    public static let maxFrames = 3000
    public static let f0Min: Float = 50
    public static let f0Max: Float = 600
    public static let pitchReferenceHz: Float = 200
    public static let harmonicLimit = 0.47
    public static let speakers = ["F2", "M2"]
    public static let vocabSize = 76
    /// Phoneme tokens (`model.phone_tokens`): <unk> (4) and the phones 17...75 (front end version 3).
    /// swift/phonemize.py checks these tables against tiny_voca.text.
    public static let phoneTokens: [Bool] = (0..<vocabSize).map { $0 == 4 || (17...75).contains($0) }
    /// Vowel tokens (`model.vowel_tokens`): 43...75.
    public static let vowelTokens: [Bool] = (0..<vocabSize).map { (43...75).contains($0) }
}

public enum Host {
    /// Frames per token from the raw duration output [2][n] (`vocoder.durations`). Float32 like the model,
    /// so the rounding (half to even, as np.round) gives the same frames.
    public static func durations(raw0: [Float], raw1: [Float], tokens: [Int], speed: Float) -> [Int] {
        let top = Float(log1p(400.0))
        return (0..<tokens.count).map { i in
            let clipped = min(max(raw0[i], 0), top)
            let frames = max(Int((expm1(clipped) / speed).rounded(.toNearestOrEven)), 1)
            return Config.phoneTokens[tokens[i]] || raw1[i] > 0 ? frames : 0
        }
    }

    /// The token that owns each frame (`index` of `vocoder.expand`).
    public static func owners(_ durations: [Int]) -> [Int] {
        var index = [Int]()
        index.reserveCapacity(durations.reduce(0, +))
        for (token, count) in durations.enumerated() where count > 0 {
            index.append(contentsOf: repeatElement(token, count: count))
        }
        return index
    }

    /// Position features [3][T] of each frame (`vocoder.expand`): relative place in its token,
    /// its complement and log1p(token length) / 4, in float32.
    public static func positions(_ durations: [Int]) -> (relative: [Float], complement: [Float], length: [Float]) {
        var relative = [Float](), complement = [Float](), length = [Float]()
        for count in durations where count > 0 {
            let owned = Float(count)
            let feature = log1p(owned) / 4
            for offset in 0..<count {
                let r = (Float(offset) + 0.5) / owned
                relative.append(r)
                complement.append(1 - r)
                length.append(feature)
            }
        }
        return (relative, complement, length)
    }

    /// F0 in Hz from the decoder's log-F0 output (`vocoder.f0_from_pitch`).
    public static func f0(pitch: [Float], shift: Float) -> [Float] {
        pitch.map { min(max(Config.pitchReferenceHz * exp($0 + shift), Config.f0Min), Config.f0Max) }
    }

    static let chunk = 1 << 14

    /// Band-limited pulse train [T * hop] for f0 [T] in Hz (`vocoder.pulse_train`, `model.pulse_train`
    /// without a start phase), in float64. The phase sum runs in one piece; the rest in parallel chunks
    /// with vForce.
    public static func pulseTrain(f0: [Float], sampleRate: Int = Config.sampleRate, hop: Int = Config.hop,
                                  limit: Double = Config.harmonicLimit) -> [Float] {
        let frames = f0.count, samples = frames * hop
        guard frames > 0 else { return [] }
        var knots = f0.map { log(max(Double($0), 1.0)) }
        knots.append(knots[frames - 1])
        var frequency = [Double](repeating: 0, count: samples)
        var output = [Float](repeating: 0, count: samples)
        let chunks = (samples + chunk - 1) / chunk
        let rate = Double(sampleRate), numerator = limit * rate, twoPi = 2 * Double.pi
        knots.withUnsafeBufferPointer { knots in
            frequency.withUnsafeMutableBufferPointer { frequency in
                let k = Shared(UnsafeMutablePointer(mutating: knots.baseAddress!)), f = Shared(frequency.baseAddress!)
                DispatchQueue.concurrentPerform(iterations: chunks) { c in
                    let start = c * chunk, count = min(chunk, samples - start)
                    for s in start..<(start + count) {
                        let position = Double(s) / Double(hop)
                        let low = position.rounded(.down)
                        let weight = position - low
                        let i = Int(low)
                        f.pointer[s] = k.pointer[i] * (1 - weight) + k.pointer[i + 1] * weight
                    }
                    var n = Int32(count)
                    vvexp(f.pointer + start, f.pointer + start, &n)
                }
            }
        }
        // theta = 2 pi * cumsum(frequency / rate), summed in order like np.cumsum.
        var theta = [Double](repeating: 0, count: samples)
        var running = 0.0
        for s in 0..<samples {
            running += frequency[s] / rate
            theta[s] = twoPi * running
        }
        frequency.withUnsafeBufferPointer { frequency in
            theta.withUnsafeMutableBufferPointer { theta in
                output.withUnsafeMutableBufferPointer { output in
                    let f = Shared(UnsafeMutablePointer(mutating: frequency.baseAddress!)), t = Shared(theta.baseAddress!)
                    let o = Shared(output.baseAddress!)
                    DispatchQueue.concurrentPerform(iterations: chunks) { c in
                        let start = c * chunk, count = min(chunk, samples - start)
                        var n = Int32(count)
                        var harmonics = [Double](repeating: 0, count: count), whole = harmonics, half = harmonics
                        var core = harmonics, top = harmonics, root = harmonics
                        let angle = t.pointer + start
                        for j in 0..<count {
                            angle[j] = fmod(angle[j], twoPi)          // np.remainder for positive values
                            harmonics[j] = max(numerator / f.pointer[start + j], 1.0)
                            whole[j] = harmonics[j].rounded(.down)
                            half[j] = angle[j] / 2
                            core[j] = (whole[j] + 0.5) * angle[j]
                            top[j] = (whole[j] + 1) * angle[j]
                            root[j] = 2.0 / harmonics[j]
                        }
                        vvsin(&half, half, &n)
                        vvsin(&core, core, &n)
                        vvcos(&top, top, &n)
                        vvsqrt(&root, root, &n)
                        for j in 0..<count {
                            let denominator = 2 * half[j]
                            let tiny = abs(denominator) < 1e-7
                            let body = tiny ? whole[j] : core[j] / denominator - 0.5
                            let wave = body + (harmonics[j] - whole[j]) * top[j]
                            o.pointer[start + j] = Float(wave * root[j])
                        }
                    }
                }
            }
        }
        return output
    }

    /// Seeded Gaussian noise [samples] (xoshiro256** + Box-Muller). Each chunk of 65536 samples has its
    /// own stream from (seed, chunk), so the result does not depend on the threads. Not numpy's or
    /// torch's numbers for the same seed.
    public static func noise(samples: Int, seed: UInt64) -> [Float] {
        var output = [Float](repeating: 0, count: samples)
        let size = 1 << 16
        let chunks = (samples + size - 1) / size
        output.withUnsafeMutableBufferPointer { output in
            let o = Shared(output.baseAddress!)
            DispatchQueue.concurrentPerform(iterations: chunks) { c in
                let start = c * size, count = min(size, samples - start), pairs = (count + 1) / 2
                var generator = Xoshiro(seed: seed &* 0x9E37_79B9_7F4A_7C15 &+ UInt64(c))
                var radius = [Float](repeating: 0, count: pairs), angle = radius
                for j in 0..<pairs {
                    radius[j] = (Float(generator.next() >> 40) + 1) * 0x1p-24           // (0, 1]
                    angle[j] = Float(generator.next() >> 40) * 0x1p-24 * 2 * Float.pi   // [0, 2 pi)
                }
                var n = Int32(pairs)
                vvlogf(&radius, radius, &n)
                var minusTwo: Float = -2
                vDSP_vsmul(radius, 1, &minusTwo, &radius, 1, vDSP_Length(pairs))
                vvsqrtf(&radius, radius, &n)
                var sine = [Float](repeating: 0, count: pairs), cosine = sine
                vvsincosf(&sine, &cosine, angle, &n)
                let out = o.pointer + start
                for j in 0..<pairs {
                    out[2 * j] = radius[j] * cosine[j]
                    if 2 * j + 1 < count { out[2 * j + 1] = radius[j] * sine[j] }
                }
            }
        }
        return output
    }

    /// `limit_peak` of tiny_voca.infer: the whole wave scaled down when its loudest sample is above `peak`.
    public static func limitPeak(_ wave: [Float], peak: Float = 0.99) -> [Float] {
        guard !wave.isEmpty else { return wave }
        var loudest: Float = 0
        vDSP_maxmgv(wave, 1, &loudest, vDSP_Length(wave.count))
        return loudest > peak ? vDSP.multiply(peak / loudest, wave) : wave
    }
}

/// A pointer that the chunks of concurrentPerform write to (disjoint ranges only).
struct Shared<T>: @unchecked Sendable {
    let pointer: UnsafeMutablePointer<T>
    init(_ pointer: UnsafeMutablePointer<T>) { self.pointer = pointer }
}

/// xoshiro256** seeded with splitmix64.
struct Xoshiro {
    var s: (UInt64, UInt64, UInt64, UInt64)

    init(seed: UInt64) {
        var x = seed
        func splitmix() -> UInt64 {
            x &+= 0x9E37_79B9_7F4A_7C15
            var z = x
            z = (z ^ (z >> 30)) &* 0xBF58_476D_1CE4_E5B9
            z = (z ^ (z >> 27)) &* 0x94D0_49BB_1331_11EB
            return z ^ (z >> 31)
        }
        s = (splitmix(), splitmix(), splitmix(), splitmix())
    }

    mutating func next() -> UInt64 {
        let r = s.1 &* 5
        let result = (r << 7 | r >> 57) &* 9
        let t = s.1 << 17
        s.2 ^= s.0
        s.3 ^= s.1
        s.1 ^= s.2
        s.0 ^= s.3
        s.2 ^= t
        s.3 = s.3 << 45 | s.3 >> 19
        return result
    }
}
