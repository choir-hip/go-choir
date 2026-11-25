import XCTest
@testable import AutoradioKit

// The failure list is at the top of VoiceChoice.swift (speaker and choice) and VoiceSampler.swift (cards); the numbers below refer to it.
// No wall-clock cutoff is asserted except the slow-voice deadline, which the test sets itself to a fraction of a second.

/// A voice that says what it was told to do. `kind`: ok, fail (throws before any sound), silent, slow, partial (one frame, then throws).
final class FakeVoice: Voice, @unchecked Sendable {
    static let stageID = "fake-voice"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "fake") }
    enum Kind { case ok, fail, silent, slow, partial }
    let kind: Kind, tag: Float
    private let lock = NSLock()
    private(set) var lines: [String] = []
    private(set) var cancelled = false
    init(_ kind: Kind, tag: Float = 1) { self.kind = kind; self.tag = tag }
    struct Boom: Error {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            let t = Task { [self] in
                for await line in text {
                    lock.lock(); lines.append(line); lock.unlock()
                    switch kind {
                    case .ok: c.yield(AudioFrame(samples: [tag, tag], sampleRate: 24000, t: 0))
                    case .fail: c.finish(throwing: Boom()); return
                    case .silent: break
                    case .slow: try? await Task.sleep(nanoseconds: 3_000_000_000); c.yield(AudioFrame(samples: [tag], sampleRate: 24000, t: 0))
                    case .partial: c.yield(AudioFrame(samples: [tag], sampleRate: 24000, t: 0)); c.finish(throwing: Boom()); return
                    }
                }
                c.finish()
            }
            c.onTermination = { [self] _ in lock.lock(); if !t.isCancelled { cancelled = true }; lock.unlock(); t.cancel() }
        }
    }
}

final class MemoryReadiness: VoiceReadinessRecord, @unchecked Sendable {
    private let lock = NSLock()
    var ready: Set<String>
    init(_ ready: Set<String> = []) { self.ready = ready }
    func isReady(model: String) -> Bool { lock.lock(); defer { lock.unlock() }; return ready.contains(model) }
    @discardableResult func markReady(model: String) -> Bool { lock.lock(); ready.insert(model); lock.unlock(); return true }
    func markNotReady(model: String) { lock.lock(); ready.remove(model); lock.unlock() }
}

