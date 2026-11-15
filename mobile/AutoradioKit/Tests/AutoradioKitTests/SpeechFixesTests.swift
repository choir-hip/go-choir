import XCTest
@testable import AutoradioKit

// Autoradio phone review slice (2026-11-15): the failure list, written before the code. Each fails on origin/main 62383b472.
//
// S1  The interview asks its first question twice and never speaks its open line. startInterview called session.start() and
//     threw the lines away; InterviewProgramme.load then saw a running session and resumed it, so a first interview spoke
//     "Welcome back." and the question, and the question was recorded twice. continueInterview resumed directly and then
//     again through the loader. The loader is the one place that starts or resumes a session on the engine; the module no
//     longer pre-starts or pre-resumes there.
//       S1a a fresh session loaded by the programme speaks the open line first, then the question, records the question once
//       S1b a paused session loaded by the programme speaks the resume line first, then the question, records it once more
//       S1c (the module's two call sites are in the Exact package, not in this target: read, not run here)
// S2  The system voice ends a paragraph at the synthesizer's first empty buffer. AVSpeechSynthesizer.write sends empty buffers
//     between phrases (Mike's speech README: it "cut every chunk off after about 13 seconds"). The end of an utterance is the
//     delegate's didFinish (or didCancel) for that utterance; an empty buffer counts as the end only when nothing at all
//     follows it for a quiet interval, for a system that never calls the delegate. UtteranceEnd is that rule. The 1 s
//     interval is Mike's and is not verified on a device.
//       S2a an empty buffer followed by more audio is not the end
//       S2b didFinish is the end, at once
//       S2c an empty buffer with nothing after it, for the quiet interval, is the end
//       S2d the end is reported once
//       S2e a line that yields no buffer at all ends after the start interval, not never
//       S2f a buffer after arming keeps the line open
// S3  Failures on the Play path leave no trace and no words on the Play control.
//       S3a ProgrammePlayer.playBlock swallows a failure to make the voice (try?): it must emit an error event for stage "voice"
//       S3b a start failure and a voice failure must reach the player's message (ChoirApp.plain, by stage "start" and "voice"),
//           not the generic "Something went wrong loading the radio."
//       S3c a voice that yields no samples at all is a "voice" error too, not a silent false
//     (The engine-start failure is already logged by LabRuntime.start as stage "start"; only its presentation was missing.)

final class SpeechFixesTests: XCTestCase {
    final class Count: @unchecked Sendable {
        private let l = NSLock(); private var n = 0
        func bump() { l.lock(); n += 1; l.unlock() }
        var value: Int { l.lock(); defer { l.unlock() }; return n }
    }

    func testS1a_AFreshSessionLoadedByTheProgrammeSpeaksTheOpenLineAndTheQuestionOnce() async throws {
        let s = InterviewSession(interview: Interview(), file: nil)
        InterviewSession.shared = s
        defer { InterviewSession.shared = nil }
        let doc = try await InterviewProgramme(StageParams()).load()      // no start() before it: the loader starts the session
        let spoken = doc.blocks.compactMap(\.spokenText)
        let bank = InterviewBank.default
        XCTAssertEqual(spoken.first, bank.line("open"), "a first interview opens with its open line")
        XCTAssertFalse(spoken.contains(bank.line("resume")), "and never says welcome back")
        XCTAssertEqual(spoken.last, s.snapshot.current!.ask)
        let q = s.snapshot.current!.id
        XCTAssertEqual(s.snapshot.state.turns.filter { $0.who == "radio" && $0.questionID == q }.count, 1)
    }

    func testS1b_APausedSessionLoadedByTheProgrammeSpeaksTheResumeLineAndTheQuestion() async throws {
        let s = InterviewSession(interview: Interview(), file: nil)
        InterviewSession.shared = s
        defer { InterviewSession.shared = nil }
        _ = s.start()
        _ = s.silence(); _ = s.silence(); _ = s.silence(); _ = s.silence()
        XCTAssertEqual(s.snapshot.status, .paused)
        let q = s.snapshot.current!.id
        let before = s.snapshot.state.turns.filter { $0.who == "radio" && $0.questionID == q }.count
        let doc = try await InterviewProgramme(StageParams()).load()
        let spoken = doc.blocks.compactMap(\.spokenText)
        XCTAssertEqual(spoken.first, InterviewBank.default.line("resume"))
        XCTAssertEqual(spoken.last, s.snapshot.current!.ask)
        XCTAssertEqual(s.snapshot.state.turns.filter { $0.who == "radio" && $0.questionID == q }.count, before + 1)
    }

