import XCTest
@testable import AutoradioKit

// A download never changes what is in use (review 2026-11-24, seat 1): only Use does. Setup finishing may fill a choice that was never made, and nothing else.
//  H1 `adopt` writes the answering/decision choice only when the person has none, and only for a model that can be one.
//  H2 a decision that failed and fell back to a built-in id changes nothing.
//  H3 `ModelInUse.ids` protects the decision model the phone uses beside the answering one.
final class ModelChoiceTests: XCTestCase {
    func suite() -> UserDefaults { UserDefaults(suiteName: "choice-\(UUID().uuidString)")! }
    func testH1AChoiceThatWasMadeIsNotOverwrittenByADownload() {
        let d = suite()
        XCTAssertTrue(ResponderChoice.adopt("lfm3-1.7b-q5_k_m", defaults: d), "nothing chosen yet: the first model fetched becomes it")
        XCTAssertFalse(ResponderChoice.adopt("qwen3-1.7b-q6_k", defaults: d), "a choice was made: a download does not replace it")
        XCTAssertEqual(d.string(forKey: ResponderChoice.key), "lfm3-1.7b-q5_k_m")
        let e = suite()
        XCTAssertTrue(DecisionChoice.adopt("qwen3.5-0.8b-q8_0", defaults: e)); XCTAssertFalse(DecisionChoice.adopt("lfm2.5-350m-q4_k_m", defaults: e))
        XCTAssertEqual(e.string(forKey: DecisionChoice.key), "qwen3.5-0.8b-q8_0")
    }
    func testH2AFailedDecisionThatFellBackToABuiltInIdChangesNothing() {
        let e = suite()
        XCTAssertFalse(DecisionChoice.adopt("system1-computer", defaults: e)); XCTAssertNil(e.string(forKey: DecisionChoice.key))
        XCTAssertFalse(ResponderChoice.adopt("system1-computer", defaults: e)); XCTAssertNil(e.string(forKey: ResponderChoice.key))
    }
    func testH3TheDecisionModelThePhoneUsesIsProtectedBesideTheAnsweringOne() {
        let reg = StageRegistry(); let c = LabConfiguration(name: "t")
        let ids = ModelInUse.ids(config: c, registry: reg, responder: "qwen3-0.6b-q8_0", decision: "lfm3-1.7b-q5_k_m", readiness: MemoryVoiceReadiness())
        XCTAssertTrue(ids.isSuperset(of: ["qwen3-0.6b-q8_0", "lfm3-1.7b-q5_k_m"]), "\(ids)")
    }
    func testH4AllKokoroVoicePacksArePartOfItsPinSoAnUpgradedInstallFetchesThem() throws {
        let n = VoiceRows.all.filter { $0.modelID == "kokoro-82m-ane" }.count
        XCTAssertGreaterThan(n, 5)
        let pin = try XCTUnwrap(ModelPins.current("kokoro-82m-ane")).pin
        XCTAssertTrue(pin.contains("voices:\(n)"), "the pin must change when the voices the model must hold change: \(pin)")
        XCTAssertFalse(try XCTUnwrap(ModelPins.current("silero-vad-v6.2.1")).pin.contains("voices"))
    }
    func testH5ChoosingAVoiceElsewhereRefreshesTheCards() {
        let r = MemoryReadiness([VoiceCatalog.kokoroModel]); var changed = 0
        final class Count: @unchecked Sendable { var n = 0 }
        let k = Count()
        let s = VoiceSampler(selected: "kokoro:af_heart", readiness: r, say: { _, _ in }, halt: {}, choose: { _ in })
        s.onChange = { k.n += 1 }
        s.syncSelected("kokoro:am_adam"); changed = k.n
        XCTAssertEqual(changed, 1, "the cards are asked again when the choice moved"); s.syncSelected("kokoro:am_adam"); XCTAssertEqual(k.n, 1, "the same choice says nothing")
    }
    // Review round 5: the decision stage's descriptor always lists Qwen3 as its need; once another decision model is the choice, Qwen3 is not in use (unless it is the answering model) and can be removed.
    func testH6TheDecisionStagesFixedNeedDoesNotKeepQwenWhenAnotherDecisionModelIsChosen() {
        let reg = StageRegistry()
        reg.register(.system1, id: "llama-qwen3-0.6b", descriptor: StageDescriptor(displayName: "d", needsModels: ["qwen3-0.6b-q8_0"])) { _ in EnergyVAD(StageParams()) }
        var c = LabConfiguration(name: "t"); c.set(.system1, StageSetting(id: "llama-qwen3-0.6b"))
        let none = MemoryVoiceReadiness()
        var ids = ModelInUse.ids(config: c, registry: reg, responder: "lfm3-1.7b-q5_k_m", decision: "lfm2.5-350m-q4_k_m", readiness: none, pins: { _ in nil })
        XCTAssertFalse(ids.contains("qwen3-0.6b-q8_0"), "the choice is lfm2.5 for deciding and lfm3 for answering: Qwen3 is unused \(ids)")
        XCTAssertTrue(ids.isSuperset(of: ["lfm3-1.7b-q5_k_m", "lfm2.5-350m-q4_k_m"]))
        ids = ModelInUse.ids(config: c, registry: reg, responder: "qwen3-0.6b-q8_0", decision: "lfm2.5-350m-q4_k_m", readiness: none, pins: { _ in nil })
        XCTAssertTrue(ids.contains("qwen3-0.6b-q8_0"), "it is the answering model")
        ids = ModelInUse.ids(config: c, registry: reg, responder: "lfm3-1.7b-q5_k_m", decision: "qwen3-0.6b-q8_0", readiness: none, pins: { _ in nil })
        XCTAssertTrue(ids.contains("qwen3-0.6b-q8_0"), "it is the decision model")
        ids = ModelInUse.ids(config: c, registry: reg, responder: "lfm3-1.7b-q5_k_m", decision: nil, readiness: none, pins: { _ in nil })
        XCTAssertTrue(ids.contains("qwen3-0.6b-q8_0"), "no decision choice known: the stage's own need stands")
    }
}
