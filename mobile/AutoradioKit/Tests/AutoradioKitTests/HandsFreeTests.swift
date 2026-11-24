import XCTest
@testable import AutoradioKit

// Hands-free listening (HandsFree.swift). The failure list is the header of that file (H1 to H8); each test names the ones it covers.
// The microphone is frames the test pushes: loud frames are speech, quiet frames are silence. The transcriber is scripted.

final class LoudBounds: SpeechBounds, @unchecked Sendable {
    static let stageID = "loud-bounds"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "loud bounds") }
    private var inSpeech = false
    init(_ p: StageParams) {}
    func reset() async { inSpeech = false }
    func speechSeconds(now: Double) async -> Double { 0 }
    func feed(_ f: AudioFrame) async -> BoundsUpdate {
        let loud = (f.samples.first ?? 0) > 0.3
        if loud && !inSpeech { inSpeech = true; return .speechStarted(at: f.t) }
        if !loud && inSpeech { inSpeech = false; return .speechEnded(at: f.t, seconds: 0.3) }
        return .none
    }
}

/// Each session is one utterance; the test pushes its live words and sets what `finish` returns.
final class ScriptedTranscriber: Transcriber, @unchecked Sendable {
    static let stageID = "scripted-tr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "scripted tr") }
    nonisolated(unsafe) static var finals: [String] = []
    nonisolated(unsafe) static var sessions: [Session] = []
    init(_ p: StageParams) {}
    func begin() async throws -> TranscriptionSession {
        let s = Session(); Self.sessions.append(s); return s
    }
    final class Session: TranscriptionSession, @unchecked Sendable {
        let partials: AsyncStream<String>
        private let cont: AsyncStream<String>.Continuation
        init() { (partials, cont) = AsyncStream<String>.makeStream() }
        func say(_ t: String) { cont.yield(t) }
        func append(_ frame: AudioFrame) {}
        func finish() async throws -> String { cont.finish(); return ScriptedTranscriber.finals.isEmpty ? "" : ScriptedTranscriber.finals.removeFirst() }
        func cancel() { cont.finish() }
    }
}

/// A second recogniser, so a test can tell which one the engine built.
final class OtherTranscriber: Transcriber, @unchecked Sendable {
    static let stageID = "other-tr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "other tr") }
    init(_ p: StageParams) {}
    func begin() async throws -> TranscriptionSession { ScriptedTranscriber.Session() }
}

/// An answer that never finishes until the pipeline is cancelled: the radio is "speaking".
final class HangingDesk: Desk, @unchecked Sendable {
    static let stageID = "hang-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "hang desk") }
    nonisolated(unsafe) static var turns: [DeskTurn] = []
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        Self.turns.append(turn)
        return AsyncThrowingStream { c in
            let t = Task { try? await Task.sleep(nanoseconds: 60_000_000_000); c.finish() }
            c.onTermination = { _ in t.cancel() }
        }
    }
}

final class StoppablePlayback: PlaybackControl, @unchecked Sendable {
    private let lock = NSLock()
    private var _stops = 0
    var stops: Int { lock.lock(); defer { lock.unlock() }; return _stops }
    func position() async -> ProgramPosition { ProgramPosition(programmeID: "p", blockIndex: 1, context: "ctx") }
    func setLevel(_ level: PlayLevel) async {}
    func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { do { for try await _ in frames {} } catch {} }
    func apply(_ revision: Revision) async {}
    func resume() async {}
    func stopVoice() async { lock.lock(); _stops += 1; lock.unlock() }
}

