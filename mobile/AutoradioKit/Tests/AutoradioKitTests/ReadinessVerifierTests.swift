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
    func testW3APassMarksTheModelAndNothingElse() async {
        let v = ReadinessVerifier(record: MemoryVoiceReadiness())
        XCTAssertFalse(v.reading.isReady(model: "m")); XCTAssertFalse(v.reading.isReady(model: "live"))
        _ = await v.verify(model: "m", check: { ok(true) })
        XCTAssertTrue(v.reading.isReady(model: "m")); XCTAssertFalse(v.reading.isReady(model: "live"), "a companion is never marked on the primary's pass")
    }
    // The escape of round 3 (the voice cards fetched and marked ready with no check) is closed by there being no path left: a card's tap fetches nothing and writes no readiness; only a row's download and its check do.
    func testW4ATapOnAVoiceCardNeverMarksAModelReady() async {
        let h = VoiceSamplerTests.Host()
        let t = VoiceSamplerTests(); let (s, r) = t.make(h)
        s.pick("pocket-tts:alba"); await t.settle(100)
        XCTAssertFalse(r.isReady(model: VoiceCatalog.pocketModel), "a tap is not a check")
        XCTAssertTrue(h.said.isEmpty, "and no sample is played from a model that is not on the phone")
    }

    // Round 4: readiness is DERIVED. Ready iff the model's files are present AND a check passed for that exact pin; a changed pin or a missing file makes it not ready with no writer involved.
    final class Pins: @unchecked Sendable {
        let lock = NSLock(); var pin = "rev-1"; var present = true
        func info(_ id: String) -> ModelPin? { lock.lock(); defer { lock.unlock() }; let p = pin, pr = present; return id == "unpinned" ? nil : ModelPin(pin: p, present: { pr }) }
        func set(pin: String? = nil, present: Bool? = nil) { lock.lock(); if let pin { self.pin = pin }; if let present { self.present = present }; lock.unlock() }
    }
    func testR4APinChangeMakesAPassedModelNotReadyWithNoWriterInvolved() {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("pin-\(UUID().uuidString)/ready.json"); let p = Pins()
        let r = FileVoiceReadiness(file: f, pins: p.info)
        r.markReady(model: "m"); XCTAssertTrue(r.isReady(model: "m"))
        p.set(pin: "rev-2"); XCTAssertFalse(r.isReady(model: "m"), "an app update that moved the pin: the check that passed was for another file set")
        XCTAssertFalse(FileVoiceReadiness(file: f, pins: p.info).isReady(model: "m"), "and a new process agrees")
        p.set(pin: "rev-1"); XCTAssertTrue(r.isReady(model: "m"), "the same pin again, the same pass")
    }
    func testR5AMissingFileMakesAPassedModelNotReady() {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("pin-\(UUID().uuidString)/ready.json"); let p = Pins()
        let r = FileVoiceReadiness(file: f, pins: p.info); r.markReady(model: "m")
        p.set(present: false); XCTAssertFalse(r.isReady(model: "m"), "voice-ready.json intact but a file gone: not ready, so nothing initialises a library that would then fetch")
    }
    func testR6AModelWithNoPinCannotBeReady() {
        let r = FileVoiceReadiness(file: FileManager.default.temporaryDirectory.appendingPathComponent("pin-\(UUID().uuidString)/ready.json"), pins: Pins().info)
        r.markReady(model: "unpinned"); XCTAssertFalse(r.isReady(model: "unpinned"))
    }
    func testR7TheRealPinsOfTheModelsTheAppOffers() {
        for id in ["fluid-speech-mini", "lfm3-1.7b-q5_k_m", KnownModels.embeddingGemma2.id, "kokoro-82m-ane", "parakeet-tdt-0.6b-v2"] {
            XCTAssertNotNil(ModelPins.current(id), "\(id) has a pin")
        }
        let empty = FileManager.default.temporaryDirectory.appendingPathComponent("pins-\(UUID().uuidString)")
        for id in ["fluid-speech-mini", "lfm3-1.7b-q5_k_m", KnownModels.embeddingGemma2.id] { XCTAssertFalse(ModelPins.current(id, root: empty)!.present(), "\(id): an empty store has none of its files") }
        XCTAssertNil(ModelPins.current("something-nobody-offers"))
    }
    func testR8ACompanionIsCheckedItselfNotMarkedOnThePrimarysPass() async {
        struct Backend: SetupBackend {
            func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {}
            func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 1, speed: 0.1, unit: "x", memoryMB: 0, outputOK: c.id != "parakeet-eou-120m", note: "") }
        }
        let rec = MemoryVoiceReadiness()
        let s = VoiceSetup(backend: Backend(), profile: DeviceProfile(chip: "t", memoryGB: 5.6, freeDiskGB: 40, onWiFi: true), stallSeconds: 5, patience: 0.05, maxTries: 1, verifier: ReadinessVerifier(record: rec))   // a 6 GB phone: Mini is first
        await s.run(only: [.transcriber])
        XCTAssertTrue(rec.isReady(model: "fluid-speech-mini"))
        XCTAssertFalse(rec.isReady(model: "parakeet-eou-120m"), "its own check failed: it is not marked on the primary's pass")
    }
}
