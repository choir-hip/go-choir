import XCTest
import CryptoKit
@testable import AutoradioKit

// Round 3 review repairs. FAILURE LIST (written before the tests and the code):
//  1. Cancel a held or tapped turn, the release never comes, the next tap's Send is swallowed.
//  2. The desk speaker's hand-over runs before the local voice exists (afterSystem1 timing), so it never waits for the local sentence
//     and never plays the hand-over sound: double speech.
//  3. Cancel while the radio holds: the programme stays dipped or paused, the session stays "talking".
//  4. A partial model file that is already whole (or a server that answers 416): the download fails on every retry.
//  5. A rejected Core ML model (not a pinned GGUF) is never found on disk and never deleted.
//  6. The desk fails (401, 413, 400) after the local voice spoke: the hand-over sound plays into silence.
//  7. (nit) the unsupported-device and empty-plan exits skip the pruning of candidate weights.

final class RecordingPlayback: PlaybackControl, @unchecked Sendable {
    let lock = NSLock()
    private(set) var levels: [PlayLevel] = [], resumes = 0
    func position() async -> ProgramPosition { ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx") }
    func setLevel(_ level: PlayLevel) async { lock.lock(); levels.append(level); lock.unlock() }
    func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { do { for try await _ in frames {} } catch {} }
    func apply(_ revision: Revision) async {}
    func resume() async { lock.lock(); resumes += 1; lock.unlock() }
}
final class RecordingSession: AudioSessionOwner, @unchecked Sendable {
    let lock = NSLock()
    private(set) var claims: [AudioState] = []
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { lock.lock(); claims.append(state); lock.unlock() }
    func describe() -> String { "" }
    func release() {}
    func observe(_ handler: @escaping @Sendable (String, String) -> Void) {}
}
final class FailingStreamingDesk: StreamingDesk, @unchecked Sendable {
    static let stageID = "failing-stream-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "failing stream desk") }
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> { AsyncThrowingStream { $0.finish() } }
    func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error> {
        AsyncThrowingStream { c in Task { try? await Task.sleep(nanoseconds: 700_000_000); c.finish(throwing: ChoirError.unauthorized) } }
    }
}

final class RoundThreeEngineTests: XCTestCase {
    override func setUp() { AckHistory.shared.clear(); TurnHistory.shared.clear(); ScriptedStreamingDesk.hang = false; ScriptedStreamingDesk.requests = []
        ScriptedStreamingDesk.sentences = ["The desk says the strike ended.", "Talks resume Monday."] }
    override func tearDown() { LocalLanguage.model = nil; ScriptedStreamingDesk.hang = false }

