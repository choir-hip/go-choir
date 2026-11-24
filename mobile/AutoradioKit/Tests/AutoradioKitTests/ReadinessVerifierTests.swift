import XCTest
@testable import AutoradioKit

// DOOR B: ONE readiness writer (ReadinessVerifier). The escape each test closes is named.
final class ReadinessVerifierTests: XCTestCase {
    func ok(_ pass: Bool) -> SetupCheck { SetupCheck(loadMs: 1, speed: 0.1, unit: "x", memoryMB: 0, outputOK: pass, note: pass ? "" : "Too little memory would be left") }
    struct Boom: Error {}

    func testW1AFailedCheckLeavesTheModelNotReadyAndDemotesALeftover() async {
        let rec = MemoryVoiceReadiness(); rec.markReady(model: "m"); let v = ReadinessVerifier(record: rec)
        let r = await v.verify(model: "m", check: { ok(false) })
        XCTAssertFalse(r.outputOK); XCTAssertFalse(v.reading.isReady(model: "m"))
    }
    func testW2ACheckThatThrowsIsNotReady() async {
        let v = ReadinessVerifier(record: MemoryVoiceReadiness())
        let r = await v.verify(model: "m", check: { throw Boom() })
        XCTAssertFalse(r.outputOK); XCTAssertFalse(v.reading.isReady(model: "m"))
    }
    func testW3APassMarksTheModelAndItsCompanionsAndNothingBefore() async {
        let v = ReadinessVerifier(record: MemoryVoiceReadiness())
        XCTAssertFalse(v.reading.isReady(model: "m")); XCTAssertFalse(v.reading.isReady(model: "live"))
        _ = await v.verify(model: "m", companions: ["live"], check: { ok(true) })
        XCTAssertTrue(v.reading.isReady(model: "m")); XCTAssertTrue(v.reading.isReady(model: "live"))
    }
    // The escape of round 3: the voice cards fetched and marked ready with no check. A fetch that succeeds and a check that fails must leave the voice not ready, and the card says so.
    func testW4TheVoiceCardPathFetchesThenVerifiesAndAFailedVerifyIsNotReady() async {
        let h = VoiceSamplerTests.Host(); h.failVerify = true
        let t = VoiceSamplerTests(); let (s, r) = t.make(h)
        s.pick("pocket-tts:alba"); await t.settle(300)
        XCTAssertEqual(h.fetched, [VoiceCatalog.pocketModel], "it was fetched")
        XCTAssertFalse(r.isReady(model: VoiceCatalog.pocketModel), "but it never passed a check, so it is not ready")
        XCTAssertEqual(s.cards().first { $0.id == "pocket-tts:alba" }?.state, "failed"); XCTAssertTrue(h.said.isEmpty, "and no sample is played from it")
    }
}
