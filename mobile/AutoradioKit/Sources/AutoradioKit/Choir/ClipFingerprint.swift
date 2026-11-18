import Foundation
#if canImport(Accelerate)
import Accelerate
#endif

/// Podcast clip fingerprint, version 1: implemented to docs/reference/PODCAST-CLIP-FINGERPRINT.md and pinned by
/// mobile/fixtures/podcast-clip-fingerprint-v1.json (ClipFingerprintTests). 16 kHz mono PCM -> 10 frames a second of 8 log-energy bands, 8 bits
/// each; a clip carries the first and the last 12 s so the phone can find it again in a differently served file.
public enum ClipFingerprint {
    public static let version = 1
    public static let sampleRate = 16_000
    public static let frameLen = 1024
    public static let hop = 1600
    public static let fps = 10.0
    public static let bands = 8
    public static let bandBins = [6, 10, 16, 25, 39, 62, 98, 154, 243]
    /// The thresholds of a found clip (the spec's, measured) and the exclusion around a peak, in frames.
    public static let foundPeak = 0.80, foundMargin = 0.19, secondExclusion = 15
    /// Head and tail of a clip agree when their offsets differ by no more than this (seconds); more is an ad inside the clip.
    public static let agreeSeconds = 0.3

    private static let window: [Double] = (0..<frameLen).map { 0.5 - 0.5 * cos(2 * Double.pi * Double($0) / Double(frameLen)) }
    private static let twiddle: [(Double, Double)] = (0..<frameLen / 2).map { let a = -2 * Double.pi * Double($0) / Double(frameLen); return (cos(a), sin(a)) }
    private static let reverse: [Int] = {
        var r = [Int](repeating: 0, count: frameLen)
        var j = 0
        for i in 1..<frameLen {
            var bit = frameLen >> 1
            while j & bit != 0 { j ^= bit; bit >>= 1 }
            j ^= bit
            r[i] = j
        }
        return r
    }()

    // MARK: The one place the band edges and the quantisation live

    /// One frame's 8 bytes from its 1024 samples (s16). Band edges, window, scale and floor are the spec's (section 1).
    static func frameBytes(_ s: UnsafeBufferPointer<Int16>, at start: Int, into out: inout [UInt8]) {
        var re = [Double](repeating: 0, count: frameLen), im = [Double](repeating: 0, count: frameLen)
        for n in 0..<frameLen { re[reverse[n]] = Double(s[start + n]) / 32768.0 * window[n] }
        var size = 2
        while size <= frameLen {
            let half = size / 2, step = frameLen / size
            var st = 0
            while st < frameLen {
                for k in 0..<half {
                    let (wr, wi) = twiddle[k * step]
                    let a = st + k, b = st + k + half
                    let tr = re[b] * wr - im[b] * wi, ti = re[b] * wi + im[b] * wr
                    re[b] = re[a] - tr; im[b] = im[a] - ti
                    re[a] += tr; im[a] += ti
                }
                st += size
            }
            size <<= 1
        }
        for b in 0..<bands {
            var sum = 0.0
            for k in bandBins[b]..<bandBins[b + 1] { sum += re[k] * re[k] + im[k] * im[k] }
            let mean = sum / Double(bandBins[b + 1] - bandBins[b]) / Double(frameLen * frameLen)
            let db = 10 * log10(mean + 1e-10)
            let v = ((db + 100) * 2.55 + 0.5).rounded(.down)
            out.append(UInt8(max(0, min(255, v))))
        }
    }

    /// The frame track of 16 kHz mono PCM: frame k covers samples [1600k, 1600k + 1024), emitted only when complete. Frame-major, 8 bytes a frame.
    public static func track(_ pcm: [Int16]) -> [UInt8] {
        guard pcm.count >= frameLen else { return [] }
        let frames = (pcm.count - frameLen) / hop + 1
        var out = [UInt8](); out.reserveCapacity(frames * bands)
        pcm.withUnsafeBufferPointer { p in for k in 0..<frames { frameBytes(p, at: k * hop, into: &out) } }
        return out
    }
    /// The same from float samples in [-1, 1] (a decoder's output): rounded to s16 first, as the spec's input is.
    public static func track(floats: [Float]) -> [UInt8] {
        track(floats.map { Int16(max(-32768, min(32767, ($0 * 32768).rounded()))) })
    }

    // MARK: Packed form

    public static func pack(_ frames: [UInt8]) -> [UInt8]? {
        guard !frames.isEmpty, frames.count % bands == 0, frames.count / bands <= 0xFFFF else { return nil }
        let n = frames.count / bands
        return [UInt8(version), UInt8(bands), UInt8(n >> 8), UInt8(n & 0xFF)] + frames
    }
    public static func unpack(_ p: [UInt8]) -> [UInt8]? {
        guard p.count >= 4 + bands, Int(p[0]) == version, Int(p[1]) == bands else { return nil }
        let n = Int(p[2]) << 8 | Int(p[3])
        guard n > 0, p.count == 4 + n * bands else { return nil }
        return Array(p[4...])
    }

    // MARK: Locating

    public struct Match: Equatable, Sendable {
        public var pos: Int, peak: Double, second: Double
        public var margin: Double { peak - second }
        public var found: Bool { peak >= foundPeak && margin >= foundMargin }
        public var seconds: Double { Double(pos) / fps }
    }

    /// Normalised cross-correlation of `template` over every position of `window` (both frame tracks); nil when the window is shorter.
    public static func locate(template: [UInt8], in window: [UInt8]) -> Match? {
        let n = template.count / bands, m = window.count / bands
        guard n > 0, m >= n, template.count % bands == 0, window.count % bands == 0 else { return nil }
        let mt = template.reduce(0.0) { $0 + Double($1) } / Double(template.count)
        let tz = template.map { Double($0) - mt }
        let tt = tz.reduce(0.0) { $0 + $1 * $1 }
        let w = window.map { Double($0) }
        var P = [Double](repeating: 0, count: m + 1), Q = P
        for x in 0..<m {
            var s = 0.0, s2 = 0.0
            for b in 0..<bands { let v = w[x * bands + b]; s += v; s2 += v * v }
            P[x + 1] = P[x] + s; Q[x + 1] = Q[x] + s2
        }
        let N = Double(n * bands), count = m - n + 1
        var scores = [Double](repeating: 0, count: count)
        var best = 0
        w.withUnsafeBufferPointer { wp in
            tz.withUnsafeBufferPointer { tp in
                for p in 0..<count {
                    let sx = P[p + n] - P[p], sxx = Q[p + n] - Q[p]
                    var dot = 0.0
                    #if canImport(Accelerate)
                    vDSP_dotprD(tp.baseAddress!, 1, wp.baseAddress! + p * bands, 1, &dot, vDSP_Length(n * bands))
                    #else
                    for i in 0..<(n * bands) { dot += tp[i] * wp[p * bands + i] }
                    #endif
                    let variance = sxx - sx * sx / N
                    if variance > 1e-9, tt > 1e-9 { scores[p] = dot / (tt * variance).squareRoot() }
                    if scores[p] > scores[best] { best = p }
                }
            }
        }
        var second = 0.0
        for (p, s) in scores.enumerated() where p < best - secondExclusion || p > best + secondExclusion { second = max(second, s) }
        return Match(pos: best, peak: scores[best], second: second)
    }
}
