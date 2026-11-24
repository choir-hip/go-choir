import XCTest
@testable import AutoradioKit

// The reader's voice (SpeakerGate). Synthetic embeddings only: the fake model maps the first sample's value to a speaker and adds a small
// deterministic wobble. REAL accuracy on a phone with the real WeSpeaker model is unmeasured: these tests prove the decisions, the
// enrolment rules, the storage and the engine's use of them, not the model.
//
// Failure list (written first): too little speech to enrol; a second speaker among the first turns; the reader with a cold (a little below
// the threshold) still passes; a clear second speaker is a mismatch; short speech, no model, a model returning the wrong size: unknown;
// forget deletes and everything passes again; the profile survives a relaunch; with a learned voice a TV turn is not shown and not sent,
// the reader's is; with no learned voice every turn passes.

final class FakeEmbedder: VoiceEmbedder, @unchecked Sendable {
    var ready = true, size = 256
    var isReady: Bool { ready }
    /// Speaker k (the first sample is 0.1 * k) is the unit vector with 1 at index k, plus a wobble of the given strength on index 100 + n.
    func embed(_ samples: [Float]) async -> [Float]? {
        guard size == 256 else { return Array(repeating: 0.1, count: size) }
        let k = Int((samples.first ?? 0) * 10 + 0.5)
        var v = [Float](repeating: 0, count: 256)
        v[k] = 1
        let wobble = Float(samples.count % 7) * 0.02
        v[100 + samples.count % 50] = wobble
        return v
    }
}

final class SpeakerGateTests: XCTestCase {
    func speech(_ speaker: Int, seconds: Double = 2, variant: Int = 0) -> [Float] {
        [Float](repeating: Float(speaker) * 0.1, count: Int(seconds * 16_000) + variant)
    }
    func gate(_ e: FakeEmbedder? = FakeEmbedder(), url: URL? = nil) -> SpeakerGate { SpeakerGate(url: url, embedder: e) }
    func enrol(_ g: SpeakerGate, speaker: Int = 1, turns: Int = 3) async { for i in 0..<turns { await g.learn(speech(speaker, variant: i * 160)) } }

