import XCTest
@testable import AutoradioKit

// Voice enrolment's native part (Voice/EnrolCapture.swift, TakeCollector.swift, TurnEngine.enrolmentTap, TurnGate). Failure list V1 to V6 is at the head of EnrolCapture.swift; the page's state machine is Exact (voicelearn.test.ts).
// The fake model reads (first sample - 0.3) as the speaker's number, as SpeakerGateTests does.
final class EnrolCaptureTests: XCTestCase {
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
    /// One sentence: about 2.2 s of speech then a pause.
    func say(_ c: EnrolCapture, _ speaker: Int) {
        c.begin()
        for _ in 0..<70 { c.feed(frame(speaker)) }
        for _ in 0..<30 { c.feed(frame(0, loud: false)) }
    }
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("enrol-\(UUID().uuidString)/speaker.json") }
    func make(url: URL? = nil, _ embed: VoiceEmbedder? = Fake()) -> (EnrolCapture, SpeakerGate) { let g = SpeakerGate(url: url, embedder: embed); return (EnrolCapture(gate: g), g) }

    func testV1V2CollectorEndsOnPauseOnTimeoutAndIgnoresShortSounds() {
        var c = TakeCollector(); for _ in 0..<70 { c.feed(frame(1)) }; XCTAssertNil(c.ended); for _ in 0..<30 { c.feed(frame(0, loud: false)) }
        XCTAssertEqual(c.ended, .heard); XCTAssertTrue(c.usable)
        var s = TakeCollector(); for _ in 0..<5 { s.feed(frame(1)) }; for _ in 0..<400 { s.feed(frame(0, loud: false)) }
        XCTAssertEqual(s.ended, .timedOut); XCTAssertFalse(s.usable, "a cough is not a sentence")
        var q = TakeCollector(); for _ in 0..<400 { q.feed(frame(0, loud: false)) }; XCTAssertFalse(q.usable); XCTAssertTrue(q.samples.isEmpty)
        var l = TakeCollector(); for _ in 0..<400 { l.feed(frame(1)) }; XCTAssertEqual(l.ended, .timedOut); XCTAssertTrue(l.usable)
        var m = TakeCollector(); for _ in 0..<40 { m.feed(frame(1)) }; for _ in 0..<30 { m.feed(frame(0, loud: false)) }
        XCTAssertNotNil(m.ended); XCTAssertFalse(m.usable, "1.3 s is under the 1.5 s the profile learns from: the take is asked again, not accepted and then ignored")
    }
    func testV5FramesBeforeBeginAndAfterTheEndAreNotKept() {
        let (c, _) = make()
        for _ in 0..<50 { c.feed(frame(1)) }; XCTAssertEqual(c.state, .idle); XCTAssertEqual(c.keep(), 0)
        say(c, 1); XCTAssertEqual(c.state, .done); XCTAssertEqual(c.keep(), 1)
        for _ in 0..<200 { c.feed(frame(2)) }; XCTAssertEqual(c.keep(), 2, "frames after the end do not extend the take; keep is the page's call")
        c.discard(); XCTAssertEqual(c.state, .idle); XCTAssertEqual(c.facts()["kept"] as? Int, 0)
    }
    func testV2ExpireEndsATakeNothingEnded() {
        let (c, _) = make(); c.begin(); c.expire(); XCTAssertEqual(c.state, .done); XCTAssertFalse(c.usable); XCTAssertEqual(c.keep(), 0)
    }
    func testV3TheProfileIsStoredOnlyWhenTheConfirmingSentenceMatched() async {
        let url = tmp(); let (c, g) = make(url: url)
        for _ in 1...3 { say(c, 1); c.keep() }
        let built = await c.build(); XCTAssertTrue(built); XCTAssertFalse(g.learned, "built in memory only"); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
        say(c, 6); let call = await c.check(); XCTAssertNotEqual(call, "match")
        c.discard(); XCTAssertFalse(g.learned); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path), "a mismatched confirmation leaves no profile file")
        XCTAssertFalse(c.store(), "nothing to store after a discard")
        for _ in 1...3 { say(c, 1); c.keep() }; _ = await c.build(); say(c, 1)
        let ok = await c.check(); XCTAssertEqual(ok, "match"); XCTAssertTrue(c.store())
        XCTAssertTrue(g.learned); XCTAssertTrue(FileManager.default.fileExists(atPath: url.path)); XCTAssertEqual(SpeakerGate(url: url, embedder: Fake()).learned, true, "it survives a relaunch")
    }
    func testV3AnAbandonedEnrolmentLeavesTheStoredVoiceAlone() async {
        let url = tmp(); let (c, g) = make(url: url)
        for _ in 1...3 { say(c, 1); c.keep() }; _ = await c.build(); say(c, 1); _ = await c.check(); XCTAssertTrue(c.store()); let before = try? Data(contentsOf: url)
        for _ in 1...3 { say(c, 5); c.keep() }; _ = await c.build(); c.discard()                          // learn again, then leave
        XCTAssertEqual(try? Data(contentsOf: url), before); XCTAssertTrue(g.learned)
    }
    func testV4DifferentVoicesDoNotMakeAProfile() async {
        let (c, g) = make()
        for k in [1, 4, 7] { say(c, k); c.keep() }
        let built = await c.build(); XCTAssertFalse(built); XCTAssertFalse(g.learned); XCTAssertEqual(c.facts()["built"] as? Bool, false)
    }
    func testV6NoModelCapturesNothing() {
        let (c, _) = make(nil); XCTAssertFalse(c.begin()); XCTAssertEqual(c.state, .error); c.feed(frame(1)); XCTAssertEqual(c.keep(), 0); XCTAssertEqual(c.facts()["ready"] as? Bool, false)
    }
}
