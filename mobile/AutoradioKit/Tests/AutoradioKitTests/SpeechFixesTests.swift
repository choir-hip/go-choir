import XCTest
@testable import AutoradioKit

// Autoradio phone review slice (2026-11-15): the failure list, written before the code. Each fails on origin/main 62383b472.
//
// S1  The interview asks its first question twice. startInterview calls session.start(), which records the question as the
//     radio's turn; InterviewProgramme.load then sees a running session and calls resume(), which asks the current question
//     again and records it again. Two identical radio turns show in "So far". A resume of a session whose current question is
//     already the last thing the radio recorded must hand back the question to be spoken and record nothing new.
//     (A paused session that is resumed still records its question: the pause line came last.)
// S2  The system voice ends a paragraph at the synthesizer's first empty buffer. AVSpeechSynthesizer.write sends empty buffers
//     between phrases (Mike's speech README: it "cut every chunk off after about 13 seconds"). The end of an utterance is the
//     delegate's didFinish (or didCancel); an empty buffer counts as the end only when nothing at all follows it for a quiet
//     interval, for a system that never calls the delegate. UtteranceEnd is that rule.
//       S2a an empty buffer followed by more audio is not the end
//       S2b didFinish is the end, at once
//       S2c an empty buffer with nothing after it, for the quiet interval, is the end
//       S2d the end is reported once
// S3  Failures on the Play path leave no trace and no words on the Play control.
//       S3a ProgrammePlayer.playBlock swallows a failure to make the voice (try?): it must emit an error event for stage "voice"
//       S3b a start failure and a voice failure must reach the player's message (ChoirApp.plain, by stage "start" and "voice"),
//           not the generic "Something went wrong loading the radio."
//     (The engine-start failure is already logged by LabRuntime.start as stage "start"; only its presentation was missing.)
// S4  The interview screen says "The microphone is open. Talk any time, even over me." whatever the talk setting is. The line
//     must say what is true for the setting: button, word or open.

final class SpeechFixesTests: XCTestCase {
    final class Count: @unchecked Sendable {
        private let l = NSLock(); private var n = 0
        func bump() { l.lock(); n += 1; l.unlock() }
        var value: Int { l.lock(); defer { l.unlock() }; return n }
    }

    func testS1_ResumeOfARunningSessionDoesNotRecordTheQuestionAgain() {
        var i = Interview()
        let first = i.start()
        let asked = i.state.turns.filter { $0.who == "radio" }.count
        let again = i.resume()                                              // what InterviewProgramme.load does after start()
        XCTAssertEqual(i.state.turns.filter { $0.who == "radio" }.count, asked, "the question is recorded once")
        XCTAssertEqual(again.last?.text, first.last?.text, "but it is still handed back to be spoken")
        XCTAssertEqual(again.last?.kind, "question")
    }

    func testS1_TheProgrammeLoadAfterStartLeavesOneQuestionInTheTalk() async throws {
        let s = InterviewSession(interview: Interview(), file: nil)
        InterviewSession.shared = s
        defer { InterviewSession.shared = nil }
        _ = s.start()
        let doc = try await InterviewProgramme(StageParams()).load()
        XCTAssertFalse(doc.blocks.isEmpty)
        let q = s.snapshot.current!.id
        XCTAssertEqual(s.snapshot.state.turns.filter { $0.who == "radio" && $0.questionID == q }.count, 1)
    }

    func testS1_AResumeAfterAPauseStillRecordsTheQuestion() {
        var i = Interview(); _ = i.start()
        _ = i.silence(); _ = i.silence(); _ = i.silence(); _ = i.silence()    // the silence ladder to a pause
        XCTAssertEqual(i.status, .paused)
        let before = i.state.turns.count
        let l = i.resume()
        XCTAssertEqual(l.last?.kind, "question")
        XCTAssertEqual(i.state.turns.count, before + 1)
    }

    func testS2a_AnEmptyBufferFollowedByAudioIsNotTheEnd() async throws {
        let ended = Count()
        let u = UtteranceEnd(quiet: 0.1) { ended.bump() }
        u.buffer(empty: false); u.buffer(empty: true)
        try await Task.sleep(nanoseconds: 40_000_000)
        u.buffer(empty: false)
        try await Task.sleep(nanoseconds: 250_000_000)
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

    func testS4_TheListeningLineIsTrueForEachTalkSetting() {
        XCTAssertTrue(AppSettings.listeningLine(forTalkMode: "open").contains("Talk any time"))
        for m in ["button", "word", ""] { XCTAssertFalse(AppSettings.listeningLine(forTalkMode: m).contains("Talk any time"), m) }
        XCTAssertTrue(AppSettings.listeningLine(forTalkMode: "button").contains("talk button"))
        XCTAssertEqual(AppSettings.listeningLine(forTalkMode: ""), AppSettings.listeningLine(forTalkMode: "button"), "the default is tap to talk")
        XCTAssertTrue(AppSettings.listeningLine(forTalkMode: "word").contains("talk button"))
    }
}
