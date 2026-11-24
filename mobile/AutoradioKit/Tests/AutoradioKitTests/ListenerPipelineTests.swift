import XCTest
@testable import AutoradioKit

// The default listening pipeline, chosen from the model list (Voice/ListenerPipeline.swift). The failure list is the header of that file (1 to 5) and of
// `LiveThenFinalTranscriber` (C1 to C4); each test names the ones it covers.

final class ListenerPipelineTests: XCTestCase {
    let everyStage: (StageKind, String) -> Bool = { _, _ in true }
    func choose(ready: Set<String>, stages: @escaping (StageKind, String) -> Bool = { _, _ in true }, table: [ModelCandidate] = CandidateTable.all) -> ListenerPipeline.Choice {
        ListenerPipeline.choose(table: table, isReady: { ready.contains($0) }, hasStage: stages)
    }
    let silero = "silero-vad-v6.2.1", eou = "parakeet-eou-120m", tdt = "parakeet-tdt-0.6b-v2", fluid = "fluid-speech-mini"

    // 1: nothing on the phone: what ran before, stated.
    func testNothingReadyIsTheLevelDetectorAndApple() {
        let c = choose(ready: [])
        XCTAssertEqual(c.bounds, StageSetting(id: "energy-vad", params: ["hangoverSeconds": .number(0.25)]))
        XCTAssertEqual(c.transcriber, StageSetting(id: "apple-speech"))
        XCTAssertTrue(c.models.isEmpty)
        XCTAssertTrue(c.summary.contains("Apple's recogniser"), c.summary)
    }

    // The default the owner asked for: Silero, live words from Parakeet EOU, the final transcript from Fluid Speech Mini.
    func testEveryModelReadyIsSileroAndLiveWordsAndFluidForTheTranscript() {
        let c = choose(ready: [silero, eou, fluid, tdt])
        XCTAssertEqual(c.bounds.id, "silero-vad")
        XCTAssertEqual(c.transcriber, StageSetting(id: "live-then-final", params: ["live": .string("parakeet-eou"), "final": .string("fluid-speech-mini")]))
        XCTAssertEqual(c.models, ["bounds": silero, "live": eou, "final": fluid])
    }

    // 4 and the in-between states: a fresh download is complete before it is ready, so only these combinations can occur.
    func testPartsOfThePipelineFallBackOneJobAtATime() {
        XCTAssertEqual(choose(ready: [silero]).bounds.id, "silero-vad")
        XCTAssertEqual(choose(ready: [silero]).transcriber.id, "apple-speech", "the speech detector alone does not change the recogniser")
        XCTAssertEqual(choose(ready: [eou]).transcriber, StageSetting(id: "parakeet-eou"), "a streaming model alone is both the live words and the transcript")
        XCTAssertEqual(choose(ready: [eou]).bounds.id, "energy-vad")
        let fluidOnly = choose(ready: [fluid]).transcriber
        XCTAssertEqual(fluidOnly, StageSetting(id: "live-then-final", params: ["live": .string("apple-speech"), "final": .string("fluid-speech-mini")]),
                       "with no streaming model, Apple's recogniser gives the live words only; the transcript is Fluid's")
        XCTAssertEqual(choose(ready: [tdt, eou]).transcriber.params["final"], .string("parakeet-tdt"))
    }

    // 2: a ready model whose stage this build lacks is not named.
    func testAModelWhoseStageIsNotInThisBuildIsSkipped() {
        let noFluid: (StageKind, String) -> Bool = { _, id in id != "fluid-speech-mini" }
        let c = choose(ready: [silero, eou, fluid], stages: noFluid)
        XCTAssertEqual(c.transcriber, StageSetting(id: "parakeet-eou"))
        let none = choose(ready: [silero, eou, fluid], stages: { _, id in id == "apple-speech" || id == "energy-vad" })
        XCTAssertEqual(none.bounds.id, "energy-vad"); XCTAssertEqual(none.transcriber.id, "apple-speech")
    }

