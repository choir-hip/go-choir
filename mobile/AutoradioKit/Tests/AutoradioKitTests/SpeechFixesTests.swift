import XCTest
@testable import AutoradioKit

// Autoradio phone review slice (2026-11-15): the failure list, written before the code. Each fails on origin/main 62383b472.
//
// S3  Failures on the Play path leave no trace and no words on the Play control.
//       S3a ProgrammePlayer.playBlock swallows a failure to make the voice (try?): it must emit an error event for stage "voice"
//       S3b a start failure and a voice failure must reach the player's message (ChoirApp.plain, by stage "start" and "voice"),
//           not the generic "Something went wrong with the radio."
//       S3c a voice that yields no samples at all is a "voice" error too, not a silent false
//     (The engine-start failure is already logged by LabRuntime.start as stage "start"; only its presentation was missing.)

final class SpeechFixesTests: XCTestCase {
    final class Count: @unchecked Sendable {
        private let l = NSLock(); private var n = 0
        func bump() { l.lock(); n += 1; l.unlock() }
        var value: Int { l.lock(); defer { l.unlock() }; return n }
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
        XCTAssertNotEqual(start, "Something went wrong with the radio.")
        XCTAssertNotEqual(voice, "Something went wrong with the radio.")
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