    func testTooLittleSpeechLearnsNothing() async {
        let g = gate(); await g.learn(speech(1, seconds: 1.0)); await g.learn(speech(1, seconds: 0.5))
        XCTAssertEqual(g.heardTurns, 0); XCTAssertFalse(g.learned)
    }
    func testThreeAlikeTurnsLearnTheVoice() async {
        let g = gate(); await enrol(g, turns: 2); XCTAssertFalse(g.learned); XCTAssertEqual(g.heardTurns, 2)
        await g.learn(speech(1, variant: 480)); XCTAssertTrue(g.learned)
    }
    func testAVisitorAmongTheFirstTurnsDoesNotBecomeTheVoice() async {
        let g = gate()
        await g.learn(speech(1)); await g.learn(speech(2, variant: 160)); await g.learn(speech(1, variant: 320))
        XCTAssertFalse(g.learned, "two of the reader and one of a visitor are not three alike turns")
        await g.learn(speech(1, variant: 480)); XCTAssertTrue(g.learned)
        let r = await g.judge(speech(1, variant: 640)); let v = await g.judge(speech(2, variant: 800))
        XCTAssertEqual(r, .match); XCTAssertEqual(v, .mismatch, "the visitor's voice was not learned")
    }
    func testAClearSecondSpeakerIsAMismatchAndTheReaderMatches() async {
        let g = gate(); await enrol(g)
        let r = await g.judge(speech(1, variant: 960)); let tv = await g.judge(speech(3, variant: 1120))
        XCTAssertEqual(r, .match); XCTAssertEqual(tv, .mismatch)
    }
    func testShortSpeechNoModelOrAWrongSizeIsUnknownAndPasses() async {
        let e = FakeEmbedder(); let g = gate(e); await enrol(g)
        let short = await g.judge(speech(3, seconds: 0.5)); XCTAssertEqual(short, .unknown)
        e.size = 12; let wrong = await g.judge(speech(3)); XCTAssertEqual(wrong, .unknown)
        e.size = 256; e.ready = false; let notReady = await g.judge(speech(3)); XCTAssertEqual(notReady, .unknown)
        g.embedder = nil; let none = await g.judge(speech(3)); XCTAssertEqual(none, .unknown)
    }
    func testNoLearnedVoiceEveryTurnPasses() async {
        let g = gate(); let c = await g.judge(speech(3)); XCTAssertEqual(c, .unknown)
    }
    func testTheDecisionErrsTowardAcceptingTheReader() {
        func v(_ m: Float) -> SpeakerProfile.Verdict { .init(outcome: m >= 0 ? .match : .mismatch, similarities: [:], margin: m) }
        XCTAssertEqual(SpeakerGate.call(for: v(0.1)), .match)
        XCTAssertEqual(SpeakerGate.call(for: v(-0.05)), .unknown, "a little below the threshold (a cold): still passes")
        XCTAssertEqual(SpeakerGate.call(for: v(-0.2)), .mismatch)
        XCTAssertEqual(SpeakerGate.call(for: .unknown), .unknown)
    }
    func testForgetDeletesTheVoiceAndEverythingPassesAgain() async throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("speaker-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: url) }
        let g = gate(url: url); await enrol(g)
        XCTAssertTrue(FileManager.default.fileExists(atPath: url.path))
        g.forget()
        XCTAssertFalse(g.learned); XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
        let c = await g.judge(speech(3)); XCTAssertEqual(c, .unknown)
    }
    func testTheVoiceSurvivesARelaunchAndIsNotAudio() async throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("speaker-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: url) }
        let g = gate(url: url); await enrol(g)
        let again = gate(url: url)
        XCTAssertTrue(again.learned)
        let c = await again.judge(speech(3)); XCTAssertEqual(c, .mismatch)
        let size = (try FileManager.default.attributesOfItem(atPath: url.path)[.size] as? Int) ?? 0
        XCTAssertLessThan(size, 20_000, "a few hundred numbers, never audio")
    }
    func testTheShippedAttributionNamesTheSources() {
        XCTAssertTrue(WeSpeakerEmbedder.attribution.contains("WeSpeaker")); XCTAssertTrue(WeSpeakerEmbedder.attribution.contains("CC BY 4.0"))
        XCTAssertEqual(WeSpeakerEmbedder.files.last?.size, 7_243_904)
    }
    func testSHA256OfKnownText() {
        XCTAssertEqual(WeSpeakerEmbedder.sha256Hex(Data("abc".utf8)), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
    }

    // MARK: the engine

    func engine(_ g: SpeakerGate, log: EventLog = EventLog(clock: LabClock())) throws -> TurnEngine {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.speechBounds, LoudBounds.self) { LoudBounds($0) }
        r.register(.transcriber, ScriptedTranscriber.self) { ScriptedTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.addressing, .init(id: "open")); cfg.set(.speechBounds, .init(id: "loud-bounds")); cfg.set(.transcriber, .init(id: "scripted-tr"))
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.holder, .init(id: "local-ack")); cfg.set(.desk, .init(id: "rec-desk"))
        RecordingDesk.turns = []; ScriptedTranscriber.finals = []; ScriptedTranscriber.sessions = []
        var env = EngineEnvironment(registry: r, log: log, playback: StoppablePlayback())
        env.speakerGate = g
        return TurnEngine(env: env, configuration: cfg)
    }
    var clock = 0.0
    func frame(_ speaker: Int, loud: Bool = true) -> AudioFrame {
        defer { clock += 0.032 }
        return AudioFrame(samples: [Float](repeating: loud ? 0.3 + Float(speaker) * 0.1 : 0, count: 512), sampleRate: 16000, t: clock)
    }
    // Speaker k's loud frames carry 0.3 + 0.1 k: the fake model reads (first sample - 0.3) as k.
    func talk(_ e: TurnEngine, speaker: Int, words: String) async throws {
        ScriptedTranscriber.finals = [words]
        for _ in 0..<45 { await e.ingestHandsFree(frame(speaker)) }
        ScriptedTranscriber.sessions.last?.say(words)
        for _ in 0..<45 { await e.ingestHandsFree(frame(0, loud: false)) }
        try await Task.sleep(nanoseconds: 400_000_000)
    }

    func testWithALearnedVoiceATVTurnIsLoggedAndStillSentInThisBuild() async throws {
        let embed = EngineEmbedder()
        let g = SpeakerGate(url: nil, embedder: embed)
        for i in 0..<3 { await g.learn([Float](repeating: 0.3 + 0.1, count: 32_000 + i * 160)) }     // the reader: speaker 1
        XCTAssertTrue(g.learned)
        let log = EventLog(clock: LabClock()); let e = try engine(g, log: log)
        try await talk(e, speaker: 3, words: "buy now while stocks last")         // the TV
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "buy now while stocks last", "the check only logs: another voice is still sent")
        try await talk(e, speaker: 1, words: "what is on today")                  // the reader
        XCTAssertEqual(RecordingDesk.turns.last?.listenerWords, "what is on today")
        let notes = log.events.compactMap { ev -> String? in if case .note(let n) = ev.event, n.hasPrefix("speaker: ") { return n }; return nil }
        XCTAssertEqual(notes.count, 2, "one verdict per turn")
        XCTAssertTrue(notes[0].hasPrefix("speaker: mismatch"), notes[0]); XCTAssertTrue(notes[1].hasPrefix("speaker: match"), notes[1])
    }
    func testWithNoLearnedVoiceEveryTurnIsSentAsToday() async throws {
        let g = SpeakerGate(url: nil, embedder: EngineEmbedder())
        let e = try engine(g)
        try await talk(e, speaker: 3, words: "anyone at all")
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "anyone at all")
    }
}

/// Maps a frame's amplitude to a speaker: 0.4 is speaker 1, 0.6 speaker 3 (the engine test's loud frames are 0.3 + 0.1 k).
final class EngineEmbedder: VoiceEmbedder, @unchecked Sendable {
    var isReady: Bool { true }
    func embed(_ samples: [Float]) async -> [Float]? {
        let k = Int(((samples.max() ?? 0) - 0.3) * 10 + 0.5)       // the loudest sample: quiet pre-roll frames do not matter
        var v = [Float](repeating: 0, count: 256); v[max(0, k)] = 1; v[100 + samples.count % 50] = 0.05
        return v
    }
}