    // 3: a better model later is a list change, not a code change.
    func testANewRowWithAHigherRankIsChosenWithNoCodeChange() {
        var table = CandidateTable.all
        let fluidRow = table.first { $0.id == fluid }!
        var better = fluidRow
        better.id = "better-mini"; better.qualityRank = 95; better.kitStageID = "better-mini-stage"
        table.append(better)
        XCTAssertEqual(choose(ready: [fluid, "better-mini"], table: table).transcriber.params["final"], .string("better-mini-stage"))
        XCTAssertEqual(choose(ready: [fluid], table: table).transcriber.params["final"], .string("fluid-speech-mini"), "the better one is not on the phone yet")
        // the shipped list ranks Fluid Speech Mini above the Parakeet batch model and Apple's recogniser
        let ranks = CandidateTable.candidates(.transcriber).map(\.id)
        XCTAssertLessThan(ranks.firstIndex(of: fluid)!, ranks.firstIndex(of: tdt)!)
        XCTAssertLessThan(ranks.firstIndex(of: fluid)!, ranks.firstIndex(of: "apple-speech")!)
    }

    // The listener configuration is built from it; before the models it is what shipped before.
    func testTheListenerConfigurationReadsTheChoice() {
        let r = StageRegistry.builtIn()
        r.register(.speechBounds, id: "silero-vad", descriptor: StageDescriptor(displayName: "silero")) { _ in EnergyVAD(StageParams()) }
        r.register(.transcriber, id: "parakeet-eou", descriptor: StageDescriptor(displayName: "eou")) { _ in StubTranscriber(text: "") }
        r.register(.transcriber, id: "fluid-speech-mini", descriptor: StageDescriptor(displayName: "fluid")) { _ in StubTranscriber(text: "") }
        let none = StageRegistry.listenerConfiguration(registry: r, readiness: MemoryReadiness())
        XCTAssertEqual(none.setting(.speechBounds)?.id, "energy-vad"); XCTAssertEqual(none.setting(.transcriber)?.id, "apple-speech")
        let all = StageRegistry.listenerConfiguration(registry: r, readiness: MemoryReadiness([silero, eou, fluid]))
        XCTAssertEqual(all.setting(.speechBounds)?.id, "silero-vad")
        XCTAssertEqual(all.setting(.transcriber)?.id, "live-then-final")
        XCTAssertEqual(all.audio, .alwaysTalking)
        // a build with no registry knows no model stage: the floors, whatever the record says
        XCTAssertEqual(StageRegistry.listenerConfiguration(registry: nil, readiness: MemoryReadiness([silero, eou, fluid])).setting(.transcriber)?.id, "apple-speech")
        XCTAssertTrue(ListenerPipeline.summary(of: all).contains("parakeet-eou") && ListenerPipeline.summary(of: all).contains("fluid-speech-mini"))
    }

    // Setup downloads the live words beside a batch model; the list says which.
    func testSetupFetchesTheLiveWordsBesideTheFinalModel() {
        XCTAssertEqual(ListenerPipeline.companions(of: fluid), [eou])
        XCTAssertEqual(ListenerPipeline.companions(of: tdt), [eou])
        XCTAssertEqual(ListenerPipeline.companions(of: eou), [])
        XCTAssertEqual(ListenerPipeline.companions(of: "apple-speech"), [])
        let pro = SetupCatalog.ladder(.transcriber).first!                    // the first row is Fluid Speech Pro; the plan drops it on a phone under 7 GB
        XCTAssertEqual(pro.id, "fluid-speech-pro")
        XCTAssertEqual(ListenerPipeline.companions(of: pro.id), [eou])
        XCTAssertEqual(pro.bytes, KnownModels.fluidSpeechPro.totalBytes + 250_000_000, "Pro's size includes the live words too")
        let row = SetupCatalog.ladder(.transcriber).first { $0.id == fluid }!
        XCTAssertEqual(row.kitStageID, "live-then-final")
        XCTAssertEqual(row.bytes, KnownModels.fluidSpeechMini.totalBytes + 250_000_000, "the size the person is asked about includes the live words")
        XCTAssertTrue(SetupCatalog.ladder(.transcriber).last!.isFloor)
    }

