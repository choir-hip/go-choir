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
        for _ in 0..<200 { c.feed(frame(2)) }; XCTAssertEqual(c.keep(), 1, "frames after the end do not extend the take, and a take is kept once: a second keep adds nothing")
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
    // B6 (review round 2): every write of a voice profile carries the account generation and is refused if it changed.
    func testV7ASignOutBetweenTheMatchAndTheStoreWritesNothing() async {
        let url = tmp(); let (c, g) = make(url: url)
        for _ in 1...3 { say(c, 1); c.keep() }; _ = await c.build(); say(c, 1); let call = await c.check(); XCTAssertEqual(call, "match")
        _ = AccountWipe.bumpGeneration()                                   // the sign-out began
        XCTAssertFalse(c.store(), "the old account's voice is not stored under the next one")
        XCTAssertFalse(g.learned); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
    }
    func testV7ASignOutDuringTheBuildDropsTheCandidate() async {
        let (c, _) = make(); for _ in 1...3 { say(c, 1); c.keep() }
        let t = Task { await c.build() }
        _ = AccountWipe.bumpGeneration()
        _ = await t.value
        say(c, 1); _ = await c.check(); XCTAssertFalse(c.store())
    }
    func testV7TheGateRefusesAStaleWriteAndForgetAndAdoptKeepOneOrder() async {
        let url = tmp(); let g = SpeakerGate(url: url, embedder: Fake())
        var p = SpeakerProfile(); for i in 0..<3 { _ = p.learn([.wespeaker: { var v = [Float](repeating: 0, count: 256); v[1] = 1; return v }()], audioMs: 2000 + i, rule: SpeakerRule()) }
        let old = AccountWipe.generation
        _ = AccountWipe.bumpGeneration()
        XCTAssertFalse(g.adopt(p, generation: old), "a profile built before the wipe is refused"); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
        XCTAssertTrue(g.adopt(p, generation: AccountWipe.generation)); XCTAssertTrue(FileManager.default.fileExists(atPath: url.path))
        g.forget(); XCTAssertFalse(g.learned); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
    }
    func testV8AFileThatCannotBeWrittenIsNotStored() async {
        let (c, g) = make(url: URL(fileURLWithPath: "/dev/null/nope/speaker.json"))
        for _ in 1...3 { say(c, 1); c.keep() }; _ = await c.build(); say(c, 1); _ = await c.check()
        XCTAssertFalse(c.store(), "the page must not say stored when nothing was written"); XCTAssertFalse(g.learned)
    }
    // Round 3, D: turn adaptation captures the generation when the turn is COMMITTED; a learn that enters after the wipe with the old generation writes nothing.
    func testV9ALearnCarryingAnOldGenerationWritesNothingEvenWhenItEntersAfterTheWipe() async {
        let url = tmp(); let g = SpeakerGate(url: url, embedder: Fake())
        let old = AccountWipe.generation
        _ = AccountWipe.bumpGeneration(); g.forget()                         // sign-out: bump, then forget
        for i in 0..<3 { await g.learn([Float](repeating: 0.4, count: 32_000 + i * 160), generation: old) }
        XCTAssertFalse(g.learned); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path), "the old account's speech made no template for the next one")
        for i in 0..<3 { await g.learn([Float](repeating: 0.4, count: 32_000 + i * 160), generation: AccountWipe.generation) }
        XCTAssertTrue(g.learned, "the same speech under the current generation is learned")
    }
    // Round 5: a build that was running when the enrolment was discarded must not put its candidate back.
    func testV10ABuildThatFinishesAfterADiscardInstallsNothing() async {
        let (c, g) = make(); for _ in 1...3 { say(c, 1); c.keep() }
        let t = Task { await c.build() }
        c.discard()                                                         // the page was left while it built
        _ = await t.value
        say(c, 1); _ = await c.check(); XCTAssertFalse(c.store(), "no candidate survived the discard"); XCTAssertFalse(g.learned)
        XCTAssertEqual(c.facts()["built"] as? Bool, false)
    }
}
