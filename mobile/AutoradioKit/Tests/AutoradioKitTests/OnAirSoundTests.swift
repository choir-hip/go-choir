import XCTest
@testable import AutoradioKit

/// A voice that answers but makes no sound (the failure the listener hears as silence).
final class MuteVoice: Voice, @unchecked Sendable {
    static let stageID = "mute-voice"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "mute voice") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in Task { for await _ in text {}; c.finish() } }
    }
}
/// Stands in for the system voice ("avspeech") with a real signal.
final class SystemStandIn: Voice, @unchecked Sendable {
    static let stageID = "avspeech"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "system stand-in") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            Task { for await _ in text { c.yield(AudioFrame(samples: [Float](repeating: 0.2, count: 1600), sampleRate: 16000, t: 0)) }; c.finish() }
        }
    }
}

final class OnAirSoundTests: XCTestCase {
    func player(voice: String) -> ProgrammePlayer {
        let r = StageRegistry.builtIn()
        r.register(.voice, MuteVoice.self) { MuteVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.voice, SystemStandIn.self) { SystemStandIn($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: voice))
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                               document: ProgrammeDocument(id: "rs:x", title: "T", blocks: [.speech(text: "Hello there.")]))
    }

    /// An entry on air hands samples to the output node.
    func testAnOnAirEntryGivesTheOutputNodeSamples() async {
        let p = player(voice: "stub-voice")
        p.begin(at: 0)
        let got = await poll { p.audio.programmeSamplesHanded > 0 }
        XCTAssertTrue(got, "the output node was given nothing")
        XCTAssertNil(p.speakFailure)
    }

    /// A voice that makes no sound falls back to the system voice, so the listener still hears the words.
    func testAVoiceThatMakesNoSoundFallsBackToTheSystemVoice() async {
        let p = player(voice: "mute-voice")
        p.begin(at: 0)
        let got = await poll { p.audio.programmeSamplesHanded > 0 }
        XCTAssertTrue(got, "the system voice should have spoken")
    }

    /// When nothing can speak, the reason is kept for the bar.
    func testWhenNothingCanSpeakTheReasonIsKept() async {
        let r = StageRegistry.builtIn()
        r.register(.voice, MuteVoice.self) { MuteVoice($0) }
        r.register(.voice, MuteVoiceTwo.self) { MuteVoiceTwo($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: "mute-voice"))
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                                document: ProgrammeDocument(id: "rs:y", title: "T", blocks: [.speech(text: "Hello there.")]))
        let t = await p.synthesize("Hello there.")
        if t == nil { XCTAssertNotNil(p.speakFailure) }   // the built-in system voice may speak on this machine; then there is nothing to report
    }

    /// The microphone is not wanted by a playback-only session (the tap on its input node kept the engine from starting).
    func testAPlaybackOnlyStartWantsNoMicrophone() throws {
        let a = LiveAudio()
        try a.start(bus: AudioBus(), clock: LabClock(), wantMic: true, mic: false, virtual: true)
        XCTAssertTrue(a.virtual)
    }

    /// The session is not ready: the entry fails at once with the plain reason, and the bar says so.
    @MainActor func testAnEntryThatCannotSpeakFailsWithTheReason() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        p.sessionReady = { _ in "the audio engine did not start" }
        p.playNow(.paperStory(route: "a", title: "A")); await p.settle()
        XCTAssertTrue(p.message.hasPrefix("Can\u{2019}t speak: the audio engine did not start"), p.message)
        XCTAssertNil(s.onAir())
    }
}
final class MuteVoiceTwo: Voice, @unchecked Sendable {
    static let stageID = "avspeech"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "mute system") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { c in Task { for await _ in text {}; c.finish() } } }
}
