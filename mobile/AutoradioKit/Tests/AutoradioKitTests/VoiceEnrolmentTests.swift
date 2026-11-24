import XCTest
@testable import AutoradioKit

// Voice enrolment (Voice/VoiceEnrolment.swift). Failure list V1 to V6 is at its head. The fake model reads (first sample - 0.3) as the speaker's number, as SpeakerGateTests does.
final class VoiceEnrolmentTests: XCTestCase {
    struct Fake: VoiceEmbedder {
        var isReady = true
        func embed(_ s: [Float]) async -> [Float]? {
            guard let f = s.first else { return nil }
            let k = Int(((f - 0.3) * 10).rounded())
            var v = [Float](repeating: 0, count: 256); v[min(max(k, 0), 255)] = 1; return v
        }
    }
    var t = 0.0
    func frame(_ speaker: Int, loud: Bool = true) -> AudioFrame {
        defer { t += 0.032 }
        return AudioFrame(samples: [Float](repeating: loud ? 0.3 + Float(speaker) * 0.1 : 0, count: 512), sampleRate: 16000, t: t)
    }
    /// One sentence: about 1.6 s of speech then silence.
    func say(_ e: VoiceEnrolment, _ speaker: Int) async {
        for _ in 0..<50 { e.feed(frame(speaker)) }
        for _ in 0..<30 { e.feed(frame(0, loud: false)) }
        for _ in 0..<40 { if [.listening, .confirming].contains(e.phase) || e.phase == .building { try? await Task.sleep(nanoseconds: 20_000_000) } }
    }
    func make(_ embed: VoiceEmbedder? = Fake()) -> (VoiceEnrolment, SpeakerGate) { let g = SpeakerGate(url: nil, embedder: embed); return (VoiceEnrolment(gate: g), g) }

    func testCollectorEndsOnPauseAndOnTimeoutAndIgnoresShortSounds() {
        var c = TakeCollector(); for _ in 0..<50 { c.feed(frame(1)) }; XCTAssertNil(c.ended); for _ in 0..<30 { c.feed(frame(0, loud: false)) }
        XCTAssertEqual(c.ended, .heard); XCTAssertTrue(c.usable)
        var s = TakeCollector(); for _ in 0..<5 { s.feed(frame(1)) }; for _ in 0..<400 { s.feed(frame(0, loud: false)) }
        XCTAssertEqual(s.ended, .timedOut); XCTAssertFalse(s.usable, "V1: a cough is not a sentence")                                  // V1, V2
        var q = TakeCollector(); for _ in 0..<400 { q.feed(frame(0, loud: false)) }; XCTAssertFalse(q.usable); XCTAssertTrue(q.samples.isEmpty)
        var l = TakeCollector(); for _ in 0..<400 { l.feed(frame(1)) }; XCTAssertEqual(l.ended, .timedOut); XCTAssertTrue(l.usable)    // V2
    }
    func testThreeSentencesBuildTheProfileAndAFourthConfirmsIt() async {
        let (e, g) = make(); e.start(); XCTAssertFalse(g.learned)
        for i in 1...3 { e.begin(); XCTAssertEqual(e.phase, .listening); await say(e, 1); if i < 3 { XCTAssertEqual(e.facts()["taken"] as? Int, i) } }
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertTrue(g.learned); XCTAssertEqual(e.phase, .confirm)
        e.begin(); XCTAssertEqual(e.phase, .confirming); await say(e, 1)
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertEqual(e.phase, .stored); XCTAssertEqual(e.facts()["call"] as? String, "match")
    }
    func testV1AnUnheardSentenceIsAskedAgainAndKeepsNothing() async {
        let (e, g) = make(); e.start(); e.begin()
        for _ in 0..<400 { e.feed(frame(0, loud: false)) }
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertEqual(e.phase, .idle); XCTAssertEqual(e.facts()["again"] as? Bool, true); XCTAssertEqual(e.facts()["taken"] as? Int, 0); XCTAssertFalse(g.learned)
    }
    func testV3MixedVoicesAreNeverStored() async {
        let (e, g) = make(); e.start()
        for k in [1, 4, 7] { e.begin(); await say(e, k) }
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertFalse(g.learned); XCTAssertEqual(e.phase, .failed); XCTAssertEqual(e.facts()["taken"] as? Int, 0)
    }
    func testV4AnotherVoiceConfirmsNothingAndChangesNothing() async {
        let (e, g) = make(); e.start()
        for _ in 1...3 { e.begin(); await say(e, 1) }
        try? await Task.sleep(nanoseconds: 100_000_000)
        e.begin(); await say(e, 6); try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertEqual(e.phase, .unsure); XCTAssertTrue(g.learned, "judging does not change the profile")
    }
    func testV5LearnAgainDeletesTheOldProfileFirstAndForgetDoes() async {
        let (e, g) = make(); e.start()
        for _ in 1...3 { e.begin(); await say(e, 1) }
        try? await Task.sleep(nanoseconds: 100_000_000); XCTAssertTrue(g.learned)
        e.start(); XCTAssertFalse(g.learned); XCTAssertEqual(e.phase, .idle)
        for _ in 1...3 { e.begin(); await say(e, 1) }
        try? await Task.sleep(nanoseconds: 100_000_000); XCTAssertTrue(g.learned)
        e.forget(); XCTAssertFalse(g.learned); XCTAssertEqual(e.facts()["taken"] as? Int, 0)
    }
    func testV6NoModelTakesNoAudio() {
        let (e, g) = make(nil); e.start(); XCTAssertEqual(e.phase, .noModel); e.begin(); e.feed(frame(1)); XCTAssertEqual(e.phase, .noModel); XCTAssertFalse(g.learned)
    }
}