    func testS2a_AnEmptyBufferFollowedByAudioIsNotTheEnd() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 1.0, startQuiet: 30) { ended.bump() }   // the next buffer comes after 10 ms, a hundredth of the quiet
        u.buffer(empty: false); u.buffer(empty: true)
        try await Task.sleep(nanoseconds: 10_000_000)
        u.buffer(empty: false)
        try await Task.sleep(nanoseconds: 1_500_000_000)
        XCTAssertEqual(ended.value, 0)
    }

    func testS2b_DidFinishIsTheEnd() {
        let ended = Count()
        let u = UtteranceEnd(quiet: 5) { ended.bump() }
        u.buffer(empty: false); u.buffer(empty: true)
        XCTAssertEqual(ended.value, 0)
        u.didFinish()
        XCTAssertEqual(ended.value, 1)
    }

    func testS2c_AnEmptyBufferThenQuietIsTheEnd() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 0.05) { ended.bump() }
        u.buffer(empty: false); u.buffer(empty: true)
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(ended.value, 1)
    }

    func testS2d_TheEndIsReportedOnce() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 0.05) { ended.bump() }
        u.buffer(empty: true); u.didFinish(); u.didFinish()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(ended.value, 1)
    }

    func testS2e_ALineThatYieldsNoBufferEndsAfterTheStartInterval() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 5, startQuiet: 0.05) { ended.bump() }
        u.arm()
        try await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertEqual(ended.value, 1)
    }

    func testS2f_ABufferAfterArmingKeepsTheLineOpen() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 5, startQuiet: 1.0) { ended.bump() }
        u.arm()
        u.buffer(empty: false)
        try await Task.sleep(nanoseconds: 1_500_000_000)
        XCTAssertEqual(ended.value, 0)
    }

    func testS3a_AVoiceThatCannotBeMadeIsAnErrorInTheTrace() async {
        let r = StageRegistry.builtIn()
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "no-such-voice"))
        let log = EventLog(clock: LabClock())
        let doc = ProgrammeDocument(id: "d", title: "T", blocks: [.speech(text: "Hello.")])
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: log, configuration: cfg, document: doc)
        let played = await p.playBlock(.speech(text: "Hello."), index: 0)
        XCTAssertFalse(played)
        let errs = log.events.compactMap { e -> String? in if case .error(let st, let m) = e.event, st == "voice" { return m } else { return nil } }
        XCTAssertEqual(errs.count, 1, "one voice error, naming the stage that could not be made")
        XCTAssertTrue(errs.first?.contains("no-such-voice") ?? false, "\(errs)")
    }

    func testS3b_StartAndVoiceFailuresHaveTheirOwnWordsForThePlayControl() {
        let start = ChoirApp.plain(stage: "start", message: "start failed: boom")
        let voice = ChoirApp.plain(stage: "voice", message: "unknownStage")
        XCTAssertNotEqual(start, "Something went wrong loading the radio.")
        XCTAssertNotEqual(voice, "Something went wrong loading the radio.")
        XCTAssertNotEqual(start, voice)
        XCTAssertTrue(ChoirApp.surfacedErrorStages.isSuperset(of: ["start", "voice"]))
    }

    final class SilentVoice: Voice, @unchecked Sendable {
        static let stageID = "silent-voice"
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "silent voice") }
        init(_ p: StageParams) {}
        func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { $0.finish() } }
    }

    func testS3c_AVoiceThatMakesNoSamplesIsAnErrorInTheTrace() async {
        let r = StageRegistry.builtIn()
        r.register(.voice, SilentVoice.self) { SilentVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "silent-voice"))
        let log = EventLog(clock: LabClock())
        let doc = ProgrammeDocument(id: "d", title: "T", blocks: [.speech(text: "Hello.")])
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: log, configuration: cfg, document: doc)
        let played = await p.playBlock(.speech(text: "Hello."), index: 0)
        XCTAssertFalse(played)
        let errs = log.events.filter { if case .error(let st, _) = $0.event { return st == "voice" } else { return false } }
        XCTAssertEqual(errs.count, 1)
    }
}