final class VoiceChoiceTests: XCTestCase {
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("voice-choice-\(UUID().uuidString)") }
    func collect(_ v: Voice, _ lines: [String]) async throws -> [Float] {
        var out: [Float] = []
        for try await f in v.speak(AsyncStream { c in for l in lines { c.yield(l) }; c.finish() }) { out += f.samples }
        return out
    }

    /// A registry with a fake neural stage "kokoro" (needs the kokoro model), a fake bundled voice "tiny-voca" (the fallback), and a plain "fixture-voice".
    func registry(neural: FakeVoice?, bundled: FakeVoice, plain: FakeVoice? = nil) -> StageRegistry {
        let r = StageRegistry()
        if let neural {
            r.register(.voice, id: "kokoro", descriptor: StageDescriptor(displayName: "k", needsModels: [VoiceCatalog.kokoroModel])) { _ in neural }
        }
        r.register(.voice, id: VoiceCatalog.bundledStageID, descriptor: StageDescriptor(displayName: "Tiny Voca")) { _ in bundled }
        if let plain { r.register(.voice, id: "fixture-voice", descriptor: StageDescriptor(displayName: "p")) { _ in plain } }
        return r
    }
    func config(_ voice: StageSetting) -> LabConfiguration { LabConfiguration(name: "t", stages: [.voice: voice]) }
    func log() -> EventLog { EventLog(clock: LabClock()) }
    func notes(_ l: EventLog) -> [String] { l.events.compactMap { if case .note(let n) = $0.event { return n } else { return nil } } }

    // 1, 2: the choice
    func testFreshInstallDefaultsToBundledTinyVocaAndIgnoresAnOldAppleSelection() {
        let s = VoiceChoiceStore(file: tmp().appendingPathComponent("v.json"))
        XCTAssertEqual(s.current().id, "tiny-voca")
        XCTAssertEqual(VoiceCatalog.option(matching: s.current())?.id, "tiny-voca:F2")
        XCTAssertEqual(StageRegistry.listenerConfiguration().setting(.voice)?.id, "tiny-voca")
        s.save(StageSetting(id: "avspeech"))
        XCTAssertEqual(s.current(), StageRegistry.listenerConfiguration().setting(.voice))
        XCTAssertFalse(StageRegistry.builtIn().stages(.voice).contains { $0.id == "avspeech" })
    }
    func testASavedChoiceSurvivesAndGarbageDoesNot() throws {
        let f = tmp().appendingPathComponent("v.json")
        let s = VoiceChoiceStore(file: f)
        let o = VoiceCatalog.option(id: "kokoro:bf_emma")!
        s.save(o.setting)
        XCTAssertEqual(VoiceChoiceStore(file: f).current(), o.setting)
        try "not json".write(to: f, atomically: true, encoding: .utf8)
        XCTAssertEqual(VoiceChoiceStore(file: f).current(), VoiceCatalog.defaultOption.setting)
        s.save(StageSetting(id: ""))
        XCTAssertEqual(VoiceChoiceStore(file: f).current(), VoiceCatalog.defaultOption.setting)
    }
    func testAStageOnlySettingMeansItsFirstVoiceAndCatalogIdsAreUnique() {
        XCTAssertEqual(VoiceCatalog.option(matching: StageSetting(id: "pocket-tts"))?.id, "pocket-tts:alba")
        XCTAssertEqual(VoiceCatalog.option(matching: StageSetting(id: VoiceCatalog.bundledStageID))?.id, "tiny-voca:F2", "the bundled stage's first voice is F2")
        XCTAssertNil(VoiceCatalog.option(matching: StageSetting(id: "avspeech")), "Apple speech is not a voice any more")
        XCTAssertNil(VoiceCatalog.option(matching: StageSetting(id: "kokoro", params: ["voice": .string("nobody")])))
        XCTAssertEqual(Set(VoiceCatalog.options.map(\.id)).count, VoiceCatalog.options.count)
        for o in VoiceCatalog.options where !o.modelID.isEmpty { XCTAssertTrue(KnownModels.contains(o.modelID), "\(o.id) names a model the catalog lacks") }
        XCTAssertTrue(VoiceCatalog.options.prefix(2).allSatisfy { $0.modelID.isEmpty }, "the bundled voices need no download")
    }
    func testAPhononVoiceWhoseModelIsNotReadySpeaksWithTheBundledVoiceAndSaysWhy() async throws {
        let neural = FakeVoice(.ok), bundled = FakeVoice(.ok)
        let r = StageRegistry()
        r.register(.voice, id: "phonon", descriptor: StageDescriptor(displayName: "p", needsModels: [VoiceCatalog.phononModel])) { _ in neural }
        r.register(.voice, id: VoiceCatalog.bundledStageID, descriptor: StageDescriptor(displayName: "Tiny Voca")) { _ in bundled }
        let l = log()
        _ = try r.makeSpeaker(config(VoiceCatalog.option(id: "phonon:en_us_Easton")!.setting), log: l, readiness: MemoryReadiness())
        XCTAssertTrue(notes(l).contains { $0.contains("not ready") && $0.contains(VoiceCatalog.phononModel) }, "\(notes(l))")
    }
    func testTheChoiceSurvivesARelaunchThroughTheRuntime() {
        let dir = tmp()
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        XCTAssertEqual(rt.configuration.setting(.voice)?.id, VoiceCatalog.bundledStageID, "a fresh install speaks with the bundled voice")
        rt.setVoice(VoiceCatalog.option(id: "kokoro:am_adam")!)
        rt.setStage(.voice, id: "pocket-tts")                               // the Components page
        let again = LabRuntime(supportDir: dir, session: NullSessionOwner())
        XCTAssertEqual(again.configuration.setting(.voice)?.id, "pocket-tts", "a voice picked on the Components page is still picked after a relaunch")
        again.setVoice(VoiceCatalog.option(id: "kokoro:am_adam")!)
        let third = LabRuntime(supportDir: dir, session: NullSessionOwner())
        XCTAssertEqual(third.configuration.setting(.voice), VoiceCatalog.option(id: "kokoro:am_adam")!.setting)
    }
    func testReadinessIsRememberedAndForgotten() {
        let f = tmp().appendingPathComponent("ready.json")
        let r = FileVoiceReadiness(file: f, pins: { _ in ModelPin(pin: "t", present: { true }) })
        XCTAssertFalse(r.isReady(model: "kokoro-82m-ane"))
        r.markReady(model: "kokoro-82m-ane")
        XCTAssertTrue(FileVoiceReadiness(file: f, pins: { _ in ModelPin(pin: "t", present: { true }) }).isReady(model: "kokoro-82m-ane"))
        r.markNotReady(model: "kokoro-82m-ane")
        XCTAssertFalse(FileVoiceReadiness(file: f, pins: { _ in ModelPin(pin: "t", present: { true }) }).isReady(model: "kokoro-82m-ane"))
    }

    // 3: not ready
    func testNotReadyMeansTheBundledVoiceAndTheLogSaysWhy() async throws {
        let neural = FakeVoice(.ok, tag: 1), bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: l, readiness: MemoryReadiness())
        let out = try await collect(v, ["Hello."])
        XCTAssertEqual(out, [9, 9])
        XCTAssertTrue(neural.lines.isEmpty, "a voice whose model is not on the phone is never asked to speak")
        XCTAssertTrue(notes(l).contains { $0.contains("not ready") && $0.contains(VoiceCatalog.kokoroModel) })
    }
    func testReadyMeansTheNeuralVoiceSpeaksAndTheLogSaysWhich() async throws {
        let neural = FakeVoice(.ok, tag: 1), bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(VoiceCatalog.option(id: "kokoro:af_heart")!.setting), log: l, readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let out = try await collect(v, ["One.", "Two."])
        XCTAssertEqual(out, [1, 1, 1, 1])
        XCTAssertTrue(bundled.lines.isEmpty)
        XCTAssertTrue(notes(l).contains { $0.contains("spoke 2 line(s) with kokoro:af_heart") }, "\(notes(l))")
    }

    // 4, 5, 6: failures, line by line
    func testAVoiceThatThrowsBeforeAnySoundIsReplacedForThatLine() async throws {
        let neural = FakeVoice(.fail), bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: l, readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let got = try await collect(v, ["A.", "B."])
        XCTAssertEqual(got, [9, 9, 9, 9])
        XCTAssertEqual(bundled.lines, ["A.", "B."])
        XCTAssertTrue(notes(l).contains { $0.contains("could not speak this line") })
    }
    func testAVoiceThatMakesNoSoundIsReplaced() async throws {
        let neural = FakeVoice(.silent), bundled = FakeVoice(.ok, tag: 9)
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: log(), readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let got = try await collect(v, ["A."])
        XCTAssertEqual(got, [9, 9])
    }
    func testAVoiceTooSlowToAFirstSoundIsReplaced() async throws {
        let neural = FakeVoice(.slow), bundled = FakeVoice(.ok, tag: 9)
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: log(),
                                                                           readiness: MemoryReadiness([VoiceCatalog.kokoroModel]), firstSoundSeconds: 0.2)
        let t0 = Date()
        let got = try await collect(v, ["A."])
        XCTAssertEqual(got, [9, 9])
        XCTAssertLessThan(Date().timeIntervalSince(t0), 2.5, "the bundled voice must not wait for the slow one")
    }
    func testAVoiceThatFailsPartwayKeepsWhatItMadeAndIsTriedAgainOnTheNextLine() async throws {
        let neural = FakeVoice(.partial, tag: 1), bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: l, readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let got = try await collect(v, ["A.", "B."])
        XCTAssertEqual(got, [1, 1], "each line kept its one frame; nothing was spoken twice")
        XCTAssertEqual(neural.lines, ["A.", "B."])
        XCTAssertTrue(bundled.lines.isEmpty)
        XCTAssertTrue(notes(l).contains { $0.contains("stopped partway") })
    }

    // 7: a cut is not a failure
    func testCuttingInDoesNotFallBack() async throws {
        let neural = FakeVoice(.slow), bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: l, readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let t = Task { _ = try? await self.collect(v, ["A."]) }
        try await Task.sleep(nanoseconds: 200_000_000)
        t.cancel()
        _ = await t.value
        try await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertTrue(bundled.lines.isEmpty, "the listener cut in: the bundled voice must not take over the line")
        XCTAssertFalse(notes(l).contains { $0.contains("bundled Tiny Voca speaks") })
    }

    // 8: other voices are returned as they are
    func testAFixtureVoiceAndTheBundledVoiceAreNotWrapped() throws {
        let plain = FakeVoice(.ok), bundled = FakeVoice(.ok)
        let r = registry(neural: nil, bundled: bundled, plain: plain)
        XCTAssertTrue(try r.makeSpeaker(config(StageSetting(id: "fixture-voice")), log: log(), readiness: MemoryReadiness()) === plain)
        XCTAssertTrue(try r.makeSpeaker(config(StageSetting(id: VoiceCatalog.bundledStageID)), log: log(), readiness: MemoryReadiness()) === bundled)
        XCTAssertTrue(try r.makeSpeaker(config(StageSetting(id: "avspeech")), log: log(), readiness: MemoryReadiness()) === bundled, "an obsolete saved Apple choice speaks with the bundled voice")
    }
    func testANeuralStageThatThisBuildLacksFallsBackToTheBundledVoice() async throws {
        let bundled = FakeVoice(.ok, tag: 9)
        let l = log()
        let v = try registry(neural: nil, bundled: bundled).makeSpeaker(config(StageSetting(id: "kokoro")), log: l, readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let got = try await collect(v, ["A."])
        XCTAssertEqual(got, [9, 9])
        XCTAssertTrue(notes(l).contains { $0.contains("not in this build") })
    }
    func testUnderTheAgentNoModelIsTouched() async throws {
        LabEnvironment.virtualAudio = true; defer { LabEnvironment.virtualAudio = false }
        let neural = FakeVoice(.ok, tag: 1), bundled = FakeVoice(.ok, tag: 9)
        let v = try registry(neural: neural, bundled: bundled).makeSpeaker(config(VoiceCatalog.option(id: "kokoro:af_heart")!.setting), log: log(), readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let got = try await collect(v, ["A."])
        XCTAssertEqual(got.count, 9_600, "the agent's virtual speech: silence of the line's length, no model loaded")
        XCTAssertTrue(got.allSatisfy { $0 == 0 })
        XCTAssertTrue(neural.lines.isEmpty)
        XCTAssertTrue(bundled.lines.isEmpty)
    }

    // 9: two speakers at once
    func testTwoSpeakersAtOnceEachSpeakTheirOwnLines() async throws {
        let neural = FakeVoice(.ok, tag: 1), bundled = FakeVoice(.ok, tag: 9)
        let r = registry(neural: neural, bundled: bundled)
        let a = try r.makeSpeaker(config(StageSetting(id: "kokoro")), log: log(), readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        let b = try r.makeSpeaker(config(StageSetting(id: "kokoro")), log: log(), readiness: MemoryReadiness([VoiceCatalog.kokoroModel]))
        async let x = collect(a, ["A1.", "A2."])
        async let y = collect(b, ["B1."])
        let (xs, ys) = try await (x, y)
        XCTAssertEqual(xs.count, 4); XCTAssertEqual(ys.count, 2)
        XCTAssertEqual(Set(neural.lines), ["A1.", "A2.", "B1."])
    }

    // Speech blocks are short, so a story's first sound comes after one short block.
    func testSpokenBlocksAreShortAndKeepEveryWord() {
        let sentence = "The harbor opened early this morning and the first boats went out."
        let body = Array(repeating: sentence, count: 12).joined(separator: " ")
        let blocks = ChoirText.blocks(title: "Harbor", text: body, maxBlocks: 200)
        XCTAssertTrue(blocks.dropFirst().allSatisfy { $0.count <= ChoirText.speechChars + sentence.count }, "a block runs to the end of its sentence, no further")
        XCTAssertGreaterThan(blocks.count, 3)
        XCTAssertEqual(blocks.dropFirst().joined(separator: " ").filter { !$0.isWhitespace }, body.filter { !$0.isWhitespace }, "no word is lost by the split")
    }
}

final class VoiceSamplerTests: XCTestCase {
    /// Records what the sampler asked of its host.
    final class Host: @unchecked Sendable {
        private let lock = NSLock()
        var said: [String] = [], chosen: [String] = [], halts = 0
        var failSay = false, sayDelay: UInt64 = 0
        /// model id -> the progress of the download its own row is running (Voice and models owns every download; the card only shows it).
        var running: [String: Double] = [:]
        func record(_ f: () -> Void) { lock.lock(); f(); lock.unlock() }
        func read<T>(_ f: () -> T) -> T { lock.lock(); defer { lock.unlock() }; return f() }
    }
    struct Down: Error {}
    func make(_ host: Host, ready: Set<String> = [], selected: String = "kokoro:af_heart") -> (VoiceSampler, MemoryReadiness) {
        let r = MemoryReadiness(ready)
        let s = VoiceSampler(selected: selected, readiness: r,
            say: { o in
                host.record { host.said.append(o.id) }
                if host.sayDelay > 0 { try await Task.sleep(nanoseconds: host.sayDelay) }
                if host.read({ host.failSay }) { throw Down() }
            },
            halt: { host.record { host.halts += 1 } },
            choose: { o in host.record { host.chosen.append(o.id) } },
            running: { m in host.read { host.running[m] } })
        return (s, r)
    }
    func settle(_ ms: UInt64 = 150) async { try? await Task.sleep(nanoseconds: ms * 1_000_000) }

    // Every card says which model it is a voice of and carries the credit that model's licence asks for (settings.contract shows both, once per model). The bundled voice has no model to download, but it still has its group and its credit.
    func testEveryCardNamesItsModelAndCarriesItsCredit() {
        let (s, _) = make(Host(), ready: [])
        let cards = s.cards()
        XCTAssertGreaterThan(cards.count, 50)
        for c in cards {
            XCTAssertFalse(c.group.isEmpty, "\(c.id) has no group to sit under")
            XCTAssertFalse(c.credit.isEmpty, "\(c.id) is offered with no credit")
            if c.model.isEmpty { XCTAssertTrue(c.id.hasPrefix("tiny-voca:"), "only the bundled voice needs no model: \(c.id)") }
            else { XCTAssertTrue(KnownModels.contains(c.model), "\(c.id) names a model the catalog lacks") }
        }
        XCTAssertEqual(cards.first { $0.id == "tiny-voca:F2" }?.group, VoiceCatalog.bundledGroup, "the bundled cards group under Tiny Voca (the shared interface's voiceCards contract)")
        XCTAssertEqual(cards.first { $0.id == "tiny-voca:F2" }?.credit, VoiceCatalog.bundledCredit)
        XCTAssertEqual(cards.first { $0.id == "kokoro:af_heart" }?.group, "Kokoro 82M")
        XCTAssertEqual(cards.first { $0.id == "phonon:en_us_Zoey" }?.group, "Phonon 31M")
        XCTAssertTrue(cards.first { $0.id == "pocket-tts:alba" }?.credit.contains("Kyutai") ?? false, "Pocket's CC BY credit comes from the catalog's own credit")
    }

    func testTappingAReadyVoiceChoosesItAndPlaysItAtOnce() async {
        let h = Host()
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel])
        s.pick("kokoro:bf_emma")
        await settle()
        XCTAssertEqual(h.said, ["kokoro:bf_emma"]); XCTAssertEqual(h.chosen, ["kokoro:bf_emma"])
        XCTAssertEqual(s.cards().first { $0.selected }?.id, "kokoro:bf_emma")
        XCTAssertEqual(s.cards().first { $0.id == "kokoro:bf_emma" }?.state, "ready")
        XCTAssertFalse(s.cards().contains { $0.playing }, "a finished sample is not shown as playing")
    }
    func testTheBundledVoiceNeedsNoDownload() async {
        let h = Host()
        let (s, _) = make(h)
        XCTAssertEqual(s.cards().first { $0.id == "tiny-voca:F2" }?.state, "ready")
        s.pick("tiny-voca:F2"); await settle()
        XCTAssertEqual(h.said, ["tiny-voca:F2"])
    }
    func testAnotherTapStopsThePreviousSampleAndOnlyTheNewestPlays() async {
        let h = Host(); h.sayDelay = 400_000_000
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel])
        s.pick("kokoro:af_heart"); await settle(50)
        XCTAssertTrue(s.cards().first { $0.id == "kokoro:af_heart" }!.playing)
        s.pick("kokoro:am_adam"); await settle(50)
        XCTAssertGreaterThanOrEqual(h.halts, 2, "each tap halts what was playing")
        XCTAssertFalse(s.cards().first { $0.id == "kokoro:af_heart" }!.playing)
        XCTAssertTrue(s.cards().first { $0.id == "kokoro:am_adam" }!.playing)
        await settle(600)
        XCTAssertFalse(s.cards().contains { $0.playing })
        XCTAssertEqual(s.selectedID, "kokoro:am_adam")
    }
    // Voice and models owns every download (owner, 2026-11-24: "the press on that row is the agreement"): tapping a voice whose model is not on the phone chooses it and fetches nothing.
    //  S1 the tap chooses the voice at once and plays nothing; the card says which model to download and its size, and the sampler asked its host to fetch nothing (it has no way to).
    //  S2 while that model's own row downloads, the card shows the row's progress; every voice of the model shows it.
    //  S3 once the model is ready the same tap plays the sample.
    func testAVoiceWithoutItsModelIsChosenAndFetchesNothing() async {
        let h = Host()
        let (s, _) = make(h)
        s.pick("kokoro:af_bella"); await settle(100)
        XCTAssertEqual(h.chosen, ["kokoro:af_bella"], "chosen at once; the radio speaks with bundled Tiny Voca until the model is ready")
        let c = s.cards().first { $0.id == "kokoro:af_bella" }!
        XCTAssertEqual(c.state, "needs"); XCTAssertTrue(c.note.contains("Download") && c.note.contains("above"), c.note); XCTAssertFalse(c.note.contains("Settings"))
        XCTAssertTrue(h.said.isEmpty)
    }
    func testACardShowsTheProgressOfItsModelsOwnRow() async {
        let h = Host(); h.record { h.running[VoiceCatalog.kokoroModel] = 0.4 }
        let (s, _) = make(h)
        for id in ["kokoro:af_bella", "kokoro:af_heart"] { let c = s.cards().first { $0.id == id }!; XCTAssertEqual(c.state, "downloading"); XCTAssertEqual(c.progress, 0.4, accuracy: 0.001) }
        XCTAssertEqual(s.cards().first { $0.id == "pocket-tts:alba" }?.state, "needs", "another model's card is unaffected")
    }
    func testOnceTheModelIsReadyTheSameTapPlaysTheSample() async {
        let h = Host()
        let (s, r) = make(h)
        s.pick("kokoro:af_bella"); await settle(50); XCTAssertTrue(h.said.isEmpty)
        r.markReady(model: VoiceCatalog.kokoroModel)
        s.pick("kokoro:af_bella"); await settle(100)
        XCTAssertEqual(h.said, ["kokoro:af_bella"]); XCTAssertEqual(s.cards().first { $0.id == "kokoro:af_bella" }?.state, "ready")
    }
    // Voice and models gives every voice a Test and a Use of its own (owner, 2026-11-24: testing and selecting unified): Test hears the voice and chooses nothing.
    //  S4 `hear` plays that voice's sample and never chooses it; a voice whose model is not on the phone plays nothing and chooses nothing; Use (`pick`) is the choosing.
    func testS4HearPlaysTheSampleAndChoosesNothing() async {
        let h = Host()
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel])
        s.hear("kokoro:am_adam"); await settle(100)
        XCTAssertEqual(h.said, ["kokoro:am_adam"]); XCTAssertTrue(h.chosen.isEmpty, "Test is not Use")
        XCTAssertEqual(s.selectedID, "kokoro:af_heart", "the voice in use did not move")
        s.hear("pocket-tts:alba"); await settle(50)
        XCTAssertEqual(h.said, ["kokoro:am_adam"], "its model is not on the phone: nothing plays"); XCTAssertTrue(h.chosen.isEmpty)
        s.hear("no-such-voice"); await settle(20); XCTAssertEqual(h.said.count, 1)
        s.pick("kokoro:am_adam"); await settle(100)
        XCTAssertEqual(h.chosen, ["kokoro:am_adam"]); XCTAssertEqual(s.selectedID, "kokoro:am_adam")
    }
    func testASampleThatCannotPlaySaysSoAndTheChoiceStands() async {
        let h = Host(); h.failSay = true
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel])
        s.pick("kokoro:bm_george"); await settle()
        XCTAssertEqual(s.selectedID, "kokoro:bm_george")
        XCTAssertFalse(s.cards().first { $0.id == "kokoro:bm_george" }!.note.isEmpty)
    }
    func testStopHaltsAndUnknownIdsAreIgnored() async {
        let h = Host(); h.sayDelay = 500_000_000
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel])
        s.pick("nobody"); await settle(50)
        XCTAssertTrue(h.chosen.isEmpty); XCTAssertTrue(h.said.isEmpty)
        s.pick("kokoro:af_heart"); await settle(50)
        s.stop(); await settle(50)
        XCTAssertFalse(s.cards().contains { $0.playing })
        XCTAssertGreaterThanOrEqual(h.halts, 2)
    }

    // Phonon, an alternative to Kokoro (owner QA round 1, 2026-11-15: "Phonon as an alternative to Kokoro; I want to listen to and compare them").
    func testPhononIsAnAlternativeBesideKokoroAndNeverTheDefault() {
        let ids = VoiceCatalog.options.map(\.id)
        XCTAssertEqual(VoiceCatalog.defaultOption.id, "tiny-voca:F2", "the bundled voice is the default; Phonon never is")
        let phonon = VoiceCatalog.options.filter { $0.stageID == "phonon" }
        XCTAssertEqual(phonon.count, 25, "every English Phonon voice of the pack's catalog")
        XCTAssertEqual(phonon.first?.id, "phonon:en_us_Easton", "the stage's own default voice first")
        XCTAssertTrue(["phonon:en_us_Zoey", "phonon:en_gb_Maeve"].allSatisfy { id in phonon.contains { $0.id == id } })
        for o in phonon {
            XCTAssertEqual(o.modelID, VoiceCatalog.phononModel, "all Phonon voices share one download")
            XCTAssertEqual(o.setting.params["voice"], .string(o.voice))
            XCTAssertFalse(o.sample.isEmpty)
        }
        let lastKokoro = ids.lastIndex { $0.hasPrefix("kokoro:") }!, firstPhonon = ids.firstIndex { $0.hasPrefix("phonon:") }!
        XCTAssertEqual(firstPhonon, lastKokoro + 1, "Phonon's cards sit right after Kokoro's so the two can be tapped back to back")
        XCTAssertTrue(VoiceCatalog.neuralStageIDs.contains("phonon"), "a Phonon voice gets the readiness check and the fall back to bundled Tiny Voca")
        XCTAssertEqual(VoiceCatalog.options.last?.id, "pocket-tts:alba", "Pocket's card is last; the bundled cards, which need no download, are first")
        XCTAssertEqual(VoiceCatalog.option(matching: StageSetting(id: "phonon"))?.id, "phonon:en_us_Easton")
    }
    func testPhononsModelIsInTheCatalogWithItsFacts() throws {
        let m = try XCTUnwrap(KnownModels.managed.first { $0.id == VoiceCatalog.phononModel })
        XCTAssertTrue(m.source.contains("FluidInference/pocket-tts-coreml/phonon/phonon-31m-en"), "the pack's repository and folder, not the speech recogniser's repository")
        XCTAssertFalse(m.source.contains("phonon-2"), "FluidInference/phonon-2-coreml is a speech recogniser, not a voice")
        XCTAssertTrue(m.license.contains("Apache-2.0"))
        XCTAssertEqual(m.cachePath, KnownModels.ttsRoot, "the library keeps voice models in its own TTS tree (TtsRootTests)")
        XCTAssertEqual(m.cacheFolders, ["pocket-tts/phonon"])
        XCTAssertGreaterThan(m.approxBytes, 150_000_000); XCTAssertLessThan(m.approxBytes, 200_000_000)
    }
    func testAPhononCardOfAReadyModelPlaysOnlyThatVoiceBackToBack() async {
        let h = Host()
        let (s, _) = make(h, ready: [VoiceCatalog.kokoroModel, VoiceCatalog.phononModel])
        s.pick("kokoro:af_heart"); await settle(100)
        s.pick("phonon:en_us_Easton"); await settle(100)
        XCTAssertEqual(h.said, ["kokoro:af_heart", "phonon:en_us_Easton"], "back to back")
    }
}