final class HandsFreeTests: XCTestCase {
    func make(desk: String = "rec-desk") throws -> (TurnEngine, EventLog, StoppablePlayback) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.speechBounds, LoudBounds.self) { LoudBounds($0) }
        r.register(.transcriber, ScriptedTranscriber.self) { ScriptedTranscriber($0) }
        r.register(.transcriber, OtherTranscriber.self) { OtherTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        r.register(.desk, HangingDesk.self) { HangingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.addressing, .init(id: "open"))
        cfg.set(.speechBounds, .init(id: "loud-bounds"))
        cfg.set(.transcriber, .init(id: "scripted-tr"))
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice"))
        cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: desk))
        RecordingDesk.turns = []; HangingDesk.turns = []
        ScriptedTranscriber.finals = []; ScriptedTranscriber.sessions = []
        let log = EventLog(clock: LabClock())
        let pb = StoppablePlayback()
        return (TurnEngine(env: EngineEnvironment(registry: r, log: log, playback: pb), configuration: cfg), log, pb)
    }

    private var t = 0.0
    func frame(loud: Bool) -> AudioFrame {
        defer { t += 0.032 }
        return AudioFrame(samples: [Float](repeating: loud ? 0.5 : 0, count: 512), sampleRate: 16000, t: t)
    }
    func push(_ e: TurnEngine, loud: Bool, _ n: Int) async { for _ in 0..<n { await e.ingestHandsFree(frame(loud: loud)) } }

    // B1 (review of d67c0cb77): the listening stages are built on the first frame and were kept for the life of the process, so a
    // configuration applied after voice setup (the models just became ready) never listened until a relaunch.
    func testAConfigurationAppliedAfterTheFirstFrameIsWhatListensNext() async throws {
        let (engine, _, _) = try make()
        await engine.handsFreeSet(true)
        await push(engine, loud: false, 3)
        let before = await engine.hf.transcriber
        XCTAssertTrue(before is ScriptedTranscriber)
        var c = await engine.configuration; c.set(.transcriber, .init(id: "other-tr")); await engine.setConfiguration(c)
        await push(engine, loud: false, 3)
        let after = await engine.hf.transcriber
        XCTAssertTrue(after is OtherTranscriber, "the new recogniser listens without a relaunch")
    }
    func testAConfigurationChangeDuringATurnWaitsForTheTurnToEnd() async throws {
        let (engine, _, _) = try make()
        await engine.handsFreeSet(true)
        await push(engine, loud: true, 6)                                  // a turn is open
        var c = await engine.configuration; c.set(.transcriber, .init(id: "other-tr")); await engine.setConfiguration(c)
        await push(engine, loud: true, 2)
        let during = await engine.hf.transcriber
        XCTAssertTrue(during is ScriptedTranscriber, "the open turn keeps the recogniser that began it")
    }

    // H1: speech, then silence: the turn is sent with the session's final text and no button.
    func testASpokenTurnReachesTheDeskWithNoButton() async throws {
        let (engine, _, _) = try make()
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["what is on today"]
        await push(engine, loud: true, 12)
        await push(engine, loud: false, 45)                       // 1.4 s of silence
        let got = await poll(8) { !RecordingDesk.turns.isEmpty }
        XCTAssertTrue(got, "the turn must reach the desk")
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "what is on today")
        XCTAssertFalse(RecordingDesk.turns.first?.interrupted ?? true)
    }

    // H2: an empty final text sends nothing and the next utterance still works.
    func testACoughIsDiscardedAndTheNextTurnStillWorks() async throws {
        let (engine, _, _) = try make()
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["", "tell me the weather"]
        await push(engine, loud: true, 5)
        await push(engine, loud: false, 45)
        await push(engine, loud: true, 12)
        await push(engine, loud: false, 45)
        let got = await poll(8) { !RecordingDesk.turns.isEmpty }
        XCTAssertTrue(got)
        XCTAssertEqual(RecordingDesk.turns.count, 1, "the cough was not sent")
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "tell me the weather")
    }

    // H4, H7: speaking over the radio stops its voice and cancels its answer; the next turn is sent.
    func testSpeakingOverTheAnswerStopsItAndStartsTheNextTurn() async throws {
        let (engine, log, pb) = try make(desk: "hang-desk")
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["ask the harbor", "no stop and tell me the weather"]
        await push(engine, loud: true, 12)
        await push(engine, loud: false, 45)
        let started = await poll(8) { !HangingDesk.turns.isEmpty }
        XCTAssertTrue(started, "the first turn is being answered")
        // The listener speaks over it: a new session starts at the first loud frame, its live words arrive.
        await push(engine, loud: true, 2)
        let session = await { () -> ScriptedTranscriber.Session? in
            _ = await poll(3) { ScriptedTranscriber.sessions.count >= 2 }
            return ScriptedTranscriber.sessions.last
        }()
        let s = try XCTUnwrap(session)
        s.say("no stop and tell")
        try await Task.sleep(nanoseconds: 150_000_000)
        await push(engine, loud: true, 12)
        let stopped = await poll(5) { pb.stops >= 1 }
        XCTAssertTrue(stopped, "the voice is stopped at once")
        XCTAssertTrue(log.events.contains { if case .note(let n) = $0.event { return n.contains("spoke over") } else { return false } })
        s.say("no stop and tell me the weather")
        await push(engine, loud: false, 45)
        let second = await poll(8) { HangingDesk.turns.count >= 2 }
        XCTAssertTrue(second, "the cut-in becomes the next turn")
        XCTAssertEqual(HangingDesk.turns.last?.listenerWords, "no stop and tell me the weather")
    }

    // H3: the radio's own words, heard back through the microphone, are not a cut-in.
    func testTheRadiosOwnWordsDoNotCutItOff() async throws {
        let (engine, _, pb) = try make(desk: "hang-desk")
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["ask the harbor"]
        await push(engine, loud: true, 12)
        await push(engine, loud: false, 45)
        _ = await poll(8) { !HangingDesk.turns.isEmpty }
        // The radio is saying a sentence; the microphone hears the same words.
        let lists = await engine.hf.heardLists
        let list = try XCTUnwrap(lists.first)
        list.add("The harbor council voted on Tuesday")
        await push(engine, loud: false, 2)                          // the policy learns the sentence started
        await push(engine, loud: true, 2)
        let session = await { () -> ScriptedTranscriber.Session? in
            _ = await poll(3) { ScriptedTranscriber.sessions.count >= 2 }
            return ScriptedTranscriber.sessions.last
        }()
        let s = try XCTUnwrap(session)
        s.say("the harbor council voted on tuesday")
        try await Task.sleep(nanoseconds: 150_000_000)
        await push(engine, loud: true, 14)
        XCTAssertEqual(pb.stops, 0, "its own words must not stop it")
    }

    // H8: without echo cancellation two words are not enough.
    func testWithoutEchoCancellationTwoNewWordsDoNotCutIn() async throws {
        let (engine, _, pb) = try make(desk: "hang-desk")
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        await engine.handsFreeConfigure(echoCancelled: false)
        ScriptedTranscriber.finals = ["ask the harbor"]
        await push(engine, loud: true, 12)
        await push(engine, loud: false, 45)
        _ = await poll(8) { !HangingDesk.turns.isEmpty }
        await push(engine, loud: true, 2)
        _ = await poll(3) { ScriptedTranscriber.sessions.count >= 2 }
        ScriptedTranscriber.sessions.last?.say("hello there")
        try await Task.sleep(nanoseconds: 150_000_000)
        await push(engine, loud: true, 14)
        XCTAssertEqual(pb.stops, 0)
    }
}
