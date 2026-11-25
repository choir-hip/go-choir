import XCTest
@testable import AutoradioKit

// A Download on a model's row only fetches and verifies; only Use changes what the radio uses (review 2026-11-24, seat 2 round 2). The first run keeps applying what it set up.
//  A1 a finished ROW download (`rowDownload`) leaves the running configuration, the listener choice and the saved choices exactly as they were.
//  A2 a finished first-run setup applies its choices (the stage it set up is the one configured).
//  A3 Use sets the stage the radio runs for a job (answering -> the holder, decision -> System 1) and keeps the choice; a model that is not on the phone changes nothing.
final class SetupApplyTests: XCTestCase {
    func dir() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("apply-\(UUID().uuidString)") }
    func tiny() -> DeviceProfile { DeviceProfile(chip: "test", memoryGB: 1, freeDiskGB: 40, onWiFi: true) }       // every ladder ends on its built-in floor
    /// The bundled voice is native (AutoradioModels); this stands in for it in a Kit test so the registry has the stage the setup chose.
    final class Bundled: Voice, @unchecked Sendable {
        static let stageID = VoiceCatalog.bundledStageID
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "Tiny Voca") }
        init(_ p: StageParams) {}
        func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { $0.finish() } }
    }
    func registry() -> StageRegistry {
        let r = StageRegistry.builtIn(); r.register(.voice, Bundled.self) { Bundled($0) }; return r
    }
    struct Backend: SetupBackend {
        func download(_ c: SetupCandidate, onBytes: @escaping @Sendable (Int64) -> Void) async throws {}
        func check(_ c: SetupCandidate) async throws -> SetupCheck { SetupCheck(loadMs: 1, speed: 0.1, unit: "x", memoryMB: 0, outputOK: true, note: "") }
    }
    func setup(row: Bool) async -> VoiceSetup {
        let s = VoiceSetup(backend: Backend(), profile: tiny(), stallSeconds: 5, patience: 0.05, maxTries: 1, rowDownload: row, verifier: ReadinessVerifier(record: MemoryVoiceReadiness()))
        await s.run(only: [.voice]); return s
    }
    func testA1AFinishedRowDownloadChangesNothingThatIsInUse() async {
        let rt = LabRuntime(supportDir: dir(), session: NullSessionOwner())
        rt.setStage(.voice, id: "pocket-tts")                                   // what the person uses
        let before = rt.configuration
        let s = await setup(row: true)
        XCTAssertFalse(s.choices.isEmpty, "setup did finish a stage")
        rt.applySetupChoices(s)
        XCTAssertEqual(rt.configuration, before, "a row's Download never changes the configuration, the listener or the voice")
    }
    func testA2AFinishedFirstRunSetupAppliesItsChoices() async {
        let rt = LabRuntime(supportDir: dir(), session: NullSessionOwner(), registry: registry())
        rt.setStage(.voice, id: "pocket-tts")
        let s = await setup(row: false)
        rt.applySetupChoices(s)
        XCTAssertEqual(rt.configuration.setting(.voice)?.id, s.choices[.voice], "the first run's voice is the configured one")
    }
    func testA3UseSetsTheStageAndKeepsTheChoiceOnlyForAModelOnThePhone() {
        let rt = LabRuntime(supportDir: dir(), session: NullSessionOwner())
        let d = UserDefaults(suiteName: "apply-\(UUID().uuidString)")!
        let ready = MemoryVoiceReadiness(); ready.markReady(model: "lfm3-1.7b-q5_k_m"); ready.markReady(model: "lfm2.5-350m-q4_k_m")
        let before = rt.configuration
        XCTAssertFalse(rt.useModel(job: "responder", id: "lfm3-1.7b-q5_k_m", live: false, readiness: MemoryVoiceReadiness(), defaults: d), "not on the phone: refused")
        XCTAssertEqual(rt.configuration, before); XCTAssertNil(d.string(forKey: ResponderChoice.key))
        XCTAssertTrue(rt.useModel(job: "responder", id: "lfm3-1.7b-q5_k_m", live: false, readiness: ready, defaults: d))
        XCTAssertEqual(d.string(forKey: ResponderChoice.key), "lfm3-1.7b-q5_k_m")
        XCTAssertEqual(rt.configuration.setting(.holder)?.id, SavedAnswerHolder.stageID, "the answering stage is the local one once a local model is the one in use")
        XCTAssertTrue(rt.useModel(job: "decision", id: "lfm2.5-350m-q4_k_m", live: false, readiness: ready, defaults: d))
        XCTAssertEqual(d.string(forKey: DecisionChoice.key), "lfm2.5-350m-q4_k_m"); XCTAssertEqual(rt.configuration.setting(.system1)?.id, "llama-qwen3-0.6b")
    }
}