    func make(desk: String = "scripted-stream-desk", deskDelay: Double = 0.3, s1Delay: Double = 0.05, afterSystem1: Bool = false,
              playback: PlaybackControl = SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx")),
              session: AudioSessionOwner = NullSessionOwner(), button: Bool = false) throws -> (TurnEngine, EventLog, TurnEngine.Turn) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, TextTranscriber.self) { TextTranscriber($0) }
        r.register(.desk, ScriptedStreamingDesk.self) { ScriptedStreamingDesk($0) }
        r.register(.desk, FailingStreamingDesk.self) { FailingStreamingDesk($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        if button { cfg.set(.addressing, .init(id: "button")) }
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(s1Delay), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "text-tr")); cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: desk, params: ["delay": .number(deskDelay)]))
        if afterSystem1 { cfg.ackTiming = .afterSystem1 }
        let log = EventLog(clock: LabClock())
        let env = EngineEnvironment(registry: r, log: log, playback: playback, session: session)
        let engine = TurnEngine(env: env, configuration: cfg)
        let t = TurnEngine.Turn(id: "t1", config: cfg, transcriber: try r.make(.transcriber, as: Transcriber.self, cfg),
                                yield: try r.make(.yieldPolicy, as: YieldPolicy.self, cfg), session: EmptyTranscription(), firstEventIndex: 0)
        log.emit(.turnEnded); log.emit(.finalTranscript("what is the top story"))
        return (engine, log, t)
    }
    static let pieces = ["Looking at the front page now. ", "The paper has several stories. ", "Checking which is first. ", "Still checking the order of them. ",
                         "Looking at the second story. ", "Reading the third one now."]

    // 1
    func testSendAfterACancelledTurnIsNotSwallowed() async throws {
        let (engine, _, _) = try make(desk: "rec-desk", button: true)
        RecordingDesk.turns = []
        await engine.buttonDown()
        await engine.cancelTurn()                               // tap-to-talk: no release of the cancelled hold ever arrives
        await engine.buttonDown(); await engine.buttonUp(held: 1.0)
        await engine.waitUntilIdle()
        XCTAssertEqual(RecordingDesk.turns.count, 1, "the next turn's Send reached the desk")
    }

    // 2 (afterSystem1: the pipeline suspends on System 1 before the local voice exists)
    func testDeskHandoverWaitsForTheLocalVoiceEvenWhenItStartsAfterSystem1() async throws {
        let m = FakeLocalModel(pieces: Self.pieces, pieceDelay: 0.35)
        LocalLanguage.model = m
        let (engine, log, t) = try make(deskDelay: 1.4, s1Delay: 0.2, afterSystem1: true)
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let ends = log.events.compactMap { e -> String? in
            switch e.event { case .localSentenceStarted: return "local"; case .transitionPlayed: return "transition"; case .deskSentenceStarted: return "desk"; default: return nil } }
        let ti = try XCTUnwrap(ends.firstIndex(of: "transition"), "a hand-over sound between the phone's voice and the desk's: \(ends)")
        XCTAssertTrue(ends[..<ti].contains("local") && ends[..<ti].allSatisfy { $0 == "local" }, "\(ends)")
        XCTAssertEqual(Array(ends[(ti + 1)...]), ["desk", "desk"], "\(ends)")
    }

    // 3
    func testCancelWhileHoldingRestoresTheProgrammeAndTheListeningSession() async throws {
        ScriptedStreamingDesk.hang = true
        let pb = RecordingPlayback(), ss = RecordingSession()
        let (engine, log, _) = try make(playback: pb, session: ss)
        let started = await engine.typedTurn("what is the top story")
        XCTAssertTrue(started)
        for _ in 0..<200 { let p = await engine.phase; if p == .holding || p == .acknowledging { break }; try await Task.sleep(nanoseconds: 25_000_000) }
        await engine.setLevelTo(.pause, reason: .pause)          // what a listener's speech did to the programme
        let before = pb.resumes, claimsBefore = ss.claims.count
        await engine.cancelTurn()
        await engine.waitUntilIdle()
        XCTAssertEqual(pb.levels.last, .play, "the programme is back at full level: \(pb.levels)")
        XCTAssertGreaterThan(pb.resumes, before, "the programme resumed")
        XCTAssertEqual(Array(ss.claims.dropFirst(claimsBefore)).last, .listening, "the listening session is claimed again")
        XCTAssertTrue(log.events.contains { if case .turnCancelled = $0.event { return true } else { return false } })
    }

    // 6
    func testNoHandoverSoundWhenTheDeskFailsWithoutSpeaking() async throws {
        LocalLanguage.model = FakeLocalModel(pieces: Self.pieces.prefix(2).map { $0 }, pieceDelay: 0.05)
        let (engine, log, t) = try make(desk: "failing-stream-desk")
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let local = log.events.filter { if case .localSentenceStarted = $0.event { return true } else { return false } }
        XCTAssertFalse(local.isEmpty, "the local voice did speak")
        XCTAssertFalse(log.events.contains { if case .transitionPlayed = $0.event { return true } else { return false } }, "no hand-over into silence")
        XCTAssertTrue(log.events.contains { if case .error(let st, _) = $0.event { return st == "desk" } else { return false } })
    }
}

final class ModelStoreResumeTests: XCTestCase {
    let payload = String(repeating: "autoradio-model-bytes-", count: 40)
    var sha: String { SHA256.hash(data: Data(payload.utf8)).map { String(format: "%02x", $0) }.joined() }
    func store() -> (ModelStore, ModelSpec, URL) {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("ms-\(UUID().uuidString)", isDirectory: true)
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        let s = ModelStore(root: root, session: URLSession(configuration: cfg))
        let spec = ModelSpec(id: "t", url: URL(string: "https://example.com/m.bin")!, sha256: sha, bytes: Int64(payload.utf8.count), fileName: "m.bin")
        return (s, spec, root)
    }