    // MARK: The stage

    final class StubTranscriber: Transcriber, @unchecked Sendable {
        static let stageID = "stub-tr"
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "stub") }
        var text: String; var failBegin = false; var failFinish = false
        var sessions: [Session] = []
        init(text: String, failBegin: Bool = false, failFinish: Bool = false) { self.text = text; self.failBegin = failBegin; self.failFinish = failFinish }
        func begin() async throws -> TranscriptionSession {
            if failBegin { throw LabError.unavailable("no model") }
            let s = Session(text: text, fail: failFinish); sessions.append(s); return s
        }
        final class Session: TranscriptionSession, @unchecked Sendable {
            let partials: AsyncStream<String>; let cont: AsyncStream<String>.Continuation
            let text: String, fail: Bool
            var frames = 0, cancelled = false, finished = false
            init(text: String, fail: Bool) { self.text = text; self.fail = fail; (partials, cont) = AsyncStream<String>.makeStream() }
            func append(_ frame: AudioFrame) { frames += 1 }
            func finish() async throws -> String { cont.finish(); finished = true; if fail { throw LabError.unavailable("boom") }; return text }
            func cancel() { cancelled = true; cont.finish() }
        }
    }

    func registry(live: StubTranscriber?, final: StubTranscriber?, apple: StubTranscriber? = nil) -> StageRegistry {
        let r = StageRegistry()
        if let live { r.register(.transcriber, id: "live-stub", descriptor: StageDescriptor(displayName: "l", needsModels: ["live-model"])) { _ in live } }
        if let final { r.register(.transcriber, id: "final-stub", descriptor: StageDescriptor(displayName: "f", needsModels: ["final-model"])) { _ in final } }
        if let apple { r.register(.transcriber, id: "apple-speech", descriptor: StageDescriptor(displayName: "a")) { _ in apple } }
        return r
    }
    func stage(_ r: StageRegistry, ready: Set<String>) -> LiveThenFinalTranscriber {
        LiveThenFinalTranscriber(StageParams(["live": .string("live-stub"), "final": .string("final-stub")]), registry: r, readiness: MemoryReadiness(ready))
    }
    func frame() -> AudioFrame { AudioFrame(samples: [Float](repeating: 0, count: 512), sampleRate: 16_000, t: 0) }

    func testBothHearEveryFrameAndTheFinalOneGivesTheTranscript() async throws {
        let live = StubTranscriber(text: "tell me"), final = StubTranscriber(text: "Tell me more about the park.")
        let s = try await stage(registry(live: live, final: final), ready: ["live-model", "final-model"]).begin()
        s.append(frame()); s.append(frame())
        XCTAssertEqual(live.sessions[0].frames, 2); XCTAssertEqual(final.sessions[0].frames, 2)
        let text = try await s.finish()
        XCTAssertEqual(text, "Tell me more about the park.", "the final transcript, not the live one")
        // S3: the live session is finished (its worker joined), never cancelled while its worker may still touch the shared manager
        XCTAssertTrue(live.sessions[0].finished, "the live session is closed once the final one answered")
        XCTAssertFalse(live.sessions[0].cancelled)
    }

    func testTheLiveWordsAreTheSessionsPartials() async throws {
        let live = StubTranscriber(text: "x"), final = StubTranscriber(text: "y")
        let s = try await stage(registry(live: live, final: final), ready: ["live-model", "final-model"]).begin()
        live.sessions[0].cont.yield("tell me")
        var got: String?
        for await p in s.partials { got = p; break }
        XCTAssertEqual(got, "tell me")
    }

    // C1
    func testAFinalModelThatIsNotOnThePhoneLeavesTheLiveOnesTranscript() async throws {
        let live = StubTranscriber(text: "tell me more"), final = StubTranscriber(text: "never")
        let t = stage(registry(live: live, final: final), ready: ["live-model"])
        let s = try await t.begin()
        XCTAssertTrue(final.sessions.isEmpty, "a model that is not ready is never started (the stage must not download)")
        let text = try await s.finish()
        XCTAssertEqual(text, "tell me more")
        XCTAssertTrue(t.notes.contains { $0.contains("final-stub is not on this phone") }, "\(t.notes)")
        // and one that is ready but cannot start
        let broken = StubTranscriber(text: "never", failBegin: true)
        let s2 = try await stage(registry(live: live, final: broken), ready: ["live-model", "final-model"]).begin()
        let t2 = try await s2.finish()
        XCTAssertEqual(t2, "tell me more")
    }

    // C2
    func testAFinalModelThatFailsAtTheEndLeavesTheLiveOnesTranscript() async throws {
        let live = StubTranscriber(text: "tell me more"), final = StubTranscriber(text: "never", failFinish: true)
        let s = try await stage(registry(live: live, final: final), ready: ["live-model", "final-model"]).begin()
        let text = try await s.finish()
        XCTAssertEqual(text, "tell me more")
    }

    // S4: an empty final transcript is not an answer when the live recogniser heard words.
    func testAnEmptyFinalTranscriptFallsBackToTheLiveWords() async throws {
        let live = StubTranscriber(text: "tell me more"), final = StubTranscriber(text: "")
        let s = try await stage(registry(live: live, final: final), ready: ["live-model", "final-model"]).begin()
        let text = try await s.finish()
        XCTAssertEqual(text, "tell me more")
        let both = StubTranscriber(text: ""), none = StubTranscriber(text: "")
        let s2 = try await stage(registry(live: both, final: none), ready: ["live-model", "final-model"]).begin()
        let empty = try await s2.finish()
        XCTAssertEqual(empty, "", "nothing heard by either is still nothing")
    }

    // B1: after voice setup the running app's listener is rebuilt from the list, speech detection and recognition together, with the composite's params.
    func testApplyingTheChoiceAfterSetupRebuildsTheRunningListener() {
        let r = StageRegistry.builtIn()
        r.register(.speechBounds, id: "silero-vad", descriptor: StageDescriptor(displayName: "silero")) { _ in EnergyVAD(StageParams()) }
        r.register(.transcriber, id: "parakeet-eou", descriptor: StageDescriptor(displayName: "eou")) { _ in StubTranscriber(text: "") }
        r.register(.transcriber, id: "fluid-speech-mini", descriptor: StageDescriptor(displayName: "fluid")) { _ in StubTranscriber(text: "") }
        let ready = MemoryReadiness()
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("lp-\(UUID().uuidString)"), session: NullSessionOwner(), registry: r, readiness: ready)
        XCTAssertEqual(rt.configuration.setting(.transcriber)?.id, "apple-speech", "before the models are on the phone")
        for m in [silero, eou, fluid] { ready.markReady(model: m) }
        rt.applyListenerChoice()
        XCTAssertEqual(rt.configuration.setting(.speechBounds)?.id, "silero-vad")
        XCTAssertEqual(rt.configuration.setting(.transcriber), StageSetting(id: "live-then-final", params: ["live": .string("parakeet-eou"), "final": .string("fluid-speech-mini")]))
    }

    // C3
    func testNoLiveModelMeansApplesRecogniserGivesTheLiveWords() async throws {
        let apple = StubTranscriber(text: "apple words"), final = StubTranscriber(text: "Final words.")
        let s = try await stage(registry(live: nil, final: final, apple: apple), ready: ["final-model"]).begin()
        let text = try await s.finish()
        XCTAssertEqual(text, "Final words.", "Apple's words steer the turn; they are not the transcript")
        XCTAssertEqual(apple.sessions.count, 1)
    }

    // C4
    func testNothingThatCanStartThrows() async {
        let t = stage(registry(live: nil, final: nil), ready: [])
        do { _ = try await t.begin(); XCTFail("began with nothing") } catch {}
    }

    func testTheStageIsRegisteredInTheBuiltInRegistry() {
        XCTAssertTrue(StageRegistry.builtIn().stages(.transcriber).contains { $0.id == "live-then-final" })
    }
}
