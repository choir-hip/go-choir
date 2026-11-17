import XCTest
@testable import AutoradioKit

/// Synthetic signals at 16 kHz. "Speech" is a harmonic series with a falling spectrum and a syllable-rate envelope; the radio's echo is the
/// reference, delayed 40 ms, low-passed, at a gain; the room has a little noise.
final class ReferenceSpeechDetectorTests: XCTestCase {
    let rate = 16_000.0
    struct Rng { var s: UInt64 = 12345; mutating func next() -> Float { s = s &* 6364136223846793005 &+ 1442695040888963407; return Float((s >> 33) % 2000) / 1000 - 1 } }

    func speech(f0: Double, seconds: Double, level: Float, from: Double = 0, phase: Double = 0) -> [Float] {
        let n = Int(seconds * rate)
        return (0..<n).map { i in
            let t = Double(i) / rate + from + phase
            let env = pow(0.5 + 0.5 * sin(2 * .pi * 3.1 * t), 2) * 0.9 + 0.1
            var v = 0.0
            for h in 1...18 { v += sin(2 * .pi * f0 * Double(h) * t) / Double(h) }
            return Float(v * env) * level
        }
    }
    func echo(of ref: [Float], gain: Float, delay: Int = 640) -> [Float] {
        var out = [Float](repeating: 0, count: ref.count); var lp: Float = 0
        for i in delay..<ref.count { lp = 0.5 * lp + 0.5 * ref[i - delay]; out[i] = lp * gain }
        return out
    }
    func mix(_ parts: [[Float]], noise: Float = 0.0003) -> [Float] {
        var r = Rng(); let n = parts.map(\.count).max() ?? 0
        return (0..<n).map { i in parts.reduce(Float(0)) { $0 + (i < $1.count ? $1[i] : 0) } + r.next() * noise }
    }
    /// Feeds in 20 ms hops; returns the time (ms) of the first event at or after `after`.
    func run(ref: [Float], mic: [Float], after: Double = 0) -> (events: [(ms: Double, e: ReferenceSpeechDetector.Event)], det: ReferenceSpeechDetector) {
        var d = ReferenceSpeechDetector(); var evs: [(Double, ReferenceSpeechDetector.Event)] = []
        let h = ReferenceSpeechDetector.hop
        var i = 0
        while i + h <= mic.count {
            d.feedReference(Array(ref[i..<min(i + h, ref.count)]))
            if let e = d.feedMic(Array(mic[i..<i + h])) { evs.append((Double(i + h) / rate * 1000, e)) }
            i += h
        }
        return (evs.map { (ms: $0.0, e: $0.1) }, d)
    }

    /// The radio's own voice from the speaker, 60 s, at speaker-to-mic levels from quiet to loud: it never pauses the radio.
    func testTheRadiosOwnVoiceNeverPausesItForSixtySeconds() {
        let ref = speech(f0: 180, seconds: 60, level: 0.15)
        for g: Float in [0.02, 0.05, 0.12, 0.25] {
            let r = run(ref: ref, mic: mix([echo(of: ref, gain: g)]))
            XCTAssertTrue(r.events.isEmpty, "echo at gain \(g) paused the radio at \(r.events.map(\.ms))")
        }
    }

    /// The radio plays and a person talks over it at conversational level (several times the echo): paused within 300 ms of their first word.
    func testATalkerOverTheRadioIsHeardWithin300ms() {
        let ref = speech(f0: 180, seconds: 30, level: 0.15)
        let onset = 20.0
        let talker = [Float](repeating: 0, count: Int(onset * rate)) + speech(f0: 115, seconds: 10, level: 0.12, from: 0, phase: 0.2)
        let r = run(ref: ref, mic: mix([echo(of: ref, gain: 0.06), talker]))
        let first = r.events.first?.ms
        print("DBG gain", r.det.debugGain, "noise", r.det.debugNoise)
        XCTAssertNotNil(first, "a talker over the radio was not heard")
        if let first { XCTAssertLessThan(first - onset * 1000, 300, "heard after \(first - onset * 1000) ms"); print("AUTORADIO test: talker over radio decided in \(first - onset * 1000) ms ratio=\(r.events.first!.e.ratio)") }
    }

    /// Nobody plays anything; a person talks: heard within 300 ms.
    func testATalkerAloneIsHeardWithin300ms() {
        let ref = [Float](repeating: 0, count: Int(10 * rate))
        let talker = [Float](repeating: 0, count: Int(5 * rate)) + speech(f0: 115, seconds: 4, level: 0.1)
        let r = run(ref: ref, mic: mix([talker]))
        let first = r.events.first?.ms
        XCTAssertNotNil(first)
        if let first { XCTAssertLessThan(first - 5000, 300) }
    }

    /// A loud programme and a quiet talker: the accepted miss. A talker 6 dB over the echo is heard; one 6 dB under it is not.
    func testAQuietTalkerUnderALoudProgrammeIsTheAcceptedMiss() {
        let ref = speech(f0: 180, seconds: 30, level: 0.2)
        let e = echo(of: ref, gain: 0.25)                       // a loud programme: about -12 dB at the mic
        func talker(_ level: Float) -> [Float] { [Float](repeating: 0, count: Int(20 * rate)) + speech(f0: 115, seconds: 10, level: level, phase: 0.2) }
        let loud = run(ref: ref, mic: mix([e, talker(0.2)]))
        XCTAssertNotNil(loud.events.first, "a talker clearly over the echo must be heard")
        let quiet = run(ref: ref, mic: mix([e, talker(0.015)]))
        XCTAssertTrue(quiet.events.isEmpty, "a talker well under the echo is the accepted miss (the turn policy hears their words instead)")
    }

    /// The same talker is found again after the radio went quiet and started again (the detector rearms).
    func testItRearmsForASecondInterruption() {
        let ref = speech(f0: 180, seconds: 40, level: 0.15)
        func at(_ s: Double) -> [Float] { [Float](repeating: 0, count: Int(s * rate)) + speech(f0: 115, seconds: 2, level: 0.12, phase: 0.2) }
        let r = run(ref: ref, mic: mix([echo(of: ref, gain: 0.05), at(15), at(30)]))
        XCTAssertEqual(r.events.count, 2)
    }
}