    func testAPartialFileThatIsAlreadyWholeIsVerifiedAndMovedWithoutARequest() async throws {
        let (s, spec, root) = store()
        await s.register(spec)
        try Data(payload.utf8).write(to: root.appendingPathComponent("m.bin.part"))
        StubProtocol.handler = { _ in XCTFail("no request is needed"); return .init(status: 500) }
        let url = try await s.ensure("t")
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), payload)
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.appendingPathComponent("m.bin.part").path))
    }

    func testA416ForAPartialFileStartsOverFromTheFirstByte() async throws {
        let (s, spec, root) = store()
        await s.register(spec)
        try Data(payload.utf8.prefix(100)).write(to: root.appendingPathComponent("m.bin.part"))
        StubProtocol.handler = { req in req.headers["range"] != nil ? .init(status: 416) : .init(raw: self.payload) }
        let url = try await s.ensure("t")
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), payload)
    }

    func testAWholeButCorruptPartialFileIsDroppedAndFetchedAgain() async throws {
        let (s, spec, root) = store()
        await s.register(spec)
        try Data(String(repeating: "x", count: payload.utf8.count).utf8).write(to: root.appendingPathComponent("m.bin.part"))
        StubProtocol.handler = { req in req.headers["range"] != nil ? .init(status: 416) : .init(raw: self.payload) }
        let url = try await s.ensure("t")
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), payload)
    }
}

final class ManagedModelCleanupTests: XCTestCase {
    func testARejectedCoreMLModelIsFoundOnDiskAndDeletedAndOtherFoldersStay() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("fluid-\(UUID().uuidString)", isDirectory: true)
        let fm = FileManager.default
        for d in ["parakeet-tdt-0.6b-v2", "kokoro-82m-coreml", "someone-elses"] { try fm.createDirectory(at: root.appendingPathComponent(d), withIntermediateDirectories: true) }
        let backend = LiveConfigBackend(store: ModelStore(root: root.appendingPathComponent("gguf")), managedRoot: root)
        let parakeet = try XCTUnwrap(CandidateTable.all.first { $0.id == "parakeet-tdt-0.6b-v2" })
        let kokoro = try XCTUnwrap(CandidateTable.all.first { $0.id == "kokoro-82m-ane" })
        let pocket = try XCTUnwrap(CandidateTable.all.first { $0.id == "pocket-tts" })
        var onDisk = await backend.isOnDisk(parakeet); XCTAssertTrue(onDisk)
        onDisk = await backend.isOnDisk(pocket); XCTAssertFalse(onDisk, "nothing fetched")
        await backend.delete(parakeet)
        onDisk = await backend.isOnDisk(parakeet); XCTAssertFalse(onDisk)
        XCTAssertFalse(fm.fileExists(atPath: root.appendingPathComponent("parakeet-tdt-0.6b-v2").path))
        XCTAssertTrue(fm.fileExists(atPath: root.appendingPathComponent("kokoro-82m-coreml").path), "another model's folder stays")
        XCTAssertTrue(fm.fileExists(atPath: root.appendingPathComponent("someone-elses").path))
        onDisk = await backend.isOnDisk(kokoro); XCTAssertTrue(onDisk)
        await backend.delete(kokoro)
        XCTAssertFalse(fm.fileExists(atPath: root.appendingPathComponent("kokoro-82m-coreml").path))
    }

    func testEveryManagedCoreMLCandidateNamesItsFolders() {
        for c in CandidateTable.all where !c.isFloor && !KnownModels.isPinned(c.id) && c.bytes > 0 {
            if let spec = KnownModels.managed.first(where: { $0.id == c.id }), spec.cachePath == KnownModels.fluidRoot {
                XCTAssertFalse((spec.cacheFolders ?? []).isEmpty, "\(c.id) has no folder to delete")
            }
        }
    }
}
